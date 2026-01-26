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

	// MTG identification fields (from Scryfall, not persisted - for Gemini)
	TypeLine    string `json:"type_line,omitempty" gorm:"-"`    // "Creature — Goblin Wizard"
	ManaCost    string `json:"mana_cost,omitempty" gorm:"-"`    // "{2}{R}{R}"
	BorderColor string `json:"border_color,omitempty" gorm:"-"` // "black", "borderless", "white"
	Artist      string `json:"artist,omitempty" gorm:"-"`       // Artist name
}

// GetPrice returns the price for a specific condition, printing type, and language.
// Fallback order:
//  1. Exact match (condition + printing + language) in CardPrices
//  2. NM price for same printing and language (if condition is not NM)
//  3. Printing-specific fallback within same language:
//     - Foil -> Normal (for holo-only cards where JustTCG stores price as "Normal")
//     - Reverse Holo -> Normal (parallel version of Normal card, NOT related to Foil/Holo Rare)
//     - Normal -> Unlimited (for WotC-era cards)
//     - Unlimited -> Normal (for modern cards)
//     - 1st Edition -> Unlimited -> Normal (WotC-era, different print run not foil)
//  4. If non-English language has no price, fall back to English prices
//  5. Base prices (PriceFoilUSD for foil variants, PriceUSD otherwise)
//  6. Final cross-fallback: if foil has no price, try non-foil and vice versa
func (c *Card) GetPrice(condition PriceCondition, printing PrintingType, language CardLanguage) float64 {
	// Default to English if empty
	if language == "" {
		language = LanguageEnglish
	}

	// Try to find price for specified language
	price := c.getPriceForLanguage(condition, printing, language)
	if price > 0 {
		return price
	}

	// If non-English language has no price, fall back to English
	if language != LanguageEnglish {
		price = c.getPriceForLanguage(condition, printing, LanguageEnglish)
		if price > 0 {
			return price
		}
	}

	// Fall back to base prices
	if printing.IsFoilVariant() {
		if c.PriceFoilUSD > 0 {
			return c.PriceFoilUSD
		}
		// Cross-fallback: foil variant with no foil price, use non-foil
		return c.PriceUSD
	}

	if c.PriceUSD > 0 {
		return c.PriceUSD
	}
	// Cross-fallback: non-foil with no price, try foil
	return c.PriceFoilUSD
}

// getPriceForLanguage looks up price for a specific language with printing fallbacks
func (c *Card) getPriceForLanguage(condition PriceCondition, printing PrintingType, language CardLanguage) float64 {
	// 1. Look for exact condition+printing+language match
	for _, p := range c.Prices {
		if p.Condition == condition && p.Printing == printing && p.Language == language {
			return p.PriceUSD
		}
	}

	// 2. Try NM price for the same printing and language (if we're looking for non-NM)
	if condition != PriceConditionNM {
		for _, p := range c.Prices {
			if p.Condition == PriceConditionNM && p.Printing == printing && p.Language == language {
				return p.PriceUSD
			}
		}
	}

	// 3. Printing-specific fallback within same language
	if printing == PrintingFoil {
		// Foil: try Normal (for holo-only cards where JustTCG stores price as "Normal")
		for _, p := range c.Prices {
			if p.Condition == condition && p.Printing == PrintingNormal && p.Language == language {
				return p.PriceUSD
			}
		}
		if condition != PriceConditionNM {
			for _, p := range c.Prices {
				if p.Condition == PriceConditionNM && p.Printing == PrintingNormal && p.Language == language {
					return p.PriceUSD
				}
			}
		}
	} else if printing == PrintingReverseHolo {
		// Reverse Holo: fall back to Normal directly
		for _, p := range c.Prices {
			if p.Condition == condition && p.Printing == PrintingNormal && p.Language == language {
				return p.PriceUSD
			}
		}
		if condition != PriceConditionNM {
			for _, p := range c.Prices {
				if p.Condition == PriceConditionNM && p.Printing == PrintingNormal && p.Language == language {
					return p.PriceUSD
				}
			}
		}
	} else if printing == PrintingNormal {
		// Normal printing: try Unlimited as fallback (for WotC-era cards)
		for _, p := range c.Prices {
			if p.Condition == condition && p.Printing == PrintingUnlimited && p.Language == language {
				return p.PriceUSD
			}
		}
		if condition != PriceConditionNM {
			for _, p := range c.Prices {
				if p.Condition == PriceConditionNM && p.Printing == PrintingUnlimited && p.Language == language {
					return p.PriceUSD
				}
			}
		}
	} else if printing == PrintingUnlimited {
		// Unlimited printing: try Normal as fallback (modern cards use Normal)
		for _, p := range c.Prices {
			if p.Condition == condition && p.Printing == PrintingNormal && p.Language == language {
				return p.PriceUSD
			}
		}
		if condition != PriceConditionNM {
			for _, p := range c.Prices {
				if p.Condition == PriceConditionNM && p.Printing == PrintingNormal && p.Language == language {
					return p.PriceUSD
				}
			}
		}
	} else if printing == Printing1stEdition {
		// 1st Edition printing: try Unlimited -> Normal as fallback
		for _, p := range c.Prices {
			if p.Condition == condition && p.Printing == PrintingUnlimited && p.Language == language {
				return p.PriceUSD
			}
		}
		if condition != PriceConditionNM {
			for _, p := range c.Prices {
				if p.Condition == PriceConditionNM && p.Printing == PrintingUnlimited && p.Language == language {
					return p.PriceUSD
				}
			}
		}
		// Then try Normal
		for _, p := range c.Prices {
			if p.Condition == condition && p.Printing == PrintingNormal && p.Language == language {
				return p.PriceUSD
			}
		}
		if condition != PriceConditionNM {
			for _, p := range c.Prices {
				if p.Condition == PriceConditionNM && p.Printing == PrintingNormal && p.Language == language {
					return p.PriceUSD
				}
			}
		}
	}

	return 0
}

