package services

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/codyseavey/tcg-tracker/backend/internal/database"
	"github.com/codyseavey/tcg-tracker/backend/internal/models"
)

type PriceWorker struct {
	pokemonService *PokemonHybridService

	// Rate limiting
	dailyLimit    int
	requestsToday int
	lastResetDate time.Time
	mu            sync.RWMutex

	// Batch config
	batchSize      int
	updateInterval time.Duration
}

type PriceStatus struct {
	Remaining       int       `json:"remaining"`
	DailyLimit      int       `json:"daily_limit"`
	RequestsToday   int       `json:"requests_today"`
	ResetsAt        time.Time `json:"resets_at"`
	LastUpdateTime  time.Time `json:"last_update_time"`
	NextUpdateTime  time.Time `json:"next_update_time"`
	CardsUpdated    int       `json:"cards_updated_today"`
}

func NewPriceWorker(pokemonService *PokemonHybridService, dailyLimit int) *PriceWorker {
	return &PriceWorker{
		pokemonService: pokemonService,
		dailyLimit:     dailyLimit,
		requestsToday:  0,
		lastResetDate:  time.Now().Truncate(24 * time.Hour),
		batchSize:      4, // With 100/day limit, ~4 per hour is safe (96/day)
		updateInterval: 1 * time.Hour,
	}
}

// Start begins the background price update worker
func (w *PriceWorker) Start(ctx context.Context) {
	log.Printf("Price worker started: will update up to %d Pokemon cards per hour (limit: %d/day)", w.batchSize, w.dailyLimit)

	// Run immediately on startup
	w.UpdateBatch()

	ticker := time.NewTicker(w.updateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Price worker stopping...")
			return
		case <-ticker.C:
			w.UpdateBatch()
		}
	}
}

// UpdateBatch updates a batch of cards with the oldest prices
func (w *PriceWorker) UpdateBatch() (updated int, err error) {
	w.mu.Lock()

	// Reset daily counter if it's a new day
	today := time.Now().Truncate(24 * time.Hour)
	if today.After(w.lastResetDate) {
		w.requestsToday = 0
		w.lastResetDate = today
		log.Println("Price worker: daily quota reset")
	}

	// Check if we have quota remaining
	backgroundQuota := w.dailyLimit - 10 // Reserve 10 for manual refreshes
	if w.requestsToday >= backgroundQuota {
		w.mu.Unlock()
		log.Printf("Price worker: daily background quota exhausted (%d/%d used)", w.requestsToday, backgroundQuota)
		return 0, nil
	}

	remainingQuota := backgroundQuota - w.requestsToday
	batchSize := w.batchSize
	if remainingQuota < batchSize {
		batchSize = remainingQuota
	}

	w.mu.Unlock()

	if batchSize <= 0 {
		return 0, nil
	}

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
	`, models.GamePokemon, batchSize).Scan(&cards)

	// If we don't have enough, add cached cards not in collection
	if len(cards) < batchSize {
		remaining := batchSize - len(cards)
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

		w.mu.Lock()
		w.requestsToday++
		w.mu.Unlock()

		updated++

		// Small delay between requests to be nice to the API
		time.Sleep(500 * time.Millisecond)
	}

	log.Printf("Price worker: updated %d card prices", updated)
	return updated, nil
}

// UpdateCard updates a single card's price (for manual refresh)
func (w *PriceWorker) UpdateCard(cardID string) (*models.Card, error) {
	w.mu.Lock()

	// Check quota for manual requests
	if w.requestsToday >= w.dailyLimit {
		w.mu.Unlock()
		return nil, nil // Quota exhausted
	}

	w.requestsToday++
	w.mu.Unlock()

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

	log.Printf("Price worker: manually refreshed price for %s", card.Name)
	return &card, nil
}

func (w *PriceWorker) updateCardPrice(card *models.Card) error {
	// Search for the card by name to get price
	priceResult, err := w.pokemonService.priceService.SearchCards(card.Name)
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

// GetStatus returns the current quota status
func (w *PriceWorker) GetStatus() PriceStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// Calculate reset time (next midnight)
	now := time.Now()
	tomorrow := now.Truncate(24 * time.Hour).Add(24 * time.Hour)

	return PriceStatus{
		Remaining:      w.dailyLimit - w.requestsToday,
		DailyLimit:     w.dailyLimit,
		RequestsToday:  w.requestsToday,
		ResetsAt:       tomorrow,
		NextUpdateTime: now.Add(w.updateInterval),
	}
}

// GetRemainingQuota returns the number of requests remaining today
func (w *PriceWorker) GetRemainingQuota() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.dailyLimit - w.requestsToday
}
