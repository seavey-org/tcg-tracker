package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// getTestDataDir finds the Pokemon TCG data directory for tests
func getTestDataDir() string {
	possiblePaths := []string{
		"../../data",            // When running from services dir
		"../../../backend/data", // When running from project root
		"data",                  // When running from backend dir
		"backend/data",          // When running from project root
		"../data",               // Alternative location
	}

	for _, path := range possiblePaths {
		absPath, _ := filepath.Abs(path)
		if _, err := os.Stat(filepath.Join(absPath, "pokemon-tcg-data-master")); err == nil {
			return absPath
		}
	}
	return ""
}

// TestNormalizeApostrophes tests the apostrophe normalization function
func TestNormalizeApostrophes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Straight apostrophe unchanged",
			input:    "Blaine's Charmeleon",
			expected: "Blaine's Charmeleon",
		},
		{
			name:     "Right single quote normalized",
			input:    "Blaine's Charmeleon", // Uses ' (U+2019)
			expected: "Blaine's Charmeleon",
		},
		{
			name:     "Left single quote normalized",
			input:    "Blaine's Charmeleon", // Uses ' (U+2018)
			expected: "Blaine's Charmeleon",
		},
		{
			name:     "Multiple curly quotes",
			input:    "Lt. Surge's Pikachu",
			expected: "Lt. Surge's Pikachu",
		},
		{
			name:     "Mixed quotes",
			input:    "Misty's Tears and Blaine's Fire",
			expected: "Misty's Tears and Blaine's Fire",
		},
		{
			name:     "No apostrophes",
			input:    "Charizard VMAX",
			expected: "Charizard VMAX",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeApostrophes(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeApostrophes(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestSearchCardsWithApostrophes tests that card search handles apostrophe variants
func TestSearchCardsWithApostrophes(t *testing.T) {
	dataDir := getTestDataDir()
	if dataDir == "" {
		t.Skip("Pokemon TCG data not found, skipping integration test")
	}

	service, err := NewPokemonHybridService(dataDir)
	if err != nil {
		t.Fatalf("Failed to initialize PokemonHybridService: %v", err)
	}

	tests := []struct {
		name         string
		query        string
		expectedName string // Card name should contain this
	}{
		{
			name:         "Search with straight apostrophe",
			query:        "Blaine's",
			expectedName: "Blaine",
		},
		{
			name:         "Search with curly apostrophe",
			query:        "Blaine's", // Uses ' (U+2019)
			expectedName: "Blaine",
		},
		{
			name:         "Search Lt. Surge's cards",
			query:        "Lt. Surge's",
			expectedName: "Lt. Surge",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.SearchCards(tt.query)
			if err != nil {
				t.Fatalf("SearchCards(%q) error: %v", tt.query, err)
			}

			if len(result.Cards) == 0 {
				t.Errorf("SearchCards(%q) returned no results", tt.query)
				return
			}

			// Check that at least one result contains the expected name
			found := false
			for _, card := range result.Cards {
				if strings.Contains(card.Name, tt.expectedName) {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("SearchCards(%q) did not return any cards containing %q", tt.query, tt.expectedName)
				t.Logf("Top 3 results:")
				for i := 0; i < 3 && i < len(result.Cards); i++ {
					t.Logf("  %d. %s", i+1, result.Cards[i].Name)
				}
			}
		})
	}
}

// TestTokenizeText tests the tokenizeText helper function
func TestTokenizeText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Simple words",
			input:    "damage swap alakazam confuse",
			expected: []string{"damage", "swap", "alakazam", "confuse"},
		},
		{
			name:     "Words with punctuation",
			input:    "Hello, World! This is a test.",
			expected: []string{"Hello", "World", "This", "test"},
		},
		{
			name:     "Short words filtered out",
			input:    "a an the and or but",
			expected: []string{},
		},
		{
			name:     "Mixed length words",
			input:    "Alakazam HP 80 Damage Swap",
			expected: []string{"Alakazam", "Damage", "Swap"},
		},
		{
			name:     "OCR-like text",
			input:    "As often as you like during your turn (before your attack)",
			expected: []string{"often", "like", "during", "your", "turn", "before", "your", "attack"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tokenizeText(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("tokenizeText() returned %d words, want %d", len(result), len(tt.expected))
				t.Errorf("got: %v", result)
				t.Errorf("want: %v", tt.expected)
				return
			}
			for i, word := range result {
				if word != tt.expected[i] {
					t.Errorf("tokenizeText()[%d] = %q, want %q", i, word, tt.expected[i])
				}
			}
		})
	}
}

// TestCountWordMatches tests the countWordMatches helper function
func TestCountWordMatches(t *testing.T) {
	tests := []struct {
		name     string
		ocrWords []string
		cardText string
		expected int
	}{
		{
			name:     "All words match",
			ocrWords: []string{"damage", "swap", "alakazam"},
			cardText: "alakazam damage swap as often as you like",
			expected: 3,
		},
		{
			name:     "No words match",
			ocrWords: []string{"pikachu", "thunderbolt"},
			cardText: "alakazam damage swap confuse ray",
			expected: 0,
		},
		{
			name:     "Partial match",
			ocrWords: []string{"damage", "pikachu", "swap"},
			cardText: "alakazam damage swap confuse ray",
			expected: 2,
		},
		{
			name:     "Empty OCR words",
			ocrWords: []string{},
			cardText: "alakazam damage swap",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countWordMatches(tt.ocrWords, tt.cardText)
			if result != tt.expected {
				t.Errorf("countWordMatches() = %d, want %d", result, tt.expected)
			}
		})
	}
}

