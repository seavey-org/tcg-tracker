package handlers

import (
	"bytes"
	"log"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/codyseavey/tcg-tracker/backend/internal/database"
	"github.com/codyseavey/tcg-tracker/backend/internal/models"
	"github.com/codyseavey/tcg-tracker/backend/internal/services"
)

type CardHandler struct {
	scryfallService    *services.ScryfallService
	pokemonService     *services.PokemonHybridService
	serverOCRService   *services.ServerOCRService
	translationService *services.HybridTranslationService
}

func NewCardHandler(scryfall *services.ScryfallService, pokemon *services.PokemonHybridService, translation *services.HybridTranslationService) *CardHandler {
	return &CardHandler{
		scryfallService:    scryfall,
		pokemonService:     pokemon,
		serverOCRService:   services.NewServerOCRService(),
		translationService: translation,
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

	// Cache cards in database asynchronously (don't block the response)
	if len(result.Cards) > 0 {
		// Copy cards slice for async goroutine (avoid data race with response)
		cardsToCache := make([]models.Card, len(result.Cards))
		copy(cardsToCache, result.Cards)
		go func(cards []models.Card) {
			db := database.GetDB()
			if err := db.Save(&cards).Error; err != nil {
				log.Printf("Warning: failed to cache %d cards: %v", len(cards), err)
			}
		}(cardsToCache)
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

	// Cache the card asynchronously (don't block the response)
	go func(cardToCache models.Card) {
		if err := db.Save(&cardToCache).Error; err != nil {
			log.Printf("Warning: failed to cache card %s: %v", cardToCache.ID, err)
		}
	}(*card)

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
			"translation_source":   parsed.TranslationSource,
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
// and returns card matches. For Japanese Pokemon cards, it uses Gemini vision
// for more accurate identification instead of OCR + text matching.
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
	var imageBytes []byte
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
		imageBytes = buf.Bytes()

		ocrResult, err = h.serverOCRService.ProcessImageBytes(imageBytes)
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

	// For Japanese Pokemon cards, use a "static translation first" approach:
	// 1. If OCR parser found a card name via static translation, use standard flow (fast)
	// 2. Only use Gemini vision if static translation didn't find a match (slower but handles unknown cards)
	if game == "pokemon" && parsed.DetectedLanguage == "Japanese" &&
		h.translationService != nil && h.translationService.IsGeminiEnabled() && len(imageBytes) > 0 {

		// Check if static translation already found a card name
		if parsed.CardName != "" {
			log.Printf("Japanese card: static translation found %q, skipping Gemini", parsed.CardName)
			// Continue to standard flow below
		} else {
			// No static translation match - try Gemini vision
			result, textMatches, err := h.identifyJapaneseCardFromImage(c, imageBytes, parsed, text)
			if err == nil && result != nil && len(result.Cards) > 0 {
				// Success with Gemini vision - return results
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
						"translation_source":   parsed.TranslationSource,
						"text_matches":         textMatches,
					},
				}
				c.JSON(http.StatusOK, response)
				return
			}
			// Fall through to standard OCR flow if Gemini vision fails
			log.Printf("Gemini vision failed for Japanese card, falling back to OCR: %v", err)
		}
	}

	// Standard OCR flow for non-Japanese cards or when Gemini vision fails
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
			"translation_source":   parsed.TranslationSource,
			"text_matches":         textMatches,
		},
	}

	// Add grouped results for MTG 2-phase selection
	if grouped != nil {
		response["grouped"] = grouped
	}

	c.JSON(http.StatusOK, response)
}

