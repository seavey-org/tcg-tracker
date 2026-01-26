package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/codyseavey/tcg-tracker/backend/internal/models"
)

const scryfallBaseURL = "https://api.scryfall.com"

// scryfallSet represents a set from Scryfall's /sets endpoint
type scryfallSet struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	SetType    string `json:"set_type"`
	ReleasedAt string `json:"released_at"`
	CardCount  int    `json:"card_count"`
	ParentCode string `json:"parent_set_code"`
	IconSVGURI string `json:"icon_svg_uri"` // SVG icon URL for set symbol
}

type ScryfallService struct {
	client *http.Client

	// Cache for sets list (refreshed every 24 hours)
	setsCacheMu   sync.RWMutex
	setsCache     []scryfallSet
	setsCacheTime time.Time
}

// setsCacheTTL is how long the sets cache is valid (24 hours)
const setsCacheTTL = 24 * time.Hour

func NewScryfallService() *ScryfallService {
	return &ScryfallService{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type scryfallSearchResponse struct {
	Data       []scryfallCard `json:"data"`
	Object     string         `json:"object"`
	TotalCards int            `json:"total_cards"`
	HasMore    bool           `json:"has_more"`
}

type scryfallCard struct {
	ImageURIs    *scryfallImages `json:"image_uris"`
	CardFaces    []scryfallFace  `json:"card_faces"`
	Prices       scryfallPrices  `json:"prices"`
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	SetName      string          `json:"set_name"`
	Set          string          `json:"set"`
	CollectorNum string          `json:"collector_number"`
	Rarity       string          `json:"rarity"`
	// Variant info for 2-phase selection
	Finishes     []string `json:"finishes"`
	FrameEffects []string `json:"frame_effects"`
	PromoTypes   []string `json:"promo_types"`
	ReleasedAt   string   `json:"released_at"`
	// Additional fields for Gemini identification
	TypeLine    string `json:"type_line"`    // "Creature — Goblin Wizard"
	ManaCost    string `json:"mana_cost"`    // "{2}{R}{R}"
	BorderColor string `json:"border_color"` // "black", "borderless", "white"
	Artist      string `json:"artist"`       // Artist name
	// Additional fields for card details
	OracleText string `json:"oracle_text"` // Rules text
	FlavorText string `json:"flavor_text"` // Flavor text
	Power      string `json:"power"`       // Creature power
	Toughness  string `json:"toughness"`   // Creature toughness
	Loyalty    string `json:"loyalty"`     // Planeswalker loyalty
}

type scryfallImages struct {
	Small  string `json:"small"`
	Normal string `json:"normal"`
	Large  string `json:"large"`
}

type scryfallFace struct {
	ImageURIs *scryfallImages `json:"image_uris"`
}

type scryfallPrices struct {
	USD     string `json:"usd"`
	USDFoil string `json:"usd_foil"`
}

func (s *ScryfallService) SearchCards(query string) (*models.CardSearchResult, error) {
	encodedQuery := url.QueryEscape(query)
	reqURL := fmt.Sprintf("%s/cards/search?q=%s", scryfallBaseURL, encodedQuery)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search scryfall: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return &models.CardSearchResult{
			Cards:      []models.Card{},
			TotalCount: 0,
			HasMore:    false,
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("scryfall API returned status %d", resp.StatusCode)
	}

	var searchResp scryfallSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode scryfall response: %w", err)
	}

	cards := make([]models.Card, len(searchResp.Data))
	for i, sc := range searchResp.Data {
		cards[i] = s.convertToCard(sc)
	}

	return &models.CardSearchResult{
		Cards:      cards,
		TotalCount: searchResp.TotalCards,
		HasMore:    searchResp.HasMore,
	}, nil
}

// SearchCardsGrouped searches for cards by name and groups results by set.
// Returns a GroupedSearchResult with cards organized by set for 2-phase selection.
func (s *ScryfallService) SearchCardsGrouped(ctx context.Context, query string) (*models.GroupedSearchResult, error) {
	// Use SearchCardPrintings to get all printings of a card
	result, err := s.SearchCardPrintings(query)
	if err != nil || len(result.Cards) == 0 {
		// Fall back to regular search
		result, err = s.SearchCards(query)
		if err != nil {
			return nil, err
		}
	}

	if len(result.Cards) == 0 {
		return &models.GroupedSearchResult{
			CardName:  query,
			SetGroups: []models.SetGroup{},
			TotalSets: 0,
		}, nil
	}

	// Get sets cache for symbol URLs
	sets, _ := s.getSets(ctx)
	setIconMap := make(map[string]string)
	for _, set := range sets {
		setIconMap[set.Code] = set.IconSVGURI
	}

	// Group cards by set
	setMap := make(map[string]*models.SetGroup)
	for _, card := range result.Cards {
		group, exists := setMap[card.SetCode]
		if !exists {
			group = &models.SetGroup{
				SetCode:     card.SetCode,
				SetName:     card.SetName,
				ReleaseDate: card.ReleasedAt,
				SymbolURL:   setIconMap[card.SetCode],
				Cards:       []models.Card{},
			}
			setMap[card.SetCode] = group
		}
		group.Cards = append(group.Cards, card)
		group.CardCount = len(group.Cards)
	}

	// Convert map to slice
	groups := make([]models.SetGroup, 0, len(setMap))
	for _, g := range setMap {
		groups = append(groups, *g)
	}

	// Sort by release date (newest first)
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].ReleaseDate > groups[j].ReleaseDate
	})

	// Use the first card's name as the canonical name
	cardName := result.Cards[0].Name

	return &models.GroupedSearchResult{
		CardName:  cardName,
		SetGroups: groups,
		TotalSets: len(groups),
	}, nil
}

