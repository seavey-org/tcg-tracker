package models

import (
	"time"
)

type Condition string

const (
	ConditionMint      Condition = "M"
	ConditionNearMint  Condition = "NM"
	ConditionExcellent Condition = "EX"
	ConditionGood      Condition = "GD"
	ConditionLightPlay Condition = "LP"
	ConditionPlayed    Condition = "PL"
	ConditionPoor      Condition = "PR"
)

type CollectionItem struct {
	Card      Card      `json:"card" gorm:"foreignKey:CardID"`
	AddedAt   time.Time `json:"added_at"`
	CardID    string    `json:"card_id" gorm:"not null;index"`
	Notes     string    `json:"notes"`
	Condition Condition `json:"condition" gorm:"default:'NM'"`
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Quantity  int       `json:"quantity" gorm:"default:1"`
	Foil      bool      `json:"foil" gorm:"default:false"`
}

type CollectionStats struct {
	TotalCards   int     `json:"total_cards"`
	UniqueCards  int     `json:"unique_cards"`
	TotalValue   float64 `json:"total_value"`
	MTGCards     int     `json:"mtg_cards"`
	PokemonCards int     `json:"pokemon_cards"`
	MTGValue     float64 `json:"mtg_value"`
	PokemonValue float64 `json:"pokemon_value"`
}

type AddToCollectionRequest struct {
	CardID    string    `json:"card_id" binding:"required"`
	Notes     string    `json:"notes"`
	Condition Condition `json:"condition"`
	Quantity  int       `json:"quantity"`
	Foil      bool      `json:"foil"`
}

type UpdateCollectionRequest struct {
	Quantity  *int       `json:"quantity"`
	Condition *Condition `json:"condition"`
	Foil      *bool      `json:"foil"`
	Notes     *string    `json:"notes"`
}
