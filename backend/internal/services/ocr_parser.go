package services

import (
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

var (
	germanKpPattern  = regexp.MustCompile(`\b(\d{2,3})\s*KP\b|\bKP\s*(\d{2,3})\b`)
	frenchPvPattern  = regexp.MustCompile(`\b(\d{2,3})\s*PV\b|\bPV\s*(\d{2,3})\b`)
	italianPsPattern = regexp.MustCompile(`\b(\d{2,3})\s*PS\b|\bPS\s*(\d{2,3})\b`)
)

// parseInt safely parses an integer, returning 0 on failure
func parseInt(s string) int {
	val, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return val
}

// OCRResult contains parsed information from OCR text
type OCRResult struct {
	FoilIndicators     []string           `json:"foil_indicators"`     // what triggered foil detection
	FirstEdIndicators  []string           `json:"first_ed_indicators"` // what triggered first edition detection
	AllLines           []string           `json:"all_lines"`
	ConditionHints     []string           `json:"condition_hints"` // hints about card condition
	CandidateSets      []string           `json:"candidate_sets"`  // possible sets when ambiguous (from set total)
	RawText            string             `json:"raw_text"`
	CardName           string             `json:"card_name"`
	CardNumber         string             `json:"card_number"`          // e.g., "25" from "025/185"
	SetTotal           string             `json:"set_total"`            // e.g., "185" from "025/185"
	SetCode            string             `json:"set_code"`             // e.g., "SWSH4" if detected
	SetName            string             `json:"set_name"`             // e.g., "Vivid Voltage" if detected
	CopyrightYear      string             `json:"copyright_year"`       // e.g., "2022" from "© 2022 Wizards"
	HP                 string             `json:"hp"`                   // e.g., "170" from "HP 170"
	Rarity             string             `json:"rarity"`               // if detected
	MatchReason        string             `json:"match_reason"`         // how set was determined: "set_code", "set_name", "set_total", "inferred"
	DetectedLanguage   string             `json:"detected_language"`    // e.g., "English", "Japanese", "German", etc.
	Confidence         float64            `json:"confidence"`           // 0-1 based on how much we extracted
	IsFoil             bool               `json:"is_foil"`              // detected foil indicators (conservative)
	IsFirstEdition     bool               `json:"is_first_edition"`     // detected first edition
	IsWotCEra          bool               `json:"is_wotc_era"`          // detected Wizards of the Coast era card
	SuggestedCondition string             `json:"suggested_condition"`  // from image analysis
	EdgeWhiteningScore float64            `json:"edge_whitening_score"` // from image analysis
	CornerScores       map[string]float64 `json:"corner_scores"`        // from image analysis
	FoilConfidence     float64            `json:"foil_confidence"`      // 0-1 confidence from text/image analysis
}

// ImageAnalysis contains results from client-side image analysis
type ImageAnalysis struct {
	IsFoilDetected     bool               `json:"is_foil_detected"`
	FoilConfidence     float64            `json:"foil_confidence"`
	SuggestedCondition string             `json:"suggested_condition"`
	EdgeWhiteningScore float64            `json:"edge_whitening_score"`
	CornerScores       map[string]float64 `json:"corner_scores"`
}

// Maximum allowed OCR text length to prevent regex DoS
const maxOCRTextLength = 10000

// Pokemon TCG set name to set code mapping
var pokemonSetNameToCode = map[string]string{
	// Scarlet & Violet Era
	"SCARLET & VIOLET":   "sv1",
	"SCARLET AND VIOLET": "sv1",
	"PALDEA EVOLVED":     "sv2",
	"OBSIDIAN FLAMES":    "sv3",
	"151":                "sv3pt5",
	// Note: "MEW" removed as it matches "MEWTWO" - use "151" for sv3pt5 detection
	"PARADOX RIFT":         "sv4",
	"PALDEAN FATES":        "sv4pt5",
	"TEMPORAL FORCES":      "sv5",
	"TWILIGHT MASQUERADE":  "sv6",
	"SHROUDED FABLE":       "sv6pt5",
	"STELLAR CROWN":        "sv7",
	"SURGING SPARKS":       "sv8",
	"PRISMATIC EVOLUTIONS": "sv8pt5",
	"JOURNEY TOGETHER":     "sv9",

	// Sword & Shield Era
	"SWORD & SHIELD":   "swsh1",
	"SWORD AND SHIELD": "swsh1",
	"REBEL CLASH":      "swsh2",
	"DARKNESS ABLAZE":  "swsh3",
	"CHAMPION'S PATH":  "swsh3pt5",
	"CHAMPIONS PATH":   "swsh3pt5",
	"VIVID VOLTAGE":    "swsh4",
	"SHINING FATES":    "swsh4pt5",
	"BATTLE STYLES":    "swsh5",
	"CHILLING REIGN":   "swsh6",
	"EVOLVING SKIES":   "swsh7",
	"CELEBRATIONS":     "cel25",
	"FUSION STRIKE":    "swsh8",
	"BRILLIANT STARS":  "swsh9",
	"ASTRAL RADIANCE":  "swsh10",
	"POKEMON GO":       "pgo",
	"LOST ORIGIN":      "swsh11",
	"SILVER TEMPEST":   "swsh12",
	"CROWN ZENITH":     "swsh12pt5",

	// Sun & Moon Era
	"SUN & MOON":        "sm1",
	"SUN AND MOON":      "sm1",
	"GUARDIANS RISING":  "sm2",
	"BURNING SHADOWS":   "sm3",
	"SHINING LEGENDS":   "sm3pt5",
	"CRIMSON INVASION":  "sm4",
	"ULTRA PRISM":       "sm5",
	"FORBIDDEN LIGHT":   "sm6",
	"CELESTIAL STORM":   "sm7",
	"DRAGON MAJESTY":    "sm7pt5",
	"LOST THUNDER":      "sm8",
	"TEAM UP":           "sm9",
	"DETECTIVE PIKACHU": "det1",
	"UNBROKEN BONDS":    "sm10",
	"UNIFIED MINDS":     "sm11",
	"HIDDEN FATES":      "sm11pt5",
	"COSMIC ECLIPSE":    "sm12",

	// XY Era
	"XY":              "xy1",
	"FLASHFIRE":       "xy2",
	"FURIOUS FISTS":   "xy3",
	"PHANTOM FORCES":  "xy4",
	"PRIMAL CLASH":    "xy5",
	"ROARING SKIES":   "xy6",
	"ANCIENT ORIGINS": "xy7",
	"BREAKTHROUGH":    "xy8",
	"BREAKPOINT":      "xy9",
	"FATES COLLIDE":   "xy10",
	"STEAM SIEGE":     "xy11",
	"EVOLUTIONS":      "xy12",

	// Black & White Era
	"BLACK & WHITE":       "bw1",
	"BLACK AND WHITE":     "bw1",
	"EMERGING POWERS":     "bw2",
	"NOBLE VICTORIES":     "bw3",
	"NEXT DESTINIES":      "bw4",
	"DARK EXPLORERS":      "bw5",
	"DRAGONS EXALTED":     "bw6",
	"BOUNDARIES CROSSED":  "bw7",
	"PLASMA STORM":        "bw8",
	"PLASMA FREEZE":       "bw9",
	"PLASMA BLAST":        "bw10",
	"LEGENDARY TREASURES": "bw11",

	// HeartGold & SoulSilver Era
	"HEARTGOLD & SOULSILVER": "hgss1",
	"HEARTGOLD SOULSILVER":   "hgss1",
	"HGSS":                   "hgss1",
	"UNLEASHED":              "hgss2",
	"UNDAUNTED":              "hgss3",
	"TRIUMPHANT":             "hgss4",
	"CALL OF LEGENDS":        "col1",

	// Diamond & Pearl Era
	"DIAMOND & PEARL":      "dp1",
	"DIAMOND AND PEARL":    "dp1",
	"MYSTERIOUS TREASURES": "dp2",
	"SECRET WONDERS":       "dp3",
	"GREAT ENCOUNTERS":     "dp4",
	"MAJESTIC DAWN":        "dp5",
	"LEGENDS AWAKENED":     "dp6",
	"STORMFRONT":           "dp7",
	"PLATINUM":             "pl1",
	"RISING RIVALS":        "pl2",
	"SUPREME VICTORS":      "pl3",
	"ARCEUS":               "pl4",

	// EX Era (Ruby & Sapphire through Power Keepers)
	"RUBY & SAPPHIRE":         "ex1",
	"RUBY AND SAPPHIRE":       "ex1",
	"SANDSTORM":               "ex2",
	"EX DRAGON":               "ex3", // Avoid matching "Dragon" alone (too common)
	"TEAM MAGMA VS TEAM AQUA": "ex4",
	"HIDDEN LEGENDS":          "ex5",
	"FIRERED & LEAFGREEN":     "ex6",
	"TEAM ROCKET RETURNS":     "ex7",
	"DEOXYS":                  "ex8",
	"EMERALD":                 "ex9",
	"UNSEEN FORCES":           "ex10",
	"DELTA SPECIES":           "ex11",
	"LEGEND MAKER":            "ex12",
	"HOLON PHANTOMS":          "ex13",
	"CRYSTAL GUARDIANS":       "ex14",
	"DRAGON FRONTIERS":        "ex15",
	"POWER KEEPERS":           "ex16",

	// Base Era (Original WotC Sets)
	"BASE SET": "base1",
	// Note: "BASE" alone is not included as it's too ambiguous
	// (matches "base damage", "base attack", etc. in card text)
	"JUNGLE":               "base2",
	"FOSSIL":               "base3",
	"BASE SET 2":           "base4",
	"TEAM ROCKET":          "base5",
	"LEGENDARY COLLECTION": "base6",
	"GYM HEROES":           "gym1",
	"GYM CHALLENGE":        "gym2",
	"NEO GENESIS":          "neo1",
	"NEO DISCOVERY":        "neo2",
	"NEO REVELATION":       "neo3",
	"NEO DESTINY":          "neo4",
	"EXPEDITION":           "ecard1",
	"AQUAPOLIS":            "ecard2",
	"SKYRIDGE":             "ecard3",

	// WotC Promo
	"WIZARDS BLACK STAR": "basep",
	"WOTC PROMO":         "basep",
	"BLACK STAR PROMO":   "basep",
}

// Pokemon TCG PTCGO codes to set code mapping
// These 2-letter codes appear on physical cards (bottom left/right) for online redemption
var pokemonPTCGOToCode = map[string]string{
	// Base Era
	"BS": "base1", // Base Set
	"JU": "base2", // Jungle
	"FO": "base3", // Fossil
	"B2": "base4", // Base Set 2
	"TR": "base5", // Team Rocket
	"LC": "base6", // Legendary Collection
	"G1": "gym1",  // Gym Heroes
	"G2": "gym2",  // Gym Challenge
	"N1": "neo1",  // Neo Genesis
	"N2": "neo2",  // Neo Discovery
	"N3": "neo3",  // Neo Revelation
	"N4": "neo4",  // Neo Destiny
	"SI": "si1",   // Southern Islands
	// e-Card Era
	// Note: "EX" removed as it conflicts with modern "ex" suffix (Koraidon ex, etc.)
	// Use set name "EXPEDITION" or set total detection instead
	"AQ": "ecard2", // Aquapolis
	"SK": "ecard3", // Skyridge
	// Note: Modern sets use different codes (e.g., SVI for Scarlet & Violet)
}

// Pokemon TCG set total to possible set codes mapping
// When a card has XX/YYY format, we can sometimes infer the set from the total
// Note: Some totals are shared between sets, those are listed with multiple options
// Sets are ordered by preference (newer/more common first)
var pokemonSetTotalToCode = map[string][]string{
	// Scarlet & Violet Era - unique totals
	"193": {"sv2"},    // Paldea Evolved (193 cards)
	"197": {"sv3"},    // Obsidian Flames (197 cards)
	"182": {"sv4"},    // Paradox Rift (182 cards)
	"218": {"sv5"},    // Temporal Forces (218 cards)
	"167": {"sv6"},    // Twilight Masquerade (167 cards)
	"175": {"sv7"},    // Stellar Crown (175 cards)
	"191": {"sv8"},    // Surging Sparks (191 cards)
	"186": {"sv8pt5"}, // Prismatic Evolutions (186 cards)
	"169": {"sv9"},    // Journey Together (169 cards)

	// Sword & Shield Era - unique totals
	"202": {"swsh1"},     // Sword & Shield base
	"192": {"swsh2"},     // Rebel Clash
	"185": {"swsh4"},     // Vivid Voltage
	"163": {"swsh5"},     // Battle Styles
	"203": {"swsh7"},     // Evolving Skies
	"264": {"swsh8"},     // Fusion Strike
	"172": {"swsh9"},     // Brilliant Stars
	"196": {"swsh11"},    // Lost Origin
	"195": {"swsh12"},    // Silver Tempest
	"159": {"swsh12pt5"}, // Crown Zenith main set

	// Shared totals (multiple possible sets) - prefer newer set
	"198": {"sv1", "swsh6"},    // SV1 or Chilling Reign
	"189": {"swsh10", "swsh3"}, // Astral Radiance or Darkness Ablaze

	// Sun & Moon Era
	"156": {"sm5"},     // Ultra Prism (156 cards)
	"131": {"sm6"},     // Forbidden Light (131 cards)
	"168": {"sm7"},     // Celestial Storm (168 cards)
	"214": {"sm8"},     // Lost Thunder (214 cards)
	"181": {"sm9"},     // Team Up (181 cards)
	"234": {"sm10"},    // Unbroken Bonds (234 cards)
	"236": {"sm11"},    // Unified Minds (236 cards)
	"271": {"sm12"},    // Cosmic Eclipse (271 cards)
	"69":  {"sm7pt5"},  // Dragon Majesty (69 cards)
	"68":  {"sm11pt5"}, // Hidden Fates (68 cards in main set)

	// XY Era
	"119": {"xy4"},  // Phantom Forces (119 cards)
	"164": {"xy5"},  // Primal Clash (164 cards)
	"162": {"xy8"},  // BREAKthrough (162 cards)
	"125": {"xy10"}, // Fates Collide (125 cards)

	// Black & White Era
	"135": {"bw8"},  // Plasma Storm (135 cards)
	"116": {"bw9"},  // Plasma Freeze (116 cards)
	"115": {"bw11"}, // Legendary Treasures (115 cards)

	// Platinum Era
	"127": {"pl1"}, // Platinum (127 cards)

	// Base Era set totals (combined with other eras that share same totals)
	"102": {"base1", "hgss4"},   // Base Set (102) or Triumphant (102)
	"64":  {"base2", "sv6pt5"},  // Jungle (64) or Shrouded Fable (64)
	"62":  {"base3"},            // Fossil (62 cards)
	"130": {"base4", "dp1"},     // Base Set 2 (130) or DP Base (130)
	"82":  {"base5"},            // Team Rocket (82 cards)
	"83":  {"base5"},            // Team Rocket alternate count (83 with Dark Raichu)
	"132": {"gym1", "dp3"},      // Gym Heroes (132) or Secret Wonders (132)
	"129": {"gym2"},             // Gym Challenge (129 cards)
	"75":  {"neo2"},             // Neo Discovery (75 cards)
	"66":  {"neo3"},             // Neo Revelation (66 cards)
	"165": {"sv3pt5", "ecard1"}, // 151 or Expedition (both have ~165)
	"144": {"ecard3"},           // Skyridge (144 cards)

	// HeartGold & SoulSilver Era
	"123": {"hgss1", "dp2"}, // HGSS Base (123) or Mysterious Treasures (123)
	"95":  {"hgss2"},        // Unleashed (95 cards)
	"90":  {"hgss3"},        // Undaunted (90 cards)

	// Diamond & Pearl Era
	"100": {"dp5"}, // Majestic Dawn (100 cards)

	// Consolidated shared totals (many sets share these)
	// 73: Champion's Path, Shining Legends
	"73": {"swsh3pt5", "sm3pt5"},
	// 72: Shining Fates
	"72": {"swsh4pt5"},
	// 78: Pokemon GO
	"78": {"pgo"},
	// 25: Celebrations
	"25": {"cel25"},
	// 91: Paldean Fates
	"91": {"sv4pt5"},
	// 98/098: Ancient Origins, Emerging Powers
	"98": {"xy7", "bw2"},
	// 99: Next Destinies, Arceus
	"99": {"bw4", "pl4"},
	// 101: Noble Victories, Plasma Blast
	"101": {"bw3", "bw10"},
	// 106: Call of Legends, Great Encounters, Stormfront
	"106": {"col1", "dp4", "dp7"},
	// 108: Roaring Skies, Evolutions, Dark Explorers
	"108": {"xy6", "xy12", "bw5"},
	// 109: Flashfire
	"109": {"xy2"},
	// 110: Legendary Collection
	"110": {"base6"},
	// 111: Neo Genesis, Furious Fists, Rising Rivals
	"111": {"neo1", "xy3", "pl2"},
	// 113: Neo Destiny
	"113": {"neo4"},
	// 114: Steam Siege, B&W Base
	"114": {"xy11", "bw1"},
	// 122: BREAKpoint
	"122": {"xy9"},
	// 124: Crimson Invasion, Dragons Exalted
	"124": {"sm4", "bw6"},
	// 145: Guardians Rising
	"145": {"sm2"},
	// 146: XY Base, Legends Awakened
	"146": {"xy1", "dp6"},
	// 147: Burning Shadows, Aquapolis, Supreme Victors
	"147": {"sm3", "ecard2", "pl3"},
	// 149: Sun & Moon Base, Boundaries Crossed
	"149": {"sm1", "bw7"},
}

// dynamicPokemonNames holds Pokemon names loaded from the card database
// This is set by InitPokemonNamesFromData() and takes priority over the fallback list
var dynamicPokemonNames []string
var dynamicNamesInitialized bool

// InitPokemonNamesFromData initializes the Pokemon name list from card database data
// This should be called after the Pokemon data is loaded to enable comprehensive name matching
func InitPokemonNamesFromData(names []string) {
	if len(names) == 0 {
		return
	}
	dynamicPokemonNames = names
	dynamicNamesInitialized = true
	log.Printf("OCR Parser: Initialized with %d Pokemon names from database", len(names))
}

// GetPokemonNameCount returns the number of Pokemon names available for OCR matching
func GetPokemonNameCount() int {
	return len(getPokemonNames())
}

// getPokemonNames returns the best available Pokemon name list
// Prefers dynamically loaded names from database, falls back to hardcoded list
func getPokemonNames() []string {
	if dynamicNamesInitialized && len(dynamicPokemonNames) > 0 {
		return dynamicPokemonNames
	}
	return fallbackPokemonNames
}

// fallbackPokemonNames contains a hardcoded list of common Pokemon names
// Used when dynamic names from the database are not available
// Sorted by length (longest first) to prevent partial matches
var fallbackPokemonNames = func() []string {
	names := []string{
		// Original 151 Pokemon (Base Set era)
		"bulbasaur", "ivysaur", "venusaur", "charmander", "charmeleon", "charizard",
		"squirtle", "wartortle", "blastoise", "caterpie", "metapod", "butterfree",
		"weedle", "kakuna", "beedrill", "pidgey", "pidgeotto", "pidgeot",
		"rattata", "raticate", "spearow", "fearow", "ekans", "arbok",
		"pikachu", "raichu", "sandshrew", "sandslash", "nidoran", "nidorina",
		"nidoqueen", "nidorino", "nidoking", "clefairy", "clefable", "vulpix",
		"ninetales", "jigglypuff", "wigglytuff", "zubat", "golbat", "oddish",
		"gloom", "vileplume", "paras", "parasect", "venonat", "venomoth",
		"diglett", "dugtrio", "meowth", "persian", "psyduck", "golduck",
		"mankey", "primeape", "growlithe", "arcanine", "poliwag", "poliwhirl",
		"poliwrath", "abra", "kadabra", "alakazam", "machop", "machoke",
		"machamp", "bellsprout", "weepinbell", "victreebel", "tentacool", "tentacruel",
		"geodude", "graveler", "golem", "ponyta", "rapidash", "slowpoke",
		"slowbro", "magnemite", "magneton", "farfetch'd", "doduo", "dodrio",
		"seel", "dewgong", "grimer", "muk", "shellder", "cloyster",
		"gastly", "haunter", "gengar", "onix", "drowzee", "hypno",
		"krabby", "kingler", "voltorb", "electrode", "exeggcute", "exeggutor",
		"cubone", "marowak", "hitmonlee", "hitmonchan", "lickitung", "koffing",
		"weezing", "rhyhorn", "rhydon", "chansey", "tangela", "kangaskhan",
		"horsea", "seadra", "goldeen", "seaking", "staryu", "starmie",
		"mr. mime", "scyther", "jynx", "electabuzz", "magmar", "pinsir",
		"tauros", "magikarp", "gyarados", "lapras", "ditto", "eevee",
		"vaporeon", "jolteon", "flareon", "porygon", "omanyte", "omastar",
		"kabuto", "kabutops", "aerodactyl", "snorlax", "articuno", "zapdos",
		"moltres", "dratini", "dragonair", "dragonite", "mewtwo", "mew",
		// Gen 2 popular
		"chikorita", "cyndaquil", "totodile", "umbreon", "espeon", "lugia",
		"ho-oh", "celebi", "tyranitar", "scizor", "heracross",
		// Later legendaries and popular Pokemon
		"rayquaza", "arceus", "giratina", "dialga", "palkia",
		"jirachi", "deoxys", "darkrai", "shaymin", "lucario", "garchomp",
		"sylveon", "greninja", "zekrom", "reshiram",
		// Sword/Shield and Scarlet/Violet Pokemon
		"zacian", "zamazenta", "eternatus", "urshifu", "calyrex",
		"miraidon", "koraidon", "chien-pao", "wo-chien", "ting-lu", "chi-yu",
		"iron valiant", "iron hands", "iron thorns", "roaring moon", "great tusk",
		"slither wing", "brute bonnet", "flutter mane", "sandy shocks",
		"lechonk", "smoliv", "fidough", "cetitan", "baxcalibur",
		"kingambit", "palafin", "tinkaton", "armarouge", "ceruledge",
		"gholdengo", "annihilape", "pawmot", "rabsca", "garganacl",
		"dondozo", "tatsugiri", "orthworm", "glimmora", "greavard",
		"houndstone", "revavroom", "cyclizar", "flamigo", "klawf",
		"lokix", "grafaiai", "squawkabilly", "nacli", "charcadet",
	}
	// Sort by length descending (longest first)
	sort.Slice(names, func(i, j int) bool {
		return len(names[i]) > len(names[j])
	})
	return names
}()

// ParseOCRText extracts card information from OCR text
func ParseOCRText(text string, game string) *OCRResult {
	return ParseOCRTextWithAnalysis(text, game, nil)
}

// ParseOCRTextWithAnalysis extracts card information from OCR text and incorporates image analysis
func ParseOCRTextWithAnalysis(text string, game string, imageAnalysis *ImageAnalysis) *OCRResult {
	// Truncate overly long text to prevent regex DoS
	if len(text) > maxOCRTextLength {
		text = text[:maxOCRTextLength]
	}

	result := &OCRResult{
		RawText:      text,
		CornerScores: make(map[string]float64),
	}

	// Split into lines and clean
	lines := strings.Split(text, "\n")
	var cleanLines []string
	for _, line := range lines {
		cleaned := strings.TrimSpace(line)
		if cleaned != "" {
			cleanLines = append(cleanLines, cleaned)
		}
	}
	result.AllLines = cleanLines

	if game == "pokemon" {
		parsePokemonOCR(result)
	} else {
		parseMTGOCR(result)
	}

	// Incorporate image analysis if provided
	if imageAnalysis != nil {
		applyImageAnalysis(result, imageAnalysis)
	}

	// Calculate confidence based on what we extracted
	result.Confidence = calculateConfidence(result)

	return result
}

// applyImageAnalysis incorporates client-side image analysis into OCR results
// Uses conservative foil detection: only auto-set IsFoil if confidence >= 0.8
func applyImageAnalysis(result *OCRResult, analysis *ImageAnalysis) {
	// Copy condition assessment data
	result.SuggestedCondition = analysis.SuggestedCondition
	result.EdgeWhiteningScore = analysis.EdgeWhiteningScore
	result.CornerScores = analysis.CornerScores

	// Use higher of text-based or image-based foil confidence
	if analysis.FoilConfidence > result.FoilConfidence {
		result.FoilConfidence = analysis.FoilConfidence
	}

	// Conservative foil detection: only auto-set IsFoil if high confidence (>= 0.8)
	if analysis.FoilConfidence >= 0.8 && analysis.IsFoilDetected {
		result.IsFoil = true
		result.FoilIndicators = append(result.FoilIndicators, "Image analysis detected foil (high confidence)")
	} else if analysis.IsFoilDetected && analysis.FoilConfidence >= 0.5 {
		// Medium confidence: add indicator but don't auto-set IsFoil
		result.FoilIndicators = append(result.FoilIndicators, "Image analysis suggests foil (medium confidence)")
		// Note: NOT setting IsFoil = true for medium confidence
	}
	// Low confidence (< 0.5): don't add any indicator or set IsFoil
}

// normalizeOCRDigits replaces common OCR misreads of digits
// O -> 0, l -> 1, I -> 1 (in numeric contexts)
func normalizeOCRDigits(s string) string {
	// Replace O with 0 only if it looks like it's in a number context
	result := strings.ReplaceAll(s, "O", "0")
	result = strings.ReplaceAll(result, "o", "0")
	// Replace lowercase l with 1 in number patterns
	result = strings.ReplaceAll(result, "l", "1")
	return result
}

// normalizeLineForNameMatch normalizes common OCR errors for name matching
// Converts digits to their common letter lookalikes: 0->o, 1->i, 5->s
func normalizeLineForNameMatch(line string) string {
	result := line
	// Replace digits with their letter lookalikes for matching
	result = strings.ReplaceAll(result, "0", "o")
	result = strings.ReplaceAll(result, "1", "i")
	result = strings.ReplaceAll(result, "5", "s")
	result = strings.ReplaceAll(result, "8", "b")
	result = strings.ReplaceAll(result, "4", "a")
	// Replace common character confusions
	result = strings.ReplaceAll(result, "rn", "m")
	result = strings.ReplaceAll(result, "RN", "M")
	result = strings.ReplaceAll(result, "cl", "d")
	result = strings.ReplaceAll(result, "CL", "D")
	result = strings.ReplaceAll(result, "ii", "u")
	result = strings.ReplaceAll(result, "ll", "u")
	// Replace spaces in the middle of names
	result = regexp.MustCompile(`(\w)\s(\w)`).ReplaceAllString(result, "$1$2")
	return result
}

// levenshteinDistance calculates the edit distance between two strings
func levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	// Create matrix
	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
		matrix[i][0] = i
	}
	for j := 0; j <= len(s2); j++ {
		matrix[0][j] = j
	}

	// Fill in the matrix
	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 1
			if s1[i-1] == s2[j-1] {
				cost = 0
			}
			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(s1)][len(s2)]
}

