package services

import (
	"log"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/codyseavey/tcg-tracker/backend/internal/models"
)

const (
	// PriceStalenessThreshold is how old a price can be before it's considered stale
	PriceStalenessThreshold = 24 * time.Hour
)

// PriceService provides unified price fetching from JustTCG
type PriceService struct {
	justTCG *JustTCGService
	db      *gorm.DB
}

// NewPriceService creates a new price service
func NewPriceService(justTCG *JustTCGService, db *gorm.DB) *PriceService {
	return &PriceService{
		justTCG: justTCG,
		db:      db,
	}
}

// GetPrice returns the price for a specific card, condition, and printing type
// Fallback order: DB cache -> JustTCG -> stale cache
func (s *PriceService) GetPrice(card *models.Card, condition models.PriceCondition, printing models.PrintingType) (float64, string, error) {
	// 1. Check database cache for fresh condition-specific price
	cachedPrice, err := s.getCachedPrice(card.ID, condition, printing)
	if err == nil && cachedPrice != nil && s.isFresh(cachedPrice.PriceUpdatedAt) {
		return cachedPrice.PriceUSD, cachedPrice.Source, nil
	}

	// 2. Try JustTCG for condition-specific pricing
	if s.justTCG != nil {
		prices, err := s.justTCG.GetCardPrices(card.Name, card.SetCode, card.Game)
		if err == nil && len(prices) > 0 {
			// Save all prices to cache
			s.saveCardPrices(card.ID, prices)

			// Return the requested price
			for _, p := range prices {
				if p.Condition == condition && p.Printing == printing {
					return p.PriceUSD, "justtcg", nil
				}
			}

			// If exact condition not found, try NM as fallback
			if condition != models.PriceConditionNM {
				for _, p := range prices {
					if p.Condition == models.PriceConditionNM && p.Printing == printing {
						return p.PriceUSD, "justtcg", nil
					}
				}
			}
		} else if err != nil {
			log.Printf("JustTCG price fetch failed for %s: %v", card.Name, err)
		}
	}

	// 3. Return stale cached price if available
	if cachedPrice != nil {
		return cachedPrice.PriceUSD, cachedPrice.Source + " (stale)", nil
	}

	// 4. Fallback to card's base price (from previous JustTCG fetch)
	isFoilVariant := printing.IsFoilVariant()
	if isFoilVariant && card.PriceFoilUSD > 0 {
		return card.PriceFoilUSD, "cached", nil
	}
	if card.PriceUSD > 0 {
		return card.PriceUSD, "cached", nil
	}

	return 0, "", nil
}

// GetAllConditionPrices returns all available prices for a card
// Priority: fresh cache -> JustTCG API -> stale cache -> base prices
func (s *PriceService) GetAllConditionPrices(card *models.Card) ([]models.CardPrice, error) {
	// 1. Check if we have cached prices
	var cachedPrices []models.CardPrice
	if err := s.db.Where("card_id = ?", card.ID).Find(&cachedPrices).Error; err != nil {
		log.Printf("Failed to fetch cached prices for card %s: %v", card.ID, err)
	}

	// Check if all cached prices are fresh
	allFresh := len(cachedPrices) > 0
	for _, p := range cachedPrices {
		if !s.isFresh(p.PriceUpdatedAt) {
			allFresh = false
			break
		}
	}

	// If all cached prices are fresh, return them
	if allFresh {
		return cachedPrices, nil
	}

	// 2. Try to fetch fresh prices from JustTCG
	if s.justTCG != nil {
		prices, err := s.justTCG.GetCardPrices(card.Name, card.SetCode, card.Game)
		if err == nil && len(prices) > 0 {
			for i := range prices {
				prices[i].CardID = card.ID
			}
			s.saveCardPrices(card.ID, prices)
			return prices, nil
		}
	}

	// 3. Return cached prices even if stale
	if len(cachedPrices) > 0 {
		return cachedPrices, nil
	}

	// 4. Return base prices from card (from previous JustTCG fetch)
	var basePrices []models.CardPrice
	if card.PriceUSD > 0 {
		basePrices = append(basePrices, models.CardPrice{
			CardID:         card.ID,
			Condition:      models.PriceConditionNM,
			Printing:       models.PrintingNormal,
			PriceUSD:       card.PriceUSD,
			Source:         "cached",
			PriceUpdatedAt: card.PriceUpdatedAt,
		})
	}
	if card.PriceFoilUSD > 0 {
		basePrices = append(basePrices, models.CardPrice{
			CardID:         card.ID,
			Condition:      models.PriceConditionNM,
			Printing:       models.PrintingFoil,
			PriceUSD:       card.PriceFoilUSD,
			Source:         "cached",
			PriceUpdatedAt: card.PriceUpdatedAt,
		})
	}

	return basePrices, nil
}

