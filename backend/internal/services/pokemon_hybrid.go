package services

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/codyseavey/tcg-tracker/backend/internal/database"
	"github.com/codyseavey/tcg-tracker/backend/internal/models"
)

const pokemonDataURL = "https://github.com/PokemonTCG/pokemon-tcg-data/archive/refs/heads/master.zip"

type PokemonHybridService struct {
	cards        []LocalPokemonCard
	sets         map[string]LocalSet
	cardIndex    map[string][]int // name -> card indices for fast lookup
	priceService *PokemonPriceTrackerService
	mu           sync.RWMutex
}

type LocalPokemonCard struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Supertype  string            `json:"supertype"`
	Subtypes   []string          `json:"subtypes"`
	HP         string            `json:"hp"`
	Types      []string          `json:"types"`
	Number     string            `json:"number"`
	Artist     string            `json:"artist"`
	Rarity     string            `json:"rarity"`
	FlavorText string            `json:"flavorText"`
	Images     LocalCardImages   `json:"images"`
	SetID      string            // Populated from filename
}

type LocalCardImages struct {
	Small string `json:"small"`
	Large string `json:"large"`
}

type LocalSet struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Series      string `json:"series"`
	Total       int    `json:"total"`
	ReleaseDate string `json:"releaseDate"`
}

func NewPokemonHybridService(dataDir string, priceTrackerAPIKey string) (*PokemonHybridService, error) {
	service := &PokemonHybridService{
		cards:        make([]LocalPokemonCard, 0),
		sets:         make(map[string]LocalSet),
		cardIndex:    make(map[string][]int),
		priceService: NewPokemonPriceTrackerService(priceTrackerAPIKey),
	}

	if err := service.loadData(dataDir); err != nil {
		return nil, err
	}

	return service, nil
}

func (s *PokemonHybridService) loadData(dataDir string) error {
	// Check if data exists, download if not
	dataPath := filepath.Join(dataDir, "pokemon-tcg-data-master")
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		fmt.Println("Pokemon TCG data not found. Downloading...")
		if err := downloadPokemonData(dataDir); err != nil {
			return fmt.Errorf("failed to download pokemon data: %w", err)
		}
		fmt.Println("Pokemon TCG data downloaded successfully.")
	}

	// Load sets
	setsFile := filepath.Join(dataDir, "pokemon-tcg-data-master", "sets", "en.json")
	setsData, err := os.ReadFile(setsFile)
	if err != nil {
		return fmt.Errorf("failed to read sets file: %w", err)
	}

	var sets []LocalSet
	if err := json.Unmarshal(setsData, &sets); err != nil {
		return fmt.Errorf("failed to parse sets: %w", err)
	}

	for _, set := range sets {
		s.sets[set.ID] = set
	}

	// Load all card files
	cardsDir := filepath.Join(dataDir, "pokemon-tcg-data-master", "cards", "en")
	files, err := os.ReadDir(cardsDir)
	if err != nil {
		return fmt.Errorf("failed to read cards directory: %w", err)
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		setID := strings.TrimSuffix(file.Name(), ".json")
		cardFile := filepath.Join(cardsDir, file.Name())
		cardData, err := os.ReadFile(cardFile)
		if err != nil {
			log.Printf("Warning: failed to read card file %s: %v", cardFile, err)
			continue
		}

		var cards []LocalPokemonCard
		if err := json.Unmarshal(cardData, &cards); err != nil {
			log.Printf("Warning: failed to parse card file %s: %v", cardFile, err)
			continue
		}

		for i := range cards {
			cards[i].SetID = setID
			idx := len(s.cards)
			s.cards = append(s.cards, cards[i])

			// Index by lowercase name for search
			nameLower := strings.ToLower(cards[i].Name)
			s.cardIndex[nameLower] = append(s.cardIndex[nameLower], idx)

			// Also index by name parts for partial matching
			parts := strings.Fields(nameLower)
			for _, part := range parts {
				if len(part) > 2 {
					s.cardIndex[part] = append(s.cardIndex[part], idx)
				}
			}
		}
	}

	return nil
}

