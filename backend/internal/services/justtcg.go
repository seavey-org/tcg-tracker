package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/codyseavey/tcg-tracker/backend/internal/models"
)

const (
	justTCGBaseURL        = "https://api.justtcg.com/v1"
	justTCGDefaultTimeout = 10 * time.Second
)

// JustTCGService handles API calls to JustTCG for card pricing
type JustTCGService struct {
	client     *http.Client
	apiKey     string
	baseURL    string
	dailyLimit int

	// Rate limiting
	mu             sync.Mutex
	requestsToday  int
	lastRequestDay time.Time
}

// JustTCGPriceResponse represents the API response for price queries
type JustTCGPriceResponse struct {
	Success bool            `json:"success"`
	Data    JustTCGCardData `json:"data"`
	Error   string          `json:"error,omitempty"`
}

// JustTCGCardData contains card information including prices
type JustTCGCardData struct {
	CardName   string         `json:"card_name"`
	SetName    string         `json:"set_name"`
	SetCode    string         `json:"set_code"`
	CardNumber string         `json:"card_number"`
	Prices     []JustTCGPrice `json:"prices"`
}

// JustTCGPrice represents a single condition/foil price entry
type JustTCGPrice struct {
	Condition string  `json:"condition"` // NM, LP, MP, HP, DMG
	Foil      bool    `json:"foil"`
	PriceUSD  float64 `json:"price_usd"`
	LastSeen  string  `json:"last_seen,omitempty"`
}

// JustTCGSearchResponse represents the API response for search queries
type JustTCGSearchResponse struct {
	Success bool              `json:"success"`
	Data    []JustTCGCardData `json:"data"`
	Error   string            `json:"error,omitempty"`
}

// NewJustTCGService creates a new JustTCG API service
func NewJustTCGService(apiKey string, dailyLimit int) *JustTCGService {
	if dailyLimit <= 0 {
		dailyLimit = 100 // Default free tier limit
	}

	return &JustTCGService{
		client: &http.Client{
			Timeout: justTCGDefaultTimeout,
		},
		apiKey:     apiKey,
		baseURL:    justTCGBaseURL,
		dailyLimit: dailyLimit,
	}
}

// checkRateLimit checks if we can make another request today
// Returns true if request can proceed, false if rate limited
func (s *JustTCGService) checkRateLimit() bool {
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

// GetCardPrices fetches condition-specific prices for a card
func (s *JustTCGService) GetCardPrices(cardName, setCode string, game models.Game) ([]models.CardPrice, error) {
	if !s.checkRateLimit() {
		return nil, fmt.Errorf("JustTCG daily rate limit exceeded")
	}

	// Build request URL
	gameStr := "pokemon"
	if game == models.GameMTG {
		gameStr = "mtg"
	}

	params := url.Values{}
	params.Set("name", cardName)
	params.Set("game", gameStr)
	if setCode != "" {
		params.Set("set", setCode)
	}

	reqURL := fmt.Sprintf("%s/cards/price?%s", s.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(context.Background(), "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add API key if provided
	if s.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.apiKey)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch prices: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JustTCG API error: status %d", resp.StatusCode)
	}

	var priceResp JustTCGPriceResponse
	if err := json.NewDecoder(resp.Body).Decode(&priceResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !priceResp.Success {
		if priceResp.Error != "" {
			return nil, fmt.Errorf("JustTCG API error: %s", priceResp.Error)
		}
		return nil, fmt.Errorf("JustTCG API returned unsuccessful response")
	}

	// Convert to CardPrice models
	now := time.Now()
	var prices []models.CardPrice
	for _, p := range priceResp.Data.Prices {
		condition := mapJustTCGCondition(p.Condition)
		if condition == "" {
			continue
		}

		prices = append(prices, models.CardPrice{
			Condition:      condition,
			Foil:           p.Foil,
			PriceUSD:       p.PriceUSD,
			Source:         "justtcg",
			PriceUpdatedAt: &now,
		})
	}

	return prices, nil
}

// SearchCards searches for cards by name and returns results with prices
func (s *JustTCGService) SearchCards(query string, game models.Game) ([]JustTCGCardData, error) {
	if !s.checkRateLimit() {
		return nil, fmt.Errorf("JustTCG daily rate limit exceeded")
	}

	gameStr := "pokemon"
	if game == models.GameMTG {
		gameStr = "mtg"
	}

	params := url.Values{}
	params.Set("q", query)
	params.Set("game", gameStr)

	reqURL := fmt.Sprintf("%s/cards/search?%s", s.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(context.Background(), "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if s.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.apiKey)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search cards: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JustTCG API error: status %d", resp.StatusCode)
	}

	var searchResp JustTCGSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !searchResp.Success {
		if searchResp.Error != "" {
			return nil, fmt.Errorf("JustTCG API error: %s", searchResp.Error)
		}
		return nil, fmt.Errorf("JustTCG API returned unsuccessful response")
	}

	return searchResp.Data, nil
}

// mapJustTCGCondition maps JustTCG condition strings to our PriceCondition type
func mapJustTCGCondition(condition string) models.PriceCondition {
	switch strings.ToUpper(condition) {
	case "NM", "NEAR MINT":
		return models.PriceConditionNM
	case "LP", "LIGHTLY PLAYED":
		return models.PriceConditionLP
	case "MP", "MODERATELY PLAYED":
		return models.PriceConditionMP
	case "HP", "HEAVILY PLAYED":
		return models.PriceConditionHP
	case "DMG", "DAMAGED":
		return models.PriceConditionDMG
	default:
		return ""
	}
}
