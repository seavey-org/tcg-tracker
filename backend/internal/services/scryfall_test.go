package services

import (
	"testing"

	"github.com/codyseavey/tcg-tracker/backend/internal/models"
)

func TestGroupCardsBySet_Empty(t *testing.T) {
	result := GroupCardsBySet([]models.Card{}, "", "", "", "")

	if result.CardName != "" {
		t.Errorf("expected empty card name, got %s", result.CardName)
	}
	if len(result.SetGroups) != 0 {
		t.Errorf("expected 0 set groups, got %d", len(result.SetGroups))
	}
	if result.TotalSets != 0 {
		t.Errorf("expected 0 total sets, got %d", result.TotalSets)
	}
}

func TestGroupCardsBySet_SingleSet(t *testing.T) {
	cards := []models.Card{
		{ID: "1", Name: "Lightning Bolt", SetCode: "neo", SetName: "Kamigawa: Neon Dynasty", ReleasedAt: "2022-02-18", Finishes: []string{"nonfoil"}},
		{ID: "2", Name: "Lightning Bolt", SetCode: "neo", SetName: "Kamigawa: Neon Dynasty", ReleasedAt: "2022-02-18", Finishes: []string{"foil"}},
	}

	result := GroupCardsBySet(cards, "", "", "", "")

	if result.CardName != "Lightning Bolt" {
		t.Errorf("expected card name 'Lightning Bolt', got %s", result.CardName)
	}
	if len(result.SetGroups) != 1 {
		t.Errorf("expected 1 set group, got %d", len(result.SetGroups))
	}
	if result.TotalSets != 1 {
		t.Errorf("expected 1 total set, got %d", result.TotalSets)
	}
	if len(result.SetGroups[0].Variants) != 2 {
		t.Errorf("expected 2 variants, got %d", len(result.SetGroups[0].Variants))
	}
}

func TestGroupCardsBySet_MultipleSets(t *testing.T) {
	cards := []models.Card{
		{ID: "1", Name: "Lightning Bolt", SetCode: "neo", SetName: "Kamigawa: Neon Dynasty", ReleasedAt: "2022-02-18"},
		{ID: "2", Name: "Lightning Bolt", SetCode: "2xm", SetName: "Double Masters", ReleasedAt: "2020-08-07"},
		{ID: "3", Name: "Lightning Bolt", SetCode: "sta", SetName: "Strixhaven Mystical Archive", ReleasedAt: "2021-04-23"},
	}

	result := GroupCardsBySet(cards, "", "", "", "")

	if result.TotalSets != 3 {
		t.Errorf("expected 3 total sets, got %d", result.TotalSets)
	}

	// Verify sorting by release date (newest first) when no OCR data
	if result.SetGroups[0].SetCode != "neo" {
		t.Errorf("expected first set to be 'neo' (newest), got %s", result.SetGroups[0].SetCode)
	}
	if result.SetGroups[1].SetCode != "sta" {
		t.Errorf("expected second set to be 'sta', got %s", result.SetGroups[1].SetCode)
	}
	if result.SetGroups[2].SetCode != "2xm" {
		t.Errorf("expected third set to be '2xm' (oldest), got %s", result.SetGroups[2].SetCode)
	}
}

func TestGroupCardsBySet_BestMatchBySetCode(t *testing.T) {
	cards := []models.Card{
		{ID: "1", Name: "Lightning Bolt", SetCode: "neo", SetName: "Kamigawa: Neon Dynasty", ReleasedAt: "2022-02-18"},
		{ID: "2", Name: "Lightning Bolt", SetCode: "2xm", SetName: "Double Masters", ReleasedAt: "2020-08-07"},
		{ID: "3", Name: "Lightning Bolt", SetCode: "sta", SetName: "Strixhaven Mystical Archive", ReleasedAt: "2021-04-23"},
	}

	// OCR detected set code "sta" (score: 100)
	result := GroupCardsBySet(cards, "sta", "", "", "")

	// Best match should be first with score 100
	if !result.SetGroups[0].IsBestMatch {
		t.Error("expected first set group to be best match")
	}
	if result.SetGroups[0].SetCode != "sta" {
		t.Errorf("expected best match to be 'sta', got %s", result.SetGroups[0].SetCode)
	}
	if result.SetGroups[0].MatchScore != 100 {
		t.Errorf("expected match score 100, got %d", result.SetGroups[0].MatchScore)
	}

	// Others should not be best match
	if result.SetGroups[1].IsBestMatch {
		t.Error("expected second set group to NOT be best match")
	}
	if result.SetGroups[2].IsBestMatch {
		t.Error("expected third set group to NOT be best match")
	}
}

