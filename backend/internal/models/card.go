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

	// MTG variant info (from Scryfall, not persisted)
	Finishes     []string `json:"finishes,omitempty" gorm:"-"`      // nonfoil, foil, etched
	FrameEffects []string `json:"frame_effects,omitempty" gorm:"-"` // showcase, borderless, extendedart
	PromoTypes   []string `json:"promo_types,omitempty" gorm:"-"`   // buyabox, prerelease
	ReleasedAt   string   `json:"released_at,omitempty" gorm:"-"`   // Set release date
}

// GetPrice returns the price for a specific condition and printing type.
// Fallback order:
//  1. Exact match (condition + printing) in CardPrices
//  2. NM price for the same printing (if condition is not NM)
//  3. Printing-specific fallback:
//     - Foil -> Normal (for holo-only cards where JustTCG stores price as "Normal")
//     - Reverse Holo -> Normal (parallel version of Normal card, NOT related to Foil/Holo Rare)
//     - Normal -> Unlimited (for WotC-era cards)
//     - Unlimited -> Normal (for modern cards)
//     - 1st Edition -> Unlimited -> Normal (WotC-era, different print run not foil)
//  4. Base prices (PriceFoilUSD for foil variants, PriceUSD otherwise)
//  5. Final cross-fallback: if foil has no price, try non-foil and vice versa
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

	// 3. Printing-specific fallback
	if printing == PrintingFoil {
		// Foil: try Normal (for holo-only cards where JustTCG stores price as "Normal")
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
	} else if printing == PrintingReverseHolo {
		// Reverse Holo: fall back to Normal directly
		// Reverse Holo is a parallel foil pattern of the Normal card, NOT related to Holo Rare/Foil
		// A Reverse Holo common ($2) should NOT fall back to Holo Rare ($177)
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

// MTGSetGroup represents a set containing variants of a scanned card
// Used for 2-phase MTG card selection (select set, then select variant)
type MTGSetGroup struct {
	SetCode     string `json:"set_code"`
	SetName     string `json:"set_name"`
	ReleasedAt  string `json:"released_at,omitempty"`
	IsBestMatch bool   `json:"is_best_match"`
	MatchScore  int    `json:"-"` // Internal scoring for sort order (not exposed to API)
	Variants    []Card `json:"variants"`
}

// MTGGroupedResult is returned for MTG card scans to enable 2-phase selection
// Phase 1: User selects set from SetGroups
// Phase 2: User selects variant within the chosen set
type MTGGroupedResult struct {
	CardName  string        `json:"card_name"`
	SetGroups []MTGSetGroup `json:"set_groups"`
	TotalSets int           `json:"total_sets"`
}
