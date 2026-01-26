package handlers

import (
	"bytes"
	"encoding/base64"
	"log"
	"net/http"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"

	"github.com/codyseavey/tcg-tracker/backend/internal/database"
	"github.com/codyseavey/tcg-tracker/backend/internal/models"
	"github.com/codyseavey/tcg-tracker/backend/internal/services"
)

// cardNameMatches checks if two card names are equivalent for validation purposes.
// Handles variations like "Poké Ball" vs "Poke Ball", accented characters, and case differences.
func cardNameMatches(resolvedName, expectedName string) bool {
	// Normalize both names: lowercase, remove accents, strip non-alphanumeric
	normalize := func(s string) string {
		// Simple accent removal by checking for common variants
		// é -> e, ü -> u, etc.
		var sb strings.Builder
		for _, r := range strings.ToLower(s) {
			switch {
			case r >= 'a' && r <= 'z':
				sb.WriteRune(r)
			case r >= '0' && r <= '9':
				sb.WriteRune(r)
			case unicode.Is(unicode.Mn, r):
				// Skip combining marks (accents)
				continue
			default:
				// Map accented chars to base chars
				switch r {
				case 'é', 'è', 'ê', 'ë':
					sb.WriteRune('e')
				case 'á', 'à', 'â', 'ä', 'ã':
					sb.WriteRune('a')
				case 'í', 'ì', 'î', 'ï':
					sb.WriteRune('i')
				case 'ó', 'ò', 'ô', 'ö', 'õ':
					sb.WriteRune('o')
				case 'ú', 'ù', 'û', 'ü':
					sb.WriteRune('u')
				case 'ñ':
					sb.WriteRune('n')
				case 'ç':
					sb.WriteRune('c')
					// Skip other non-alphanumeric chars (spaces, punctuation)
				}
			}
		}
		return sb.String()
	}

	return normalize(resolvedName) == normalize(expectedName)
}

type CardHandler struct {
	scryfallService *services.ScryfallService
	pokemonService  *services.PokemonHybridService
	geminiService   *services.GeminiService
}

// cacheCardsAsync saves cards to the database asynchronously so they can be
// referenced when adding to collection. This is needed because Pokemon cards
// come from local JSON files and must be cached in SQLite for collection lookups.
func cacheCardsAsync(cards []models.Card) {
	if len(cards) == 0 {
		return
	}
	// Copy cards slice to avoid data race with response serialization
	cardsToCache := make([]models.Card, len(cards))
	copy(cardsToCache, cards)
	go func(cards []models.Card) {
		db := database.GetDB()
		if err := db.Save(&cards).Error; err != nil {
			log.Printf("Warning: failed to cache %d cards: %v", len(cards), err)
		}
	}(cardsToCache)
}

func NewCardHandler(scryfall *services.ScryfallService, pokemon *services.PokemonHybridService, gemini *services.GeminiService) *CardHandler {
	return &CardHandler{
		scryfallService: scryfall,
		pokemonService:  pokemon,
		geminiService:   gemini,
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

	// Cache cards so they can be added to collection
	cacheCardsAsync(result.Cards)

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

// ListSets returns sets matching the query, with symbol URLs for display.
// GET /api/sets?q=<query>&game=<pokemon|mtg>
func (h *CardHandler) ListSets(c *gin.Context) {
	query := c.Query("q")
	game := c.Query("game")

	var sets []services.SetInfo
	var err error

	switch game {
	case "mtg":
		sets, err = h.scryfallService.ListAllSets(c.Request.Context(), query)
	case "pokemon":
		sets = h.pokemonService.ListAllSets(query)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "game parameter must be 'mtg' or 'pokemon'"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"sets": sets})
}

// GetSetCards returns all cards in a specific set, optionally filtered by name.
// GET /api/sets/:setCode/cards?game=<pokemon|mtg>&q=<name_filter>
func (h *CardHandler) GetSetCards(c *gin.Context) {
	setCode := c.Param("setCode")
	game := c.Query("game")
	nameFilter := c.Query("q")

	if setCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "setCode parameter is required"})
		return
	}

	var result *models.CardSearchResult
	var err error

	switch game {
	case "mtg":
		result, err = h.scryfallService.GetSetCards(setCode, nameFilter)
	case "pokemon":
		result, err = h.pokemonService.GetSetCards(setCode, nameFilter)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "game parameter must be 'mtg' or 'pokemon'"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Cache cards so they can be added to collection
	cacheCardsAsync(result.Cards)

	c.JSON(http.StatusOK, result)
}

// SearchCardsGrouped searches for cards by name and groups results by set.
// GET /api/cards/search/grouped?q=<name>&game=<pokemon|mtg>
func (h *CardHandler) SearchCardsGrouped(c *gin.Context) {
	query := c.Query("q")
	game := c.Query("game")

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter 'q' is required"})
		return
	}

	var result *models.GroupedSearchResult
	var err error

	switch game {
	case "mtg":
		result, err = h.scryfallService.SearchCardsGrouped(c.Request.Context(), query)
	case "pokemon":
		result, err = h.pokemonService.SearchCardsGrouped(query)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "game parameter must be 'mtg' or 'pokemon'"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Cache all cards so they can be added to collection
	for _, group := range result.SetGroups {
		cacheCardsAsync(group.Cards)
	}

	c.JSON(http.StatusOK, result)
}

