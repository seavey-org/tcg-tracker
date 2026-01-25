package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/codyseavey/tcg-tracker/backend/internal/metrics"
)

const (
	geminiModel          = "gemini-2.0-flash"
	geminiAPIURL         = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent"
	geminiTimeout        = 60 * time.Second // Longer timeout for multi-turn
	imageDownloadTimeout = 10 * time.Second
	maxTurns             = 10 // Max conversation turns before giving up
)

// GeminiService handles card identification via Gemini Vision API with function calling
type GeminiService struct {
	apiKey     string
	httpClient *http.Client
	imgClient  *http.Client
	enabled    bool
	imageCache *lru.Cache[string, string] // cardID -> base64 image, max 50 entries
}

// NewGeminiService creates a new Gemini service
func NewGeminiService() *GeminiService {
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		if keyPath := os.Getenv("GOOGLE_API_KEY_FILE"); keyPath != "" {
			if data, err := os.ReadFile(keyPath); err == nil {
				apiKey = strings.TrimSpace(string(data))
			}
		}
	}

	// Create LRU cache for images (max 50 images, ~50MB)
	imageCache, err := lru.New[string, string](50)
	if err != nil {
		log.Printf("Failed to create image cache: %v", err)
	}

	svc := &GeminiService{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: geminiTimeout},
		imgClient:  &http.Client{Timeout: imageDownloadTimeout},
		enabled:    apiKey != "",
		imageCache: imageCache,
	}

	if svc.enabled {
		log.Printf("Gemini service: enabled (model=%s, image_cache=50)", geminiModel)
	} else {
		log.Printf("Gemini service: disabled (no GOOGLE_API_KEY)")
	}

	return svc
}

// IsEnabled returns whether Gemini is available
func (s *GeminiService) IsEnabled() bool {
	return s.enabled
}

// CandidateCard represents a potential match for visual comparison
// Includes rich metadata to help Gemini filter candidates without image viewing
type CandidateCard struct {
	// Core identification
	ID       string `json:"id"`
	Name     string `json:"name"`
	SetCode  string `json:"set_code"`
	SetName  string `json:"set_name"`
	Number   string `json:"number"`
	ImageURL string `json:"image_url"`

	// Common to both games
	Rarity      string `json:"rarity,omitempty"`       // "Rare Holo", "mythic", etc.
	Artist      string `json:"artist,omitempty"`       // Artist name for art verification
	ReleaseDate string `json:"release_date,omitempty"` // Set release date (YYYY-MM-DD)

	// Pokemon-specific fields
	Subtypes       []string `json:"subtypes,omitempty"`        // ["V", "VMAX", "ex", "GX"]
	HP             string   `json:"hp,omitempty"`              // "320" - visible on card
	Types          []string `json:"types,omitempty"`           // ["Fire", "Water"] - energy types
	RegulationMark string   `json:"regulation_mark,omitempty"` // "D", "E", "F", "G" - bottom-left of modern cards

	// MTG-specific fields
	TypeLine     string   `json:"type_line,omitempty"`     // "Creature — Goblin Wizard"
	ManaCost     string   `json:"mana_cost,omitempty"`     // "{2}{R}{R}" - top-right corner
	BorderColor  string   `json:"border_color,omitempty"`  // "black", "borderless", "white"
	FrameEffects []string `json:"frame_effects,omitempty"` // ["showcase", "extendedart"]
	PromoTypes   []string `json:"promo_types,omitempty"`   // ["prerelease", "buyabox"]
}

// IdentificationResult is the final result returned to the client
type IdentificationResult struct {
	CardID          string          `json:"card_id"`                     // Matched card ID (empty if no match)
	CardName        string          `json:"card_name"`                   // Card name (may be non-English if that's what was on the card)
	CanonicalNameEN string          `json:"canonical_name_en"`           // English name for lookup/display (always English)
	SetCode         string          `json:"set_code"`                    // Set code
	SetName         string          `json:"set_name"`                    // Set name
	Number          string          `json:"card_number"`                 // Card number
	Game            string          `json:"game"`                        // "pokemon" or "mtg"
	ObservedLang    string          `json:"observed_language,omitempty"` // Language observed on card (e.g., "Japanese", "English")
	IsFoil          bool            `json:"is_foil"`                     // Whether the card appears to be foil/holo
	IsFirstEdition  bool            `json:"is_first_edition"`            // Whether the card has a 1st Edition stamp (Pokemon)
	Confidence      float64         `json:"confidence"`                  // 0-1 confidence score
	Reasoning       string          `json:"reasoning"`                   // Gemini's explanation
	TurnsUsed       int             `json:"turns_used"`                  // Number of API turns used
	Candidates      []CandidateCard `json:"candidates,omitempty"`        // Alternative candidates if low confidence
	SearchTerms     []string        `json:"search_terms,omitempty"`      // Card names Gemini searched for (fallback for handler)
}

// CardSearcher is the interface for searching cards (implemented by Pokemon/Scryfall services)
type CardSearcher interface {
	// SearchByName searches for cards by name, returns up to limit results
	SearchByName(ctx context.Context, name string, limit int) ([]CandidateCard, error)
	// SearchInSet searches for cards within a specific set, optionally filtered by name
	SearchInSet(ctx context.Context, setCode, name string, limit int) ([]CandidateCard, error)
	// GetBySetAndNumber gets a specific card by set code and collector number
	GetBySetAndNumber(ctx context.Context, setCode, number string) (*CandidateCard, error)
	// GetCardImage downloads a card image by ID, returns base64-encoded image
	GetCardImage(ctx context.Context, cardID string) (string, error)
	// GetCardDetails returns full card details for verification (attacks, abilities, text)
	GetCardDetails(ctx context.Context, cardID string) (*CardDetails, error)
	// ListSets returns sets matching a query
	ListSets(ctx context.Context, query string) ([]SetInfo, error)
	// GetSetInfo returns detailed information about a specific set
	GetSetInfo(ctx context.Context, setCode string) (*SetDetails, error)
}

