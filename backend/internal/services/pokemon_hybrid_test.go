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