// looksLikeOCRGarbage detects short strings that are likely OCR noise
// rather than real card names. Examples: "TQG", "Zollvp", "HPAO"
// Real card names usually have spaces, punctuation, or are longer.
func looksLikeOCRGarbage(name string) bool {
	// Names with spaces or apostrophes are likely real (e.g., "Professor's Research")
	if strings.Contains(name, " ") || strings.Contains(name, "'") {
		return false
	}

	// Longer names (10+ chars) are less likely to be garbage
	if len(name) >= 10 {
		return false
	}

	lower := strings.ToLower(name)

	// Short all-uppercase strings (like "TQG", "HPAO") are likely garbage
	// unless they're known Pokemon names (which would have matched fuzzy)
	upper := strings.ToUpper(name)
	if name == upper && len(name) <= 8 {
		return true
	}

	// Short strings without vowels are suspicious (most real words have vowels)
	hasVowel := strings.ContainsAny(lower, "aeiou")
	if !hasVowel && len(name) <= 6 {
		return true
	}

	// Check for unusual consonant clusters that indicate OCR garbage
	// Real English/Pokemon names rarely have 3+ consonants in a row (except common patterns)
	consonantRun := 0
	maxConsonantRun := 0
	for _, r := range lower {
		if strings.ContainsRune("bcdfghjklmnpqrstvwxyz", r) {
			consonantRun++
			if consonantRun > maxConsonantRun {
				maxConsonantRun = consonantRun
			}
		} else {
			consonantRun = 0
		}
	}
	// "Zollvp" has "llvp" = 4 consonants in a row, which is unusual
	if maxConsonantRun >= 4 && len(name) <= 8 {
		return true
	}

	return false
}