// CardDetails contains full card information for text verification
type CardDetails struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	SetCode  string `json:"set_code"`
	SetName  string `json:"set_name"`
	Number   string `json:"number"`
	Rarity   string `json:"rarity,omitempty"`
	Artist   string `json:"artist,omitempty"`
	ImageURL string `json:"image_url,omitempty"`

	// Pokemon-specific
	HP             string        `json:"hp,omitempty"`              // "320"
	Types          []string      `json:"types,omitempty"`           // ["Fire"]
	Subtypes       []string      `json:"subtypes,omitempty"`        // ["V", "VMAX"]
	Attacks        []AttackInfo  `json:"attacks,omitempty"`         // Attack details
	Abilities      []AbilityInfo `json:"abilities,omitempty"`       // Ability details
	Weaknesses     []string      `json:"weaknesses,omitempty"`      // ["Water x2"]
	Resistances    []string      `json:"resistances,omitempty"`     // ["Fighting -30"]
	RetreatCost    int           `json:"retreat_cost,omitempty"`    // Number of energy to retreat
	RegulationMark string        `json:"regulation_mark,omitempty"` // "F", "G"
	EvolvesFrom    string        `json:"evolves_from,omitempty"`    // What this Pokemon evolves from

	// MTG-specific
	TypeLine   string `json:"type_line,omitempty"`   // "Creature — Goblin Wizard"
	ManaCost   string `json:"mana_cost,omitempty"`   // "{2}{R}{R}"
	OracleText string `json:"oracle_text,omitempty"` // Rules text
	Power      string `json:"power,omitempty"`       // Creature power
	Toughness  string `json:"toughness,omitempty"`   // Creature toughness
	Loyalty    string `json:"loyalty,omitempty"`     // Planeswalker loyalty
	FlavorText string `json:"flavor_text,omitempty"` // Flavor text
}

// AttackInfo describes a Pokemon card attack
type AttackInfo struct {
	Name   string `json:"name"`             // Attack name
	Cost   string `json:"cost,omitempty"`   // Energy cost, e.g., "Fire Fire Colorless"
	Damage string `json:"damage,omitempty"` // Damage dealt, e.g., "120+"
	Text   string `json:"text,omitempty"`   // Attack effect text
}

// AbilityInfo describes a Pokemon card ability
type AbilityInfo struct {
	Name string `json:"name"`           // Ability name
	Type string `json:"type,omitempty"` // Ability type (e.g., "Ability", "Poke-Body")
	Text string `json:"text,omitempty"` // Ability effect text
}

// SetInfo contains information about a card set
type SetInfo struct {
	ID          string `json:"id"`                     // Set code (e.g., "swsh4", "MH2")
	Name        string `json:"name"`                   // Full name (e.g., "Vivid Voltage")
	Series      string `json:"series,omitempty"`       // Series name (e.g., "Sword & Shield")
	ReleaseDate string `json:"release_date,omitempty"` // YYYY-MM-DD
	TotalCards  int    `json:"total_cards,omitempty"`  // Number of cards in set
}

// SetDetails contains detailed information about a specific set
type SetDetails struct {
	ID                string `json:"id"`                           // Set code
	Name              string `json:"name"`                         // Full name
	Series            string `json:"series,omitempty"`             // Series name
	ReleaseDate       string `json:"release_date,omitempty"`       // YYYY-MM-DD
	TotalCards        int    `json:"total_cards,omitempty"`        // Number of cards
	SymbolDescription string `json:"symbol_description,omitempty"` // Description of set symbol for visual matching
	SetType           string `json:"set_type,omitempty"`           // expansion, promo, masters, etc. (MTG)
}

// JapaneseCardSearcher is an optional interface for searching Japanese-exclusive cards.
// Implemented by PokemonHybridService when Japanese card data is loaded.
type JapaneseCardSearcher interface {
	// SearchJapaneseByName searches for Japanese-exclusive cards by name
	SearchJapaneseByName(ctx context.Context, name string, limit int) ([]CandidateCard, error)
}

// detectMimeType returns the MIME type for image bytes
func detectMimeType(data []byte) string {
	// http.DetectContentType uses the first 512 bytes
	contentType := http.DetectContentType(data)
	// It returns things like "image/jpeg", "image/png", "image/gif", "image/webp"
	// For non-image types or unknown, default to jpeg (most common for photos)
	if !strings.HasPrefix(contentType, "image/") {
		return "image/jpeg"
	}
	return contentType
}

