package services

import (
	"testing"

	"github.com/codyseavey/tcg-tracker/backend/internal/models"
)

func TestGroupCardsBySet_Empty(t *testing.T) {
	result := GroupCardsBySet([]models.Card{}, "", "")

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

	result := GroupCardsBySet(cards, "", "")

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

	result := GroupCardsBySet(cards, "", "")

	if result.TotalSets != 3 {
		t.Errorf("expected 3 total sets, got %d", result.TotalSets)
	}

	// Verify sorting by release date (newest first)
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

	// OCR detected set code "sta"
	result := GroupCardsBySet(cards, "sta", "")

	// Best match should be first
	if !result.SetGroups[0].IsBestMatch {
		t.Error("expected first set group to be best match")
	}
	if result.SetGroups[0].SetCode != "sta" {
		t.Errorf("expected best match to be 'sta', got %s", result.SetGroups[0].SetCode)
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

	// OCR detected collector number "96" but no set code
	result := GroupCardsBySet(cards, "", "96")

	// 2xm should be best match based on collector number
	var found bool
	for _, g := range result.SetGroups {
		if g.SetCode == "2xm" && g.IsBestMatch {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected '2xm' to be best match based on collector number 96")
	}
}

func TestGroupCardsBySet_SetCodeTakesPriorityOverCollectorNumber(t *testing.T) {
	cards := []models.Card{
		{ID: "1", Name: "Lightning Bolt", SetCode: "neo", SetName: "Kamigawa: Neon Dynasty", ReleasedAt: "2022-02-18", CardNumber: "96"},
		{ID: "2", Name: "Lightning Bolt", SetCode: "2xm", SetName: "Double Masters", ReleasedAt: "2020-08-07", CardNumber: "96"},
	}

	// OCR detected set code "neo" AND collector number "96" (which matches both)
	result := GroupCardsBySet(cards, "neo", "96")

	// neo should be best match because set code takes priority
	if result.SetGroups[0].SetCode != "neo" {
		t.Errorf("expected best match to be 'neo' (set code match), got %s", result.SetGroups[0].SetCode)
	}
	if !result.SetGroups[0].IsBestMatch {
		t.Error("expected 'neo' to be marked as best match")
	}

	// 2xm should NOT be best match even though it has matching collector number
	// because when set code is provided, only set code matching triggers best match
	for _, g := range result.SetGroups {
		if g.SetCode == "2xm" && g.IsBestMatch {
			t.Error("expected '2xm' to NOT be best match when set code 'neo' was provided")
		}
	}
}