func (s *PokemonHybridService) SearchCards(query string) (*models.CardSearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	queryLower := strings.ToLower(strings.TrimSpace(query))

	// Find matching cards
	matchedIndices := make(map[int]bool)

	// Exact name match first
	if indices, ok := s.cardIndex[queryLower]; ok {
		for _, idx := range indices {
			matchedIndices[idx] = true
		}
	}

	// Partial match if not enough results
	if len(matchedIndices) < 20 {
		for name, indices := range s.cardIndex {
			if strings.Contains(name, queryLower) {
				for _, idx := range indices {
					matchedIndices[idx] = true
					if len(matchedIndices) >= 50 {
						break
					}
				}
			}
			if len(matchedIndices) >= 50 {
				break
			}
		}
	}

	// Convert to cards
	cards := make([]models.Card, 0, len(matchedIndices))
	for idx := range matchedIndices {
		if idx < len(s.cards) {
			card := s.convertToCard(s.cards[idx])
			cards = append(cards, card)
		}
		if len(cards) >= 20 {
			break
		}
	}

	// Enrich with prices from PriceTracker (batch or individual)
	s.enrichWithPrices(cards, query)

	return &models.CardSearchResult{
		Cards:      cards,
		TotalCount: len(matchedIndices),
		HasMore:    len(matchedIndices) > 20,
	}, nil
}

func (s *PokemonHybridService) enrichWithPrices(cards []models.Card, query string) {
	db := database.GetDB()
	cacheThreshold := 24 * time.Hour // Prices older than this are considered stale

	// First, check database for cached prices
	needsPriceIDs := []string{}
	for i := range cards {
		var cachedCard models.Card
		if err := db.First(&cachedCard, "id = ?", cards[i].ID).Error; err == nil {
			// Card exists in cache
			if cachedCard.PriceUpdatedAt != nil && time.Since(*cachedCard.PriceUpdatedAt) < cacheThreshold {
				// Use cached price (fresh enough)
				cards[i].PriceUSD = cachedCard.PriceUSD
				cards[i].PriceFoilUSD = cachedCard.PriceFoilUSD
				cards[i].PriceUpdatedAt = cachedCard.PriceUpdatedAt
				cards[i].PriceSource = "cached"
				continue
			}
		}
		// Mark as needing price update
		cards[i].PriceSource = "pending"
		needsPriceIDs = append(needsPriceIDs, cards[i].ID)
	}

	// If all cards have cached prices, we're done (no API call needed!)
	if len(needsPriceIDs) == 0 {
		return
	}

	// Otherwise, fetch prices from API for cards that need it
	// Note: This still uses quota, but only for uncached cards
	priceResult, err := s.priceService.SearchCards(query)
	if err != nil || priceResult == nil {
		return
	}

	// Create a map of prices by name for matching
	priceMap := make(map[string]models.Card)
	for _, priceCard := range priceResult.Cards {
		key := strings.ToLower(priceCard.Name)
		priceMap[key] = priceCard
	}

	// Match prices to cards that need them
	for i := range cards {
		if cards[i].PriceSource == "cached" {
			continue // Already has cached price
		}

		key := strings.ToLower(cards[i].Name)
		if priceCard, ok := priceMap[key]; ok {
			now := time.Now()
			cards[i].PriceUSD = priceCard.PriceUSD
			cards[i].PriceFoilUSD = priceCard.PriceFoilUSD
			cards[i].PriceUpdatedAt = &now
			cards[i].PriceSource = "api"
		}
	}
}

