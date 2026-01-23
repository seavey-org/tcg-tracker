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
	sets      map[string]LocalSet
	cardIndex map[string][]int // name -> card indices for fast lookup
	wordIndex map[string][]int // word -> card indices for full-text search
	cards     []LocalPokemonCard
	mu        sync.RWMutex
}

// LocalAttack represents an attack on a Pokemon card
type LocalAttack struct {
	Name   string `json:"name"`
	Text   string `json:"text"`
	Damage string `json:"damage"`
}

// LocalAbility represents an ability on a Pokemon card
type LocalAbility struct {
	Name string `json:"name"`
	Text string `json:"text"`
	Type string `json:"type"`
}

type LocalPokemonCard struct {
	Subtypes               []string        `json:"subtypes"`
	Types                  []string        `json:"types"`
	Images                 LocalCardImages `json:"images"`
	Attacks                []LocalAttack   `json:"attacks"`
	Abilities              []LocalAbility  `json:"abilities"`
	NationalPokedexNumbers []int           `json:"nationalPokedexNumbers"`
	ID                     string          `json:"id"`
	Name                   string          `json:"name"`
	Supertype              string          `json:"supertype"`
	HP                     string          `json:"hp"`
	Number                 string          `json:"number"`
	Artist                 string          `json:"artist"`
	Rarity                 string          `json:"rarity"`
	FlavorText             string          `json:"flavorText"`
	EvolvesFrom            string          `json:"evolvesFrom"`
	SetID                  string          // Populated from filename

	// Pre-computed searchable text (built at load time)
	searchableText string
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

// buildSearchableText creates a lowercase concatenation of all text on the card
// for efficient full-text matching during OCR identification
func (c *LocalPokemonCard) buildSearchableText() {
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

	c.searchableText = strings.ToLower(strings.Join(parts, " "))
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
			// Build searchable text for full-text matching
			cards[i].buildSearchableText()

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

			// Build inverted index for full-text search
			// Index all significant words from searchable text
			indexWords := extractIndexWords(cards[i].searchableText)
			for _, word := range indexWords {
				s.wordIndex[word] = append(s.wordIndex[word], idx)
			}
		}
	}

	log.Printf("Pokemon data loaded: %d cards, %d sets, %d indexed words",
		len(s.cards), len(s.sets), len(s.wordIndex))

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

	// Load cached prices only (fast) - PriceWorker handles updates via JustTCG
	s.loadCachedPrices(cards)

	return &models.CardSearchResult{
		Cards:      cards,
		TotalCount: len(scored),
		HasMore:    len(scored) > maxResults,
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
		// Filter by candidate sets if specified
		if filterBySet {
			if !setFilter[strings.ToLower(localCard.SetID)] {
				continue
			}
		}

		cardNameLower := strings.ToLower(localCard.Name)
		score := 0

		// Name matching (required for inclusion)
		if cardNameLower == nameLower {
			score = 1000 // Exact name match
		} else if strings.HasPrefix(cardNameLower, nameLower+" ") {
			score = 800 // Name with suffix (e.g., "Charizard" matches "Charizard V")
		} else if strings.Contains(cardNameLower, nameLower) {
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
		if filterBySet && !setFilter[strings.ToLower(localCard.SetID)] {
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

		// Apply set filter
		if filterBySet && !setFilter[strings.ToLower(localCard.SetID)] {
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

	// Name match (highest priority)
	nameLower := strings.ToLower(localCard.Name)
	if strings.Contains(ocrLower, nameLower) {
		score += 1000
		matched = append(matched, "name")
	} else {
		// Check if name words appear in OCR (partial name match)
		nameWords := strings.Fields(nameLower)
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