// IdentifyCard uses Gemini with function calling to identify a card from an image
func (s *GeminiService) IdentifyCard(
	ctx context.Context,
	imageBytes []byte,
	pokemonSearcher CardSearcher,
	mtgSearcher CardSearcher,
) (*IdentificationResult, error) {
	if !s.enabled {
		return nil, fmt.Errorf("Gemini service not enabled (no GOOGLE_API_KEY)")
	}

	startTime := time.Now()

	// Build initial message with the card image
	imageB64 := base64.StdEncoding.EncodeToString(imageBytes)
	mimeType := detectMimeType(imageBytes)

	// Conversation history
	contents := []geminiContent{
		{
			Role: "user",
			Parts: []geminiPart{
				{InlineData: &geminiInlineData{MimeType: mimeType, Data: imageB64}},
				{Text: systemPrompt},
			},
		},
	}

	var result *IdentificationResult
	turnsUsed := 0
	viewCardImageCalled := false // Track if Gemini ever called view_card_image
	var searchTerms []string     // Track card names Gemini searched for
	searchTermsSeen := make(map[string]bool)

	// Conversation loop - Gemini calls tools until it has an answer
	for turn := 0; turn < maxTurns; turn++ {
		turnsUsed++

		// Call Gemini
		resp, err := s.callGeminiWithTools(ctx, contents)
		if err != nil {
			return nil, fmt.Errorf("turn %d failed: %w", turn+1, err)
		}

		// Check if Gemini wants to call functions
		if len(resp.FunctionCalls) > 0 {
			// Log each function call with its arguments
			for _, call := range resp.FunctionCalls {
				argsJSON, _ := json.Marshal(call.Args)
				log.Printf("Gemini turn %d: calling %s(%s)", turn+1, call.Name, string(argsJSON))
				if call.Name == "view_card_image" {
					viewCardImageCalled = true
				}
				// Track search terms for fallback
				if call.Name == "search_pokemon_cards" || call.Name == "search_mtg_cards" {
					if name, ok := call.Args["name"].(string); ok && name != "" {
						if !searchTermsSeen[name] {
							searchTermsSeen[name] = true
							searchTerms = append(searchTerms, name)
						}
					}
				}
			}

			// Process function calls
			callResults, err := s.executeFunctionCalls(ctx, resp.FunctionCalls, pokemonSearcher, mtgSearcher)
			if err != nil {
				log.Printf("Gemini turn %d: function call error: %v", turn+1, err)
			}

			// Add model's response to history
			contents = append(contents, geminiContent{
				Role:  "model",
				Parts: resp.Parts,
			})

			// Add function responses and any images to history
			for _, result := range callResults {
				// Add the function response
				contents = append(contents, geminiContent{
					Role: "function",
					Parts: []geminiPart{
						{FunctionResponse: result.response},
					},
				})

				// If this was a view_card_image call, inject the actual image
				if result.imageData != "" {
					contents = append(contents, geminiContent{
						Role: "user",
						Parts: []geminiPart{
							{Text: fmt.Sprintf("Here is the image for card %s:", result.imageCardID)},
							{InlineData: &geminiInlineData{MimeType: "image/jpeg", Data: result.imageData}},
						},
					})
				}
			}

			log.Printf("Gemini turn %d: executed %d function calls", turn+1, len(resp.FunctionCalls))
			continue
		}

		// Gemini returned a final answer (text response)
		if resp.Text != "" {
			result, err = s.parseIdentificationResult(resp.Text)
			if err != nil {
				log.Printf("Gemini turn %d: failed to parse result: %v", turn+1, err)
				// Ask Gemini to try again with proper format
				contents = append(contents, geminiContent{
					Role:  "model",
					Parts: []geminiPart{{Text: resp.Text}},
				})
				contents = append(contents, geminiContent{
					Role:  "user",
					Parts: []geminiPart{{Text: "Please provide the result as valid JSON matching the expected schema."}},
				})
				continue
			}

			result.TurnsUsed = turnsUsed

			// Log warning if Gemini returned a card_id without calling view_card_image
			if result.CardID != "" && !viewCardImageCalled {
				log.Printf("WARNING: Gemini returned card_id '%s' without calling view_card_image - artwork not verified!", result.CardID)
			}
			break
		}

		// No function calls and no text - something went wrong
		log.Printf("Gemini turn %d: empty response", turn+1)
		break
	}

	// Record metrics
	metrics.GeminiRequestsTotal.Add(float64(turnsUsed))
	metrics.GeminiAPILatency.Observe(time.Since(startTime).Seconds())
	if result != nil {
		metrics.GeminiConfidenceHistogram.Observe(result.Confidence)
	}

	if result == nil {
		log.Printf("Gemini identification failed: no result after %d turns, view_card_image_called=%v, search_terms=%v", turnsUsed, viewCardImageCalled, searchTerms)
		return &IdentificationResult{
			Game:        "pokemon", // Assume Pokemon since that's most common for Japanese cards
			Confidence:  0,
			Reasoning:   "Failed to identify card after max turns",
			TurnsUsed:   turnsUsed,
			SearchTerms: searchTerms,
		}, nil
	}

	// Add search terms to successful result too (for fallback candidates)
	result.SearchTerms = searchTerms

	// Log summary of identification session
	log.Printf("Gemini identification complete: card_id=%q, canonical_name=%q, view_card_image_called=%v, turns=%d",
		result.CardID, result.CanonicalNameEN, viewCardImageCalled, turnsUsed)

	return result, nil
}

// functionCallResult holds the result of a function call, which may include images
type functionCallResult struct {
	response *geminiFunctionResponse
	// If non-empty, this image should be injected into the conversation after the function response
	imageData   string // base64-encoded image
	imageCardID string // Card ID for context
}