func (s *ScryfallService) GetCard(id string) (*models.Card, error) {
	reqURL := fmt.Sprintf("%s/cards/%s", scryfallBaseURL, id)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get card from scryfall: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("scryfall API returned status %d", resp.StatusCode)
	}

	var sc scryfallCard
	if err := json.NewDecoder(resp.Body).Decode(&sc); err != nil {
		return nil, fmt.Errorf("failed to decode scryfall response: %w", err)
	}

	card := s.convertToCard(sc)
	return &card, nil
}

func (s *ScryfallService) convertToCard(sc scryfallCard) models.Card {
	var imageURL, imageURLLarge string

	if sc.ImageURIs != nil {
		imageURL = sc.ImageURIs.Normal
		imageURLLarge = sc.ImageURIs.Large
	} else if len(sc.CardFaces) > 0 && sc.CardFaces[0].ImageURIs != nil {
		imageURL = sc.CardFaces[0].ImageURIs.Normal
		imageURLLarge = sc.CardFaces[0].ImageURIs.Large
	}

	var priceUSD, priceFoilUSD float64
	if sc.Prices.USD != "" {
		_, _ = fmt.Sscanf(sc.Prices.USD, "%f", &priceUSD)
	}
	if sc.Prices.USDFoil != "" {
		_, _ = fmt.Sscanf(sc.Prices.USDFoil, "%f", &priceFoilUSD)
	}

	now := time.Now()
	return models.Card{
		ID:             sc.ID,
		Game:           models.GameMTG,
		Name:           sc.Name,
		SetName:        sc.SetName,
		SetCode:        sc.Set,
		CardNumber:     sc.CollectorNum,
		Rarity:         sc.Rarity,
		ImageURL:       imageURL,
		ImageURLLarge:  imageURLLarge,
		PriceUSD:       priceUSD,
		PriceFoilUSD:   priceFoilUSD,
		PriceUpdatedAt: &now,
		// Variant info for 2-phase selection
		Finishes:     sc.Finishes,
		FrameEffects: sc.FrameEffects,
		PromoTypes:   sc.PromoTypes,
		ReleasedAt:   sc.ReleasedAt,
		// Gemini identification fields
		TypeLine:    sc.TypeLine,
		ManaCost:    sc.ManaCost,
		BorderColor: sc.BorderColor,
		Artist:      sc.Artist,
	}
}

