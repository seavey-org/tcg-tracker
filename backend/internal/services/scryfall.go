package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
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

// GroupCardsBySet groups a flat list of cards by set for 2-phase selection
// Sorting: best match first (by OCR set code), then by release date (newest first)
// Also considers collector number match for determining best match
func GroupCardsBySet(cards []models.Card, ocrSetCode, ocrCardNumber string) *models.MTGGroupedResult {
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

	// Determine best match based on OCR data
	// Priority: exact set code match > collector number match within a set
	ocrSetCodeLower := strings.ToLower(ocrSetCode)
	for i := range groups {
		if strings.EqualFold(groups[i].SetCode, ocrSetCodeLower) {
			groups[i].IsBestMatch = true
		} else if ocrCardNumber != "" {
			// Check if any variant in this set has matching collector number
			for _, v := range groups[i].Variants {
				if v.CardNumber == ocrCardNumber {
					// Collector number match is a secondary signal but not as strong as set code
					// Only mark as best match if no set code was provided
					if ocrSetCode == "" {
						groups[i].IsBestMatch = true
					}
					break
				}
			}
		}
	}

	// Sort: best match first, then by release date (newest first)
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].IsBestMatch != groups[j].IsBestMatch {
			return groups[i].IsBestMatch // true sorts before false
		}
		// Compare release dates (format: "2022-02-18")
		// Newer dates sort first (descending order)
		return groups[i].ReleasedAt > groups[j].ReleasedAt
	})

	// Extract card name from first card
	cardName := cards[0].Name

	return &models.MTGGroupedResult{
		CardName:  cardName,
		SetGroups: groups,
		TotalSets: len(groups),
	}
}