// fuzzyMatchPokemonName attempts to find a Pokemon name that closely matches the input
// Returns the matched name and whether a match was found
func fuzzyMatchPokemonName(input string) (string, bool) {
	input = strings.ToLower(strings.TrimSpace(input))
	if len(input) < 3 {
		return "", false
	}

	// First try exact match
	for _, name := range getPokemonNames() {
		if input == name {
			return name, true
		}
	}

	// Try normalized match
	normalizedInput := strings.ToLower(normalizeLineForNameMatch(input))
	for _, name := range getPokemonNames() {
		if normalizedInput == name {
			return name, true
		}
	}

	// Try fuzzy matching with Levenshtein distance
	// Allow 1 error for names <= 6 chars, 2 errors for longer names
	bestMatch := ""
	bestDistance := 999

	for _, name := range getPokemonNames() {
		// Skip if lengths are too different (optimization)
		if abs(len(input)-len(name)) > 3 {
			continue
		}

		distance := levenshteinDistance(normalizedInput, name)
		maxAllowedDistance := 1
		if len(name) > 6 {
			maxAllowedDistance = 2
		}

		if distance <= maxAllowedDistance && distance < bestDistance {
			bestDistance = distance
			bestMatch = name
		}
	}

	if bestMatch != "" {
		return bestMatch, true
	}

	return "", false
}

