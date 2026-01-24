package handlers

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/codyseavey/tcg-tracker/backend/internal/database"
	"github.com/codyseavey/tcg-tracker/backend/internal/models"
	"github.com/codyseavey/tcg-tracker/backend/internal/services"
)

type CollectionHandler struct {
	scryfallService     *services.ScryfallService
	pokemonService      *services.PokemonHybridService
	imageStorageService *services.ImageStorageService
	snapshotService     *services.SnapshotService
	priceWorker         *services.PriceWorker
}

func NewCollectionHandler(scryfall *services.ScryfallService, pokemon *services.PokemonHybridService, imageStorage *services.ImageStorageService, snapshot *services.SnapshotService, priceWorker *services.PriceWorker) *CollectionHandler {
	return &CollectionHandler{
		scryfallService:     scryfall,
		pokemonService:      pokemon,
		imageStorageService: imageStorage,
		snapshotService:     snapshot,
		priceWorker:         priceWorker,
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

// Maximum quantity allowed per collection item
const maxQuantity = 9999

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

	// Validate and set defaults
	quantity := req.Quantity
	if quantity == 0 {
		quantity = 1
	}
	if quantity < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "quantity must be positive"})
		return
	}
	if quantity > maxQuantity {
		c.JSON(http.StatusBadRequest, gin.H{"error": "quantity exceeds maximum allowed (9999)"})
		return
	}
	condition := req.Condition
	if condition == "" {
		condition = models.ConditionNearMint
	}
	printing := req.Printing
	if printing == "" {
		printing = models.PrintingNormal
	}
	language := models.NormalizeLanguage(string(req.Language))

	// Handle scanned image FIRST - if provided, we NEVER merge (each scan is a unique physical card)
	var scannedImagePath string
	hasScannedImage := false
	if req.ScannedImageData != "" && h.imageStorageService != nil {
		imageData, err := base64.StdEncoding.DecodeString(req.ScannedImageData)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid image data"})
			return
		}
		filename, err := h.imageStorageService.SaveImage(imageData)
		if err != nil {
			// Log but don't fail - image is optional
			scannedImagePath = ""
		} else {
			scannedImagePath = filename
			hasScannedImage = true
		}
	}

	// If we have a scanned image, ALWAYS create a new item (qty=1) - never merge
	// Each scanned card represents a specific physical card that needs individual tracking
	if hasScannedImage {
		item := models.CollectionItem{
			CardID:           req.CardID,
			Quantity:         1, // Always 1 for scanned cards
			Condition:        condition,
			Printing:         printing,
			Language:         language,
			Notes:            req.Notes,
			AddedAt:          time.Now(),
			ScannedImagePath: scannedImagePath,
		}

		if err := db.Create(&item).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		db.Preload("Card").First(&item, item.ID)
		c.JSON(http.StatusCreated, item)
		return
	}

	// No scanned image - try to merge into existing NON-SCANNED stack with same language
	var existingItem models.CollectionItem
	err := db.Where("card_id = ? AND condition = ? AND printing = ? AND language = ? AND (scanned_image_path IS NULL OR scanned_image_path = '')",
		req.CardID, condition, printing, language).
		First(&existingItem).Error

	if err == nil {
		// Merge into existing non-scanned stack
		existingItem.Quantity += quantity
		if err := db.Save(&existingItem).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		db.Preload("Card").First(&existingItem, existingItem.ID)
		c.JSON(http.StatusOK, existingItem)
		return
	}

	// No existing stack to merge into - create new item
	item := models.CollectionItem{
		CardID:           req.CardID,
		Quantity:         quantity,
		Condition:        condition,
		Printing:         printing,
		Language:         language,
		Notes:            req.Notes,
		AddedAt:          time.Now(),
		ScannedImagePath: "", // No scan
	}

	if err := db.Create(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

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

	// Validate quantity if provided
	if req.Quantity != nil {
		if *req.Quantity < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "quantity must be positive"})
			return
		}
		if *req.Quantity > maxQuantity {
			c.JSON(http.StatusBadRequest, gin.H{"error": "quantity exceeds maximum allowed (9999)"})
			return
		}
	}

	// Check if condition, printing, or language is changing
	// Note: normalize language before comparing since item.Language is already normalized
	conditionChanging := req.Condition != nil && *req.Condition != item.Condition
	printingChanging := req.Printing != nil && *req.Printing != item.Printing
	languageChanging := req.Language != nil && models.NormalizeLanguage(string(*req.Language)) != item.Language
	attributeChanging := conditionChanging || printingChanging || languageChanging

	// Determine the new values
	newCondition := item.Condition
	if req.Condition != nil {
		newCondition = *req.Condition
	}
	newPrinting := item.Printing
	if req.Printing != nil {
		newPrinting = *req.Printing
	}
	newLanguage := item.Language
	if req.Language != nil {
		newLanguage = models.NormalizeLanguage(string(*req.Language))
	}

	// Scanned items always stay individual - just update in place
	// They represent specific physical cards that have been visually assessed
	// Quantity is always 1 for scanned items (one scan = one physical card)
	if item.ScannedImagePath != "" {
		if req.Condition != nil {
			item.Condition = *req.Condition
		}
		if req.Printing != nil {
			item.Printing = *req.Printing
		}
		if req.Language != nil {
			item.Language = models.NormalizeLanguage(string(*req.Language))
		}
		// Quantity is intentionally not updated for scanned items
		// Each scan represents exactly one physical card
		if req.Notes != nil {
			item.Notes = *req.Notes
		}

		if err := db.Save(&item).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		db.Preload("Card").First(&item, item.ID)
		c.JSON(http.StatusOK, models.CollectionUpdateResponse{
			Item:      item,
			Operation: "updated",
			Message:   "Updated scanned card",
		})
		return
	}

	// Non-scanned item below this point

	// If condition, printing, or language is changing, we need smart split/merge logic
	if attributeChanging {
		if item.Quantity > 1 {
			// Stack with qty > 1: split off 1 copy with new attributes
			originalQty := item.Quantity

			// Look for existing non-scanned stack to merge the split copy into
			var target models.CollectionItem
			err := db.Where("card_id = ? AND condition = ? AND printing = ? AND language = ? AND (scanned_image_path IS NULL OR scanned_image_path = '') AND id != ?",
				item.CardID, newCondition, newPrinting, newLanguage, item.ID).
				First(&target).Error

			var resultItem models.CollectionItem
			if err == nil {
				// Merge into existing stack
				target.Quantity += 1
				if err := db.Save(&target).Error; err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				resultItem = target
			} else {
				// Create new item for the split copy
				newItem := models.CollectionItem{
					CardID:           item.CardID,
					Quantity:         1,
					Condition:        newCondition,
					Printing:         newPrinting,
					Language:         newLanguage,
					Notes:            "", // Fresh item, no notes
					AddedAt:          time.Now(),
					ScannedImagePath: "",
				}
				if err := db.Create(&newItem).Error; err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				resultItem = newItem
			}

			// Decrement original stack
			item.Quantity -= 1
			if err := db.Save(&item).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			db.Preload("Card").First(&resultItem, resultItem.ID)
			c.JSON(http.StatusOK, models.CollectionUpdateResponse{
				Item:      resultItem,
				Operation: "split",
				Message:   fmt.Sprintf("Split 1 card from stack of %d", originalQty),
			})
			return
		}

		// Single non-scanned item (qty=1): try to merge into existing stack
		var target models.CollectionItem
		err := db.Where("card_id = ? AND condition = ? AND printing = ? AND language = ? AND (scanned_image_path IS NULL OR scanned_image_path = '') AND id != ?",
			item.CardID, newCondition, newPrinting, newLanguage, item.ID).
			First(&target).Error

		if err == nil {
			// Merge into existing stack and delete this item
			target.Quantity += 1
			if err := db.Save(&target).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if err := db.Delete(&item).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			db.Preload("Card").First(&target, target.ID)
			c.JSON(http.StatusOK, models.CollectionUpdateResponse{
				Item:      target,
				Operation: "merged",
				Message:   "Merged into existing stack",
			})
			return
		}

		// No existing stack to merge into - update in place
		item.Condition = newCondition
		item.Printing = newPrinting
		item.Language = newLanguage
		if req.Notes != nil {
			item.Notes = *req.Notes
		}
		if err := db.Save(&item).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		db.Preload("Card").First(&item, item.ID)
		c.JSON(http.StatusOK, models.CollectionUpdateResponse{
			Item:      item,
			Operation: "updated",
			Message:   "",
		})
		return
	}

	// No attribute change - just update quantity and/or notes in place
	if req.Quantity != nil {
		item.Quantity = *req.Quantity
	}
	if req.Notes != nil {
		item.Notes = *req.Notes
	}

	if err := db.Save(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	db.Preload("Card").First(&item, item.ID)
	c.JSON(http.StatusOK, models.CollectionUpdateResponse{
		Item:      item,
		Operation: "updated",
		Message:   "",
	})
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

	// Calculate values and counts by game using condition-appropriate prices
	// First, try to use condition-specific prices from card_prices table
	// Falls back to card base prices (NM) if no condition price exists
	type gameStats struct {
		Game       string
		Count      int
		TotalValue float64
	}

	var gameResults []gameStats
	db.Table("collection_items").
		Select(`
			cards.game,
			SUM(collection_items.quantity) as count,
			SUM(
				COALESCE(
					(SELECT cp.price_usd FROM card_prices cp
					 WHERE cp.card_id = cards.id
					 AND cp.condition = (
						CASE collection_items.condition
							WHEN 'M' THEN 'NM'
							WHEN 'NM' THEN 'NM'
							WHEN 'EX' THEN 'LP'
							WHEN 'LP' THEN 'LP'
							WHEN 'GD' THEN 'MP'
							WHEN 'PL' THEN 'HP'
							WHEN 'PR' THEN 'DMG'
							ELSE 'NM'
						END
					 )
					 AND cp.printing = collection_items.printing
					 AND cp.language = COALESCE(NULLIF(collection_items.language, ''), 'English')
					 LIMIT 1),
					(SELECT cp.price_usd FROM card_prices cp
					 WHERE cp.card_id = cards.id
					 AND cp.condition = (
						CASE collection_items.condition
							WHEN 'M' THEN 'NM'
							WHEN 'NM' THEN 'NM'
							WHEN 'EX' THEN 'LP'
							WHEN 'LP' THEN 'LP'
							WHEN 'GD' THEN 'MP'
							WHEN 'PL' THEN 'HP'
							WHEN 'PR' THEN 'DMG'
							ELSE 'NM'
						END
					 )
					 AND cp.printing = collection_items.printing
					 AND cp.language = 'English'
					 AND COALESCE(NULLIF(collection_items.language, ''), 'English') != 'English'
					 LIMIT 1),
					CASE 
						WHEN collection_items.printing IN ('Foil', '1st Edition', 'Reverse Holofoil') 
						THEN cards.price_foil_usd 
						ELSE cards.price_usd 
					END
				) * collection_items.quantity
			) as total_value
		`).
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

// RefreshPrices immediately triggers a price update batch without waiting for the
// scheduled 15-minute interval. Updates up to 100 cards per batch, prioritizing:
// 1. User-requested refreshes (from the urgent queue)
// 2. Collection cards without prices
// 3. Collection cards with oldest prices (to fill the batch to 100)
func (h *CollectionHandler) RefreshPrices(c *gin.Context) {
	if h.priceWorker == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "price worker not available"})
		return
	}

	// Trigger immediate batch update (uses existing priority logic to select cards)
	updated, err := h.priceWorker.UpdateBatch()
	if err != nil {
		log.Printf("RefreshPrices: batch update failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "batch update failed: " + err.Error()})
		return
	}

	status := h.priceWorker.GetStatus()
	c.JSON(http.StatusOK, gin.H{
		"updated":         updated,
		"queue_size":      status.QueueSize,
		"daily_remaining": status.Remaining,
	})
}

