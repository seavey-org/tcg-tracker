// migrate-japanese-collection migrates Japanese collection items from English card IDs
// to proper Japanese card IDs (jp-*) for accurate pricing.
//
// Usage: go run main.go -db=<path> -data=<dir> [-dry-run] [-execute]
//
// The tool:
// 1. Finds collection items where language='Japanese' AND card_id NOT LIKE 'jp-%'
// 2. For each item, searches Japanese cards by name
// 3. If unique match: auto-migrates (with --execute)
// 4. If ambiguous: prompts user to select (interactive mode)
// 5. If no match: logs for manual review
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gorm.io/gorm"

	"github.com/codyseavey/tcg-tracker/backend/internal/database"
	"github.com/codyseavey/tcg-tracker/backend/internal/models"
)

// JapaneseCard represents a card from the Japanese pokemon-tcg-data-japan files
type JapaneseCard struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Supertype   string `json:"supertype"`
	Number      string `json:"number"`
	Rarity      string `json:"rarity"`
	TCGPlayerID string `json:"tcgplayerId"`
	SetID       string // Populated from filename
	SetName     string // Populated from sets.json
}

type JapaneseSet struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Series string `json:"series"`
}

// englishToJapaneseSet maps English set codes to their Japanese equivalents.
// This helps resolve ambiguous matches when a card exists in multiple Japanese sets.
var englishToJapaneseSet = map[string]string{
	// Base era
	"base1": "jp-base-expansion-pack",    // Base Set → Base Expansion Pack
	"base3": "jp-mystery-of-the-fossils", // Fossil → Mystery of the Fossils
	"base5": "jp-rocket-gang",            // Team Rocket → Rocket Gang

	// Neo era
	"neo1": "jp-gold-silver-to-a-new-world", // Neo Genesis → Gold, Silver, to a New World
	"neo2": "jp-awakening-legends",          // Neo Discovery → Awakening Legends (partial)
	"neo3": "jp-awakening-legends",          // Neo Revelation → Awakening Legends

	// Gym era - these were split in Japan
	"gym1": "jp-leaders-stadium", // Gym Heroes → Leaders' Stadium
	"gym2": "jp-leaders-stadium", // Gym Challenge → Leaders' Stadium (partial)
}

// MigrationResult tracks the outcome of each migration attempt
type MigrationResult struct {
	CollectionItemID uint
	OldCardID        string
	OldCardName      string
	NewCardID        string
	NewCardName      string
	Action           string // "migrated", "skipped", "no_match", "ambiguous_skipped"
	Reason           string
}

