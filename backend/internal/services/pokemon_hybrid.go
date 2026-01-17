package services

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/codyseavey/tcg-tracker/backend/internal/database"
	"github.com/codyseavey/tcg-tracker/backend/internal/models"
)

const pokemonDataURL = "https://github.com/PokemonTCG/pokemon-tcg-data/archive/refs/heads/master.zip"

type PokemonHybridService struct {
	tcgdexService *TCGdexService
	sets          map[string]LocalSet
	cardIndex     map[string][]int // name -> card indices for fast lookup
	cards         []LocalPokemonCard
	mu            sync.RWMutex
}

type LocalPokemonCard struct {
	Subtypes   []string        `json:"subtypes"`
	Types      []string        `json:"types"`
	Images     LocalCardImages `json:"images"`
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Supertype  string          `json:"supertype"`
	HP         string          `json:"hp"`
	Number     string          `json:"number"`
	Artist     string          `json:"artist"`
	Rarity     string          `json:"rarity"`
	FlavorText string          `json:"flavorText"`
	SetID      string          // Populated from filename
}

type LocalCardImages struct {
	Small string `json:"small"`
	Large string `json:"large"`
}

type LocalSet struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Series      string `json:"series"`
	ReleaseDate string `json:"releaseDate"`
	Total       int    `json:"total"`
}

func NewPokemonHybridService(dataDir string) (*PokemonHybridService, error) {
	service := &PokemonHybridService{
		cards:         make([]LocalPokemonCard, 0),
		sets:          make(map[string]LocalSet),
		cardIndex:     make(map[string][]int),
		tcgdexService: NewTCGdexService(),
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

	// Score cards based on match quality
	type scoredMatch struct {
		idx   int
		score int // Higher = better match
	}
	scored := make([]scoredMatch, 0)
	seen := make(map[int]bool)

	// First pass: Find all cards that contain the query in their name
	for idx, card := range s.cards {
		nameLower := strings.ToLower(card.Name)

		score := 0

		// Exact full name match (highest priority)
		if nameLower == queryLower {
			score = 1000
		} else if strings.EqualFold(card.Name, query) {
			score = 1000
		} else if nameLower == queryLower+" v" || nameLower == queryLower+" vmax" || nameLower == queryLower+" vstar" || nameLower == queryLower+" ex" || nameLower == queryLower+" gx" {
			// Pokemon variant match (e.g., searching "Charizard" matches "Charizard V")
			score = 900
		} else if strings.HasPrefix(nameLower, queryLower+" ") {
			// Name starts with query followed by space (e.g., "Houndour" matches "Houndour ex")
			score = 800
		} else if strings.HasPrefix(nameLower, queryLower) {
			// Name starts with query (e.g., "Hound" matches "Houndour")
			score = 700
		} else if strings.Contains(nameLower, " "+queryLower) || strings.HasSuffix(nameLower, "'s "+queryLower) {
			// Query appears as a word in the name (e.g., "Team Magma's Houndour" contains "Houndour")
			score = 600
		} else if strings.Contains(nameLower, queryLower) {
			// Query appears anywhere in name
			score = 500
		}

		if score > 0 && !seen[idx] {
			seen[idx] = true
			scored = append(scored, scoredMatch{idx: idx, score: score})
		}
	}

	// Second pass: Check index for partial word matches (if not enough results)
	if len(scored) < 50 {
		for name, indices := range s.cardIndex {
			if strings.Contains(name, queryLower) || strings.Contains(queryLower, name) {
				for _, idx := range indices {
					if !seen[idx] {
						seen[idx] = true
						scored = append(scored, scoredMatch{idx: idx, score: 400})
					}
				}
			}
		}
	}

	// Sort by score (descending), then by name for consistency
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return s.cards[scored[i].idx].Name < s.cards[scored[j].idx].Name
	})

	// Convert to cards (limit to 50 for performance)
	maxResults := 50
	if len(scored) < maxResults {
		maxResults = len(scored)
	}

	cards := make([]models.Card, 0, maxResults)
	for i := 0; i < maxResults; i++ {
		card := s.convertToCard(s.cards[scored[i].idx])
		cards = append(cards, card)
	}

	// Load cached prices only (fast) - don't block on API calls
	s.loadCachedPrices(cards)

	// Enrich remaining cards with prices in background (non-blocking)
	go s.enrichWithPricesAsync(cards)

	return &models.CardSearchResult{
		Cards:      cards,
		TotalCount: len(scored),
		HasMore:    len(scored) > maxResults,
	}, nil
}

// loadCachedPrices loads prices from database cache only (fast, no API calls)
func (s *PokemonHybridService) loadCachedPrices(cards []models.Card) {
	db := database.GetDB()
	cacheThreshold := 24 * time.Hour

	for i := range cards {
		var cachedCard models.Card
		if err := db.First(&cachedCard, "id = ?", cards[i].ID).Error; err == nil {
			if cachedCard.PriceUpdatedAt != nil && time.Since(*cachedCard.PriceUpdatedAt) < cacheThreshold {
				cards[i].PriceUSD = cachedCard.PriceUSD
				cards[i].PriceFoilUSD = cachedCard.PriceFoilUSD
				cards[i].PriceUpdatedAt = cachedCard.PriceUpdatedAt
				cards[i].PriceSource = "cached"
			} else {
				cards[i].PriceSource = "stale"
			}
		} else {
			cards[i].PriceSource = "pending"
		}
	}
}