// abs returns the absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// normalizeCardName applies OCR corrections specifically for Pokemon names
func normalizeCardName(name string) string {
	// Common OCR errors for specific Pokemon names
	corrections := map[string]string{
		// Base Set Pokemon with common OCR errors
		"Charizarcl":  "Charizard",
		"Charízard":   "Charizard",
		"Char1zard":   "Charizard",
		"Blasto1se":   "Blastoise",
		"Blastoíse":   "Blastoise",
		"Venusaur":    "Venusaur",
		"P1kachu":     "Pikachu",
		"Píkachu":     "Pikachu",
		"Ra1chu":      "Raichu",
		"N1netales":   "Ninetales",
		"Alakazarn":   "Alakazam",
		"A1akazam":    "Alakazam",
		"Mewtw0":      "Mewtwo",
		"Macharnp":    "Machamp",
		"Macharno":    "Machamp",
		"Gyarad0s":    "Gyarados",
		"Gy arados":   "Gyarados", // Space in the middle
		"Dragon1te":   "Dragonite",
		"Art1cuno":    "Articuno",
		"Za pdos":     "Zapdos",
		"Snorl ax":    "Snorlax",
		"Genqar":      "Gengar",
		"Drat1ni":     "Dratini",
		"Dragon air":  "Dragonair",
		"Electabuz z": "Electabuzz",
		"E1ectabuzz":  "Electabuzz",
		"Magnern1te":  "Magnemite",
		"Magneton":    "Magneton",
		"Jig glypuff": "Jigglypuff",
		"Wiggly tuff": "Wigglytuff",
		"Butterfr ee": "Butterfree",
		"Caterp1e":    "Caterpie",
		"Po1ywag":     "Poliwag",
		"Po1iwrath":   "Poliwrath",
		"Star m1e":    "Starmie",
		"Hyp no":      "Hypno",
		"Aero dactyl": "Aerodactyl",
		"Orn astar":   "Omastar",
		"Kabu tops":   "Kabutops",
		// Dark Pokemon (Team Rocket)
		"Dark Chari zard": "Dark Charizard",
		"Dark B1astoise":  "Dark Blastoise",
		"Dark Dragon1te":  "Dark Dragonite",
		// Gym Leader Pokemon
		"Lt. Surge's": "Lt. Surge's",
		"Lt Surge's":  "Lt. Surge's",
		"Sabr1na's":   "Sabrina's",
		"Er1ka's":     "Erika's",
		"G1ovanni's":  "Giovanni's",
		"Bla1ne's":    "Blaine's",
		"B1aine's":    "Blaine's",
	}

	result := name
	for wrong, correct := range corrections {
		lowerResult := strings.ToLower(result)
		lowerWrong := strings.ToLower(wrong)
		if strings.Contains(lowerResult, lowerWrong) {
			// Case-insensitive replacement: find the position and replace
			idx := strings.Index(lowerResult, lowerWrong)
			result = result[:idx] + correct + result[idx+len(wrong):]
			break
		}
	}

	// Remove common OCR artifacts
	result = strings.TrimSpace(result)

	// Remove stray characters at the end of names
	result = regexp.MustCompile(`[^a-zA-Z']+$`).ReplaceAllString(result, "")

	return result
}

