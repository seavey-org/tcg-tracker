package services

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/codyseavey/tcg-tracker/backend/internal/database"
	"github.com/codyseavey/tcg-tracker/backend/internal/metrics"
	"github.com/codyseavey/tcg-tracker/backend/internal/models"
)

// Constants for price worker configuration
const (
	// defaultBatchSize is the number of cards to update per batch
	// Paid tier allows 100 cards per request
	defaultBatchSize = 100
)

// UnmatchedCard represents a card that couldn't be matched for price updates
type UnmatchedCard struct {
	CardID     string `json:"card_id"`
	Name       string `json:"name"`
	CardNumber string `json:"card_number"`
	SetName    string `json:"set_name"`
	Reason     string `json:"reason"`
}

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

	// Cards that couldn't be matched for price updates
	unmatchedCards []UnmatchedCard
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

	// Cards that can't receive price updates (missing TCGPlayerID)
	UnmatchedCards []UnmatchedCard `json:"unmatched_cards,omitempty"`
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
			SELECT DISTINCT c.* FROM cards c
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
// For Pokemon cards without TCGPlayerIDs, it syncs the set first to discover IDs
// This ensures all cards can use efficient batch POST (no individual GETs)
func (w *PriceWorker) batchUpdatePrices(cards []models.Card) (int, error) {
	start := time.Now()
	db := database.GetDB()

	// First pass: identify Pokemon cards missing TCGPlayerIDs and sync their sets
	setsToSync := make(map[string][]int) // setName -> indices of cards needing sync
	for i, card := range cards {
		if card.Game == models.GamePokemon && card.TCGPlayerID == "" {
			setName := card.SetName
			if setName == "" {
				setName = card.SetCode
			}
			setsToSync[setName] = append(setsToSync[setName], i)
		}
	}

	// Sync sets to discover TCGPlayerIDs before batch request
	// Track which sets were successfully synced vs had intermittent failures
	successfullySyncedSets := make(map[string]bool)
	permanentlyFailedSets := make(map[string]string) // setName -> reason
	var newUnmatchedCards []UnmatchedCard

	if len(setsToSync) > 0 {
		log.Printf("Price worker: syncing %d sets to discover TCGPlayerIDs", len(setsToSync))

		for setName, cardIndices := range setsToSync {
			// Check quota before each set sync - this is INTERMITTENT, will retry next batch
			if w.justTCG.GetRequestsRemaining() < 2 {
				log.Printf("Price worker: quota low, skipping set sync for %s (will retry)", setName)
				continue // Don't mark as permanent failure
			}

			// Convert to JustTCG set ID - unknown mapping is PERMANENT
			justTCGSetID := convertToJustTCGSetID(setName)
			if justTCGSetID == "" {
				reason := "Unknown set mapping - needs code update in tcgplayer_sync.go"
				log.Printf("ERROR: Price worker: %s for set %q", reason, setName)
				permanentlyFailedSets[setName] = reason
				continue
			}

			// Fetch set data - API errors are INTERMITTENT
			setData, err := w.justTCG.FetchSetTCGPlayerIDs(justTCGSetID)
			if err != nil {
				log.Printf("Price worker: failed to sync set %s: %v (will retry)", setName, err)
				continue // Don't mark as permanent failure
			}

			// Set synced successfully
			successfullySyncedSets[setName] = true

			// Update cards with discovered TCGPlayerIDs
			for _, idx := range cardIndices {
				card := &cards[idx]
				tcgPlayerID := ""

				// Try matching by card number first
				if card.CardNumber != "" {
					normalizedNum := strings.TrimLeft(card.CardNumber, "0")
					if normalizedNum == "" {
						normalizedNum = "0"
					}
					if id, ok := setData.CardsByNum[normalizedNum]; ok {
						tcgPlayerID = id
					} else if id, ok := setData.CardsByNum[card.CardNumber]; ok {
						tcgPlayerID = id
					}
				}

				// Fallback to name matching (with normalized name for special characters)
				if tcgPlayerID == "" {
					normalizedName := normalizeNameForPriceMatch(card.Name)
					if id, ok := setData.CardsByName[normalizedName]; ok {
						tcgPlayerID = id
					}
				}

				if tcgPlayerID != "" {
					card.TCGPlayerID = tcgPlayerID
					// Save to DB immediately so it persists
					db.Model(card).Update("tcg_player_id", tcgPlayerID)
					log.Printf("Price worker: discovered TCGPlayerID %s for %s", tcgPlayerID, card.Name)

					// Remove from unmatched list if it was there
					w.ClearUnmatchedCard(card.ID)
				} else {
					// Set was synced successfully but card not found - this is PERMANENT
					reason := "Card not found in JustTCG set data (checked by number and name)"
					log.Printf("ERROR: Price worker: UNMATCHED CARD - %s (#%s) from set %q - %s",
						card.Name, card.CardNumber, setName, reason)
					newUnmatchedCards = append(newUnmatchedCards, UnmatchedCard{
						CardID:     card.ID,
						Name:       card.Name,
						CardNumber: card.CardNumber,
						SetName:    setName,
						Reason:     reason,
					})
				}
			}
		}

		// Add cards from permanently failed sets (unknown mapping)
		for setName, reason := range permanentlyFailedSets {
			for _, idx := range setsToSync[setName] {
				card := &cards[idx]
				newUnmatchedCards = append(newUnmatchedCards, UnmatchedCard{
					CardID:     card.ID,
					Name:       card.Name,
					CardNumber: card.CardNumber,
					SetName:    setName,
					Reason:     reason,
				})
			}
		}
	}

	// Build lookup requests (now all Pokemon cards should have TCGPlayerIDs)
	lookups := make([]CardLookup, 0, len(cards))
	cardMap := make(map[string]*models.Card)

	for i := range cards {
		card := &cards[i]
		gameStr := "pokemon"
		scryfallID := ""
		tcgPlayerID := ""

		if card.Game == models.GameMTG {
			gameStr = "magic-the-gathering"
			scryfallID = card.ID
		} else {
			tcgPlayerID = card.TCGPlayerID
			// Skip Pokemon cards without TCGPlayerID
			if tcgPlayerID == "" {
				// Only mark as permanently unmatched if we actually tried to sync the set
				// Cards from sets that were skipped due to quota/API errors will retry next batch
				setName := card.SetName
				if setName == "" {
					setName = card.SetCode
				}
				_, wasSuccessfullySynced := successfullySyncedSets[setName]
				_, wasPermanentlyFailed := permanentlyFailedSets[setName]

				// If set wasn't processed at all (quota/API error), this is intermittent - don't add to unmatched
				// Cards from successfully synced sets or permanently failed sets were already handled above
				if !wasSuccessfullySynced && !wasPermanentlyFailed {
					log.Printf("Price worker: skipping %s (set %s not synced yet, will retry)", card.Name, setName)
				}
				// Note: cards from successfully synced sets that weren't found are already in newUnmatchedCards
				// Note: cards from permanently failed sets are already in newUnmatchedCards
				continue
			}
		}

		lookups = append(lookups, CardLookup{
			CardID:      card.ID,
			ScryfallID:  scryfallID,
			TCGPlayerID: tcgPlayerID,
			Name:        card.Name,
			Set:         card.SetCode,
			SetName:     card.SetName,
			Game:        gameStr,
		})
		cardMap[card.ID] = card
	}

	// Track unmatched cards for API visibility
	if len(newUnmatchedCards) > 0 {
		w.mu.Lock()
		// Merge with existing unmatched cards, avoiding duplicates
		existingIDs := make(map[string]bool)
		for _, c := range w.unmatchedCards {
			existingIDs[c.CardID] = true
		}
		for _, c := range newUnmatchedCards {
			if !existingIDs[c.CardID] {
				w.unmatchedCards = append(w.unmatchedCards, c)
			}
		}
		w.mu.Unlock()

		log.Printf("ERROR: Price worker: %d cards PERMANENTLY SKIPPED (no TCGPlayerID found):", len(newUnmatchedCards))
		for _, card := range newUnmatchedCards {
			log.Printf("  - %s (#%s, set: %s)", card.Name, card.CardNumber, card.SetName)
		}
		log.Printf("These cards will not receive price updates until their TCGPlayerID is discovered. Check set mappings in tcgplayer_sync.go")
	}

	if len(lookups) == 0 {
		log.Printf("Price worker: no cards with valid IDs to update")
		return 0, nil
	}

	// Single batch POST request (all cards now have external IDs)
	result, err := w.justTCG.BatchGetPrices(lookups)
	if err != nil {
		log.Printf("Price worker: batch request failed: %v", err)
		return 0, err
	}

	// Save results
	updated := 0
	now := time.Now()
	for cardID, prices := range result.Prices {
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
				card.PriceSource = p.Source
			}
		}

		// Always update timestamp when we fetch prices (even if no NM prices returned)
		card.PriceUpdatedAt = &now

		db.Save(card)
		updated++
	}

	w.mu.Lock()
	w.cardsUpdatedToday += updated
	w.lastUpdateTime = time.Now()
	w.mu.Unlock()

	// Update Prometheus metrics
	metrics.PriceUpdatesTotal.Add(float64(updated))
	metrics.PriceUpdatesToday.Set(float64(w.cardsUpdatedToday))
	metrics.PriceQueueSize.Set(float64(w.GetQueueSize()))
	metrics.PriceBatchDuration.Observe(time.Since(start).Seconds())

	// Update JustTCG quota metrics
	metrics.JustTCGQuotaRemaining.Set(float64(w.priceService.GetJustTCGRequestsRemaining()))
	metrics.JustTCGQuotaLimit.Set(float64(w.priceService.GetJustTCGDailyLimit()))

	// Update collection metrics (includes updated values)
	metrics.UpdateCollectionMetrics(db)

	log.Printf("Price worker: batch updated %d card prices (discovered %d TCGPlayerIDs)",
		updated, len(result.DiscoveredTCGPIDs))
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
		UnmatchedCards:    w.unmatchedCards,
	}
}

// ClearUnmatchedCard removes a card from the unmatched list (e.g., after manual fix)
func (w *PriceWorker) ClearUnmatchedCard(cardID string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for i, c := range w.unmatchedCards {
		if c.CardID == cardID {
			w.unmatchedCards = append(w.unmatchedCards[:i], w.unmatchedCards[i+1:]...)
			return
		}
	}
}

// ClearAllUnmatchedCards clears the unmatched cards list
func (w *PriceWorker) ClearAllUnmatchedCards() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.unmatchedCards = nil
}
