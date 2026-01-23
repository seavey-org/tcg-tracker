package handlers

import (
	"bytes"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/codyseavey/tcg-tracker/backend/internal/database"
	"github.com/codyseavey/tcg-tracker/backend/internal/models"
	"github.com/codyseavey/tcg-tracker/backend/internal/services"
)

type CardHandler struct {
	scryfallService  *services.ScryfallService
	pokemonService   *services.PokemonHybridService
	serverOCRService *services.ServerOCRService
}

func NewCardHandler(scryfall *services.ScryfallService, pokemon *services.PokemonHybridService) *CardHandler {
	return &CardHandler{
		scryfallService:  scryfall,
		pokemonService:   pokemon,
		serverOCRService: services.NewServerOCRService(),
	}
}

func (h *CardHandler) SearchCards(c *gin.Context) {
	query := c.Query("q")
	game := c.Query("game")
	setIDs := strings.TrimSpace(c.Query("set_ids"))

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter 'q' is required"})
		return
	}

	var result *models.CardSearchResult
	var err error

	switch game {
	case "mtg":
		result, err = h.scryfallService.SearchCards(query)
	case "pokemon":
		result, err = h.pokemonService.SearchCards(query)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "game parameter must be 'mtg' or 'pokemon'"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if setIDs != "" {
		allowed := map[string]struct{}{}
		for _, id := range strings.Split(setIDs, ",") {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			allowed[strings.ToLower(id)] = struct{}{}
		}

		if len(allowed) > 0 {
			filtered := make([]models.Card, 0, len(result.Cards))
			for i := range result.Cards {
				if _, ok := allowed[strings.ToLower(result.Cards[i].SetCode)]; ok {
					filtered = append(filtered, result.Cards[i])
				}
			}
			result.Cards = filtered
			result.TotalCount = len(filtered)
			result.HasMore = false
		}
	}

	// Cache cards in database (log errors but don't fail the request)
	db := database.GetDB()
	for i := range result.Cards {
		if err := db.Save(&result.Cards[i]).Error; err != nil {
			// Log the error but continue - caching failure shouldn't fail the search
			c.Writer.Header().Set("X-Cache-Warning", "Some cards failed to cache")
		}
	}

	c.JSON(http.StatusOK, result)
}

func (h *CardHandler) GetCard(c *gin.Context) {
	id := c.Param("id")
	game := c.Query("game")

	// First try to get from cache
	db := database.GetDB()
	var cachedCard models.Card
	if err := db.First(&cachedCard, "id = ?", id).Error; err == nil {
		c.JSON(http.StatusOK, cachedCard)
		return
	}

	// If not in cache, fetch from API
	var card *models.Card
	var err error

	switch game {
	case "mtg":
		card, err = h.scryfallService.GetCard(id)
	case "pokemon":
		card, err = h.pokemonService.GetCard(id)
	default:
		// Try to determine game from ID format
		// Scryfall IDs are UUIDs, Pokemon TCG IDs are like "xy1-1"
		card, err = h.scryfallService.GetCard(id)
		if card == nil && err == nil {
			card, err = h.pokemonService.GetCard(id)
		}
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if card == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "card not found"})
		return
	}

	// Cache the card (log error but don't fail the request)
	if err := db.Save(card).Error; err != nil {
		c.Writer.Header().Set("X-Cache-Warning", "Failed to cache card")
	}

	c.JSON(http.StatusOK, card)
}