func TestGroupCardsBySet_BestMatchByCollectorNumber(t *testing.T) {
	cards := []models.Card{
		{ID: "1", Name: "Lightning Bolt", SetCode: "neo", SetName: "Kamigawa: Neon Dynasty", ReleasedAt: "2022-02-18", CardNumber: "123"},
		{ID: "2", Name: "Lightning Bolt", SetCode: "2xm", SetName: "Double Masters", ReleasedAt: "2020-08-07", CardNumber: "96"},
		{ID: "3", Name: "Lightning Bolt", SetCode: "sta", SetName: "Strixhaven Mystical Archive", ReleasedAt: "2021-04-23", CardNumber: "42"},
	}

	// OCR detected collector number "96" but no set code (score: 50)
	result := GroupCardsBySet(cards, "", "96", "", "")

	// 2xm should be best match based on collector number
	if result.SetGroups[0].SetCode != "2xm" {
		t.Errorf("expected '2xm' to be first (best match), got %s", result.SetGroups[0].SetCode)
	}
	if !result.SetGroups[0].IsBestMatch {
		t.Error("expected '2xm' to be marked as best match")
	}
	if result.SetGroups[0].MatchScore != 50 {
		t.Errorf("expected match score 50, got %d", result.SetGroups[0].MatchScore)
	}
}

func TestGroupCardsBySet_SetCodeTakesPriorityOverCollectorNumber(t *testing.T) {
	cards := []models.Card{
		{ID: "1", Name: "Lightning Bolt", SetCode: "neo", SetName: "Kamigawa: Neon Dynasty", ReleasedAt: "2022-02-18", CardNumber: "96"},
		{ID: "2", Name: "Lightning Bolt", SetCode: "2xm", SetName: "Double Masters", ReleasedAt: "2020-08-07", CardNumber: "96"},
	}

	// OCR detected set code "neo" AND collector number "96" (which matches both)
	// neo: 100 (set code) + 50 (collector number) = 150
	// 2xm: 50 (collector number) = 50
	result := GroupCardsBySet(cards, "neo", "96", "", "")

	// neo should be best match because set code gives higher score
	if result.SetGroups[0].SetCode != "neo" {
		t.Errorf("expected best match to be 'neo' (set code match), got %s", result.SetGroups[0].SetCode)
	}
	if !result.SetGroups[0].IsBestMatch {
		t.Error("expected 'neo' to be marked as best match")
	}
	if result.SetGroups[0].MatchScore != 150 {
		t.Errorf("expected match score 150, got %d", result.SetGroups[0].MatchScore)
	}

	// 2xm should have score 50 (collector number match only)
	if result.SetGroups[1].MatchScore != 50 {
		t.Errorf("expected 2xm match score 50, got %d", result.SetGroups[1].MatchScore)
	}
}

func TestGroupCardsBySet_SetTotalMatch(t *testing.T) {
	cards := []models.Card{
		{ID: "1", Name: "Lightning Bolt", SetCode: "neo", SetName: "Kamigawa: Neon Dynasty", ReleasedAt: "2022-02-18", CardNumber: "302"},
		{ID: "2", Name: "Lightning Bolt", SetCode: "2xm", SetName: "Double Masters", ReleasedAt: "2020-08-07", CardNumber: "332"},
		{ID: "3", Name: "Lightning Bolt", SetCode: "sta", SetName: "Strixhaven Mystical Archive", ReleasedAt: "2021-04-23", CardNumber: "63"},
	}

	// OCR detected set total "302" (from "123/302")
	// neo has max collector number 302, so it should get +30 for set total match
	result := GroupCardsBySet(cards, "", "", "302", "")

	if result.SetGroups[0].SetCode != "neo" {
		t.Errorf("expected 'neo' to be first (set total match), got %s", result.SetGroups[0].SetCode)
	}
	if result.SetGroups[0].MatchScore != 30 {
		t.Errorf("expected match score 30, got %d", result.SetGroups[0].MatchScore)
	}
}

