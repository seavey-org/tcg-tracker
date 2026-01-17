package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/codyseavey/tcg-tracker/backend/internal/models"
)

const tcgdexBaseURL = "https://api.tcgdex.net/v2/en"

type TCGdexService struct {
	client *http.Client
}

func NewTCGdexService() *TCGdexService {
	return &TCGdexService{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type tcgdexCard struct {
	ID       string        `json:"id"`
	LocalID  string        `json:"localId"`
	Name     string        `json:"name"`
	Rarity   string        `json:"rarity"`
	Image    string        `json:"image"`
	Set      tcgdexSet     `json:"set"`
	Pricing  *tcgdexPrices `json:"pricing"`
}

type tcgdexSet struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type tcgdexPrices struct {
	TCGPlayer *tcgdexTCGPlayer `json:"tcgplayer"`
}

type tcgdexTCGPlayer struct {
	Updated  string              `json:"updated"`
	Unit     string              `json:"unit"`
	Normal   *tcgdexPriceVariant `json:"normal"`
	Holofoil *tcgdexPriceVariant `json:"holofoil"`
	Reverse  *tcgdexPriceVariant `json:"reverse-holofoil"`
}

type tcgdexPriceVariant struct {
	LowPrice    float64 `json:"lowPrice"`
	MidPrice    float64 `json:"midPrice"`
	HighPrice   float64 `json:"highPrice"`
	MarketPrice float64 `json:"marketPrice"`
}

type tcgdexSearchResult struct {
	ID      string `json:"id"`
	LocalID string `json:"localId"`
	Name    string `json:"name"`
	Image   string `json:"image"`
}

// GetCard fetches a single card with pricing data from TCGdex
func (s *TCGdexService) GetCard(id string) (*models.Card, error) {
	reqURL := fmt.Sprintf("%s/cards/%s", tcgdexBaseURL, id)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get card from tcgdex: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tcgdex API returned status %d", resp.StatusCode)
	}

	var card tcgdexCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return s.convertToCard(card), nil
}

// SearchCards searches for cards by name using TCGdex
func (s *TCGdexService) SearchCards(query string) (*models.CardSearchResult, error) {
	// TCGdex search endpoint
	reqURL := fmt.Sprintf("%s/cards?name=%s", tcgdexBaseURL, url.QueryEscape(query))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search tcgdex: %w", err)
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
		return nil, fmt.Errorf("tcgdex API returned status %d", resp.StatusCode)
	}

	var searchResults []tcgdexSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&searchResults); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Limit results and fetch full card data for each (to get prices)
	maxResults := 20
	if len(searchResults) > maxResults {
		searchResults = searchResults[:maxResults]
	}

	cards := make([]models.Card, 0, len(searchResults))
	for _, sr := range searchResults {
		card, err := s.GetCard(sr.ID)
		if err != nil || card == nil {
			// Skip cards that fail to fetch
			continue
		}
		cards = append(cards, *card)
	}

	return &models.CardSearchResult{
		Cards:      cards,
		TotalCount: len(searchResults),
		HasMore:    len(searchResults) >= maxResults,
	}, nil
}

// GetCardPrice fetches only the pricing data for a card
func (s *TCGdexService) GetCardPrice(id string) (priceUSD, priceFoilUSD float64, err error) {
	card, err := s.GetCard(id)
	if err != nil {
		return 0, 0, err
	}
	if card == nil {
		return 0, 0, nil
	}
	return card.PriceUSD, card.PriceFoilUSD, nil
}

func (s *TCGdexService) convertToCard(tc tcgdexCard) *models.Card {
	now := time.Now()

	var priceUSD, priceFoilUSD float64
	var priceSource string = "pending"

	if tc.Pricing != nil && tc.Pricing.TCGPlayer != nil {
		priceSource = "tcgdex"

		// Get normal price
		if tc.Pricing.TCGPlayer.Normal != nil {
			priceUSD = tc.Pricing.TCGPlayer.Normal.MarketPrice
		}

		// Get holofoil price
		if tc.Pricing.TCGPlayer.Holofoil != nil {
			priceFoilUSD = tc.Pricing.TCGPlayer.Holofoil.MarketPrice
			// If no normal price, use holofoil as the base price
			if priceUSD == 0 {
				priceUSD = priceFoilUSD
			}
		}

		// Try reverse holo if no other prices
		if priceUSD == 0 && tc.Pricing.TCGPlayer.Reverse != nil {
			priceUSD = tc.Pricing.TCGPlayer.Reverse.MarketPrice
		}
	}

	// Build image URL (TCGdex provides base URL, we add quality suffix)
	imageURL := tc.Image
	imageURLLarge := tc.Image
	if imageURL != "" {
		imageURL = imageURL + "/low.webp"
		imageURLLarge = imageURLLarge + "/high.webp"
	}

	card := &models.Card{
		ID:            tc.ID,
		Game:          models.GamePokemon,
		Name:          tc.Name,
		SetName:       tc.Set.Name,
		SetCode:       tc.Set.ID,
		CardNumber:    tc.LocalID,
		Rarity:        tc.Rarity,
		ImageURL:      imageURL,
		ImageURLLarge: imageURLLarge,
		PriceUSD:      priceUSD,
		PriceFoilUSD:  priceFoilUSD,
		PriceSource:   priceSource,
	}

	if priceUSD > 0 || priceFoilUSD > 0 {
		card.PriceUpdatedAt = &now
	}

	return card
}