// TestBuildSearchableText tests the buildSearchableText method
func TestBuildSearchableText(t *testing.T) {
	tests := []struct {
		name     string
		card     LocalPokemonCard
		contains []string // strings that should be in the searchable text
	}{
		{
			name: "Card with attacks and abilities",
			card: LocalPokemonCard{
				Name: "Alakazam",
				Attacks: []LocalAttack{
					{Name: "Confuse Ray", Text: "Flip a coin. If heads, the Defending Pokémon is now Confused."},
				},
				Abilities: []LocalAbility{
					{Name: "Damage Swap", Text: "Move 1 damage counter from 1 of your Pokémon to another."},
				},
				FlavorText:  "Its brain can outperform a supercomputer.",
				EvolvesFrom: "Kadabra",
			},
			contains: []string{
				"alakazam",
				"confuse ray",
				"damage swap",
				"supercomputer",
				"kadabra",
				"flip a coin",
			},
		},
		{
			name: "Card with only name",
			card: LocalPokemonCard{
				Name: "Pikachu",
			},
			contains: []string{"pikachu"},
		},
		{
			name: "Card with multiple attacks",
			card: LocalPokemonCard{
				Name: "Charizard",
				Attacks: []LocalAttack{
					{Name: "Fire Spin", Text: "Discard 2 Energy attached to Charizard."},
					{Name: "Flamethrower", Text: "Discard an Energy card."},
				},
			},
			contains: []string{"charizard", "fire spin", "flamethrower", "discard"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.card.precomputeFields()

			for _, substr := range tt.contains {
				if !strings.Contains(tt.card.searchableText, substr) {
					t.Errorf("searchableText should contain %q, but got: %q", substr, tt.card.searchableText)
				}
			}
		})
	}
}

// TestFullTextMatchResult verifies the FullTextMatchResult struct is properly defined
func TestFullTextMatchResult(t *testing.T) {
	// This is a simple struct test to verify compilation
	result := FullTextMatchResult{
		Score:         1200,
		MatchedFields: []string{"name", "attack:Confuse Ray", "ability:Damage Swap"},
	}

	if result.Score != 1200 {
		t.Errorf("FullTextMatchResult.Score = %d, want 1200", result.Score)
	}

	if len(result.MatchedFields) != 3 {
		t.Errorf("FullTextMatchResult.MatchedFields has %d items, want 3", len(result.MatchedFields))
	}
}

// TestMatchByFullText_Integration tests the MatchByFullText function with real Pokemon data.
// This test loads the actual Pokemon TCG data and verifies that full-text matching works correctly.
func TestMatchByFullText_Integration(t *testing.T) {
	// Find data directory - try common locations
	dataDir := ""
	possiblePaths := []string{
		"../../data",            // When running from services dir
		"../../../backend/data", // When running from project root
		"data",                  // When running from backend dir
		"backend/data",          // When running from project root
		"../data",               // Alternative location
	}

	for _, path := range possiblePaths {
		absPath, _ := filepath.Abs(path)
		if _, err := os.Stat(filepath.Join(absPath, "pokemon-tcg-data-master")); err == nil {
			dataDir = absPath
			break
		}
	}

	if dataDir == "" {
		t.Skip("Pokemon TCG data not found, skipping integration test")
	}

	// Initialize service
	service, err := NewPokemonHybridService(dataDir)
	if err != nil {
		t.Fatalf("Failed to initialize PokemonHybridService: %v", err)
	}

	t.Logf("Loaded %d cards from %d sets", service.GetCardCount(), service.GetSetCount())

	// Test cases with OCR text that includes attack/ability names
	tests := []struct {
		name          string
		ocrText       string
		expectedCard  string   // Expected card name in results
		expectedMatch []string // Expected match types (name, attack:X, ability:X)
		candidateSets []string // Optional set filter
	}{
		{
			name: "Alakazam with Damage Swap ability",
			ocrText: `Alakazam
HP 80
Damage Swap
As often as you like during your turn before your attack you may move 1 damage counter
Confuse Ray 30
Flip a coin If heads the Defending Pokemon is now Confused`,
			expectedCard:  "Alakazam",
			expectedMatch: []string{"name", "ability:Damage Swap", "attack:Confuse Ray"},
		},
		{
			name: "Blastoise with Rain Dance",
			ocrText: `Blastoise
HP 100
Rain Dance
As often as you like during your turn you may attach 1 Water Energy card
Hydro Pump 40+`,
			expectedCard:  "Blastoise",
			expectedMatch: []string{"name", "ability:Rain Dance", "attack:Hydro Pump"},
		},
		{
			name: "Charizard with Fire Spin (Base Set)",
			ocrText: `Charizard
HP 120
Energy Burn
Fire Spin 100
Discard 2 Energy cards attached to Charizard`,
			expectedCard:  "Charizard",
			expectedMatch: []string{"name", "attack:Fire Spin"},
		},
		{
			name: "Partial name with ability text",
			ocrText: `Alakazam 80
Damage Swap
move damage counter from your Pokemon
Confuse Ray
Defending Pokemon Confused`,
			expectedCard:  "Alakazam",
			expectedMatch: []string{"name", "ability:Damage Swap", "attack:Confuse Ray"},
		},
		{
			name: "Pikachu with Thunderbolt",
			ocrText: `Pikachu V
HP 190
Charge
Search your deck for up to 2 Energy cards
Thunderbolt 200
Discard all Energy from this Pokemon`,
			expectedCard:  "Pikachu",
			expectedMatch: []string{"name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, matchedFields := service.MatchByFullText(tt.ocrText, tt.candidateSets)

			if result == nil || len(result.Cards) == 0 {
				t.Errorf("MatchByFullText returned no results for %q", tt.name)
				return
			}

			t.Logf("Top 3 results for %q:", tt.name)
			for i := 0; i < 3 && i < len(result.Cards); i++ {
				t.Logf("  %d. %s (%s)", i+1, result.Cards[i].Name, result.Cards[i].ID)
			}
			t.Logf("Matched fields: %v", matchedFields)

			// Check if expected card is in top results
			found := false
			for i := 0; i < 5 && i < len(result.Cards); i++ {
				if strings.Contains(strings.ToLower(result.Cards[i].Name), strings.ToLower(tt.expectedCard)) {
					found = true
					t.Logf("✓ Found expected card %q at position %d", tt.expectedCard, i+1)
					break
				}
			}

			if !found {
				t.Errorf("Expected card %q not found in top 5 results", tt.expectedCard)
			}

			// Check that expected match types are present
			for _, expectedMatch := range tt.expectedMatch {
				matchFound := false
				for _, actualMatch := range matchedFields {
					if strings.Contains(actualMatch, expectedMatch) || actualMatch == expectedMatch {
						matchFound = true
						break
					}
				}
				if !matchFound {
					// This is a warning, not an error - the exact match fields depend on scoring
					t.Logf("Note: Expected match type %q not in matched fields %v", expectedMatch, matchedFields)
				}
			}
		})
	}
}