func TestGroupCardsBySet_CopyrightYearMatch(t *testing.T) {
	cards := []models.Card{
		{ID: "1", Name: "Lightning Bolt", SetCode: "neo", SetName: "Kamigawa: Neon Dynasty", ReleasedAt: "2022-02-18"},
		{ID: "2", Name: "Lightning Bolt", SetCode: "2xm", SetName: "Double Masters", ReleasedAt: "2020-08-07"},
		{ID: "3", Name: "Lightning Bolt", SetCode: "sta", SetName: "Strixhaven Mystical Archive", ReleasedAt: "2021-04-23"},
	}

	// OCR detected copyright year "2021"
	// sta was released in 2021, so it should get +20 for year match
	result := GroupCardsBySet(cards, "", "", "", "2021")

	if result.SetGroups[0].SetCode != "sta" {
		t.Errorf("expected 'sta' to be first (copyright year match), got %s", result.SetGroups[0].SetCode)
	}
	if result.SetGroups[0].MatchScore != 20 {
		t.Errorf("expected match score 20, got %d", result.SetGroups[0].MatchScore)
	}
}

func TestGroupCardsBySet_CombinedScoring(t *testing.T) {
	cards := []models.Card{
		// neo: will match set code (100) + collector number (50) + year (20) = 170
		{ID: "1", Name: "Lightning Bolt", SetCode: "neo", SetName: "Kamigawa: Neon Dynasty", ReleasedAt: "2022-02-18", CardNumber: "123"},
		// 2xm: will match collector number (50) only = 50
		{ID: "2", Name: "Lightning Bolt", SetCode: "2xm", SetName: "Double Masters", ReleasedAt: "2020-08-07", CardNumber: "123"},
		// sta: will match year (20) only = 20
		{ID: "3", Name: "Lightning Bolt", SetCode: "sta", SetName: "Strixhaven Mystical Archive", ReleasedAt: "2022-04-23", CardNumber: "42"},
	}

	// OCR: set code "neo", collector number "123", year "2022"
	result := GroupCardsBySet(cards, "neo", "123", "", "2022")

	// Verify order: neo (170) > 2xm (50) > sta (20)
	if result.SetGroups[0].SetCode != "neo" {
		t.Errorf("expected 'neo' first, got %s", result.SetGroups[0].SetCode)
	}
	if result.SetGroups[0].MatchScore != 170 {
		t.Errorf("expected neo score 170, got %d", result.SetGroups[0].MatchScore)
	}

	if result.SetGroups[1].SetCode != "2xm" {
		t.Errorf("expected '2xm' second, got %s", result.SetGroups[1].SetCode)
	}
	if result.SetGroups[1].MatchScore != 50 {
		t.Errorf("expected 2xm score 50, got %d", result.SetGroups[1].MatchScore)
	}

	if result.SetGroups[2].SetCode != "sta" {
		t.Errorf("expected 'sta' third, got %s", result.SetGroups[2].SetCode)
	}
	if result.SetGroups[2].MatchScore != 20 {
		t.Errorf("expected sta score 20, got %d", result.SetGroups[2].MatchScore)
	}
}