// GetCardBySetAndNumber retrieves a specific card by set code and collector number
// Uses Scryfall's exact lookup: GET /cards/:set/:number
// Returns nil, nil if the card is not found (404)
func (s *ScryfallService) GetCardBySetAndNumber(setCode, number string) (*models.Card, error) {
	// Scryfall expects path params, so we must PathEscape.
	setEscaped := url.PathEscape(strings.ToLower(setCode))
	numberEscaped := url.PathEscape(number)
	reqURL := fmt.Sprintf("%s/cards/%s/%s", scryfallBaseURL, setEscaped, numberEscaped)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get card from scryfall: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("scryfall API returned status %d", resp.StatusCode)
	}

	var sc scryfallCard
	if err := json.NewDecoder(resp.Body).Decode(&sc); err != nil {
		return nil, fmt.Errorf("failed to decode scryfall response: %w", err)
	}

	card := s.convertToCard(sc)
	return &card, nil
}

// SearchCardPrintings searches for all printings of a card by exact name
// Uses Scryfall's unique:prints to get all versions across all sets
func (s *ScryfallService) SearchCardPrintings(cardName string) (*models.CardSearchResult, error) {
	// Use exact name match with unique:prints to get all printings
	// Escape quotes for Scryfall query syntax.
	safeName := strings.ReplaceAll(cardName, "\"", "\\\"")
	query := fmt.Sprintf(`!"%s" unique:prints`, safeName)
	encodedQuery := url.QueryEscape(query)
	reqURL := fmt.Sprintf("%s/cards/search?q=%s", scryfallBaseURL, encodedQuery)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search scryfall: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return &models.CardSearchResult{
			Cards:      []models.Card{},
			TotalCount: 0,
			HasMore:    false,
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("scryfall API returned status %d", resp.StatusCode)
	}

	var searchResp scryfallSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode scryfall response: %w", err)
	}

	cards := make([]models.Card, len(searchResp.Data))
	for i, sc := range searchResp.Data {
		cards[i] = s.convertToCard(sc)
	}

	return &models.CardSearchResult{
		Cards:      cards,
		TotalCount: searchResp.TotalCards,
		HasMore:    searchResp.HasMore,
	}, nil
}

// SearchByName implements CardSearcher interface for Gemini function calling.
// Searches for MTG cards by name and returns up to limit results.
func (s *ScryfallService) SearchByName(ctx context.Context, name string, limit int) ([]CandidateCard, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 20 {
		limit = 20
	}

	// First try exact name match with all printings
	result, err := s.SearchCardPrintings(name)
	if err != nil || len(result.Cards) == 0 {
		// Fall back to fuzzy search
		result, err = s.SearchCards(name)
		if err != nil {
			return nil, fmt.Errorf("failed to search cards: %w", err)
		}
	}

	// Sort by release date (newest first)
	sort.Slice(result.Cards, func(i, j int) bool {
		return result.Cards[i].ReleasedAt > result.Cards[j].ReleasedAt
	})

	// Convert to CandidateCard
	var candidates []CandidateCard
	for i := 0; i < len(result.Cards) && len(candidates) < limit; i++ {
		card := result.Cards[i]
		imageURL := card.ImageURLLarge
		if imageURL == "" {
			imageURL = card.ImageURL
		}
		if imageURL == "" {
			continue // Skip cards without images
		}

		candidates = append(candidates, CandidateCard{
			ID:       card.ID,
			Name:     card.Name,
			SetCode:  card.SetCode,
			SetName:  card.SetName,
			Number:   card.CardNumber,
			ImageURL: imageURL,
			// Enriched data for Gemini filtering
			Rarity:       card.Rarity,
			Artist:       card.Artist,
			ReleaseDate:  card.ReleasedAt,
			TypeLine:     card.TypeLine,
			ManaCost:     card.ManaCost,
			BorderColor:  card.BorderColor,
			FrameEffects: card.FrameEffects,
			PromoTypes:   card.PromoTypes,
		})
	}

	return candidates, nil
}