// IdentifyCardFromImage uses Gemini Vision with function calling to identify
// a trading card from an uploaded image. Gemini can search for cards and
// compare images to find the exact match.
func (h *CardHandler) IdentifyCardFromImage(c *gin.Context) {
	// Check if Gemini is available
	if h.geminiService == nil || !h.geminiService.IsEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "Card identification is not available",
			"message": "Gemini API key not configured",
		})
		return
	}

	// Handle image - check both file upload and base64 JSON body
	var imageBytes []byte
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
	} else {
		// Try JSON body with base64 image
		var req struct {
			Image string `json:"image"` // Base64 encoded image
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "No image provided",
				"message": "Upload an image file or provide base64 encoded image in JSON body",
			})
			return
		}

		// Decode base64 to get raw bytes
		imageBytes, err = base64.StdEncoding.DecodeString(req.Image)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid base64 image data"})
			return
		}
	}

	// Use Gemini to identify the card
	result, err := h.geminiService.IdentifyCard(
		c.Request.Context(),
		imageBytes,
		h.pokemonService,  // implements CardSearcher
		h.scryfallService, // implements CardSearcher
	)
	if err != nil {
		log.Printf("Gemini identification failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Card identification failed",
			"details": err.Error(),
		})
		return
	}

	// Helper to resolve a card by ID, trying both services if game is unknown
	resolveCard := func(cardID, game string) *models.Card {
		if game == "pokemon" {
			return h.pokemonService.GetCardByID(cardID)
		} else if game == "mtg" {
			card, _ := h.scryfallService.GetCard(cardID)
			return card
		}
		// game is unknown - try both
		if card := h.pokemonService.GetCardByID(cardID); card != nil {
			return card
		}
		if card, _ := h.scryfallService.GetCard(cardID); card != nil {
			return card
		}
		return nil
	}

	// Validate that card_id matches the identified name to catch Gemini hallucinations
	// e.g., Gemini says "Poké Ball #96" but returns card_id for "Double Colorless Energy #96"
	validatedCardID := result.CardID
	if result.CardID != "" && result.CanonicalNameEN != "" {
		if resolvedCard := resolveCard(result.CardID, result.Game); resolvedCard != nil {
			if !cardNameMatches(resolvedCard.Name, result.CanonicalNameEN) {
				log.Printf("Gemini card_id mismatch: identified '%s' but card_id '%s' resolves to '%s'",
					result.CanonicalNameEN, result.CardID, resolvedCard.Name)
				// Clear the card_id since it doesn't match - we'll search by name instead
				validatedCardID = ""
			}
		}
	}

	// Build response with all Gemini fields (use validated card_id)
	response := gin.H{
		"card_id":           validatedCardID,
		"card_name":         result.CardName,
		"canonical_name_en": result.CanonicalNameEN,
		"set_code":          result.SetCode,
		"set_name":          result.SetName,
		"card_number":       result.Number,
		"game":              result.Game,
		"observed_language": result.ObservedLang,
		"confidence":        result.Confidence,
		"reasoning":         result.Reasoning,
		"turns_used":        result.TurnsUsed,
	}

	// Always build a cards array for the client to display
	var cards []models.Card

	// If we have a validated card ID, fetch the primary match and put it first
	if validatedCardID != "" {
		if card := resolveCard(validatedCardID, result.Game); card != nil {
			cards = append(cards, *card)
		}
	}

	// Build list of search terms to try: canonical name first, then fallback to Gemini's search terms
	var searchTermsToTry []string
	if result.CanonicalNameEN != "" {
		searchTermsToTry = append(searchTermsToTry, result.CanonicalNameEN)
	}
	// Add Gemini's search terms as fallback (these are the card names it searched during identification)
	for _, term := range result.SearchTerms {
		// Don't duplicate
		isDupe := false
		for _, existing := range searchTermsToTry {
			if existing == term {
				isDupe = true
				break
			}
		}
		if !isDupe {
			searchTermsToTry = append(searchTermsToTry, term)
		}
	}

	// Search by each term to populate candidates so user can pick the right card
	// This handles: exact match (user picks from variants), mismatch, timeout, low confidence
	for _, searchName := range searchTermsToTry {
		var searchResult *models.CardSearchResult
		var err error

		if result.Game == "pokemon" {
			searchResult, err = h.pokemonService.SearchCards(searchName)
		} else if result.Game == "mtg" {
			searchResult, err = h.scryfallService.SearchCards(searchName)
		}

		if err == nil && searchResult != nil {
			// Add search results as candidates (limit to 20 total to give user good selection)
			for _, card := range searchResult.Cards {
				if len(cards) >= 20 {
					break
				}
				// Don't add duplicates (primary match was already added above)
				isDupe := false
				for _, existing := range cards {
					if existing.ID == card.ID {
						isDupe = true
						break
					}
				}
				if !isDupe {
					cards = append(cards, card)
				}
			}
		}

		// Stop if we have enough candidates
		if len(cards) >= 20 {
			break
		}
	}

	// Also include Gemini's alternative candidates (for low confidence or user choice)
	for _, candidate := range result.Candidates {
		// Skip if already added
		isDupe := false
		for _, existing := range cards {
			if existing.ID == candidate.ID {
				isDupe = true
				break
			}
		}
		if isDupe {
			continue
		}
		if card := resolveCard(candidate.ID, result.Game); card != nil {
			cards = append(cards, *card)
		}
	}

	// Cache all resolved cards
	if len(cards) > 0 {
		cacheCardsAsync(cards)
	}

	response["cards"] = cards
	response["total_count"] = len(cards)
	response["has_more"] = false

	c.JSON(http.StatusOK, response)
}