// identifyJapaneseCardFromImage uses Gemini vision to identify a Japanese Pokemon card
// from an image, then searches for the card in the database.
func (h *CardHandler) identifyJapaneseCardFromImage(
	c *gin.Context,
	imageBytes []byte,
	parsed *services.OCRResult,
	ocrText string,
) (*models.CardSearchResult, []string, error) {
	// Use Gemini vision to identify the card
	translationResult, err := h.translationService.IdentifyFromImage(c.Request.Context(), imageBytes, "image/jpeg")
	if err != nil {
		return nil, nil, err
	}

	// Update parsed with translation info
	parsed.TranslationSource = translationResult.Source

	// Get the card name from Gemini
	cardName := translationResult.TranslatedText
	if cardName == "" {
		return nil, nil, nil
	}

	// Get additional hints from Gemini candidates (set code, card number)
	var setCode, cardNumber string
	if len(translationResult.Candidates) > 0 {
		candidate := translationResult.Candidates[0]
		setCode = candidate.SetCode
		cardNumber = candidate.CardNumber
	}

	log.Printf("Gemini vision identified Japanese card: name=%q, set=%q, num=%q",
		cardName, setCode, cardNumber)

	// Priority 1: If Gemini gave us set code and card number, try exact lookup
	if setCode != "" && cardNumber != "" {
		exactCard := h.pokemonService.GetCardBySetAndNumber(
			strings.ToLower(setCode),
			cardNumber,
		)
		if exactCard != nil {
			log.Printf("Gemini exact match: %s (%s #%s)", exactCard.Name, exactCard.SetCode, exactCard.CardNumber)
			return &models.CardSearchResult{
				Cards:      []models.Card{*exactCard},
				TotalCount: 1,
				HasMore:    false,
			}, nil, nil
		}
	}

	// Priority 2: Search by card name (Gemini gives us English name)
	result, err := h.pokemonService.SearchCards(cardName)
	if err != nil {
		return nil, nil, err
	}

	// If we got results, rank them by any additional OCR hints we have
	if result != nil && len(result.Cards) > 0 {
		// Use OCR-extracted set code/number if Gemini didn't provide them
		if parsed.SetCode != "" && setCode == "" {
			setCode = parsed.SetCode
		}
		if parsed.CardNumber != "" && cardNumber == "" {
			cardNumber = parsed.CardNumber
		}

		// Rank cards by how well they match the hints
		if setCode != "" || cardNumber != "" {
			result.Cards = rankCardMatchesWithHints(result.Cards, setCode, cardNumber)
		}

		log.Printf("Gemini vision matched %d cards for %q", len(result.Cards), cardName)
	}

	return result, nil, nil
}