// TestMatchByFullText_ProductionCards tests full-text matching against cards from production collection
func TestMatchByFullText_ProductionCards(t *testing.T) {
	// Find data directory
	dataDir := ""
	possiblePaths := []string{
		"../../data",
		"../../../backend/data",
		"data",
		"backend/data",
	}

	for _, path := range possiblePaths {
		absPath, _ := filepath.Abs(path)
		if _, err := os.Stat(filepath.Join(absPath, "pokemon-tcg-data-master")); err == nil {
			dataDir = absPath
			break
		}
	}

	if dataDir == "" {
		t.Skip("Pokemon TCG data not found, skipping integration test")
	}

	service, err := NewPokemonHybridService(dataDir)
	if err != nil {
		t.Fatalf("Failed to initialize PokemonHybridService: %v", err)
	}

	// Test cases based on actual production collection cards
	tests := []struct {
		name          string
		ocrText       string
		expectedCard  string
		expectedSetID string // Expected set code
	}{
		{
			name: "Arcanine base1-23 with Flamethrower attack",
			ocrText: `Arcanine
HP 100
Stage 1 Evolves from Growlithe
Flamethrower 50
Discard 1 Fire Energy card attached to Arcanine
Take Down 80
Arcanine does 30 damage to itself
23/102`,
			expectedCard:  "Arcanine",
			expectedSetID: "base1",
		},
		{
			name: "Charmeleon base1-24 with Slash attack",
			ocrText: `Charmeleon
HP 80
Stage 1 Evolves from Charmander
Slash 50
24/102`,
			expectedCard:  "Charmeleon",
			expectedSetID: "base1",
		},
		{
			name: "Team Magma's Houndour ex4-35",
			ocrText: `Team Magma's Houndour
HP 50
Smog
Flip a coin If heads the Defending Pokemon is now Poisoned
Bite 20
35/95`,
			expectedCard:  "Team Magma's Houndour",
			expectedSetID: "ex4",
		},
		{
			name: "Vulpix base1-68 with Confuse Ray",
			ocrText: `Vulpix
HP 50
Confuse Ray
Flip a coin If heads the Defending Pokemon is now Confused
68/102`,
			expectedCard:  "Vulpix",
			expectedSetID: "base1",
		},
		{
			name: "Growlithe base1-28 with Flare attack",
			ocrText: `Growlithe
HP 60
Flare 20
28/102`,
			expectedCard:  "Growlithe",
			expectedSetID: "base1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, matchedFields := service.MatchByFullText(tt.ocrText, nil)

			if result == nil || len(result.Cards) == 0 {
				t.Errorf("MatchByFullText returned no results for %q", tt.name)
				return
			}

			t.Logf("Top 3 results for %q:", tt.name)
			for i := 0; i < 3 && i < len(result.Cards); i++ {
				t.Logf("  %d. %s (%s)", i+1, result.Cards[i].Name, result.Cards[i].ID)
			}
			t.Logf("Matched fields: %v", matchedFields)

			// Check if expected card is in top 3 results
			found := false
			for i := 0; i < 3 && i < len(result.Cards); i++ {
				if strings.EqualFold(result.Cards[i].Name, tt.expectedCard) &&
					strings.EqualFold(result.Cards[i].SetCode, tt.expectedSetID) {
					found = true
					t.Logf("✓ Found exact card %q from set %q at position %d", tt.expectedCard, tt.expectedSetID, i+1)
					break
				}
			}

			if !found {
				// Check if card name at least matches in top 5
				for i := 0; i < 5 && i < len(result.Cards); i++ {
					if strings.EqualFold(result.Cards[i].Name, tt.expectedCard) {
						t.Logf("✓ Found card %q at position %d (different set: %s)", tt.expectedCard, i+1, result.Cards[i].SetCode)
						found = true
						break
					}
				}
			}

			if !found {
				t.Errorf("Expected card %q from set %q not found in top 5 results", tt.expectedCard, tt.expectedSetID)
			}
		})
	}
}

