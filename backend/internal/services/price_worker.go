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
	defaultBatchSize = 20
)

type PriceWorker struct {
	priceService   *PriceService
	justTCG        *JustTCGService
	pokemonService *PokemonHybridService
	updateInterval time.Duration
	mu             sync.RWMutex

	// Batch config
	batchSize int

	// Priority queue for user-requested refreshes
	urgentQueue []string
	urgentMu    sync.Mutex

	// Stats (reset at midnight)
	cardsUpdatedToday int
	lastUpdateTime    time.Time
	lastStatsDay      time.Time // Track which day the stats are for
}

type PriceStatus struct {
	LastUpdateTime    time.Time `json:"last_update_time"`
	NextUpdateTime    time.Time `json:"next_update_time"`
	CardsUpdatedToday int       `json:"cards_updated_today"`
	BatchSize         int       `json:"batch_size"`
	QueueSize         int       `json:"queue_size"`

	// JustTCG quota info
	DailyLimit int       `json:"daily_limit"`
	Remaining  int       `json:"remaining"`
	ResetsAt   time.Time `json:"resets_at,omitempty"`
}

func NewPriceWorker(priceService *PriceService, pokemonService *PokemonHybridService, justTCG *JustTCGService) *PriceWorker {
	return &PriceWorker{
		priceService:   priceService,
		justTCG:        justTCG,
		pokemonService: pokemonService,
		batchSize:      defaultBatchSize,
		updateInterval: 15 * time.Minute,
	}
}

// QueueRefresh adds a card to the high-priority refresh queue
func (w *PriceWorker) QueueRefresh(cardID string) int {
	w.urgentMu.Lock()
	defer w.urgentMu.Unlock()

	// Avoid duplicates
	for _, id := range w.urgentQueue {
		if id == cardID {
			// Return current position (1-indexed)
			for i, qid := range w.urgentQueue {
				if qid == cardID {
					return i + 1
				}
			}
		}
	}
	w.urgentQueue = append(w.urgentQueue, cardID)
	log.Printf("Price worker: queued refresh for card %s (queue size: %d)", cardID, len(w.urgentQueue))
	return len(w.urgentQueue)
}

// GetQueueSize returns current urgent queue size
func (w *PriceWorker) GetQueueSize() int {
	w.urgentMu.Lock()
	defer w.urgentMu.Unlock()
	return len(w.urgentQueue)
}

// resetDailyStatsIfNeeded resets cardsUpdatedToday at midnight
func (w *PriceWorker) resetDailyStatsIfNeeded() {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	if w.lastStatsDay.Before(today) {
		if !w.lastStatsDay.IsZero() {
			log.Printf("Price worker: daily stats reset (previous day: %d cards updated)", w.cardsUpdatedToday)
		}
		w.cardsUpdatedToday = 0
		w.lastStatsDay = today
	}
}

// hasJustTCGQuota checks if we have JustTCG API quota remaining
// Returns false only if JustTCG is configured and quota is exhausted
func (w *PriceWorker) hasJustTCGQuota() bool {
	if w.justTCG == nil {
		return false // No JustTCG configured
	}
	return w.priceService.GetJustTCGRequestsRemaining() > 0
}

// Start begins the background price update worker
func (w *PriceWorker) Start(ctx context.Context) {
	log.Printf("Price worker started: will update %d cards every %v (Pokemon and MTG)", w.batchSize, w.updateInterval)

	// Run immediately on startup
	if updated, err := w.UpdateBatch(); err != nil {
		log.Printf("Price worker: initial batch update failed: %v", err)
	} else {
		log.Printf("Price worker: initial batch updated %d cards", updated)
	}

	ticker := time.NewTicker(w.updateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Price worker stopping...")
			return
		case <-ticker.C:
			if updated, err := w.UpdateBatch(); err != nil {
				log.Printf("Price worker: batch update failed: %v", err)
			} else if updated > 0 {
				log.Printf("Price worker: batch updated %d cards", updated)
			}
		}
	}
}

