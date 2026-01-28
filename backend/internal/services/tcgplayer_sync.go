package services

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/codyseavey/tcg-tracker/backend/internal/database"
	"github.com/codyseavey/tcg-tracker/backend/internal/models"
)

// normalizeNameForPriceMatch converts special characters to ASCII for JustTCG matching
// JustTCG uses ASCII names while Pokemon TCG data uses Unicode
func normalizeNameForPriceMatch(name string) string {
	name = strings.ToLower(name)
	// Pokemon-specific characters
	name = strings.ReplaceAll(name, "♀", " f")     // Nidoran♀ -> nidoran f
	name = strings.ReplaceAll(name, "♂", " m")     // Nidoran♂ -> nidoran m
	name = strings.ReplaceAll(name, "é", "e")      // Pokémon -> pokemon
	name = strings.ReplaceAll(name, "δ", " delta") // Deoxys δ -> deoxys delta
	name = strings.ReplaceAll(name, "'", "")       // Farfetch'd -> farfetchd
	name = strings.ReplaceAll(name, "'", "")       // Curly apostrophe
	name = strings.ReplaceAll(name, ".", "")       // Mr. Mime -> mr mime
	name = strings.ReplaceAll(name, "-", " ")      // Ho-Oh -> ho oh
	// Spelling variations between Pokemon TCG data and JustTCG
	name = strings.ReplaceAll(name, "impostor", "imposter") // Impostor -> Imposter (JustTCG spelling)
	// Normalize multiple spaces
	for strings.Contains(name, "  ") {
		name = strings.ReplaceAll(name, "  ", " ")
	}
	return strings.TrimSpace(name)
}

// TCGPlayerSyncService handles bulk syncing of TCGPlayerIDs from JustTCG
type TCGPlayerSyncService struct {
	justTCG *JustTCGService
	mu      sync.Mutex
	running bool
}

// SyncResult contains the results of a TCGPlayerID sync operation
type SyncResult struct {
	SetsProcessed  int           `json:"sets_processed"`
	CardsUpdated   int           `json:"cards_updated"`
	CardsSkipped   int           `json:"cards_skipped"`
	Errors         []string      `json:"errors,omitempty"`
	Duration       time.Duration `json:"duration"`
	RequestsUsed   int           `json:"requests_used"`
	QuotaRemaining int           `json:"quota_remaining"`
}

// NewTCGPlayerSyncService creates a new sync service
func NewTCGPlayerSyncService(justTCG *JustTCGService) *TCGPlayerSyncService {
	return &TCGPlayerSyncService{
		justTCG: justTCG,
	}
}