// GetBySetAndNumber implements CardSearcher interface for Gemini function calling.
// Gets a specific MTG card by set code and collector number.
func (s *ScryfallService) GetBySetAndNumber(ctx context.Context, setCode, number string) (*CandidateCard, error) {
	card, err := s.GetCardBySetAndNumber(setCode, number)
	if err != nil {
		return nil, err
	}
	if card == nil {
		return nil, nil
	}

	imageURL := card.ImageURLLarge
	if imageURL == "" {
		imageURL = card.ImageURL
	}

	return &CandidateCard{
		ID:       card.ID,
		Name:     card.Name,
		SetCode:  card.SetCode,
		SetName:  card.SetName,
		Number:   card.CardNumber,
		ImageURL: imageURL,
		// Enriched data for Gemini filtering
		Rarity:       card.Rarity,
		Artist:       card.Artist,
		ReleaseDate:  card.ReleasedAt,
		TypeLine:     card.TypeLine,
		ManaCost:     card.ManaCost,
		BorderColor:  card.BorderColor,
		FrameEffects: card.FrameEffects,
		PromoTypes:   card.PromoTypes,
	}, nil
}

// GetCardImage implements CardSearcher interface for Gemini function calling.
// Downloads a card image by ID and returns base64-encoded image data.
func (s *ScryfallService) GetCardImage(ctx context.Context, cardID string) (string, error) {
	// Get the card to find its image URL
	card, err := s.GetCard(cardID)
	if err != nil {
		return "", err
	}
	if card == nil {
		return "", fmt.Errorf("card not found: %s", cardID)
	}

	imageURL := card.ImageURLLarge
	if imageURL == "" {
		imageURL = card.ImageURL
	}
	if imageURL == "" {
		return "", fmt.Errorf("no image URL for card: %s", cardID)
	}

	// Download the image
	req, err := http.NewRequestWithContext(ctx, "GET", imageURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d fetching image", resp.StatusCode)
	}

	// Read and encode
	data, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

// GetCardDetails implements CardSearcher interface for Gemini function calling.
// Returns full card details including oracle text, power/toughness for verification.
func (s *ScryfallService) GetCardDetails(ctx context.Context, cardID string) (*CardDetails, error) {
	// Fetch the card from Scryfall by ID
	reqURL := fmt.Sprintf("%s/cards/%s", scryfallBaseURL, url.PathEscape(cardID))

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch card: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("card not found: %s", cardID)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("scryfall returned status %d", resp.StatusCode)
	}

	var sc scryfallCard
	if err := json.NewDecoder(resp.Body).Decode(&sc); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	imageURL := ""
	if sc.ImageURIs != nil {
		imageURL = sc.ImageURIs.Large
		if imageURL == "" {
			imageURL = sc.ImageURIs.Normal
		}
	}

	return &CardDetails{
		ID:         sc.ID,
		Name:       sc.Name,
		SetCode:    sc.Set,
		SetName:    sc.SetName,
		Number:     sc.CollectorNum,
		Rarity:     sc.Rarity,
		Artist:     sc.Artist,
		ImageURL:   imageURL,
		TypeLine:   sc.TypeLine,
		ManaCost:   sc.ManaCost,
		OracleText: sc.OracleText,
		Power:      sc.Power,
		Toughness:  sc.Toughness,
		Loyalty:    sc.Loyalty,
		FlavorText: sc.FlavorText,
	}, nil
}

