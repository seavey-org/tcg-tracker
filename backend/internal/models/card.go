package models

import (
	"time"
)

type Game string

const (
	GameMTG     Game = "mtg"
	GamePokemon Game = "pokemon"
)

type Card struct {
	PriceUpdatedAt *time.Time `json:"price_updated_at"`
	LastPriceCheck *time.Time `json:"last_price_check"` // When we last attempted to fetch price
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	ID             string     `json:"id" gorm:"primaryKey"`
	Name           string     `json:"name" gorm:"not null;index"`
	SetName        string     `json:"set_name"`
	SetCode        string     `json:"set_code"`
	CardNumber     string     `json:"card_number"`
	Rarity         string     `json:"rarity"`
	ImageURL       string     `json:"image_url"`
	ImageURLLarge  string     `json:"image_url_large"`
	PriceSource    string     `json:"price_source"` // "api", "cached", or "pending"
	Game           Game       `json:"game" gorm:"not null;index"`
	PriceUSD       float64    `json:"price_usd"`
	PriceFoilUSD   float64    `json:"price_foil_usd"`
}

type CardSearchResult struct {
	Cards      []Card `json:"cards"`
	TotalCount int    `json:"total_count"`
	HasMore    bool   `json:"has_more"`
}