// IsRunning returns whether a sync is currently in progress
func (s *TCGPlayerSyncService) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// SyncMissingTCGPlayerIDs finds Pokemon cards without TCGPlayerIDs and syncs them
// This is the main entry point for both the background job and admin endpoint
func (s *TCGPlayerSyncService) SyncMissingTCGPlayerIDs(ctx context.Context) (*SyncResult, error) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil, nil // Already running
	}
	s.running = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	start := time.Now()
	result := &SyncResult{}

	db := database.GetDB()

	// Find all Pokemon cards missing TCGPlayerIDs that are in our collection
	var cardsToSync []models.Card
	err := db.Raw(`
		SELECT DISTINCT c.* FROM cards c
		INNER JOIN collection_items ci ON ci.card_id = c.id
		WHERE c.game = 'pokemon' AND (c.tcg_player_id IS NULL OR c.tcg_player_id = '')
	`).Scan(&cardsToSync).Error
	if err != nil {
		return nil, err
	}

	if len(cardsToSync) == 0 {
		log.Println("TCGPlayerSync: no cards need syncing")
		result.Duration = time.Since(start)
		result.QuotaRemaining = s.justTCG.GetRequestsRemaining()
		return result, nil
	}

	log.Printf("TCGPlayerSync: found %d Pokemon cards missing TCGPlayerIDs", len(cardsToSync))

	// Group cards by set
	cardsBySet := make(map[string][]models.Card)
	for _, card := range cardsToSync {
		setName := card.SetName
		if setName == "" {
			setName = card.SetCode
		}
		cardsBySet[setName] = append(cardsBySet[setName], card)
	}

	log.Printf("TCGPlayerSync: cards spread across %d sets", len(cardsBySet))

	// Process each set
	for setName, cards := range cardsBySet {
		select {
		case <-ctx.Done():
			result.Errors = append(result.Errors, "sync cancelled")
			result.Duration = time.Since(start)
			result.QuotaRemaining = s.justTCG.GetRequestsRemaining()
			log.Printf("TCGPlayerSync: cancelled in %v - %d sets, %d cards updated, %d skipped, %d requests used",
				result.Duration, result.SetsProcessed, result.CardsUpdated, result.CardsSkipped, result.RequestsUsed)
			return result, ctx.Err()
		default:
		}

		// Check quota before each set
		if s.justTCG.GetRequestsRemaining() < 2 {
			result.Errors = append(result.Errors, "quota exhausted, stopping early")
			break
		}

		// Convert our set name to JustTCG format
		justTCGSetID := convertToJustTCGSetID(setName)
		if justTCGSetID == "" {
			log.Printf("TCGPlayerSync: skipping unknown set %q", setName)
			result.CardsSkipped += len(cards)
			continue
		}

		// Fetch TCGPlayerIDs for this set
		setData, err := s.justTCG.FetchSetTCGPlayerIDs(justTCGSetID)
		if err != nil {
			log.Printf("TCGPlayerSync: failed to fetch set %s: %v", justTCGSetID, err)
			result.Errors = append(result.Errors, err.Error())
			result.CardsSkipped += len(cards)
			continue
		}

		result.SetsProcessed++
		result.RequestsUsed++ // Approximate - pagination may use more

		log.Printf("TCGPlayerSync: set %s returned %d cards from JustTCG (by num: %d, by name: %d)",
			justTCGSetID, setData.TotalCards, len(setData.CardsByNum), len(setData.CardsByName))

		// Match and update cards
		setUpdated := 0
		for i := range cards {
			card := &cards[i]
			tcgPlayerID := ""

			// Try matching by card number first
			if card.CardNumber != "" {
				// Normalize card number (remove leading zeros)
				normalizedNum := strings.TrimLeft(card.CardNumber, "0")
				if normalizedNum == "" {
					normalizedNum = "0"
				}

				if id, ok := setData.CardsByNum[normalizedNum]; ok {
					tcgPlayerID = id
				} else if id, ok := setData.CardsByNum[card.CardNumber]; ok {
					tcgPlayerID = id
				}
			}

			// Fallback to name matching (with normalized name for special characters)
			if tcgPlayerID == "" {
				normalizedName := normalizeNameForPriceMatch(card.Name)
				if id, ok := setData.CardsByName[normalizedName]; ok {
					tcgPlayerID = id
				}
			}

			if tcgPlayerID != "" {
				card.TCGPlayerID = tcgPlayerID
				if err := db.Model(card).Update("tcg_player_id", tcgPlayerID).Error; err != nil {
					log.Printf("TCGPlayerSync: failed to update card %s: %v", card.ID, err)
					result.Errors = append(result.Errors, err.Error())
				} else {
					result.CardsUpdated++
					setUpdated++
				}
			} else {
				// Log failed matches for debugging
				normalizedName := normalizeNameForPriceMatch(card.Name)
				log.Printf("TCGPlayerSync: no match for %q #%s in set %s (normalized name: %q)",
					card.Name, card.CardNumber, justTCGSetID, normalizedName)
				// Extra debug for Machamp
				if strings.Contains(strings.ToLower(card.Name), "machamp") {
					log.Printf("TCGPlayerSync debug: Machamp lookup failed. CardsByNum has %d entries, CardsByName has %d entries",
						len(setData.CardsByNum), len(setData.CardsByName))
					log.Printf("TCGPlayerSync debug: Looking for num=%q or normalizedNum=%q, name=%q",
						card.CardNumber, strings.TrimLeft(card.CardNumber, "0"), normalizedName)
					// Check if there's anything with "8" in the keys
					for k, v := range setData.CardsByNum {
						if k == "8" || k == "08" || k == "8/102" {
							log.Printf("TCGPlayerSync debug: Found key %q -> %s", k, v)
						}
					}
					for k, v := range setData.CardsByName {
						if strings.Contains(k, "machamp") {
							log.Printf("TCGPlayerSync debug: Found name key %q -> %s", k, v)
						}
					}
				}
				result.CardsSkipped++
			}
		}

		log.Printf("TCGPlayerSync: processed set %s, updated %d cards", setName, result.CardsUpdated)
	}

	result.Duration = time.Since(start)
	result.QuotaRemaining = s.justTCG.GetRequestsRemaining()

	log.Printf("TCGPlayerSync: completed in %v - %d sets, %d cards updated, %d skipped, %d requests used",
		result.Duration, result.SetsProcessed, result.CardsUpdated, result.CardsSkipped, result.RequestsUsed)

	return result, nil
}