// executeFunctionCalls processes Gemini's function calls and returns responses
func (s *GeminiService) executeFunctionCalls(
	ctx context.Context,
	calls []geminiFunctionCall,
	pokemonSearcher CardSearcher,
	mtgSearcher CardSearcher,
) ([]functionCallResult, error) {
	var results []functionCallResult

	for _, call := range calls {
		var resultJSON []byte
		var err error
		var imageData, imageCardID string

		switch call.Name {
		case "search_pokemon_cards":
			resultJSON, err = s.handleSearchCards(ctx, call.Args, pokemonSearcher)
		case "search_mtg_cards":
			resultJSON, err = s.handleSearchCards(ctx, call.Args, mtgSearcher)
		case "search_japanese_pokemon_cards":
			// Check if the Pokemon searcher implements JapaneseCardSearcher
			if japaneseSearcher, ok := pokemonSearcher.(interface {
				SearchJapaneseByNameForGemini(ctx context.Context, name string, limit int) ([]CandidateCard, error)
			}); ok {
				resultJSON, err = s.handleSearchJapaneseCards(ctx, call.Args, japaneseSearcher)
			} else {
				err = fmt.Errorf("Japanese card search not available (no Japanese card data loaded)")
			}
		case "get_pokemon_card":
			resultJSON, err = s.handleGetCard(ctx, call.Args, pokemonSearcher)
		case "get_mtg_card":
			resultJSON, err = s.handleGetCard(ctx, call.Args, mtgSearcher)
		case "view_card_image":
			// Special handling - returns image data to inject into conversation
			imageData, imageCardID, err = s.handleViewCardImage(ctx, call.Args, pokemonSearcher, mtgSearcher)
			if err == nil {
				resultJSON = []byte(fmt.Sprintf(`{"card_id": "%s", "status": "image_loaded"}`, imageCardID))
			}
		case "get_card_details":
			resultJSON, err = s.handleGetCardDetails(ctx, call.Args, pokemonSearcher, mtgSearcher)
		case "list_pokemon_sets":
			resultJSON, err = s.handleListSets(ctx, call.Args, pokemonSearcher)
		case "list_mtg_sets":
			resultJSON, err = s.handleListSets(ctx, call.Args, mtgSearcher)
		case "view_multiple_card_images":
			// Special handling - returns multiple images to inject into conversation
			images, batchErr := s.handleViewMultipleCardImages(ctx, call.Args, pokemonSearcher, mtgSearcher)
			if batchErr == nil && len(images) > 0 {
				// Return the first image as the primary, others will be in batchImages
				imageData = images[0].imageData
				imageCardID = images[0].cardID
				// Build response with all card IDs
				cardIDs := make([]string, len(images))
				for i, img := range images {
					cardIDs[i] = img.cardID
				}
				resultJSON, _ = json.Marshal(map[string]interface{}{
					"card_ids": cardIDs,
					"count":    len(images),
					"status":   "images_loaded",
				})
			} else {
				err = batchErr
			}
		case "search_cards_in_set":
			game, _ := call.Args["game"].(string)
			if game == "mtg" {
				resultJSON, err = s.handleSearchCardsInSet(ctx, call.Args, mtgSearcher)
			} else {
				resultJSON, err = s.handleSearchCardsInSet(ctx, call.Args, pokemonSearcher)
			}
		case "get_set_info":
			game, _ := call.Args["game"].(string)
			if game == "mtg" {
				resultJSON, err = s.handleGetSetInfo(ctx, call.Args, mtgSearcher)
			} else {
				resultJSON, err = s.handleGetSetInfo(ctx, call.Args, pokemonSearcher)
			}
		default:
			err = fmt.Errorf("unknown function: %s", call.Name)
		}

		response := &geminiFunctionResponse{
			Name: call.Name,
		}

		if err != nil {
			response.Response = map[string]interface{}{"error": err.Error()}
		} else {
			var result interface{}
			if err := json.Unmarshal(resultJSON, &result); err != nil {
				response.Response = map[string]interface{}{"error": "invalid response format"}
			} else {
				response.Response = result
			}
		}

		results = append(results, functionCallResult{
			response:    response,
			imageData:   imageData,
			imageCardID: imageCardID,
		})
	}

	return results, nil
}

func (s *GeminiService) handleSearchCards(ctx context.Context, args map[string]interface{}, searcher CardSearcher) ([]byte, error) {
	name, _ := args["name"].(string)
	if name == "" {
		return json.Marshal(map[string]interface{}{"error": "name is required"})
	}

	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
		if limit > 20 {
			limit = 20
		}
	}

	cards, err := searcher.SearchByName(ctx, name, limit)
	if err != nil {
		return json.Marshal(map[string]interface{}{"error": err.Error()})
	}

	return json.Marshal(map[string]interface{}{
		"cards": cards,
		"count": len(cards),
	})
}

// JapaneseSearcher is the interface for the SearchJapaneseByNameForGemini method
type JapaneseSearcher interface {
	SearchJapaneseByNameForGemini(ctx context.Context, name string, limit int) ([]CandidateCard, error)
}

func (s *GeminiService) handleSearchJapaneseCards(ctx context.Context, args map[string]interface{}, searcher JapaneseSearcher) ([]byte, error) {
	name, _ := args["name"].(string)
	if name == "" {
		return json.Marshal(map[string]interface{}{"error": "name is required"})
	}

	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
		if limit > 20 {
			limit = 20
		}
	}

	cards, err := searcher.SearchJapaneseByNameForGemini(ctx, name, limit)
	if err != nil {
		return json.Marshal(map[string]interface{}{"error": err.Error()})
	}

	if len(cards) == 0 {
		return json.Marshal(map[string]interface{}{
			"cards":   cards,
			"count":   0,
			"message": "No Japanese-exclusive cards found. The card may be available in the English database if it was released internationally.",
		})
	}

	return json.Marshal(map[string]interface{}{
		"cards": cards,
		"count": len(cards),
	})
}

func (s *GeminiService) handleGetCard(ctx context.Context, args map[string]interface{}, searcher CardSearcher) ([]byte, error) {
	setCode, _ := args["set_code"].(string)
	number, _ := args["number"].(string)

	if setCode == "" || number == "" {
		return json.Marshal(map[string]interface{}{"error": "set_code and number are required"})
	}

	card, err := searcher.GetBySetAndNumber(ctx, setCode, number)
	if err != nil {
		return json.Marshal(map[string]interface{}{"error": err.Error()})
	}
	if card == nil {
		return json.Marshal(map[string]interface{}{"error": "card not found"})
	}

	return json.Marshal(card)
}

// handleViewCardImage returns (imageBase64, cardID, error)
// The image data is returned separately so it can be injected into the conversation
// Uses LRU cache to avoid re-downloading the same image
func (s *GeminiService) handleViewCardImage(ctx context.Context, args map[string]interface{}, pokemonSearcher, mtgSearcher CardSearcher) (string, string, error) {
	cardID, _ := args["card_id"].(string)
	game, _ := args["game"].(string)

	if cardID == "" {
		return "", "", fmt.Errorf("card_id is required")
	}

	// Check cache first
	cacheKey := game + ":" + cardID
	if s.imageCache != nil {
		if cached, ok := s.imageCache.Get(cacheKey); ok {
			log.Printf("Image cache hit: %s", cardID)
			return cached, cardID, nil
		}
	}

	var searcher CardSearcher
	if game == "mtg" {
		searcher = mtgSearcher
	} else {
		searcher = pokemonSearcher
	}

	imageB64, err := searcher.GetCardImage(ctx, cardID)
	if err != nil {
		return "", cardID, err
	}

	// Cache the result
	if s.imageCache != nil {
		s.imageCache.Add(cacheKey, imageB64)
		log.Printf("Image cached: %s", cardID)
	}

	return imageB64, cardID, nil
}