// UpdateBatch updates a batch of cards with priority ordering:
// 1. User-requested refreshes
// 2. Collection cards without prices
// 3. Collection cards with oldest prices
func (w *PriceWorker) UpdateBatch() (updated int, err error) {
	// Reset daily stats at midnight
	w.resetDailyStatsIfNeeded()

	// Check JustTCG quota - skip batch if exhausted
	if !w.hasJustTCGQuota() {
		resetTime := w.priceService.GetJustTCGResetTime()
		log.Printf("Price worker: JustTCG quota exhausted, skipping until %s", resetTime.Format("15:04"))
		return 0, nil
	}

	db := database.GetDB()
	var cardsToUpdate []models.Card
	var cardIDs []string

	// Priority 1: User-requested refreshes
	w.urgentMu.Lock()
	urgentIDs := w.urgentQueue
	if len(urgentIDs) > w.batchSize {
		urgentIDs = urgentIDs[:w.batchSize]
		w.urgentQueue = w.urgentQueue[w.batchSize:]
	} else {
		w.urgentQueue = nil
	}
	w.urgentMu.Unlock()

	if len(urgentIDs) > 0 {
		var urgentCards []models.Card
		db.Where("id IN ?", urgentIDs).Find(&urgentCards)
		cardsToUpdate = append(cardsToUpdate, urgentCards...)
		for _, c := range urgentCards {
			cardIDs = append(cardIDs, c.ID)
		}
		log.Printf("Price worker: processing %d urgent refresh requests", len(urgentCards))
	}

	remaining := w.batchSize - len(cardsToUpdate)

	// Priority 2: Collection cards without prices
	if remaining > 0 {
		var noPriceCards []models.Card
		query := `
			SELECT DISTINCT c.* FROM cards c
			INNER JOIN collection_items ci ON ci.card_id = c.id
			LEFT JOIN card_prices cp ON cp.card_id = c.id
			WHERE cp.id IS NULL
		`
		if len(cardIDs) > 0 {
			db.Raw(query+" AND c.id NOT IN (?) LIMIT ?", cardIDs, remaining).Scan(&noPriceCards)
		} else {
			db.Raw(query+" LIMIT ?", remaining).Scan(&noPriceCards)
		}

		cardsToUpdate = append(cardsToUpdate, noPriceCards...)
		for _, c := range noPriceCards {
			cardIDs = append(cardIDs, c.ID)
		}
		remaining -= len(noPriceCards)
	}

	// Priority 3: Collection cards with oldest prices
	if remaining > 0 {
		var oldestCards []models.Card
		query := `
			SELECT c.* FROM cards c
			INNER JOIN collection_items ci ON ci.card_id = c.id
		`
		if len(cardIDs) > 0 {
			db.Raw(query+" WHERE c.id NOT IN (?) ORDER BY c.price_updated_at ASC NULLS FIRST LIMIT ?",
				cardIDs, remaining).Scan(&oldestCards)
		} else {
			db.Raw(query+" ORDER BY c.price_updated_at ASC NULLS FIRST LIMIT ?",
				remaining).Scan(&oldestCards)
		}
		cardsToUpdate = append(cardsToUpdate, oldestCards...)
	}

	if len(cardsToUpdate) == 0 {
		log.Println("Price worker: no cards to update")
		return 0, nil
	}

	log.Printf("Price worker: updating prices for %d cards", len(cardsToUpdate))

	return w.batchUpdatePrices(cardsToUpdate)
}

// batchUpdatePrices uses the batch API to update all cards at once
func (w *PriceWorker) batchUpdatePrices(cards []models.Card) (int, error) {
	// Build lookup requests
	lookups := make([]CardLookup, len(cards))
	cardMap := make(map[string]*models.Card) // Keyed by CardID only

	for i, card := range cards {
		gameStr := "pokemon"
		if card.Game == models.GameMTG {
			gameStr = "magic-the-gathering"
		}

		lookups[i] = CardLookup{
			CardID: card.ID,
			Name:   card.Name,
			Set:    card.SetCode,
			Game:   gameStr,
		}
		cardMap[card.ID] = &cards[i]
	}

	// Single batch request
	results, err := w.justTCG.BatchGetPrices(lookups)
	if err != nil {
		log.Printf("Price worker: batch request failed: %v", err)
		return 0, err
	}

	// Save results - keys should be CardIDs from JustTCG service
	updated := 0
	db := database.GetDB()

	for cardID, prices := range results {
		if len(prices) == 0 {
			continue
		}

		card, ok := cardMap[cardID]
		if !ok {
			log.Printf("Price worker: received prices for unknown card ID: %s", cardID)
			continue
		}

		// Set card ID on all prices
		for i := range prices {
			prices[i].CardID = card.ID
		}
		w.priceService.SaveCardPrices(card.ID, prices)

		// Update base card prices for backward compatibility
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
		db.Save(card)
		updated++
	}

	w.mu.Lock()
	w.cardsUpdatedToday += updated
	w.lastUpdateTime = time.Now()
	w.mu.Unlock()

	log.Printf("Price worker: batch updated %d card prices", updated)
	return updated, nil
}

// GetStatus returns the current status
func (w *PriceWorker) GetStatus() PriceStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return PriceStatus{
		LastUpdateTime:    w.lastUpdateTime,
		NextUpdateTime:    w.lastUpdateTime.Add(w.updateInterval),
		CardsUpdatedToday: w.cardsUpdatedToday,
		BatchSize:         w.batchSize,
		QueueSize:         w.GetQueueSize(),
		DailyLimit:        w.priceService.GetJustTCGDailyLimit(),
		Remaining:         w.priceService.GetJustTCGRequestsRemaining(),
		ResetsAt:          w.priceService.GetJustTCGResetTime(),
	}
}