// SyncSet syncs TCGPlayerIDs for a specific set (admin use)
func (s *TCGPlayerSyncService) SyncSet(ctx context.Context, ourSetName string) (*SyncResult, error) {
	start := time.Now()
	result := &SyncResult{}

	db := database.GetDB()

	// Find all Pokemon cards in this set missing TCGPlayerIDs
	var cardsToSync []models.Card
	err := db.Where("game = ? AND (set_name = ? OR set_code = ?) AND (tcg_player_id IS NULL OR tcg_player_id = '')",
		models.GamePokemon, ourSetName, ourSetName).Find(&cardsToSync).Error
	if err != nil {
		return nil, err
	}

	if len(cardsToSync) == 0 {
		result.Duration = time.Since(start)
		result.QuotaRemaining = s.justTCG.GetRequestsRemaining()
		return result, nil
	}

	// Convert to JustTCG set ID
	justTCGSetID := convertToJustTCGSetID(ourSetName)
	if justTCGSetID == "" {
		result.Errors = append(result.Errors, "unknown set: "+ourSetName)
		return result, nil
	}

	// Fetch TCGPlayerIDs
	setData, err := s.justTCG.FetchSetTCGPlayerIDs(justTCGSetID)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result, err
	}

	result.SetsProcessed = 1
	result.RequestsUsed = 1

	// Match and update cards
	for i := range cardsToSync {
		card := &cardsToSync[i]
		tcgPlayerID := ""

		// Try matching by card number
		if card.CardNumber != "" {
			normalizedNum := strings.TrimLeft(card.CardNumber, "0")
			if normalizedNum == "" {
				normalizedNum = "0"
			}

			if id, ok := setData.CardsByNum[normalizedNum]; ok {
				tcgPlayerID = id
			} else if id, ok := setData.CardsByNum[card.CardNumber]; ok {
				tcgPlayerID = id
			}
		}

		// Fallback to name (with normalized name for special characters)
		if tcgPlayerID == "" {
			normalizedName := normalizeNameForPriceMatch(card.Name)
			if id, ok := setData.CardsByName[normalizedName]; ok {
				tcgPlayerID = id
			}
		}

		if tcgPlayerID != "" {
			card.TCGPlayerID = tcgPlayerID
			if err := db.Model(card).Update("tcg_player_id", tcgPlayerID).Error; err != nil {
				result.Errors = append(result.Errors, err.Error())
			} else {
				result.CardsUpdated++
			}
		} else {
			result.CardsSkipped++
		}
	}

	result.Duration = time.Since(start)
	result.QuotaRemaining = s.justTCG.GetRequestsRemaining()

	return result, nil
}