func (h *CardHandler) IdentifyCard(c *gin.Context) {
	var req struct {
		Text          string                  `json:"text" binding:"required"`
		Game          string                  `json:"game"`
		ImageAnalysis *services.ImageAnalysis `json:"image_analysis"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Parse OCR text to extract card details, incorporating image analysis if provided
	parsed := services.ParseOCRTextWithAnalysis(req.Text, req.Game, req.ImageAnalysis)

	// Search and match cards using shared logic
	result, textMatches, grouped, err := h.searchAndMatchCards(c, parsed, req.Game, req.Text)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Include full parsing info in response for mobile app
	response := gin.H{
		"cards":       result.Cards,
		"total_count": result.TotalCount,
		"has_more":    result.HasMore,
		"parsed": gin.H{
			"card_name":            parsed.CardName,
			"card_number":          parsed.CardNumber,
			"set_total":            parsed.SetTotal,
			"set_code":             parsed.SetCode,
			"set_name":             parsed.SetName,
			"hp":                   parsed.HP,
			"rarity":               parsed.Rarity,
			"is_foil":              parsed.IsFoil,
			"foil_indicators":      parsed.FoilIndicators,
			"foil_confidence":      parsed.FoilConfidence,
			"is_first_edition":     parsed.IsFirstEdition,
			"first_ed_indicators":  parsed.FirstEdIndicators,
			"confidence":           parsed.Confidence,
			"condition_hints":      parsed.ConditionHints,
			"suggested_condition":  parsed.SuggestedCondition,
			"edge_whitening_score": parsed.EdgeWhiteningScore,
			"corner_scores":        parsed.CornerScores,
			"match_reason":         parsed.MatchReason,
			"candidate_sets":       parsed.CandidateSets,
			"detected_language":    parsed.DetectedLanguage,
			"text_matches":         textMatches,
		},
	}

	// Add grouped results for MTG 2-phase selection
	if grouped != nil {
		response["grouped"] = grouped
	}

	c.JSON(http.StatusOK, response)
}

// IdentifyCardFromImage processes an uploaded image with server-side OCR
// and returns card matches
func (h *CardHandler) IdentifyCardFromImage(c *gin.Context) {
	// Check if server OCR is available
	if !h.serverOCRService.IsAvailable() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "Server-side OCR is not available",
			"message": "Please use client-side OCR instead",
		})
		return
	}

	// Get game parameter
	game := c.PostForm("game")
	if game == "" {
		game = c.Query("game")
	}
	if game != "pokemon" && game != "mtg" {
		game = "pokemon" // Default to Pokemon
	}

	// Handle image - check both file upload and base64 JSON body
	var ocrResult *services.ServerOCRResult
	var err error

	// Try to get uploaded file
	file, err := c.FormFile("image")
	if err == nil {
		// Handle file upload
		src, err := file.Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to open uploaded file"})
			return
		}
		defer src.Close()

		// Read file content
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(src); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read uploaded file"})
			return
		}

		ocrResult, err = h.serverOCRService.ProcessImageBytes(buf.Bytes())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "OCR processing failed",
				"details": ocrResult.Error,
			})
			return
		}
	} else {
		// Try JSON body with base64 image
		var req struct {
			Image string `json:"image"` // Base64 encoded image
			Game  string `json:"game"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "No image provided",
				"message": "Upload an image file or provide base64 encoded image in JSON body",
			})
			return
		}

		if req.Game != "" {
			game = req.Game
		}

		ocrResult, err = h.serverOCRService.ProcessBase64Image(req.Image)
		// Note: when using base64 JSON body, we don't currently keep the decoded bytes for set identification.
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "OCR processing failed",
				"details": ocrResult.Error,
			})
			return
		}
	}

	// Parse the OCR text
	text := strings.Join(ocrResult.Lines, "\n")
	parsed := services.ParseOCRText(text, game)

	// Search and match cards using shared logic
	result, textMatches, grouped, err := h.searchAndMatchCards(c, parsed, game, text)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Return results
	response := gin.H{
		"cards":       result.Cards,
		"total_count": result.TotalCount,
		"has_more":    result.HasMore,
		"ocr": gin.H{
			"text":       text,
			"lines":      ocrResult.Lines,
			"confidence": ocrResult.Confidence,
		},
		"parsed": gin.H{
			"card_name":            parsed.CardName,
			"card_number":          parsed.CardNumber,
			"set_total":            parsed.SetTotal,
			"set_code":             parsed.SetCode,
			"set_name":             parsed.SetName,
			"hp":                   parsed.HP,
			"rarity":               parsed.Rarity,
			"is_foil":              parsed.IsFoil,
			"foil_indicators":      parsed.FoilIndicators,
			"foil_confidence":      parsed.FoilConfidence,
			"is_first_edition":     parsed.IsFirstEdition,
			"first_ed_indicators":  parsed.FirstEdIndicators,
			"confidence":           parsed.Confidence,
			"condition_hints":      parsed.ConditionHints,
			"suggested_condition":  parsed.SuggestedCondition,
			"edge_whitening_score": parsed.EdgeWhiteningScore,
			"corner_scores":        parsed.CornerScores,
			"match_reason":         parsed.MatchReason,
			"candidate_sets":       parsed.CandidateSets,
			"detected_language":    parsed.DetectedLanguage,
			"text_matches":         textMatches,
		},
	}

	// Add grouped results for MTG 2-phase selection
	if grouped != nil {
		response["grouped"] = grouped
	}

	c.JSON(http.StatusOK, response)
}

