package services

import (
	"archive/zip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/codyseavey/tcg-tracker/backend/internal/database"
	"github.com/codyseavey/tcg-tracker/backend/internal/models"
)

const pokemonDataURL = "https://github.com/PokemonTCG/pokemon-tcg-data/archive/refs/heads/master.zip"

type PokemonHybridService struct {
	sets      map[string]LocalSet
	cardIndex map[string][]int // name -> card indices for fast lookup
	wordIndex map[string][]int // word -> card indices for full-text search
	idIndex   map[string]int   // card ID -> card index for O(1) lookup
	cards     []LocalPokemonCard
	mu        sync.RWMutex
}

// LocalAttack represents an attack on a Pokemon card
type LocalAttack struct {
	Name                string   `json:"name"`
	Cost                []string `json:"cost"`
	ConvertedEnergyCost int      `json:"convertedEnergyCost"`
	Damage              string   `json:"damage"`
	Text                string   `json:"text"`
}

// LocalAbility represents an ability on a Pokemon card
type LocalAbility struct {
	Name string `json:"name"`
	Text string `json:"text"`
	Type string `json:"type"`
}

// LocalWeakness represents a weakness on a Pokemon card
type LocalWeakness struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type LocalPokemonCard struct {
	Subtypes               []string        `json:"subtypes"`
	Types                  []string        `json:"types"`
	Images                 LocalCardImages `json:"images"`
	Attacks                []LocalAttack   `json:"attacks"`
	Abilities              []LocalAbility  `json:"abilities"`
	Weaknesses             []LocalWeakness `json:"weaknesses"`
	Resistances            []LocalWeakness `json:"resistances"`
	NationalPokedexNumbers []int           `json:"nationalPokedexNumbers"`
	RetreatCost            []string        `json:"retreatCost"`
	EvolvesTo              []string        `json:"evolvesTo"`
	ID                     string          `json:"id"`
	Name                   string          `json:"name"`
	Supertype              string          `json:"supertype"`
	HP                     string          `json:"hp"`
	Number                 string          `json:"number"`
	Artist                 string          `json:"artist"`
	Rarity                 string          `json:"rarity"`
	FlavorText             string          `json:"flavorText"`
	EvolvesFrom            string          `json:"evolvesFrom"`
	RegulationMark         string          `json:"regulationMark"`
	TCGPlayerID            string          `json:"tcgplayerId,omitempty"` // TCGPlayer product ID (for Japanese cards)
	ConvertedRetreatCost   int             `json:"convertedRetreatCost"`
	SetID                  string          // Populated from filename
	IsJapanese             bool            // True if this is a Japanese-exclusive card

	// Pre-computed fields for performance (built at load time)
	searchableText string // Full-text searchable content
	nameLower      string // Lowercase name for fast matching
	setIDLower     string // Lowercase set ID for fast filtering
}

type LocalCardImages struct {
	Small string `json:"small"`
	Large string `json:"large"`
}

// LocalSetImages contains the image URLs for a Pokemon set
type LocalSetImages struct {
	Symbol string `json:"symbol"`
	Logo   string `json:"logo"`
}

type LocalSet struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Series      string         `json:"series"`
	ReleaseDate string         `json:"releaseDate"`
	Total       int            `json:"total"`
	Images      LocalSetImages `json:"images"`
}

// normalizeApostrophes converts curly/smart quotes to straight apostrophes for consistent matching.
// Card data may use ' (U+2019 RIGHT SINGLE QUOTATION MARK) but users type ' (U+0027 APOSTROPHE).
func normalizeApostrophes(s string) string {
	s = strings.ReplaceAll(s, "'", "'") // Right single quote -> straight apostrophe
	s = strings.ReplaceAll(s, "'", "'") // Left single quote -> straight apostrophe
	return s
}

// precomputeFields pre-computes lowercase and searchable text fields at load time
// for efficient matching during OCR identification. This avoids repeated string
// operations during the hot path.
func (c *LocalPokemonCard) precomputeFields() {
	// Pre-compute lowercase versions for fast matching
	// Normalize apostrophes so "Blaine's" matches "Blaine's"
	c.nameLower = strings.ToLower(normalizeApostrophes(c.Name))
	c.setIDLower = strings.ToLower(c.SetID)

	// Build searchable text from all card content
	var parts []string
	parts = append(parts, c.Name)

	for _, attack := range c.Attacks {
		parts = append(parts, attack.Name)
		if attack.Text != "" {
			parts = append(parts, attack.Text)
		}
	}

	for _, ability := range c.Abilities {
		parts = append(parts, ability.Name)
		if ability.Text != "" {
			parts = append(parts, ability.Text)
		}
	}

	if c.FlavorText != "" {
		parts = append(parts, c.FlavorText)
	}

	if c.EvolvesFrom != "" {
		parts = append(parts, c.EvolvesFrom)
	}

	c.searchableText = strings.ToLower(normalizeApostrophes(strings.Join(parts, " ")))
}

// tokenizeText splits text into significant words (4+ characters)
func tokenizeText(text string) []string {
	words := strings.Fields(text)
	result := make([]string, 0, len(words))
	for _, word := range words {
		// Clean punctuation and keep only words 4+ chars
		cleaned := strings.Trim(word, ".,!?\"'();:-")
		if len(cleaned) >= 4 {
			result = append(result, cleaned)
		}
	}
	return result
}

// countWordMatches counts how many OCR words appear in the card's searchable text
func countWordMatches(ocrWords []string, cardText string) int {
	count := 0
	for _, word := range ocrWords {
		if strings.Contains(cardText, word) {
			count++
		}
	}
	return count
}

// matchShortNameAsWord checks if a short name (1-2 chars) appears as a standalone word in text
// This prevents "N" from matching any text containing the letter "n"
// Returns true only if the name appears with word boundaries (spaces, punctuation, start/end)
func matchShortNameAsWord(text, shortName string) bool {
	if len(shortName) == 0 {
		return false
	}

	// Split text into words (preserving all words, not filtering by length)
	words := strings.Fields(text)
	for _, word := range words {
		// Clean punctuation from word for comparison
		cleaned := strings.Trim(word, ".,!?\"'();:-/")
		if cleaned == shortName {
			return true
		}
	}
	return false
}

