package services

import (
	"context"
	"encoding/json"
	"fmt"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCard represents a card to test with expected OCR results
type TestCard struct {
	Name        string
	SetCode     string
	CardNumber  string
	Game        string
	ImageURL    string
	ExpectedHP  string // Pokemon only
	ExpectedSet string // Expected set name/code to find
}

// pokemonTestCards are real Pokemon cards to test OCR with
var pokemonTestCards = []TestCard{
	{
		Name:        "Pikachu",
		SetCode:     "swsh4",
		CardNumber:  "43",
		Game:        "pokemon",
		ImageURL:    "https://images.pokemontcg.io/swsh4/43_hires.png",
		ExpectedHP:  "60",
		ExpectedSet: "swsh4",
	},
	{
		Name:        "Charizard",
		SetCode:     "swsh3",
		CardNumber:  "20",
		Game:        "pokemon",
		ImageURL:    "https://images.pokemontcg.io/swsh3/20_hires.png",
		ExpectedHP:  "170",
		ExpectedSet: "swsh3",
	},
	{
		Name:        "Mewtwo",
		SetCode:     "sv3pt5",
		CardNumber:  "150",
		Game:        "pokemon",
		ImageURL:    "https://images.pokemontcg.io/sv3pt5/150_hires.png",
		ExpectedHP:  "120",
		ExpectedSet: "sv3pt5",
	},
}

// mtgTestCards are real MTG cards to test OCR with
var mtgTestCards = []TestCard{
	{
		Name:       "Lightning Bolt",
		SetCode:    "2xm",
		CardNumber: "141",
		Game:       "mtg",
		ImageURL:   "", // Will be fetched from Scryfall
	},
	{
		Name:       "Sol Ring",
		SetCode:    "c21",
		CardNumber: "263",
		Game:       "mtg",
		ImageURL:   "", // Will be fetched from Scryfall
	},
}

// TestOCRWithRealPokemonCards tests OCR with real Pokemon card images
func TestOCRWithRealPokemonCards(t *testing.T) {
	service := NewServerOCRService()
	if !service.IsAvailable() {
		t.Skip("Tesseract not available")
	}

	testdataDir := "testdata/images"
	if err := os.MkdirAll(testdataDir, 0755); err != nil {
		t.Fatalf("Failed to create testdata dir: %v", err)
	}

	for _, card := range pokemonTestCards {
		t.Run(card.Name, func(t *testing.T) {
			// Download image if needed
			imagePath := filepath.Join(testdataDir, fmt.Sprintf("pokemon_%s_%s.png", card.SetCode, card.CardNumber))
			if _, err := os.Stat(imagePath); os.IsNotExist(err) {
				if err := downloadImage(card.ImageURL, imagePath); err != nil {
					t.Skipf("Failed to download test image: %v", err)
				}
			}

			// Read image
			imageData, err := os.ReadFile(imagePath)
			if err != nil {
				t.Fatalf("Failed to read image: %v", err)
			}

			// Run OCR
			result, err := service.ProcessImageBytes(imageData)
			if err != nil {
				t.Fatalf("OCR failed: %v", err)
			}

			t.Logf("OCR output for %s:\n%s", card.Name, result.Text)
			t.Logf("Lines: %v", result.Lines)
			t.Logf("Confidence: %.2f", result.Confidence)

			// Parse OCR text
			parsed := ParseOCRText(result.Text, "pokemon")
			t.Logf("Parsed card name: %s", parsed.CardName)
			t.Logf("Parsed card number: %s", parsed.CardNumber)
			t.Logf("Parsed HP: %s", parsed.HP)
			t.Logf("Parsed set code: %s", parsed.SetCode)

			// Verify we can at least extract something useful
			if parsed.CardName == "" && parsed.CardNumber == "" && parsed.HP == "" {
				t.Errorf("OCR extracted no useful information from %s", card.Name)
			}

			// Check if we got the expected card number (allowing for leading zero differences)
			if parsed.CardNumber != "" {
				parsedNum := strings.TrimLeft(parsed.CardNumber, "0")
				expectedNum := strings.TrimLeft(card.CardNumber, "0")
				if parsedNum == expectedNum {
					t.Logf("✓ Card number matched: %s", parsed.CardNumber)
				}
			}

			// Check if we found HP
			if card.ExpectedHP != "" && parsed.HP != "" {
				if parsed.HP == card.ExpectedHP {
					t.Logf("✓ HP matched: %s", parsed.HP)
				}
			}
		})
	}
}

// TestOCRWithRealMTGCards tests OCR with real MTG card images
func TestOCRWithRealMTGCards(t *testing.T) {
	service := NewServerOCRService()
	if !service.IsAvailable() {
		t.Skip("Tesseract not available")
	}

	testdataDir := "testdata/images"
	if err := os.MkdirAll(testdataDir, 0755); err != nil {
		t.Fatalf("Failed to create testdata dir: %v", err)
	}

	for _, card := range mtgTestCards {
		t.Run(card.Name, func(t *testing.T) {
			// Get image URL from Scryfall if not set
			imageURL := card.ImageURL
			if imageURL == "" {
				var err error
				imageURL, err = getScryfallImageURL(card.SetCode, card.CardNumber)
				if err != nil {
					t.Skipf("Failed to get Scryfall image URL: %v", err)
				}
			}

			// Download image if needed
			imagePath := filepath.Join(testdataDir, fmt.Sprintf("mtg_%s_%s.png", card.SetCode, card.CardNumber))
			if _, err := os.Stat(imagePath); os.IsNotExist(err) {
				if err := downloadImage(imageURL, imagePath); err != nil {
					t.Skipf("Failed to download test image: %v", err)
				}
			}

			// Read image
			imageData, err := os.ReadFile(imagePath)
			if err != nil {
				t.Fatalf("Failed to read image: %v", err)
			}

			// Run OCR
			result, err := service.ProcessImageBytes(imageData)
			if err != nil {
				t.Fatalf("OCR failed: %v", err)
			}

			t.Logf("OCR output for %s:\n%s", card.Name, result.Text)
			t.Logf("Lines: %v", result.Lines)
			t.Logf("Confidence: %.2f", result.Confidence)

			// Parse OCR text
			parsed := ParseOCRText(result.Text, "mtg")
			t.Logf("Parsed card name: %s", parsed.CardName)
			t.Logf("Parsed card number: %s", parsed.CardNumber)
			t.Logf("Parsed set code: %s", parsed.SetCode)

			// Verify we can at least extract something useful
			if parsed.CardName == "" && parsed.CardNumber == "" {
				t.Errorf("OCR extracted no useful information from %s", card.Name)
			}
		})
	}
}

// Helper functions

func downloadImage(url, destPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	file, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	return err
}

func getScryfallImageURL(setCode, number string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	url := fmt.Sprintf("https://api.scryfall.com/cards/%s/%s", setCode, number)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Scryfall API error: %d", resp.StatusCode)
	}

	var data struct {
		ImageURIs struct {
			Large string `json:"large"`
			PNG   string `json:"png"`
		} `json:"image_uris"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	if data.ImageURIs.PNG != "" {
		return data.ImageURIs.PNG, nil
	}
	return data.ImageURIs.Large, nil
}