func (s *PokemonHybridService) GetCard(id string) (*models.Card, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	db := database.GetDB()
	cacheThreshold := 24 * time.Hour

	// Find card by ID
	for _, localCard := range s.cards {
		if localCard.ID == id {
			card := s.convertToCard(localCard)

			// First check if we have a cached price
			var cachedCard models.Card
			if err := db.First(&cachedCard, "id = ?", id).Error; err == nil {
				if cachedCard.PriceUpdatedAt != nil && time.Since(*cachedCard.PriceUpdatedAt) < cacheThreshold {
					// Use cached price
					card.PriceUSD = cachedCard.PriceUSD
					card.PriceFoilUSD = cachedCard.PriceFoilUSD
					card.PriceUpdatedAt = cachedCard.PriceUpdatedAt
					card.PriceSource = "cached"
					return &card, nil
				}
			}

			// No cached price or it's stale, mark as pending
			// The background worker will update it
			card.PriceSource = "pending"
			return &card, nil
		}
	}

	return nil, nil
}

func (s *PokemonHybridService) convertToCard(lc LocalPokemonCard) models.Card {
	setName := lc.SetID
	if set, ok := s.sets[lc.SetID]; ok {
		setName = set.Name
	}

	now := time.Now()
	return models.Card{
		ID:             lc.ID,
		Game:           models.GamePokemon,
		Name:           lc.Name,
		SetName:        setName,
		SetCode:        lc.SetID,
		CardNumber:     lc.Number,
		Rarity:         lc.Rarity,
		ImageURL:       lc.Images.Small,
		ImageURLLarge:  lc.Images.Large,
		PriceUpdatedAt: &now,
	}
}

func (s *PokemonHybridService) GetCardCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.cards)
}

func (s *PokemonHybridService) GetSetCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sets)
}

// GetCardBySetAndNumber finds a specific card by set code and card number
func (s *PokemonHybridService) GetCardBySetAndNumber(setCode, cardNumber string) *models.Card {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Normalize card number (remove leading zeros)
	normalizedNum := strings.TrimLeft(cardNumber, "0")
	if normalizedNum == "" {
		normalizedNum = "0"
	}

	for _, localCard := range s.cards {
		// Check if set code matches
		if strings.ToLower(localCard.SetID) != strings.ToLower(setCode) {
			continue
		}

		// Check card number (handle leading zeros)
		localNum := strings.TrimLeft(localCard.Number, "0")
		if localNum == "" {
			localNum = "0"
		}

		if localNum == normalizedNum || localCard.Number == cardNumber {
			card := s.convertToCard(localCard)
			return &card
		}
	}

	return nil
}

func downloadPokemonData(dataDir string) error {
	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Download the zip file
	zipPath := filepath.Join(dataDir, "pokemon-tcg-data.zip")

	// Use a client with timeout for large downloads
	client := &http.Client{
		Timeout: 5 * time.Minute,
	}
	resp, err := client.Get(pokemonDataURL)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	// Create zip file with deferred close
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}
	defer zipFile.Close()

	_, err = io.Copy(zipFile, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write zip file: %w", err)
	}

	// Close file before extracting
	zipFile.Close()

	// Extract zip file
	if err := extractZip(zipPath, dataDir); err != nil {
		return fmt.Errorf("failed to extract zip: %w", err)
	}

	// Clean up zip file after successful extraction
	if err := os.Remove(zipPath); err != nil {
		log.Printf("Warning: failed to clean up zip file: %v", err)
	}

	// Rename extracted folder (github adds -master suffix)
	extractedPath := filepath.Join(dataDir, "pokemon-tcg-data-master")
	if _, err := os.Stat(extractedPath); os.IsNotExist(err) {
		// Try alternate name
		altPath := filepath.Join(dataDir, "pokemon-tcg-data")
		if _, err := os.Stat(altPath); err == nil {
			if renameErr := os.Rename(altPath, extractedPath); renameErr != nil {
				return fmt.Errorf("failed to rename extracted directory: %w", renameErr)
			}
		}
	}

	return nil
}

func extractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(destDir, f.Name)

		// Check for ZipSlip vulnerability
		if !strings.HasPrefix(fpath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		// Create file
		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}

	return nil
}