// normalizeWordForIndex normalizes a word for indexing
// Handles common OCR errors and returns empty string if word should be skipped
func normalizeWordForIndex(word string) string {
	// Skip very short words
	if len(word) < 3 {
		return ""
	}

	// Skip pure numbers
	isAllDigits := true
	for _, c := range word {
		if c < '0' || c > '9' {
			isAllDigits = false
			break
		}
	}
	if isAllDigits {
		return ""
	}

	// Skip common stop words that appear in card text but aren't useful for matching
	stopWords := map[string]bool{
		"the": true, "and": true, "for": true, "you": true, "your": true,
		"this": true, "that": true, "with": true, "from": true, "into": true,
		"each": true, "all": true, "any": true, "can": true, "may": true,
		"one": true, "two": true, "pokemon": true, "card": true, "cards": true,
		"energy": true, "damage": true, "attack": true, "turn": true,
	}
	if stopWords[word] {
		return ""
	}

	return word
}

// extractIndexWords extracts normalized words from text for indexing
func extractIndexWords(text string) []string {
	words := strings.Fields(text)
	result := make([]string, 0, len(words))
	seen := make(map[string]bool)

	for _, word := range words {
		// Clean punctuation
		cleaned := strings.Trim(word, ".,!?\"'();:-")
		normalized := normalizeWordForIndex(cleaned)
		if normalized != "" && !seen[normalized] {
			seen[normalized] = true
			result = append(result, normalized)
		}
	}
	return result
}

func NewPokemonHybridService(dataDir string) (*PokemonHybridService, error) {
	service := &PokemonHybridService{
		cards:     make([]LocalPokemonCard, 0),
		sets:      make(map[string]LocalSet),
		cardIndex: make(map[string][]int),
		wordIndex: make(map[string][]int),
		idIndex:   make(map[string]int),
	}

	if err := service.loadData(dataDir); err != nil {
		return nil, err
	}

	return service, nil
}

func (s *PokemonHybridService) loadData(dataDir string) error {
	// Check if English data exists, download if not
	dataPath := filepath.Join(dataDir, "pokemon-tcg-data-master")
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		fmt.Println("Pokemon TCG data not found. Downloading...")
		if err := downloadPokemonData(dataDir); err != nil {
			return fmt.Errorf("failed to download pokemon data: %w", err)
		}
		fmt.Println("Pokemon TCG data downloaded successfully.")
	}

	// Load English sets
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

	// Load all English card files
	cardsDir := filepath.Join(dataDir, "pokemon-tcg-data-master", "cards", "en")
	if err := s.loadCardsFromDirectory(cardsDir, false); err != nil {
		return fmt.Errorf("failed to load English cards: %w", err)
	}

	englishCount := len(s.cards)

	// Check if Japanese data exists, copy from bundled if not
	japanDataPath := filepath.Join(dataDir, "pokemon-tcg-data-japan")
	if _, err := os.Stat(japanDataPath); os.IsNotExist(err) {
		// Try to copy from bundled data (Docker image includes this)
		bundledDir := os.Getenv("BUNDLED_POKEMON_DATA_DIR")
		if bundledDir != "" {
			bundledJapanPath := filepath.Join(bundledDir, "pokemon-tcg-data-japan")
			if _, err := os.Stat(bundledJapanPath); err == nil {
				log.Printf("Copying bundled Japanese Pokemon data to %s...", japanDataPath)
				if err := copyDir(bundledJapanPath, japanDataPath); err != nil {
					log.Printf("Warning: failed to copy bundled Japanese data: %v", err)
				} else {
					log.Printf("Japanese Pokemon data copied successfully.")
				}
			}
		}
	}

	// Load Japanese card data if available
	if _, err := os.Stat(japanDataPath); err == nil {
		// Load Japanese sets
		japanSetsFile := filepath.Join(japanDataPath, "sets.json")
		if japanSetsData, err := os.ReadFile(japanSetsFile); err == nil {
			var japanSets []LocalSet
			if err := json.Unmarshal(japanSetsData, &japanSets); err == nil {
				for _, set := range japanSets {
					s.sets[set.ID] = set
				}
			}
		}

		// Load Japanese card files
		japanCardsDir := filepath.Join(japanDataPath, "cards")
		if err := s.loadCardsFromDirectory(japanCardsDir, true); err != nil {
			log.Printf("Warning: failed to load Japanese cards: %v", err)
		}
	}

	japaneseCount := len(s.cards) - englishCount
	log.Printf("Pokemon data loaded: %d cards (%d English, %d Japanese), %d sets, %d indexed words",
		len(s.cards), englishCount, japaneseCount, len(s.sets), len(s.wordIndex))

	return nil
}

// loadCardsFromDirectory loads card JSON files from a directory and indexes them
func (s *PokemonHybridService) loadCardsFromDirectory(cardsDir string, isJapanese bool) error {
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
			cards[i].IsJapanese = isJapanese
			// Pre-compute lowercase fields and searchable text
			cards[i].precomputeFields()

			idx := len(s.cards)
			s.cards = append(s.cards, cards[i])

			// Index by ID for O(1) lookups
			s.idIndex[cards[i].ID] = idx

			// Index by lowercase name for search (using pre-computed field)
			s.cardIndex[cards[i].nameLower] = append(s.cardIndex[cards[i].nameLower], idx)

			// Also index by name parts for partial matching
			parts := strings.Fields(cards[i].nameLower)
			for _, part := range parts {
				if len(part) > 2 {
					s.cardIndex[part] = append(s.cardIndex[part], idx)
				}
			}

			// Build inverted index for full-text search
			// Index all significant words from searchable text
			indexWords := extractIndexWords(cards[i].searchableText)
			for _, word := range indexWords {
				s.wordIndex[word] = append(s.wordIndex[word], idx)
			}
		}
	}

	return nil
}