func parsePokemonOCR(result *OCRResult) {
	text := result.RawText
	upperText := strings.ToUpper(text)

	// Normalize common OCR digit misreads for number extraction
	normalizedText := normalizeOCRDigits(text)

	// Extract card number pattern: XXX/YYY (e.g., "025/185", "TG17/TG30")
	// Use normalized text to handle O/0 and l/1 confusion
	cardNumRegex := regexp.MustCompile(`(?:^|\s)(\d{1,3})\s*/\s*(\d{1,3})(?:\s|$|[^0-9])`)
	if matches := cardNumRegex.FindStringSubmatch(normalizedText); len(matches) >= 3 {
		// Remove leading zeros
		result.CardNumber = strings.TrimLeft(matches[1], "0")
		if result.CardNumber == "" {
			result.CardNumber = "0"
		}
		result.SetTotal = matches[2]
	}

	// Try TG (Trainer Gallery) format: TG17/TG30
	tgRegex := regexp.MustCompile(`TG(\d+)\s*/\s*TG(\d+)`)
	if matches := tgRegex.FindStringSubmatch(text); len(matches) >= 2 {
		result.CardNumber = "TG" + matches[1]
	}

	// Try GG (Galarian Gallery) format: GG01/GG70
	ggRegex := regexp.MustCompile(`GG(\d+)\s*/\s*GG(\d+)`)
	if matches := ggRegex.FindStringSubmatch(text); len(matches) >= 2 {
		result.CardNumber = "GG" + matches[1]
	}

	// Try SV (Shiny Vault) format: SV49/SV94, SV49/68
	// This format is used in Hidden Fates, Shining Fates, etc.
	svRegex := regexp.MustCompile(`SV(\d+)\s*/\s*(?:SV)?(\d+)`)
	if matches := svRegex.FindStringSubmatch(text); len(matches) >= 3 {
		result.CardNumber = "SV" + matches[1]
		// Don't set SetTotal for SV format as it's a subset total, not the main set
	}

	// Extract HP using two tiers of patterns:
	// Tier 1: Explicit HP patterns (with "HP" text) - most reliable
	// Tier 2: Fallback patterns (modern cards without HP text)
	explicitHPPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)HP[ ]*(\d{2,3})`), // "HP 170", "HP170"
		regexp.MustCompile(`(?i)(\d{2,3})[ ]*HP`), // "170 HP", "170HP"
		// Note: Removed [HhWw][ ]+(\d{2,3}) as it caused false positives with attack names like "Gnaw 10"
		regexp.MustCompile(`(?i)4P[ ]*(\d{2,3})`), // "4P 60" (OCR error for HP)
	}

	fallbackHPPatterns := []*regexp.Regexp{
		regexp.MustCompile(`[A-Z](\d{2,3})\s*[&@©]`),        // "D170 @" pattern
		regexp.MustCompile(`[~.,]?(\d{3})\s*[&@©®)>]`),      // Modern card: "~310@", "220©", ".330)"
		regexp.MustCompile(`(?i)VMAX[^0-9]*(\d{3})`),        // VMAX cards: number near VMAX text
		regexp.MustCompile(`(?i)ex[^0-9]*(\d{2,3})\s*[©®]`), // ex cards: "ex...220©"
	}

	// First try explicit HP patterns - collect all and pick most common/highest
	hpCounts := make(map[string]int)
	for _, hpRegex := range explicitHPPatterns {
		allMatches := hpRegex.FindAllStringSubmatch(text, -1)
		for _, matches := range allMatches {
			if len(matches) >= 2 {
				hp := matches[1]
				if hpVal := parseInt(hp); hpVal >= 10 && hpVal <= 400 {
					hpCounts[hp]++
				}
			}
		}
	}

	// Pick best from explicit patterns: prefer higher frequency, then higher value
	if len(hpCounts) > 0 {
		var bestHP string
		var bestCount int
		var bestVal int
		for hp, count := range hpCounts {
			val := parseInt(hp)
			if count > bestCount || (count == bestCount && val > bestVal) {
				bestHP = hp
				bestCount = count
				bestVal = val
			}
		}
		result.HP = bestHP
	}

	// If no explicit HP found, try fallback patterns (for modern cards)
	if result.HP == "" {
		for _, hpRegex := range fallbackHPPatterns {
			if matches := hpRegex.FindStringSubmatch(text); len(matches) >= 2 {
				hp := matches[1]
				if hpVal := parseInt(hp); hpVal >= 10 && hpVal <= 400 {
					result.HP = hp
					break
				}
			}
		}
	}

	// Extract set code patterns with full set code (e.g., SWSH4, SV1, XY12)
	// Only match codes that have numbers to avoid false positives with PTCGO codes
	// Skip SV pattern if we already have an SV card number (Shiny Vault cards use SV## format)
	var setCodeRegex *regexp.Regexp
	if strings.HasPrefix(result.CardNumber, "SV") {
		// Card has Shiny Vault number - don't match SV as set code
		setCodeRegex = regexp.MustCompile(`\b(SWSH\d{1,2}|XY\d{1,2}|SM\d{1,2}(?:PT5)?|BW\d{1,2}|DP\d{1,2}|EX\d{1,2}|PGO|CEL25|PR-SW|PR-SV)\b`)
	} else {
		setCodeRegex = regexp.MustCompile(`\b(SWSH\d{1,2}|SV\d{1,2}(?:PT5)?|XY\d{1,2}|SM\d{1,2}(?:PT5)?|BW\d{1,2}|DP\d{1,2}|EX\d{1,2}|PGO|CEL25|PR-SW|PR-SV)\b`)
	}
	if matches := setCodeRegex.FindStringSubmatch(upperText); len(matches) >= 1 {
		result.SetCode = strings.ToLower(matches[0])
		result.MatchReason = "set_code"
	}

	// Detect WotC era (Wizards of the Coast - Base Set through Expedition)
	// Run this early so set detection can use the hint
	detectWotCEra(result, upperText)

	// Try to detect set from set name if no set code found
	if result.SetCode == "" {
		detectSetFromName(result, upperText)
	}

	// Try to detect set from PTCGO code (2-letter codes like BS, JU, FO)
	// PTCGO codes are specific identifiers so check these first.
	if result.SetCode == "" {
		detectSetFromPTCGO(result, upperText)
	}

	// Try to detect set from card number total if still no set code.
	// This is a fallback when PTCGO code isn't detected.
	if result.SetCode == "" {
		detectSetFromTotal(result)
	}

	// Detect foil/holo indicators (conservative detection)
	detectFoilIndicators(result, upperText)

	// Detect first edition (Pokemon Base Set era)
	detectFirstEdition(result, upperText)

	// Detect rarity (before card name extraction so we can skip rarity lines)
	detectPokemonRarity(result, upperText)

	// Detect condition hints (grading labels, damage indicators)
	detectConditionHints(result, upperText)

	// Extract card name - usually first substantial line or after HP
	result.CardName = extractPokemonCardName(result.AllLines, result.Rarity)

	// Detect language from OCR text (Japanese, German, French, Italian, or English)
	result.DetectedLanguage = detectLanguage(text)
}

// detectFoilIndicators checks for foil/holographic card indicators
// Uses conservative detection: only explicit foil text triggers IsFoil=true
// Card types (V, VMAX, etc.) do NOT trigger foil as they come in both foil and non-foil
func detectFoilIndicators(result *OCRResult, upperText string) {
	// High confidence patterns (0.9) - these ARE foil, auto-set IsFoil
	highConfidencePatterns := map[string]string{
		"HOLOFOIL":     "Holofoil text detected",
		"REVERSE HOLO": "Reverse holo text detected",
		"HOLO RARE":    "Holo rare text detected",
	}

	// Check HOLO separately to avoid matching other HOLO patterns twice
	if strings.Contains(upperText, "HOLO") &&
		!strings.Contains(upperText, "HOLOFOIL") &&
		!strings.Contains(upperText, "HOLO RARE") &&
		!strings.Contains(upperText, "REVERSE HOLO") {
		result.IsFoil = true
		result.FoilConfidence = 0.9
		result.FoilIndicators = append(result.FoilIndicators, "Holographic text detected")
	}

	for pattern, hint := range highConfidencePatterns {
		if strings.Contains(upperText, pattern) {
			result.IsFoil = true
			result.FoilConfidence = 0.9
			result.FoilIndicators = append(result.FoilIndicators, hint)
		}
	}

	// Also check for standalone "FOIL" text
	foilRegex := regexp.MustCompile(`\bFOIL\b`)
	if foilRegex.MatchString(upperText) {
		result.IsFoil = true
		result.FoilConfidence = 0.9
		result.FoilIndicators = append(result.FoilIndicators, "Foil text detected")
	}

	// Medium confidence patterns (0.6) - often foil but NOT auto-set
	// These add to FoilConfidence but don't set IsFoil automatically
	mediumConfidencePatterns := map[string]string{
		"RAINBOW":       "Rainbow rare indicator",
		"GOLD":          "Gold card indicator",
		"SECRET":        "Secret rare indicator",
		"FULL ART":      "Full art card",
		"SPECIAL ART":   "Special art rare",
		"ILLUSTRATION":  "Special illustration rare",
		"ALT ART":       "Alternate art card",
		"ALTERNATE ART": "Alternate art card",
		"SHINY":         "Shiny variant text",
	}

	for pattern, hint := range mediumConfidencePatterns {
		if strings.Contains(upperText, pattern) {
			// Only update confidence if not already high
			if result.FoilConfidence < 0.6 {
				result.FoilConfidence = 0.6
			}
			result.FoilIndicators = append(result.FoilIndicators, hint)
			// Note: NOT setting IsFoil = true for medium confidence
		}
	}

	// NOTE: Card types (V, VMAX, VSTAR, GX, EX, MEGA, PRIME) are intentionally
	// NOT checked for foil detection. These card types come in both foil and
	// non-foil variants, so they should not trigger automatic foil detection.
}

// detectPokemonRarity detects card rarity from text
func detectPokemonRarity(result *OCRResult, upperText string) {
	// Rarity patterns ordered from longest/most specific to shortest
	// This ensures we match "SPECIAL ART RARE" before just "RARE"
	rarityPatterns := []struct {
		pattern string
		rarity  string
	}{
		{"ILLUSTRATION RARE", "Illustration Rare"},
		{"SPECIAL ART RARE", "Special Art Rare"},
		{"SECRET RARE", "Secret Rare"},
		{"DOUBLE RARE", "Double Rare"},
		{"HYPER RARE", "Hyper Rare"},
		{"ULTRA RARE", "Ultra Rare"},
		{"RARE HOLO", "Rare Holo"},
		{"UNCOMMON", "Uncommon"},
		{"COMMON", "Common"},
		{"PROMO", "Promo"},
		{"RARE", "Rare"}, // Must be last among "RARE" variants
	}

	for _, rp := range rarityPatterns {
		if strings.Contains(upperText, rp.pattern) {
			result.Rarity = rp.rarity
			return
		}
	}

	// Check for rarity symbols (circle, diamond, star)
	// These may appear as specific characters in OCR
	if strings.ContainsAny(upperText, "★☆●◆◇") {
		if strings.Contains(upperText, "★") || strings.Contains(upperText, "☆") {
			result.Rarity = "Rare"
		} else if strings.Contains(upperText, "◆") || strings.Contains(upperText, "◇") {
			result.Rarity = "Uncommon"
		} else if strings.Contains(upperText, "●") {
			result.Rarity = "Common"
		}
	}
}

func extractPokemonCardName(lines []string, detectedRarity string) string {
	// Patterns that should only skip if they are an EXACT match (word boundaries)
	// These are single words that can appear as part of valid card names
	// e.g., "Energy Switch" should NOT be skipped even though it contains "energy"
	exactSkipPatterns := []string{
		"basic", "stage", "pokemon", "trainer", "energy",
		"attack", "weakness", "resistance", "retreat", "rule",
		"prize", "discard", "damage", "opponent",
		// English card type words (appear on Japanese cards as the only English text)
		"supporter", // Trainer subtype
		"item",      // Trainer subtype
		"stadium",   // Trainer subtype
		"tool",      // Pokemon Tool
		// Japanese trainer card type indicators
		"サポート",     // Supporter
		"グッズ",      // Item
		"スタジアム",    // Stadium
		"ポケモンのどうぐ", // Pokemon Tool
		"たねポケモン",   // Basic Pokemon
		"進化ポケモン",   // Evolution Pokemon
		"特性",       // Ability
		"ワザ",       // Attack
		// Common Japanese card text
		"このカード", // This card
		"自分の",   // Your
		"相手の",   // Opponent's
		"山札",    // Deck
		"手札",    // Hand
		"トラッシュ", // Trash/Discard
	}

	// Patterns that should skip if the line CONTAINS them (multi-word phrases)
	// These are phrases that indicate the line is descriptive text, not a card name
	containsSkipPatterns := []string{
		"once during", "when you", "your turn",
		"evolves from", "knocked out",
		// Header / type lines that often appear when OCR misses the name line
		"trainer -", // e.g. "TRAINER - SUPPORTER"
		"basic pokemon",
		"stage 1",
		"stage 2",
		"illus", "©", "nintendo",
	}

	// Rarity-related words that might appear as separate lines
	// Use exact match since "Gold" alone should be skipped but we might have
	// a card with "Gold" in its name (though unlikely)
	raritySkipPatterns := []string{
		"holo", "rare", "uncommon", "common", "promo",
		"gold", "rainbow", "secret", "full art", "reverse",
		"illustration", "special art", "ultra", "hyper", "double",
	}

	// Helper to check if a line/name should be skipped
	shouldSkipName := func(name string) bool {
		trimmed := strings.TrimSpace(name)
		trimmed = strings.Trim(trimmed, ".,:;|!¡?¿()[]{}<>\"'`~")
		lower := strings.ToLower(trimmed)
		upper := strings.ToUpper(trimmed)

		// Skip empty or very short strings
		if len(trimmed) < 2 {
			return true
		}

		// Skip strings starting with special characters (OCR noise like "@N町")
		if len(trimmed) > 0 {
			firstRune := []rune(trimmed)[0]
			if !unicode.IsLetter(firstRune) && !unicode.IsDigit(firstRune) {
				return true
			}
		}

		// Skip lines that look like Pokemon set codes (common on Japanese cards).
		// Avoid treating "SV2A" / "SWSH4" etc. as a card name when OCR fails.
		if regexp.MustCompile(`^[A-Z0-9]{3,6}$`).MatchString(upper) {
			hasDigit := strings.IndexFunc(upper, func(r rune) bool { return r >= '0' && r <= '9' }) >= 0
			if hasDigit {
				return true
			}
			switch upper {
			case "SV", "SWSH", "SM", "XY", "BW", "DP", "HS", "HGSS", "EX", "POP", "PL":
				return true
			}
		}

		// Check exact match patterns first (single words that shouldn't be card names)
		for _, pattern := range exactSkipPatterns {
			if lower == pattern {
				return true
			}
		}

		// Check contains patterns (multi-word phrases that indicate non-name text)
		for _, pattern := range containsSkipPatterns {
			if strings.Contains(lower, pattern) {
				return true
			}
		}

		// Check rarity patterns (exact match for short single words)
		for _, pattern := range raritySkipPatterns {
			if lower == pattern {
				return true
			}
		}

		return false
	}

	// Gym Leader and Team Rocket prefixes for WotC era cards
	gymLeaderPrefixes := []string{
		"lt. surge's", "lt surge's", "sabrina's", "brock's", "misty's",
		"erika's", "koga's", "blaine's", "giovanni's",
		"dark", "light", "rocket's", "team rocket's",
	}

	// Use pre-sorted Pokemon names (sorted by length, longest first at package init)
	// This prevents partial matches (e.g., "mewtwo" is checked before "mew")

	// First pass: look for known Pokemon names in the text, prioritizing earlier lines
	// (the card name is typically the first line, while "Evolves from X" comes later)
	for _, line := range lines {
		// Normalize the line for OCR errors (0->o, 1->i, etc.) for name matching
		normalizedLine := normalizeLineForNameMatch(line)
		lower := strings.ToLower(normalizedLine)

		// Skip lines that should not be card names
		// Use original line (not normalized) for skip check because normalization
		// removes spaces which breaks multi-word phrase detection like "evolves from"
		if shouldSkipName(line) {
			continue
		}

		// Check if this line contains a known Pokemon name (sorted by length, longest first)
		for _, pokeName := range getPokemonNames() {
			if strings.Contains(lower, pokeName) {
				// Check for gym leader or team rocket prefixes
				for _, prefix := range gymLeaderPrefixes {
					if strings.Contains(lower, prefix+" "+pokeName) || strings.Contains(lower, prefix+pokeName) {
						// Found a prefixed name like "Dark Charizard" or "Lt. Surge's Electabuzz"
						name := cleanPokemonName(line, prefix+" "+pokeName)
						if name != "" {
							return name
						}
					}
				}
				// Clean up the line to extract just the name part
				name := cleanPokemonName(line, pokeName)
				if name != "" {
					return name
				}
			}
		}
	}

	// Fallback 1: Search all text for any known Pokemon name
	// This helps when OCR doesn't capture the card name line but mentions the Pokemon elsewhere
	allText := strings.Join(lines, " ")
	// Use two versions of the text:
	// - lowerAllText: preserves spaces for "evolves from" pattern matching
	// - normalizedAllText: aggressive normalization for finding names with OCR errors
	lowerAllText := strings.ToLower(allText)
	normalizedAllText := strings.ToLower(normalizeLineForNameMatch(allText))
	for _, pokeName := range getPokemonNames() {
		if strings.Contains(normalizedAllText, pokeName) {
			// Skip if this name only appears in "evolves from" context
			// e.g., "Evolves from Charmeleon" should not match for a Charizard card
			// Use lowerAllText (not normalized) for this check to preserve spaces
			evolvesPattern := regexp.MustCompile(`evolves\s+from\s+` + regexp.QuoteMeta(pokeName))
			if evolvesPattern.MatchString(lowerAllText) {
				// Check if the name appears elsewhere (not just in evolves from)
				// Remove the "evolves from X" text and check if name still exists
				cleanedText := evolvesPattern.ReplaceAllString(lowerAllText, "")
				if !strings.Contains(cleanedText, pokeName) {
					continue // Name only appears in evolves from context, skip it
				}
			}
			// Found a Pokemon name - capitalize it properly
			result := strings.ToUpper(string(pokeName[0])) + pokeName[1:]
			return result
		}
	}

	// Fallback 2: Try fuzzy matching on early lines (handles OCR errors in names)
	for i, line := range lines {
		// Only check the first few lines (card name is near the top)
		if i >= 5 {
			break
		}

		// Skip lines that describe evolution
		// Skip lines that should not be card names
		if shouldSkipName(line) {
			continue
		}

		// For Japanese cards, extract English words from mixed text
		// For English cards, extract all words
		var words []string
		if containsJapaneseCharacters(line) {
			words = extractEnglishWordsFromLine(line)
		} else {
			words = regexp.MustCompile(`[A-Za-z]{3,}`).FindAllString(line, -1)
		}

		for _, word := range words {
			if matchedName, found := fuzzyMatchPokemonName(word); found {
				// Capitalize first letter
				result := strings.ToUpper(string(matchedName[0])) + matchedName[1:]
				return result
			}
		}
	}

	// Fallback 3: Try to find a reasonable first line that could be a name
	for _, line := range lines {
		// Skip short lines
		if len(line) < 3 {
			continue
		}

		// Skip lines that are just numbers
		if regexp.MustCompile(`^[\d\s/]+$`).MatchString(line) {
			continue
		}

		// Skip lines that should not be card names
		if shouldSkipName(line) {
			continue
		}

		// Skip lines with too many special symbols (not including Japanese/CJK characters)
		// Only count actual symbols like copyright, trademark, card symbols, etc.
		symbolCount := 0
		for _, r := range line {
			// Count only actual symbols, not letters (including Japanese)
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsSpace(r) &&
				r != '\'' && r != '-' && r != '/' {
				symbolCount++
			}
		}
		if symbolCount > 5 {
			continue
		}

		// This might be the card name
		// Clean it up - remove HP values, etc.
		name := cleanPokemonName(line, "")
		if len(name) >= 3 {
			// Check if the cleaned name should be skipped
			if shouldSkipName(name) {
				continue
			}

			// Try fuzzy match on cleaned name
			if matchedName, found := fuzzyMatchPokemonName(name); found {
				result := strings.ToUpper(string(matchedName[0])) + matchedName[1:]
				return result
			}

			// For short all-caps strings without spaces (likely OCR garbage like "TQG"),
			// skip if fuzzy match fails. Real card names usually have spaces, punctuation,
			// or are longer.
			if looksLikeOCRGarbage(name) {
				continue
			}

			return name
		}
	}

	// Fallback 4: return first line with letters (but not skip words)
	for _, line := range lines {
		if regexp.MustCompile(`[a-zA-Z]{3,}`).MatchString(line) {
			name := cleanPokemonName(line, "")
			if name == "" {
				continue
			}

			// Check if the cleaned name should be skipped
			if shouldSkipName(name) {
				continue
			}

			// Try fuzzy match as last resort
			if matchedName, found := fuzzyMatchPokemonName(name); found {
				result := strings.ToUpper(string(matchedName[0])) + matchedName[1:]
				return result
			}

			// For short all-caps strings without spaces (likely OCR garbage),
			// skip if fuzzy match fails.
			if looksLikeOCRGarbage(name) {
				continue
			}

			return name
		}
	}

	return ""
}

