package models

import (
	"time"
)

// CollectionValueSnapshot stores daily collection value for historical tracking
type CollectionValueSnapshot struct {
	ID           uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	SnapshotDate time.Time `json:"snapshot_date" gorm:"uniqueIndex;not null"`
	TotalCards   int       `json:"total_cards"`
	UniqueCards  int       `json:"unique_cards"`
	TotalValue   float64   `json:"total_value"`
	MTGCards     int       `json:"mtg_cards"`
	PokemonCards int       `json:"pokemon_cards"`
	MTGValue     float64   `json:"mtg_value"`
	PokemonValue float64   `json:"pokemon_value"`
	CreatedAt    time.Time `json:"created_at"`
}

// ValueHistoryResponse is the API response for value history
type ValueHistoryResponse struct {
	Snapshots []CollectionValueSnapshot `json:"snapshots"`
	Period    string                    `json:"period"` // "week", "month", "year", "all"
}
