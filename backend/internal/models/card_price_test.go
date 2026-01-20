package models

import (
	"testing"
)

func TestMapCollectionConditionToPriceCondition(t *testing.T) {
	tests := []struct {
		name      string
		condition Condition
		expected  PriceCondition
	}{
		{"Mint maps to NM", ConditionMint, PriceConditionNM},
		{"Near Mint maps to NM", ConditionNearMint, PriceConditionNM},
		{"Excellent maps to LP", ConditionExcellent, PriceConditionLP},
		{"Light Play maps to LP", ConditionLightPlay, PriceConditionLP},
		{"Good maps to MP", ConditionGood, PriceConditionMP},
		{"Played maps to HP", ConditionPlayed, PriceConditionHP},
		{"Poor maps to DMG", ConditionPoor, PriceConditionDMG},
		{"Unknown defaults to NM", Condition("UNKNOWN"), PriceConditionNM},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MapCollectionConditionToPriceCondition(tt.condition)
			if result != tt.expected {
				t.Errorf("MapCollectionConditionToPriceCondition(%s) = %s, want %s", tt.condition, result, tt.expected)
			}
		})
	}
}

func TestAllPriceConditions(t *testing.T) {
	conditions := AllPriceConditions()

	// Should have 5 conditions
	if len(conditions) != 5 {
		t.Errorf("AllPriceConditions() returned %d conditions, want 5", len(conditions))
	}

	// Verify all expected conditions are present
	expected := map[PriceCondition]bool{
		PriceConditionNM:  false,
		PriceConditionLP:  false,
		PriceConditionMP:  false,
		PriceConditionHP:  false,
		PriceConditionDMG: false,
	}

	for _, cond := range conditions {
		if _, ok := expected[cond]; !ok {
			t.Errorf("Unexpected condition: %s", cond)
		}
		expected[cond] = true
	}

	for cond, found := range expected {
		if !found {
			t.Errorf("Missing condition: %s", cond)
		}
	}
}

func TestCardGetPrice(t *testing.T) {
	card := &Card{
		ID:           "test-card",
		PriceUSD:     10.00,
		PriceFoilUSD: 20.00,
		Prices: []CardPrice{
			{Condition: PriceConditionNM, Printing: PrintingNormal, PriceUSD: 10.00},
			{Condition: PriceConditionNM, Printing: PrintingFoil, PriceUSD: 20.00},
			{Condition: PriceConditionLP, Printing: PrintingNormal, PriceUSD: 8.00},
			{Condition: PriceConditionLP, Printing: PrintingFoil, PriceUSD: 16.00},
			{Condition: PriceConditionMP, Printing: PrintingNormal, PriceUSD: 6.00},
		},
	}

	tests := []struct {
		name      string
		condition PriceCondition
		printing  PrintingType
		expected  float64
	}{
		{"NM normal", PriceConditionNM, PrintingNormal, 10.00},
		{"NM foil", PriceConditionNM, PrintingFoil, 20.00},
		{"LP normal", PriceConditionLP, PrintingNormal, 8.00},
		{"LP foil", PriceConditionLP, PrintingFoil, 16.00},
		{"MP normal", PriceConditionMP, PrintingNormal, 6.00},
		{"HP normal fallback to base", PriceConditionHP, PrintingNormal, 10.00},            // Falls back to base price
		{"DMG foil fallback to base", PriceConditionDMG, PrintingFoil, 20.00},              // Falls back to foil base price
		{"NM 1st Edition fallback to Normal", PriceConditionNM, Printing1stEdition, 10.00}, // No 1st ed price, falls back to Normal (not foil!)
		{"NM Reverse Holo fallback to foil", PriceConditionNM, PrintingReverseHolo, 20.00}, // No reverse price, IsFoilVariant=true
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := card.GetPrice(tt.condition, tt.printing)
			if result != tt.expected {
				t.Errorf("GetPrice(%s, %s) = %f, want %f", tt.condition, tt.printing, result, tt.expected)
			}
		})
	}
}