func main() {
	dbPath := flag.String("db", "", "Path to SQLite database (required)")
	dataDir := flag.String("data", "", "Path to pokemon-tcg-data directory (required)")
	dryRun := flag.Bool("dry-run", false, "Preview changes without modifying database")
	execute := flag.Bool("execute", false, "Execute the migration (required to make changes)")
	skipAmbiguous := flag.Bool("skip-ambiguous", false, "Skip ambiguous matches instead of prompting")
	flag.Parse()

	if *dbPath == "" || *dataDir == "" {
		fmt.Println("Usage: migrate-japanese-collection -db=<path> -data=<dir> [options]")
		fmt.Println("")
		fmt.Println("Migrates Japanese collection items from English card IDs to proper")
		fmt.Println("Japanese card IDs (jp-*) for accurate pricing from JustTCG.")
		fmt.Println("")
		fmt.Println("Options:")
		fmt.Println("  -db              Path to SQLite database (required)")
		fmt.Println("  -data            Path to pokemon-tcg-data directory (required)")
		fmt.Println("  -dry-run         Preview changes without modifying database")
		fmt.Println("  -execute         Execute the migration (required to make changes)")
		fmt.Println("  -skip-ambiguous  Skip ambiguous matches instead of prompting")
		fmt.Println("")
		fmt.Println("Examples:")
		fmt.Println("  # Preview what would be migrated")
		fmt.Println("  migrate-japanese-collection -db=./tcg_tracker.db -data=./data -dry-run")
		fmt.Println("")
		fmt.Println("  # Execute migration with interactive prompts")
		fmt.Println("  migrate-japanese-collection -db=./tcg_tracker.db -data=./data -execute")
		os.Exit(1)
	}

	if !*dryRun && !*execute {
		fmt.Println("Error: Must specify either -dry-run or -execute")
		os.Exit(1)
	}

	// Load Japanese cards from data files
	log.Println("Loading Japanese card data...")
	japaneseCards, sets, err := loadJapaneseCards(*dataDir)
	if err != nil {
		log.Fatalf("Failed to load Japanese cards: %v", err)
	}
	log.Printf("Loaded %d Japanese cards from %d sets", len(japaneseCards), len(sets))

	// Build name index for fast lookup
	nameIndex := buildNameIndex(japaneseCards)

	// Initialize database
	if err := database.Initialize(*dbPath); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	db := database.GetDB()

	// Find Japanese collection items with English card IDs
	var items []models.CollectionItem
	if err := db.Preload("Card").
		Where("language = ?", models.LanguageJapanese).
		Where("card_id NOT LIKE ?", "jp-%").
		Find(&items).Error; err != nil {
		log.Fatalf("Failed to query collection items: %v", err)
	}

	log.Printf("Found %d Japanese collection items with English card IDs", len(items))

	if len(items) == 0 {
		fmt.Println("No items to migrate!")
		return
	}

	// Process each item
	var results []MigrationResult
	reader := bufio.NewReader(os.Stdin)

	for i, item := range items {
		cardName := item.Card.Name
		fmt.Printf("\n[%d/%d] Processing: %s (ID: %s, Set: %s)\n",
			i+1, len(items), cardName, item.CardID, item.Card.SetCode)

		// Find matching Japanese cards, using set mapping to resolve ambiguity
		matches := findMatches(cardName, item.Card.SetCode, nameIndex, japaneseCards, sets)

		var result MigrationResult
		result.CollectionItemID = item.ID
		result.OldCardID = item.CardID
		result.OldCardName = cardName

		if len(matches) == 0 {
			result.Action = "no_match"
			result.Reason = "No Japanese card found with matching name"
			fmt.Printf("  ❌ No match found for '%s'\n", cardName)
		} else if len(matches) == 1 {
			// Unique match - auto-migrate
			match := matches[0]
			result.NewCardID = match.ID
			result.NewCardName = match.Name
			result.Action = "migrated"
			result.Reason = fmt.Sprintf("Unique match in %s", match.SetName)
			fmt.Printf("  ✓ Unique match: %s (%s)\n", match.ID, match.SetName)

			if *execute && !*dryRun {
				if err := migrateItem(db, item.ID, match.ID); err != nil {
					result.Action = "error"
					result.Reason = err.Error()
					fmt.Printf("  ⚠ Migration failed: %v\n", err)
				}
			}
		} else {
			// Multiple matches - need user input
			fmt.Printf("  ⚠ Found %d possible matches:\n", len(matches))
			for j, m := range matches {
				fmt.Printf("    [%d] %s - %s (%s #%s)\n", j+1, m.ID, m.Name, m.SetName, m.Number)
			}

			if *skipAmbiguous {
				result.Action = "ambiguous_skipped"
				result.Reason = fmt.Sprintf("Multiple matches (%d), skipped", len(matches))
				fmt.Printf("  → Skipped (ambiguous)\n")
			} else if *dryRun {
				result.Action = "ambiguous_skipped"
				result.Reason = fmt.Sprintf("Multiple matches (%d), would prompt in execute mode", len(matches))
			} else {
				// Interactive prompt
				fmt.Printf("  Select match (1-%d), or 's' to skip: ", len(matches))
				input, _ := reader.ReadString('\n')
				input = strings.TrimSpace(strings.ToLower(input))

				if input == "s" || input == "" {
					result.Action = "ambiguous_skipped"
					result.Reason = "User skipped"
					fmt.Printf("  → Skipped by user\n")
				} else {
					var selection int
					if _, err := fmt.Sscanf(input, "%d", &selection); err == nil && selection >= 1 && selection <= len(matches) {
						match := matches[selection-1]
						result.NewCardID = match.ID
						result.NewCardName = match.Name
						result.Action = "migrated"
						result.Reason = fmt.Sprintf("User selected from %d matches", len(matches))

						if *execute {
							if err := migrateItem(db, item.ID, match.ID); err != nil {
								result.Action = "error"
								result.Reason = err.Error()
								fmt.Printf("  ⚠ Migration failed: %v\n", err)
							} else {
								fmt.Printf("  ✓ Migrated to %s\n", match.ID)
							}
						}
					} else {
						result.Action = "ambiguous_skipped"
						result.Reason = "Invalid selection"
						fmt.Printf("  → Invalid selection, skipped\n")
					}
				}
			}
		}

		results = append(results, result)
	}

	// Print summary
	printSummary(results, *dryRun)
}

func loadJapaneseCards(dataDir string) ([]JapaneseCard, map[string]JapaneseSet, error) {
	japanDir := filepath.Join(dataDir, "pokemon-tcg-data", "pokemon-tcg-data-japan")

	// Load sets
	setsFile := filepath.Join(japanDir, "sets.json")
	setsData, err := os.ReadFile(setsFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read sets.json: %w", err)
	}

	var setsList []JapaneseSet
	if err := json.Unmarshal(setsData, &setsList); err != nil {
		return nil, nil, fmt.Errorf("failed to parse sets.json: %w", err)
	}

	sets := make(map[string]JapaneseSet)
	for _, s := range setsList {
		sets[s.ID] = s
	}

	// Load cards from all set files
	cardsDir := filepath.Join(japanDir, "cards")
	files, err := os.ReadDir(cardsDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read cards directory: %w", err)
	}

	var allCards []JapaneseCard
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		setID := strings.TrimSuffix(file.Name(), ".json")
		cardFile := filepath.Join(cardsDir, file.Name())
		data, err := os.ReadFile(cardFile)
		if err != nil {
			log.Printf("Warning: failed to read %s: %v", cardFile, err)
			continue
		}

		var cards []JapaneseCard
		if err := json.Unmarshal(data, &cards); err != nil {
			log.Printf("Warning: failed to parse %s: %v", cardFile, err)
			continue
		}

		// Populate set info
		set := sets[setID]
		for i := range cards {
			cards[i].SetID = setID
			cards[i].SetName = set.Name
		}

		allCards = append(allCards, cards...)
	}

	return allCards, sets, nil
}