// rankCardMatchesWithHints reorders cards based on set code and card number hints
func rankCardMatchesWithHints(cards []models.Card, setCode, cardNumber string) []models.Card {
	if setCode == "" && cardNumber == "" {
		return cards
	}

	type scoredCard struct {
		card  models.Card
		score int
	}

	scored := make([]scoredCard, len(cards))
	setCodeLower := strings.ToLower(setCode)

	for i, card := range cards {
		score := 0

		// Exact set code match
		if setCode != "" && strings.ToLower(card.SetCode) == setCodeLower {
			score += 100
		}

		// Exact card number match
		if cardNumber != "" && card.CardNumber == cardNumber {
			score += 50
		}

		// Partial card number match (handles leading zeros)
		if cardNumber != "" {
			cardNum := strings.TrimLeft(card.CardNumber, "0")
			parsedNum := strings.TrimLeft(cardNumber, "0")
			if cardNum == parsedNum {
				score += 40
			}
		}

		scored[i] = scoredCard{card: card, score: score}
	}

	// Sort by score descending
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
	var exactCard *models.Card // Reused for exact match preservation after ranking

	if game == "pokemon" {
		// Priority 1: If we have a set code and card number, try exact lookup first
		// This is especially important for Japanese cards where we can't extract the card name
		// but can reliably extract the set code and card number
		if parsed.SetCode != "" && parsed.CardNumber != "" {
			exactCard = h.pokemonService.GetCardBySetAndNumber(
				strings.ToLower(parsed.SetCode),
				parsed.CardNumber,
			)
		}

		// Priority 2: If no card name was extracted (common with Japanese cards),
		// and we found an exact match, use it as the primary result
		if parsed.CardName == "" && exactCard != nil {
			result = &models.CardSearchResult{
				Cards:      []models.Card{*exactCard},
				TotalCount: 1,
				HasMore:    false,
			}
		}

		// Priority 2.5: For vintage Japanese cards with Pokedex number but no card name/set,
		// search by National Pokedex number to find matching Pokemon
		if result == nil && parsed.PokedexNumber > 0 && parsed.CardName == "" {
			result = h.pokemonService.SearchByPokedexNumber(parsed.PokedexNumber)
		}

		// Priority 3: Use full-text matching when we have substantial OCR text
		// This matches against card name, attacks, abilities, and flavor text
		// BUT require a minimum confidence score to accept results
		if result == nil && len(fallbackText) >= 20 {
			result, textMatches = h.pokemonService.MatchByFullText(
				fallbackText,
				parsed.CandidateSets,
			)

			// Score threshold: require at least a partial name match (500) to accept
			// Score reference: name_exact=1000, name_partial=500, attack=200, number=300
			const minAcceptScore = 500
			if result != nil && len(result.Cards) > 0 {
				topScore := result.TopScore
				topCard := result.Cards[0].Name

				if topScore < minAcceptScore {
					// Full-text match is low confidence
					if exactCard != nil {
						// Prefer exact set+number match over low-confidence full-text
						log.Printf("CardMatch: full-text score too low (%d < %d) for %q, using exactCard %q instead",
							topScore, minAcceptScore, topCard, exactCard.Name)
						result = &models.CardSearchResult{
							Cards:      []models.Card{*exactCard},
							TotalCount: 1,
							HasMore:    false,
						}
						textMatches = nil
					} else {
						// No exact card, reset to try other methods
						log.Printf("CardMatch: full-text score too low (%d < %d) for %q, trying other methods",
							topScore, minAcceptScore, topCard)
						result = nil
						textMatches = nil
					}
				}
			} else {
				result = nil // Reset to trigger fallback
				textMatches = nil
			}
		}

		// Priority 4: If we have candidate sets from set total inference, use targeted search
		if result == nil && len(parsed.CandidateSets) > 0 && parsed.CardName != "" {
			result = h.pokemonService.SearchByNameAndNumber(
				parsed.CardName,
				parsed.CardNumber,
				parsed.CandidateSets,
			)
		}

		// Priority 5: Standard name-based search (only if we have a valid card name)
		if result == nil && parsed.CardName != "" {
			result, err = h.pokemonService.SearchCards(searchQuery)
		}

		// Priority 6: Last resort - if we still have no results and have an exact card, use it
		if (result == nil || len(result.Cards) == 0) && exactCard != nil {
			result = &models.CardSearchResult{
				Cards:      []models.Card{*exactCard},
				TotalCount: 1,
				HasMore:    false,
			}
		}

		// Ensure exact match is at the front of results (if we have other results too)
		if result != nil && len(result.Cards) > 0 && exactCard != nil {
			if result.Cards[0].ID != exactCard.ID {
				// Check if exact card is already in results
				alreadyInResults := false
				for i, card := range result.Cards {
					if card.ID == exactCard.ID {
						// Move it to the front
						result.Cards = append([]models.Card{card}, append(result.Cards[:i], result.Cards[i+1:]...)...)
						alreadyInResults = true
						break
					}
				}
				if !alreadyInResults {
					result.Cards = append([]models.Card{*exactCard}, result.Cards...)
				}
			}
		}

		// Priority 7: Translation fallback for Japanese cards with low confidence matches.
		// Trigger translation when:
		// 1. No results found (result is nil or empty)
		// 2. TopScore == 0 (matched via name search, not full-text - suspicious for Japanese)
		// 3. TopScore > 0 but < threshold (low confidence full-text match)
		// 4. Matched card name is very short (1-3 chars) - likely OCR garbage like "N", "Eri"
		shouldTranslate := false
		if h.translationService != nil && parsed.DetectedLanguage == "Japanese" {
			if result == nil || len(result.Cards) == 0 {
				// No match at all
				shouldTranslate = true
			} else if result.TopScore == 0 {
				// Matched via name search (no score) - suspicious for Japanese cards
				// especially if the matched name is very short (likely OCR garbage)
				matchedName := result.Cards[0].Name
				if len(matchedName) <= 3 {
					shouldTranslate = true
					log.Printf("CardMatch: suspicious short name match %q for Japanese card, triggering translation", matchedName)
				}
			} else if result.TopScore > 0 && result.TopScore < h.translationService.GetConfidenceThreshold() {
				// Low confidence full-text match
				shouldTranslate = true
			}
		}
		if shouldTranslate {
			// Get current score (0 if no result)
			currentScore := 0
			if result != nil {
				currentScore = result.TopScore
			}
			translationResult, _ := h.translationService.TranslateForMatching(
				c.Request.Context(),
				fallbackText,
				parsed.DetectedLanguage,
				currentScore,
			)

			// If translation was performed (API or static map changed the text), retry matching
			if translationResult != nil && translationResult.TranslatedText != fallbackText {
				translatedText := translationResult.TranslatedText
				translatedParsed := services.ParseOCRText(translatedText, game)

				// Use set info from Gemini candidates if available
				if len(translationResult.Candidates) > 0 {
					for _, candidate := range translationResult.Candidates {
						if candidate.SetCode != "" && !contains(parsed.CandidateSets, candidate.SetCode) {
							parsed.CandidateSets = append(parsed.CandidateSets, candidate.SetCode)
						}
						if candidate.CardNumber != "" && parsed.CardNumber == "" {
							parsed.CardNumber = candidate.CardNumber
						}
					}
				}

				// Try full-text matching with translated text
				if len(translatedText) >= 20 {
					translatedResult, translatedMatches := h.pokemonService.MatchByFullText(
						translatedText,
						parsed.CandidateSets, // Use original + Gemini hints
					)

					// Use translated result if it's better than current (or if we had no result)
					if translatedResult != nil && len(translatedResult.Cards) > 0 && translatedResult.TopScore > currentScore {
						result = translatedResult
						textMatches = translatedMatches
						parsed.TranslationSource = translationResult.Source
					}
				}

				// Name-based search on translated name only if we still have nothing
				if (result == nil || len(result.Cards) == 0) && translatedParsed.CardName != "" {
					nameResult, _ := h.pokemonService.SearchCards(translatedParsed.CardName)
					if nameResult != nil && len(nameResult.Cards) > 0 {
						result = nameResult
						parsed.TranslationSource = translationResult.Source
					}
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

	// For Pokemon: preserve exact set+number match at the top even after ranking.
	// Reuse exactCard from Priority 1 lookup to avoid duplicate database/search call.
	if game == "pokemon" && exactCard != nil && len(result.Cards) > 0 && result.Cards[0].ID != exactCard.ID {
		for i, card := range result.Cards {
			if card.ID == exactCard.ID {
				result.Cards = append([]models.Card{card}, append(result.Cards[:i], result.Cards[i+1:]...)...)
				break
			}
		}
	}

	// Cache cards in database asynchronously (don't block the response)
	if len(result.Cards) > 0 {
		// Copy cards slice for async goroutine (avoid data race)
		cardsToCache := make([]models.Card, len(result.Cards))
		copy(cardsToCache, result.Cards)
		go func(cards []models.Card) {
			db := database.GetDB()
			if err := db.Save(&cards).Error; err != nil {
				log.Printf("Warning: failed to cache %d identified cards: %v", len(cards), err)
			}
		}(cardsToCache)
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

// contains checks if a string slice contains a value
func contains(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