// fetchAndCacheSets fetches all sets from Scryfall and caches them.
// Returns the cached sets (caller should hold no locks when calling).
func (s *ScryfallService) fetchAndCacheSets(ctx context.Context) ([]scryfallSet, error) {
	reqURL := fmt.Sprintf("%s/sets", scryfallBaseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch sets: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("scryfall returned status %d", resp.StatusCode)
	}

	var response struct {
		Data []scryfallSet `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Update cache
	s.setsCacheMu.Lock()
	s.setsCache = response.Data
	s.setsCacheTime = time.Now()
	s.setsCacheMu.Unlock()

	return response.Data, nil
}

// getSets returns cached sets if valid, otherwise fetches fresh data.
func (s *ScryfallService) getSets(ctx context.Context) ([]scryfallSet, error) {
	// Check cache first (read lock)
	s.setsCacheMu.RLock()
	if len(s.setsCache) > 0 && time.Since(s.setsCacheTime) < setsCacheTTL {
		sets := s.setsCache
		s.setsCacheMu.RUnlock()
		return sets, nil
	}
	s.setsCacheMu.RUnlock()

	// Cache miss or stale, fetch fresh data
	return s.fetchAndCacheSets(ctx)
}

// ListSets implements CardSearcher interface for Gemini function calling.
// Returns MTG sets matching the query. Results are cached for 24 hours.
func (s *ScryfallService) ListSets(ctx context.Context, query string) ([]SetInfo, error) {
	sets, err := s.getSets(ctx)
	if err != nil {
		return nil, err
	}

	queryLower := strings.ToLower(strings.TrimSpace(query))
	var results []SetInfo

	for _, set := range sets {
		nameLower := strings.ToLower(set.Name)
		codeLower := strings.ToLower(set.Code)
		typeLower := strings.ToLower(set.SetType)

		if strings.Contains(nameLower, queryLower) ||
			strings.Contains(codeLower, queryLower) ||
			strings.Contains(typeLower, queryLower) {
			results = append(results, SetInfo{
				ID:          set.Code,
				Name:        set.Name,
				Series:      set.SetType, // Use set_type as series for MTG
				ReleaseDate: set.ReleasedAt,
				TotalCards:  set.CardCount,
				SymbolURL:   set.IconSVGURI, // Scryfall provides SVG icons
			})
		}
	}

	// Sort by release date (newest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].ReleaseDate > results[j].ReleaseDate
	})

	// Limit to 20 results
	if len(results) > 20 {
		results = results[:20]
	}

	return results, nil
}

// ListAllSets returns all MTG sets with optional query filter.
// Unlike ListSets, this method has no result limit and is intended for browsing.
func (s *ScryfallService) ListAllSets(ctx context.Context, query string) ([]SetInfo, error) {
	sets, err := s.getSets(ctx)
	if err != nil {
		return nil, err
	}

	queryLower := strings.ToLower(strings.TrimSpace(query))
	var results []SetInfo

	for _, set := range sets {
		// If query is empty, include all sets
		if queryLower == "" {
			results = append(results, SetInfo{
				ID:          set.Code,
				Name:        set.Name,
				Series:      set.SetType,
				ReleaseDate: set.ReleasedAt,
				TotalCards:  set.CardCount,
				SymbolURL:   set.IconSVGURI,
			})
			continue
		}

		nameLower := strings.ToLower(set.Name)
		codeLower := strings.ToLower(set.Code)
		typeLower := strings.ToLower(set.SetType)

		if strings.Contains(nameLower, queryLower) ||
			strings.Contains(codeLower, queryLower) ||
			strings.Contains(typeLower, queryLower) {
			results = append(results, SetInfo{
				ID:          set.Code,
				Name:        set.Name,
				Series:      set.SetType,
				ReleaseDate: set.ReleasedAt,
				TotalCards:  set.CardCount,
				SymbolURL:   set.IconSVGURI,
			})
		}
	}

	// Sort by release date (newest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].ReleaseDate > results[j].ReleaseDate
	})

	return results, nil
}

// GetSetCards returns all cards in a specific MTG set, optionally filtered by name.
// Returns Card models for API responses.
func (s *ScryfallService) GetSetCards(setCode, nameFilter string) (*models.CardSearchResult, error) {
	// Build Scryfall query: set:xxx [name]
	query := fmt.Sprintf("set:%s", strings.ToLower(setCode))
	if nameFilter != "" {
		query += fmt.Sprintf(" %s", nameFilter)
	}

	result, err := s.SearchCards(query)
	if err != nil {
		return nil, fmt.Errorf("failed to search cards in set: %w", err)
	}

	// Sort by collector number
	sort.Slice(result.Cards, func(i, j int) bool {
		numI, _ := strconv.Atoi(strings.TrimLeft(result.Cards[i].CardNumber, "0"))
		numJ, _ := strconv.Atoi(strings.TrimLeft(result.Cards[j].CardNumber, "0"))
		return numI < numJ
	})

	return result, nil
}

// SearchInSet implements CardSearcher interface for Gemini function calling.
// Searches for cards within a specific MTG set, optionally filtered by name.
func (s *ScryfallService) SearchInSet(ctx context.Context, setCode, name string, limit int) ([]CandidateCard, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	// Build Scryfall query: set:xxx name:yyy
	query := fmt.Sprintf("set:%s", strings.ToLower(setCode))
	if name != "" {
		query += fmt.Sprintf(" %s", name)
	}

	result, err := s.SearchCards(query)
	if err != nil {
		return nil, fmt.Errorf("failed to search cards in set: %w", err)
	}

	// Sort by collector number
	sort.Slice(result.Cards, func(i, j int) bool {
		numI, _ := strconv.Atoi(strings.TrimLeft(result.Cards[i].CardNumber, "0"))
		numJ, _ := strconv.Atoi(strings.TrimLeft(result.Cards[j].CardNumber, "0"))
		return numI < numJ
	})

	// Convert to CandidateCard
	var candidates []CandidateCard
	for i := 0; i < len(result.Cards) && len(candidates) < limit; i++ {
		card := result.Cards[i]
		imageURL := card.ImageURLLarge
		if imageURL == "" {
			imageURL = card.ImageURL
		}
		if imageURL == "" {
			continue
		}

		candidates = append(candidates, CandidateCard{
			ID:           card.ID,
			Name:         card.Name,
			SetCode:      card.SetCode,
			SetName:      card.SetName,
			Number:       card.CardNumber,
			ImageURL:     imageURL,
			Rarity:       card.Rarity,
			Artist:       card.Artist,
			ReleaseDate:  card.ReleasedAt,
			TypeLine:     card.TypeLine,
			ManaCost:     card.ManaCost,
			BorderColor:  card.BorderColor,
			FrameEffects: card.FrameEffects,
			PromoTypes:   card.PromoTypes,
		})
	}

	return candidates, nil
}

// GetSetInfo implements CardSearcher interface for Gemini function calling.
// Returns detailed information about a specific MTG set.
func (s *ScryfallService) GetSetInfo(ctx context.Context, setCode string) (*SetDetails, error) {
	reqURL := fmt.Sprintf("%s/sets/%s", scryfallBaseURL, url.PathEscape(strings.ToLower(setCode)))

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch set: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("set not found: %s", setCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("scryfall returned status %d", resp.StatusCode)
	}

	var setData struct {
		Code       string `json:"code"`
		Name       string `json:"name"`
		SetType    string `json:"set_type"`
		ReleasedAt string `json:"released_at"`
		CardCount  int    `json:"card_count"`
		IconSVGURI string `json:"icon_svg_uri"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&setData); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Generate symbol description based on set type
	symbolDesc := generateMTGSymbolDescription(setData.Name, setData.SetType)

	return &SetDetails{
		ID:                setData.Code,
		Name:              setData.Name,
		ReleaseDate:       setData.ReleasedAt,
		TotalCards:        setData.CardCount,
		SetType:           setData.SetType,
		SymbolDescription: symbolDesc,
	}, nil
}

