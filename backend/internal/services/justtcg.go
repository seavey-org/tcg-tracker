package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/codyseavey/tcg-tracker/backend/internal/models"
)

const (
	justTCGBaseURL   = "https://api.justtcg.com/v1"
	justTCGBatchSize = 100 // Max cards per request (paid tier)
	justTCGRateLimit = 50  // Requests per minute (paid tier)
)

// JustTCGService handles API calls to JustTCG for card pricing
type JustTCGService struct {
	client  *http.Client
	apiKey  string
	baseURL string

	// Rate limiting (50 requests per minute for paid tier)
	rateLimiter *rate.Limiter

	// Daily/monthly tracking
	mu             sync.Mutex
	dailyLimit     int
	requestsToday  int
	lastRequestDay time.Time
}

// JustTCG API response structures (matching actual API)
type JustTCGResponse struct {
	Data     []JustTCGCard `json:"data"`
	Meta     *JustTCGMeta  `json:"meta,omitempty"`
	Metadata JustTCGUsage  `json:"_metadata"`
	Error    string        `json:"error,omitempty"`
	Code     string        `json:"code,omitempty"`
}

type JustTCGCard struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Game        string           `json:"game"`
	Set         string           `json:"set"`
	SetName     string           `json:"set_name"`
	Number      string           `json:"number"`
	TCGPlayerID string           `json:"tcgplayerId"`
	ScryfallID  string           `json:"scryfallId"`
	Rarity      string           `json:"rarity"`
	Variants    []JustTCGVariant `json:"variants"`
}

type JustTCGVariant struct {
	ID          string  `json:"id"`
	Printing    string  `json:"printing"`  // "Normal", "Foil", "1st Edition", etc.
	Condition   string  `json:"condition"` // "Near Mint", "Lightly Played", etc.
	Language    string  `json:"language"`
	Price       float64 `json:"price"`
	LastUpdated int64   `json:"lastUpdated"`
}

type JustTCGMeta struct {
	Total   int  `json:"total"`
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	HasMore bool `json:"hasMore"`
}

type JustTCGUsage struct {
	APIPlan                   string `json:"apiPlan"`
	APIRequestLimit           int    `json:"apiRequestLimit"`
	APIRequestsUsed           int    `json:"apiRequestsUsed"`
	APIRequestsRemaining      int    `json:"apiRequestsRemaining"`
	APIDailyLimit             int    `json:"apiDailyLimit"`
	APIDailyRequestsUsed      int    `json:"apiDailyRequestsUsed"`
	APIDailyRequestsRemaining int    `json:"apiDailyRequestsRemaining"`
	APIRateLimit              int    `json:"apiRateLimit"`
}

// CardLookup specifies how to find a card in JustTCG
type CardLookup struct {
	CardID      string // Our internal card ID (for mapping results back)
	TCGPlayerID string `json:"tcgplayerId,omitempty"`
	ScryfallID  string `json:"scryfallId,omitempty"` // For MTG cards - enables batch POST lookup
	Name        string `json:"q,omitempty"`
	Set         string `json:"set,omitempty"`      // Our set code (not sent to API, used internally)
	SetName     string `json:"set_name,omitempty"` // Human-readable set name for matching results
	Game        string `json:"game,omitempty"`
}

// batchPostRequest is the request body for batch POST lookups
type batchPostRequest struct {
	ScryfallID  string `json:"scryfallId,omitempty"`
	TCGPlayerID string `json:"tcgplayerId,omitempty"`
	Condition   string `json:"condition,omitempty"`
}

// BatchPriceResult contains prices and any discovered external IDs
type BatchPriceResult struct {
	Prices            map[string][]models.CardPrice // cardID -> prices
	DiscoveredTCGPIDs map[string]string             // cardID -> tcgplayerId (discovered during fetch)
}