// cleanPokemonName cleans up OCR noise from a Pokemon name
func cleanPokemonName(line, knownName string) string {
	// Normalize full-width ASCII characters (common in Japanese cards)
	// e.g., Ｎ -> N, Ｖ -> V
	name := normalizeFullWidthASCII(line)

	// Remove HP values
	name = regexp.MustCompile(`\s*HP\s*\d+`).ReplaceAllString(name, "")
	name = regexp.MustCompile(`\s*\d{2,3}\s*HP`).ReplaceAllString(name, "")

	// If the line contains Japanese characters, try to extract English text
	// since our Pokemon database is English-only
	if containsJapaneseCharacters(name) {
		englishWords := extractEnglishWordsFromLine(name)
		if len(englishWords) > 0 {
			// Join the English words and use that as the candidate name
			name = strings.Join(englishWords, " ")
		} else {
			// No English text found in Japanese line - can't match to English database
			// Return empty to signal we need to rely on card number matching
			return ""
		}
	}

	// Remove common OCR artifacts at the start (numbers, symbols) but keep letters
	name = regexp.MustCompile(`^[^a-zA-Z]*`).ReplaceAllString(name, "")

	// If we know the Pokemon name, try to extract it with its suffixes (V, VMAX, ex, etc.)
	if knownName != "" {
		// Build a pattern to match the known name with optional suffix
		// Note: Order matters - check longer suffixes first (VMAX before V)
		pattern := regexp.MustCompile(`(?i)(` + regexp.QuoteMeta(knownName) + `)\s*(VMAX|VSTAR|MEGA|PRIME|GX|EX|ex|V)?`)
		if match := pattern.FindStringSubmatch(name); len(match) >= 2 {
			result := match[1]
			if len(match) >= 3 && match[2] != "" {
				suffix := match[2]
				// Preserve original case for EX/ex (EX era vs modern ex cards)
				// Modern Scarlet & Violet uses lowercase "ex", older EX era uses uppercase "EX"
				if strings.ToLower(suffix) == "ex" {
					// Keep the original case from the OCR text
					result += " " + suffix
				} else {
					result += " " + strings.ToUpper(suffix)
				}
			}
			// Capitalize first letter of Pokemon name
			if len(result) > 0 {
				result = strings.ToUpper(string(result[0])) + result[1:]
			}
			return result
		}
	}

	// Clean up remaining artifacts - for English text only
	// Allow '.' for names like "Mr. Mime".
	name = regexp.MustCompile(`[^a-zA-Z0-9\s'.-]`).ReplaceAllString(name, "")
	name = regexp.MustCompile(`\s+`).ReplaceAllString(name, " ")
	name = strings.TrimSpace(name)

	// Apply OCR corrections for common Pokemon name misspellings
	name = normalizeCardName(name)

	return name
}

func parseMTGOCR(result *OCRResult) {
	text := result.RawText
	upperText := strings.ToUpper(text)

	// MTG collector number pattern: e.g., "123/456"
	// Must be on its own line or have clear context - avoid matching power/toughness like "4/5"
	// Look for larger numbers (collector numbers are typically 3+ digits in at least one part)
	// or look for the pattern on its own line
	collectorRegex := regexp.MustCompile(`(?:^|\n)\s*(\d{1,4})\s*/\s*(\d{2,4})\s*(?:\n|$)`)
	if matches := collectorRegex.FindStringSubmatch(text); len(matches) >= 3 {
		result.CardNumber = matches[1]
		result.SetTotal = matches[2]
	} else {
		// Fallback: look for collector number patterns where total is > 50 (unlikely to be P/T)
		fallbackRegex := regexp.MustCompile(`(\d{1,4})\s*/\s*(\d{2,4})`)
		for _, match := range fallbackRegex.FindAllStringSubmatch(text, -1) {
			if len(match) >= 3 {
				// If the total is > 50, it's likely a collector number
				// Power/toughness rarely exceeds 20
				total := match[2]
				if len(total) >= 2 {
					result.CardNumber = match[1]
					result.SetTotal = total
					break
				}
			}
		}
	}

	// MTG set codes are 3-4 uppercase letters, typically on their own line
	// Common false positives to skip
	falsePositives := map[string]bool{
		// Common English words
		"THE": true, "AND": true, "FOR": true, "YOU": true, "ARE": true,
		"WAS": true, "HAS": true, "HAD": true, "NOT": true, "ALL": true,
		"CAN": true, "HER": true, "HIS": true, "BUT": true, "ITS": true,
		"OUT": true, "GET": true, "HIM": true, "PUT": true, "END": true,
		"ADD": true, "TAP": true, "MAY": true, "TWO": true, "ONE": false, // ONE is a real set!
		"USE": true, "ANY": true, "OWN": true, "WAY": true, "NEW": true,
		// Card type words
		"FOIL": true, "BOLT": true, "RING": true, "VEIL": true, "SIX": true,
		"SOL": true, "ART": true, "DEAL": true, "CARD": true, "DRAW": true,
		"EACH": true, "FROM": true, "INTO": true, "ONTO": true, "THAT": true,
		"THIS": true, "WITH": true, "YOUR": true,
		// Foil indicators (should not be treated as set codes)
		"ETCHED": true, "SURGE": true,
		// Common words that appear in card text
		"THEN": true, "WHEN": true, "LIFE": true, "LOSE": true, "GAIN": true,
		"DIES": true, "TURN": true, "COPY": true, "COST": true, "MANA": true,
		"STEP": true, "NEXT": true, "MILL": true, "CAST": true, "PLAY": true,
		// Common artist name fragments (first/last names that look like set codes)
		"RAHN": true, "JOHN": true, "MARK": true, "ADAM": true, "CARL": true,
		"ERIC": true, "GREG": true, "IVAN": true, "JACK": true, "KARL": true,
		"LARS": true, "MIKE": true, "NICK": true, "NOAH": true, "PAUL": true,
		"RYAN": true, "SEAN": true, "TODD": true, "TONY": true, "ZACK": true,
		// Common illustrator text
		"ILLUS": true, "ILLU": true,
	}

	// Look for set code - prefer codes that appear on their own line
	// MTG set codes can start with numbers (like 2XM, 2LU) or letters
	setCodeRegex := regexp.MustCompile(`\b([A-Z0-9][A-Z0-9]{2,3})\b`)
	var candidates []string
	for _, match := range setCodeRegex.FindAllStringSubmatch(upperText, -1) {
		code := match[1]
		// Skip pure numbers
		if regexp.MustCompile(`^\d+$`).MatchString(code) {
			continue
		}
		if !falsePositives[code] {
			candidates = append(candidates, code)
		}
	}

	// If we have candidates, prefer ones that look like set codes (3 letters, all uppercase)
	// and appear later in the text (set codes are typically at the bottom of cards)
	if len(candidates) > 0 {
		// Take the last candidate that isn't a common word
		for i := len(candidates) - 1; i >= 0; i-- {
			code := candidates[i]
			// Valid MTG set codes are 3-4 characters
			if len(code) >= 3 && len(code) <= 4 {
				result.SetCode = code
				result.MatchReason = "set_code"
				break
			}
		}
	}

	// Detect foil indicators for MTG
	mtgFoilPatterns := []string{"FOIL", "ETCHED", "SURGE", "SHOWCASE", "BORDERLESS", "EXTENDED ART"}
	for _, pattern := range mtgFoilPatterns {
		if strings.Contains(upperText, pattern) {
			result.IsFoil = true
			result.FoilIndicators = append(result.FoilIndicators, pattern+" card variant")
		}
	}

	// Detect condition hints
	detectConditionHints(result, upperText)

	// Extract copyright year: "© 2022", "©2022", "TM & © 2022", "2022 Wizards"
	copyrightRegex := regexp.MustCompile(`©\s*(\d{4})`)
	if matches := copyrightRegex.FindStringSubmatch(text); len(matches) >= 2 {
		result.CopyrightYear = matches[1]
	}

	// Card name is typically the first line
	result.CardName = extractMTGCardName(result.AllLines)

	// Detect language from OCR text (Japanese, German, French, Italian, or English)
	result.DetectedLanguage = detectLanguage(text)
}