// enrichWithPricesAsync fetches prices from API in background and updates database.
// This function only updates the database - it does not modify the input slice.
// Card IDs are copied to avoid any race conditions with the caller.
func (s *PokemonHybridService) enrichWithPricesAsync(cards []models.Card) {
	// Copy card IDs and price sources to avoid race conditions
	type cardInfo struct {
		id          string
		priceSource string
	}
	cardInfos := make([]cardInfo, len(cards))
	for i, c := range cards {
		cardInfos[i] = cardInfo{id: c.ID, priceSource: c.PriceSource}
	}

	db := database.GetDB()

	for _, info := range cardInfos {
		if info.priceSource == "cached" {
			continue // Already have fresh price
		}

		// Fetch price from TCGdex
		priceCard, err := s.tcgdexService.GetCard(info.id)
		if err != nil {
			log.Printf("Failed to fetch price for %s: %v", info.id, err)
			continue
		}

		if priceCard != nil && (priceCard.PriceUSD > 0 || priceCard.PriceFoilUSD > 0) {
			now := time.Now()
			// Update the database directly instead of modifying the input slice
			if err := db.Model(&models.Card{}).Where("id = ?", info.id).Updates(map[string]interface{}{
				"price_usd":        priceCard.PriceUSD,
				"price_foil_usd":   priceCard.PriceFoilUSD,
				"price_updated_at": &now,
				"last_price_check": &now,
				"price_source":     "tcgdex",
			}).Error; err != nil {
				log.Printf("Failed to save price for %s: %v", info.id, err)
			}
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

			// Fetch price from TCGdex (no rate limits)
			priceCard, err := s.tcgdexService.GetCard(id)
			if err != nil {
				log.Printf("Failed to fetch price for %s: %v", id, err)
				card.PriceSource = "error"
				db.Save(&card)
				return &card, nil
			}

			if priceCard != nil && (priceCard.PriceUSD > 0 || priceCard.PriceFoilUSD > 0) {
				now := time.Now()
				card.PriceUSD = priceCard.PriceUSD
				card.PriceFoilUSD = priceCard.PriceFoilUSD
				card.PriceUpdatedAt = &now
				card.LastPriceCheck = &now
				card.PriceSource = "tcgdex"
			} else {
				card.PriceSource = "not_found"
			}

			db.Save(&card)
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

	return models.Card{
		ID:            lc.ID,
		Game:          models.GamePokemon,
		Name:          lc.Name,
		SetName:       setName,
		SetCode:       lc.SetID,
		CardNumber:    lc.Number,
		Rarity:        lc.Rarity,
		ImageURL:      lc.Images.Small,
		ImageURLLarge: lc.Images.Large,
		PriceSource:   "pending",
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
		if !strings.EqualFold(localCard.SetID, setCode) {
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

// GetAllPokemonNames returns all unique Pokemon names from the loaded card data
// Names are returned in lowercase and sorted by length (longest first) for OCR matching
// Includes both Pokemon creature names and all card names for comprehensive matching
func (s *PokemonHybridService) GetAllPokemonNames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Use a map to deduplicate names
	nameSet := make(map[string]bool)

	for _, card := range s.cards {
		name := strings.ToLower(card.Name)

		// For Pokemon cards, extract base name without suffixes
		if card.Supertype == "Pokémon" || card.Supertype == "Pokemon" {
			baseName := extractBasePokemonName(name)
			if baseName != "" && len(baseName) >= 3 {
				nameSet[baseName] = true
			}
		}

		// Add the full card name for all card types (Pokemon, Trainer, Energy)
		// This enables matching Trainer cards like "Professor Oak", "Bill", etc.
		if len(name) >= 3 {
			nameSet[name] = true
		}
	}

	// Convert to slice
	names := make([]string, 0, len(nameSet))
	for name := range nameSet {
		names = append(names, name)
	}

	// Sort by length descending (longest first) to prevent partial matches
	sort.Slice(names, func(i, j int) bool {
		return len(names[i]) > len(names[j])
	})

	return names
}

// extractBasePokemonName removes common Pokemon card suffixes to get the base name
func extractBasePokemonName(name string) string {
	// Common suffixes to remove (order matters - check longer ones first)
	suffixes := []string{
		" vmax", " vstar", " v-union", " v",
		" gx", " ex", " mega", " prime",
		" lv.x", " lvx", " legend", " star",
		" δ", " delta", " radiant",
	}

	result := name
	for _, suffix := range suffixes {
		if strings.HasSuffix(result, suffix) {
			result = strings.TrimSuffix(result, suffix)
			break
		}
	}

	return strings.TrimSpace(result)
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", pokemonDataURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := client.Do(req)
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
			if err := os.MkdirAll(fpath, os.ModePerm); err != nil {
				return err
			}
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
