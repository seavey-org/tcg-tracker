package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/codyseavey/tcg-tracker/backend/internal/models"
)

const pokemonTCGBaseURL = "https://api.pokemontcg.io/v2"

type PokemonTCGService struct {
	client *http.Client
	apiKey string
}

func NewPokemonTCGService(apiKey string) *PokemonTCGService {
	return &PokemonTCGService{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiKey: apiKey,
	}
}

type pokemonSearchResponse struct {
	Data       []pokemonCard `json:"data"`
	TotalCount int           `json:"totalCount"`
	Page       int           `json:"page"`
	PageSize   int           `json:"pageSize"`
	Count      int           `json:"count"`
}

type pokemonCard struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Set       pokemonSet       `json:"set"`
	Number    string           `json:"number"`
	Rarity    string           `json:"rarity"`
	Images    pokemonImages    `json:"images"`
	TCGPlayer *pokemonTCGPrice `json:"tcgplayer"`
}

type pokemonSet struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type pokemonImages struct {
	Small string `json:"small"`
	Large string `json:"large"`
}

type pokemonTCGPrice struct {
	URL       string                     `json:"url"`
	UpdatedAt string                     `json:"updatedAt"`
	Prices    map[string]pokemonPriceSet `json:"prices"`
}

type pokemonPriceSet struct {
	Low    float64 `json:"low"`
	Mid    float64 `json:"mid"`
	High   float64 `json:"high"`
	Market float64 `json:"market"`
}

func (s *PokemonTCGService) SearchCards(query string) (*models.CardSearchResult, error) {
	encodedQuery := url.QueryEscape(fmt.Sprintf("name:%s*", query))
	reqURL := fmt.Sprintf("%s/cards?q=%s", pokemonTCGBaseURL, encodedQuery)

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if s.apiKey != "" {
		req.Header.Set("X-Api-Key", s.apiKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search pokemon tcg: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pokemon tcg API returned status %d", resp.StatusCode)
	}

	var searchResp pokemonSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode pokemon tcg response: %w", err)
	}

	cards := make([]models.Card, len(searchResp.Data))
	for i, pc := range searchResp.Data {
		cards[i] = s.convertToCard(pc)
	}

	return &models.CardSearchResult{
		Cards:      cards,
		TotalCount: searchResp.TotalCount,
		HasMore:    searchResp.TotalCount > searchResp.Page*searchResp.PageSize,
	}, nil
}

func (s *PokemonTCGService) GetCard(id string) (*models.Card, error) {
	reqURL := fmt.Sprintf("%s/cards/%s", pokemonTCGBaseURL, id)

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if s.apiKey != "" {
		req.Header.Set("X-Api-Key", s.apiKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get card from pokemon tcg: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pokemon tcg API returned status %d", resp.StatusCode)
	}

	var response struct {
		Data pokemonCard `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode pokemon tcg response: %w", err)
	}

	card := s.convertToCard(response.Data)
	return &card, nil
}

func (s *PokemonTCGService) convertToCard(pc pokemonCard) models.Card {
	var priceUSD, priceFoilUSD float64

	if pc.TCGPlayer != nil && pc.TCGPlayer.Prices != nil {
		if normal, ok := pc.TCGPlayer.Prices["normal"]; ok {
			priceUSD = normal.Market
		}
		if holofoil, ok := pc.TCGPlayer.Prices["holofoil"]; ok {
			priceFoilUSD = holofoil.Market
		} else if reverseHolo, ok := pc.TCGPlayer.Prices["reverseHolofoil"]; ok {
			priceFoilUSD = reverseHolo.Market
		}
	}

	now := time.Now()
	return models.Card{
		ID:             pc.ID,
		Game:           models.GamePokemon,
		Name:           pc.Name,
		SetName:        pc.Set.Name,
		SetCode:        pc.Set.ID,
		CardNumber:     pc.Number,
		Rarity:         pc.Rarity,
		ImageURL:       pc.Images.Small,
		ImageURLLarge:  pc.Images.Large,
		PriceUSD:       priceUSD,
		PriceFoilUSD:   priceFoilUSD,
		PriceUpdatedAt: &now,
	}
}
