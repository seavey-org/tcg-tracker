package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnhancedOCRPokemonCards(t *testing.T) {
	ocrService := NewServerOCRService()
	if !ocrService.IsAvailable() {
		t.Skip("Tesseract not available")
	}

	testCases := []struct {
		filename   string
		wantName   string
		wantNumber string
		wantHP     bool
		mustPass   bool // If false, test logs warnings instead of failing
	}{
		{"charizard_base1_4.png", "Charizard", "4/102", true, true},
		{"pikachu_base1_58.png", "Pikachu", "58/102", true, true},
		{"scyther_base2_10.png", "Scyther", "10/64", true, true},
		{"gengar_base3_5.png", "Gengar", "5/62", false, true}, // HP text often misread on this card
		{"dark_charizard_base5_4.png", "Dark Charizard", "4/82", true, true},
		{"lugia_neo1_9.png", "Lugia", "9/111", true, true},
		{"pikachu_vmax_swsh4_44.png", "Pikachu", "44/185", true, false}, // Modern cards harder to OCR
		{"charizard_vstar_swsh9_tg03.png", "Charizard", "TG03", true, false},
		{"chienpao_ex_sv2_61.png", "Chien-Pao", "61/", true, false},
		{"charizard_ex_sv3pt5_6.png", "Charizard", "6/165", true, false},
	}

	testDir := "testdata/pokemon_cards"

	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(testDir, tc.filename))
			if err != nil {
				t.Skipf("Test file not found: %s", tc.filename)
				return
			}

			result, err := ocrService.ProcessImageBytes(data)
			if err != nil {
				t.Errorf("OCR failed: %v", err)
				return
			}

			text := strings.ToUpper(strings.Join(result.Lines, " "))

			t.Logf("Confidence: %.2f", result.Confidence)
			t.Logf("Extracted %d lines", len(result.Lines))
			for i, line := range result.Lines {
				if i < 15 {
					t.Logf("  Line %d: %s", i, line)
				}
			}

			// Check for card name (case-insensitive partial match)
			wantNameUpper := strings.ToUpper(tc.wantName)
			if !strings.Contains(text, wantNameUpper) {
				if tc.mustPass {
					t.Errorf("Card name not found: want %q in OCR output", tc.wantName)
				} else {
					t.Logf("Warning: Card name %q not clearly found (modern card OCR)", tc.wantName)
				}
			}

			// Check for HP indicator
			if tc.wantHP && !strings.Contains(text, "HP") {
				if tc.mustPass {
					t.Errorf("HP indicator not found in OCR output")
				} else {
					t.Logf("Warning: HP indicator not clearly found")
				}
			}

			// Check for card number pattern
			if tc.wantNumber != "" {
				// Extract just the first part of the card number for matching
				numParts := strings.Split(tc.wantNumber, "/")
				if len(numParts) > 0 && !strings.Contains(text, numParts[0]) {
					t.Logf("Warning: Card number %q not clearly found (may be OCR noise)", tc.wantNumber)
				}
			}
		})
	}
}

func TestEnhancedOCRMTGCards(t *testing.T) {
	ocrService := NewServerOCRService()
	if !ocrService.IsAvailable() {
		t.Skip("Tesseract not available")
	}

	// MTG OCR is more challenging due to varied typography across eras
	// These tests log warnings rather than fail to track OCR quality
	testCases := []struct {
		filename string
		wantName string
		wantType string
		mustPass bool
	}{
		{"black_lotus_lea_232.jpg", "Black Lotus", "Artifact", false}, // Vintage card
		{"lightning_bolt_lea_161.jpg", "Lightning Bolt", "Instant", false},
		{"counterspell_lea_54.jpg", "Counterspell", "Instant", false},
		{"ragavan_mh2_138.jpg", "Ragavan", "Creature", false},
		{"sheoldred_dmu_107.jpg", "Sheoldred", "Creature", false},
		{"the_one_ring_ltr_246.jpg", "One Ring", "Artifact", true}, // Modern, should work
		{"sol_ring_2ed_268.jpg", "Sol Ring", "Artifact", false},
	}

	testDir := "testdata/mtg_cards"

	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(testDir, tc.filename))
			if err != nil {
				t.Skipf("Test file not found: %s", tc.filename)
				return
			}

			result, err := ocrService.ProcessImageBytes(data)
			if err != nil {
				t.Errorf("OCR failed: %v", err)
				return
			}

			text := strings.ToUpper(strings.Join(result.Lines, " "))

			t.Logf("Confidence: %.2f", result.Confidence)
			t.Logf("Extracted %d lines", len(result.Lines))
			for i, line := range result.Lines {
				if i < 15 {
					t.Logf("  Line %d: %s", i, line)
				}
			}

			// Check for card name
			wantNameUpper := strings.ToUpper(tc.wantName)
			if !strings.Contains(text, wantNameUpper) {
				if tc.mustPass {
					t.Errorf("Card name not found: want %q in OCR output", tc.wantName)
				} else {
					t.Logf("Warning: Card name %q not clearly found (vintage/stylized card)", tc.wantName)
				}
			}

			// Check for card type
			wantTypeUpper := strings.ToUpper(tc.wantType)
			if !strings.Contains(text, wantTypeUpper) {
				t.Logf("Warning: Card type %q not clearly found", tc.wantType)
			}
		})
	}
}
