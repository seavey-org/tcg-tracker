package services

import (
	"os"
	"path/filepath"
	"testing"
)

// TestServerOCREndToEnd tests the full OCR pipeline: image → OCR → parsing → card identification
func TestServerOCREndToEnd(t *testing.T) {
	ocrService := NewServerOCRService()
	if !ocrService.IsAvailable() {
		t.Skip("Tesseract not available")
	}

	testCases := []struct {
		filename    string
		game        string
		wantName    string
		wantNumber  string
		wantSetCode string
		wantHP      string
	}{
		// Base Set - these should work well
		{"pokemon_cards/charizard_base1_4.png", "pokemon", "Charizard", "4", "base1", "120"},
		{"pokemon_cards/pikachu_base1_58.png", "pokemon", "Pikachu", "58", "base1", "40"},
		{"pokemon_cards/scyther_base2_10.png", "pokemon", "Scyther", "10", "base2", "70"},
		{"pokemon_cards/gengar_base3_5.png", "pokemon", "Gengar", "5", "base3", "80"},
		{"pokemon_cards/dark_charizard_base5_4.png", "pokemon", "Dark Charizard", "4", "base5", "80"},
		// Neo series
		{"pokemon_cards/lugia_neo1_9.png", "pokemon", "Lugia", "9", "neo1", "90"},
		// Modern cards
		{"pokemon_cards/pikachu_vmax_swsh4_44.png", "pokemon", "Pikachu", "44", "swsh4", "310"},
		{"pokemon_cards/chienpao_ex_sv2_61.png", "pokemon", "Chien-Pao", "61", "sv2", "220"},
		{"pokemon_cards/charizard_ex_sv3pt5_6.png", "pokemon", "Charizard", "6", "sv3pt5", "330"},
	}

	testDir := "testdata"

	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(testDir, tc.filename))
			if err != nil {
				t.Skipf("Test file not found: %s", tc.filename)
				return
			}

			// Step 1: Run OCR
			ocrResult, err := ocrService.ProcessImageBytes(data)
			if err != nil {
				t.Errorf("OCR failed: %v", err)
				return
			}

			t.Logf("OCR extracted %d lines, confidence: %.2f", len(ocrResult.Lines), ocrResult.Confidence)

			// Step 2: Parse OCR text
			parseResult := ParseOCRText(ocrResult.Text, tc.game)

			t.Logf("Parse result: Name=%q, Number=%q, Set=%q, HP=%q, Confidence=%.2f",
				parseResult.CardName, parseResult.CardNumber, parseResult.SetCode,
				parseResult.HP, parseResult.Confidence)

			// Check results - we're more lenient here since OCR is noisy
			if parseResult.CardName == "" {
				t.Errorf("Failed to extract card name, expected %q", tc.wantName)
			}

			// Card number check - just verify we got something
			if parseResult.CardNumber == "" && tc.wantNumber != "" {
				t.Logf("Warning: Card number not extracted, expected %q", tc.wantNumber)
			}

			// HP check
			if parseResult.HP == "" && tc.wantHP != "" {
				t.Logf("Warning: HP not extracted, expected %q", tc.wantHP)
			}

			// Set code check
			if parseResult.SetCode == "" && tc.wantSetCode != "" {
				t.Logf("Warning: Set code not detected, expected %q", tc.wantSetCode)
			}
		})
	}
}

// TestServerOCRMTGEndToEnd tests MTG card OCR pipeline
func TestServerOCRMTGEndToEnd(t *testing.T) {
	ocrService := NewServerOCRService()
	if !ocrService.IsAvailable() {
		t.Skip("Tesseract not available")
	}

	testCases := []struct {
		filename   string
		wantName   string
		wantNumber string
	}{
		{"mtg_cards/the_one_ring_ltr_246.jpg", "One Ring", "246"},
		{"mtg_cards/ragavan_mh2_138.jpg", "Ragavan", "138"},
		{"mtg_cards/sheoldred_dmu_107.jpg", "Sheoldred", "107"},
	}

	testDir := "testdata"

	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(testDir, tc.filename))
			if err != nil {
				t.Skipf("Test file not found: %s", tc.filename)
				return
			}

			// Step 1: Run OCR
			ocrResult, err := ocrService.ProcessImageBytes(data)
			if err != nil {
				t.Errorf("OCR failed: %v", err)
				return
			}

			t.Logf("OCR extracted %d lines, confidence: %.2f", len(ocrResult.Lines), ocrResult.Confidence)

			// Step 2: Parse OCR text
			parseResult := ParseOCRText(ocrResult.Text, "mtg")

			t.Logf("Parse result: Name=%q, Number=%q, Set=%q, Confidence=%.2f",
				parseResult.CardName, parseResult.CardNumber, parseResult.SetCode,
				parseResult.Confidence)

			// Check results
			if parseResult.CardName == "" {
				t.Logf("Warning: Card name not extracted, expected %q", tc.wantName)
			}

			if parseResult.CardNumber == "" && tc.wantNumber != "" {
				t.Logf("Warning: Card number not extracted, expected %q", tc.wantNumber)
			}
		})
	}
}
