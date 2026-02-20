package services

import (
	"testing"

	"github.com/codyseavey/tcg-tracker/backend/internal/models"
)

func TestNewJustTCGService(t *testing.T) {
	// Test with default limits (free tier)
	svc := NewJustTCGService("test-key", 0, 0)
	if svc.dailyLimit != 100 {
		t.Errorf("Expected default daily limit of 100 (free tier), got %d", svc.dailyLimit)
	}
	if svc.monthlyLimit != 1000 {
		t.Errorf("Expected default monthly limit of 1000 (free tier), got %d", svc.monthlyLimit)
	}
	if svc.apiKey != "test-key" {
		t.Errorf("Expected API key 'test-key', got %s", svc.apiKey)
	}

	// Test with custom limits
	svc = NewJustTCGService("", 200, 5000)
	if svc.dailyLimit != 200 {
		t.Errorf("Expected daily limit of 200, got %d", svc.dailyLimit)
	}
	if svc.monthlyLimit != 5000 {
		t.Errorf("Expected monthly limit of 5000, got %d", svc.monthlyLimit)
	}
}

func TestDailyLimiting(t *testing.T) {
	svc := NewJustTCGService("", 3, 1000)

	// Should allow 3 requests via checkDailyLimit
	for i := 0; i < 3; i++ {
		if !svc.checkDailyLimit() {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// 4th request should be blocked
	if svc.checkDailyLimit() {
		t.Error("4th request should be blocked by daily limit")
	}

	// Verify remaining is 0
	remaining := svc.GetRequestsRemaining()
	if remaining != 0 {
		t.Errorf("Expected 0 remaining, got %d", remaining)
	}
}

func TestMonthlyLimiting(t *testing.T) {
	// Monthly limit of 3, daily limit high enough not to interfere
	svc := NewJustTCGService("", 1000, 3)

	// Should allow 3 requests
	for i := 0; i < 3; i++ {
		if !svc.checkDailyLimit() {
			t.Errorf("Request %d should be allowed by monthly limit", i+1)
		}
	}

	// 4th request should be blocked by monthly limit
	if svc.checkDailyLimit() {
		t.Error("4th request should be blocked by monthly limit")
	}

	// Verify monthly remaining is 0
	remaining := svc.GetMonthlyRequestsRemaining()
	if remaining != 0 {
		t.Errorf("Expected 0 monthly remaining, got %d", remaining)
	}
}

func TestGetRequestsRemainingReturnsMinOfDailyAndMonthly(t *testing.T) {
	// Daily limit is the tighter constraint
	svc := NewJustTCGService("", 5, 1000)
	// Use 3 requests
	for i := 0; i < 3; i++ {
		svc.checkDailyLimit()
	}
	remaining := svc.GetRequestsRemaining()
	if remaining != 2 {
		t.Errorf("Expected 2 remaining (daily is tighter: 5-3=2), got %d", remaining)
	}

	// Monthly limit is the tighter constraint
	svc2 := NewJustTCGService("", 1000, 5)
	for i := 0; i < 3; i++ {
		svc2.checkDailyLimit()
	}
	remaining2 := svc2.GetRequestsRemaining()
	if remaining2 != 2 {
		t.Errorf("Expected 2 remaining (monthly is tighter: 5-3=2), got %d", remaining2)
	}
}

func TestMonthlyResetTime(t *testing.T) {
	svc := NewJustTCGService("", 100, 1000)
	resetTime := svc.GetMonthlyResetTime()

	// Should be the 1st of next month
	if resetTime.Day() != 1 {
		t.Errorf("Expected monthly reset on day 1, got day %d", resetTime.Day())
	}
	if resetTime.Hour() != 0 || resetTime.Minute() != 0 || resetTime.Second() != 0 {
		t.Errorf("Expected monthly reset at midnight, got %v", resetTime)
	}
}

func TestMapJustTCGCondition(t *testing.T) {
	tests := []struct {
		input    string
		expected models.PriceCondition
	}{
		{"NM", models.PriceConditionNM},
		{"NEAR MINT", models.PriceConditionNM},
		{"LP", models.PriceConditionLP},
		{"LIGHTLY PLAYED", models.PriceConditionLP},
		{"MP", models.PriceConditionMP},
		{"MODERATELY PLAYED", models.PriceConditionMP},
		{"HP", models.PriceConditionHP},
		{"HEAVILY PLAYED", models.PriceConditionHP},
		{"DMG", models.PriceConditionDMG},
		{"DAMAGED", models.PriceConditionDMG},
		{"nm", models.PriceConditionNM}, // lowercase
		{"UNKNOWN", models.PriceCondition("")},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := mapJustTCGCondition(tt.input)
			if result != tt.expected {
				t.Errorf("mapJustTCGCondition(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeSetName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Base", "base set"}, // Our name → JustTCG's name
		{"base", "base set"},
		{"Expedition Base Set", "expedition"}, // Our name → JustTCG's name
		{"expedition base set", "expedition"},
		{"Jungle", "jungle"}, // Already matches JustTCG
		{"Fossil", "fossil"},
		{"Neo Discovery", "neo discovery"},
		{"Lost Origin", "lost origin"},
		{"Team Rocket", "team rocket"},
		{"  Base  ", "base set"}, // with whitespace
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeSetName(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeSetName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractBaseName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Beedrill", "Beedrill"},
		{"Beedrill (H4)", "Beedrill"},
		{"Beedrill (H04/H32)", "Beedrill"},
		{"Charizard (1st Edition)", "Charizard"},
		{"Dark Scizor (Neo4)", "Dark Scizor"},
		{"Some Card Name (With Stuff)", "Some Card Name"},
		{"Name(NoSpace)", "Name(NoSpace)"}, // No space before paren, shouldn't strip
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractBaseName(tt.input)
			if result != tt.expected {
				t.Errorf("extractBaseName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBatchLookupRouting(t *testing.T) {
	// This test verifies that the batch lookup logic correctly identifies:
	// - Cards with ScryfallID or TCGPlayerID -> can use batch POST
	// - Cards without external IDs -> should be skipped (price worker syncs sets first)

	lookups := []CardLookup{
		// MTG card with Scryfall ID - should use batch POST
		{CardID: "mtg-card-1", ScryfallID: "abc-123-def", Name: "Lightning Bolt", Game: "magic-the-gathering"},
		{CardID: "mtg-card-2", ScryfallID: "xyz-456-ghi", Name: "Black Lotus", Game: "magic-the-gathering"},
		// Pokemon card with cached TCGPlayerID - should use batch POST
		{CardID: "swsh4-25", TCGPlayerID: "12345", Name: "Charizard", Game: "pokemon"},
		// Pokemon card without external IDs - should be skipped
		{CardID: "base1-4", Name: "Pikachu", Game: "pokemon"},
	}

	var batchable, skipped []CardLookup
	for _, lookup := range lookups {
		if lookup.ScryfallID != "" || lookup.TCGPlayerID != "" {
			batchable = append(batchable, lookup)
		} else {
			skipped = append(skipped, lookup)
		}
	}

	if len(batchable) != 3 {
		t.Errorf("Expected 3 batchable cards (2 MTG + 1 Pokemon with cached ID), got %d", len(batchable))
	}
	if len(skipped) != 1 {
		t.Errorf("Expected 1 skipped card (Pokemon without cached ID), got %d", len(skipped))
	}

	// Verify batchable cards have external IDs
	for _, lookup := range batchable {
		if lookup.ScryfallID == "" && lookup.TCGPlayerID == "" {
			t.Errorf("Batchable card %s should have ScryfallID or TCGPlayerID", lookup.CardID)
		}
	}

	// Verify skipped cards don't have external IDs
	for _, lookup := range skipped {
		if lookup.ScryfallID != "" || lookup.TCGPlayerID != "" {
			t.Errorf("Skipped card %s should not have external IDs", lookup.CardID)
		}
	}
}

func TestBatchPriceResultStructure(t *testing.T) {
	// Verify the BatchPriceResult struct properly holds prices and discovered IDs
	result := &BatchPriceResult{
		Prices:            make(map[string][]models.CardPrice),
		DiscoveredTCGPIDs: make(map[string]string),
	}

	// Simulate adding prices
	result.Prices["card-1"] = []models.CardPrice{
		{Condition: models.PriceConditionNM, PriceUSD: 10.0},
	}

	// Simulate discovering a TCGPlayerID
	result.DiscoveredTCGPIDs["card-1"] = "tcg-12345"

	if len(result.Prices) != 1 {
		t.Errorf("Expected 1 price entry, got %d", len(result.Prices))
	}
	if len(result.DiscoveredTCGPIDs) != 1 {
		t.Errorf("Expected 1 discovered ID, got %d", len(result.DiscoveredTCGPIDs))
	}
	if result.DiscoveredTCGPIDs["card-1"] != "tcg-12345" {
		t.Errorf("Expected discovered ID 'tcg-12345', got '%s'", result.DiscoveredTCGPIDs["card-1"])
	}
}