func (s *PokemonHybridService) SearchCards(query string) (*models.CardSearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Normalize apostrophes for consistent matching (handles "Blaine's" vs "Blaine's")
	queryLower := strings.ToLower(strings.TrimSpace(normalizeApostrophes(query)))

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

	// Third pass: Search by set name (if query looks like a set name and not enough results)
	// Only do this if we have fewer than 20 results from card name matching
	if len(scored) < 20 {
		for setID, set := range s.sets {
			setNameLower := strings.ToLower(set.Name)
			setScore := 0

			// Exact set name match
			if setNameLower == queryLower {
				setScore = 350
			} else if strings.Contains(setNameLower, queryLower) {
				// Partial set name match (e.g., "vivid" matches "Vivid Voltage")
				setScore = 300
			} else if strings.Contains(queryLower, setNameLower) {
				// Query contains set name
				setScore = 250
			}

			if setScore > 0 {
				// Add all cards from this set
				for idx, card := range s.cards {
					if card.SetID == setID && !seen[idx] {
						seen[idx] = true
						scored = append(scored, scoredMatch{idx: idx, score: setScore})
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

	// Load cached prices only (fast) - PriceWorker handles updates via JustTCG
	s.loadCachedPrices(cards)

	return &models.CardSearchResult{
		Cards:      cards,
		TotalCount: len(scored),
		HasMore:    len(scored) > maxResults,
	}, nil
}

// SearchCardsGrouped searches for cards by name and groups results by set.
// Returns a GroupedSearchResult with cards organized by set for 2-phase selection.
func (s *PokemonHybridService) SearchCardsGrouped(query string) (*models.GroupedSearchResult, error) {
	// First do a regular search to get matching cards
	result, err := s.SearchCards(query)
	if err != nil {
		return nil, err
	}

	if len(result.Cards) == 0 {
		return &models.GroupedSearchResult{
			CardName:  query,
			SetGroups: []models.SetGroup{},
			TotalSets: 0,
		}, nil
	}

	// Lock for accessing s.sets map
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Group cards by set
	setMap := make(map[string]*models.SetGroup)
	for _, card := range result.Cards {
		group, exists := setMap[card.SetCode]
		if !exists {
			// Get set info, use card's info as fallback if set not found
			var series, releaseDate, symbolURL string
			if set, setExists := s.sets[card.SetCode]; setExists {
				series = set.Series
				releaseDate = set.ReleaseDate
				symbolURL = set.Images.Symbol
			}
			group = &models.SetGroup{
				SetCode:     card.SetCode,
				SetName:     card.SetName,
				Series:      series,
				ReleaseDate: releaseDate,
				SymbolURL:   symbolURL,
				Cards:       []models.Card{},
			}
			setMap[card.SetCode] = group
		}
		group.Cards = append(group.Cards, card)
		group.CardCount = len(group.Cards)
	}

	// Convert map to slice
	groups := make([]models.SetGroup, 0, len(setMap))
	for _, g := range setMap {
		groups = append(groups, *g)
	}

	// Sort by release date (newest first)
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].ReleaseDate > groups[j].ReleaseDate
	})

	// Determine the canonical card name (use the first result's name)
	cardName := result.Cards[0].Name
	// If query was for a base name (e.g., "Charizard"), use that
	queryLower := strings.ToLower(strings.TrimSpace(query))
	for _, card := range result.Cards {
		if strings.ToLower(card.Name) == queryLower {
			cardName = card.Name
			break
		}
	}

	return &models.GroupedSearchResult{
		CardName:  cardName,
		SetGroups: groups,
		TotalSets: len(groups),
	}, nil
}

// loadCachedPrices loads prices from database cache only (fast, no API calls)
// Uses batch query to avoid N+1 database calls.
func (s *PokemonHybridService) loadCachedPrices(cards []models.Card) {
	db := database.GetDB()
	if db == nil {
		// Database not initialized (e.g., in tests), mark all as pending
		for i := range cards {
			cards[i].PriceSource = "pending"
		}
		return
	}
	if len(cards) == 0 {
		return
	}

	cacheThreshold := 24 * time.Hour

	// Batch query: fetch all cached cards in one query instead of N queries
	ids := make([]string, len(cards))
	for i, card := range cards {
		ids[i] = card.ID
	}

	var cachedCards []models.Card
	db.Where("id IN ?", ids).Find(&cachedCards)

	// Build lookup map for O(1) access
	cacheMap := make(map[string]*models.Card, len(cachedCards))
	for i := range cachedCards {
		cacheMap[cachedCards[i].ID] = &cachedCards[i]
	}

	// Apply cached prices
	for i := range cards {
		if cached, ok := cacheMap[cards[i].ID]; ok {
			if cached.PriceUpdatedAt != nil && time.Since(*cached.PriceUpdatedAt) < cacheThreshold {
				cards[i].PriceUSD = cached.PriceUSD
				cards[i].PriceFoilUSD = cached.PriceFoilUSD
				cards[i].PriceUpdatedAt = cached.PriceUpdatedAt
				cards[i].PriceSource = "cached"
			} else {
				cards[i].PriceSource = "stale"
			}
		} else {
			cards[i].PriceSource = "pending"
		}
	}
}

// Note: Price fetching is now handled exclusively by the PriceWorker using JustTCG.
// Cards are returned with cached prices or "pending" status, and the PriceWorker
// updates them in the background.

func (s *PokemonHybridService) GetCard(id string) (*models.Card, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	db := database.GetDB()
	cacheThreshold := 24 * time.Hour

	// Find card by ID in local data
	for _, localCard := range s.cards {
		if localCard.ID == id {
			card := s.convertToCard(localCard)

			// Load cached price from database - PriceWorker handles updates via JustTCG
			var cachedCard models.Card
			if err := db.First(&cachedCard, "id = ?", id).Error; err == nil {
				card.PriceUSD = cachedCard.PriceUSD
				card.PriceFoilUSD = cachedCard.PriceFoilUSD
				card.PriceUpdatedAt = cachedCard.PriceUpdatedAt

				if cachedCard.PriceUpdatedAt != nil && time.Since(*cachedCard.PriceUpdatedAt) < cacheThreshold {
					card.PriceSource = "cached"
				} else if cachedCard.PriceUSD > 0 || cachedCard.PriceFoilUSD > 0 {
					card.PriceSource = "stale"
				} else {
					card.PriceSource = "pending"
				}
			} else {
				card.PriceSource = "pending"
			}

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
		TCGPlayerID:   lc.TCGPlayerID, // Include TCGPlayerID for Japanese cards
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

// SearchJapaneseByName searches for Japanese-exclusive cards by name.
// Returns cards where IsJapanese is true and name matches.
// This is used by Gemini when it identifies a card as Japanese to find
// Japanese-exclusive printings (like Leader's Stadium cards).
func (s *PokemonHybridService) SearchJapaneseByName(name string) []*models.Card {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nameLower := strings.ToLower(strings.TrimSpace(name))
	var results []*models.Card

	for _, localCard := range s.cards {
		// Only include Japanese cards
		if !localCard.IsJapanese {
			continue
		}

		// Check for name match (exact or contains)
		if localCard.nameLower == nameLower ||
			strings.Contains(localCard.nameLower, nameLower) ||
			strings.Contains(nameLower, localCard.nameLower) {
			card := s.convertToCard(localCard)
			results = append(results, &card)
		}
	}

	return results
}

// SearchJapaneseByNameForGemini implements JapaneseCardSearcher interface for Gemini function calling.
// Searches for Japanese-exclusive Pokemon cards by name and returns candidates with images.
func (s *PokemonHybridService) SearchJapaneseByNameForGemini(ctx context.Context, name string, limit int) ([]CandidateCard, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}
	if limit > 20 {
		limit = 20
	}

	nameLower := strings.ToLower(strings.TrimSpace(name))
	if nameLower == "" {
		return nil, nil
	}

	// Score cards based on match quality
	type scoredMatch struct {
		card  LocalPokemonCard
		score int
	}
	var matches []scoredMatch

	for _, localCard := range s.cards {
		// Only include Japanese cards
		if !localCard.IsJapanese {
			continue
		}

		score := 0

		// Exact name match (highest priority)
		if localCard.nameLower == nameLower {
			score = 1000
		} else if strings.HasPrefix(localCard.nameLower, nameLower+" ") {
			// Name with suffix
			score = 800
		} else if strings.HasSuffix(localCard.nameLower, " "+nameLower) {
			// Name with prefix
			score = 700
		} else if strings.Contains(localCard.nameLower, nameLower) {
			// Partial name match
			score = 500
		}

		if score > 0 {
			matches = append(matches, scoredMatch{card: localCard, score: score})
		}
	}

	// Sort by score descending, then by name
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return matches[i].card.Name < matches[j].card.Name
	})

	// Convert to CandidateCard
	var candidates []CandidateCard
	for i := 0; i < len(matches) && len(candidates) < limit; i++ {
		lc := matches[i].card
		imageURL := lc.Images.Large
		if imageURL == "" {
			imageURL = lc.Images.Small
		}
		if imageURL == "" {
			continue // Skip cards without images
		}

		set := s.sets[lc.SetID]
		candidates = append(candidates, CandidateCard{
			ID:       lc.ID,
			Name:     lc.Name,
			SetCode:  lc.SetID,
			SetName:  set.Name,
			Number:   lc.Number,
			ImageURL: imageURL,
			// Enriched data for Gemini filtering
			Rarity:      lc.Rarity,
			Artist:      lc.Artist,
			ReleaseDate: set.ReleaseDate,
			Subtypes:    lc.Subtypes,
			HP:          lc.HP,
			Types:       lc.Types,
		})
	}

	return candidates, nil
}

// GetJapaneseCardByID finds a Japanese card by its ID (prefixed with "jp-")
func (s *PokemonHybridService) GetJapaneseCardByID(id string) *models.Card {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if idx, ok := s.idIndex[id]; ok {
		localCard := s.cards[idx]
		if localCard.IsJapanese {
			card := s.convertToCard(localCard)
			return &card
		}
	}
	return nil
}

// GetCardByID finds a specific card by its ID (simple lookup without price loading).
// Used for quick lookups from cached card IDs.
// Uses O(1) index lookup instead of linear scan.
func (s *PokemonHybridService) GetCardByID(id string) *models.Card {
	s.mu.RLock()
	defer s.mu.RUnlock()

	idx, ok := s.idIndex[id]
	if !ok {
		return nil
	}

	card := s.convertToCard(s.cards[idx])
	return &card
}

// SearchByPokedexNumber finds Pokemon cards by their National Pokedex number
// Returns all cards with that Pokedex number, sorted by release date (newest first)
func (s *PokemonHybridService) SearchByPokedexNumber(pokedexNum int) *models.CardSearchResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if pokedexNum <= 0 {
		return &models.CardSearchResult{Cards: []models.Card{}}
	}

	var matches []models.Card
	for _, localCard := range s.cards {
		// Only Pokemon cards have Pokedex numbers
		if localCard.Supertype != "Pokémon" {
			continue
		}

		for _, num := range localCard.NationalPokedexNumbers {
			if num == pokedexNum {
				matches = append(matches, s.convertToCard(localCard))
				break
			}
		}
	}

	// Sort by set release date (newest first) for better UX
	sort.Slice(matches, func(i, j int) bool {
		setI := s.sets[strings.ToLower(matches[i].SetCode)]
		setJ := s.sets[strings.ToLower(matches[j].SetCode)]
		return setI.ReleaseDate > setJ.ReleaseDate
	})

	return &models.CardSearchResult{
		Cards:      matches,
		TotalCount: len(matches),
		HasMore:    false,
	}
}

// SearchByNameAndNumber searches for cards matching name and number across candidate sets
// Returns cards ranked by match quality (exact matches first, then partial matches)
// If candidateSets is empty, searches all sets
func (s *PokemonHybridService) SearchByNameAndNumber(name, cardNumber string, candidateSets []string) *models.CardSearchResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nameLower := strings.ToLower(strings.TrimSpace(name))
	if nameLower == "" {
		return &models.CardSearchResult{Cards: []models.Card{}}
	}

	// Build set filter map for O(1) lookup
	setFilter := make(map[string]bool)
	for _, setCode := range candidateSets {
		setFilter[strings.ToLower(setCode)] = true
	}
	filterBySet := len(candidateSets) > 0

	// Normalize card number (remove leading zeros)
	normalizedNum := strings.TrimLeft(cardNumber, "0")
	if normalizedNum == "" && cardNumber != "" {
		normalizedNum = "0"
	}

	type scoredCard struct {
		card  models.Card
		score int
	}

	scored := []scoredCard{}

	for _, localCard := range s.cards {
		// Filter by candidate sets if specified (using pre-computed lowercase)
		if filterBySet {
			if !setFilter[localCard.setIDLower] {
				continue
			}
		}

		score := 0

		// Name matching (required for inclusion) - using pre-computed lowercase
		if localCard.nameLower == nameLower {
			score = 1000 // Exact name match
		} else if strings.HasPrefix(localCard.nameLower, nameLower+" ") {
			score = 800 // Name with suffix (e.g., "Charizard" matches "Charizard V")
		} else if strings.Contains(localCard.nameLower, nameLower) {
			score = 500 // Partial name match
		} else {
			continue // Skip cards that don't match the name
		}

		// Card number matching (bonus points)
		if cardNumber != "" {
			localNum := strings.TrimLeft(localCard.Number, "0")
			if localNum == "" {
				localNum = "0"
			}
			if localNum == normalizedNum || localCard.Number == cardNumber {
				score += 200 // Exact number match
			}
		}

		card := s.convertToCard(localCard)
		scored = append(scored, scoredCard{card: card, score: score})
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].card.Name < scored[j].card.Name
	})

	// Convert to result (limit to 50 for performance)
	maxResults := 50
	if len(scored) < maxResults {
		maxResults = len(scored)
	}

	cards := make([]models.Card, maxResults)
	for i := 0; i < maxResults; i++ {
		cards[i] = scored[i].card
	}

	// Load cached prices
	s.loadCachedPrices(cards)

	return &models.CardSearchResult{
		Cards:      cards,
		TotalCount: len(scored),
		HasMore:    len(scored) > maxResults,
	}
}

