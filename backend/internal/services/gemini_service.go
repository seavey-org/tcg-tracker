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

	svc := &GeminiService{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: geminiTimeout},
		imgClient:  &http.Client{Timeout: imageDownloadTimeout},
		enabled:    apiKey != "",
	}

	if svc.enabled {
		log.Printf("Gemini service: enabled (model=%s)", geminiModel)
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
type CandidateCard struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	SetCode  string `json:"set_code"`
	SetName  string `json:"set_name"`
	Number   string `json:"number"`
	ImageURL string `json:"image_url"`
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
	Confidence      float64         `json:"confidence"`                  // 0-1 confidence score
	Reasoning       string          `json:"reasoning"`                   // Gemini's explanation
	TurnsUsed       int             `json:"turns_used"`                  // Number of API turns used
	Candidates      []CandidateCard `json:"candidates,omitempty"`        // Alternative candidates if low confidence
}

// CardSearcher is the interface for searching cards (implemented by Pokemon/Scryfall services)
type CardSearcher interface {
	// SearchByName searches for cards by name, returns up to limit results
	SearchByName(ctx context.Context, name string, limit int) ([]CandidateCard, error)
	// GetBySetAndNumber gets a specific card by set code and collector number
	GetBySetAndNumber(ctx context.Context, setCode, number string) (*CandidateCard, error)
	// GetCardImage downloads a card image by ID, returns base64-encoded image
	GetCardImage(ctx context.Context, cardID string) (string, error)
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
		log.Printf("Gemini identification failed: no result after %d turns, view_card_image_called=%v", turnsUsed, viewCardImageCalled)
		return &IdentificationResult{
			Game:       "unknown",
			Confidence: 0,
			Reasoning:  "Failed to identify card after max turns",
			TurnsUsed:  turnsUsed,
		}, nil
	}

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
func (s *GeminiService) handleViewCardImage(ctx context.Context, args map[string]interface{}, pokemonSearcher, mtgSearcher CardSearcher) (string, string, error) {
	cardID, _ := args["card_id"].(string)
	game, _ := args["game"].(string)

	if cardID == "" {
		return "", "", fmt.Errorf("card_id is required")
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

	return imageB64, cardID, nil
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

YOUR TASK: Identify the EXACT card printing shown in the image by matching ARTWORK and SET SYMBOLS.

CRITICAL WORKFLOW - YOU MUST FOLLOW THESE STEPS:
1. Analyze the scanned image: Is it Pokemon or MTG? What language? Read the card name.
2. Search for the card by its ENGLISH name using search_pokemon_cards or search_mtg_cards
3. For EACH promising candidate from search results, call view_card_image to SEE the actual card image
4. VISUALLY COMPARE the scanned card's artwork with each candidate image you retrieve
5. ONLY return a card_id for a card whose image you have ACTUALLY VIEWED and CONFIRMED matches

IMPORTANT - YOU CANNOT SEE IMAGE URLs:
- Search results contain image URLs, but you CANNOT see those URLs as images
- You MUST call view_card_image to actually see and compare a card's artwork
- Do NOT assume artwork matches based on card name alone - many cards have multiple artworks
- Do NOT skip the view_card_image step - it is REQUIRED

VISUAL COMPARISON CHECKLIST (do this for each candidate):
- [ ] Called view_card_image to retrieve the candidate's image
- [ ] Compared the ILLUSTRATION (character pose, background, art style)
- [ ] Compared the SET SYMBOL/ICON (indicates which set the card is from)
- [ ] Verified the artwork matches EXACTLY, not just "similar"

SET SYMBOL LOCATIONS:
- Pokemon: Set symbol on the right side of the card, below the artwork
- MTG: Set symbol in the middle-right area of the card
- Japanese/foreign cards have the SAME artwork but may have slightly different symbols

TOOLS AVAILABLE:
- search_pokemon_cards: Search by name, returns card metadata (NOT viewable images)
- search_mtg_cards: Search by name, returns card metadata (NOT viewable images)
- get_pokemon_card: Get card by set+number, returns metadata (NOT viewable images)
- get_mtg_card: Get card by set+number, returns metadata (NOT viewable images)
- view_card_image: REQUIRED - Actually retrieves and shows you the card image for comparison

CRITICAL RULES:
1. You MUST call view_card_image at least once before returning any card_id
2. The card_id you return MUST be for a card whose image you VIEWED and VERIFIED
3. NEVER return a card_id based only on name/number match - ALWAYS verify artwork
4. If you cannot verify artwork match, set card_id="" and list candidates

SET CODES:
- Pokemon: "swsh4", "sv4", "mew", "base1", "neo1", "base2" (Jungle), etc.
- MTG: 3 letters like "2XM", "MH2", "ONE"
- Our database only contains ENGLISH cards

LANGUAGE DETECTION:
- Japanese: Japanese characters (ポケモン, モンスターボール, etc.)
- German: "KP" for HP
- French: "PV" for HP

When you have VIEWED and VERIFIED artwork match, respond with JSON:
{
  "card_id": "the card ID (only if you called view_card_image and verified match)",
  "card_name": "Name as printed on the card (may be non-English)",
  "canonical_name_en": "English name for this card",
  "set_code": "set code",
  "set_name": "Set Name",
  "card_number": "collector number from the scanned card",
  "game": "pokemon" or "mtg",
  "observed_language": "English" or "Japanese" or "German" etc.",
  "confidence": 0.0-1.0,
  "reasoning": "MUST describe the artwork comparison you performed"
}

If you did not find a match or could not verify:
{
  "card_id": "",
  "card_name": "Name as printed on the card",
  "canonical_name_en": "English translation of the name",
  "game": "pokemon" or "mtg",
  "observed_language": "detected language",
  "confidence": 0.0,
  "reasoning": "Explain what images you compared and why none matched",
  "candidates": [{"id": "card-id", "name": "Card Name"}, ...]
}`

var toolDeclarations = []geminiFunctionDecl{
	{
		Name:        "search_pokemon_cards",
		Description: "Search for Pokemon TCG cards by name. Returns cards with their IDs, names, set info, and image URLs.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Card name to search for (e.g., 'Charizard', 'Pikachu V', 'Professor's Research')",
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
		Description: "Search for Magic: The Gathering cards by name. Returns cards with their IDs, names, set info, and image URLs.",
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
		Description: "View a candidate card's official image to compare artwork with the scanned card. Use this to verify the artwork matches exactly.",
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
}
