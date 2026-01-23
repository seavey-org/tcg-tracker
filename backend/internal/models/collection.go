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
	ID               uint         `json:"id" gorm:"primaryKey;autoIncrement"`
	CardID           string       `json:"card_id" gorm:"not null;index"`
	Card             Card         `json:"card" gorm:"foreignKey:CardID"`
	Quantity         int          `json:"quantity" gorm:"default:1"`
	Condition        Condition    `json:"condition" gorm:"default:'NM'"`
	Printing         PrintingType `json:"printing" gorm:"default:'Normal'"`
	Language         CardLanguage `json:"language" gorm:"default:'English'"`
	Notes            string       `json:"notes"`
	AddedAt          time.Time    `json:"added_at"`
	ScannedImagePath string       `json:"scanned_image_path" gorm:"default:null"`
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
	CardID           string       `json:"card_id" binding:"required"`
	Quantity         int          `json:"quantity"`
	Condition        Condition    `json:"condition"`
	Printing         PrintingType `json:"printing"`
	Language         CardLanguage `json:"language"`
	Notes            string       `json:"notes"`
	ScannedImageData string       `json:"scanned_image_data,omitempty"` // base64 encoded
}

type UpdateCollectionRequest struct {
	Quantity  *int          `json:"quantity"`
	Condition *Condition    `json:"condition"`
	Printing  *PrintingType `json:"printing"`
	Language  *CardLanguage `json:"language"`
	Notes     *string       `json:"notes"`
}

// CollectionUpdateResponse includes the updated item plus operation info
type CollectionUpdateResponse struct {
	Item      CollectionItem `json:"item"`
	Operation string         `json:"operation"` // "updated", "split", "merged"
	Message   string         `json:"message,omitempty"`
}

// CollectionVariant summarizes items with same printing+condition+language
type CollectionVariant struct {
	Printing   PrintingType `json:"printing"`
	Condition  Condition    `json:"condition"`
	Language   CardLanguage `json:"language"`
	Quantity   int          `json:"quantity"`
	Value      float64      `json:"value"`
	HasScans   bool         `json:"has_scans"`
	ScannedQty int          `json:"scanned_qty"`
}

// GroupedCollectionItem represents a card with all its collection entries grouped
type GroupedCollectionItem struct {
	Card          Card                `json:"card"`
	TotalQuantity int                 `json:"total_quantity"`
	TotalValue    float64             `json:"total_value"`
	ScannedCount  int                 `json:"scanned_count"`
	Variants      []CollectionVariant `json:"variants"`
	Items         []CollectionItem    `json:"items"`
}