// PriceResult contains price lookup result with metadata about which language was used
type PriceResult struct {
	Price         float64      // The price found
	PriceLanguage CardLanguage // Which language's price was actually used
	IsFallback    bool         // True if fell back to a different language than requested
}

// GetPriceWithSource returns the price along with metadata about which language's
// price was actually used. This is useful for displaying to users when Japanese
// cards are priced using English market data.
func (c *Card) GetPriceWithSource(condition PriceCondition, printing PrintingType, language CardLanguage) PriceResult {
	// Default to English if empty
	if language == "" {
		language = LanguageEnglish
	}

	// Try to find price for specified language
	price := c.getPriceForLanguage(condition, printing, language)
	if price > 0 {
		return PriceResult{
			Price:         price,
			PriceLanguage: language,
			IsFallback:    false,
		}
	}

	// If non-English language has no price, fall back to English
	if language != LanguageEnglish {
		price = c.getPriceForLanguage(condition, printing, LanguageEnglish)
		if price > 0 {
			return PriceResult{
				Price:         price,
				PriceLanguage: LanguageEnglish,
				IsFallback:    true,
			}
		}
	}

	// Fall back to base prices (no language info available)
	if printing.IsFoilVariant() {
		if c.PriceFoilUSD > 0 {
			return PriceResult{
				Price:         c.PriceFoilUSD,
				PriceLanguage: LanguageEnglish, // Base prices are English
				IsFallback:    language != LanguageEnglish,
			}
		}
		// Cross-fallback: foil variant with no foil price, use non-foil
		return PriceResult{
			Price:         c.PriceUSD,
			PriceLanguage: LanguageEnglish,
			IsFallback:    language != LanguageEnglish,
		}
	}

	if c.PriceUSD > 0 {
		return PriceResult{
			Price:         c.PriceUSD,
			PriceLanguage: LanguageEnglish,
			IsFallback:    language != LanguageEnglish,
		}
	}
	// Cross-fallback: non-foil with no price, try foil
	return PriceResult{
		Price:         c.PriceFoilUSD,
		PriceLanguage: LanguageEnglish,
		IsFallback:    language != LanguageEnglish,
	}
}

type CardSearchResult struct {
	Cards      []Card `json:"cards"`
	TotalCount int    `json:"total_count"`
	HasMore    bool   `json:"has_more"`
	TopScore   int    `json:"-"` // Best match score (internal, for translation fallback)
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

// SetGroup represents a set containing cards for grouped search results
// Used for 2-phase card browsing (search by name -> see sets -> select card)
type SetGroup struct {
	SetCode     string `json:"set_code"`
	SetName     string `json:"set_name"`
	Series      string `json:"series,omitempty"`
	ReleaseDate string `json:"release_date,omitempty"`
	SymbolURL   string `json:"symbol_url,omitempty"`
	CardCount   int    `json:"card_count"`
	Cards       []Card `json:"cards"`
}

// GroupedSearchResult is returned for card name searches to enable 2-phase selection
// Phase 1: User sees sets containing cards matching the name
// Phase 2: User selects set to see variants
type GroupedSearchResult struct {
	CardName  string     `json:"card_name"`
	SetGroups []SetGroup `json:"set_groups"`
	TotalSets int        `json:"total_sets"`
}

// MTGGroupedResult is returned for MTG card scans to enable 2-phase selection
// Phase 1: User selects set from SetGroups
// Phase 2: User selects variant within the chosen set
type MTGGroupedResult struct {
	CardName  string        `json:"card_name"`
	SetGroups []MTGSetGroup `json:"set_groups"`
	TotalSets int           `json:"total_sets"`
}
