package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/codyseavey/tcg-tracker/backend/internal/models"
)

const scryfallBaseURL = "https://api.scryfall.com"

type ScryfallService struct {
	client *http.Client
}

func NewScryfallService() *ScryfallService {
	return &ScryfallService{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type scryfallSearchResponse struct {
	Object     string         `json:"object"`
	TotalCards int            `json:"total_cards"`
	HasMore    bool           `json:"has_more"`
	Data       []scryfallCard `json:"data"`
}

type scryfallCard struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	SetName       string            `json:"set_name"`
	Set           string            `json:"set"`
	CollectorNum  string            `json:"collector_number"`
	Rarity        string            `json:"rarity"`
	ImageURIs     *scryfallImages   `json:"image_uris"`
	CardFaces     []scryfallFace    `json:"card_faces"`
	Prices        scryfallPrices    `json:"prices"`
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

	resp, err := s.client.Get(reqURL)
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

func (s *ScryfallService) GetCard(id string) (*models.Card, error) {
	reqURL := fmt.Sprintf("%s/cards/%s", scryfallBaseURL, id)

	resp, err := s.client.Get(reqURL)
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
		fmt.Sscanf(sc.Prices.USD, "%f", &priceUSD)
	}
	if sc.Prices.USDFoil != "" {
		fmt.Sscanf(sc.Prices.USDFoil, "%f", &priceFoilUSD)
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
	}
}