// generateMTGSymbolDescription creates a description of MTG set symbols
func generateMTGSymbolDescription(name, setType string) string {
	// Common set type descriptions
	typeDescriptions := map[string]string{
		"expansion":        "Standard expansion symbol",
		"core":             "Core set symbol (M-series)",
		"masters":          "Masters series premium symbol",
		"draft_innovation": "Special draft format symbol",
		"commander":        "Commander deck symbol",
		"promo":            "Promotional card symbol",
		"funny":            "Un-set style silver-bordered symbol",
	}

	if desc, ok := typeDescriptions[setType]; ok {
		return desc
	}
	return fmt.Sprintf("%s set symbol", setType)
}

// GroupCardsBySet groups a flat list of cards by set for 2-phase selection
// Uses confidence scoring based on multiple OCR signals:
//   - Set code match: +100 points
//   - Collector number match: +50 points
//   - Set total match (exact): +30 points
//   - Copyright year match: +20 points
//
// Results are sorted by score descending, then release date (newest first)
func GroupCardsBySet(cards []models.Card, ocrSetCode, ocrCardNumber, ocrSetTotal, ocrYear string) *models.MTGGroupedResult {
	if len(cards) == 0 {
		return &models.MTGGroupedResult{
			CardName:  "",
			SetGroups: []models.MTGSetGroup{},
			TotalSets: 0,
		}
	}

	// Group cards by SetCode
	setMap := make(map[string]*models.MTGSetGroup)

	for _, card := range cards {
		group, exists := setMap[card.SetCode]
		if !exists {
			group = &models.MTGSetGroup{
				SetCode:    card.SetCode,
				SetName:    card.SetName,
				ReleasedAt: card.ReleasedAt,
				Variants:   []models.Card{},
			}
			setMap[card.SetCode] = group
		}
		group.Variants = append(group.Variants, card)
	}

	// Convert map to slice
	groups := make([]models.MTGSetGroup, 0, len(setMap))
	for _, g := range setMap {
		groups = append(groups, *g)
	}

	// Calculate confidence score for each set group
	// Scoring:
	//   +100: Set code exact match
	//   +50:  Collector number exists in set
	//   +30:  Max collector number matches OCR set total exactly
	//   +20:  Release year matches OCR copyright year
	ocrSetCodeLower := strings.ToLower(ocrSetCode)
	ocrTotalInt, _ := strconv.Atoi(ocrSetTotal)

	for i := range groups {
		score := 0

		// Set code match (+100)
		if ocrSetCode != "" && strings.EqualFold(groups[i].SetCode, ocrSetCodeLower) {
			score += 100
		}

		// Check variants for collector number match and find max collector number
		maxCollectorNum := 0
		hasCollectorMatch := false
		for _, v := range groups[i].Variants {
			// Collector number match (+50)
			if ocrCardNumber != "" && v.CardNumber == ocrCardNumber {
				hasCollectorMatch = true
			}
			// Parse collector number to find max (for set total matching)
			// Only consider numeric collector numbers (skip things like "A1", "P1")
			if num, err := strconv.Atoi(v.CardNumber); err == nil && num > maxCollectorNum {
				maxCollectorNum = num
			}
		}
		if hasCollectorMatch {
			score += 50
		}

		// Set total match (+30) - exact match only
		if ocrTotalInt > 0 && maxCollectorNum > 0 && ocrTotalInt == maxCollectorNum {
			score += 30
		}

		// Copyright year match (+20)
		if ocrYear != "" && len(groups[i].ReleasedAt) >= 4 {
			releaseYear := groups[i].ReleasedAt[:4] // "2022-02-18" -> "2022"
			if ocrYear == releaseYear {
				score += 20
			}
		}

		groups[i].MatchScore = score
	}

	// Sort by score descending, then release date descending (newest first)
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].MatchScore != groups[j].MatchScore {
			return groups[i].MatchScore > groups[j].MatchScore
		}
		return groups[i].ReleasedAt > groups[j].ReleasedAt
	})

	// Mark highest score as best match (only if score > 0)
	if len(groups) > 0 && groups[0].MatchScore > 0 {
		groups[0].IsBestMatch = true
	}

	// Extract card name from first card
	cardName := cards[0].Name

	return &models.MTGGroupedResult{
		CardName:  cardName,
		SetGroups: groups,
		TotalSets: len(groups),
	}
}