// GetOCRStatus returns the status of server-side OCR capability
func (h *CardHandler) GetOCRStatus(c *gin.Context) {
	ocrAvailable := h.serverOCRService.IsAvailable()

	c.JSON(http.StatusOK, gin.H{
		"server_ocr_available": ocrAvailable,
		"message": func() string {
			if ocrAvailable {
				return "Server-side OCR is available. You can upload images for processing."
			}
			return "Server-side OCR is not available. Please use client-side OCR."
		}(),
	})
}

// searchAndMatchCards performs card search and matching based on OCR results.
// This is shared logic between IdentifyCard and IdentifyCardFromImage.
// Returns: result, textMatches (fields that matched for top result), grouped (for MTG 2-phase), error
func (h *CardHandler) searchAndMatchCards(c *gin.Context, parsed *services.OCRResult, game, fallbackText string) (*models.CardSearchResult, []string, *models.MTGGroupedResult, error) {
	// Determine search query (use card name or fall back to raw text)
	searchQuery := parsed.CardName
	if searchQuery == "" {
		searchQuery = fallbackText
	}

	// Search using the extracted text
	var result *models.CardSearchResult
	var textMatches []string
	var grouped *models.MTGGroupedResult
	var err error

	if game == "pokemon" {
		// Use full-text matching when we have substantial OCR text
		// This matches against card name, attacks, abilities, and flavor text
		if len(fallbackText) >= 20 {
			result, textMatches = h.pokemonService.MatchByFullText(
				fallbackText,
				parsed.CandidateSets,
			)
			// If full-text matching found good results, use them
			// Otherwise fall back to traditional search
			if result == nil || len(result.Cards) == 0 {
				result = nil // Reset to trigger fallback
				textMatches = nil
			}
		}

		// Fallback: If we have candidate sets from set total inference, use targeted search
		if result == nil && len(parsed.CandidateSets) > 0 && parsed.CardName != "" {
			result = h.pokemonService.SearchByNameAndNumber(
				parsed.CardName,
				parsed.CardNumber,
				parsed.CandidateSets,
			)
		}

		// Fallback: Standard name-based search
		if result == nil {
			result, err = h.pokemonService.SearchCards(searchQuery)
		}

		// If we have a set code and card number, try to find the exact card
		if result != nil && parsed.SetCode != "" && parsed.CardNumber != "" {
			exactCard := h.pokemonService.GetCardBySetAndNumber(
				strings.ToLower(parsed.SetCode),
				parsed.CardNumber,
			)
			if exactCard != nil {
				// Put exact match at the front (if not already there)
				if len(result.Cards) == 0 || result.Cards[0].ID != exactCard.ID {
					result.Cards = append([]models.Card{*exactCard}, result.Cards...)
				}
			}
		}
	} else {
		// MTG handling - 2-phase grouped selection
		var exactCard *models.Card

		// Step 1: Try exact lookup if we have set code AND collector number
		if parsed.SetCode != "" && parsed.CardNumber != "" {
			exactCard, _ = h.scryfallService.GetCardBySetAndNumber(
				parsed.SetCode, parsed.CardNumber)
		}

		// Step 2: Search for all printings by name to get all variants
		if searchQuery != "" {
			result, err = h.scryfallService.SearchCardPrintings(searchQuery)
		}

		// Step 3: If no results from printings search, try regular search
		// Preserve original error if both searches fail
		if (result == nil || len(result.Cards) == 0) && searchQuery != "" {
			originalErr := err
			result, err = h.scryfallService.SearchCards(searchQuery)
			// If fallback also failed, prefer the original error for debugging
			if err != nil && originalErr != nil {
				err = originalErr
			}
		}

		// Step 4: If exact match found, ensure it's in results
		if exactCard != nil {
			if result == nil {
				result = &models.CardSearchResult{
					Cards:      []models.Card{},
					TotalCount: 0,
					HasMore:    false,
				}
			}

			found := false
			for _, c := range result.Cards {
				if c.ID == exactCard.ID {
					found = true
					break
				}
			}
			if !found {
				result.Cards = append([]models.Card{*exactCard}, result.Cards...)
				// Keep TotalCount semantics as returned by Scryfall. We are prepending
				// an already-known card to improve the top match, not changing the
				// underlying query's total.
			}
		}

		// Step 5: Group results by set for 2-phase UI
		if result != nil && len(result.Cards) > 0 {
			grouped = services.GroupCardsBySet(result.Cards, parsed.SetCode, parsed.CardNumber, parsed.SetTotal, parsed.CopyrightYear)
		}
	}

	if err != nil {
		return nil, nil, nil, err
	}

	// Ensure handlers never see a nil result (prevents panics in IdentifyCard).
	// For MTG, this can happen when OCR yields no searchable text and exact lookup
	// does not return a card.
	if result == nil {
		result = &models.CardSearchResult{
			Cards:      []models.Card{},
			TotalCount: 0,
			HasMore:    false,
		}
	}

	// Filter and rank results based on parsed OCR data
	if len(result.Cards) > 0 {
		result.Cards = rankCardMatches(result.Cards, parsed)
	}

	// Cache cards in database (log errors but don't fail the request)
	db := database.GetDB()
	for i := range result.Cards {
		if err := db.Save(&result.Cards[i]).Error; err != nil {
			// Log the error but continue - caching failure shouldn't fail identification
			c.Writer.Header().Set("X-Cache-Warning", "Some cards failed to cache")
		}
	}

	return result, textMatches, grouped, nil
}

