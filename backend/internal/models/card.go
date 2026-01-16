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
	ID             string     `json:"id" gorm:"primaryKey"`
	Game           Game       `json:"game" gorm:"not null;index"`
	Name           string     `json:"name" gorm:"not null;index"`
	SetName        string     `json:"set_name"`
	SetCode        string     `json:"set_code"`
	CardNumber     string     `json:"card_number"`
	Rarity         string     `json:"rarity"`
	ImageURL       string     `json:"image_url"`
	ImageURLLarge  string     `json:"image_url_large"`
	PriceUSD       float64    `json:"price_usd"`
	PriceFoilUSD   float64    `json:"price_foil_usd"`
	PriceUpdatedAt *time.Time `json:"price_updated_at"`
	PriceSource    string     `json:"price_source"`      // "api", "cached", or "pending"
	LastPriceCheck *time.Time `json:"last_price_check"`  // When we last attempted to fetch price
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type CardSearchResult struct {
	Cards      []Card `json:"cards"`
	TotalCount int    `json:"total_count"`
	HasMore    bool   `json:"has_more"`
}