// NewJustTCGService creates a new JustTCG API service
func NewJustTCGService(apiKey string, dailyLimit int) *JustTCGService {
	if dailyLimit <= 0 {
		dailyLimit = 1000 // Default paid tier limit
	}

	// Rate limiter: 50 requests per minute (paid tier) = 1 request every 1.2 seconds
	limiter := rate.NewLimiter(rate.Every(1200*time.Millisecond), 1)

	return &JustTCGService{
		client: &http.Client{
			Timeout: 30 * time.Second, // Longer timeout for batch requests
		},
		apiKey:      apiKey,
		baseURL:     justTCGBaseURL,
		dailyLimit:  dailyLimit,
		rateLimiter: limiter,
	}
}

// checkDailyLimit checks if we can make another request today
// Returns true if request can proceed, false if rate limited
func (s *JustTCGService) checkDailyLimit() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Reset counter if new day
	if s.lastRequestDay.Before(today) {
		s.requestsToday = 0
		s.lastRequestDay = today
	}

	if s.requestsToday >= s.dailyLimit {
		return false
	}

	s.requestsToday++
	return true
}

// GetRequestsRemaining returns the number of requests remaining today
func (s *JustTCGService) GetRequestsRemaining() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Reset counter if new day
	if s.lastRequestDay.Before(today) {
		return s.dailyLimit
	}

	remaining := s.dailyLimit - s.requestsToday
	if remaining < 0 {
		return 0
	}
	return remaining
}

// GetDailyLimit returns the configured daily limit
func (s *JustTCGService) GetDailyLimit() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.dailyLimit
}

// GetResetTime returns the next local midnight reset time
func (s *JustTCGService) GetResetTime() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
}

// GetCardPrices fetches condition-specific prices for a single card by name.
// Uses the search endpoint and attempts to match by set when provided.
func (s *JustTCGService) GetCardPrices(cardName, setCode string, game models.Game) ([]models.CardPrice, error) {
	if strings.TrimSpace(cardName) == "" {
		return nil, nil
	}

	gameStr := "pokemon"
	if game == models.GameMTG {
		gameStr = "magic-the-gathering"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed: %w", err)
	}

	if !s.checkDailyLimit() {
		return nil, fmt.Errorf("JustTCG daily rate limit exceeded")
	}

	params := url.Values{}
	params.Set("game", gameStr)
	params.Set("q", cardName)
	params.Set("limit", "50")
	params.Set("include_price_history", "false")
	params.Set("include_statistics", "")

	setParam := ""
	if setCode != "" {
		if game == models.GamePokemon {
			setParam = convertToJustTCGSetID(setCode)
		} else {
			setParam = strings.ToLower(setCode)
		}
	}
	if setParam != "" {
		params.Set("set", setParam)
	}

	reqURL := fmt.Sprintf("%s/cards?%s", s.baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch card prices: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JustTCG API error: status %d", resp.StatusCode)
	}

	var apiResp JustTCGResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if apiResp.Error != "" {
		return nil, fmt.Errorf("JustTCG API error: %s", apiResp.Error)
	}

	s.updateRemaining(apiResp.Metadata.APIDailyRequestsRemaining)

	if len(apiResp.Data) == 0 {
		return nil, nil
	}

	nameTarget := strings.ToLower(extractBaseName(cardName))
	setCodeLower := strings.ToLower(strings.TrimSpace(setCode))
	setParamLower := strings.ToLower(setParam)
	setNameTarget := normalizeSetName(setCode)

	setMatches := func(card JustTCGCard) bool {
		if setCodeLower == "" {
			return true
		}
		cardSet := strings.ToLower(card.Set)
		if setParamLower != "" && cardSet == setParamLower {
			return true
		}
		if cardSet == setCodeLower {
			return true
		}
		if setNameTarget == "" {
			return false
		}
		return normalizeSetName(card.SetName) == setNameTarget
	}

	var matched *JustTCGCard
	var nameMatch *JustTCGCard
	for i := range apiResp.Data {
		card := &apiResp.Data[i]
		if strings.ToLower(extractBaseName(card.Name)) != nameTarget {
			continue
		}
		if setMatches(*card) {
			matched = card
			break
		}
		if nameMatch == nil {
			nameMatch = card
		}
	}

	if matched == nil {
		matched = nameMatch
	}

	if matched == nil {
		return nil, nil
	}

	prices := s.convertVariantsToPrices(matched.Variants)
	if len(prices) == 0 {
		return nil, nil
	}

	return prices, nil
}

