package handlers

import (
	"net/http"
	"strings"

	"github.com/codyseavey/tcg-tracker/backend/internal/database"
	"github.com/codyseavey/tcg-tracker/backend/internal/models"
	"github.com/codyseavey/tcg-tracker/backend/internal/services"
	"github.com/gin-gonic/gin"
)

type CardHandler struct {
	scryfallService *services.ScryfallService
	pokemonService  *services.PokemonHybridService
}

func NewCardHandler(scryfall *services.ScryfallService, pokemon *services.PokemonHybridService) *CardHandler {
	return &CardHandler{
		scryfallService: scryfall,
		pokemonService:  pokemon,
	}
}

func (h *CardHandler) SearchCards(c *gin.Context) {
	query := c.Query("q")
	game := c.Query("game")

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

	// Cache cards in database
	db := database.GetDB()
	for _, card := range result.Cards {
		db.Save(&card)
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

	// Cache the card
	db.Save(card)

	c.JSON(http.StatusOK, card)
}

func (h *CardHandler) IdentifyCard(c *gin.Context) {
	var req struct {
		Text string `json:"text" binding:"required"`
		Game string `json:"game"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Parse OCR text to extract card details
	parsed := services.ParseOCRText(req.Text, req.Game)

	// Use card name for search (fall back to raw text if no name extracted)
	searchQuery := parsed.CardName
	if searchQuery == "" {
		searchQuery = req.Text
	}

	// Search using the extracted text
	var result *models.CardSearchResult
	var err error

	if req.Game == "pokemon" {
		result, err = h.pokemonService.SearchCards(searchQuery)

		// If we have a set code and card number, try to find the exact card
		if parsed.SetCode != "" && parsed.CardNumber != "" {
			exactCard := h.pokemonService.GetCardBySetAndNumber(
				strings.ToLower(parsed.SetCode),
				parsed.CardNumber,
			)
			if exactCard != nil {
				// Put exact match at the front
				result.Cards = append([]models.Card{*exactCard}, result.Cards...)
			}
		}
	} else {
		// Default to MTG or search both
		result, err = h.scryfallService.SearchCards(searchQuery)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Filter and rank results based on parsed OCR data
	if result != nil && len(result.Cards) > 0 {
		result.Cards = rankCardMatches(result.Cards, parsed)
	}

	// Include parsing info in response for debugging
	c.JSON(http.StatusOK, gin.H{
		"cards":       result.Cards,
		"total_count": result.TotalCount,
		"has_more":    result.HasMore,
		"parsed": gin.H{
			"card_name":   parsed.CardName,
			"card_number": parsed.CardNumber,
			"set_total":   parsed.SetTotal,
			"set_code":    parsed.SetCode,
			"confidence":  parsed.Confidence,
		},
	})
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

	// Sort by score descending
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Extract sorted cards
	result := make([]models.Card, len(scored))
	for i, sc := range scored {
		result[i] = sc.card
	}

	return result
}