// rankCardMatches reorders cards based on how well they match the OCR data
func rankCardMatches(cards []models.Card, parsed *services.OCRResult) []models.Card {
	if parsed.CardNumber == "" && parsed.SetCode == "" {
		return cards // No additional info to filter on
	}

	type scoredCard struct {
		card  models.Card
		score int
	}

	scored := make([]scoredCard, len(cards))
	for i, card := range cards {
		score := 0

		// Exact card number match is highest priority
		if parsed.CardNumber != "" && card.CardNumber == parsed.CardNumber {
			score += 100
		}

		// Set code match
		if parsed.SetCode != "" {
			setCodeUpper := strings.ToUpper(card.SetCode)
			if strings.Contains(setCodeUpper, parsed.SetCode) {
				score += 50
			}
		}

		// Partial card number match (handles leading zeros)
		if parsed.CardNumber != "" {
			cardNum := strings.TrimLeft(card.CardNumber, "0")
			parsedNum := strings.TrimLeft(parsed.CardNumber, "0")
			if cardNum == parsedNum {
				score += 80
			}
		}

		scored[i] = scoredCard{card: card, score: score}
	}

	// Sort by score descending using standard library
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Extract sorted cards
	result := make([]models.Card, len(scored))
	for i, sc := range scored {
		result[i] = sc.card
	}

	return result
}