// BatchGetPrices fetches prices for multiple cards using the batch POST endpoint.
// All cards must have either TCGPlayerID or ScryfallID - cards without are skipped.
// The price worker is responsible for syncing sets to discover TCGPlayerIDs before calling this.
// Uses 1 API request for up to 100 cards (paid tier).
func (s *JustTCGService) BatchGetPrices(lookups []CardLookup) (*BatchPriceResult, error) {
	if len(lookups) == 0 {
		return &BatchPriceResult{
			Prices:            make(map[string][]models.CardPrice),
			DiscoveredTCGPIDs: make(map[string]string),
		}, nil
	}
	if len(lookups) > justTCGBatchSize {
		return nil, fmt.Errorf("batch size %d exceeds max %d", len(lookups), justTCGBatchSize)
	}

	// Filter to only cards with external IDs (TCGPlayerID or ScryfallID)
	var batchableLookups []CardLookup
	for _, lookup := range lookups {
		if lookup.TCGPlayerID != "" || lookup.ScryfallID != "" {
			batchableLookups = append(batchableLookups, lookup)
		} else {
			log.Printf("JustTCG: skipping card %s - no external ID", lookup.CardID)
		}
	}

	if len(batchableLookups) == 0 {
		return &BatchPriceResult{
			Prices:            make(map[string][]models.CardPrice),
			DiscoveredTCGPIDs: make(map[string]string),
		}, nil
	}

	// Single batch POST request for all cards
	return s.fetchCardsBatchPost(batchableLookups)
}

// fetchCardsBatchPost fetches multiple cards using a single POST request.
// This is the efficient path for cards with TCGPlayerID or ScryfallID.
// Uses 1 API request for up to 100 cards (paid tier).
func (s *JustTCGService) fetchCardsBatchPost(lookups []CardLookup) (*BatchPriceResult, error) {
	if len(lookups) == 0 {
		return &BatchPriceResult{
			Prices:            make(map[string][]models.CardPrice),
			DiscoveredTCGPIDs: make(map[string]string),
		}, nil
	}

	// Wait for rate limiter (single request for the whole batch)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed: %w", err)
	}

	// Check daily limit (counts as 1 request regardless of batch size)
	if !s.checkDailyLimit() {
		return nil, fmt.Errorf("JustTCG daily rate limit exceeded")
	}

	// Build request body - prefer TCGPlayerID, fallback to ScryfallID
	requestBody := make([]batchPostRequest, len(lookups))

	for i, lookup := range lookups {
		req := batchPostRequest{}
		if lookup.TCGPlayerID != "" {
			req.TCGPlayerID = lookup.TCGPlayerID
		} else if lookup.ScryfallID != "" {
			req.ScryfallID = lookup.ScryfallID
		}
		requestBody[i] = req
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal batch request: %w", err)
	}

	// Build URL with query params to reduce response size
	params := url.Values{}
	params.Set("include_price_history", "false")
	params.Set("include_statistics", "")

	reqURL := fmt.Sprintf("%s/cards?%s", s.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to create batch request: %w", err)
	}

	s.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("batch request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JustTCG batch API error: status %d", resp.StatusCode)
	}

	var apiResp JustTCGResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode batch response: %w", err)
	}

	if apiResp.Error != "" {
		return nil, fmt.Errorf("JustTCG batch API error: %s", apiResp.Error)
	}

	result := &BatchPriceResult{
		Prices:            make(map[string][]models.CardPrice),
		DiscoveredTCGPIDs: make(map[string]string),
	}

	// Build lookup maps to match responses by ID (NOT by index!)
	// JustTCG may skip cards it can't find, so index-based matching is WRONG
	tcgPlayerIDToCardID := make(map[string]string)
	scryfallIDToCardID := make(map[string]string)
	for _, lookup := range lookups {
		if lookup.TCGPlayerID != "" {
			tcgPlayerIDToCardID[lookup.TCGPlayerID] = lookup.CardID
		}
		if lookup.ScryfallID != "" {
			scryfallIDToCardID[lookup.ScryfallID] = lookup.CardID
		}
	}

	// Match responses by TCGPlayerID or ScryfallID
	for _, card := range apiResp.Data {
		prices := s.convertVariantsToPrices(card.Variants)
		if len(prices) == 0 {
			continue
		}

		// Find our card ID by matching the response's TCGPlayerID or ScryfallID
		var cardID string
		if card.TCGPlayerID != "" {
			cardID = tcgPlayerIDToCardID[card.TCGPlayerID]
		}
		if cardID == "" && card.ScryfallID != "" {
			cardID = scryfallIDToCardID[card.ScryfallID]
		}

		if cardID == "" {
			log.Printf("JustTCG: received prices for unknown card (tcgplayerId=%s, scryfallId=%s, name=%s)",
				card.TCGPlayerID, card.ScryfallID, card.Name)
			continue
		}

		result.Prices[cardID] = prices
	}

	s.updateRemaining(apiResp.Metadata.APIDailyRequestsRemaining)

	log.Printf("JustTCG: batch POST fetched %d/%d cards with prices (remaining: %d daily)",
		len(result.Prices), len(lookups), apiResp.Metadata.APIDailyRequestsRemaining)

	return result, nil
}