// GetGroupedCollection returns collection items grouped by card_id
// Each group contains summary info plus all individual items for that card
//
// Query parameters:
// - game: filter by game ("pokemon" or "mtg")
// - q: search by card name or set name (case-insensitive)
// - sort: sort order ("added_at", "name", "value", "price_updated") - default "added_at"
func (h *CollectionHandler) GetGroupedCollection(c *gin.Context) {
	db := database.GetDB()

	var items []models.CollectionItem
	query := db.Preload("Card").Preload("Card.Prices")

	// Always join cards table for filtering/sorting
	needsJoin := false

	// Optional game filter
	if game := c.Query("game"); game != "" {
		if !needsJoin {
			query = query.Joins("JOIN cards ON cards.id = collection_items.card_id")
			needsJoin = true
		}
		query = query.Where("cards.game = ?", game)
	}

	// Search filter (name or set)
	if search := c.Query("q"); search != "" {
		if !needsJoin {
			query = query.Joins("JOIN cards ON cards.id = collection_items.card_id")
			needsJoin = true
		}
		searchPattern := "%" + search + "%"
		query = query.Where("cards.name LIKE ? OR cards.set_name LIKE ? OR cards.set_code LIKE ?",
			searchPattern, searchPattern, searchPattern)
	}

	// Sorting
	sortBy := c.DefaultQuery("sort", "added_at")
	switch sortBy {
	case "name":
		if !needsJoin {
			query = query.Joins("JOIN cards ON cards.id = collection_items.card_id")
		}
		query = query.Order("cards.name ASC")
	case "price_updated":
		if !needsJoin {
			query = query.Joins("JOIN cards ON cards.id = collection_items.card_id")
		}
		query = query.Order("cards.price_updated_at DESC NULLS LAST")
	case "added_at":
		fallthrough
	default:
		query = query.Order("collection_items.added_at DESC")
	}

	if err := query.Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Group items by card_id
	cardGroups := make(map[string][]models.CollectionItem)
	cardMap := make(map[string]models.Card)

	for _, item := range items {
		cardGroups[item.CardID] = append(cardGroups[item.CardID], item)
		if _, exists := cardMap[item.CardID]; !exists {
			cardMap[item.CardID] = item.Card
		}
	}

	// Build grouped response
	var result []models.GroupedCollectionItem

	for cardID, groupItems := range cardGroups {
		card := cardMap[cardID]

		// Calculate totals
		totalQty := 0
		totalValue := 0.0
		scannedCount := 0

		// Track variants by printing+condition
		variantMap := make(map[string]*models.CollectionVariant)

		for _, item := range groupItems {
			totalQty += item.Quantity

			// Calculate item value using condition-specific pricing with language
			// GetPriceWithSource returns metadata about whether a language fallback occurred
			priceCondition := models.MapCollectionConditionToPriceCondition(item.Condition)
			priceResult := card.GetPriceWithSource(priceCondition, item.Printing, item.Language)
			itemValue := priceResult.Price * float64(item.Quantity)
			totalValue += itemValue

			// Count scanned cards
			if item.ScannedImagePath != "" {
				scannedCount++
			}

			// Aggregate variants by printing+condition+language
			variantKey := fmt.Sprintf("%s|%s|%s", item.Printing, item.Condition, item.Language)
			if v, exists := variantMap[variantKey]; exists {
				v.Quantity += item.Quantity
				v.Value += itemValue
				if item.ScannedImagePath != "" {
					v.HasScans = true
					v.ScannedQty++
				}
			} else {
				hasScans := item.ScannedImagePath != ""
				scannedQty := 0
				if hasScans {
					scannedQty = 1
				}
				variantMap[variantKey] = &models.CollectionVariant{
					Printing:      item.Printing,
					Condition:     item.Condition,
					Language:      item.Language,
					Quantity:      item.Quantity,
					Value:         itemValue,
					HasScans:      hasScans,
					ScannedQty:    scannedQty,
					PriceLanguage: priceResult.PriceLanguage,
					PriceFallback: priceResult.IsFallback,
				}
			}
		}

		// Convert variant map to slice
		variants := make([]models.CollectionVariant, 0, len(variantMap))
		for _, v := range variantMap {
			variants = append(variants, *v)
		}

		result = append(result, models.GroupedCollectionItem{
			Card:          card,
			TotalQuantity: totalQty,
			TotalValue:    totalValue,
			ScannedCount:  scannedCount,
			Variants:      variants,
			Items:         groupItems,
		})
	}

	c.JSON(http.StatusOK, result)
}

// GetValueHistory returns collection value snapshots for charting
func (h *CollectionHandler) GetValueHistory(c *gin.Context) {
	if h.snapshotService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "snapshot service not available"})
		return
	}

	period := c.DefaultQuery("period", "month")

	snapshots, err := h.snapshotService.GetHistory(period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.ValueHistoryResponse{
		Snapshots: snapshots,
		Period:    period,
	})
}