// handleGetCardDetails returns full card details for text verification
func (s *GeminiService) handleGetCardDetails(ctx context.Context, args map[string]interface{}, pokemonSearcher, mtgSearcher CardSearcher) ([]byte, error) {
	cardID, _ := args["card_id"].(string)
	game, _ := args["game"].(string)

	if cardID == "" {
		return json.Marshal(map[string]interface{}{"error": "card_id is required"})
	}

	var searcher CardSearcher
	if game == "mtg" {
		searcher = mtgSearcher
	} else {
		searcher = pokemonSearcher
	}

	details, err := searcher.GetCardDetails(ctx, cardID)
	if err != nil {
		return json.Marshal(map[string]interface{}{"error": err.Error()})
	}

	return json.Marshal(details)
}

// handleListSets returns sets matching a query
func (s *GeminiService) handleListSets(ctx context.Context, args map[string]interface{}, searcher CardSearcher) ([]byte, error) {
	query, _ := args["query"].(string)

	if query == "" {
		return json.Marshal(map[string]interface{}{"error": "query is required"})
	}

	sets, err := searcher.ListSets(ctx, query)
	if err != nil {
		return json.Marshal(map[string]interface{}{"error": err.Error()})
	}

	return json.Marshal(map[string]interface{}{
		"sets":  sets,
		"count": len(sets),
	})
}

// handleViewMultipleCardImages fetches multiple card images at once
// Returns images to be injected into the conversation
func (s *GeminiService) handleViewMultipleCardImages(ctx context.Context, args map[string]interface{}, pokemonSearcher, mtgSearcher CardSearcher) ([]struct{ cardID, imageData string }, error) {
	cardIDsRaw, _ := args["card_ids"].([]interface{})
	game, _ := args["game"].(string)

	if len(cardIDsRaw) == 0 {
		return nil, fmt.Errorf("card_ids array is required")
	}
	if len(cardIDsRaw) > 3 {
		return nil, fmt.Errorf("maximum 3 card_ids allowed per call")
	}

	var searcher CardSearcher
	if game == "mtg" {
		searcher = mtgSearcher
	} else {
		searcher = pokemonSearcher
	}

	var results []struct{ cardID, imageData string }
	for _, idRaw := range cardIDsRaw {
		cardID, ok := idRaw.(string)
		if !ok || cardID == "" {
			continue
		}

		// Check cache first
		cacheKey := game + ":" + cardID
		var imageB64 string
		if s.imageCache != nil {
			if cached, ok := s.imageCache.Get(cacheKey); ok {
				log.Printf("Image cache hit (batch): %s", cardID)
				imageB64 = cached
			}
		}

		if imageB64 == "" {
			var err error
			imageB64, err = searcher.GetCardImage(ctx, cardID)
			if err != nil {
				log.Printf("Failed to fetch image for %s: %v", cardID, err)
				continue
			}
			// Cache the result
			if s.imageCache != nil {
				s.imageCache.Add(cacheKey, imageB64)
				log.Printf("Image cached (batch): %s", cardID)
			}
		}

		results = append(results, struct{ cardID, imageData string }{cardID, imageB64})
	}

	return results, nil
}

// handleSearchCardsInSet searches for cards within a specific set
func (s *GeminiService) handleSearchCardsInSet(ctx context.Context, args map[string]interface{}, searcher CardSearcher) ([]byte, error) {
	setCode, _ := args["set_code"].(string)
	name, _ := args["name"].(string) // optional
	game, _ := args["game"].(string)

	if setCode == "" {
		return json.Marshal(map[string]interface{}{"error": "set_code is required"})
	}

	limit := 20
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
		if limit > 50 {
			limit = 50
		}
	}

	cards, err := searcher.SearchInSet(ctx, setCode, name, limit)
	if err != nil {
		return json.Marshal(map[string]interface{}{
			"error":      err.Error(),
			"set_code":   setCode,
			"suggestion": fmt.Sprintf("Try list_%s_sets to verify the set code exists", game),
		})
	}

	if len(cards) == 0 {
		return json.Marshal(map[string]interface{}{
			"cards":      cards,
			"count":      0,
			"set_code":   setCode,
			"suggestion": "No cards found. Try a different search term or verify the set code.",
		})
	}

	return json.Marshal(map[string]interface{}{
		"cards":    cards,
		"count":    len(cards),
		"set_code": setCode,
	})
}

// handleGetSetInfo returns detailed information about a specific set
func (s *GeminiService) handleGetSetInfo(ctx context.Context, args map[string]interface{}, searcher CardSearcher) ([]byte, error) {
	setCode, _ := args["set_code"].(string)

	if setCode == "" {
		return json.Marshal(map[string]interface{}{"error": "set_code is required"})
	}

	setInfo, err := searcher.GetSetInfo(ctx, setCode)
	if err != nil {
		return json.Marshal(map[string]interface{}{
			"error":      err.Error(),
			"set_code":   setCode,
			"suggestion": "Use list_pokemon_sets or list_mtg_sets to find valid set codes",
		})
	}

	return json.Marshal(setInfo)
}

func (s *GeminiService) parseIdentificationResult(text string) (*IdentificationResult, error) {
	// Try to extract JSON from the response
	text = strings.TrimSpace(text)

	// Handle markdown code blocks
	if strings.HasPrefix(text, "```json") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimSuffix(text, "```")
		text = strings.TrimSpace(text)
	} else if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimSuffix(text, "```")
		text = strings.TrimSpace(text)
	}

	var result IdentificationResult
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w (text: %s)", err, text)
	}

	return &result, nil
}

