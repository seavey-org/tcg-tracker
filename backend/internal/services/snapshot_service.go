package services

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/codyseavey/tcg-tracker/backend/internal/database"
	"github.com/codyseavey/tcg-tracker/backend/internal/models"
)

// SnapshotService handles collection value snapshots
type SnapshotService struct {
	mu            sync.RWMutex
	lastSnapshot  time.Time
	snapshotHour  int // Hour of day to take snapshot (0-23)
	checkInterval time.Duration
}

// NewSnapshotService creates a new snapshot service
func NewSnapshotService() *SnapshotService {
	return &SnapshotService{
		snapshotHour:  23, // Default: 11 PM
		checkInterval: 15 * time.Minute,
	}
}

// Start begins the background snapshot worker
func (s *SnapshotService) Start(ctx context.Context) {
	log.Println("Snapshot service started: will record daily collection value")

	// Check if we need to take a snapshot for today on startup
	s.checkAndSnapshot()

	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Snapshot service stopping...")
			return
		case <-ticker.C:
			s.checkAndSnapshot()
		}
	}
}

// checkAndSnapshot checks if a snapshot is needed and takes one
func (s *SnapshotService) checkAndSnapshot() {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Check if we already have a snapshot for today
	if s.hasSnapshotForDate(today) {
		return
	}

	// Only take automatic snapshots at or after the configured hour
	if now.Hour() >= s.snapshotHour {
		if err := s.TakeSnapshot(); err != nil {
			log.Printf("Snapshot service: failed to take snapshot: %v", err)
		}
	}
}

// hasSnapshotForDate checks if a snapshot exists for the given date
func (s *SnapshotService) hasSnapshotForDate(date time.Time) bool {
	db := database.GetDB()

	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	var count int64
	db.Model(&models.CollectionValueSnapshot{}).
		Where("snapshot_date >= ? AND snapshot_date < ?", startOfDay, endOfDay).
		Count(&count)

	return count > 0
}

// TakeSnapshot records the current collection value
func (s *SnapshotService) TakeSnapshot() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	db := database.GetDB()
	now := time.Now()
	snapshotDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Calculate current stats (reusing the same logic as GetStats handler)
	stats := s.calculateStats()

	snapshot := models.CollectionValueSnapshot{
		SnapshotDate: snapshotDate,
		TotalCards:   stats.TotalCards,
		UniqueCards:  stats.UniqueCards,
		TotalValue:   stats.TotalValue,
		MTGCards:     stats.MTGCards,
		PokemonCards: stats.PokemonCards,
		MTGValue:     stats.MTGValue,
		PokemonValue: stats.PokemonValue,
		CreatedAt:    now,
	}

	// Use upsert to handle duplicate dates
	result := db.Where("DATE(snapshot_date) = DATE(?)", snapshotDate).
		Assign(models.CollectionValueSnapshot{
			TotalCards:   snapshot.TotalCards,
			UniqueCards:  snapshot.UniqueCards,
			TotalValue:   snapshot.TotalValue,
			MTGCards:     snapshot.MTGCards,
			PokemonCards: snapshot.PokemonCards,
			MTGValue:     snapshot.MTGValue,
			PokemonValue: snapshot.PokemonValue,
		}).
		FirstOrCreate(&snapshot)

	if result.Error != nil {
		return result.Error
	}

	s.lastSnapshot = now
	log.Printf("Snapshot service: recorded value snapshot for %s (total: $%.2f, cards: %d)",
		snapshotDate.Format("2006-01-02"), stats.TotalValue, stats.TotalCards)

	return nil
}

// calculateStats computes current collection statistics
func (s *SnapshotService) calculateStats() models.CollectionStats {
	db := database.GetDB()
	var stats models.CollectionStats

	// Total and unique cards
	db.Model(&models.CollectionItem{}).Select("COALESCE(SUM(quantity), 0)").Scan(&stats.TotalCards)
	var uniqueCount int64
	db.Model(&models.CollectionItem{}).Distinct("card_id").Count(&uniqueCount)
	stats.UniqueCards = int(uniqueCount)

	// Calculate values and counts by game using condition-appropriate prices
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

	return stats
}

// GetHistory retrieves value snapshots for a given period
func (s *SnapshotService) GetHistory(period string) ([]models.CollectionValueSnapshot, error) {
	db := database.GetDB()
	var snapshots []models.CollectionValueSnapshot

	now := time.Now()
	var startDate time.Time

	switch period {
	case "week":
		startDate = now.AddDate(0, 0, -7)
	case "month":
		startDate = now.AddDate(0, -1, 0)
	case "3month":
		startDate = now.AddDate(0, -3, 0)
	case "year":
		startDate = now.AddDate(-1, 0, 0)
	case "all":
		startDate = time.Time{} // No filter
	default:
		startDate = now.AddDate(0, -1, 0) // Default to 1 month
	}

	query := db.Order("snapshot_date ASC")
	if !startDate.IsZero() {
		query = query.Where("snapshot_date >= ?", startDate)
	}

	if err := query.Find(&snapshots).Error; err != nil {
		return nil, err
	}

	return snapshots, nil
}

// GetLastSnapshot returns the most recent snapshot
func (s *SnapshotService) GetLastSnapshot() *models.CollectionValueSnapshot {
	db := database.GetDB()
	var snapshot models.CollectionValueSnapshot

	if err := db.Order("snapshot_date DESC").First(&snapshot).Error; err != nil {
		return nil
	}

	return &snapshot
}

// ForceTakeSnapshot takes a snapshot regardless of timing (for manual triggers)
func (s *SnapshotService) ForceTakeSnapshot() error {
	return s.TakeSnapshot()
}