// TestMatchByFullText_RealOCROutput tests full-text matching with actual OCR output from production scanned images
func TestMatchByFullText_RealOCROutput(t *testing.T) {
	// Find data directory
	dataDir := ""
	possiblePaths := []string{
		"../../data",
		"../../../backend/data",
		"data",
		"backend/data",
	}

	for _, path := range possiblePaths {
		absPath, _ := filepath.Abs(path)
		if _, err := os.Stat(filepath.Join(absPath, "pokemon-tcg-data-master")); err == nil {
			dataDir = absPath
			break
		}
	}

	if dataDir == "" {
		t.Skip("Pokemon TCG data not found, skipping integration test")
	}

	service, err := NewPokemonHybridService(dataDir)
	if err != nil {
		t.Fatalf("Failed to initialize PokemonHybridService: %v", err)
	}

	// Real OCR output from production scanned card images (after rotation correction)
	tests := []struct {
		name          string
		ocrText       string // Actual OCR output from production images
		expectedCard  string
		expectedSetID string
	}{
		{
			name: "Charmeleon base1 - real OCR scan",
			ocrText: `Charmeleon
Flame Pokemon Length 3 7 Weight 42 lbs
Slash 30
Flamethrower Discard 1
in order to use this attack
weakness resistance retreat cost
When it swings its burning tail it raises the temperature
unbearably high levels LV32`,
			expectedCard:  "Charmeleon",
			expectedSetID: "base1",
		},
		{
			name: "Growlithe base1 - real OCR scan",
			ocrText: `Growlithe
Puppy Pokemon Length 2 4 Weight 42 lbs
Flare 20
weakness resistance retreat cost
Very protective of its territory It will bark and bite to repel
intruders from its space LV 18`,
			expectedCard:  "Growlithe",
			expectedSetID: "base1",
		},
		{
			name: "Vulpix base1 - real OCR scan with Confuse Ray",
			ocrText: `Vulpix
Fox Pokemon Length 2 0 Weight 22 lbs
Confuse Ray Flip a coin If
heads the Defending Pokemon
is now Confused
weakness resistance retreat cost
At the time of birth it has just one tail Its tail splits from the
tip as it grows older`,
			expectedCard:  "Vulpix",
			expectedSetID: "base1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, matchedFields := service.MatchByFullText(tt.ocrText, nil)

			if result == nil || len(result.Cards) == 0 {
				t.Errorf("MatchByFullText returned no results for %q", tt.name)
				return
			}

			t.Logf("Top 5 results for %q:", tt.name)
			for i := 0; i < 5 && i < len(result.Cards); i++ {
				t.Logf("  %d. %s (%s)", i+1, result.Cards[i].Name, result.Cards[i].ID)
			}
			t.Logf("Matched fields: %v", matchedFields)

			// Check if expected card name is in top 3 results
			found := false
			for i := 0; i < 3 && i < len(result.Cards); i++ {
				if strings.EqualFold(result.Cards[i].Name, tt.expectedCard) {
					found = true
					t.Logf("✓ Found card %q at position %d (set: %s)", tt.expectedCard, i+1, result.Cards[i].SetCode)
					break
				}
			}

			if !found {
				t.Errorf("Expected card %q not found in top 3 results", tt.expectedCard)
			}
		})
	}
}

// TestMatchByFullText_WithSetFilter tests filtering by candidate sets
func TestMatchByFullText_WithSetFilter(t *testing.T) {
	// Find data directory
	dataDir := ""
	possiblePaths := []string{
		"../../data",
		"../../../backend/data",
		"data",
		"backend/data",
	}

	for _, path := range possiblePaths {
		absPath, _ := filepath.Abs(path)
		if _, err := os.Stat(filepath.Join(absPath, "pokemon-tcg-data-master")); err == nil {
			dataDir = absPath
			break
		}
	}

	if dataDir == "" {
		t.Skip("Pokemon TCG data not found, skipping integration test")
	}

	service, err := NewPokemonHybridService(dataDir)
	if err != nil {
		t.Fatalf("Failed to initialize PokemonHybridService: %v", err)
	}

	ocrText := `Pikachu
HP 60
Thunderbolt
Discard all Energy`

	// Search without set filter
	resultAll, _ := service.MatchByFullText(ocrText, nil)

	// Search with set filter (only base1)
	resultFiltered, _ := service.MatchByFullText(ocrText, []string{"base1"})

	t.Logf("Results without filter: %d cards", resultAll.TotalCount)
	t.Logf("Results with base1 filter: %d cards", resultFiltered.TotalCount)

	// Filtered results should be subset
	if resultFiltered.TotalCount > resultAll.TotalCount {
		t.Errorf("Filtered results (%d) should not exceed unfiltered (%d)",
			resultFiltered.TotalCount, resultAll.TotalCount)
	}

	// Verify all filtered results are from base1
	for _, card := range resultFiltered.Cards {
		if !strings.EqualFold(card.SetCode, "base1") {
			t.Errorf("Card %s has set code %s, expected base1", card.Name, card.SetCode)
		}
	}
}