func extractMTGCardName(lines []string) string {
	skipPatterns := []string{
		"creature", "instant", "sorcery", "enchantment", "artifact",
		"legendary", "flying", "trample", "when", "©", "wizards",
	}

	for _, line := range lines {
		lower := strings.ToLower(line)

		if len(line) < 2 {
			continue
		}

		// Skip type lines and abilities
		skip := false
		for _, pattern := range skipPatterns {
			if strings.Contains(lower, pattern) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		// Skip mana cost lines (contain {W}, {U}, etc. or just numbers)
		if regexp.MustCompile(`\{[WUBRG]\}|^[\d\s]+$`).MatchString(line) {
			continue
		}

		return strings.TrimSpace(line)
	}

	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return ""
}

// detectConditionHints looks for indicators of card condition in the text
// Note: OCR from card images rarely detects condition directly, but this
// can pick up on certain visual artifacts that OCR might capture, or
// condition labels if scanning cards with grading labels
func detectConditionHints(result *OCRResult, upperText string) {
	// Grading service indicators
	gradingPatterns := map[string]string{
		"PSA":       "PSA graded card",
		"BGS":       "Beckett graded card",
		"CGC":       "CGC graded card",
		"SGC":       "SGC graded card",
		"MINT":      "Mint condition indicator",
		"NEAR MINT": "Near Mint condition",
		"NM":        "Near Mint abbreviation",
		"GEM MINT":  "Gem Mint condition",
		"PRISTINE":  "Pristine condition",
	}

	for pattern, hint := range gradingPatterns {
		if strings.Contains(upperText, pattern) {
			result.ConditionHints = append(result.ConditionHints, hint)
		}
	}

	// Look for grade numbers (e.g., "PSA 10", "BGS 9.5")
	gradeRegex := regexp.MustCompile(`(PSA|BGS|CGC|SGC)\s*(\d+\.?\d?)`)
	if matches := gradeRegex.FindStringSubmatch(upperText); len(matches) >= 3 {
		result.ConditionHints = append(result.ConditionHints,
			matches[1]+" grade: "+matches[2])
	}

	// Condition issues that might be visible in OCR
	issuePatterns := map[string]string{
		"DAMAGED":   "Damaged condition",
		"PLAYED":    "Played condition",
		"CREASED":   "Card has crease",
		"SCRATCHED": "Card has scratches",
		"WORN":      "Card shows wear",
	}

	for pattern, hint := range issuePatterns {
		if strings.Contains(upperText, pattern) {
			result.ConditionHints = append(result.ConditionHints, hint)
		}
	}
}

// detectWotCEra checks for indicators that this is a Wizards of the Coast era card
// WotC era cards (1999-2003) have distinctive copyright text and formatting
func detectWotCEra(result *OCRResult, upperText string) {
	// Look for Wizards of the Coast copyright patterns
	// Base Set through Expedition era cards have "Wizards" copyright
	wotcPatterns := []string{
		"WIZARDS OF THE COAST",
		"WIZARDS",
		"WOTC",
		// Common OCR errors for Wizards
		"WIZAROS",
		"W1ZARDS",
		"WlZARDS",
		"WTZARDS",
		"WI2ARDS",
		"WIZARD5",
		"WIZARO5",
		"W!ZARDS",
		"WIZBROS",
		"WIZAPDS",
		"WIZARDS.",
		// Nintendo copyright years (with various OCR errors for © symbol)
		"©1995",
		"©1996",
		"©1997",
		"©1998",
		"©1999",
		"©2000",
		"©2001",
		"©2002",
		"©2003",
		// OCR variations - C for ©
		"C1995", "C1996", "C1997", "C1998", "C1999",
		"C2000", "C2001", "C2002", "C2003",
		// OCR variations - 0 for ©
		"01995", "01996", "01997", "01998", "01999",
		"02000", "02001", "02002", "02003",
		// OCR variations - @ for ©
		"@1995", "@1996", "@1997", "@1998", "@1999",
		"@2000", "@2001", "@2002", "@2003",
		// OCR variations - ( for ©
		"(1995", "(1996", "(1997", "(1998", "(1999",
		"(2000", "(2001", "(2002", "(2003",
		// OCR variations - space before year
		"© 1995", "© 1996", "© 1997", "© 1998", "© 1999",
		"© 2000", "© 2001", "© 2002", "© 2003",
		// Additional patterns for WotC era detection
		"NINTENDO", // All WotC cards have Nintendo copyright
		"CREATURES",
		"GAMEFREAK",
		"GAME FREAK",
	}

	for _, pattern := range wotcPatterns {
		if strings.Contains(upperText, pattern) {
			result.IsWotCEra = true
			return
		}
	}

	// Use regex to catch more year patterns with OCR errors
	// Matches patterns like: c1999, C 1999, @1999, etc.
	yearRegex := regexp.MustCompile(`[©C@O0(\[][\ ]?(199[5-9]|200[0-3])`)
	if yearRegex.MatchString(upperText) {
		result.IsWotCEra = true
		return
	}

	// Also check for WotC era based on low set totals without modern set codes
	// Base era sets have specific totals: 102, 64, 62, 82, 83, 110, 132, 111, 75, 66, 113
	// If we see these totals AND no modern set code, it's likely WotC era
	wotcSetTotals := map[string]bool{
		"102": true, // Base Set
		"64":  true, // Jungle
		"62":  true, // Fossil
		"82":  true, // Team Rocket
		"83":  true, // Team Rocket (with Dark Raichu)
		"110": true, // Legendary Collection
		"132": true, // Gym Heroes/Challenge
		"111": true, // Neo Genesis
		"75":  true, // Neo Discovery
		"66":  true, // Neo Revelation
		"113": true, // Neo Destiny
		"130": true, // Base Set 2
	}

	if result.SetTotal != "" && result.SetCode == "" && wotcSetTotals[result.SetTotal] {
		// Check if there's no modern indicator
		hasModernIndicator := strings.Contains(upperText, "SWSH") ||
			strings.Contains(upperText, "SV") ||
			strings.Contains(upperText, "SM") ||
			strings.Contains(upperText, "XY") ||
			strings.Contains(upperText, "BW") ||
			strings.Contains(upperText, "HGSS") ||
			strings.Contains(upperText, "DP")

		if !hasModernIndicator {
			result.IsWotCEra = true
		}
	}
}

// detectFirstEdition checks for first edition indicators (Pokemon cards only)
// First edition cards from Base Set era have a "1ST EDITION" stamp
func detectFirstEdition(result *OCRResult, upperText string) {
	// Check patterns from most specific to least specific to avoid duplicate matches
	// "1ST EDITION" contains "1ST ED", so check longer patterns first
	if strings.Contains(upperText, "1ST EDITION") {
		result.IsFirstEdition = true
		result.FirstEdIndicators = append(result.FirstEdIndicators, "1ST EDITION detected")
	} else if strings.Contains(upperText, "FIRST EDITION") {
		result.IsFirstEdition = true
		result.FirstEdIndicators = append(result.FirstEdIndicators, "FIRST EDITION detected")
	} else if strings.Contains(upperText, "1ST ED") {
		// Only match "1ST ED" if we didn't already match "1ST EDITION"
		result.IsFirstEdition = true
		result.FirstEdIndicators = append(result.FirstEdIndicators, "1ST ED detected")
	}

	// Check for "SHADOWLESS" - these are related to first edition era
	// but not exactly first edition (they're after 1st ed but before unlimited)
	// Add as indicator but don't auto-set first edition
	if strings.Contains(upperText, "SHADOWLESS") {
		result.FirstEdIndicators = append(result.FirstEdIndicators, "Shadowless variant (verify if 1st edition)")
	}
}

func calculateConfidence(result *OCRResult) float64 {
	score := 0.0

	if result.CardName != "" {
		score += 0.4
	}
	if result.CardNumber != "" {
		score += 0.3
	}
	if result.SetTotal != "" || result.SetCode != "" {
		score += 0.2
	}
	if result.HP != "" {
		score += 0.1
	}

	return score
}

// detectSetFromName tries to detect set code from set name in OCR text
func detectSetFromName(result *OCRResult, upperText string) {
	// Check for set names in the text (longest matches first for accuracy)
	// Sort by length descending to match longer names first
	type setMatch struct {
		name string
		code string
	}
	matches := []setMatch{}

	// Short set names that need word boundary checking to avoid false matches
	// e.g., "BASE" should not match "BASE DAMAGE" from attack text
	shortNames := map[string]bool{
		"BASE": true, "FOSSIL": true, "JUNGLE": true,
	}

	for name, code := range pokemonSetNameToCode {
		if strings.Contains(upperText, name) {
			// For short names, require word boundaries (space/punctuation/start/end)
			if shortNames[name] {
				// Use regex to check for word boundary
				pattern := regexp.MustCompile(`(?:^|[\s,.:;!?])` + regexp.QuoteMeta(name) + `(?:[\s,.:;!?]|$)`)
				if !pattern.MatchString(upperText) {
					continue // Skip false match
				}
			}
			matches = append(matches, setMatch{name: name, code: code})
		}
	}

	// Find the longest match (most specific)
	if len(matches) > 0 {
		longest := matches[0]
		for _, m := range matches[1:] {
			if len(m.name) > len(longest.name) {
				longest = m
			}
		}
		result.SetCode = longest.code
		result.SetName = longest.name
		result.MatchReason = "set_name"
	}
}

// detectSetFromTotal tries to infer set code from card set total (e.g., /185 -> Vivid Voltage)
// Sets CandidateSets when there are multiple possible sets, and MatchReason accordingly
func detectSetFromTotal(result *OCRResult) {
	if result.SetTotal == "" || result.SetCode != "" {
		return
	}

	// Normalize the set total (remove leading zeros for matching)
	normalizedTotal := strings.TrimLeft(result.SetTotal, "0")
	if normalizedTotal == "" {
		normalizedTotal = "0"
	}

	var possibleSets []string

	// Try with the original (padded) version first
	if sets, ok := pokemonSetTotalToCode[result.SetTotal]; ok {
		possibleSets = sets
	} else if sets, ok := pokemonSetTotalToCode[normalizedTotal]; ok {
		// Try with normalized total (without leading zeros)
		possibleSets = sets
	}

	if len(possibleSets) == 0 {
		return
	}

	// Set the best candidate as SetCode
	result.SetCode = selectBestSetFromTotal(possibleSets, result.IsWotCEra)

	// If multiple possible sets, populate CandidateSets for the frontend
	if len(possibleSets) > 1 {
		result.CandidateSets = possibleSets
		result.MatchReason = "inferred_from_total"
	} else {
		result.MatchReason = "unique_set_total"
	}
}

// selectBestSetFromTotal picks the most likely set when multiple sets share the same total
// Uses a priority system that considers set era and popularity
// If isWotCEra is true, prioritizes classic WotC sets (Base through Neo)
func selectBestSetFromTotal(possibleSets []string, isWotCEra bool) string {
	if len(possibleSets) == 1 {
		return possibleSets[0]
	}

	// WotC era sets (Base through Neo/e-Card)
	baseEraSets := map[string]bool{
		"base1": true, "base2": true, "base3": true, "base4": true, "base5": true, "base6": true,
		"gym1": true, "gym2": true,
		"neo1": true, "neo2": true, "neo3": true, "neo4": true,
		"ecard1": true, "ecard2": true, "ecard3": true,
	}

	// If we detected WotC era, prioritize base era sets
	if isWotCEra {
		for _, set := range possibleSets {
			if baseEraSets[set] {
				return set
			}
		}
	}

	// Default priority order for ambiguous totals:
	// 1. Newer era sets (sv*, swsh*) - more popular and commonly scanned
	// 2. Base era sets (base1-6, gym1-2, neo1-4) - classic sets still commonly scanned
	// 3. Middle era sets (hgss*, dp*, ex*, bw*, sm*, xy*)

	// First check for modern sets (Scarlet & Violet, Sword & Shield)
	for _, set := range possibleSets {
		if strings.HasPrefix(set, "sv") || strings.HasPrefix(set, "swsh") {
			return set
		}
	}

	// Then check for classic WotC era sets (Base through Neo)
	for _, set := range possibleSets {
		if baseEraSets[set] {
			return set
		}
	}

	// Otherwise return the first option
	return possibleSets[0]
}

// detectSetFromPTCGO tries to detect set code from PTCGO 2-letter codes
// These codes appear on physical cards and are used for Pokemon TCG Online/Live
func detectSetFromPTCGO(result *OCRResult, upperText string) {
	// Look for PTCGO codes: 2 letters (BS, JU, FO, TR, LC) or letter+number (G1, G2, N1-N4, B2)
	// Use word boundaries to avoid matching within words
	ptcgoRegex := regexp.MustCompile(`\b([A-Z][A-Z0-9])\b`)

	for _, match := range ptcgoRegex.FindAllStringSubmatch(upperText, -1) {
		code := match[1]
		if setCode, ok := pokemonPTCGOToCode[code]; ok {
			result.SetCode = setCode
			result.MatchReason = "ptcgo_code"
			return
		}
	}
}

// detectLanguage detects the card's language from OCR text.
// Uses multiple signals: Japanese characters, HP text variations, and common words.
//
// Detection priority:
// 1. Japanese (CJK characters) - most reliable, distinct character set
// 2. German (KP for HP) - unique HP abbreviation
// 3. French (PV for HP) - unique HP abbreviation
// 4. Italian (PS for HP) - unique HP abbreviation
// 5. English - default if no other language detected
func detectLanguage(text string) string {
	upperText := strings.ToUpper(text)

	// Check for Japanese (hiragana, katakana, or kanji)
	// This is the most reliable signal since Japanese uses distinct character sets
	if containsJapaneseCharacters(text) {
		return "Japanese"
	}

	// Check for German: "KP" is used for HP (Kraftpunkte)
	// Must be careful not to match "KP" in other contexts
	if containsGermanIndicators(upperText) {
		return "German"
	}

	// Check for French: "PV" is used for HP (Points de Vie)
	if containsFrenchIndicators(upperText) {
		return "French"
	}

	// Check for Italian: "PS" is used for HP (Punti Salute)
	if containsItalianIndicators(upperText) {
		return "Italian"
	}

	// Default to English
	return "English"
}

// containsJapaneseCharacters checks if text contains Japanese characters
// (hiragana, katakana, or CJK unified ideographs/kanji)
func containsJapaneseCharacters(text string) bool {
	for _, r := range text {
		// Hiragana range: U+3040 - U+309F
		// Katakana range: U+30A0 - U+30FF
		// CJK Unified Ideographs: U+4E00 - U+9FFF (includes kanji)
		if unicode.Is(unicode.Hiragana, r) ||
			unicode.Is(unicode.Katakana, r) ||
			unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// extractEnglishWordsFromLine extracts English words from a line that may contain
// mixed Japanese and English text. Returns words that are 2+ ASCII letters.
func extractEnglishWordsFromLine(line string) []string {
	// First normalize full-width ASCII to regular ASCII
	normalized := normalizeFullWidthASCII(line)

	// Match sequences of ASCII letters that may include punctuation commonly
	// found in Pokemon names, e.g. "Farfetch'd" or "Mr. Mime".
	// Keep an optional trailing '.' for abbreviations like "Mr.".
	wordPattern := regexp.MustCompile(`[A-Za-z]+(?:[.'-][A-Za-z]+)*\.?|[A-Za-z]`)
	matches := wordPattern.FindAllString(normalized, -1)

	var words []string
	for _, w := range matches {
		upper := strings.ToUpper(w)
		// Keep words that are at least 2 chars or single meaningful letters
		// Single letters N, V, G, X are valid Pokemon card names/suffixes
		if len(w) >= 2 {
			words = append(words, w)
			continue
		}
		if len(w) == 1 && strings.ContainsAny(upper, "NVGX") {
			// Normalize single-letter suffixes to uppercase.
			words = append(words, upper)
		}
	}
	return words
}

// normalizeFullWidthASCII converts full-width ASCII characters to half-width
// Japanese cards often use full-width letters like Ｎ, Ａ, etc.
func normalizeFullWidthASCII(text string) string {
	var result strings.Builder
	for _, r := range text {
		// Full-width ASCII range: U+FF01-U+FF5E maps to U+0021-U+007E
		if r >= 0xFF01 && r <= 0xFF5E {
			result.WriteRune(r - 0xFF01 + 0x0021)
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// containsGermanIndicators checks for German card indicators
// German cards use "KP" for HP (Kraftpunkte) and German Pokemon names
func containsGermanIndicators(upperText string) bool {
	// German HP pattern: "KP" followed by or preceded by a number
	// e.g., "120 KP", "KP 120"
	if germanKpPattern.MatchString(upperText) {
		return true
	}

	// German energy types
	germanIndicators := []string{
		"FEUER-ENERGIE",    // Fire Energy
		"WASSER-ENERGIE",   // Water Energy
		"PFLANZEN-ENERGIE", // Grass Energy
		"ELEKTRO-ENERGIE",  // Electric Energy
		"PSYCHO-ENERGIE",   // Psychic Energy
		"KAMPF-ENERGIE",    // Fighting Energy
		"FINSTERNIS",       // Darkness
		"METALL-ENERGIE",   // Metal Energy
		"RÜCKZUG",          // Retreat (Rückzugskosten = retreat cost)
	}

	for _, indicator := range germanIndicators {
		if strings.Contains(upperText, indicator) {
			return true
		}
	}

	return false
}

// containsFrenchIndicators checks for French card indicators
// French cards use "PV" for HP (Points de Vie)
func containsFrenchIndicators(upperText string) bool {
	// French HP pattern: "PV" followed by or preceded by a number
	// e.g., "120 PV", "PV 120"
	if frenchPvPattern.MatchString(upperText) {
		return true
	}

	// French energy types and common words
	frenchIndicators := []string{
		"ÉNERGIE",    // Energy (with accent)
		"ENERGIE",    // Energy (without accent, OCR may miss it)
		"FEU",        // Fire
		"EAU",        // Water
		"PLANTE",     // Grass
		"ÉLECTRIQUE", // Electric
		"PSY",        // Psychic
		"COMBAT",     // Fighting
		"OBSCURITÉ",  // Darkness
		"MÉTAL",      // Metal
		"RETRAITE",   // Retreat
	}

	for _, indicator := range frenchIndicators {
		if strings.Contains(upperText, indicator) {
			return true
		}
	}

	return false
}

// containsItalianIndicators checks for Italian card indicators
// Italian cards use "PS" for HP (Punti Salute)
func containsItalianIndicators(upperText string) bool {
	// Italian HP pattern: "PS" followed by or preceded by a number
	// e.g., "120 PS", "PS 120"
	// Note: Need to be careful as "PS" could appear in other contexts
	if italianPsPattern.MatchString(upperText) {
		return true
	}

	// Italian energy types and common words
	italianIndicators := []string{
		"ENERGIA",  // Energy
		"FUOCO",    // Fire
		"ACQUA",    // Water
		"ERBA",     // Grass
		"ELETTRO",  // Electric
		"PSICO",    // Psychic
		"LOTTA",    // Fighting
		"OSCURITÀ", // Darkness
		"METALLO",  // Metal
		"RITIRATA", // Retreat
	}

	for _, indicator := range italianIndicators {
		if strings.Contains(upperText, indicator) {
			return true
		}
	}

	return false
}
