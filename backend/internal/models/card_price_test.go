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
			{Condition: PriceConditionNM, Printing: PrintingNormal, Language: LanguageEnglish, PriceUSD: 10.00},
			{Condition: PriceConditionNM, Printing: PrintingFoil, Language: LanguageEnglish, PriceUSD: 20.00},
			{Condition: PriceConditionLP, Printing: PrintingNormal, Language: LanguageEnglish, PriceUSD: 8.00},
			{Condition: PriceConditionLP, Printing: PrintingFoil, Language: LanguageEnglish, PriceUSD: 16.00},
			{Condition: PriceConditionMP, Printing: PrintingNormal, Language: LanguageEnglish, PriceUSD: 6.00},
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
		{"HP normal fallback to base", PriceConditionHP, PrintingNormal, 10.00},              // Falls back to base price
		{"DMG foil fallback to base", PriceConditionDMG, PrintingFoil, 20.00},                // Falls back to foil base price
		{"NM 1st Edition fallback to Normal", PriceConditionNM, Printing1stEdition, 10.00},   // No 1st ed price, falls back to Normal (not foil!)
		{"NM Reverse Holo fallback to Normal", PriceConditionNM, PrintingReverseHolo, 10.00}, // Reverse Holo falls back to Normal, NOT Foil
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := card.GetPrice(tt.condition, tt.printing, LanguageEnglish)
			if result != tt.expected {
				t.Errorf("GetPrice(%s, %s, English) = %f, want %f", tt.condition, tt.printing, result, tt.expected)
			}
		})
	}
}

func TestCardGetPriceLanguageFallbackToEnglish(t *testing.T) {
	card := Card{
		PriceUSD:     0,
		PriceFoilUSD: 0,
		Prices: []CardPrice{
			{Condition: PriceConditionNM, Printing: PrintingNormal, Language: LanguageEnglish, PriceUSD: 10.00},
			{Condition: PriceConditionLP, Printing: PrintingNormal, Language: LanguageEnglish, PriceUSD: 8.00},
		},
	}

	// No Japanese prices exist, should fall back to English card_prices.
	if got := card.GetPrice(PriceConditionNM, PrintingNormal, LanguageJapanese); got != 10.00 {
		t.Errorf("GetPrice(NM, Normal, Japanese) = %f, want %f", got, 10.00)
	}
	if got := card.GetPrice(PriceConditionLP, PrintingNormal, LanguageJapanese); got != 8.00 {
		t.Errorf("GetPrice(LP, Normal, Japanese) = %f, want %f", got, 8.00)
	}
}

func TestGetPriceWithSource(t *testing.T) {
	// Test card with both Japanese and English prices
	cardWithBothLanguages := Card{
		PriceUSD:     5.00,
		PriceFoilUSD: 15.00,
		Prices: []CardPrice{
			{Condition: PriceConditionNM, Printing: PrintingNormal, Language: LanguageEnglish, PriceUSD: 10.00},
			{Condition: PriceConditionNM, Printing: PrintingNormal, Language: LanguageJapanese, PriceUSD: 25.00},
		},
	}

	// Japanese price should be returned with no fallback
	result := cardWithBothLanguages.GetPriceWithSource(PriceConditionNM, PrintingNormal, LanguageJapanese)
	if result.Price != 25.00 {
		t.Errorf("Expected Japanese price 25.00, got %f", result.Price)
	}
	if result.PriceLanguage != LanguageJapanese {
		t.Errorf("Expected PriceLanguage Japanese, got %s", result.PriceLanguage)
	}
	if result.IsFallback {
		t.Error("Expected IsFallback=false when Japanese price exists")
	}

	// English price should be returned with no fallback
	result = cardWithBothLanguages.GetPriceWithSource(PriceConditionNM, PrintingNormal, LanguageEnglish)
	if result.Price != 10.00 {
		t.Errorf("Expected English price 10.00, got %f", result.Price)
	}
	if result.PriceLanguage != LanguageEnglish {
		t.Errorf("Expected PriceLanguage English, got %s", result.PriceLanguage)
	}
	if result.IsFallback {
		t.Error("Expected IsFallback=false when English price exists")
	}

	// Test card with only English prices (common case for scanned Japanese cards)
	cardWithEnglishOnly := Card{
		PriceUSD:     5.00,
		PriceFoilUSD: 15.00,
		Prices: []CardPrice{
			{Condition: PriceConditionNM, Printing: PrintingNormal, Language: LanguageEnglish, PriceUSD: 10.00},
		},
	}

	// Japanese request should fall back to English with IsFallback=true
	result = cardWithEnglishOnly.GetPriceWithSource(PriceConditionNM, PrintingNormal, LanguageJapanese)
	if result.Price != 10.00 {
		t.Errorf("Expected fallback English price 10.00, got %f", result.Price)
	}
	if result.PriceLanguage != LanguageEnglish {
		t.Errorf("Expected PriceLanguage English (fallback), got %s", result.PriceLanguage)
	}
	if !result.IsFallback {
		t.Error("Expected IsFallback=true when falling back to English for Japanese card")
	}

	// Test card with no detailed prices (should use base prices)
	cardWithBaseOnly := Card{
		PriceUSD:     5.00,
		PriceFoilUSD: 15.00,
		Prices:       []CardPrice{},
	}

	result = cardWithBaseOnly.GetPriceWithSource(PriceConditionNM, PrintingNormal, LanguageJapanese)
	if result.Price != 5.00 {
		t.Errorf("Expected base price 5.00, got %f", result.Price)
	}
	if result.PriceLanguage != LanguageEnglish {
		t.Errorf("Expected PriceLanguage English (base), got %s", result.PriceLanguage)
	}
	if !result.IsFallback {
		t.Error("Expected IsFallback=true when using base price for Japanese card")
	}

	// Foil variant should use foil base price
	result = cardWithBaseOnly.GetPriceWithSource(PriceConditionNM, PrintingFoil, LanguageJapanese)
	if result.Price != 15.00 {
		t.Errorf("Expected foil base price 15.00, got %f", result.Price)
	}
}

