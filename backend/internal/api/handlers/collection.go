package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/codyseavey/tcg-tracker/backend/internal/database"
	"github.com/codyseavey/tcg-tracker/backend/internal/models"
	"github.com/codyseavey/tcg-tracker/backend/internal/services"
	"github.com/gin-gonic/gin"
)

type CollectionHandler struct {
	scryfallService *services.ScryfallService
	pokemonService  *services.PokemonHybridService
}

func NewCollectionHandler(scryfall *services.ScryfallService, pokemon *services.PokemonHybridService) *CollectionHandler {
	return &CollectionHandler{
		scryfallService: scryfall,
		pokemonService:  pokemon,
	}
}

func (h *CollectionHandler) GetCollection(c *gin.Context) {
	db := database.GetDB()

	var items []models.CollectionItem
	query := db.Preload("Card").Order("added_at DESC")

	// Optional filters
	if game := c.Query("game"); game != "" {
		query = query.Joins("JOIN cards ON cards.id = collection_items.card_id").
			Where("cards.game = ?", game)
	}

	if err := query.Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, items)
}

func (h *CollectionHandler) AddToCollection(c *gin.Context) {
	var req models.AddToCollectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db := database.GetDB()

	// Verify card exists in cache or fetch it
	var card models.Card
	if err := db.First(&card, "id = ?", req.CardID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "card not found, please search for it first"})
		return
	}

	// Set defaults
	quantity := req.Quantity
	if quantity == 0 {
		quantity = 1
	}
	condition := req.Condition
	if condition == "" {
		condition = models.ConditionNearMint
	}

	// Check if already in collection (same card, condition, foil)
	var existingItem models.CollectionItem
	err := db.Where("card_id = ? AND condition = ? AND foil = ?", req.CardID, condition, req.Foil).
		First(&existingItem).Error

	if err == nil {
		// Update quantity
		existingItem.Quantity += quantity
		if err := db.Save(&existingItem).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		db.Preload("Card").First(&existingItem, existingItem.ID)
		c.JSON(http.StatusOK, existingItem)
		return
	}

	// Create new collection item
	item := models.CollectionItem{
		CardID:    req.CardID,
		Quantity:  quantity,
		Condition: condition,
		Foil:      req.Foil,
		Notes:     req.Notes,
		AddedAt:   time.Now(),
	}

	if err := db.Create(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Load the card relationship
	db.Preload("Card").First(&item, item.ID)

	c.JSON(http.StatusCreated, item)
}

func (h *CollectionHandler) UpdateCollectionItem(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req models.UpdateCollectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db := database.GetDB()

	var item models.CollectionItem
	if err := db.First(&item, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "item not found"})
		return
	}

	// Update fields if provided
	if req.Quantity != nil {
		item.Quantity = *req.Quantity
	}
	if req.Condition != nil {
		item.Condition = *req.Condition
	}
	if req.Foil != nil {
		item.Foil = *req.Foil
	}
	if req.Notes != nil {
		item.Notes = *req.Notes
	}

	if err := db.Save(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	db.Preload("Card").First(&item, item.ID)
	c.JSON(http.StatusOK, item)
}

func (h *CollectionHandler) DeleteCollectionItem(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	db := database.GetDB()

	result := db.Delete(&models.CollectionItem{}, id)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "item not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *CollectionHandler) GetStats(c *gin.Context) {
	db := database.GetDB()

	var stats models.CollectionStats

	// Total and unique cards
	db.Model(&models.CollectionItem{}).Select("COALESCE(SUM(quantity), 0)").Scan(&stats.TotalCards)
	var uniqueCount int64
	db.Model(&models.CollectionItem{}).Distinct("card_id").Count(&uniqueCount)
	stats.UniqueCards = int(uniqueCount)

	// Calculate values and counts by game
	type gameStats struct {
		Game       string
		Count      int
		TotalValue float64
	}

	var gameResults []gameStats
	db.Table("collection_items").
		Select("cards.game, SUM(collection_items.quantity) as count, SUM(CASE WHEN collection_items.foil THEN cards.price_foil_usd ELSE cards.price_usd END * collection_items.quantity) as total_value").
		Joins("JOIN cards ON cards.id = collection_items.card_id").
		Group("cards.game").
		Scan(&gameResults)

	for _, gr := range gameResults {
		switch gr.Game {
		case "mtg":
			stats.MTGCards = gr.Count
			stats.MTGValue = gr.TotalValue
		case "pokemon":
			stats.PokemonCards = gr.Count
			stats.PokemonValue = gr.TotalValue
		}
	}

	stats.TotalValue = stats.MTGValue + stats.PokemonValue

	c.JSON(http.StatusOK, stats)
}

func (h *CollectionHandler) RefreshPrices(c *gin.Context) {
	db := database.GetDB()

	var cards []models.Card
	if err := db.Find(&cards).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	updated := 0
	for _, card := range cards {
		var updatedCard *models.Card
		var err error

		switch card.Game {
		case models.GameMTG:
			updatedCard, err = h.scryfallService.GetCard(card.ID)
		case models.GamePokemon:
			updatedCard, err = h.pokemonService.GetCard(card.ID)
		}

		if err != nil || updatedCard == nil {
			continue
		}

		card.PriceUSD = updatedCard.PriceUSD
		card.PriceFoilUSD = updatedCard.PriceFoilUSD
		card.PriceUpdatedAt = updatedCard.PriceUpdatedAt

		if err := db.Save(&card).Error; err == nil {
			updated++
		}
	}

	c.JSON(http.StatusOK, gin.H{"updated": updated})
}
