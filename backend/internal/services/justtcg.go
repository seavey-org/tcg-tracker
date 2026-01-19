package services

import (
	"bytes"
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
	justTCGBatchSize = 20 // Max cards per request
	justTCGRateLimit = 10 // Requests per minute
)

// JustTCGService handles API calls to JustTCG for card pricing
type JustTCGService struct {
	client  *http.Client
	apiKey  string
	baseURL string

	// Rate limiting (10 requests per minute)
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
	Name        string `json:"q,omitempty"`
	Set         string `json:"set,omitempty"`
	Game        string `json:"game,omitempty"`
}

// NewJustTCGService creates a new JustTCG API service
func NewJustTCGService(apiKey string, dailyLimit int) *JustTCGService {
	if dailyLimit <= 0 {
		dailyLimit = 100 // Default free tier limit
	}

	// Rate limiter: 10 requests per minute = 1 request every 6 seconds
	limiter := rate.NewLimiter(rate.Every(6*time.Second), 1)

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

// GetCardPrices fetches condition-specific prices for a single card
// This is a convenience wrapper around BatchGetPrices for backward compatibility
func (s *JustTCGService) GetCardPrices(cardName, setCode string, game models.Game) ([]models.CardPrice, error) {
	gameStr := "pokemon"
	if game == models.GameMTG {
		gameStr = "magic-the-gathering"
	}

	lookup := CardLookup{
		CardID: "", // Will be set by caller
		Name:   cardName,
		Set:    setCode,
		Game:   gameStr,
	}

	results, err := s.BatchGetPrices([]CardLookup{lookup})
	if err != nil {
		return nil, err
	}

	// Return the first result (there should only be one)
	for _, prices := range results {
		return prices, nil
	}

	return nil, nil
}

// BatchGetPrices fetches prices for multiple cards in a single API call (max 20)
func (s *JustTCGService) BatchGetPrices(lookups []CardLookup) (map[string][]models.CardPrice, error) {
	if len(lookups) == 0 {
		return nil, nil
	}
	if len(lookups) > justTCGBatchSize {
		return nil, fmt.Errorf("batch size %d exceeds max %d", len(lookups), justTCGBatchSize)
	}

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

	// For single card lookup, use GET with query params
	if len(lookups) == 1 {
		return s.getSingleCard(lookups[0])
	}

	// For multiple cards, use POST batch endpoint
	return s.postBatchCards(lookups)
}

// getSingleCard fetches a single card using GET request
func (s *JustTCGService) getSingleCard(lookup CardLookup) (map[string][]models.CardPrice, error) {
	params := url.Values{}
	if lookup.Name != "" {
		params.Set("q", lookup.Name)
	}
	if lookup.Game != "" {
		params.Set("game", lookup.Game)
	}
	if lookup.Set != "" {
		params.Set("set", lookup.Set)
	}
	// Request all conditions but limit stats to reduce response size
	params.Set("include_price_history", "false")
	params.Set("include_statistics", "")

	reqURL := fmt.Sprintf("%s/cards?%s", s.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(context.Background(), "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch prices: %w", err)
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

	// Convert response to CardPrice map
	results := make(map[string][]models.CardPrice)
	for _, card := range apiResp.Data {
		prices := s.convertVariantsToPrices(card.Variants)
		if len(prices) > 0 {
			// Use the card name as key since we don't have our internal ID
			key := card.Name
			if lookup.CardID != "" {
				key = lookup.CardID
			}
			results[key] = prices
		}
	}

	s.updateRemaining(apiResp.Metadata.APIDailyRequestsRemaining)

	log.Printf("JustTCG: fetched %d cards, %d with prices (remaining: %d daily)",
		len(apiResp.Data), len(results), apiResp.Metadata.APIDailyRequestsRemaining)

	return results, nil
}

// postBatchCards fetches multiple cards using POST request
func (s *JustTCGService) postBatchCards(lookups []CardLookup) (map[string][]models.CardPrice, error) {
	// Build request body
	requestBody := make([]map[string]interface{}, len(lookups))
	for i, lookup := range lookups {
		item := make(map[string]interface{})
		if lookup.TCGPlayerID != "" {
			item["tcgplayerId"] = lookup.TCGPlayerID
		}
		if lookup.Name != "" {
			item["q"] = lookup.Name
		}
		if lookup.Game != "" {
			item["game"] = lookup.Game
		}
		if lookup.Set != "" {
			item["set"] = lookup.Set
		}
		requestBody[i] = item
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	reqURL := fmt.Sprintf("%s/cards?include_price_history=false&include_statistics=", s.baseURL)
	req, err := http.NewRequestWithContext(context.Background(), "POST", reqURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch prices: %w", err)
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

	// Convert response to CardPrice map
	// Match results back to lookups by name+set (NOT by index - API may return in different order)
	results := make(map[string][]models.CardPrice)

	// Build lookup index by name+set for proper matching
	lookupByNameSet := make(map[string]CardLookup)
	lookupByName := make(map[string]CardLookup)
	for _, lookup := range lookups {
		key := strings.ToLower(lookup.Name + "|" + lookup.Set)
		lookupByNameSet[key] = lookup
		// Also index by name alone as fallback
		lookupByName[strings.ToLower(lookup.Name)] = lookup
	}

	for _, card := range apiResp.Data {
		prices := s.convertVariantsToPrices(card.Variants)
		if len(prices) == 0 {
			continue
		}

		// Try to match by name+set first (most reliable)
		key := strings.ToLower(card.Name + "|" + card.Set)
		if lookup, ok := lookupByNameSet[key]; ok && lookup.CardID != "" {
			results[lookup.CardID] = prices
			continue
		}

		// Fallback to name-only match
		if lookup, ok := lookupByName[strings.ToLower(card.Name)]; ok && lookup.CardID != "" {
			results[lookup.CardID] = prices
			continue
		}

		// Last resort: use the card name as key
		results[card.Name] = prices
	}

	s.updateRemaining(apiResp.Metadata.APIDailyRequestsRemaining)

	log.Printf("JustTCG: batch fetched %d cards, %d with prices (remaining: %d daily)",
		len(apiResp.Data), len(results), apiResp.Metadata.APIDailyRequestsRemaining)

	return results, nil
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
func (s *JustTCGService) convertVariantsToPrices(variants []JustTCGVariant) []models.CardPrice {
	var prices []models.CardPrice
	now := time.Now()

	for _, v := range variants {
		condition := mapJustTCGCondition(v.Condition)
		printing := mapJustTCGPrinting(v.Printing)

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