// callGeminiWithTools makes a request to Gemini with function calling enabled
func (s *GeminiService) callGeminiWithTools(ctx context.Context, contents []geminiContent) (*geminiModelResponse, error) {
	req := geminiRequestWithTools{
		Contents: contents,
		Tools:    []geminiTool{{FunctionDeclarations: toolDeclarations}},
		GenerationConfig: geminiGenConfig{
			Temperature:     0.1,
			MaxOutputTokens: 2048,
		},
	}

	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf(geminiAPIURL, geminiModel) + "?key=" + s.apiKey
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		metrics.GeminiErrorsTotal.WithLabelValues("network").Inc()
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		metrics.GeminiErrorsTotal.WithLabelValues("read").Inc()
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		metrics.GeminiErrorsTotal.WithLabelValues("api").Inc()
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp geminiAPIResponseWithTools
	if err := json.Unmarshal(body, &apiResp); err != nil {
		metrics.GeminiErrorsTotal.WithLabelValues("parse").Inc()
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	if apiResp.Error != nil {
		metrics.GeminiErrorsTotal.WithLabelValues("api").Inc()
		return nil, fmt.Errorf("API error %d: %s", apiResp.Error.Code, apiResp.Error.Message)
	}

	if len(apiResp.Candidates) == 0 {
		metrics.GeminiErrorsTotal.WithLabelValues("empty").Inc()
		return nil, fmt.Errorf("no response from Gemini")
	}

	// Extract function calls and text from response
	result := &geminiModelResponse{
		Parts: apiResp.Candidates[0].Content.Parts,
	}

	for _, part := range apiResp.Candidates[0].Content.Parts {
		if part.FunctionCall != nil {
			result.FunctionCalls = append(result.FunctionCalls, *part.FunctionCall)
		}
		if part.Text != "" {
			result.Text = part.Text
		}
	}

	return result, nil
}

// fetchImage downloads an image from a URL
func (s *GeminiService) FetchImage(ctx context.Context, imageURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", imageURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.imgClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// Limit to 5MB
	return io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
}

// Gemini API types for function calling

type geminiRequestWithTools struct {
	Contents         []geminiContent `json:"contents"`
	Tools            []geminiTool    `json:"tools"`
	GenerationConfig geminiGenConfig `json:"generationConfig"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string                  `json:"text,omitempty"`
	InlineData       *geminiInlineData       `json:"inline_data,omitempty"`
	FunctionCall     *geminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResponse `json:"functionResponse,omitempty"`
}

type geminiInlineData struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

type geminiFunctionCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

type geminiFunctionResponse struct {
	Name     string      `json:"name"`
	Response interface{} `json:"response"`
}

type geminiTool struct {
	FunctionDeclarations []geminiFunctionDecl `json:"function_declarations"`
}

type geminiFunctionDecl struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type geminiGenConfig struct {
	ResponseMimeType string  `json:"responseMimeType,omitempty"`
	Temperature      float64 `json:"temperature"`
	MaxOutputTokens  int     `json:"maxOutputTokens"`
}

type geminiAPIResponseWithTools struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
			Role  string       `json:"role"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type geminiModelResponse struct {
	Parts         []geminiPart
	FunctionCalls []geminiFunctionCall
	Text          string
}

// System prompt and tool declarations

const systemPrompt = `You are a trading card identification expert. I'm showing you a photo of a trading card (Pokemon TCG or Magic: The Gathering).

YOUR TASK: Identify the EXACT card printing shown in the image.

=== POKEMON CARD VISUAL GUIDE ===
+----------------------------------+
|  [NAME]               [HP] [TYPE]|  <- HP top-right (e.g., "HP 320"), type symbol far right
|  +----------------------------+  |
|  |                            |  |
|  |         ARTWORK            |  |
|  |                            |  |
|  +----------------------------[S]|  <- [S] = SET SYMBOL at bottom-right of art box
|[1]                               |  <- [1] = 1ST EDITION stamp left of art (if present)
|  Attack Name           Damage    |
|  --------------------------------|
|[R]        [Artist]     [###/###] |  <- [R] = REGULATION MARK (D,E,F,G,H) bottom-left
|                        [RARITY]  |  <- COLLECTOR NUMBER + RARITY bottom-right
+----------------------------------+

KEY POKEMON IDENTIFIERS:
- Collector number: Bottom-right, format "025/185" or just "025"
- Set symbol: Small icon at bottom-right of artwork box (matches the set)
- 1st Edition stamp: Black "1" in shadow, LEFT side below artwork (WotC era: Base-Neo)
- Regulation mark: Single letter (D,E,F,G,H) at bottom-left (modern cards 2019+)
- Rarity: ● common, ◆ uncommon, ★ rare, ★H holo rare, ★★★ ultra rare
- Subtypes in name: "V", "VMAX", "VSTAR", "ex", "GX", "EX" indicate card variant
- Language hints: HP=English, KP=German, PV=French, PS=Spanish/Italian

=== MTG CARD VISUAL GUIDE ===
+----------------------------------+
|  [NAME]               [MANA COST]|  <- Mana symbols top-right corner
|  +----------------------------+  |
|  |                            |  |
|  |         ARTWORK            |  |
|  |                            |  |
|  +----------------------------+  |
|  [TYPE LINE]             [SET S] |  <- SET SYMBOL middle-right (color=rarity)
|  --------------------------------|
|  Rules text...                   |
|  --------------------------------|
|  [COLLECTOR#]           [P/T]    |  <- Power/Toughness bottom-right (creatures)
+----------------------------------+

KEY MTG IDENTIFIERS:
- Set symbol color: GOLD=mythic, ORANGE=rare, SILVER=uncommon, BLACK=common
- Border: Black=standard, White=pre-8th edition, Borderless=premium, Silver=Un-sets
- Frame effects: "Showcase" (special art frame), "Extended art", "Borderless"
- Collector number: Bottom-left, numbers beyond set size (285/280) = bonus/variant
- Type line: "Creature — Goblin Wizard" visible below artwork

=== EFFICIENT WORKFLOW (3-4 turns target) ===

TURN 1 - ANALYZE & SEARCH:
1. Determine game (Pokemon or MTG) from card layout
2. Read the card name (in any language)
3. Note: HP/subtypes (Pokemon), mana cost/type (MTG), collector number if visible
4. Search using English name: search_pokemon_cards or search_mtg_cards

TURN 2 - FILTER CANDIDATES:
Search results now include RICH DATA - use it to filter WITHOUT viewing images:
- Pokemon: hp, subtypes, types, rarity, artist, release_date, regulation_mark
- MTG: type_line, mana_cost, rarity, border_color, frame_effects, artist

Example filtering:
- Scanned card shows HP 320, "VMAX" in name → filter for hp="320", subtypes contains "VMAX"
- Scanned card shows regulation mark "G" → filter for regulation_mark="G"
- If unsure, use get_card_details to verify HP/attacks/abilities match

TURN 3 - VERIFY ARTWORK:
Call view_card_image for the 1-2 best candidates after filtering.
Compare these specific features:
1. CHARACTER POSE: Body position, facing direction, action
2. BACKGROUND: Sky, landscape, patterns, energy effects, colors
3. ART STYLE: 3D CGI vs hand-drawn vs watercolor
4. COMPOSITION: Full body vs close-up, centered vs off-center

TURN 4 - RETURN RESULT:
Return the matching card_id with confidence score.

=== TOOLS REFERENCE ===

SEARCH TOOLS (return rich metadata for filtering):
- search_pokemon_cards: Returns id, name, set, number, rarity, hp, types, subtypes, artist, release_date, regulation_mark
- search_mtg_cards: Returns id, name, set, number, rarity, type_line, mana_cost, border_color, frame_effects, artist
- search_japanese_pokemon_cards: For Japanese-exclusive cards with different artwork
- search_cards_in_set(set_code, name?, game): Search within a specific set (more targeted, use after identifying set)

LOOKUP TOOLS (for exact matches):
- get_pokemon_card(set_code, number): Get specific card by set+number
- get_mtg_card(set_code, number): Get specific MTG card by set+number
- list_pokemon_sets(query): Find set codes by name/series (e.g., "Vivid Voltage", "Sword & Shield")
- list_mtg_sets(query): Find MTG set codes by name/type (e.g., "Modern Horizons", "masters")
- get_set_info(set_code, game): Get detailed set info including symbol description (helps match set symbols visually)

VERIFICATION TOOLS:
- get_card_details: Get full card data (attacks, abilities, oracle text) to verify text matches
- view_card_image: REQUIRED before returning card_id - compare actual artwork
- view_multiple_card_images(card_ids, game): View 2-3 images at once (more efficient than multiple single calls)

=== IMPORTANT RULES ===

1. USE METADATA FIRST: Filter candidates by hp/subtypes/type_line before viewing images
2. VERIFY ARTWORK: You MUST call view_card_image at least once before returning a card_id
3. ONE MATCH RULE: Only return card_id for a card you VIEWED and VERIFIED
4. NO MATCH: Return card_id="" with candidates list if no artwork matches

=== SPECIAL CASES ===

JAPANESE CARDS:
- Read Japanese text (ポケモン = Pokemon, etc.)
- Most Japanese cards share artwork with English → search English database first
- If artwork doesn't match ANY English version → use search_japanese_pokemon_cards
- Japanese-exclusive sets: "Leaders' Stadium", "Gym" sets (IDs prefixed with "jp-")

1ST EDITION DETECTION (Pokemon):
- Look for black "1" stamp LEFT of artwork, below the art box
- Only exists on WotC-era sets: Base Set, Jungle, Fossil, Team Rocket, Gym Heroes/Challenge, Neo series
- Set is_first_edition: true if stamp is present

FOIL/HOLO DETECTION:
- Look for holographic sheen, rainbow gradients, sparkle patterns
- Check artwork area AND card border for holo effects
- Set is_foil: true if any holographic elements visible

LANGUAGE DETECTION:
- Japanese: Japanese characters (カタカナ, ひらがな, 漢字)
- German: "KP" for HP, German text
- French: "PV" for HP, French text
- Spanish: "PS" for HP
- Set observed_language to the detected language

=== RESPONSE FORMAT ===

When you have VERIFIED artwork match:
{
  "card_id": "the-verified-card-id",
  "card_name": "Name as printed on card (may be non-English)",
  "canonical_name_en": "English name",
  "set_code": "swsh4",
  "set_name": "Vivid Voltage",
  "card_number": "025",
  "game": "pokemon",
  "observed_language": "Japanese",
  "is_foil": false,
  "is_first_edition": false,
  "confidence": 0.95,
  "reasoning": "Matched by: HP 320 matches, VMAX subtype matches, artwork comparison shows same pose/background"
}

If NO match found after verification:
{
  "card_id": "",
  "card_name": "Name on card",
  "canonical_name_en": "English translation",
  "game": "pokemon",
  "observed_language": "Japanese",
  "is_foil": false,
  "is_first_edition": false,
  "confidence": 0.0,
  "reasoning": "Viewed swsh4-25 and sv4-25 but neither artwork matched the scanned card",
  "candidates": [{"id": "swsh4-25", "name": "Charizard VMAX"}, ...]
}`

var toolDeclarations = []geminiFunctionDecl{
	{
		Name:        "search_pokemon_cards",
		Description: "Search for Pokemon TCG cards by name. Returns RICH DATA for each card: id, name, set_code, set_name, number, rarity, hp, types (energy), subtypes (V/VMAX/ex/GX), artist, release_date. Use this metadata to FILTER candidates before calling view_card_image. For example, if scanned card shows HP 320 and 'VMAX', filter results by hp='320' and subtypes containing 'VMAX'.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Card name to search for in ENGLISH (e.g., 'Charizard', 'Pikachu V', 'Professor's Research')",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of results (default 10, max 20)",
				},
			},
			"required": []string{"name"},
		},
	},
	{
		Name:        "search_mtg_cards",
		Description: "Search for MTG cards by name. Returns RICH DATA for each card: id, name, set_code, set_name, number, rarity, type_line, mana_cost, border_color, frame_effects (showcase/borderless/extendedart), promo_types, artist, release_date. Use this metadata to FILTER candidates - e.g., if scanned card is borderless, filter for border_color='borderless'.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Card name to search for (e.g., 'Lightning Bolt', 'Black Lotus')",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of results (default 10, max 20)",
				},
			},
			"required": []string{"name"},
		},
	},
	{
		Name:        "get_pokemon_card",
		Description: "Get a specific Pokemon card by set code and collector number. Use this when you can read the set code and number from the card.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"set_code": map[string]interface{}{
					"type":        "string",
					"description": "Set code (e.g., 'swsh4', 'sv4', 'base1', 'neo1', 'mew')",
				},
				"number": map[string]interface{}{
					"type":        "string",
					"description": "Collector number (e.g., '25', '025', 'TG15')",
				},
			},
			"required": []string{"set_code", "number"},
		},
	},
	{
		Name:        "get_mtg_card",
		Description: "Get a specific MTG card by set code and collector number. Use this when you can read the set code and number from the card.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"set_code": map[string]interface{}{
					"type":        "string",
					"description": "Three-letter set code (e.g., '2XM', 'MH2', 'ONE')",
				},
				"number": map[string]interface{}{
					"type":        "string",
					"description": "Collector number",
				},
			},
			"required": []string{"set_code", "number"},
		},
	},
	{
		Name:        "view_card_image",
		Description: "REQUIRED before returning any card_id. Downloads and shows you the official card image for visual comparison. Compare: 1) Character pose and position, 2) Background elements and colors, 3) Art style (3D CGI vs hand-drawn), 4) Overall composition. Only call this for 1-2 top candidates AFTER filtering by metadata.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"card_id": map[string]interface{}{
					"type":        "string",
					"description": "The card ID from search results",
				},
				"game": map[string]interface{}{
					"type":        "string",
					"description": "Game type: 'pokemon' or 'mtg'",
				},
			},
			"required": []string{"card_id", "game"},
		},
	},
	{
		Name:        "search_japanese_pokemon_cards",
		Description: "Search for Japanese-exclusive Pokemon TCG cards by name. Use this when the card is Japanese AND the English versions have different artwork (e.g., censored artwork like Misty's Tears, Japanese-only sets like Leaders' Stadium). Returns Japanese cards with their IDs, names, set info, and image URLs.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Card name to search for in English (e.g., 'Misty\\'s Tears', 'Sabrina')",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of results (default 10, max 20)",
				},
			},
			"required": []string{"name"},
		},
	},
	{
		Name:        "get_card_details",
		Description: "Get full details for a specific card including HP, attacks, abilities (Pokemon) or oracle text, power/toughness (MTG). Use this to VERIFY that readable text on the scanned card matches a candidate. For example, if you can read HP '320' and attack 'Max Blaze' on the scanned card, use this to verify the candidate has the same values.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"card_id": map[string]interface{}{
					"type":        "string",
					"description": "The card ID from search results to get details for",
				},
				"game": map[string]interface{}{
					"type":        "string",
					"description": "Game type: 'pokemon' or 'mtg'",
				},
			},
			"required": []string{"card_id", "game"},
		},
	},
	{
		Name:        "list_pokemon_sets",
		Description: "Search for Pokemon TCG sets by name, series, or set code. Use this when you need to find a set code from a set name or symbol, or to narrow down which era a card is from. Returns set codes, names, series, release dates, and card counts.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query: set name (e.g., 'Vivid Voltage'), series (e.g., 'Sword & Shield'), or set code (e.g., 'swsh4')",
				},
			},
			"required": []string{"query"},
		},
	},
	{
		Name:        "list_mtg_sets",
		Description: "Search for MTG sets by name, set type, or set code. Use this to find set codes or to understand what sets exist in a particular category. Returns set codes, names, types (expansion/masters/promo), release dates, and card counts.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query: set name (e.g., 'Modern Horizons'), set type (e.g., 'masters'), or set code (e.g., 'MH2')",
				},
			},
			"required": []string{"query"},
		},
	},
	{
		Name:        "view_multiple_card_images",
		Description: "View 2-3 card images at once for efficient comparison. More efficient than multiple view_card_image calls. Use after filtering candidates by metadata (HP, subtypes, rarity). Maximum 3 cards per call.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"card_ids": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Array of card IDs to view (max 3)",
				},
				"game": map[string]interface{}{
					"type":        "string",
					"description": "Game type: 'pokemon' or 'mtg'",
				},
			},
			"required": []string{"card_ids", "game"},
		},
	},
	{
		Name:        "search_cards_in_set",
		Description: "Search for cards within a specific set. More targeted than general search - use when you've identified the set from the set symbol or other indicators. Can optionally filter by card name.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"set_code": map[string]interface{}{
					"type":        "string",
					"description": "Set code (e.g., 'swsh4' for Pokemon, 'MH2' for MTG)",
				},
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Optional: filter by card name within the set",
				},
				"game": map[string]interface{}{
					"type":        "string",
					"description": "Game type: 'pokemon' or 'mtg'",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum results (default 20, max 50)",
				},
			},
			"required": []string{"set_code", "game"},
		},
	},
	{
		Name:        "get_set_info",
		Description: "Get detailed information about a specific set by its code. Returns set name, series, release date, total cards, and a description of the set symbol to help with visual matching.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"set_code": map[string]interface{}{
					"type":        "string",
					"description": "Set code (e.g., 'swsh4' for Pokemon, 'MH2' for MTG)",
				},
				"game": map[string]interface{}{
					"type":        "string",
					"description": "Game type: 'pokemon' or 'mtg'",
				},
			},
			"required": []string{"set_code", "game"},
		},
	},
}