func TestGroupCardsBySet_AllSignalsCombined(t *testing.T) {
	cards := []models.Card{
		// This card matches all signals: set code, collector number, set total, and year
		{ID: "1", Name: "Lightning Bolt", SetCode: "neo", SetName: "Kamigawa: Neon Dynasty", ReleasedAt: "2022-02-18", CardNumber: "123"},
		{ID: "2", Name: "Lightning Bolt", SetCode: "neo", SetName: "Kamigawa: Neon Dynasty", ReleasedAt: "2022-02-18", CardNumber: "302"},
	}

	// OCR: all signals point to neo
	// Score: 100 (set code) + 50 (collector number 123) + 30 (set total 302) + 20 (year 2022) = 200
	result := GroupCardsBySet(cards, "neo", "123", "302", "2022")

	if result.SetGroups[0].MatchScore != 200 {
		t.Errorf("expected score 200, got %d", result.SetGroups[0].MatchScore)
	}
	if !result.SetGroups[0].IsBestMatch {
		t.Error("expected best match to be true")
	}
}

func TestGroupCardsBySet_TiedScoresSortByDate(t *testing.T) {
	cards := []models.Card{
		{ID: "1", Name: "Lightning Bolt", SetCode: "neo", SetName: "Kamigawa: Neon Dynasty", ReleasedAt: "2022-02-18", CardNumber: "123"},
		{ID: "2", Name: "Lightning Bolt", SetCode: "2xm", SetName: "Double Masters", ReleasedAt: "2020-08-07", CardNumber: "123"},
		{ID: "3", Name: "Lightning Bolt", SetCode: "sta", SetName: "Strixhaven Mystical Archive", ReleasedAt: "2021-04-23", CardNumber: "123"},
	}

	// All sets have matching collector number (score 50 each)
	// Should sort by release date when scores are tied
	result := GroupCardsBySet(cards, "", "123", "", "")

	// All have same score, so should be sorted by date (newest first)
	if result.SetGroups[0].SetCode != "neo" {
		t.Errorf("expected 'neo' first (newest), got %s", result.SetGroups[0].SetCode)
	}
	if result.SetGroups[1].SetCode != "sta" {
		t.Errorf("expected 'sta' second, got %s", result.SetGroups[1].SetCode)
	}
	if result.SetGroups[2].SetCode != "2xm" {
		t.Errorf("expected '2xm' third (oldest), got %s", result.SetGroups[2].SetCode)
	}

	// All should have same score
	for _, g := range result.SetGroups {
		if g.MatchScore != 50 {
			t.Errorf("expected all scores to be 50, got %d for %s", g.MatchScore, g.SetCode)
		}
	}
}

func TestGroupCardsBySet_NoBestMatchWhenNoSignals(t *testing.T) {
	cards := []models.Card{
		{ID: "1", Name: "Lightning Bolt", SetCode: "neo", SetName: "Kamigawa: Neon Dynasty", ReleasedAt: "2022-02-18"},
		{ID: "2", Name: "Lightning Bolt", SetCode: "2xm", SetName: "Double Masters", ReleasedAt: "2020-08-07"},
	}

	// No OCR signals provided
	result := GroupCardsBySet(cards, "", "", "", "")

	// No group should be marked as best match when there's no confidence
	for _, g := range result.SetGroups {
		if g.IsBestMatch {
			t.Errorf("expected no best match when no OCR signals, but %s was marked", g.SetCode)
		}
		if g.MatchScore != 0 {
			t.Errorf("expected score 0, got %d for %s", g.MatchScore, g.SetCode)
		}
	}
}

func TestGroupCardsBySet_NonNumericCollectorNumbers(t *testing.T) {
	cards := []models.Card{
		// Some sets have non-numeric collector numbers like "A1", "P1", etc.
		{ID: "1", Name: "Lightning Bolt", SetCode: "neo", SetName: "Kamigawa: Neon Dynasty", ReleasedAt: "2022-02-18", CardNumber: "P1"},
		{ID: "2", Name: "Lightning Bolt", SetCode: "neo", SetName: "Kamigawa: Neon Dynasty", ReleasedAt: "2022-02-18", CardNumber: "302"},
	}

	// Set total should still work with the numeric collector number
	result := GroupCardsBySet(cards, "", "", "302", "")

	if result.SetGroups[0].MatchScore != 30 {
		t.Errorf("expected score 30 (set total match), got %d", result.SetGroups[0].MatchScore)
	}
}