// FullTextMatchResult contains a matched card with details about what matched
type FullTextMatchResult struct {
	Card          models.Card
	Score         int
	MatchedFields []string // For debugging/confidence: ["name", "attack:Confuse Ray", "ability:Damage Swap"]
}

// scoredCard is used internally for ranking card matches
type scoredCard struct {
	card          models.Card
	score         int
	matchedFields []string
}

// findCandidatesByIndex uses the inverted index to find card indices that match any OCR word
// Returns a deduplicated slice of card indices
func (s *PokemonHybridService) findCandidatesByIndex(ocrWords []string, setFilter map[string]bool, filterBySet bool) []int {
	// Use map to deduplicate candidate indices
	candidateSet := make(map[int]bool)

	for _, word := range ocrWords {
		// Normalize word for index lookup
		normalized := normalizeWordForIndex(word)
		if normalized == "" {
			continue
		}

		// Look up word in index
		if indices, ok := s.wordIndex[normalized]; ok {
			for _, idx := range indices {
				// Apply set filter if specified
				if filterBySet {
					if !setFilter[strings.ToLower(s.cards[idx].SetID)] {
						continue
					}
				}
				candidateSet[idx] = true
			}
		}
	}

	// Convert to slice
	candidates := make([]int, 0, len(candidateSet))
	for idx := range candidateSet {
		candidates = append(candidates, idx)
	}

	return candidates
}

