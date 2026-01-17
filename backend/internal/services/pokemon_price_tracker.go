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

const pokemonPriceTrackerBaseURL = "https://www.pokemonpricetracker.com/api/v2"

type PokemonPriceTrackerService struct {
	client *http.Client
	apiKey string
}

func NewPokemonPriceTrackerService(apiKey string) *PokemonPriceTrackerService {
	return &PokemonPriceTrackerService{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiKey: apiKey,
	}
}

type pptSearchResponse struct {
	Data     []pptCard   `json:"data"`
	Metadata pptMetadata `json:"metadata"`
}

type pptMetadata struct {
	Total   int  `json:"total"`
	Count   int  `json:"count"`
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	HasMore bool `json:"hasMore"`
}

type pptCard struct {
	Prices      pptPrices `json:"prices"`
	ID          string    `json:"id"`
	TCGPlayerID string    `json:"tcgPlayerId"`
	Name        string    `json:"name"`
	SetName     string    `json:"setName"`
	SetID       string    `json:"setId"`
	CardNumber  string    `json:"cardNumber"`
	Rarity      string    `json:"rarity"`
	ImageURL    string    `json:"imageUrl"`
	ImageCdnUrl string    `json:"imageCdnUrl"`
}

type pptPrices struct {
	Variants map[string]map[string]any `json:"variants"`
	Market   float64                   `json:"market"`
}

func (s *PokemonPriceTrackerService) SearchCards(query string) (*models.CardSearchResult, error) {
	return s.SearchCardsWithTimeout(query, 30*time.Second)
}

// SearchCardsWithTimeout searches for cards with a custom timeout
func (s *PokemonPriceTrackerService) SearchCardsWithTimeout(query string, timeout time.Duration) (*models.CardSearchResult, error) {
	reqURL := fmt.Sprintf("%s/cards?search=%s&limit=20", pokemonPriceTrackerBaseURL, url.QueryEscape(query))

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search pokemon price tracker: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pokemon price tracker API returned status %d", resp.StatusCode)
	}

	var searchResp pptSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	cards := make([]models.Card, len(searchResp.Data))
	for i, pc := range searchResp.Data {
		cards[i] = s.convertToCard(pc)
	}

	return &models.CardSearchResult{
		Cards:      cards,
		TotalCount: searchResp.Metadata.Total,
		HasMore:    searchResp.Metadata.HasMore,
	}, nil
}

func (s *PokemonPriceTrackerService) GetCard(id string) (*models.Card, error) {
	// PokemonPriceTracker uses tcgPlayerId for lookups
	reqURL := fmt.Sprintf("%s/cards?tcgPlayerId=%s", pokemonPriceTrackerBaseURL, id)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get card: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pokemon price tracker API returned status %d", resp.StatusCode)
	}

	var searchResp pptSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(searchResp.Data) == 0 {
		return nil, nil
	}

	card := s.convertToCard(searchResp.Data[0])
	return &card, nil
}

func (s *PokemonPriceTrackerService) convertToCard(pc pptCard) models.Card {
	now := time.Now()

	// Use the best available image
	imageURL := pc.ImageURL
	if pc.ImageCdnUrl != "" {
		imageURL = pc.ImageCdnUrl
	}

	// Get foil price from variants if available
	var foilPrice float64
	if pc.Prices.Variants != nil {
		if holofoil, ok := pc.Prices.Variants["Holofoil"]; ok {
			if nm, ok := holofoil["Near Mint"]; ok {
				if nmMap, ok := nm.(map[string]any); ok {
					if price, ok := nmMap["price"].(float64); ok {
						foilPrice = price
					}
				}
			}
		}
	}

	return models.Card{
		ID:             pc.TCGPlayerID,
		Game:           models.GamePokemon,
		Name:           pc.Name,
		SetName:        pc.SetName,
		SetCode:        pc.SetID,
		CardNumber:     pc.CardNumber,
		Rarity:         pc.Rarity,
		ImageURL:       imageURL,
		ImageURLLarge:  pc.ImageCdnUrl,
		PriceUSD:       pc.Prices.Market,
		PriceFoilUSD:   foilPrice,
		PriceUpdatedAt: &now,
	}
}