// convertToJustTCGSetID converts our set name/code to JustTCG's set ID format
// JustTCG uses format like "vivid-voltage-pokemon", "base-set-pokemon"
func convertToJustTCGSetID(ourSetName string) string {
	// Normalize: lowercase, replace spaces with hyphens
	normalized := strings.ToLower(ourSetName)
	normalized = strings.ReplaceAll(normalized, " ", "-")
	normalized = strings.ReplaceAll(normalized, "'", "")
	normalized = strings.ReplaceAll(normalized, "&", "and")

	// Known mappings from our set codes/names to JustTCG set IDs
	// JustTCG format: swsh0X-set-name-pokemon or sv0X-set-name-pokemon
	knownMappings := map[string]string{
		// Sword & Shield era - main sets (JustTCG uses swsh0X- prefix)
		"swsh1":            "swsh01-sword-and-shield-pokemon",
		"sword-and-shield": "swsh01-sword-and-shield-pokemon",
		"swsh2":            "swsh02-rebel-clash-pokemon",
		"rebel-clash":      "swsh02-rebel-clash-pokemon",
		"swsh3":            "swsh03-darkness-ablaze-pokemon",
		"darkness-ablaze":  "swsh03-darkness-ablaze-pokemon",
		"swsh4":            "swsh04-vivid-voltage-pokemon",
		"vivid-voltage":    "swsh04-vivid-voltage-pokemon",
		"swsh5":            "swsh05-battle-styles-pokemon",
		"battle-styles":    "swsh05-battle-styles-pokemon",
		"swsh6":            "swsh06-chilling-reign-pokemon",
		"chilling-reign":   "swsh06-chilling-reign-pokemon",
		"swsh7":            "swsh07-evolving-skies-pokemon",
		"evolving-skies":   "swsh07-evolving-skies-pokemon",
		"swsh8":            "swsh08-fusion-strike-pokemon",
		"fusion-strike":    "swsh08-fusion-strike-pokemon",
		"swsh9":            "swsh09-brilliant-stars-pokemon",
		"brilliant-stars":  "swsh09-brilliant-stars-pokemon",
		"swsh10":           "swsh10-astral-radiance-pokemon",
		"astral-radiance":  "swsh10-astral-radiance-pokemon",
		"swsh11":           "swsh11-lost-origin-pokemon",
		"lost-origin":      "swsh11-lost-origin-pokemon",
		"swsh12":           "swsh12-silver-tempest-pokemon",
		"silver-tempest":   "swsh12-silver-tempest-pokemon",
		"swsh12pt5":        "swsh12pt5-crown-zenith-pokemon",
		"crown-zenith":     "swsh12pt5-crown-zenith-pokemon",

		// Sword & Shield special sets
		"swsh35":         "swsh35-champions-path-pokemon",
		"champions-path": "swsh35-champions-path-pokemon",
		"swsh45":         "swsh45-shining-fates-pokemon",
		"shining-fates":  "swsh45-shining-fates-pokemon",
		"cel25":          "cel25-celebrations-pokemon",
		"celebrations":   "cel25-celebrations-pokemon",
		"pgo":            "pgo-pokemon-go-pokemon",
		"pokemon-go":     "pgo-pokemon-go-pokemon",
		"swsh45sv":       "swsh45sv-shiny-vault-pokemon",
		"shiny-vault":    "swsh45sv-shiny-vault-pokemon",

		// Scarlet & Violet era - main sets (JustTCG uses sv0X- prefix)
		"sv1":                  "sv01-scarlet-and-violet-pokemon",
		"scarlet-and-violet":   "sv01-scarlet-and-violet-pokemon",
		"sv2":                  "sv02-paldea-evolved-pokemon",
		"paldea-evolved":       "sv02-paldea-evolved-pokemon",
		"sv3":                  "sv03-obsidian-flames-pokemon",
		"obsidian-flames":      "sv03-obsidian-flames-pokemon",
		"sv3pt5":               "sv03pt5-151-pokemon",
		"151":                  "sv03pt5-151-pokemon",
		"pokemon-151":          "sv03pt5-151-pokemon",
		"sv4":                  "sv04-paradox-rift-pokemon",
		"paradox-rift":         "sv04-paradox-rift-pokemon",
		"sv4pt5":               "sv04pt5-paldean-fates-pokemon",
		"paldean-fates":        "sv04pt5-paldean-fates-pokemon",
		"sv5":                  "sv05-temporal-forces-pokemon",
		"temporal-forces":      "sv05-temporal-forces-pokemon",
		"sv6":                  "sv06-twilight-masquerade-pokemon",
		"twilight-masquerade":  "sv06-twilight-masquerade-pokemon",
		"sv6pt5":               "sv06pt5-shrouded-fable-pokemon",
		"shrouded-fable":       "sv06pt5-shrouded-fable-pokemon",
		"sv7":                  "sv07-stellar-crown-pokemon",
		"stellar-crown":        "sv07-stellar-crown-pokemon",
		"sv8":                  "sv08-surging-sparks-pokemon",
		"surging-sparks":       "sv08-surging-sparks-pokemon",
		"sv8pt5":               "sv08pt5-prismatic-evolutions-pokemon",
		"prismatic-evolutions": "sv08pt5-prismatic-evolutions-pokemon",

		// Scarlet & Violet special sets
		"svp":                       "svp-scarlet-and-violet-promos-pokemon",
		"scarlet-and-violet-promos": "svp-scarlet-and-violet-promos-pokemon",

		// Classic sets (WotC era)
		"si1":                  "southern-islands-pokemon",
		"southern-islands":     "southern-islands-pokemon",
		"base1":                "base-set-pokemon",
		"base":                 "base-set-pokemon",
		"base-set":             "base-set-pokemon",
		"jungle":               "jungle-pokemon",
		"fossil":               "fossil-pokemon",
		"base2":                "base-set-2-pokemon",
		"base4":                "base-set-2-pokemon", // Pokemon TCG API code
		"base-set-2":           "base-set-2-pokemon",
		"team-rocket":          "team-rocket-pokemon",
		"gym1":                 "gym-heroes-pokemon",
		"gym-heroes":           "gym-heroes-pokemon",
		"gym2":                 "gym-challenge-pokemon",
		"gym-challenge":        "gym-challenge-pokemon",
		"neo1":                 "neo-genesis-pokemon",
		"neo-genesis":          "neo-genesis-pokemon",
		"neo2":                 "neo-discovery-pokemon",
		"neo-discovery":        "neo-discovery-pokemon",
		"neo3":                 "neo-revelation-pokemon",
		"neo-revelation":       "neo-revelation-pokemon",
		"neo4":                 "neo-destiny-pokemon",
		"neo-destiny":          "neo-destiny-pokemon",
		"base6":                "legendary-collection-pokemon",
		"legendary-collection": "legendary-collection-pokemon",

		// e-Card era
		"ecard1":              "expedition-pokemon",
		"expedition-base-set": "expedition-pokemon",
		"expedition":          "expedition-pokemon",
		"ecard2":              "aquapolis-pokemon",
		"aquapolis":           "aquapolis-pokemon",
		"ecard3":              "skyridge-pokemon",
		"skyridge":            "skyridge-pokemon",

		// EX era
		"ex1":                     "ruby-and-sapphire-pokemon",
		"ruby-and-sapphire":       "ruby-and-sapphire-pokemon",
		"ex2":                     "sandstorm-pokemon",
		"sandstorm":               "sandstorm-pokemon",
		"ex3":                     "dragon-pokemon",
		"dragon":                  "dragon-pokemon",
		"ex4":                     "team-magma-vs-team-aqua-pokemon",
		"team-magma-vs-team-aqua": "team-magma-vs-team-aqua-pokemon",
		"ex5":                     "hidden-legends-pokemon",
		"hidden-legends":          "hidden-legends-pokemon",
		"ex6":                     "firered-leafgreen-pokemon",
		"firered-and-leafgreen":   "firered-leafgreen-pokemon",
		"firered-leafgreen":       "firered-leafgreen-pokemon",
		"ex7":                     "team-rocket-returns-pokemon",
		"team-rocket-returns":     "team-rocket-returns-pokemon",
		"ex8":                     "deoxys-pokemon",
		"deoxys":                  "deoxys-pokemon",
		"ex9":                     "emerald-pokemon",
		"emerald":                 "emerald-pokemon",
		"ex10":                    "unseen-forces-pokemon",
		"unseen-forces":           "unseen-forces-pokemon",
		"ex11":                    "delta-species-pokemon",
		"delta-species":           "delta-species-pokemon",
		"ex12":                    "legend-maker-pokemon",
		"legend-maker":            "legend-maker-pokemon",
		"ex13":                    "holon-phantoms-pokemon",
		"holon-phantoms":          "holon-phantoms-pokemon",
		"ex14":                    "crystal-guardians-pokemon",
		"crystal-guardians":       "crystal-guardians-pokemon",
		"ex15":                    "dragon-frontiers-pokemon",
		"dragon-frontiers":        "dragon-frontiers-pokemon",
		"ex16":                    "power-keepers-pokemon",
		"power-keepers":           "power-keepers-pokemon",

		// Diamond & Pearl era
		"dp1":                  "diamond-and-pearl-pokemon",
		"diamond-and-pearl":    "diamond-and-pearl-pokemon",
		"dp2":                  "mysterious-treasures-pokemon",
		"mysterious-treasures": "mysterious-treasures-pokemon",
		"dp3":                  "secret-wonders-pokemon",
		"secret-wonders":       "secret-wonders-pokemon",
		"dp4":                  "great-encounters-pokemon",
		"great-encounters":     "great-encounters-pokemon",
		"dp5":                  "majestic-dawn-pokemon",
		"majestic-dawn":        "majestic-dawn-pokemon",
		"dp6":                  "legends-awakened-pokemon",
		"legends-awakened":     "legends-awakened-pokemon",
		"dp7":                  "stormfront-pokemon",
		"stormfront":           "stormfront-pokemon",

		// Platinum era
		"pl1":             "platinum-pokemon",
		"platinum":        "platinum-pokemon",
		"pl2":             "rising-rivals-pokemon",
		"rising-rivals":   "rising-rivals-pokemon",
		"pl3":             "supreme-victors-pokemon",
		"supreme-victors": "supreme-victors-pokemon",
		"pl4":             "arceus-pokemon",
		"arceus":          "arceus-pokemon",

		// HeartGold SoulSilver era
		"hgss1":                "heartgold-and-soulsilver-pokemon",
		"heartgold-soulsilver": "heartgold-and-soulsilver-pokemon",
		"hgss2":                "unleashed-pokemon",
		"unleashed":            "unleashed-pokemon",
		"hgss3":                "undaunted-pokemon",
		"undaunted":            "undaunted-pokemon",
		"hgss4":                "triumphant-pokemon",
		"triumphant":           "triumphant-pokemon",

		// Black & White era
		"bw1":                 "black-and-white-pokemon",
		"black-and-white":     "black-and-white-pokemon",
		"bw2":                 "emerging-powers-pokemon",
		"emerging-powers":     "emerging-powers-pokemon",
		"bw3":                 "noble-victories-pokemon",
		"noble-victories":     "noble-victories-pokemon",
		"bw4":                 "next-destinies-pokemon",
		"next-destinies":      "next-destinies-pokemon",
		"bw5":                 "dark-explorers-pokemon",
		"dark-explorers":      "dark-explorers-pokemon",
		"bw6":                 "dragons-exalted-pokemon",
		"dragons-exalted":     "dragons-exalted-pokemon",
		"bw7":                 "boundaries-crossed-pokemon",
		"boundaries-crossed":  "boundaries-crossed-pokemon",
		"bw8":                 "plasma-storm-pokemon",
		"plasma-storm":        "plasma-storm-pokemon",
		"bw9":                 "plasma-freeze-pokemon",
		"plasma-freeze":       "plasma-freeze-pokemon",
		"bw10":                "plasma-blast-pokemon",
		"plasma-blast":        "plasma-blast-pokemon",
		"bw11":                "legendary-treasures-pokemon",
		"legendary-treasures": "legendary-treasures-pokemon",

		// XY era
		"xy1":             "xy-pokemon",
		"xy":              "xy-pokemon",
		"xy2":             "flashfire-pokemon",
		"flashfire":       "flashfire-pokemon",
		"xy3":             "furious-fists-pokemon",
		"furious-fists":   "furious-fists-pokemon",
		"xy4":             "phantom-forces-pokemon",
		"phantom-forces":  "phantom-forces-pokemon",
		"xy5":             "primal-clash-pokemon",
		"primal-clash":    "primal-clash-pokemon",
		"xy6":             "roaring-skies-pokemon",
		"roaring-skies":   "roaring-skies-pokemon",
		"xy7":             "ancient-origins-pokemon",
		"ancient-origins": "ancient-origins-pokemon",
		"xy8":             "breakthrough-pokemon",
		"breakthrough":    "breakthrough-pokemon",
		"xy9":             "breakpoint-pokemon",
		"breakpoint":      "breakpoint-pokemon",
		"xy10":            "fates-collide-pokemon",
		"fates-collide":   "fates-collide-pokemon",
		"xy11":            "steam-siege-pokemon",
		"steam-siege":     "steam-siege-pokemon",
		"xy12":            "evolutions-pokemon",
		"evolutions":      "evolutions-pokemon",

		// Sun & Moon era
		"sm1":              "sun-and-moon-pokemon",
		"sun-and-moon":     "sun-and-moon-pokemon",
		"sm2":              "guardians-rising-pokemon",
		"guardians-rising": "guardians-rising-pokemon",
		"sm3":              "burning-shadows-pokemon",
		"burning-shadows":  "burning-shadows-pokemon",
		"sm4":              "crimson-invasion-pokemon",
		"crimson-invasion": "crimson-invasion-pokemon",
		"sm5":              "ultra-prism-pokemon",
		"ultra-prism":      "ultra-prism-pokemon",
		"sm6":              "forbidden-light-pokemon",
		"forbidden-light":  "forbidden-light-pokemon",
		"sm7":              "celestial-storm-pokemon",
		"celestial-storm":  "celestial-storm-pokemon",
		"sm8":              "lost-thunder-pokemon",
		"lost-thunder":     "lost-thunder-pokemon",
		"sm9":              "team-up-pokemon",
		"team-up":          "team-up-pokemon",
		"sm10":             "unbroken-bonds-pokemon",
		"unbroken-bonds":   "unbroken-bonds-pokemon",
		"sm11":             "unified-minds-pokemon",
		"unified-minds":    "unified-minds-pokemon",
		"sm12":             "cosmic-eclipse-pokemon",
		"cosmic-eclipse":   "cosmic-eclipse-pokemon",

		// Promos
		"basep":                      "wotc-promo-pokemon",
		"wizards-black-star-promos":  "wotc-promo-pokemon",
		"np":                         "nintendo-promos-pokemon",
		"nintendo-black-star-promos": "nintendo-promos-pokemon",
		"pop1":                       "pop-series-1-pokemon",
		"pop2":                       "pop-series-2-pokemon",
		"pop3":                       "pop-series-3-pokemon",
		"pop4":                       "pop-series-4-pokemon",
		"pop5":                       "pop-series-5-pokemon",
		"pop6":                       "pop-series-6-pokemon",
		"pop7":                       "pop-series-7-pokemon",
		"pop8":                       "pop-series-8-pokemon",
		"pop9":                       "pop-series-9-pokemon",

		// EX Trainer Kits - JustTCG combines both halves into one set
		"tk1a":                  "ex-trainer-kit-1-latias-latios-pokemon",
		"tk1b":                  "ex-trainer-kit-1-latias-latios-pokemon",
		"ex-trainer-kit-latias": "ex-trainer-kit-1-latias-latios-pokemon",
		"ex-trainer-kit-latios": "ex-trainer-kit-1-latias-latios-pokemon",
		"tk2a":                  "ex-trainer-kit-2-plusle-minun-pokemon",
		"tk2b":                  "ex-trainer-kit-2-plusle-minun-pokemon",
	}

	// Check direct mapping first
	if justTCGID, ok := knownMappings[normalized]; ok {
		return justTCGID
	}

	// Check if our set code matches
	if justTCGID, ok := knownMappings[ourSetName]; ok {
		return justTCGID
	}

	// Unknown set - return empty string so caller can handle appropriately
	// Don't guess with "-pokemon" suffix as it causes failed API requests
	return ""
}