// scoreCards scores a specific set of card indices against OCR text
func (s *PokemonHybridService) scoreCards(indices []int, ocrLower string, ocrWords []string, setFilter map[string]bool, filterBySet bool) []scoredCard {
	scored := make([]scoredCard, 0, len(indices))

	for _, idx := range indices {
		localCard := s.cards[idx]

		// Apply set filter (redundant if already filtered in findCandidatesByIndex, but safe)
		if filterBySet && !setFilter[localCard.setIDLower] {
			continue
		}

		score, matched := s.scoreCard(&localCard, ocrLower, ocrWords)
		if score > 0 {
			card := s.convertToCard(localCard)
			scored = append(scored, scoredCard{card: card, score: score, matchedFields: matched})
		}
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].card.Name < scored[j].card.Name
	})

	return scored
}

// scoreAllCards scores all cards against OCR text (used as fallback)
func (s *PokemonHybridService) scoreAllCards(ocrLower string, ocrWords []string, setFilter map[string]bool, filterBySet bool) []scoredCard {
	scored := make([]scoredCard, 0)

	for i := range s.cards {
		localCard := &s.cards[i]

		// Apply set filter (using pre-computed lowercase)
		if filterBySet && !setFilter[localCard.setIDLower] {
			continue
		}

		score, matched := s.scoreCard(localCard, ocrLower, ocrWords)
		if score > 0 {
			card := s.convertToCard(*localCard)
			scored = append(scored, scoredCard{card: card, score: score, matchedFields: matched})
		}
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].card.Name < scored[j].card.Name
	})

	return scored
}

// scoreCard scores a single card against OCR text
// Returns score and list of matched fields
func (s *PokemonHybridService) scoreCard(localCard *LocalPokemonCard, ocrLower string, ocrWords []string) (int, []string) {
	score := 0
	matched := []string{}

	// Name match (highest priority) - using pre-computed lowercase
	// For very short names (<=2 chars like "N"), require word boundary match
	// to avoid matching any text containing the letter (e.g., "Pikachu" has "n")
	nameMatched := false
	if len(localCard.nameLower) <= 2 {
		// Short name: must match as a standalone word using word boundaries
		// Check against raw OCR words (including short ones, not the filtered ocrWords)
		nameMatched = matchShortNameAsWord(ocrLower, localCard.nameLower)
	} else {
		// Normal length name: substring match is fine
		nameMatched = strings.Contains(ocrLower, localCard.nameLower)
	}

	if nameMatched {
		score += 1000
		matched = append(matched, "name")
	} else {
		// Check if name words appear in OCR (partial name match)
		nameWords := strings.Fields(localCard.nameLower)
		nameWordsFound := 0
		for _, word := range nameWords {
			if len(word) >= 3 && strings.Contains(ocrLower, word) {
				nameWordsFound++
			}
		}
		if nameWordsFound > 0 && nameWordsFound == len(nameWords) {
			score += 500
			matched = append(matched, "name_partial")
		}
	}

	// Attack name matches
	for _, attack := range localCard.Attacks {
		attackNameLower := strings.ToLower(attack.Name)
		if len(attackNameLower) >= 4 && strings.Contains(ocrLower, attackNameLower) {
			score += 200
			matched = append(matched, "attack:"+attack.Name)
		}
	}

	// Ability name matches
	for _, ability := range localCard.Abilities {
		abilityNameLower := strings.ToLower(ability.Name)
		if len(abilityNameLower) >= 4 && strings.Contains(ocrLower, abilityNameLower) {
			score += 200
			matched = append(matched, "ability:"+ability.Name)
		}
	}

	// Card number match
	if localCard.Number != "" {
		// Check for both padded and unpadded number
		normalizedNum := strings.TrimLeft(localCard.Number, "0")
		if normalizedNum == "" {
			normalizedNum = "0"
		}
		// Look for number patterns like "025/185" or just "25"
		if strings.Contains(ocrLower, "/"+localCard.Number) ||
			strings.Contains(ocrLower, localCard.Number+"/") ||
			strings.Contains(ocrLower, " "+normalizedNum+"/") ||
			strings.Contains(ocrLower, "/"+normalizedNum+" ") {
			score += 300
			matched = append(matched, "number:"+localCard.Number)
		}
	}

	// Word overlap scoring (for partial text matches)
	if len(ocrWords) > 0 && localCard.searchableText != "" {
		wordMatches := countWordMatches(ocrWords, localCard.searchableText)
		score += wordMatches * 10
	}

	return score, matched
}

