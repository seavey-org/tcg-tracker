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
	PriceUpdatedAt *time.Time  `json:"price_updated_at"`
	LastPriceCheck *time.Time  `json:"last_price_check"` // When we last attempted to fetch price
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
	ID             string      `json:"id" gorm:"primaryKey"`
	Name           string      `json:"name" gorm:"not null;index"`
	SetName        string      `json:"set_name"`
	SetCode        string      `json:"set_code"`
	CardNumber     string      `json:"card_number"`
	Rarity         string      `json:"rarity"`
	ImageURL       string      `json:"image_url"`
	ImageURLLarge  string      `json:"image_url_large"`
	PriceSource    string      `json:"price_source"` // "api", "cached", or "pending"
	TCGPlayerID    string      `json:"tcgplayer_id"` // Cached from JustTCG for batch lookups
	Game           Game        `json:"game" gorm:"not null;index"`
	PriceUSD       float64     `json:"price_usd"`      // Backward compat: NM non-foil price
	PriceFoilUSD   float64     `json:"price_foil_usd"` // Backward compat: NM foil price
	Prices         []CardPrice `json:"prices,omitempty" gorm:"foreignKey:CardID;references:ID"`
}

// GetPrice returns the price for a specific condition and printing type.
// Fallback order:
//  1. Exact match (condition + printing) in CardPrices
//  2. NM price for the same printing (if condition is not NM)
//  3. For foil variants (Reverse Holo): try standard Foil price first
//  4. Cross-printing fallback:
//     - Foil/ReverseHolo -> Normal (for holo-only cards)
//     - Normal -> Unlimited (for WotC-era cards)
//     - Unlimited -> Normal (for modern cards)
//     - 1st Edition -> Unlimited -> Normal (WotC-era, different print run not foil)
//  5. Base prices (PriceFoilUSD for foil variants, PriceUSD otherwise)
//  6. Final cross-fallback: if foil has no price, try non-foil and vice versa
func (c *Card) GetPrice(condition PriceCondition, printing PrintingType) float64 {
	// 1. Look for exact condition+printing match
	for _, p := range c.Prices {
		if p.Condition == condition && p.Printing == printing {
			return p.PriceUSD
		}
	}

	// 2. Try NM price for the same printing (if we're looking for non-NM)
	if condition != PriceConditionNM {
		for _, p := range c.Prices {
			if p.Condition == PriceConditionNM && p.Printing == printing {
				return p.PriceUSD
			}
		}
	}

	// 3. For foil variants (1st Ed, Reverse Holo), try standard Foil price
	if printing.IsFoilVariant() && printing != PrintingFoil {
		for _, p := range c.Prices {
			if p.Condition == condition && p.Printing == PrintingFoil {
				return p.PriceUSD
			}
		}
		// Try NM Foil
		if condition != PriceConditionNM {
			for _, p := range c.Prices {
				if p.Condition == PriceConditionNM && p.Printing == PrintingFoil {
					return p.PriceUSD
				}
			}
		}
	}

	// 4. Cross-printing fallback (for holo-only cards where JustTCG stores price as "Normal")
	if printing.IsFoilVariant() {
		// Foil variants: try Normal price (holo-only cards)
		for _, p := range c.Prices {
			if p.Condition == condition && p.Printing == PrintingNormal {
				return p.PriceUSD
			}
		}
		if condition != PriceConditionNM {
			for _, p := range c.Prices {
				if p.Condition == PriceConditionNM && p.Printing == PrintingNormal {
					return p.PriceUSD
				}
			}
		}
	} else if printing == PrintingNormal {
		// Normal printing: try Unlimited as fallback (for WotC-era cards)
		for _, p := range c.Prices {
			if p.Condition == condition && p.Printing == PrintingUnlimited {
				return p.PriceUSD
			}
		}
		if condition != PriceConditionNM {
			for _, p := range c.Prices {
				if p.Condition == PriceConditionNM && p.Printing == PrintingUnlimited {
					return p.PriceUSD
				}
			}
		}
	} else if printing == PrintingUnlimited {
		// Unlimited printing: try Normal as fallback (modern cards use Normal)
		for _, p := range c.Prices {
			if p.Condition == condition && p.Printing == PrintingNormal {
				return p.PriceUSD
			}
		}
		if condition != PriceConditionNM {
			for _, p := range c.Prices {
				if p.Condition == PriceConditionNM && p.Printing == PrintingNormal {
					return p.PriceUSD
				}
			}
		}
	} else if printing == Printing1stEdition {
		// 1st Edition printing: try Unlimited -> Normal as fallback
		// 1st Edition is NOT a foil, it's a different WotC-era print run
		for _, p := range c.Prices {
			if p.Condition == condition && p.Printing == PrintingUnlimited {
				return p.PriceUSD
			}
		}
		if condition != PriceConditionNM {
			for _, p := range c.Prices {
				if p.Condition == PriceConditionNM && p.Printing == PrintingUnlimited {
					return p.PriceUSD
				}
			}
		}
		// Then try Normal
		for _, p := range c.Prices {
			if p.Condition == condition && p.Printing == PrintingNormal {
				return p.PriceUSD
			}
		}
		if condition != PriceConditionNM {
			for _, p := range c.Prices {
				if p.Condition == PriceConditionNM && p.Printing == PrintingNormal {
					return p.PriceUSD
				}
			}
		}
	}

	// 5. Fall back to base prices
	if printing.IsFoilVariant() {
		if c.PriceFoilUSD > 0 {
			return c.PriceFoilUSD
		}
		// 6. Cross-fallback: foil variant with no foil price, use non-foil
		return c.PriceUSD
	}

	if c.PriceUSD > 0 {
		return c.PriceUSD
	}
	// 6. Cross-fallback: non-foil with no price, try foil
	return c.PriceFoilUSD
}

type CardSearchResult struct {
	Cards      []Card `json:"cards"`
	TotalCount int    `json:"total_count"`
	HasMore    bool   `json:"has_more"`
}