// setHeaders sets common headers for API requests
func (s *JustTCGService) setHeaders(req *http.Request) {
	if s.apiKey != "" {
		req.Header.Set("X-API-Key", s.apiKey)
	}
	req.Header.Set("Accept", "application/json")
}

// updateRemaining syncs our counters with JustTCG metadata
func (s *JustTCGService) updateRemaining(remaining int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if remaining < 0 {
		return
	}

	// Infer requestsToday from remaining, keep same daily limit
	requestsToday := s.dailyLimit - remaining
	if requestsToday < 0 {
		requestsToday = 0
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	s.requestsToday = requestsToday
	s.lastRequestDay = today
}

// convertVariantsToPrices converts JustTCG variants to our CardPrice model
// Includes language information for accurate pricing of foreign cards
func (s *JustTCGService) convertVariantsToPrices(variants []JustTCGVariant) []models.CardPrice {
	var prices []models.CardPrice
	now := time.Now()

	for _, v := range variants {
		condition := mapJustTCGCondition(v.Condition)
		printing := mapJustTCGPrinting(v.Printing)
		language := models.NormalizeLanguage(v.Language)

		if condition == "" {
			continue
		}
		if printing == "" {
			printing = models.PrintingNormal
		}

		// Skip if price is 0 or negative
		if v.Price <= 0 {
			continue
		}

		prices = append(prices, models.CardPrice{
			Condition:      condition,
			Printing:       printing,
			Language:       language,
			PriceUSD:       v.Price,
			Source:         "justtcg",
			PriceUpdatedAt: &now,
		})
	}

	return prices
}

// mapJustTCGCondition maps JustTCG condition strings to our PriceCondition type
func mapJustTCGCondition(condition string) models.PriceCondition {
	switch strings.ToLower(condition) {
	case "near mint", "nm":
		return models.PriceConditionNM
	case "lightly played", "lp":
		return models.PriceConditionLP
	case "moderately played", "mp":
		return models.PriceConditionMP
	case "heavily played", "hp":
		return models.PriceConditionHP
	case "damaged", "dmg":
		return models.PriceConditionDMG
	default:
		return ""
	}
}

// mapJustTCGPrinting maps JustTCG printing strings to our PrintingType
func mapJustTCGPrinting(printing string) models.PrintingType {
	switch printing {
	case "Normal":
		return models.PrintingNormal
	case "Foil":
		return models.PrintingFoil
	case "1st Edition":
		return models.Printing1stEdition
	case "Unlimited":
		return models.PrintingUnlimited
	case "Reverse Holofoil":
		return models.PrintingReverseHolo
	default:
		// Try case-insensitive match
		switch strings.ToLower(printing) {
		case "normal":
			return models.PrintingNormal
		case "foil", "holo", "holofoil":
			return models.PrintingFoil
		case "1st edition", "first edition":
			return models.Printing1stEdition
		case "unlimited":
			return models.PrintingUnlimited
		case "reverse holofoil", "reverse holo", "reverse":
			return models.PrintingReverseHolo
		default:
			return ""
		}
	}
}

// normalizeSetName normalizes set names for matching between our database and JustTCG
// Our set names (e.g., "Base") need to match JustTCG's (e.g., "Base Set")
func normalizeSetName(name string) string {
	name = strings.TrimSpace(name)

	// Explicit normalizations for sets where our names differ from JustTCG's
	// Our name (lowercase) → JustTCG's name (lowercase)
	normalizations := map[string]string{
		"base":                "base set",   // We store "Base", JustTCG uses "Base Set"
		"expedition base set": "expedition", // We store "Expedition Base Set", JustTCG uses "Expedition"
	}

	lower := strings.ToLower(name)
	if normalized, ok := normalizations[lower]; ok {
		return normalized
	}

	return lower
}

// extractBaseName extracts the base card name, stripping suffixes like "(H4)" or "(1st Edition)"
// JustTCG returns names like "Beedrill (H4)" but our cards are just "Beedrill"
func extractBaseName(name string) string {
	// Strip anything in parentheses at the end
	if idx := strings.LastIndex(name, " ("); idx > 0 {
		return strings.TrimSpace(name[:idx])
	}
	return name
}

// SetTCGPlayerIDMap holds TCGPlayerIDs for cards in a set, keyed by card number
type SetTCGPlayerIDMap struct {
	SetName     string            // JustTCG set name
	CardsByNum  map[string]string // card number -> TCGPlayerID
	CardsByName map[string]string // lowercase card name -> TCGPlayerID (fallback)
	TotalCards  int
}

// FetchSetTCGPlayerIDs fetches all cards from a JustTCG set and returns their TCGPlayerIDs.
// This enables bulk prepopulation of TCGPlayerIDs for efficient batching.
// setID should be in JustTCG format, e.g., "vivid-voltage-pokemon"
func (s *JustTCGService) FetchSetTCGPlayerIDs(setID string) (*SetTCGPlayerIDMap, error) {
	// Wait for rate limiter
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := s.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed: %w", err)
	}

	// Check daily limit
	if !s.checkDailyLimit() {
		return nil, fmt.Errorf("JustTCG daily rate limit exceeded")
	}

	result := &SetTCGPlayerIDMap{
		SetName:     setID,
		CardsByNum:  make(map[string]string),
		CardsByName: make(map[string]string),
	}

	// Fetch all cards from the set (paginated)
	offset := 0
	limit := 100 // Paid tier allows 100 per request

	for {
		params := url.Values{}
		params.Set("game", "pokemon")
		params.Set("set", setID)
		params.Set("limit", fmt.Sprintf("%d", limit))
		params.Set("offset", fmt.Sprintf("%d", offset))
		params.Set("include_price_history", "false")
		params.Set("include_statistics", "")

		reqURL := fmt.Sprintf("%s/cards?%s", s.baseURL, params.Encode())

		req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		s.setHeaders(req)

		resp, err := s.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch set cards: %w", err)
		}

		var apiResp JustTCGResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		resp.Body.Close()

		if apiResp.Error != "" {
			return nil, fmt.Errorf("JustTCG API error: %s", apiResp.Error)
		}

		s.updateRemaining(apiResp.Metadata.APIDailyRequestsRemaining)

		// Extract TCGPlayerIDs from cards
		for _, card := range apiResp.Data {
			// Debug: log cards with "machamp" in name (case-insensitive)
			if strings.Contains(strings.ToLower(card.Name), "machamp") {
				log.Printf("JustTCG debug: found Machamp variant: name=%q number=%q tcgplayerId=%s", card.Name, card.Number, card.TCGPlayerID)
			}
			if card.TCGPlayerID == "" {
				continue
			}

			// Store by card number (primary match)
			// Handle formats like "073/102", "73", "73/102"
			if card.Number != "" && card.Number != "N/A" {
				result.CardsByNum[card.Number] = card.TCGPlayerID

				// Extract number without /total suffix
				numOnly := card.Number
				if idx := strings.Index(card.Number, "/"); idx > 0 {
					numOnly = card.Number[:idx]
					result.CardsByNum[numOnly] = card.TCGPlayerID
				}

				// Also store without leading zeros (e.g., "073" -> "73")
				numStripped := strings.TrimLeft(numOnly, "0")
				if numStripped == "" {
					numStripped = "0"
				}
				if numStripped != numOnly {
					result.CardsByNum[numStripped] = card.TCGPlayerID
				}
			}

			// Store by name (fallback match) - normalize for special characters
			baseName := normalizeNameForPriceMatch(extractBaseName(card.Name))
			result.CardsByName[baseName] = card.TCGPlayerID
		}

		result.TotalCards += len(apiResp.Data)

		// Check if there are more pages
		if apiResp.Meta == nil || !apiResp.Meta.HasMore {
			break
		}

		offset += limit

		// Check daily limit before each pagination request
		if !s.checkDailyLimit() {
			log.Printf("JustTCG: daily limit reached during pagination for set %s", setID)
			break
		}

		// Rate limit between pages
		if err := s.rateLimiter.Wait(ctx); err != nil {
			log.Printf("JustTCG: rate limit wait failed during pagination: %v", err)
			break
		}
	}

	log.Printf("JustTCG: fetched %d cards from set %s, found %d TCGPlayerIDs",
		result.TotalCards, setID, len(result.CardsByNum))

	return result, nil
}