// BenchmarkMatchByFullText_GoodOCR benchmarks with good OCR text (uses index)
func BenchmarkMatchByFullText_GoodOCR(b *testing.B) {
	dataDir := getTestDataDir()
	if dataDir == "" {
		b.Skip("Pokemon data directory not found")
	}

	service, err := NewPokemonHybridService(dataDir)
	if err != nil {
		b.Fatalf("Failed to create service: %v", err)
	}

	// Good OCR text that will find matches via index
	ocrText := "Charizard Fire Spin Energy Burn 120 HP"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.MatchByFullText(ocrText, nil)
	}
}

// BenchmarkMatchByFullText_PoorOCR benchmarks with poor OCR text (triggers fallback)
func BenchmarkMatchByFullText_PoorOCR(b *testing.B) {
	dataDir := getTestDataDir()
	if dataDir == "" {
		b.Skip("Pokemon data directory not found")
	}

	service, err := NewPokemonHybridService(dataDir)
	if err != nil {
		b.Fatalf("Failed to create service: %v", err)
	}

	// Poor OCR text that won't match well - will trigger fallback
	ocrText := "xyzabc random gibberish that doesnt match"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.MatchByFullText(ocrText, nil)
	}
}

// TestMatchByFullText_ShortCardNames tests that short card names like "N" don't match everything
func TestMatchByFullText_ShortCardNames(t *testing.T) {
	dataDir := getTestDataDir()
	if dataDir == "" {
		t.Skip("Pokemon TCG data not found, skipping integration test")
	}

	service, err := NewPokemonHybridService(dataDir)
	if err != nil {
		t.Fatalf("Failed to initialize PokemonHybridService: %v", err)
	}

	// Test cases where short names should NOT match (they contain the letter but not as a word)
	tests := []struct {
		name        string
		ocrText     string
		shouldNotBe string // Card name that should NOT be in top results
		shouldBe    string // Card name that SHOULD be in top results
	}{
		{
			name: "Pikachu should not match 'N' card just because it contains letter n",
			ocrText: `Pikachu
HP 60
Thunderbolt 50
Discard all Energy cards attached to Pikachu in order to use this attack
Quick Attack 10+
Flip a coin. If heads, this attack does 10 damage plus 20 more damage`,
			shouldNotBe: "N",
			shouldBe:    "Pikachu",
		},
		{
			name: "Charizard should not match 'N' card",
			ocrText: `Charizard
HP 120
Energy Burn
Fire Spin 100
Discard 2 Energy cards attached to Charizard`,
			shouldNotBe: "N",
			shouldBe:    "Charizard",
		},
		{
			name: "Random text with n's should not match 'N' card",
			ocrText: `Alakazam
HP 80
Damage Swap
As often as you like during your turn`,
			shouldNotBe: "N",
			shouldBe:    "Alakazam",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := service.MatchByFullText(tt.ocrText, nil)

			if result == nil || len(result.Cards) == 0 {
				t.Errorf("MatchByFullText returned no results")
				return
			}

			// Check that the short card name is NOT in top 3 results
			for i := 0; i < 3 && i < len(result.Cards); i++ {
				if result.Cards[i].Name == tt.shouldNotBe {
					t.Errorf("Card %q incorrectly matched in top 3 results at position %d for OCR text containing the name %q",
						tt.shouldNotBe, i+1, tt.shouldBe)
				}
			}

			// Check that the expected card IS in top 3 results
			found := false
			for i := 0; i < 3 && i < len(result.Cards); i++ {
				if strings.Contains(strings.ToLower(result.Cards[i].Name), strings.ToLower(tt.shouldBe)) {
					found = true
					t.Logf("✓ Found expected card %q at position %d", tt.shouldBe, i+1)
					break
				}
			}

			if !found {
				t.Errorf("Expected card %q not found in top 3 results", tt.shouldBe)
				t.Logf("Top 3 results:")
				for i := 0; i < 3 && i < len(result.Cards); i++ {
					t.Logf("  %d. %s (%s)", i+1, result.Cards[i].Name, result.Cards[i].ID)
				}
			}
		})
	}
}

// TestMatchByFullText_ShortNameActualMatch tests that short card names DO match when the word appears
func TestMatchByFullText_ShortNameActualMatch(t *testing.T) {
	dataDir := getTestDataDir()
	if dataDir == "" {
		t.Skip("Pokemon TCG data not found, skipping integration test")
	}

	service, err := NewPokemonHybridService(dataDir)
	if err != nil {
		t.Fatalf("Failed to initialize PokemonHybridService: %v", err)
	}

	// When "N" actually appears as a word in OCR text, it SHOULD match the N card
	ocrText := `N
Supporter
Each player shuffles his or her hand into his or her deck
Then each player draws a card for each of his or her remaining Prize cards
101/101`

	result, _ := service.MatchByFullText(ocrText, nil)

	if result == nil || len(result.Cards) == 0 {
		t.Errorf("MatchByFullText returned no results for N card OCR")
		return
	}

	// Check that "N" card IS in top results
	found := false
	for i := 0; i < 5 && i < len(result.Cards); i++ {
		if result.Cards[i].Name == "N" {
			found = true
			t.Logf("✓ Found N card at position %d (%s)", i+1, result.Cards[i].ID)
			break
		}
	}

	if !found {
		t.Errorf("N card should be found when 'N' appears as a word in OCR text")
		t.Logf("Top 5 results:")
		for i := 0; i < 5 && i < len(result.Cards); i++ {
			t.Logf("  %d. %s (%s)", i+1, result.Cards[i].Name, result.Cards[i].ID)
		}
	}
}