// UpdateCardPrices fetches and saves all condition prices for a card
// Returns the number of prices updated
func (s *PriceService) UpdateCardPrices(card *models.Card) (int, error) {
	prices, err := s.GetAllConditionPrices(card)
	if err != nil {
		return 0, err
	}

	// Also update the base card prices for backward compatibility
	if len(prices) > 0 {
		for _, p := range prices {
			if p.Condition == models.PriceConditionNM {
				if p.Printing.IsFoilVariant() {
					card.PriceFoilUSD = p.PriceUSD
				} else {
					card.PriceUSD = p.PriceUSD
				}
				card.PriceUpdatedAt = p.PriceUpdatedAt
				card.PriceSource = p.Source
			}
		}
		if err := s.db.Save(card).Error; err != nil {
			log.Printf("Failed to update base prices for card %s: %v", card.ID, err)
			// Don't fail the whole operation, prices were still fetched
		}
	}

	return len(prices), nil
}

// getCachedPrice retrieves a cached price from the database
func (s *PriceService) getCachedPrice(cardID string, condition models.PriceCondition, printing models.PrintingType) (*models.CardPrice, error) {
	var price models.CardPrice
	err := s.db.Where("card_id = ? AND condition = ? AND printing = ?", cardID, condition, printing).First(&price).Error
	if err != nil {
		return nil, err
	}
	return &price, nil
}

// SaveCardPrices saves prices to the database (upsert) - exported for use by price worker
func (s *PriceService) SaveCardPrices(cardID string, prices []models.CardPrice) {
	s.saveCardPrices(cardID, prices)
}

// saveCardPrices saves prices to the database using bulk upsert
func (s *PriceService) saveCardPrices(cardID string, prices []models.CardPrice) {
	if len(prices) == 0 {
		return
	}

	// Ensure all prices have the correct card ID
	for i := range prices {
		prices[i].CardID = cardID
	}

	// Bulk upsert: insert or update on conflict with unique index (card_id, condition, printing)
	err := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "card_id"}, {Name: "condition"}, {Name: "printing"}},
		DoUpdates: clause.AssignmentColumns([]string{"price_usd", "source", "price_updated_at", "updated_at"}),
	}).Create(&prices).Error

	if err != nil {
		log.Printf("Failed to save prices for card %s: %v", cardID, err)
	}
}

// isFresh checks if a price update time is within the staleness threshold
func (s *PriceService) isFresh(updatedAt *time.Time) bool {
	if updatedAt == nil {
		return false
	}
	return time.Since(*updatedAt) < PriceStalenessThreshold
}

// GetJustTCGRequestsRemaining returns remaining JustTCG API requests for today
func (s *PriceService) GetJustTCGRequestsRemaining() int {
	if s.justTCG == nil {
		return 0
	}
	return s.justTCG.GetRequestsRemaining()
}

// GetJustTCGDailyLimit returns the configured daily limit
func (s *PriceService) GetJustTCGDailyLimit() int {
	if s.justTCG == nil {
		return 0
	}
	return s.justTCG.GetDailyLimit()
}

// GetJustTCGResetTime returns the next reset time
func (s *PriceService) GetJustTCGResetTime() time.Time {
	if s.justTCG == nil {
		return time.Time{}
	}
	return s.justTCG.GetResetTime()
}
