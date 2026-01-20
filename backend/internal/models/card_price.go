package models

import (
	"time"
)

// PriceCondition represents the condition for pricing purposes
// Maps to JustTCG conditions
type PriceCondition string

const (
	PriceConditionNM  PriceCondition = "NM"  // Near Mint
	PriceConditionLP  PriceCondition = "LP"  // Lightly Played
	PriceConditionMP  PriceCondition = "MP"  // Moderately Played
	PriceConditionHP  PriceCondition = "HP"  // Heavily Played
	PriceConditionDMG PriceCondition = "DMG" // Damaged
)

// PrintingType represents card printing variants from JustTCG API
type PrintingType string

const (
	PrintingNormal      PrintingType = "Normal"
	PrintingFoil        PrintingType = "Foil"
	Printing1stEdition  PrintingType = "1st Edition"
	PrintingUnlimited   PrintingType = "Unlimited"
	PrintingReverseHolo PrintingType = "Reverse Holofoil"
)

// AllPrintingTypes returns all valid printing types
func AllPrintingTypes() []PrintingType {
	return []PrintingType{
		PrintingNormal,
		PrintingFoil,
		Printing1stEdition,
		PrintingUnlimited,
		PrintingReverseHolo,
	}
}

// IsFoilVariant returns true if this printing type is an actual foil/holographic variant.
// 1st Edition is NOT a foil variant - it's a different print run of the same card.
func (p PrintingType) IsFoilVariant() bool {
	return p == PrintingFoil || p == PrintingReverseHolo
}

// CardPrice stores condition and printing-specific prices for a card
type CardPrice struct {
	ID             uint           `json:"id" gorm:"primaryKey"`
	CardID         string         `json:"card_id" gorm:"not null;uniqueIndex:idx_card_cond_print"`
	Condition      PriceCondition `json:"condition" gorm:"not null;uniqueIndex:idx_card_cond_print"`
	Printing       PrintingType   `json:"printing" gorm:"not null;uniqueIndex:idx_card_cond_print;default:'Normal'"`
	PriceUSD       float64        `json:"price_usd"`
	Source         string         `json:"source"` // "justtcg" (sole price source)
	PriceUpdatedAt *time.Time     `json:"price_updated_at"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// MapCollectionConditionToPriceCondition maps the app's collection condition
// to the price condition used by JustTCG
func MapCollectionConditionToPriceCondition(condition Condition) PriceCondition {
	switch condition {
	case ConditionMint, ConditionNearMint:
		return PriceConditionNM
	case ConditionExcellent, ConditionLightPlay:
		return PriceConditionLP
	case ConditionGood:
		return PriceConditionMP
	case ConditionPlayed:
		return PriceConditionHP
	case ConditionPoor:
		return PriceConditionDMG
	default:
		return PriceConditionNM
	}
}

// AllPriceConditions returns all valid price conditions
func AllPriceConditions() []PriceCondition {
	return []PriceCondition{
		PriceConditionNM,
		PriceConditionLP,
		PriceConditionMP,
		PriceConditionHP,
		PriceConditionDMG,
	}
}

// DerivePrintingFromLegacy converts legacy foil/firstEdition bools to PrintingType
// Used for migration and backward compatibility
func DerivePrintingFromLegacy(foil, firstEdition bool) PrintingType {
	if firstEdition {
		return Printing1stEdition
	}
	if foil {
		return PrintingFoil
	}
	return PrintingNormal
}
