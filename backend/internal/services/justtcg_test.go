package services

import (
	"testing"

	"github.com/codyseavey/tcg-tracker/backend/internal/models"
)

func TestNewJustTCGService(t *testing.T) {
	// Test with default limit
	svc := NewJustTCGService("test-key", 0)
	if svc.dailyLimit != 100 {
		t.Errorf("Expected default daily limit of 100, got %d", svc.dailyLimit)
	}
	if svc.apiKey != "test-key" {
		t.Errorf("Expected API key 'test-key', got %s", svc.apiKey)
	}

	// Test with custom limit
	svc = NewJustTCGService("", 200)
	if svc.dailyLimit != 200 {
		t.Errorf("Expected daily limit of 200, got %d", svc.dailyLimit)
	}
}

func TestDailyLimiting(t *testing.T) {
	svc := NewJustTCGService("", 3)

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