// MatchByFullText scores cards by matching OCR text against all card text
// (name, attacks, abilities, flavor text, evolution info).
// Returns cards sorted by match score (highest first).
// If candidateSets is provided, only searches within those sets.
//
// Uses inverted index for fast candidate lookup, with fallback to full scan
// if index results are poor (ensures no false negatives).
func (s *PokemonHybridService) MatchByFullText(ocrText string, candidateSets []string) (*models.CardSearchResult, []string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Tokenize OCR text into significant words
	ocrLower := strings.ToLower(ocrText)
	ocrWords := tokenizeText(ocrLower)

	// Build set filter for O(1) lookup
	setFilter := make(map[string]bool)
	for _, setCode := range candidateSets {
		setFilter[strings.ToLower(setCode)] = true
	}
	filterBySet := len(candidateSets) > 0

	// Use inverted index to find candidate cards
	// This is much faster than scanning all cards
	candidateIndices := s.findCandidatesByIndex(ocrWords, setFilter, filterBySet)

	// Score candidate cards
	scored := s.scoreCards(candidateIndices, ocrLower, ocrWords, setFilter, filterBySet)

	// RELIABILITY FALLBACK: If best score is low, do full scan to catch
	// cards that might have been missed due to OCR errors
	const minConfidentScore = 500 // Name partial match or better
	if len(scored) == 0 || scored[0].score < minConfidentScore {
		// Full scan as fallback - this is the original behavior
		fullScored := s.scoreAllCards(ocrLower, ocrWords, setFilter, filterBySet)
		if len(fullScored) > 0 && (len(scored) == 0 || fullScored[0].score > scored[0].score) {
			scored = fullScored
		}
	}

	// Sort by score descending, then by name for consistency
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].card.Name < scored[j].card.Name
	})

	// Return top results (limit to 50 for performance)
	maxResults := 50
	if len(scored) < maxResults {
		maxResults = len(scored)
	}

	cards := make([]models.Card, maxResults)
	var topMatchedFields []string
	for i := 0; i < maxResults; i++ {
		cards[i] = scored[i].card
		if i == 0 {
			topMatchedFields = scored[i].matchedFields
		}
	}

	// Load cached prices
	s.loadCachedPrices(cards)

	// Get top score for translation fallback decision
	topScore := 0
	if len(scored) > 0 {
		topScore = scored[0].score
	}

	return &models.CardSearchResult{
		Cards:      cards,
		TotalCount: len(scored),
		HasMore:    len(scored) > maxResults,
		TopScore:   topScore,
	}, topMatchedFields
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

// SearchByName implements CardSearcher interface for Gemini function calling.
// Searches for Pokemon cards by name and returns up to limit results.
func (s *PokemonHybridService) SearchByName(ctx context.Context, name string, limit int) ([]CandidateCard, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}
	if limit > 20 {
		limit = 20
	}

	nameLower := strings.ToLower(strings.TrimSpace(name))
	if nameLower == "" {
		return nil, nil
	}

	// Score cards based on match quality
	type scoredMatch struct {
		card  LocalPokemonCard
		score int
	}
	var matches []scoredMatch

	for _, localCard := range s.cards {
		score := 0

		// Exact name match (highest priority)
		if localCard.nameLower == nameLower {
			score = 1000
		} else if strings.HasPrefix(localCard.nameLower, nameLower+" ") {
			// Name with suffix (e.g., "Charizard" matches "Charizard V")
			score = 800
		} else if strings.HasSuffix(localCard.nameLower, " "+nameLower) {
			// Name with prefix
			score = 700
		} else if strings.Contains(localCard.nameLower, nameLower) {
			// Partial name match
			score = 500
		}

		if score > 0 {
			matches = append(matches, scoredMatch{card: localCard, score: score})
		}
	}

	// Sort by score descending, then by name
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return matches[i].card.Name < matches[j].card.Name
	})

	// Convert to CandidateCard
	var candidates []CandidateCard
	for i := 0; i < len(matches) && len(candidates) < limit; i++ {
		lc := matches[i].card
		imageURL := lc.Images.Large
		if imageURL == "" {
			imageURL = lc.Images.Small
		}
		if imageURL == "" {
			continue // Skip cards without images
		}

		set := s.sets[lc.SetID]
		candidates = append(candidates, CandidateCard{
			ID:       lc.ID,
			Name:     lc.Name,
			SetCode:  lc.SetID,
			SetName:  set.Name,
			Number:   lc.Number,
			ImageURL: imageURL,
			// Enriched data for Gemini filtering
			Rarity:      lc.Rarity,
			Artist:      lc.Artist,
			ReleaseDate: set.ReleaseDate,
			Subtypes:    lc.Subtypes,
			HP:          lc.HP,
			Types:       lc.Types,
		})
	}

	return candidates, nil
}