// normalizeName converts special characters to their ASCII equivalents for matching.
// JustTCG uses ASCII names (e.g., "Pokemon March", "Nidoran m") while English card data
// uses Unicode (e.g., "Pokémon March", "Nidoran ♂").
func normalizeName(name string) string {
	// Convert to lowercase first
	name = strings.ToLower(name)

	// Gender symbols
	name = strings.ReplaceAll(name, "♂", " m")
	name = strings.ReplaceAll(name, "♀", " f")

	// Accented characters (common in Pokemon names)
	name = strings.ReplaceAll(name, "é", "e")
	name = strings.ReplaceAll(name, "è", "e")
	name = strings.ReplaceAll(name, "ê", "e")
	name = strings.ReplaceAll(name, "ë", "e")

	// Clean up any double spaces
	for strings.Contains(name, "  ") {
		name = strings.ReplaceAll(name, "  ", " ")
	}

	return strings.TrimSpace(name)
}

func buildNameIndex(cards []JapaneseCard) map[string][]int {
	index := make(map[string][]int)
	for i, card := range cards {
		normalized := normalizeName(card.Name)
		index[normalized] = append(index[normalized], i)
	}
	return index
}

func findMatches(englishName, englishSetCode string, nameIndex map[string][]int, cards []JapaneseCard, sets map[string]JapaneseSet) []JapaneseCard {
	normalized := normalizeName(englishName)

	var matches []JapaneseCard

	// Try exact match first
	if indices, ok := nameIndex[normalized]; ok {
		for _, idx := range indices {
			matches = append(matches, cards[idx])
		}
	}

	// If no exact matches, try partial matches
	if len(matches) == 0 {
		for _, card := range cards {
			cardNormalized := normalizeName(card.Name)
			if strings.Contains(cardNormalized, normalized) || strings.Contains(normalized, cardNormalized) {
				matches = append(matches, card)
			}
		}
	}

	// If we have multiple matches and a set mapping, filter by mapped set
	if len(matches) > 1 {
		if mappedSet, ok := englishToJapaneseSet[englishSetCode]; ok {
			var filtered []JapaneseCard
			for _, m := range matches {
				if m.SetID == mappedSet {
					filtered = append(filtered, m)
				}
			}
			// Only use filtered results if we found matches in the mapped set
			if len(filtered) > 0 {
				matches = filtered
			}
		}
	}

	// Sort by set name for consistent display
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].SetName < matches[j].SetName
	})

	return matches
}

func migrateItem(db *gorm.DB, itemID uint, newCardID string) error {
	// Note: Pokemon cards are loaded into memory by PokemonHybridService, not stored in the database.
	// The card data will be populated when the collection is queried.
	// We just need to update the card_id reference.

	// Update the collection item's card_id
	return db.Model(&models.CollectionItem{}).
		Where("id = ?", itemID).
		Update("card_id", newCardID).Error
}

func printSummary(results []MigrationResult, dryRun bool) {
	var migrated, noMatch, ambiguousSkipped, errors int
	for _, r := range results {
		switch r.Action {
		case "migrated":
			migrated++
		case "no_match":
			noMatch++
		case "ambiguous_skipped":
			ambiguousSkipped++
		case "error":
			errors++
		}
	}

	fmt.Println("")
	fmt.Println("=== Migration Summary ===")
	if dryRun {
		fmt.Println("(DRY RUN - no changes made)")
	}
	fmt.Printf("Migrated:          %d\n", migrated)
	fmt.Printf("No match found:    %d\n", noMatch)
	fmt.Printf("Ambiguous skipped: %d\n", ambiguousSkipped)
	fmt.Printf("Errors:            %d\n", errors)
	fmt.Printf("Total processed:   %d\n", len(results))

	// Print items that need manual attention
	if noMatch > 0 {
		fmt.Println("\n--- Items with no Japanese match (need manual mapping) ---")
		for _, r := range results {
			if r.Action == "no_match" {
				fmt.Printf("  ID %d: %s (%s)\n", r.CollectionItemID, r.OldCardName, r.OldCardID)
			}
		}
	}

	if ambiguousSkipped > 0 && dryRun {
		fmt.Println("\n--- Items with multiple matches (will prompt in execute mode) ---")
		for _, r := range results {
			if r.Action == "ambiguous_skipped" {
				fmt.Printf("  ID %d: %s (%s) - %s\n", r.CollectionItemID, r.OldCardName, r.OldCardID, r.Reason)
			}
		}
	}
}
