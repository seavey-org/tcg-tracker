package services

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/codyseavey/tcg-tracker/backend/internal/database"
	"github.com/codyseavey/tcg-tracker/backend/internal/models"
)

// Constants for price worker configuration
const (
	// defaultBatchSize is the number of cards to update per batch
	// TCGdex has no rate limits, so we can be more aggressive
	defaultBatchSize = 20
	// apiRequestDelay is the delay between API requests (minimal for TCGdex)
	apiRequestDelay = 100 * time.Millisecond
)

type PriceWorker struct {
	pokemonService *PokemonHybridService
	updateInterval time.Duration
	mu             sync.RWMutex

	// Batch config
	batchSize int

	// Stats
	cardsUpdatedToday int
	lastUpdateTime    time.Time
}

type PriceStatus struct {
	LastUpdateTime    time.Time `json:"last_update_time"`
	NextUpdateTime    time.Time `json:"next_update_time"`
	CardsUpdatedToday int       `json:"cards_updated_today"`
	BatchSize         int       `json:"batch_size"`
}

func NewPriceWorker(pokemonService *PokemonHybridService, _ int) *PriceWorker {
	return &PriceWorker{
		pokemonService: pokemonService,
		batchSize:      defaultBatchSize,
		updateInterval: 1 * time.Hour,
	}
}

// Start begins the background price update worker
func (w *PriceWorker) Start(ctx context.Context) {
	log.Printf("Price worker started: will update %d Pokemon cards per hour using TCGdex (no rate limits)", w.batchSize)

	// Run immediately on startup
	_, _ = w.UpdateBatch()

	ticker := time.NewTicker(w.updateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Price worker stopping...")
			return
		case <-ticker.C:
			_, _ = w.UpdateBatch()
		}
	}
}

// UpdateBatch updates a batch of cards with the oldest prices
func (w *PriceWorker) UpdateBatch() (updated int, err error) {
	db := database.GetDB()

	// Get cards that need price updates
	// Priority: cards in collection with oldest/no prices
	var cards []models.Card

	// First: Pokemon cards in collection with no price or oldest prices
	db.Raw(`
		SELECT c.* FROM cards c
		INNER JOIN collection_items ci ON ci.card_id = c.id
		WHERE c.game = ?
		ORDER BY c.price_updated_at ASC NULLS FIRST
		LIMIT ?
	`, models.GamePokemon, w.batchSize).Scan(&cards)

	// If we don't have enough, add cached cards not in collection
	if len(cards) < w.batchSize {
		remaining := w.batchSize - len(cards)
		var moreCards []models.Card
		db.Where("game = ?", models.GamePokemon).
			Order("price_updated_at ASC NULLS FIRST").
			Limit(remaining).
			Offset(len(cards)).
			Find(&moreCards)
		cards = append(cards, moreCards...)
	}

	if len(cards) == 0 {
		log.Println("Price worker: no cards to update")
		return 0, nil
	}

	log.Printf("Price worker: updating prices for %d cards", len(cards))

	for _, card := range cards {
		if err := w.updateCardPrice(&card); err != nil {
			log.Printf("Price worker: failed to update %s: %v", card.Name, err)
			continue
		}

		// Save updated card to database
		if err := db.Save(&card).Error; err != nil {
			log.Printf("Price worker: failed to save %s: %v", card.Name, err)
			continue
		}

		updated++

		// Small delay between requests to be nice to the API
		time.Sleep(apiRequestDelay)
	}

	w.mu.Lock()
	w.cardsUpdatedToday += updated
	w.lastUpdateTime = time.Now()
	w.mu.Unlock()

	log.Printf("Price worker: updated %d card prices", updated)
	return updated, nil
}

// UpdateCard updates a single card's price (for manual refresh)
func (w *PriceWorker) UpdateCard(cardID string) (*models.Card, error) {
	db := database.GetDB()

	var card models.Card
	if err := db.First(&card, "id = ?", cardID).Error; err != nil {
		return nil, err
	}

	if card.Game != models.GamePokemon {
		// For MTG, use Scryfall (no rate limit concerns)
		return nil, nil
	}

	if err := w.updateCardPrice(&card); err != nil {
		return nil, err
	}

	if err := db.Save(&card).Error; err != nil {
		return nil, err
	}

	w.mu.Lock()
	w.cardsUpdatedToday++
	w.mu.Unlock()

	log.Printf("Price worker: manually refreshed price for %s", card.Name)
	return &card, nil
}

func (w *PriceWorker) updateCardPrice(card *models.Card) error {
	// Search for the card by name to get price from TCGdex
	priceResult, err := w.pokemonService.tcgdexService.SearchCards(card.Name)
	if err != nil {
		return err
	}

	if priceResult == nil || len(priceResult.Cards) == 0 {
		// Mark that we checked but found no price
		now := time.Now()
		card.LastPriceCheck = &now
		card.PriceSource = "not_found"
		return nil
	}

	// Find the best matching card
	for _, priceCard := range priceResult.Cards {
		// Try to match by set and number
		if priceCard.SetCode == card.SetCode && priceCard.CardNumber == card.CardNumber {
			now := time.Now()
			card.PriceUSD = priceCard.PriceUSD
			card.PriceFoilUSD = priceCard.PriceFoilUSD
			card.PriceUpdatedAt = &now
			card.LastPriceCheck = &now
			card.PriceSource = "api"
			return nil
		}
	}

	// If no exact match, use first result with same name
	for _, priceCard := range priceResult.Cards {
		if priceCard.Name == card.Name {
			now := time.Now()
			card.PriceUSD = priceCard.PriceUSD
			card.PriceFoilUSD = priceCard.PriceFoilUSD
			card.PriceUpdatedAt = &now
			card.LastPriceCheck = &now
			card.PriceSource = "api"
			return nil
		}
	}

	// Mark that we checked but couldn't match
	now := time.Now()
	card.LastPriceCheck = &now
	card.PriceSource = "unmatched"
	return nil
}

// GetStatus returns the current status
func (w *PriceWorker) GetStatus() PriceStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()

	now := time.Now()

	return PriceStatus{
		LastUpdateTime:    w.lastUpdateTime,
		NextUpdateTime:    now.Add(w.updateInterval),
		CardsUpdatedToday: w.cardsUpdatedToday,
		BatchSize:         w.batchSize,
	}
}