// TestCardGetPriceWotCCard tests the fallback behavior for WotC-era cards
// where JustTCG stores prices as "Unlimited" and "1st Edition" instead of "Normal".
// When the user adds a card with default printing "Normal", we should use Unlimited prices.
func TestCardGetPriceWotCCard(t *testing.T) {
	// Simulates a WotC-era card like Grimer from Team Rocket
	// JustTCG returns "Unlimited" (~$1) and "1st Edition" (~$100) prices
	wotcCard := &Card{
		ID:           "base5-57",
		PriceUSD:     0.89,   // Base price from Unlimited
		PriceFoilUSD: 100.00, // Base foil price (1st Edition is NOT a foil variant)
		Prices: []CardPrice{
			{Condition: PriceConditionNM, Printing: PrintingUnlimited, PriceUSD: 0.89},
			{Condition: PriceConditionLP, Printing: PrintingUnlimited, PriceUSD: 0.50},
			{Condition: PriceConditionNM, Printing: Printing1stEdition, PriceUSD: 100.00},
			{Condition: PriceConditionLP, Printing: Printing1stEdition, PriceUSD: 80.00},
		},
	}

	tests := []struct {
		name      string
		condition PriceCondition
		printing  PrintingType
		expected  float64
	}{
		// Normal printing should fall back to Unlimited (the common WotC variant)
		{"NM Normal falls back to Unlimited", PriceConditionNM, PrintingNormal, 0.89},
		{"LP Normal falls back to Unlimited", PriceConditionLP, PrintingNormal, 0.50},
		{"HP Normal falls back to NM Unlimited", PriceConditionHP, PrintingNormal, 0.89},
		// Unlimited printing should work directly
		{"NM Unlimited exact match", PriceConditionNM, PrintingUnlimited, 0.89},
		{"LP Unlimited exact match", PriceConditionLP, PrintingUnlimited, 0.50},
		// 1st Edition should return the expensive price
		{"NM 1st Edition exact match", PriceConditionNM, Printing1stEdition, 100.00},
		{"LP 1st Edition exact match", PriceConditionLP, Printing1stEdition, 80.00},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wotcCard.GetPrice(tt.condition, tt.printing)
			if result != tt.expected {
				t.Errorf("GetPrice(%s, %s) = %f, want %f", tt.condition, tt.printing, result, tt.expected)
			}
		})
	}
}

// TestCardGetPrice1stEditionFallback tests the specific bug where 1st Edition
// was incorrectly falling back to Foil prices instead of Unlimited/Normal.
// This caused cards like Team Rocket Grimer to show $320 instead of $1.
func TestCardGetPrice1stEditionFallback(t *testing.T) {
	// Simulates Team Rocket Grimer: JustTCG has Normal ($0.68) and Foil ($160.25)
	// but the user has marked their card as "1st Edition"
	// 1st Edition should fall back to Normal/Unlimited, NOT Foil
	card := &Card{
		ID:           "base5-57",
		PriceUSD:     0.68,   // Base NM Normal price
		PriceFoilUSD: 160.25, // Base foil price (expensive holo version)
		Prices: []CardPrice{
			{Condition: PriceConditionNM, Printing: PrintingNormal, PriceUSD: 0.68},
			{Condition: PriceConditionLP, Printing: PrintingNormal, PriceUSD: 0.50},
			{Condition: PriceConditionNM, Printing: PrintingFoil, PriceUSD: 160.25},
		},
	}

	tests := []struct {
		name      string
		condition PriceCondition
		printing  PrintingType
		expected  float64
	}{
		// 1st Edition should NOT use the Foil price!
		{"NM 1st Edition should use Normal, not Foil", PriceConditionNM, Printing1stEdition, 0.68},
		{"LP 1st Edition should use Normal, not Foil", PriceConditionLP, Printing1stEdition, 0.50},
		{"HP 1st Edition should use NM Normal, not Foil", PriceConditionHP, Printing1stEdition, 0.68},
		// Foil should still use Foil price
		{"NM Foil uses Foil price", PriceConditionNM, PrintingFoil, 160.25},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := card.GetPrice(tt.condition, tt.printing)
			if result != tt.expected {
				t.Errorf("GetPrice(%s, %s) = %f, want %f", tt.condition, tt.printing, result, tt.expected)
			}
		})
	}
}

// TestCardGetPriceHoloOnlyCard tests the fallback behavior for holo-only cards
// where JustTCG stores the price under "Normal" printing (since there's no non-holo version).
// When the user marks the collection item as "Foil", we should still return the price.
func TestCardGetPriceHoloOnlyCard(t *testing.T) {
	// Simulates a holo-only card like Forretress from Neo Discovery
	// JustTCG stores the price as "Normal" since there's no non-holo variant
	holoOnlyCard := &Card{
		ID:           "neo2-2",
		PriceUSD:     18.84, // Base price (from Normal printing)
		PriceFoilUSD: 0,     // No separate foil price (it's the only variant)
		Prices: []CardPrice{
			// JustTCG only returns "Normal" printing for holo-only cards
			{Condition: PriceConditionNM, Printing: PrintingNormal, PriceUSD: 18.84},
			{Condition: PriceConditionLP, Printing: PrintingNormal, PriceUSD: 15.00},
		},
	}

	tests := []struct {
		name      string
		condition PriceCondition
		printing  PrintingType
		expected  float64
	}{
		// User marks card as "Foil" but JustTCG only has "Normal" prices
		{"NM Foil uses Normal price", PriceConditionNM, PrintingFoil, 18.84},
		{"LP Foil uses Normal price", PriceConditionLP, PrintingFoil, 15.00},
		{"HP Foil uses NM Normal fallback", PriceConditionHP, PrintingFoil, 18.84},
		// Normal printing should work as expected
		{"NM Normal exact match", PriceConditionNM, PrintingNormal, 18.84},
		{"LP Normal exact match", PriceConditionLP, PrintingNormal, 15.00},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := holoOnlyCard.GetPrice(tt.condition, tt.printing)
			if result != tt.expected {
				t.Errorf("GetPrice(%s, %s) = %f, want %f", tt.condition, tt.printing, result, tt.expected)
			}
		})
	}
}