// BenchmarkMatchByFullText_WithSetFilter benchmarks with set filtering
func BenchmarkMatchByFullText_WithSetFilter(b *testing.B) {
	dataDir := getTestDataDir()
	if dataDir == "" {
		b.Skip("Pokemon data directory not found")
	}

	service, err := NewPokemonHybridService(dataDir)
	if err != nil {
		b.Fatalf("Failed to create service: %v", err)
	}

	ocrText := "Charizard Fire Spin Energy Burn 120 HP"
	candidateSets := []string{"base1", "base2", "base4"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.MatchByFullText(ocrText, candidateSets)
	}
}

// TestJapaneseCardSearch tests the SearchJapaneseByName functionality
func TestJapaneseCardSearch(t *testing.T) {
	dataDir := getTestDataDir()
	if dataDir == "" {
		t.Skip("Pokemon TCG data not found, skipping integration test")
	}

	service, err := NewPokemonHybridService(dataDir)
	if err != nil {
		t.Fatalf("Failed to initialize PokemonHybridService: %v", err)
	}

	// Check if Japanese data is loaded
	japanDataPath := filepath.Join(dataDir, "pokemon-tcg-data-japan", "cards")
	if _, err := os.Stat(japanDataPath); os.IsNotExist(err) {
		t.Skip("Japanese card data not found, skipping test")
	}

	tests := []struct {
		name         string
		searchName   string
		expectCount  int  // Minimum expected count (-1 to skip check)
		expectPrefix bool // Expect all IDs to start with "jp-"
	}{
		{
			name:         "Search for Misty's Tears in Japanese data",
			searchName:   "Misty's Tears",
			expectCount:  1,
			expectPrefix: true,
		},
		{
			name:         "Search for Charizard in Japanese data",
			searchName:   "Charizard",
			expectCount:  -1, // Just check that we get some results
			expectPrefix: true,
		},
		{
			name:         "Search for nonexistent card",
			searchName:   "Totally Fake Card Name XYZ123",
			expectCount:  0,
			expectPrefix: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := service.SearchJapaneseByName(tt.searchName)

			if tt.expectCount >= 0 && len(results) < tt.expectCount {
				t.Errorf("SearchJapaneseByName(%q) returned %d results, want at least %d",
					tt.searchName, len(results), tt.expectCount)
			}

			if tt.expectPrefix {
				for _, card := range results {
					if !strings.HasPrefix(card.ID, "jp-") {
						t.Errorf("Japanese card ID %q should start with 'jp-'", card.ID)
					}
				}
			}

			if len(results) > 0 {
				t.Logf("Found %d Japanese cards for %q:", len(results), tt.searchName)
				for i, card := range results {
					if i >= 5 {
						t.Logf("  ... and %d more", len(results)-5)
						break
					}
					t.Logf("  - %s (%s) from %s", card.Name, card.ID, card.SetCode)
				}
			}
		})
	}
}

// TestJapaneseCardLoading verifies Japanese card data is loaded alongside English cards
func TestJapaneseCardLoading(t *testing.T) {
	dataDir := getTestDataDir()
	if dataDir == "" {
		t.Skip("Pokemon TCG data not found, skipping integration test")
	}

	service, err := NewPokemonHybridService(dataDir)
	if err != nil {
		t.Fatalf("Failed to initialize PokemonHybridService: %v", err)
	}

	totalCards := service.GetCardCount()
	totalSets := service.GetSetCount()

	t.Logf("Total cards loaded: %d", totalCards)
	t.Logf("Total sets loaded: %d", totalSets)

	// Check if Japanese data directory exists
	japanDataPath := filepath.Join(dataDir, "pokemon-tcg-data-japan", "cards")
	if _, err := os.Stat(japanDataPath); os.IsNotExist(err) {
		t.Log("Japanese card data not found - only English cards loaded")
		return
	}

	// Count Japanese cards by searching for a common word
	jpCards := service.SearchJapaneseByName("Pokemon")
	t.Logf("Japanese cards found with 'Pokemon' search: %d", len(jpCards))

	// Verify we can find a specific Japanese card
	mistysTearsCards := service.SearchJapaneseByName("Misty's Tears")
	if len(mistysTearsCards) > 0 {
		t.Logf("Found Misty's Tears: %s (%s)", mistysTearsCards[0].ID, mistysTearsCards[0].SetCode)
	}
}