func TestNormalizeLanguage(t *testing.T) {
	tests := []struct {
		in   string
		want CardLanguage
	}{
		{"English", LanguageEnglish},
		{"en", LanguageEnglish},
		{"", LanguageEnglish},
		{"Japanese", LanguageJapanese},
		{"ja", LanguageJapanese},
		{"JP", LanguageJapanese},
		{"German", LanguageGerman},
		{"de", LanguageGerman},
		{"ger", LanguageGerman},
		{"French", LanguageFrench},
		{"fr", LanguageFrench},
		{"Italian", LanguageItalian},
		{"it", LanguageItalian},
		{"unknown", LanguageEnglish},
	}

	for _, tt := range tests {
		if got := NormalizeLanguage(tt.in); got != tt.want {
			t.Errorf("NormalizeLanguage(%q) = %q, want %q", tt.in, got, tt.want)
		}
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
			{Condition: PriceConditionNM, Printing: PrintingUnlimited, Language: LanguageEnglish, PriceUSD: 0.89},
			{Condition: PriceConditionLP, Printing: PrintingUnlimited, Language: LanguageEnglish, PriceUSD: 0.50},
			{Condition: PriceConditionNM, Printing: Printing1stEdition, Language: LanguageEnglish, PriceUSD: 100.00},
			{Condition: PriceConditionLP, Printing: Printing1stEdition, Language: LanguageEnglish, PriceUSD: 80.00},
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
			result := wotcCard.GetPrice(tt.condition, tt.printing, LanguageEnglish)
			if result != tt.expected {
				t.Errorf("GetPrice(%s, %s, English) = %f, want %f", tt.condition, tt.printing, result, tt.expected)
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
			{Condition: PriceConditionNM, Printing: PrintingNormal, Language: LanguageEnglish, PriceUSD: 0.68},
			{Condition: PriceConditionLP, Printing: PrintingNormal, Language: LanguageEnglish, PriceUSD: 0.50},
			{Condition: PriceConditionNM, Printing: PrintingFoil, Language: LanguageEnglish, PriceUSD: 160.25},
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
			result := card.GetPrice(tt.condition, tt.printing, LanguageEnglish)
			if result != tt.expected {
				t.Errorf("GetPrice(%s, %s, English) = %f, want %f", tt.condition, tt.printing, result, tt.expected)
			}
		})
	}
}

// TestCardGetPriceReverseHoloFallback tests the specific bug where Reverse Holo
// was incorrectly falling back to Foil prices instead of Normal prices.
// This caused cards like EX Dragon Shelgon Reverse Holo to show $177 instead of $1.46.
// Reverse Holo is a parallel foil pattern of the Normal card, NOT related to Holo Rare/Foil.
func TestCardGetPriceReverseHoloFallback(t *testing.T) {
	// Simulates EX Dragon Shelgon: JustTCG has Normal ($1.46) and Foil/Holo ($177)
	// The user has a Reverse Holo version, which should fall back to Normal
	card := &Card{
		ID:           "ex3-20",
		PriceUSD:     1.46,   // Base NM Normal price
		PriceFoilUSD: 177.00, // Holo Rare price (completely different variant)
		Prices: []CardPrice{
			{Condition: PriceConditionNM, Printing: PrintingNormal, Language: LanguageEnglish, PriceUSD: 1.46},
			{Condition: PriceConditionLP, Printing: PrintingNormal, Language: LanguageEnglish, PriceUSD: 1.00},
			{Condition: PriceConditionNM, Printing: PrintingFoil, Language: LanguageEnglish, PriceUSD: 177.00},
		},
	}

	tests := []struct {
		name      string
		condition PriceCondition
		printing  PrintingType
		expected  float64
	}{
		// Reverse Holo should NOT use the Foil/Holo Rare price!
		{"NM Reverse Holo should use Normal, not Foil", PriceConditionNM, PrintingReverseHolo, 1.46},
		{"LP Reverse Holo should use Normal, not Foil", PriceConditionLP, PrintingReverseHolo, 1.00},
		{"HP Reverse Holo should use NM Normal, not Foil", PriceConditionHP, PrintingReverseHolo, 1.46},
		// Foil should still use Foil price
		{"NM Foil uses Foil price", PriceConditionNM, PrintingFoil, 177.00},
		// Normal should use Normal price
		{"NM Normal uses Normal price", PriceConditionNM, PrintingNormal, 1.46},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := card.GetPrice(tt.condition, tt.printing, LanguageEnglish)
			if result != tt.expected {
				t.Errorf("GetPrice(%s, %s, English) = %f, want %f", tt.condition, tt.printing, result, tt.expected)
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
			{Condition: PriceConditionNM, Printing: PrintingNormal, Language: LanguageEnglish, PriceUSD: 18.84},
			{Condition: PriceConditionLP, Printing: PrintingNormal, Language: LanguageEnglish, PriceUSD: 15.00},
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
			result := holoOnlyCard.GetPrice(tt.condition, tt.printing, LanguageEnglish)
			if result != tt.expected {
				t.Errorf("GetPrice(%s, %s, English) = %f, want %f", tt.condition, tt.printing, result, tt.expected)
			}
		})
	}
}