// GetBySetAndNumber implements CardSearcher interface for Gemini function calling.
// Gets a specific Pokemon card by set code and collector number.
// Tries multiple ID formats to handle variations in how card IDs are stored.
func (s *PokemonHybridService) GetBySetAndNumber(ctx context.Context, setCode, number string) (*CandidateCard, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	setCodeLower := strings.ToLower(setCode)
	numberClean := strings.TrimLeft(number, "0")

	// Try multiple ID formats to handle variations
	// Pokemon TCG data uses various formats: "swsh4-25", "swsh4-025", "base1-4", etc.
	idVariants := []string{
		setCodeLower + "-" + numberClean,        // Most common: "swsh4-25"
		setCodeLower + "-" + number,             // Original input: "swsh4-025"
		setCodeLower + "-0" + numberClean,       // Single zero pad: "swsh4-025"
		setCodeLower + "-00" + numberClean,      // Double zero pad: "swsh4-0025"
		setCodeLower + numberClean,              // No separator: "swsh425"
		setCodeLower + number,                   // No separator with original: "swsh4025"
		strings.ToUpper(setCode) + "-" + number, // Upper case set: "SWSH4-025"
	}

	var idx int
	var ok bool
	for _, id := range idVariants {
		idx, ok = s.idIndex[id]
		if ok {
			break
		}
	}
	if !ok {
		return nil, nil
	}

	lc := s.cards[idx]
	imageURL := lc.Images.Large
	if imageURL == "" {
		imageURL = lc.Images.Small
	}
	if imageURL == "" {
		return nil, nil
	}

	set := s.sets[lc.SetID]
	return &CandidateCard{
		ID:       lc.ID,
		Name:     lc.Name,
		SetCode:  lc.SetID,
		SetName:  set.Name,
		Number:   lc.Number,
		ImageURL: imageURL,
		// Enriched data for Gemini filtering
		Rarity:      lc.Rarity,
		Artist:      lc.Artist,
		ReleaseDate: set.ReleaseDate,
		Subtypes:    lc.Subtypes,
		HP:          lc.HP,
		Types:       lc.Types,
	}, nil
}

// GetCardImage implements CardSearcher interface for Gemini function calling.
// Downloads a card image by ID and returns base64-encoded image data.
func (s *PokemonHybridService) GetCardImage(ctx context.Context, cardID string) (string, error) {
	s.mu.RLock()
	idx, ok := s.idIndex[cardID]
	if !ok {
		s.mu.RUnlock()
		return "", fmt.Errorf("card not found: %s", cardID)
	}
	card := s.cards[idx]
	s.mu.RUnlock()

	imageURL := card.Images.Large
	if imageURL == "" {
		imageURL = card.Images.Small
	}
	if imageURL == "" {
		return "", fmt.Errorf("no image URL for card: %s", cardID)
	}

	// Download the image
	req, err := http.NewRequestWithContext(ctx, "GET", imageURL, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d fetching image", resp.StatusCode)
	}

	// Read and encode
	data, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

// GetCardDetails implements CardSearcher interface for Gemini function calling.
// Returns full card details including attacks, abilities, and other text for verification.
func (s *PokemonHybridService) GetCardDetails(ctx context.Context, cardID string) (*CardDetails, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	idx, ok := s.idIndex[cardID]
	if !ok {
		return nil, fmt.Errorf("card not found: %s", cardID)
	}

	lc := s.cards[idx]
	set := s.sets[lc.SetID]

	imageURL := lc.Images.Large
	if imageURL == "" {
		imageURL = lc.Images.Small
	}

	// Convert attacks with energy cost
	var attacks []AttackInfo
	for _, atk := range lc.Attacks {
		costStr := ""
		if len(atk.Cost) > 0 {
			costStr = strings.Join(atk.Cost, " ")
		}
		attacks = append(attacks, AttackInfo{
			Name:   atk.Name,
			Cost:   costStr,
			Damage: atk.Damage,
			Text:   atk.Text,
		})
	}

	// Convert abilities
	var abilities []AbilityInfo
	for _, ab := range lc.Abilities {
		abilities = append(abilities, AbilityInfo{
			Name: ab.Name,
			Type: ab.Type,
			Text: ab.Text,
		})
	}

	// Convert weaknesses to readable format
	var weaknesses []string
	for _, w := range lc.Weaknesses {
		weaknesses = append(weaknesses, fmt.Sprintf("%s %s", w.Type, w.Value))
	}

	// Convert resistances to readable format
	var resistances []string
	for _, r := range lc.Resistances {
		resistances = append(resistances, fmt.Sprintf("%s %s", r.Type, r.Value))
	}

	return &CardDetails{
		ID:             lc.ID,
		Name:           lc.Name,
		SetCode:        lc.SetID,
		SetName:        set.Name,
		Number:         lc.Number,
		Rarity:         lc.Rarity,
		Artist:         lc.Artist,
		ImageURL:       imageURL,
		HP:             lc.HP,
		Types:          lc.Types,
		Subtypes:       lc.Subtypes,
		Attacks:        attacks,
		Abilities:      abilities,
		Weaknesses:     weaknesses,
		Resistances:    resistances,
		RetreatCost:    lc.ConvertedRetreatCost,
		RegulationMark: lc.RegulationMark,
		EvolvesFrom:    lc.EvolvesFrom,
	}, nil
}

// ListSets implements CardSearcher interface for Gemini function calling.
// Returns Pokemon TCG sets matching the query.
func (s *PokemonHybridService) ListSets(ctx context.Context, query string) ([]SetInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	queryLower := strings.ToLower(strings.TrimSpace(query))
	var results []SetInfo

	for _, set := range s.sets {
		// Match on set name, ID, or series
		nameLower := strings.ToLower(set.Name)
		seriesLower := strings.ToLower(set.Series)
		idLower := strings.ToLower(set.ID)

		if strings.Contains(nameLower, queryLower) ||
			strings.Contains(seriesLower, queryLower) ||
			strings.Contains(idLower, queryLower) {
			results = append(results, SetInfo{
				ID:          set.ID,
				Name:        set.Name,
				Series:      set.Series,
				ReleaseDate: set.ReleaseDate,
				TotalCards:  set.Total,
				SymbolURL:   set.Images.Symbol,
				LogoURL:     set.Images.Logo,
			})
		}
	}

	// Sort by release date (newest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].ReleaseDate > results[j].ReleaseDate
	})

	// Limit to 20 results
	if len(results) > 20 {
		results = results[:20]
	}

	return results, nil
}