// DebugSearchCard searches JustTCG for a card by name and logs the results
func (s *JustTCGService) DebugSearchCard(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.rateLimiter.Wait(ctx); err != nil {
		log.Printf("JustTCG debug search: rate limit error: %v", err)
		return
	}
	if !s.checkDailyLimit() {
		log.Printf("JustTCG debug search: quota exceeded")
		return
	}

	params := url.Values{}
	params.Set("game", "pokemon")
	params.Set("search", name)
	params.Set("limit", "50")

	reqURL := fmt.Sprintf("%s/cards?%s", s.baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		log.Printf("JustTCG debug search: request error: %v", err)
		return
	}
	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		log.Printf("JustTCG debug search: http error: %v", err)
		return
	}
	defer resp.Body.Close()

	var apiResp JustTCGResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		log.Printf("JustTCG debug search: decode error: %v", err)
		return
	}

	s.updateRemaining(apiResp.Metadata.APIDailyRequestsRemaining)

	log.Printf("JustTCG debug search for %q: found %d cards", name, len(apiResp.Data))
	for _, card := range apiResp.Data {
		// Log ALL results to see what's in the search
		log.Printf("  - %s | #%s | set: %s | tcgplayerId: %s", card.Name, card.Number, card.Set, card.TCGPlayerID)
	}
}

// FetchAllPokemonSets fetches the list of all Pokemon sets from JustTCG
func (s *JustTCGService) FetchAllPokemonSets() ([]string, error) {
	// Wait for rate limiter
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed: %w", err)
	}

	// Check daily limit
	if !s.checkDailyLimit() {
		return nil, fmt.Errorf("JustTCG daily rate limit exceeded")
	}

	params := url.Values{}
	params.Set("game", "pokemon")

	reqURL := fmt.Sprintf("%s/sets?%s", s.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch sets: %w", err)
	}
	defer resp.Body.Close()

	// Parse response - sets endpoint returns different structure
	var setsResp struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
		Metadata JustTCGUsage `json:"_metadata"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&setsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	s.updateRemaining(setsResp.Metadata.APIDailyRequestsRemaining)

	var setIDs []string
	for _, set := range setsResp.Data {
		setIDs = append(setIDs, set.ID)
	}

	log.Printf("JustTCG: fetched %d Pokemon sets", len(setIDs))
	return setIDs, nil
}