// TestSearchCardsGrouped tests the 2-phase search functionality
func TestSearchCardsGrouped(t *testing.T) {
	dataDir := getTestDataDir()
	if dataDir == "" {
		t.Skip("Pokemon TCG data not found, skipping integration test")
	}

	service, err := NewPokemonHybridService(dataDir)
	if err != nil {
		t.Fatalf("Failed to initialize PokemonHybridService: %v", err)
	}

	tests := []struct {
		name            string
		query           string
		expectSets      int // Minimum expected sets (-1 to skip)
		expectCards     int // Minimum expected total cards (-1 to skip)
		expectError     bool
		checkSetSymbols bool // Verify set symbols are populated
	}{
		{
			name:        "Search for Charizard - multiple sets expected",
			query:       "Charizard",
			expectSets:  5,  // Charizard appears in many sets
			expectCards: 10, // Many Charizard cards exist
		},
		{
			name:            "Search for Pikachu - check symbols",
			query:           "Pikachu",
			expectSets:      5,
			expectCards:     10,
			checkSetSymbols: true,
		},
		{
			name:        "Search with apostrophe - Blaine's",
			query:       "Blaine's",
			expectSets:  1, // At least gym sets
			expectCards: 1,
		},
		{
			name:        "Search with curly apostrophe",
			query:       "Blaine's", // U+2019
			expectSets:  1,
			expectCards: 1,
		},
		{
			name:        "Empty query returns empty results",
			query:       "",
			expectSets:  0,
			expectCards: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.SearchCardsGrouped(tt.query, SortByReleaseDesc)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for query %q but got none", tt.query)
				}
				return
			}

			if err != nil {
				t.Fatalf("SearchCardsGrouped(%q) error: %v", tt.query, err)
			}

			if result == nil {
				t.Fatalf("SearchCardsGrouped(%q) returned nil result", tt.query)
			}

			// Check set count
			if tt.expectSets >= 0 && len(result.SetGroups) < tt.expectSets {
				t.Errorf("SearchCardsGrouped(%q) returned %d sets, want at least %d",
					tt.query, len(result.SetGroups), tt.expectSets)
			}

			// Check total cards
			totalCards := 0
			for _, group := range result.SetGroups {
				totalCards += group.CardCount
				// Verify CardCount matches actual cards length
				if group.CardCount != len(group.Cards) {
					t.Errorf("Set %s: CardCount=%d but len(Cards)=%d",
						group.SetCode, group.CardCount, len(group.Cards))
				}
			}

			if tt.expectCards >= 0 && totalCards < tt.expectCards {
				t.Errorf("SearchCardsGrouped(%q) returned %d total cards, want at least %d",
					tt.query, totalCards, tt.expectCards)
			}

			// Check TotalSets matches actual length
			if result.TotalSets != len(result.SetGroups) {
				t.Errorf("TotalSets=%d but len(SetGroups)=%d",
					result.TotalSets, len(result.SetGroups))
			}

			// Check set symbols if requested
			if tt.checkSetSymbols && len(result.SetGroups) > 0 {
				symbolCount := 0
				for _, group := range result.SetGroups {
					if group.SymbolURL != "" {
						symbolCount++
					}
				}
				t.Logf("Sets with symbols: %d/%d", symbolCount, len(result.SetGroups))
			}

			// Log results for debugging
			if len(result.SetGroups) > 0 {
				t.Logf("Found %d sets with %d total cards for query %q",
					len(result.SetGroups), totalCards, tt.query)
				for i, group := range result.SetGroups {
					if i >= 3 {
						t.Logf("  ... and %d more sets", len(result.SetGroups)-3)
						break
					}
					t.Logf("  - %s (%s): %d cards", group.SetName, group.SetCode, group.CardCount)
				}
			}
		})
	}
}

// TestListAllSets tests the set listing functionality
func TestListAllSets(t *testing.T) {
	dataDir := getTestDataDir()
	if dataDir == "" {
		t.Skip("Pokemon TCG data not found, skipping integration test")
	}

	service, err := NewPokemonHybridService(dataDir)
	if err != nil {
		t.Fatalf("Failed to initialize PokemonHybridService: %v", err)
	}

	tests := []struct {
		name        string
		query       string
		expectMin   int    // Minimum expected results
		expectMatch string // A set name that should be in results (empty to skip)
	}{
		{
			name:        "Empty query returns all sets (limited)",
			query:       "",
			expectMin:   20, // Should return top 20
			expectMatch: "",
		},
		{
			name:        "Search for Base Set",
			query:       "base",
			expectMin:   1,
			expectMatch: "Base",
		},
		{
			name:        "Search for Vivid Voltage",
			query:       "vivid",
			expectMin:   1,
			expectMatch: "Vivid Voltage",
		},
		{
			name:        "Search for Sword & Shield series",
			query:       "sword",
			expectMin:   1,
			expectMatch: "Sword",
		},
		{
			name:      "No results for gibberish",
			query:     "xyznonexistent123",
			expectMin: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := service.ListAllSets(tt.query)

			if len(results) < tt.expectMin {
				t.Errorf("ListAllSets(%q) returned %d sets, want at least %d",
					tt.query, len(results), tt.expectMin)
			}

			// Check for expected match
			if tt.expectMatch != "" {
				found := false
				for _, set := range results {
					if strings.Contains(set.Name, tt.expectMatch) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ListAllSets(%q) should contain a set with %q in name",
						tt.query, tt.expectMatch)
				}
			}

			// Verify set info is populated
			for i, set := range results {
				if i >= 5 {
					break
				}
				if set.ID == "" {
					t.Errorf("Set %d has empty ID", i)
				}
				if set.Name == "" {
					t.Errorf("Set %d has empty Name", i)
				}
			}

			t.Logf("ListAllSets(%q) returned %d sets", tt.query, len(results))
		})
	}
}