// ListAllSets returns all Pokemon sets with optional query filter.
// Unlike ListSets, this method has no result limit and is intended for browsing.
func (s *PokemonHybridService) ListAllSets(query string) []SetInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	queryLower := strings.ToLower(strings.TrimSpace(query))
	var results []SetInfo

	for _, set := range s.sets {
		// If query is empty, include all sets
		if queryLower == "" {
			results = append(results, SetInfo{
				ID:          set.ID,
				Name:        set.Name,
				Series:      set.Series,
				ReleaseDate: set.ReleaseDate,
				TotalCards:  set.Total,
				SymbolURL:   set.Images.Symbol,
				LogoURL:     set.Images.Logo,
			})
			continue
		}

		// Match on set name, ID, or series
		nameLower := strings.ToLower(set.Name)
		seriesLower := strings.ToLower(set.Series)
		idLower := strings.ToLower(set.ID)

		if strings.Contains(nameLower, queryLower) ||
			strings.Contains(seriesLower, queryLower) ||
			strings.Contains(idLower, queryLower) {
			results = append(results, SetInfo{
				ID:          set.ID,
				Name:        set.Name,
				Series:      set.Series,
				ReleaseDate: set.ReleaseDate,
				TotalCards:  set.Total,
				SymbolURL:   set.Images.Symbol,
				LogoURL:     set.Images.Logo,
			})
		}
	}

	// Sort by release date (newest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].ReleaseDate > results[j].ReleaseDate
	})

	return results
}

// GetSetCards returns all cards in a specific set, optionally filtered by name.
// Returns Card models for API responses (not CandidateCards).
func (s *PokemonHybridService) GetSetCards(setCode, nameFilter string) (*models.CardSearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	setCodeLower := strings.ToLower(strings.TrimSpace(setCode))
	nameLower := strings.ToLower(strings.TrimSpace(nameFilter))

	// Check if set exists
	_, exists := s.sets[setCodeLower]
	if !exists {
		return nil, fmt.Errorf("set not found: %s", setCode)
	}

	var cards []models.Card
	for _, lc := range s.cards {
		if lc.setIDLower != setCodeLower {
			continue
		}

		// If name filter provided, check for match
		if nameLower != "" && !strings.Contains(lc.nameLower, nameLower) {
			continue
		}

		cards = append(cards, s.convertToCard(lc))
	}

	// Sort by collector number
	sort.Slice(cards, func(i, j int) bool {
		numI, _ := strconv.Atoi(strings.TrimLeft(cards[i].CardNumber, "0"))
		numJ, _ := strconv.Atoi(strings.TrimLeft(cards[j].CardNumber, "0"))
		return numI < numJ
	})

	// Load cached prices
	s.loadCachedPrices(cards)

	return &models.CardSearchResult{
		Cards:      cards,
		TotalCount: len(cards),
		HasMore:    false,
	}, nil
}

// SearchInSet implements CardSearcher interface for Gemini function calling.
// Searches for cards within a specific set, optionally filtered by name.
func (s *PokemonHybridService) SearchInSet(ctx context.Context, setCode, name string, limit int) ([]CandidateCard, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	setCodeLower := strings.ToLower(strings.TrimSpace(setCode))
	nameLower := strings.ToLower(strings.TrimSpace(name))

	// Check if set exists
	set, exists := s.sets[setCodeLower]
	if !exists {
		return nil, fmt.Errorf("set not found: %s", setCode)
	}

	var candidates []CandidateCard
	for _, lc := range s.cards {
		if lc.setIDLower != setCodeLower {
			continue
		}

		// If name filter provided, check for match
		if nameLower != "" && !strings.Contains(lc.nameLower, nameLower) {
			continue
		}

		imageURL := lc.Images.Large
		if imageURL == "" {
			imageURL = lc.Images.Small
		}
		if imageURL == "" {
			continue
		}

		candidates = append(candidates, CandidateCard{
			ID:             lc.ID,
			Name:           lc.Name,
			SetCode:        lc.SetID,
			SetName:        set.Name,
			Number:         lc.Number,
			ImageURL:       imageURL,
			Rarity:         lc.Rarity,
			Artist:         lc.Artist,
			ReleaseDate:    set.ReleaseDate,
			Subtypes:       lc.Subtypes,
			HP:             lc.HP,
			Types:          lc.Types,
			RegulationMark: lc.RegulationMark,
		})

		if len(candidates) >= limit {
			break
		}
	}

	// Sort by collector number
	sort.Slice(candidates, func(i, j int) bool {
		numI, _ := strconv.Atoi(strings.TrimLeft(candidates[i].Number, "0"))
		numJ, _ := strconv.Atoi(strings.TrimLeft(candidates[j].Number, "0"))
		return numI < numJ
	})

	return candidates, nil
}

// GetSetInfo implements CardSearcher interface for Gemini function calling.
// Returns detailed information about a specific set.
func (s *PokemonHybridService) GetSetInfo(ctx context.Context, setCode string) (*SetDetails, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	setCodeLower := strings.ToLower(strings.TrimSpace(setCode))
	set, exists := s.sets[setCodeLower]
	if !exists {
		return nil, fmt.Errorf("set not found: %s", setCode)
	}

	// Generate a symbol description based on set series and name
	symbolDesc := s.generateSetSymbolDescription(set)

	return &SetDetails{
		ID:                set.ID,
		Name:              set.Name,
		Series:            set.Series,
		ReleaseDate:       set.ReleaseDate,
		TotalCards:        set.Total,
		SymbolDescription: symbolDesc,
	}, nil
}

// generateSetSymbolDescription creates a text description of the set symbol for visual matching
func (s *PokemonHybridService) generateSetSymbolDescription(set LocalSet) string {
	// Map common series to symbol descriptions
	symbolDescriptions := map[string]string{
		"Sword & Shield":         "Shield-shaped emblem with sword",
		"Scarlet & Violet":       "Hexagonal pattern with Pokemon outline",
		"Sun & Moon":             "Sun and moon combined symbol",
		"XY":                     "X and Y intersecting",
		"Black & White":          "Black and white split design",
		"HeartGold & SoulSilver": "Heart and soul combined emblem",
		"Platinum":               "Platinum arc design",
		"Diamond & Pearl":        "Diamond and pearl shapes",
		"EX":                     "EX text in stylized font",
		"Neo":                    "Neo-style geometric pattern",
		"Gym":                    "Gym badge style symbol",
		"Base":                   "Simple Pokemon ball or star",
	}

	// Check for series match
	for seriesKey, desc := range symbolDescriptions {
		if strings.Contains(set.Series, seriesKey) {
			return desc
		}
	}

	// Default description
	return fmt.Sprintf("%s series symbol", set.Series)
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

// copyDir recursively copies a directory from src to dst
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source: %w", err)
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat source file: %w", err)
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	return nil
}