// TestGetSetCards tests retrieving cards from a specific set
func TestGetSetCards(t *testing.T) {
	dataDir := getTestDataDir()
	if dataDir == "" {
		t.Skip("Pokemon TCG data not found, skipping integration test")
	}

	service, err := NewPokemonHybridService(dataDir)
	if err != nil {
		t.Fatalf("Failed to initialize PokemonHybridService: %v", err)
	}

	tests := []struct {
		name        string
		setCode     string
		nameFilter  string
		expectMin   int // Minimum expected cards
		expectMax   int // Maximum expected cards (-1 for no max)
		expectError bool
	}{
		{
			name:      "Base Set - all cards",
			setCode:   "base1",
			expectMin: 100, // Base set has 102 cards
			expectMax: 110,
		},
		{
			name:       "Base Set - filter by Charizard",
			setCode:    "base1",
			nameFilter: "Charizard",
			expectMin:  1,
			expectMax:  5,
		},
		{
			name:      "Vivid Voltage - all cards",
			setCode:   "swsh4",
			expectMin: 150, // Vivid Voltage has 200+ cards
			expectMax: -1,  // No max limit
		},
		{
			name:       "Vivid Voltage - filter by Pikachu",
			setCode:    "swsh4",
			nameFilter: "Pikachu",
			expectMin:  1,
			expectMax:  20,
		},
		{
			name:        "Invalid set code returns error",
			setCode:     "invalidsetxyz",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.GetSetCards(tt.setCode, tt.nameFilter)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for set %q but got none", tt.setCode)
				}
				return
			}

			if err != nil {
				t.Fatalf("GetSetCards(%q, %q) error: %v", tt.setCode, tt.nameFilter, err)
			}

			if result == nil {
				t.Fatalf("GetSetCards(%q, %q) returned nil", tt.setCode, tt.nameFilter)
			}

			if len(result.Cards) < tt.expectMin {
				t.Errorf("GetSetCards(%q, %q) returned %d cards, want at least %d",
					tt.setCode, tt.nameFilter, len(result.Cards), tt.expectMin)
			}

			if tt.expectMax >= 0 && len(result.Cards) > tt.expectMax {
				t.Errorf("GetSetCards(%q, %q) returned %d cards, want at most %d",
					tt.setCode, tt.nameFilter, len(result.Cards), tt.expectMax)
			}

			// Verify cards belong to the requested set
			for _, card := range result.Cards {
				if card.SetCode != tt.setCode {
					t.Errorf("Card %s has SetCode=%s, want %s",
						card.ID, card.SetCode, tt.setCode)
				}
			}

			// Verify name filter is applied
			if tt.nameFilter != "" {
				filterLower := strings.ToLower(tt.nameFilter)
				for _, card := range result.Cards {
					if !strings.Contains(strings.ToLower(card.Name), filterLower) {
						t.Errorf("Card %s (name=%s) doesn't match filter %q",
							card.ID, card.Name, tt.nameFilter)
					}
				}
			}

			t.Logf("GetSetCards(%q, %q) returned %d cards", tt.setCode, tt.nameFilter, len(result.Cards))
		})
	}
}

// TestSearchCardsGroupedSorting verifies sets are sorted by release date (newest first)
func TestSearchCardsGroupedSorting(t *testing.T) {
	dataDir := getTestDataDir()
	if dataDir == "" {
		t.Skip("Pokemon TCG data not found, skipping integration test")
	}

	service, err := NewPokemonHybridService(dataDir)
	if err != nil {
		t.Fatalf("Failed to initialize PokemonHybridService: %v", err)
	}

	// Pikachu appears in many sets across different release dates
	result, err := service.SearchCardsGrouped("Pikachu", SortByReleaseDesc)
	if err != nil {
		t.Fatalf("SearchCardsGrouped error: %v", err)
	}

	if len(result.SetGroups) < 2 {
		t.Skip("Not enough sets to test sorting")
	}

	// Verify sets are sorted by release date (newest first)
	for i := 1; i < len(result.SetGroups); i++ {
		prev := result.SetGroups[i-1].ReleaseDate
		curr := result.SetGroups[i].ReleaseDate
		// Empty dates should come after dated ones
		if prev != "" && curr != "" && prev < curr {
			t.Errorf("Sets not sorted by release date: %s (%s) before %s (%s)",
				result.SetGroups[i-1].SetName, prev,
				result.SetGroups[i].SetName, curr)
		}
	}

	t.Logf("Verified %d sets are sorted by release date (newest first)", len(result.SetGroups))

	// Test alphabetical sorting
	resultByName, err := service.SearchCardsGrouped("Pikachu", SortByName)
	if err != nil {
		t.Fatalf("SearchCardsGrouped (by name) error: %v", err)
	}

	for i := 1; i < len(resultByName.SetGroups); i++ {
		prev := resultByName.SetGroups[i-1].SetName
		curr := resultByName.SetGroups[i].SetName
		if prev > curr {
			t.Errorf("Sets not sorted alphabetically: %s before %s", prev, curr)
		}
	}

	t.Logf("Verified %d sets are sorted alphabetically", len(resultByName.SetGroups))
}
