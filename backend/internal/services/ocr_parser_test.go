package services

import (
	"strings"
	"testing"
)

// TestPokemonCardNumberExtraction tests card number parsing from OCR text
func TestPokemonCardNumberExtraction(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantCardNumber string
		wantSetTotal   string
		wantHP         string
		wantSetCode    string
		wantCardName   string
		wantIsFoil     bool
		minConfidence  float64
	}{
		{
			name: "Standard Pokemon card with leading zeros",
			input: `Charizard
HP 170
STAGE 2
025/185
SWSH4`,
			wantCardNumber: "25",
			wantSetTotal:   "185",
			wantHP:         "170",
			wantSetCode:    "swsh4",
			wantCardName:   "Charizard",
			minConfidence:  0.8,
		},
		{
			name: "Pokemon card without leading zeros",
			input: `Pikachu V
HP 190
25/185`,
			wantCardNumber: "25",
			wantSetTotal:   "185",
			wantHP:         "190",
			wantCardName:   "Pikachu V",
			wantIsFoil:     false, // Conservative: V cards come in both foil and non-foil
			minConfidence:  0.7,
		},
		{
			name: "Trainer Gallery card TG format",
			input: `Umbreon VMAX
TG17/TG30
Darkness`,
			wantCardNumber: "TG17",
			wantCardName:   "Umbreon VMAX",
			wantIsFoil:     false, // Conservative: VMAX cards come in both foil and non-foil
			minConfidence:  0.4,
		},
		{
			name: "Galarian Gallery card GG format",
			input: `Eevee
GG01/GG70
Basic Pokemon`,
			wantCardNumber: "GG01",
			wantCardName:   "Eevee",
			minConfidence:  0.4,
		},
		{
			name: "HP after value pattern",
			input: `Mewtwo
170 HP
Basic Pokemon
054/198`,
			wantCardNumber: "54",
			wantSetTotal:   "198",
			wantHP:         "170",
			wantCardName:   "Mewtwo",
			minConfidence:  0.7,
		},
		{
			name: "VMAX card detection",
			input: `Rayquaza VMAX
Dragon
HP 320
217/203`,
			wantCardNumber: "217",
			wantSetTotal:   "203",
			wantHP:         "320",
			wantCardName:   "Rayquaza VMAX",
			wantIsFoil:     false, // Conservative: VMAX cards come in both foil and non-foil
			minConfidence:  0.8,
		},
		{
			name: "VSTAR card detection",
			input: `Arceus VSTAR
Colorless
HP 280
123/172
SWSH9`,
			wantCardNumber: "123",
			wantSetTotal:   "172",
			wantHP:         "280",
			wantSetCode:    "swsh9",
			wantCardName:   "Arceus VSTAR",
			wantIsFoil:     false, // Conservative: VSTAR cards come in both foil and non-foil
			minConfidence:  0.8,
		},
		{
			name: "EX card modern",
			input: `Charizard ex
Fire
HP 330
006/091
SV4`,
			wantCardNumber: "6",
			wantSetTotal:   "091",
			wantHP:         "330",
			wantSetCode:    "sv4",
			wantCardName:   "Charizard ex",
			wantIsFoil:     false, // Conservative: ex cards come in both foil and non-foil
			minConfidence:  0.8,
		},
		{
			name: "Set name detection - Vivid Voltage",
			input: `Pikachu
HP 60
Basic Pokemon
Vivid Voltage
063/185`,
			wantCardNumber: "63",
			wantSetTotal:   "185",
			wantHP:         "60",
			wantSetCode:    "swsh4",
			wantCardName:   "Pikachu",
			minConfidence:  0.8,
		},
		{
			name: "Set name detection - Scarlet & Violet",
			input: `Sprigatito
HP 60
Grass
Scarlet & Violet
013/198`,
			wantCardNumber: "13",
			wantSetTotal:   "198",
			wantHP:         "60",
			wantSetCode:    "sv1",
			wantCardName:   "Sprigatito",
			minConfidence:  0.8,
		},
		{
			name: "Holo rare detection",
			input: `Umbreon
Darkness
HP 110
Holo Rare
Evolving Skies
SWSH7`,
			wantHP:       "110",
			wantSetCode:  "swsh7",
			wantCardName: "Umbreon",
			wantIsFoil:   true,
		},
		{
			name: "Full art card detection",
			input: `Mew VMAX
Full Art
HP 310
Fusion Strike
269/264`,
			wantCardNumber: "269",
			wantSetTotal:   "264",
			wantHP:         "310",
			wantCardName:   "Mew VMAX",
			wantIsFoil:     false, // Conservative: Full Art only medium confidence, not auto-foil
			minConfidence:  0.7,
		},
		{
			name: "Secret rare rainbow",
			input: `Pikachu VMAX
Rainbow Rare
Secret
HP 310
188/185`,
			wantCardNumber: "188",
			wantSetTotal:   "185",
			wantHP:         "310",
			wantCardName:   "Pikachu VMAX",
			wantIsFoil:     false, // Conservative: Rainbow/Secret only medium confidence, not auto-foil
			minConfidence:  0.7,
		},
		{
			name: "Noisy OCR with partial text",
			input: `chari2ard
H P 1 7 0
025/185
s wsh4`,
			wantCardNumber: "25",
			wantSetTotal:   "185",
			// Note: noisy OCR may not extract all fields correctly
		},
		{
			name: "Reverse holo detection",
			input: `Ditto
Colorless
HP 70
Reverse Holo
132/198`,
			wantCardNumber: "132",
			wantSetTotal:   "198",
			wantHP:         "70",
			wantCardName:   "Ditto",
			wantIsFoil:     true,
		},
		{
			name: "Gold card detection",
			input: `Energy Switch
Trainer - Item
Gold
163/159`,
			wantCardNumber: "163",
			wantSetTotal:   "159",
			wantCardName:   "Energy Switch",
			wantIsFoil:     false, // Conservative: Gold only medium confidence, not auto-foil
		},
		{
			name: "Promo card",
			input: `Pikachu
Lightning
HP 60
Promo
SWSH039`,
			wantHP:       "60",
			wantCardName: "Pikachu",
		},
		{
			name: "151 set detection",
			input: `Mewtwo ex
Psychic
HP 230
151
150/165`,
			wantCardNumber: "150",
			wantSetTotal:   "165",
			wantHP:         "230",
			wantSetCode:    "sv3pt5",
			wantCardName:   "Mewtwo ex",
			wantIsFoil:     false, // Conservative: ex cards come in both foil and non-foil
			minConfidence:  0.8,
		},
		{
			name: "Special illustration rare",
			input: `Charizard
Illustration Rare
Special Art
Fire
HP 180
199/165`,
			wantCardNumber: "199",
			wantSetTotal:   "165",
			wantHP:         "180",
			wantCardName:   "Charizard",
			wantIsFoil:     false, // Conservative: Illustration/Special Art only medium confidence, not auto-foil
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseOCRText(tt.input, "pokemon")

			if tt.wantCardNumber != "" && result.CardNumber != tt.wantCardNumber {
				t.Errorf("CardNumber = %q, want %q", result.CardNumber, tt.wantCardNumber)
			}

			if tt.wantSetTotal != "" && result.SetTotal != tt.wantSetTotal {
				t.Errorf("SetTotal = %q, want %q", result.SetTotal, tt.wantSetTotal)
			}

			if tt.wantHP != "" && result.HP != tt.wantHP {
				t.Errorf("HP = %q, want %q", result.HP, tt.wantHP)
			}

			if tt.wantSetCode != "" && result.SetCode != tt.wantSetCode {
				t.Errorf("SetCode = %q, want %q", result.SetCode, tt.wantSetCode)
			}

			if tt.wantCardName != "" && result.CardName != tt.wantCardName {
				t.Errorf("CardName = %q, want %q", result.CardName, tt.wantCardName)
			}

			if tt.wantIsFoil && !result.IsFoil {
				t.Errorf("IsFoil = %v, want %v", result.IsFoil, tt.wantIsFoil)
			}

			if tt.minConfidence > 0 && result.Confidence < tt.minConfidence {
				t.Errorf("Confidence = %v, want >= %v", result.Confidence, tt.minConfidence)
			}
		})
	}
}

// TestMTGCardExtraction tests MTG card parsing from OCR text
func TestMTGCardExtraction(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantCardNumber string
		wantSetTotal   string
		wantSetCode    string
		wantCardName   string
		wantIsFoil     bool
		minConfidence  float64
	}{
		{
			name: "Standard MTG card with set on own line",
			input: `Lightning Bolt
Instant
Deal 3 damage to any target.
ONE
123/456`,
			wantCardNumber: "123",
			wantSetTotal:   "456",
			wantSetCode:    "ONE",
			wantCardName:   "Lightning Bolt",
			minConfidence:  0.7,
		},
		{
			name: "MTG creature card with P/T separate from collector",
			input: `Sheoldred, the Apocalypse
Legendary Creature - Phyrexian Praetor
4/5
107/281
DMU`,
			wantCardNumber: "107",
			wantSetTotal:   "281",
			wantSetCode:    "DMU",
			wantCardName:   "Sheoldred, the Apocalypse",
			minConfidence:  0.7,
		},
		{
			name: "MTG foil card",
			input: `Ragavan, Nimble Pilferer
Foil
Creature - Monkey Pirate
138/303
MH2`,
			wantCardNumber: "138",
			wantSetTotal:   "303",
			wantSetCode:    "MH2",
			wantCardName:   "Ragavan, Nimble Pilferer",
			wantIsFoil:     true,
			minConfidence:  0.7,
		},
		{
			name: "MTG showcase card",
			input: `The One Ring
Showcase
Legendary Artifact
246/281
LTR`,
			wantCardNumber: "246",
			wantSetTotal:   "281",
			wantSetCode:    "LTR",
			wantCardName:   "The One Ring",
			wantIsFoil:     true,
		},
		{
			name: "MTG borderless card",
			input: `Wrenn and Six
Borderless
Legendary Planeswalker - Wrenn
312/303
2LU`,
			wantCardNumber: "312",
			wantSetTotal:   "303",
			wantSetCode:    "2LU",
			wantCardName:   "Wrenn and Six",
			wantIsFoil:     true,
		},
		{
			name: "MTG etched foil",
			input: `Atraxa, Praetors' Voice
Etched
Legendary Creature - Phyrexian Angel Horror
190/332
2XM`,
			wantCardNumber: "190",
			wantSetTotal:   "332",
			wantSetCode:    "2XM",
			wantCardName:   "Atraxa, Praetors' Voice",
			wantIsFoil:     true,
		},
		{
			name: "MTG extended art",
			input: `Force of Negation
Extended Art
Instant
399/303
MH2`,
			wantCardNumber: "399",
			wantSetTotal:   "303",
			wantSetCode:    "MH2",
			wantCardName:   "Force of Negation",
			wantIsFoil:     true,
		},
		{
			name: "MTG card with flavor text",
			input: `Sol Ring
Artifact
456/789
CMD`,
			wantCardNumber: "456",
			wantSetTotal:   "789",
			wantSetCode:    "CMD",
			wantCardName:   "Sol Ring",
		},
		{
			name: "MTG planeswalker",
			input: `Liliana of the Veil
Legendary Planeswalker - Liliana
097/281
DMU`,
			wantCardNumber: "097",
			wantSetTotal:   "281",
			wantSetCode:    "DMU",
			wantCardName:   "Liliana of the Veil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseOCRText(tt.input, "mtg")

			if tt.wantCardNumber != "" && result.CardNumber != tt.wantCardNumber {
				t.Errorf("CardNumber = %q, want %q", result.CardNumber, tt.wantCardNumber)
			}

			if tt.wantSetTotal != "" && result.SetTotal != tt.wantSetTotal {
				t.Errorf("SetTotal = %q, want %q", result.SetTotal, tt.wantSetTotal)
			}

			if tt.wantSetCode != "" && result.SetCode != tt.wantSetCode {
				t.Errorf("SetCode = %q, want %q", result.SetCode, tt.wantSetCode)
			}

			if tt.wantCardName != "" && result.CardName != tt.wantCardName {
				t.Errorf("CardName = %q, want %q", result.CardName, tt.wantCardName)
			}

			if tt.wantIsFoil && !result.IsFoil {
				t.Errorf("IsFoil = %v, want %v", result.IsFoil, tt.wantIsFoil)
			}

			if tt.minConfidence > 0 && result.Confidence < tt.minConfidence {
				t.Errorf("Confidence = %v, want >= %v", result.Confidence, tt.minConfidence)
			}
		})
	}
}

// TestImageAnalysisIntegration tests that image analysis is properly applied
func TestImageAnalysisIntegration(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		analysis      *ImageAnalysis
		wantIsFoil    bool
		wantCondition string
	}{
		{
			name:  "High confidence foil from image analysis",
			input: "Pikachu\nHP 60\n025/185",
			analysis: &ImageAnalysis{
				IsFoilDetected:     true,
				FoilConfidence:     0.85,
				SuggestedCondition: "NM",
				EdgeWhiteningScore: 0.02,
				CornerScores: map[string]float64{
					"topLeft":     0.01,
					"topRight":    0.02,
					"bottomLeft":  0.01,
					"bottomRight": 0.03,
				},
			},
			wantIsFoil:    true,
			wantCondition: "NM",
		},
		{
			name:  "Low confidence foil NOT auto-detected (conservative)",
			input: "Bulbasaur\nHP 70\n001/185",
			analysis: &ImageAnalysis{
				IsFoilDetected:     true,
				FoilConfidence:     0.5,
				SuggestedCondition: "LP",
				EdgeWhiteningScore: 0.12,
				CornerScores: map[string]float64{
					"topLeft":     0.10,
					"topRight":    0.12,
					"bottomLeft":  0.11,
					"bottomRight": 0.15,
				},
			},
			wantIsFoil:    false, // Conservative: needs >= 0.8 confidence to auto-set foil
			wantCondition: "LP",
		},
		{
			name:  "Non-foil with good condition",
			input: "Charmander\nHP 60\n004/185",
			analysis: &ImageAnalysis{
				IsFoilDetected:     false,
				FoilConfidence:     0.2,
				SuggestedCondition: "NM",
				EdgeWhiteningScore: 0.01,
				CornerScores: map[string]float64{
					"topLeft":     0.01,
					"topRight":    0.01,
					"bottomLeft":  0.01,
					"bottomRight": 0.01,
				},
			},
			wantIsFoil:    false,
			wantCondition: "NM",
		},
		{
			name:  "VMAX text no longer auto-triggers foil (conservative)",
			input: "Charizard VMAX\nHP 330\n020/185",
			analysis: &ImageAnalysis{
				IsFoilDetected:     false,
				FoilConfidence:     0.3,
				SuggestedCondition: "MP",
				EdgeWhiteningScore: 0.25,
				CornerScores:       map[string]float64{},
			},
			wantIsFoil:    false, // Conservative: VMAX no longer auto-triggers foil
			wantCondition: "MP",
		},
		{
			name:          "Nil image analysis doesn't break parsing",
			input:         "Squirtle\nHP 60\n007/185",
			analysis:      nil,
			wantIsFoil:    false,
			wantCondition: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseOCRTextWithAnalysis(tt.input, "pokemon", tt.analysis)

			if result.IsFoil != tt.wantIsFoil {
				t.Errorf("IsFoil = %v, want %v", result.IsFoil, tt.wantIsFoil)
			}

			if result.SuggestedCondition != tt.wantCondition {
				t.Errorf("SuggestedCondition = %q, want %q", result.SuggestedCondition, tt.wantCondition)
			}

			if tt.analysis != nil {
				if result.EdgeWhiteningScore != tt.analysis.EdgeWhiteningScore {
					t.Errorf("EdgeWhiteningScore = %v, want %v", result.EdgeWhiteningScore, tt.analysis.EdgeWhiteningScore)
				}
			}
		})
	}
}

// TestSetDetectionFromTotal tests inference of set code from card total
func TestSetDetectionFromTotal(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantSetCode string
	}{
		{
			name:        "Vivid Voltage from 185 total",
			input:       "Pikachu\n001/185",
			wantSetCode: "swsh4",
		},
		{
			name:        "151 from 165 total",
			input:       "Mew\n150/165",
			wantSetCode: "sv3pt5",
		},
		{
			name:        "Paldea Evolved from 193 total",
			input:       "Spidops ex\n089/193",
			wantSetCode: "sv2",
		},
		{
			name:        "Obsidian Flames from 197 total",
			input:       "Tyranitar ex\n156/197",
			wantSetCode: "sv3",
		},
		{
			name:        "Paradox Rift from 182 total",
			input:       "Iron Crown ex\n081/182",
			wantSetCode: "sv4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseOCRText(tt.input, "pokemon")

			if result.SetCode != tt.wantSetCode {
				t.Errorf("SetCode = %q, want %q", result.SetCode, tt.wantSetCode)
			}
		})
	}
}

// TestSetDetectionFromName tests inference of set code from set name in text
func TestSetDetectionFromName(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantSetCode string
		wantSetName string
	}{
		{
			name:        "Vivid Voltage set name",
			input:       "Pikachu\nVivid Voltage\n001/185",
			wantSetCode: "swsh4",
			wantSetName: "VIVID VOLTAGE",
		},
		{
			name:        "Sword & Shield base",
			input:       "Cinderace\nSword & Shield\n036/202",
			wantSetCode: "swsh1",
			wantSetName: "SWORD & SHIELD",
		},
		{
			name:        "Evolving Skies",
			input:       "Umbreon VMAX\nEvolving Skies\n215/203",
			wantSetCode: "swsh7",
			wantSetName: "EVOLVING SKIES",
		},
		{
			name:        "Paldean Fates",
			input:       "Charizard ex\nPaldean Fates\n054/091",
			wantSetCode: "sv4pt5",
			wantSetName: "PALDEAN FATES",
		},
		{
			name:        "Scarlet & Violet with ampersand",
			input:       "Koraidon ex\nScarlet & Violet\n125/198",
			wantSetCode: "sv1",
			wantSetName: "SCARLET & VIOLET",
		},
		{
			name:        "Scarlet and Violet with 'and'",
			input:       "Miraidon ex\nScarlet and Violet\n081/198",
			wantSetCode: "sv1",
			wantSetName: "SCARLET AND VIOLET",
		},
		{
			name:        "Champion's Path with apostrophe",
			input:       "Charizard V\nChampion's Path\n079/073",
			wantSetCode: "swsh3pt5",
			wantSetName: "CHAMPION'S PATH",
		},
		{
			name:        "Champions Path without apostrophe",
			input:       "Machamp V\nChampions Path\n026/073",
			wantSetCode: "swsh3pt5",
			wantSetName: "CHAMPIONS PATH",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseOCRText(tt.input, "pokemon")

			if result.SetCode != tt.wantSetCode {
				t.Errorf("SetCode = %q, want %q", result.SetCode, tt.wantSetCode)
			}

			if tt.wantSetName != "" && result.SetName != tt.wantSetName {
				t.Errorf("SetName = %q, want %q", result.SetName, tt.wantSetName)
			}
		})
	}
}

// TestFoilDetection tests foil detection with CONSERVATIVE behavior
// Card types (V, VMAX, VSTAR, GX, EX) do NOT auto-trigger foil
// Only explicit foil text (HOLO, FOIL, REVERSE HOLO) auto-triggers foil
// See TestConservativeFoilDetection for more comprehensive coverage
func TestFoilDetection(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		game       string
		wantIsFoil bool
		wantHints  []string
	}{
		// Pokemon card types - should NOT auto-trigger foil (conservative)
		{
			name:       "Pokemon V card",
			input:      "Pikachu V\nHP 190",
			game:       "pokemon",
			wantIsFoil: false, // Conservative: V cards come in both foil and non-foil
		},
		{
			name:       "Pokemon VMAX card",
			input:      "Charizard VMAX\nHP 330",
			game:       "pokemon",
			wantIsFoil: false, // Conservative: VMAX cards come in both foil and non-foil
		},
		{
			name:       "Pokemon VSTAR card",
			input:      "Arceus VSTAR\nHP 280",
			game:       "pokemon",
			wantIsFoil: false, // Conservative: VSTAR cards come in both foil and non-foil
		},
		{
			name:       "Pokemon GX card",
			input:      "Umbreon GX\nHP 200",
			game:       "pokemon",
			wantIsFoil: false, // Conservative: GX cards come in both foil and non-foil
		},
		{
			name:       "Pokemon EX card",
			input:      "Mewtwo EX\nHP 170",
			game:       "pokemon",
			wantIsFoil: false, // Conservative: EX cards come in both foil and non-foil
		},
		{
			name:       "Pokemon ex lowercase modern",
			input:      "Charizard ex\nHP 330",
			game:       "pokemon",
			wantIsFoil: false, // Conservative: ex cards come in both foil and non-foil
		},
		// Explicit foil text - SHOULD auto-trigger foil
		{
			name:       "Pokemon Holo text",
			input:      "Pikachu\nHolo Rare\nHP 60",
			game:       "pokemon",
			wantIsFoil: true, // Explicit "Holo" text
		},
		{
			name:       "Pokemon Reverse Holo",
			input:      "Magikarp\nReverse Holo\nHP 30",
			game:       "pokemon",
			wantIsFoil: true, // Explicit "Reverse Holo" text
		},
		// Medium confidence patterns - should NOT auto-trigger foil
		{
			name:       "Pokemon Full Art",
			input:      "Professor's Research\nFull Art\nTrainer - Supporter",
			game:       "pokemon",
			wantIsFoil: false, // Medium confidence only
		},
		{
			name:       "Pokemon Rainbow Rare",
			input:      "Pikachu VMAX\nRainbow Rare\nHP 310",
			game:       "pokemon",
			wantIsFoil: false, // Medium confidence only
		},
		{
			name:       "Pokemon Secret Rare",
			input:      "Gold Energy\nSecret\n188/185",
			game:       "pokemon",
			wantIsFoil: false, // Medium confidence only
		},
		{
			name:       "Pokemon Gold card",
			input:      "Switch\nGold\nTrainer - Item",
			game:       "pokemon",
			wantIsFoil: false, // Medium confidence only
		},
		{
			name:       "Pokemon Illustration Rare",
			input:      "Miraidon\nIllustration Rare\nHP 220",
			game:       "pokemon",
			wantIsFoil: false, // Medium confidence only
		},
		{
			name:       "Pokemon Special Art Rare",
			input:      "Giratina V\nSpecial Art Rare\nHP 220",
			game:       "pokemon",
			wantIsFoil: false, // Medium confidence only
		},
		{
			name:       "Pokemon shiny",
			input:      "Charizard\nShiny\nHP 170",
			game:       "pokemon",
			wantIsFoil: false, // Medium confidence only
		},
		// MTG explicit foil text - SHOULD auto-trigger foil
		{
			name:       "MTG foil",
			input:      "Lightning Bolt\nFoil\nInstant",
			game:       "mtg",
			wantIsFoil: true,
		},
		{
			name:       "MTG etched",
			input:      "Sol Ring\nEtched\nArtifact",
			game:       "mtg",
			wantIsFoil: true,
		},
		{
			name:       "MTG showcase",
			input:      "The One Ring\nShowcase\nLegendary Artifact",
			game:       "mtg",
			wantIsFoil: true,
		},
		{
			name:       "MTG borderless",
			input:      "Force of Will\nBorderless\nInstant",
			game:       "mtg",
			wantIsFoil: true,
		},
		{
			name:       "MTG extended art",
			input:      "Dockside Extortionist\nExtended Art\nCreature",
			game:       "mtg",
			wantIsFoil: true,
		},
		// Regular cards - should NOT be foil
		{
			name:       "Regular Pokemon card not foil",
			input:      "Bulbasaur\nHP 70\nBasic\n001/185",
			game:       "pokemon",
			wantIsFoil: false,
		},
		{
			name:       "Regular MTG card not foil",
			input:      "Island\nBasic Land - Island",
			game:       "mtg",
			wantIsFoil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseOCRText(tt.input, tt.game)

			if result.IsFoil != tt.wantIsFoil {
				t.Errorf("IsFoil = %v, want %v (indicators: %v)", result.IsFoil, tt.wantIsFoil, result.FoilIndicators)
			}
		})
	}
}

// TestRarityDetection tests rarity extraction
func TestRarityDetection(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantRarity string
	}{
		{
			name:       "Illustration Rare",
			input:      "Charizard\nIllustration Rare\nHP 180",
			wantRarity: "Illustration Rare",
		},
		{
			name:       "Special Art Rare",
			input:      "Giratina V\nSpecial Art Rare\nHP 220",
			wantRarity: "Special Art Rare",
		},
		{
			name:       "Hyper Rare",
			input:      "Pikachu VMAX\nHyper Rare\nHP 310",
			wantRarity: "Hyper Rare",
		},
		{
			name:       "Secret Rare",
			input:      "Gold Switch\nSecret Rare\nItem",
			wantRarity: "Secret Rare",
		},
		{
			name:       "Ultra Rare",
			input:      "Mewtwo V\nUltra Rare\nHP 220",
			wantRarity: "Ultra Rare",
		},
		{
			name:       "Double Rare",
			input:      "Charizard ex\nDouble Rare\nHP 330",
			wantRarity: "Double Rare",
		},
		{
			name:       "Rare Holo",
			input:      "Umbreon\nRare Holo\nHP 110",
			wantRarity: "Rare Holo",
		},
		{
			name:       "Rare",
			input:      "Snorlax\nRare\nHP 150",
			wantRarity: "Rare",
		},
		{
			name:       "Uncommon",
			input:      "Pidgeotto\nUncommon\nHP 90",
			wantRarity: "Uncommon",
		},
		{
			name:       "Common",
			input:      "Rattata\nCommon\nHP 40",
			wantRarity: "Common",
		},
		{
			name:       "Promo",
			input:      "Pikachu\nPromo\nHP 60",
			wantRarity: "Promo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseOCRText(tt.input, "pokemon")

			if result.Rarity != tt.wantRarity {
				t.Errorf("Rarity = %q, want %q", result.Rarity, tt.wantRarity)
			}
		})
	}
}

// TestCardNameExtraction tests card name extraction from various formats
func TestCardNameExtraction(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		game         string
		wantCardName string
	}{
		{
			name: "Simple Pokemon name",
			input: `Pikachu
HP 60
Basic Pokemon
Lightning`,
			game:         "pokemon",
			wantCardName: "Pikachu",
		},
		{
			name: "Pokemon with V suffix",
			input: `Charizard V
HP 220
Fire
Stage 1`,
			game:         "pokemon",
			wantCardName: "Charizard V",
		},
		{
			name: "Pokemon with ex suffix lowercase",
			input: `Koraidon ex
HP 230
Fighting
Basic`,
			game:         "pokemon",
			wantCardName: "Koraidon ex",
		},
		{
			name: "Pokemon removes HP from name",
			input: `Mewtwo HP 150
Psychic`,
			game:         "pokemon",
			wantCardName: "Mewtwo",
		},
		{
			name: "MTG simple name",
			input: `Lightning Bolt
Instant
Deal 3 damage`,
			game:         "mtg",
			wantCardName: "Lightning Bolt",
		},
		{
			name: "MTG legendary creature",
			input: `Sheoldred, the Apocalypse
Legendary Creature - Phyrexian
4/5`,
			game:         "mtg",
			wantCardName: "Sheoldred, the Apocalypse",
		},
		{
			name: "MTG planeswalker",
			input: `Liliana of the Veil
Legendary Planeswalker
[+1]: Each player discards`,
			game:         "mtg",
			wantCardName: "Liliana of the Veil",
		},
		{
			name: "Pokemon skips Basic line",
			input: `Basic
Bulbasaur
HP 70
Grass`,
			game:         "pokemon",
			wantCardName: "Bulbasaur",
		},
		{
			name: "Pokemon skips Stage line",
			input: `Stage 2
Blastoise
HP 180
Water`,
			game:         "pokemon",
			wantCardName: "Blastoise",
		},
		{
			name: "MTG skips creature line",
			input: `Creature - Human
Thalia, Guardian of Thraben
First Strike`,
			game:         "mtg",
			wantCardName: "Thalia, Guardian of Thraben",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseOCRText(tt.input, tt.game)

			if result.CardName != tt.wantCardName {
				t.Errorf("CardName = %q, want %q", result.CardName, tt.wantCardName)
			}
		})
	}
}

// TestConditionHintDetection tests detection of grading labels
func TestConditionHintDetection(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantHintCount int
		wantHints     []string
	}{
		{
			name:          "PSA graded card",
			input:         "Charizard\nPSA 10\nGem Mint",
			wantHintCount: 3, // PSA, Gem Mint, and the grade
		},
		{
			name:          "BGS graded card",
			input:         "Pikachu\nBGS 9.5\n",
			wantHintCount: 2,
		},
		{
			name:          "Near Mint label",
			input:         "Mewtwo\nNear Mint\nHP 150",
			wantHintCount: 1,
		},
		{
			name:          "Damaged card",
			input:         "Squirtle\nDamaged\nHP 50",
			wantHintCount: 1,
		},
		{
			name:          "No grading labels",
			input:         "Bulbasaur\nHP 70\nBasic",
			wantHintCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseOCRText(tt.input, "pokemon")

			if len(result.ConditionHints) < tt.wantHintCount {
				t.Errorf("ConditionHints count = %d, want >= %d (hints: %v)",
					len(result.ConditionHints), tt.wantHintCount, result.ConditionHints)
			}
		})
	}
}

// TestConfidenceCalculation tests confidence scoring
func TestConfidenceCalculation(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		game          string
		minConfidence float64
		maxConfidence float64
	}{
		{
			name: "Full Pokemon data high confidence",
			input: `Charizard
HP 170
025/185
SWSH4`,
			game:          "pokemon",
			minConfidence: 0.9,
			maxConfidence: 1.0,
		},
		{
			name: "Name and number only",
			input: `Pikachu
025/185`,
			game:          "pokemon",
			minConfidence: 0.6,
			maxConfidence: 1.0, // Set total detection adds extra confidence
		},
		{
			name:          "Name only low confidence",
			input:         "Bulbasaur",
			game:          "pokemon",
			minConfidence: 0.3,
			maxConfidence: 0.5,
		},
		{
			name: "MTG with all data",
			input: `Lightning Bolt
ONE
123/456`,
			game:          "mtg",
			minConfidence: 0.7,
			maxConfidence: 1.0,
		},
		{
			name:          "Very short text low confidence",
			input:         "abc",
			game:          "pokemon",
			minConfidence: 0.0,
			maxConfidence: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseOCRText(tt.input, tt.game)

			if result.Confidence < tt.minConfidence {
				t.Errorf("Confidence = %v, want >= %v", result.Confidence, tt.minConfidence)
			}

			if result.Confidence > tt.maxConfidence {
				t.Errorf("Confidence = %v, want <= %v", result.Confidence, tt.maxConfidence)
			}
		})
	}
}

// TestMaxTextLengthProtection tests that overly long text is truncated
func TestMaxTextLengthProtection(t *testing.T) {
	// Create a very long string
	longText := ""
	for i := 0; i < 20000; i++ {
		longText += "a"
	}

	result := ParseOCRText(longText, "pokemon")

	// Should not panic and should truncate
	if len(result.RawText) > maxOCRTextLength {
		t.Errorf("RawText length = %d, want <= %d", len(result.RawText), maxOCRTextLength)
	}
}

// TestEdgeCases tests various edge cases and error conditions
func TestEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		game  string
	}{
		{
			name:  "Empty string",
			input: "",
			game:  "pokemon",
		},
		{
			name:  "Only whitespace",
			input: "   \n\n\t  ",
			game:  "pokemon",
		},
		{
			name:  "Only numbers",
			input: "123 456 789",
			game:  "pokemon",
		},
		{
			name:  "Special characters only",
			input: "!@#$%^&*()",
			game:  "pokemon",
		},
		{
			name:  "Very noisy OCR",
			input: "C h a r i 2 a r d\nH P 1 7 0\n0 2 5 / 1 8 5",
			game:  "pokemon",
		},
		{
			name:  "Mixed case game type",
			input: "Pikachu\nHP 60",
			game:  "POKEMON",
		},
		{
			name:  "Unknown game type",
			input: "Some Card\n123/456",
			game:  "yugioh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			result := ParseOCRText(tt.input, tt.game)

			// Result should not be nil
			if result == nil {
				t.Error("Result should not be nil")
			}
		})
	}
}

// Real world OCR text samples that might be difficult to parse
func TestRealWorldOCRSamples(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		game           string
		wantCardNumber string
		wantCardName   string
	}{
		{
			name: "OCR with line breaks in middle of text",
			input: `Char
izard
HP 170
Basic Fire
025/185`,
			game:           "pokemon",
			wantCardNumber: "25",
			// Card name might be "Char" due to line break
		},
		{
			name: "OCR with extra spaces",
			input: `Pikachu    V
H  P     190
025  /  185`,
			game:           "pokemon",
			wantCardNumber: "25",
		},
		{
			name: "OCR with misread characters",
			input: `Charizard
HP l70
O25/l85`,
			game: "pokemon",
			// May not parse correctly due to l/1 confusion
		},
		{
			name: "OCR reading card upside down partial",
			input: `581/520
HSMS
071 dH
drazirahC`,
			game: "pokemon",
			// Unlikely to parse correctly
		},
		{
			name: "Japanese text mixed with English",
			input: `リザードン
Charizard
HP 180
025/185`,
			game:           "pokemon",
			wantCardNumber: "25",
			wantCardName:   "Charizard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseOCRText(tt.input, tt.game)

			// Just verify no panic - actual parsing may vary
			if result == nil {
				t.Error("Result should not be nil")
			}

			if tt.wantCardNumber != "" && result.CardNumber != tt.wantCardNumber {
				t.Logf("CardNumber = %q, expected %q (may vary due to OCR quality)", result.CardNumber, tt.wantCardNumber)
			}

			if tt.wantCardName != "" && result.CardName != tt.wantCardName {
				t.Logf("CardName = %q, expected %q (may vary due to OCR quality)", result.CardName, tt.wantCardName)
			}
		})
	}
}

// TestPokemonRealWorldOCRSamples tests actual OCR output from Pokemon cards
func TestPokemonRealWorldOCRSamples(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		wantCardNumber    string
		wantSetCode       string
		wantCardName      string
		wantHP            string
		wantIsFoil        bool
		wantMinConfidence float64
	}{
		{
			name: "Charizard VMAX from Darkness Ablaze - clean scan",
			input: `Charizard VMAX
HP 330
Fire
VMAX Evolution
Darkness Ablaze
020/189
SWSH3
©2020 Pokemon`,
			wantCardNumber:    "20",
			wantSetCode:       "swsh3",
			wantCardName:      "Charizard VMAX",
			wantHP:            "330",
			wantIsFoil:        false, // Conservative: VMAX no longer auto-triggers foil
			wantMinConfidence: 0.9,
		},
		{
			name: "Pikachu V from Vivid Voltage - typical scan",
			input: `Pikachu V
Lightning
HP 190
Basic Pokemon V
When your Pokemon V is Knocked
Out, your opponent takes 2 Prize cards.
Vivid Voltage
043/185`,
			wantCardNumber:    "43",
			wantSetCode:       "swsh4",
			wantCardName:      "Pikachu V",
			wantHP:            "190",
			wantIsFoil:        false, // Conservative: V cards no longer auto-trigger foil
			wantMinConfidence: 0.8,
		},
		{
			name: "Umbreon VMAX Trainer Gallery",
			input: `Umbreon VMAX
HP 310
Darkness
TG17/TG30
Brilliant Stars
SWSH9
Illus. HYOGONOSUKE`,
			wantCardNumber:    "TG17",
			wantSetCode:       "swsh9",
			wantCardName:      "Umbreon VMAX",
			wantHP:            "310",
			wantIsFoil:        false, // Conservative: VMAX no longer auto-triggers foil
			wantMinConfidence: 0.7,
		},
		{
			name: "Mewtwo ex from 151",
			input: `Mewtwo ex
HP 330
Psychic
Basic Pokemon ex
When your Pokemon ex is Knocked
Out, your opponent takes 2 Prize cards.
151
150/165
SV3PT5`,
			wantCardNumber:    "150",
			wantSetCode:       "sv3pt5",
			wantCardName:      "Mewtwo ex",
			wantHP:            "330",
			wantIsFoil:        false, // Conservative: ex cards no longer auto-trigger foil
			wantMinConfidence: 0.9,
		},
		{
			name: "Charizard ex Special Art Rare from Paldean Fates",
			input: `Charizard ex
HP 330
Fire
Special Art Rare
Illustration Rare
Paldean Fates
054/091`,
			wantCardNumber:    "54",
			wantSetCode:       "sv4pt5",
			wantCardName:      "Charizard ex",
			wantHP:            "330",
			wantIsFoil:        false, // Conservative: Special Art/Illustration Rare only medium confidence
			wantMinConfidence: 0.8,
		},
		{
			name: "Regular Pokemon card - no foil indicators",
			input: `Bulbasaur
HP 70
Grass
Basic Pokemon
001/198
Scarlet & Violet`,
			wantCardNumber:    "1",
			wantSetCode:       "sv1",
			wantCardName:      "Bulbasaur",
			wantHP:            "70",
			wantIsFoil:        false,
			wantMinConfidence: 0.7,
		},
		{
			name: "Trainer card - Professor's Research",
			input: `Professor's Research
Trainer - Supporter
Discard your hand and draw 7 cards.
You may play only 1 Supporter card during your turn.
147/198
Scarlet & Violet`,
			wantCardNumber:    "147",
			wantSetCode:       "sv1",
			wantCardName:      "Professor's Research",
			wantIsFoil:        false,
			wantMinConfidence: 0.6,
		},
		{
			name: "Energy card",
			input: `Basic Lightning Energy
Energy
Scarlet & Violet
Illustrations by 5ban Graphics`,
			wantSetCode:       "sv1",
			wantMinConfidence: 0.3,
		},
		{
			name: "Galarian Gallery card",
			input: `Eevee
HP 60
Colorless
GG01/GG70
Crown Zenith
Galarian Gallery`,
			wantCardNumber:    "GG01",
			wantSetCode:       "swsh12pt5",
			wantCardName:      "Eevee",
			wantHP:            "60",
			wantMinConfidence: 0.5,
		},
		{
			name: "Card with blurry HP - number/letter confusion",
			input: `Pikachu
HP 6O
Lightning
Basic Pokemon
O25/185
SWSH4`,
			wantCardNumber:    "25", // Should handle O->0 confusion in number
			wantSetCode:       "swsh4",
			wantCardName:      "Pikachu",
			wantMinConfidence: 0.5,
		},
		{
			name: "Promo card format",
			input: `Pikachu
HP 60
Lightning
Promo
SWSH039
McDonald's Collection 2021`,
			wantCardName:      "Pikachu",
			wantHP:            "60",
			wantMinConfidence: 0.4,
		},
		{
			name: "Japanese to English set name",
			input: `Sprigatito
草 Grass
HP 60
Violet ex
013/078`,
			wantCardNumber:    "13",
			wantCardName:      "Sprigatito",
			wantHP:            "60",
			wantMinConfidence: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseOCRText(tt.input, "pokemon")

			if result == nil {
				t.Fatal("Result should not be nil")
			}

			if tt.wantCardNumber != "" && result.CardNumber != tt.wantCardNumber {
				t.Errorf("CardNumber = %q, want %q", result.CardNumber, tt.wantCardNumber)
			}

			if tt.wantSetCode != "" && result.SetCode != tt.wantSetCode {
				t.Errorf("SetCode = %q, want %q", result.SetCode, tt.wantSetCode)
			}

			if tt.wantCardName != "" && result.CardName != tt.wantCardName {
				t.Errorf("CardName = %q, want %q", result.CardName, tt.wantCardName)
			}

			if tt.wantHP != "" && result.HP != tt.wantHP {
				t.Errorf("HP = %q, want %q", result.HP, tt.wantHP)
			}

			if tt.wantIsFoil && !result.IsFoil {
				t.Errorf("IsFoil = %v, want %v (indicators: %v)", result.IsFoil, tt.wantIsFoil, result.FoilIndicators)
			}

			if result.Confidence < tt.wantMinConfidence {
				t.Errorf("Confidence = %v, want >= %v", result.Confidence, tt.wantMinConfidence)
			}
		})
	}
}

// TestMTGRealWorldOCRSamples tests actual OCR output from MTG cards
func TestMTGRealWorldOCRSamples(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		wantCardNumber    string
		wantSetCode       string
		wantCardName      string
		wantIsFoil        bool
		wantMinConfidence float64
	}{
		{
			name: "Sheoldred, the Apocalypse - standard",
			input: `Sheoldred, the Apocalypse
Legendary Creature — Phyrexian Praetor
Deathtouch
Whenever you draw a card, you gain 2 life.
Whenever an opponent draws a card, they lose 2 life.
4/5
107/281
DMU
Illus. Chris Rahn`,
			wantCardNumber:    "107",
			wantSetCode:       "DMU",
			wantCardName:      "Sheoldred, the Apocalypse",
			wantIsFoil:        false,
			wantMinConfidence: 0.7,
		},
		{
			name: "The One Ring - Showcase variant",
			input: `The One Ring
Legendary Artifact
Indestructible
Showcase
When The One Ring enters the battlefield, if you cast it,
you gain protection from everything until your next turn.
At the beginning of your upkeep, you lose 1 life for each
burden counter on The One Ring.
246/281
LTR`,
			wantCardNumber:    "246",
			wantSetCode:       "LTR",
			wantCardName:      "The One Ring",
			wantIsFoil:        true, // Showcase triggers foil
			wantMinConfidence: 0.7,
		},
		{
			name: "Lightning Bolt - Modern Horizons 2",
			input: `Lightning Bolt
Instant
Lightning Bolt deals 3 damage to any target.
073/303
MH2
Illus. Christopher Moeller`,
			wantCardNumber:    "073",
			wantSetCode:       "MH2",
			wantCardName:      "Lightning Bolt",
			wantIsFoil:        false,
			wantMinConfidence: 0.7,
		},
		{
			name: "Ragavan, Nimble Pilferer - Foil",
			input: `Ragavan, Nimble Pilferer
Foil
Legendary Creature — Monkey Pirate
Whenever Ragavan, Nimble Pilferer deals combat
damage to a player, create a Treasure token and exile
the top card of that player's library. Until end of turn,
you may cast that card.
Dash 1R
138/303
MH2`,
			wantCardNumber:    "138",
			wantSetCode:       "MH2",
			wantCardName:      "Ragavan, Nimble Pilferer",
			wantIsFoil:        true,
			wantMinConfidence: 0.7,
		},
		{
			name: "Sol Ring - Commander set",
			input: `Sol Ring
Artifact
1
T: Add CC.
456/789
CMD`,
			wantCardNumber:    "456",
			wantSetCode:       "CMD",
			wantCardName:      "Sol Ring",
			wantIsFoil:        false,
			wantMinConfidence: 0.6,
		},
		{
			name: "Atraxa, Praetors Voice - Etched Foil",
			input: `Atraxa, Praetors' Voice
Etched
Legendary Creature — Phyrexian Angel Horror
Flying, vigilance, deathtouch, lifelink
At the beginning of your end step, proliferate.
4/4
190/332
2XM`,
			wantCardNumber:    "190",
			wantSetCode:       "2XM",
			wantCardName:      "Atraxa, Praetors' Voice",
			wantIsFoil:        true,
			wantMinConfidence: 0.7,
		},
		{
			name: "Force of Negation - Extended Art",
			input: `Force of Negation
Extended Art
Instant
If it's not your turn, you may exile a blue card
from your hand rather than pay this spell's mana cost.
Counter target noncreature spell. If that spell
is countered this way, exile it instead of
putting it into its owner's graveyard.
399/303
MH2`,
			wantCardNumber:    "399",
			wantSetCode:       "MH2",
			wantCardName:      "Force of Negation",
			wantIsFoil:        true,
			wantMinConfidence: 0.7,
		},
		{
			name: "Planeswalker - Liliana of the Veil",
			input: `Liliana of the Veil
Legendary Planeswalker — Liliana
+1: Each player discards a card.
−2: Target player sacrifices a creature.
−6: Separate all permanents target player
controls into two piles. That player
sacrifices all permanents in the pile of their choice.
3
097/281
DMU`,
			wantCardNumber:    "097",
			wantSetCode:       "DMU",
			wantCardName:      "Liliana of the Veil",
			wantIsFoil:        false,
			wantMinConfidence: 0.7,
		},
		{
			name: "Basic Land - Island",
			input: `Island
Basic Land — Island
T: Add U.
Illus. Rob Alexander
262/281
DMU`,
			wantCardNumber:    "262",
			wantSetCode:       "DMU",
			wantMinConfidence: 0.5,
		},
		{
			name: "MTG card with alternate set code format",
			input: `Wrenn and Six
Borderless
Legendary Planeswalker — Wrenn
+1: Return up to one target land card from
your graveyard to your hand.
312/303
2LU`,
			wantCardNumber:    "312",
			wantSetCode:       "2LU",
			wantCardName:      "Wrenn and Six",
			wantIsFoil:        true,
			wantMinConfidence: 0.6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseOCRText(tt.input, "mtg")

			if result == nil {
				t.Fatal("Result should not be nil")
			}

			if tt.wantCardNumber != "" && result.CardNumber != tt.wantCardNumber {
				t.Errorf("CardNumber = %q, want %q", result.CardNumber, tt.wantCardNumber)
			}

			if tt.wantSetCode != "" && result.SetCode != tt.wantSetCode {
				t.Errorf("SetCode = %q, want %q", result.SetCode, tt.wantSetCode)
			}

			if tt.wantCardName != "" && result.CardName != tt.wantCardName {
				t.Errorf("CardName = %q, want %q", result.CardName, tt.wantCardName)
			}

			if tt.wantIsFoil != result.IsFoil {
				t.Errorf("IsFoil = %v, want %v (indicators: %v)", result.IsFoil, tt.wantIsFoil, result.FoilIndicators)
			}

			if result.Confidence < tt.wantMinConfidence {
				t.Errorf("Confidence = %v, want >= %v", result.Confidence, tt.wantMinConfidence)
			}
		})
	}
}

// TestJapaneseCardParsing tests OCR parsing for Japanese Pokemon cards
// Japanese cards are challenging because:
// 1. They have Japanese text that shouldn't be matched to English names
// 2. Some have English names printed alongside Japanese
// 3. Trainer cards often have only Japanese names
// 4. Card numbers and set codes are universal and should still be extracted
func TestJapaneseCardParsing(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantCardNumber string
		wantSetCode    string
		wantCardName   string
		wantHP         string
		wantLanguage   string
		wantNameEmpty  bool // true if we expect no name match (rely on number)
	}{
		{
			name: "Japanese Pikachu with English name present",
			input: `ピカチュウ
Pikachu
HP 60
かみなりタイプ
たねポケモン
でんこうせっか 10
025/185
SV1`,
			wantCardNumber: "25",
			wantSetCode:    "sv1",
			wantCardName:   "Pikachu",
			wantHP:         "60",
			wantLanguage:   "Japanese",
		},
		{
			name: "Japanese trainer card (pure Japanese, no English name)",
			input: `博士の研究
サポート
自分の手札をすべてトラッシュし
山札を7枚引く
147/198
SV1`,
			wantCardNumber: "147",
			wantSetCode:    "sv1",
			wantLanguage:   "Japanese",
			wantNameEmpty:  true, // No English name to match
		},

		{
			name: "Japanese Charizard with no English name",
			input: `リザードン
HP 180
ほのおタイプ
2進化ポケモン
025/185
SWSH4`,
			wantCardNumber: "25",
			wantSetCode:    "swsh4",
			wantHP:         "180",
			wantLanguage:   "Japanese",
			wantNameEmpty:  true, // No English name, must rely on number
		},
		{
			name: "Japanese Pokemon V with English suffix",
			input: `ピカチュウV
Pikachu V
HP 190
かみなりタイプ
043/185
SWSH4`,
			wantCardNumber: "43",
			wantSetCode:    "swsh4",
			wantCardName:   "Pikachu V",
			wantHP:         "190",
			wantLanguage:   "Japanese",
		},
		{
			name: "Japanese Boss's Orders trainer",
			input: `ボスの指令
サポート
相手のベンチポケモンを1匹選び
バトルポケモンと入れ替える
132/172
SWSH3`,
			wantCardNumber: "132",
			wantSetCode:    "swsh3",
			wantLanguage:   "Japanese",
			wantNameEmpty:  true, // Japanese-only trainer
		},
		{
			name: "Mixed Japanese/English with OCR errors",
			input: `ピカ チュウ
P1kachu
HP 6O
025/l85
SV1`,
			wantCardNumber: "25",
			wantSetCode:    "sv1",
			wantCardName:   "Pikachu", // Should fuzzy match despite OCR errors
			wantLanguage:   "Japanese",
		},
		{
			name: "Japanese card with English attack names",
			input: `ミュウツー
Mewtwo
HP 130
Psychic
Psystrike 100
150/165
SV3PT5`,
			wantCardNumber: "150",
			wantSetCode:    "sv3pt5",
			wantCardName:   "Mewtwo",
			wantHP:         "130",
			wantLanguage:   "Japanese",
		},
		{
			name: "Japanese item card",
			input: `ふしぎなアメ
グッズ
自分の手札から進化ポケモンを1枚選び
そのポケモンに進化する
089/165
SV2A`,
			wantCardNumber: "89",
			wantLanguage:   "Japanese",
			wantNameEmpty:  true,
		},
		{
			// This is the critical test case: TRAINER is a card type, NOT a card name.
			// Before the fix, "TRAINER" was being returned as the card name which caused
			// Japanese trainer cards to match the wrong English cards.
			name: "Japanese trainer with TRAINER as only English text",
			input: `博士の研究
TRAINER
サポート
自分の手札をすべてトラッシュし
147/198
SV1`,
			wantCardNumber: "147",
			wantSetCode:    "sv1",
			wantLanguage:   "Japanese",
			wantNameEmpty:  true, // TRAINER should be skipped, not used as card name
		},
		{
			// Focus: SUPPORTER should not be used as card name
			name: "Japanese supporter with SUPPORTER as only English text",
			input: `ナンジャモ
SUPPORTER
相手のデッキから1枚引く
091/165
SV2A`,
			wantCardNumber: "91",
			// Set code inference may vary - the focus is that SUPPORTER is skipped
			wantLanguage:  "Japanese",
			wantNameEmpty: true, // SUPPORTER should be skipped
		},
		{
			// Focus: ITEM should not be used as card name
			name: "Japanese item with ITEM as only English text",
			input: `ハイパーボール
ITEM
手札を2枚トラッシュ
ポケモンを1枚サーチ
123/165
SV2A`,
			wantCardNumber: "123",
			// Set code inference may vary - the focus is that ITEM is skipped
			wantLanguage:  "Japanese",
			wantNameEmpty: true, // ITEM should be skipped
		},
		{
			// Focus: STADIUM should not be used as card name
			name: "Japanese stadium with STADIUM as only English text",
			input: `頂への雪道
STADIUM
おたがいのポケモンの特性を無効
089/098
SV4`,
			wantCardNumber: "89",
			// Set code inference may vary - the focus is that STADIUM is skipped
			wantLanguage:  "Japanese",
			wantNameEmpty: true, // STADIUM should be skipped
		},
		{
			name: "Japanese card minimal text - just name and number",
			input: `ゲッコウガ
015/078`,
			wantCardNumber: "15",
			wantLanguage:   "Japanese",
			wantNameEmpty:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseOCRText(tt.input, "pokemon")

			if result == nil {
				t.Fatal("Result should not be nil")
			}

			// Card number should always be extractable (universal format)
			if tt.wantCardNumber != "" && result.CardNumber != tt.wantCardNumber {
				t.Errorf("CardNumber = %q, want %q", result.CardNumber, tt.wantCardNumber)
			}

			// Set code should be extractable when present
			if tt.wantSetCode != "" && !strings.EqualFold(result.SetCode, tt.wantSetCode) {
				t.Errorf("SetCode = %q, want %q", result.SetCode, tt.wantSetCode)
			}

			// HP should be extractable (uses same format)
			if tt.wantHP != "" && result.HP != tt.wantHP {
				t.Errorf("HP = %q, want %q", result.HP, tt.wantHP)
			}

			// Language detection
			if tt.wantLanguage != "" && result.DetectedLanguage != tt.wantLanguage {
				t.Errorf("DetectedLanguage = %q, want %q", result.DetectedLanguage, tt.wantLanguage)
			}

			// Card name handling
			if tt.wantNameEmpty {
				// For Japanese-only cards, we should NOT extract garbage as the name.
				// We rely on set+number matching instead.
				if result.CardName != "" {
					t.Errorf("CardName = %q, want empty", result.CardName)
				}
			} else if tt.wantCardName != "" {
				if !strings.EqualFold(result.CardName, tt.wantCardName) {
					t.Errorf("CardName = %q, want %q", result.CardName, tt.wantCardName)
				}
			}
		})
	}
}

// TestJapaneseHelperFunctions tests the Japanese text handling helper functions
func TestJapaneseHelperFunctions(t *testing.T) {
	t.Run("containsJapaneseCharacters", func(t *testing.T) {
		tests := []struct {
			input string
			want  bool
		}{
			{"Hello", false},
			{"ピカチュウ", true},
			{"Pikachu ピカチュウ", true},
			{"リザードン", true},
			{"123/456", false},
			{"HP 60", false},
			{"博士の研究", true},
			{"Ｎ", false}, // Full-width N is not Japanese script
			{"サポート", true},
		}
		for _, tt := range tests {
			got := containsJapaneseCharacters(tt.input)
			if got != tt.want {
				t.Errorf("containsJapaneseCharacters(%q) = %v, want %v", tt.input, got, tt.want)
			}
		}
	})

	t.Run("normalizeFullWidthASCII", func(t *testing.T) {
		tests := []struct {
			input string
			want  string
		}{
			{"Ｎ", "N"},
			{"ＥＸ", "EX"},
			{"ＶＭＡ Ｘ", "VMA X"},
			{"Pikachu", "Pikachu"}, // Already normal
			{"ピカチュウ", "ピカチュウ"},     // Japanese unchanged
			{"Ｖ Ｓ Ｔ Ａ Ｒ", "V S T A R"},
		}
		for _, tt := range tests {
			got := normalizeFullWidthASCII(tt.input)
			if got != tt.want {
				t.Errorf("normalizeFullWidthASCII(%q) = %q, want %q", tt.input, got, tt.want)
			}
		}
	})

	t.Run("extractEnglishWordsFromLine", func(t *testing.T) {
		tests := []struct {
			input string
			want  []string
		}{
			{"Pikachu", []string{"Pikachu"}},
			{"ピカチュウ Pikachu", []string{"Pikachu"}},
			{"HP 60", []string{"HP"}},
			{"ピカチュウV", []string{"V"}},
			{"Pikachu V VMAX", []string{"Pikachu", "V", "VMAX"}},
			{"かみなりタイプ", nil}, // No English
			{"", nil},
			{"Mr. Mime", []string{"Mr.", "Mime"}},
		}
		for _, tt := range tests {
			got := extractEnglishWordsFromLine(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("extractEnglishWordsFromLine(%q) = %v, want %v", tt.input, got, tt.want)
				continue
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("extractEnglishWordsFromLine(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		}
	})
}

// TestOCREdgeCasesAndErrors tests error handling and edge cases
func TestOCREdgeCasesAndErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
		game  string
	}{
		{
			name:  "Very long text (DoS protection)",
			input: string(make([]byte, 15000)), // Exceeds maxOCRTextLength
			game:  "pokemon",
		},
		{
			name:  "Binary data",
			input: "\x00\x01\x02\x03\x04\x05",
			game:  "pokemon",
		},
		{
			name:  "Only unicode",
			input: "日本語のカード名前",
			game:  "pokemon",
		},
		{
			name:  "HTML-like content",
			input: "<html><body>Pikachu</body></html>",
			game:  "pokemon",
		},
		{
			name:  "SQL injection attempt",
			input: "'; DROP TABLE cards; --",
			game:  "pokemon",
		},
		{
			name:  "Extremely long single line",
			input: strings.Repeat("Pikachu ", 1000),
			game:  "pokemon",
		},
		{
			name:  "Mixed newline types",
			input: "Pikachu\r\nHP 60\rBasic\n025/185",
			game:  "pokemon",
		},
		{
			name:  "Tab characters",
			input: "Pikachu\tV\tHP\t190",
			game:  "pokemon",
		},
		{
			name:  "Case variations in game type",
			input: "Pikachu\nHP 60",
			game:  "POKEMON",
		},
		{
			name:  "Unknown game type",
			input: "Blue-Eyes White Dragon\nATK 3000",
			game:  "yugioh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			result := ParseOCRText(tt.input, tt.game)

			if result == nil {
				t.Error("Result should not be nil")
			}

			// Verify text is truncated if too long
			if len(tt.input) > maxOCRTextLength && len(result.RawText) > maxOCRTextLength {
				t.Errorf("RawText length = %d, should be truncated to %d", len(result.RawText), maxOCRTextLength)
			}
		})
	}
}

// TestParseOCRTextFromSampleFiles tests parsing OCR text from sample files
func TestParseOCRTextFromSampleFiles(t *testing.T) {
	// Test Pokemon samples
	pokemonSamples := []struct {
		name     string
		expected struct {
			cardName   string
			setCode    string
			cardNumber string
			hp         string
		}
	}{
		{
			name: "Charizard VMAX",
			expected: struct {
				cardName   string
				setCode    string
				cardNumber string
				hp         string
			}{
				cardName:   "Charizard VMAX",
				setCode:    "swsh3",
				cardNumber: "20",
				hp:         "330",
			},
		},
	}

	for _, tt := range pokemonSamples {
		t.Run("Pokemon_"+tt.name, func(t *testing.T) {
			input := `Charizard VMAX
HP 330
Fire
Darkness Ablaze
020/189
SWSH3`
			result := ParseOCRText(input, "pokemon")

			if result.CardName != tt.expected.cardName {
				t.Errorf("CardName = %q, want %q", result.CardName, tt.expected.cardName)
			}
			if result.SetCode != tt.expected.setCode {
				t.Errorf("SetCode = %q, want %q", result.SetCode, tt.expected.setCode)
			}
			if result.CardNumber != tt.expected.cardNumber {
				t.Errorf("CardNumber = %q, want %q", result.CardNumber, tt.expected.cardNumber)
			}
			if result.HP != tt.expected.hp {
				t.Errorf("HP = %q, want %q", result.HP, tt.expected.hp)
			}
		})
	}
}

// TestBaseSetCardExtraction tests OCR parsing for classic Base Set era cards
func TestBaseSetCardExtraction(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantCardNumber string
		wantSetCode    string
		wantCardName   string
		wantHP         string
		wantIsFoil     bool
		minConfidence  float64
	}{
		{
			name: "Base Set Charizard Holo",
			input: `Charizard
HP 120
Stage 2
Evolves from Charmeleon
Fire Spin
4/102
©1999 Wizards`,
			wantCardNumber: "4",
			wantSetCode:    "base1",
			wantCardName:   "Charizard",
			wantHP:         "120",
			minConfidence:  0.7,
		},
		{
			name: "Base Set Blastoise",
			input: `Blastoise
HP 100
Stage 2
Water
Rain Dance
2/102`,
			wantCardNumber: "2",
			wantSetCode:    "base1",
			wantCardName:   "Blastoise",
			wantHP:         "100",
			minConfidence:  0.7,
		},
		{
			name: "Base Set Venusaur",
			input: `Venusaur
HP 100
Stage 2
Grass
Energy Trans
15/102`,
			wantCardNumber: "15",
			wantSetCode:    "base1",
			wantCardName:   "Venusaur",
			wantHP:         "100",
			minConfidence:  0.7,
		},
		{
			name: "Base Set Pikachu",
			input: `Pikachu
HP 40
Basic
Lightning
58/102`,
			wantCardNumber: "58",
			wantSetCode:    "base1",
			wantCardName:   "Pikachu",
			wantHP:         "40",
			minConfidence:  0.7,
		},
		{
			name: "Jungle Jolteon Holo",
			input: `Jolteon
HP 70
Stage 1
Evolves from Eevee
Lightning
4/64
Jungle`,
			wantCardNumber: "4",
			wantSetCode:    "base2",
			wantCardName:   "Jolteon",
			wantHP:         "70",
			minConfidence:  0.7,
		},
		{
			name: "Fossil Gengar Holo",
			input: `Gengar
HP 80
Stage 2
Psychic
Curse
5/62`,
			wantCardNumber: "5",
			wantSetCode:    "base3",
			wantCardName:   "Gengar",
			wantHP:         "80",
			minConfidence:  0.7,
		},
		{
			name: "Team Rocket Dark Charizard",
			input: `Dark Charizard
HP 80
Stage 2
Fire
Team Rocket
4/82`,
			wantCardNumber: "4",
			wantSetCode:    "base5",
			wantCardName:   "Dark Charizard",
			wantHP:         "80",
			minConfidence:  0.6,
		},
		{
			name: "Neo Genesis Lugia",
			input: `Lugia
HP 90
Basic
Psychic
Neo Genesis
9/111`,
			wantCardNumber: "9",
			wantSetCode:    "neo1",
			wantCardName:   "Lugia",
			wantHP:         "90",
			minConfidence:  0.7,
		},
		{
			name: "Base Set card with PTCGO code",
			input: `Alakazam
HP 80
Stage 2
Psychic
BS
1/102`,
			wantCardNumber: "1",
			wantSetCode:    "base1",
			wantCardName:   "Alakazam",
			wantHP:         "80",
			minConfidence:  0.7,
		},
		{
			name: "Jungle card with PTCGO code",
			input: `Scyther
HP 70
Basic
Grass
JU
10/64`,
			wantCardNumber: "10",
			wantSetCode:    "base2",
			wantCardName:   "Scyther",
			wantHP:         "70",
			minConfidence:  0.7,
		},
		{
			name: "Fossil card with PTCGO code",
			input: `Dragonite
HP 100
Stage 2
Colorless
FO
4/62`,
			wantCardNumber: "4",
			wantSetCode:    "base3",
			wantCardName:   "Dragonite",
			wantHP:         "100",
			minConfidence:  0.7,
		},
		{
			name: "Base Set with set name text",
			input: `Mewtwo
HP 60
Basic
Psychic
Base Set
10/102`,
			wantCardNumber: "10",
			wantSetCode:    "base1",
			wantCardName:   "Mewtwo",
			wantHP:         "60",
			minConfidence:  0.7,
		},
		{
			name: "Gym Heroes Lt. Surge's Electabuzz",
			input: `Lt. Surge's Electabuzz
HP 70
Basic
Lightning
Gym Heroes
27/132`,
			wantCardNumber: "27",
			wantSetCode:    "gym1",
			wantCardName:   "Lt. Surge's Electabuzz",
			wantHP:         "70",
			minConfidence:  0.5,
		},
		{
			name: "Neo Discovery Umbreon",
			input: `Umbreon
HP 70
Stage 1
Darkness
Neo Discovery
32/75`,
			wantCardNumber: "32",
			wantSetCode:    "neo2",
			wantCardName:   "Umbreon",
			wantHP:         "70",
			minConfidence:  0.7,
		},
		{
			name: "Base Set with noisy OCR",
			input: `Machamp
H P 1 0 0
Stage 2
Fighting
8/l02`,
			// Note: l/1 confusion in number, should still extract name
			wantCardName:  "Machamp",
			minConfidence: 0.4,
		},
		{
			name: "Legendary Collection Charizard",
			input: `Charizard
HP 120
Stage 2
Fire
Legendary Collection
3/110`,
			wantCardNumber: "3",
			wantSetCode:    "base6",
			wantCardName:   "Charizard",
			wantHP:         "120",
			minConfidence:  0.7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseOCRText(tt.input, "pokemon")

			if result == nil {
				t.Fatal("Result should not be nil")
			}

			if tt.wantCardNumber != "" && result.CardNumber != tt.wantCardNumber {
				t.Errorf("CardNumber = %q, want %q", result.CardNumber, tt.wantCardNumber)
			}

			if tt.wantSetCode != "" && result.SetCode != tt.wantSetCode {
				t.Errorf("SetCode = %q, want %q", result.SetCode, tt.wantSetCode)
			}

			if tt.wantCardName != "" && result.CardName != tt.wantCardName {
				t.Errorf("CardName = %q, want %q", result.CardName, tt.wantCardName)
			}

			if tt.wantHP != "" && result.HP != tt.wantHP {
				t.Errorf("HP = %q, want %q", result.HP, tt.wantHP)
			}

			if tt.wantIsFoil && !result.IsFoil {
				t.Errorf("IsFoil = %v, want %v", result.IsFoil, tt.wantIsFoil)
			}

			if tt.minConfidence > 0 && result.Confidence < tt.minConfidence {
				t.Errorf("Confidence = %v, want >= %v", result.Confidence, tt.minConfidence)
			}
		})
	}
}

// TestPTCGOCodeDetection tests detection of PTCGO 2-letter set codes
func TestPTCGOCodeDetection(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantSetCode string
	}{
		{
			name:        "Base Set code BS",
			input:       "Pikachu\nBS\n58/102",
			wantSetCode: "base1",
		},
		{
			name:        "Jungle code JU",
			input:       "Eevee\nJU\n51/64",
			wantSetCode: "base2",
		},
		{
			name:        "Fossil code FO",
			input:       "Aerodactyl\nFO\n1/62",
			wantSetCode: "base3",
		},
		{
			name:        "Team Rocket code TR",
			input:       "Dark Dragonite\nTR\n5/82",
			wantSetCode: "base5",
		},
		{
			name:        "Gym Heroes code G1",
			input:       "Brock's Rhydon\nG1\n2/132",
			wantSetCode: "gym1",
		},
		{
			name:        "Gym Challenge code G2",
			input:       "Blaine's Charizard\nG2\n2/132",
			wantSetCode: "gym2",
		},
		{
			name:        "Neo Genesis code N1",
			input:       "Feraligatr\nN1\n4/111",
			wantSetCode: "neo1",
		},
		{
			name:        "Legendary Collection code LC",
			input:       "Mewtwo\nLC\n29/110",
			wantSetCode: "base6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseOCRText(tt.input, "pokemon")

			if result.SetCode != tt.wantSetCode {
				t.Errorf("SetCode = %q, want %q", result.SetCode, tt.wantSetCode)
			}
		})
	}
}

// TestConservativeFoilDetection tests that foil detection is conservative
// Card types (V, VMAX, VSTAR, GX, EX) should NOT trigger foil
// Only explicit foil text (HOLO, FOIL, REVERSE HOLO) should trigger foil
func TestConservativeFoilDetection(t *testing.T) {
	tests := []struct {
		name               string
		input              string
		game               string
		wantIsFoil         bool
		wantFoilConfidence float64
		wantIndicatorCount int
	}{
		{
			name:               "Pikachu V - should NOT be foil",
			input:              "Pikachu V\nHP 190\n025/185",
			game:               "pokemon",
			wantIsFoil:         false,
			wantFoilConfidence: 0,
			wantIndicatorCount: 0,
		},
		{
			name:               "Charizard VMAX - should NOT be foil",
			input:              "Charizard VMAX\nHP 330\n020/189",
			game:               "pokemon",
			wantIsFoil:         false,
			wantFoilConfidence: 0,
			wantIndicatorCount: 0,
		},
		{
			name:               "Arceus VSTAR - should NOT be foil",
			input:              "Arceus VSTAR\nHP 280\n123/172",
			game:               "pokemon",
			wantIsFoil:         false,
			wantFoilConfidence: 0,
			wantIndicatorCount: 0,
		},
		{
			name:               "Mewtwo GX - should NOT be foil",
			input:              "Mewtwo GX\nHP 190\n072/073",
			game:               "pokemon",
			wantIsFoil:         false,
			wantFoilConfidence: 0,
			wantIndicatorCount: 0,
		},
		{
			name:               "Charizard EX - should NOT be foil",
			input:              "Charizard EX\nHP 180\n011/108",
			game:               "pokemon",
			wantIsFoil:         false,
			wantFoilConfidence: 0,
			wantIndicatorCount: 0,
		},
		{
			name:               "Charizard ex (lowercase) - should NOT be foil",
			input:              "Charizard ex\nHP 330\n006/091",
			game:               "pokemon",
			wantIsFoil:         false,
			wantFoilConfidence: 0,
			wantIndicatorCount: 0,
		},
		{
			name:               "Pikachu with Holo text - SHOULD be foil",
			input:              "Pikachu\nHolo\nHP 60\n058/102",
			game:               "pokemon",
			wantIsFoil:         true,
			wantFoilConfidence: 0.9,
			wantIndicatorCount: 1,
		},
		{
			name:               "Reverse Holo card - SHOULD be foil",
			input:              "Bulbasaur\nReverse Holo\nHP 70\n001/198",
			game:               "pokemon",
			wantIsFoil:         true,
			wantFoilConfidence: 0.9,
			wantIndicatorCount: 1,
		},
		{
			name:               "Holo Rare text - SHOULD be foil",
			input:              "Umbreon\nHolo Rare\nHP 110\n095/203",
			game:               "pokemon",
			wantIsFoil:         true,
			wantFoilConfidence: 0.9,
			wantIndicatorCount: 1,
		},
		{
			name:               "VMAX with Rainbow Rare - medium confidence only",
			input:              "Pikachu VMAX\nRainbow Rare\nHP 310\n188/185",
			game:               "pokemon",
			wantIsFoil:         false, // Rainbow alone doesn't trigger foil
			wantFoilConfidence: 0.6,
			wantIndicatorCount: 1,
		},
		{
			name:               "Gold card - medium confidence only",
			input:              "Switch\nGold\nTrainer - Item\n163/159",
			game:               "pokemon",
			wantIsFoil:         false, // Gold alone doesn't trigger foil
			wantFoilConfidence: 0.6,
			wantIndicatorCount: 1,
		},
		{
			name:               "Secret rare - medium confidence only",
			input:              "Energy\nSecret\n188/185",
			game:               "pokemon",
			wantIsFoil:         false, // Secret alone doesn't trigger foil
			wantFoilConfidence: 0.6,
			wantIndicatorCount: 1,
		},
		{
			name:               "Regular Pokemon - not foil",
			input:              "Bulbasaur\nHP 70\nBasic\n001/185",
			game:               "pokemon",
			wantIsFoil:         false,
			wantFoilConfidence: 0,
			wantIndicatorCount: 0,
		},
		{
			name:               "MTG with FOIL text - SHOULD be foil",
			input:              "Lightning Bolt\nFoil\nInstant\n073/303",
			game:               "mtg",
			wantIsFoil:         true,
			wantFoilConfidence: 0, // MTG uses different detection path
			wantIndicatorCount: 1,
		},
		{
			name:               "MTG with ETCHED text - SHOULD be foil",
			input:              "Sol Ring\nEtched\nArtifact",
			game:               "mtg",
			wantIsFoil:         true,
			wantFoilConfidence: 0, // MTG uses different detection
			wantIndicatorCount: 1,
		},
		{
			name:               "Regular MTG card - not foil",
			input:              "Island\nBasic Land - Island",
			game:               "mtg",
			wantIsFoil:         false,
			wantFoilConfidence: 0,
			wantIndicatorCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseOCRText(tt.input, tt.game)

			if result.IsFoil != tt.wantIsFoil {
				t.Errorf("IsFoil = %v, want %v (indicators: %v)", result.IsFoil, tt.wantIsFoil, result.FoilIndicators)
			}

			if result.FoilConfidence != tt.wantFoilConfidence {
				t.Errorf("FoilConfidence = %v, want %v", result.FoilConfidence, tt.wantFoilConfidence)
			}

			if len(result.FoilIndicators) != tt.wantIndicatorCount {
				t.Errorf("FoilIndicators count = %d, want %d (indicators: %v)",
					len(result.FoilIndicators), tt.wantIndicatorCount, result.FoilIndicators)
			}
		})
	}
}

// TestFirstEditionDetection tests detection of 1st Edition Pokemon cards
func TestFirstEditionDetection(t *testing.T) {
	tests := []struct {
		name                  string
		input                 string
		wantIsFirstEdition    bool
		wantFirstEdIndicators int
	}{
		{
			name:                  "1ST EDITION text",
			input:                 "Charizard\n1ST EDITION\nHP 120\n4/102",
			wantIsFirstEdition:    true,
			wantFirstEdIndicators: 1,
		},
		{
			name:                  "1ST ED abbreviation",
			input:                 "Blastoise\n1ST ED\nHP 100\n2/102",
			wantIsFirstEdition:    true,
			wantFirstEdIndicators: 1,
		},
		{
			name:                  "FIRST EDITION text",
			input:                 "Venusaur\nFIRST EDITION\nHP 100\n15/102",
			wantIsFirstEdition:    true,
			wantFirstEdIndicators: 1,
		},
		{
			name:                  "Regular Base Set card - no first edition",
			input:                 "Pikachu\nHP 40\n58/102",
			wantIsFirstEdition:    false,
			wantFirstEdIndicators: 0,
		},
		{
			name:                  "Modern card - no first edition",
			input:                 "Charizard VMAX\nHP 330\n020/189",
			wantIsFirstEdition:    false,
			wantFirstEdIndicators: 0,
		},
		{
			name:                  "Shadowless card - indicator but not first edition",
			input:                 "Charizard\nShadowless\nHP 120\n4/102",
			wantIsFirstEdition:    false,
			wantFirstEdIndicators: 1, // Shadowless is an indicator for verification
		},
		{
			name:                  "1st Edition with Holo",
			input:                 "Alakazam\n1ST EDITION\nHolo\nHP 80\n1/102",
			wantIsFirstEdition:    true,
			wantFirstEdIndicators: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseOCRText(tt.input, "pokemon")

			if result.IsFirstEdition != tt.wantIsFirstEdition {
				t.Errorf("IsFirstEdition = %v, want %v", result.IsFirstEdition, tt.wantIsFirstEdition)
			}

			if len(result.FirstEdIndicators) != tt.wantFirstEdIndicators {
				t.Errorf("FirstEdIndicators count = %d, want %d (indicators: %v)",
					len(result.FirstEdIndicators), tt.wantFirstEdIndicators, result.FirstEdIndicators)
			}
		})
	}
}

// TestWotCEraDetection tests detection of Wizards of the Coast era cards
func TestWotCEraDetection(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantIsWotC  bool
		wantSetCode string
	}{
		{
			name: "Wizards copyright text",
			input: `Charizard
HP 120
4/102
©1999 Wizards of the Coast`,
			wantIsWotC:  true,
			wantSetCode: "base1",
		},
		{
			name: "Wizards abbreviated",
			input: `Blastoise
HP 100
2/102
WIZARDS`,
			wantIsWotC:  true,
			wantSetCode: "base1",
		},
		{
			name: "1999 copyright year",
			input: `Venusaur
HP 100
15/102
©1999`,
			wantIsWotC:  true,
			wantSetCode: "base1",
		},
		{
			name: "OCR error W1ZARDS",
			input: `Pikachu
HP 40
58/102
W1ZARDS`,
			wantIsWotC:  true,
			wantSetCode: "base1",
		},
		{
			name: "Set total 102 with no modern indicators",
			input: `Alakazam
HP 80
1/102`,
			wantIsWotC:  true,
			wantSetCode: "base1",
		},
		{
			name: "Set total 64 (Jungle)",
			input: `Jolteon
HP 70
4/64`,
			wantIsWotC:  true,
			wantSetCode: "base2", // Jungle
		},
		{
			name: "Set total 62 (Fossil)",
			input: `Gengar
HP 80
5/62`,
			wantIsWotC:  true,
			wantSetCode: "base3", // Fossil
		},
		{
			name: "Modern card - not WotC era",
			input: `Charizard VMAX
HP 330
SWSH3
020/189`,
			wantIsWotC:  false,
			wantSetCode: "swsh3",
		},
		{
			name: "Modern set total 185 - not WotC era",
			input: `Pikachu
HP 60
025/185`,
			wantIsWotC:  false,
			wantSetCode: "swsh4",
		},
		{
			name: "Ambiguous total 102 with Wizards copyright - should be base1 not hgss4",
			input: `Raichu
HP 90
14/102
©1999 Wizards`,
			wantIsWotC:  true,
			wantSetCode: "base1", // Not hgss4
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseOCRText(tt.input, "pokemon")

			if result.IsWotCEra != tt.wantIsWotC {
				t.Errorf("IsWotCEra = %v, want %v", result.IsWotCEra, tt.wantIsWotC)
			}

			if tt.wantSetCode != "" && result.SetCode != tt.wantSetCode {
				t.Errorf("SetCode = %q, want %q", result.SetCode, tt.wantSetCode)
			}
		})
	}
}

// TestOCRNameCorrection tests OCR error correction for Pokemon names
func TestOCRNameCorrection(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantCardName string
	}{
		{
			name: "Charizard with 1 instead of i",
			input: `Char1zard
HP 120
4/102`,
			wantCardName: "Charizard",
		},
		{
			name: "Alakazam with rn instead of m",
			input: `Alakazarn
HP 80
1/102`,
			wantCardName: "Alakazam",
		},
		{
			name: "Pikachu with 1 instead of i",
			input: `P1kachu
HP 40
58/102`,
			wantCardName: "Pikachu",
		},
		{
			name: "Mewtwo with 0 instead of o",
			input: `Mewtw0
HP 60
10/102`,
			wantCardName: "Mewtwo",
		},
		{
			name: "Machamp with rn instead of m",
			input: `Macharnp
HP 100
8/102`,
			wantCardName: "Machamp",
		},
		{
			name: "Gyarados with 0 instead of o",
			input: `Gyarad0s
HP 100
6/102`,
			wantCardName: "Gyarados",
		},
		{
			name: "Dragonite with 1 instead of i",
			input: `Dragon1te
HP 100
4/62`,
			wantCardName: "Dragonite",
		},
		{
			name: "Gengar with q instead of g",
			input: `Genqar
HP 80
5/62`,
			wantCardName: "Gengar",
		},
		{
			name: "Snorlax with space",
			input: `Snorl ax
HP 90
11/64`,
			wantCardName: "Snorlax",
		},
		{
			name: "Blastoise with 1 instead of i",
			input: `Blasto1se
HP 100
2/102`,
			wantCardName: "Blastoise",
		},
		{
			name: "Clean name should not change",
			input: `Charizard
HP 120
4/102`,
			wantCardName: "Charizard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseOCRText(tt.input, "pokemon")

			if result.CardName != tt.wantCardName {
				t.Errorf("CardName = %q, want %q", result.CardName, tt.wantCardName)
			}
		})
	}
}

// TestFuzzyMatchPokemonName tests the fuzzy matching function for Pokemon names with OCR errors
func TestFuzzyMatchPokemonName(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantMatch string
		wantFound bool
	}{
		{
			name:      "Exact match",
			input:     "charizard",
			wantMatch: "charizard",
			wantFound: true,
		},
		{
			name:      "One character off - Char1zard",
			input:     "char1zard",
			wantMatch: "charizard",
			wantFound: true,
		},
		{
			name:      "One character off - Charizarc",
			input:     "charizarc",
			wantMatch: "charizard",
			wantFound: true,
		},
		{
			name:      "Two characters off - Char1zaro",
			input:     "char1zaro",
			wantMatch: "charizard",
			wantFound: true,
		},
		{
			name:      "OCR error - Blastolse",
			input:     "blastolse",
			wantMatch: "blastoise",
			wantFound: true,
		},
		{
			name:      "OCR error - Plkachu",
			input:     "plkachu",
			wantMatch: "pikachu",
			wantFound: true,
		},
		{
			name:      "OCR error - P1kachu",
			input:     "p1kachu",
			wantMatch: "pikachu",
			wantFound: true,
		},
		{
			name:      "OCR error - Alakazarn (rn->m)",
			input:     "alakazarn",
			wantMatch: "alakazam",
			wantFound: true,
		},
		{
			name:      "OCR error - Genqar (q->g)",
			input:     "genqar",
			wantMatch: "gengar",
			wantFound: true,
		},
		{
			name:      "OCR error - Venusaur (correct)",
			input:     "venusaur",
			wantMatch: "venusaur",
			wantFound: true,
		},
		{
			name:      "OCR error - Mewtw0 (0->o)",
			input:     "mewtw0",
			wantMatch: "mewtwo",
			wantFound: true,
		},
		{
			name:      "Too many errors - no match",
			input:     "xyzabc",
			wantMatch: "",
			wantFound: false,
		},
		{
			name:      "Short string - no match",
			input:     "ab",
			wantMatch: "",
			wantFound: false,
		},
		{
			name:      "OCR error - Snorl4x (4->a)",
			input:     "snorl4x",
			wantMatch: "snorlax",
			wantFound: true,
		},
		{
			name:      "OCR error - Gyarad0s (0->o)",
			input:     "gyarad0s",
			wantMatch: "gyarados",
			wantFound: true,
		},
		{
			name:      "OCR error - Dragonlte (l->i)",
			input:     "dragonlte",
			wantMatch: "dragonite",
			wantFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, found := fuzzyMatchPokemonName(tt.input)
			if found != tt.wantFound {
				t.Errorf("fuzzyMatchPokemonName(%q) found = %v, want %v", tt.input, found, tt.wantFound)
			}
			if match != tt.wantMatch {
				t.Errorf("fuzzyMatchPokemonName(%q) match = %q, want %q", tt.input, match, tt.wantMatch)
			}
		})
	}
}

// TestBaseSetRealWorldOCR tests realistic OCR output from Base Set card scans
func TestBaseSetRealWorldOCR(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantCardNumber string
		wantSetCode    string
		wantCardName   string
		wantHP         string
		wantIsWotC     bool
		minConfidence  float64
	}{
		{
			name: "Base Set Charizard - clean scan",
			input: `Charizard
HP 120
Stage 2 Evolves from Charmeleon
Fire Spin 100
Discard 2 Energy cards attached to Charizard
4/102
Illus. Mitsuhiro Arita
©1995, 96, 98, 99 Wizards of the Coast`,
			wantCardNumber: "4",
			wantSetCode:    "base1",
			wantCardName:   "Charizard",
			wantHP:         "120",
			wantIsWotC:     true,
			minConfidence:  0.7,
		},
		{
			name: "Base Set Blastoise - moderate OCR quality",
			input: `Blasto1se
HP l00
Stage 2 Evolves from Wartoitle
Rain Dance
2/l02
©1999 Wizards`,
			wantCardNumber: "2",
			wantSetCode:    "base1",
			wantCardName:   "Blastoise",
			wantIsWotC:     true,
			minConfidence:  0.5,
		},
		{
			name: "Base Set Venusaur - noisy scan",
			input: `Venusaur
HP 100
Energy Trans
Pokémon Power
15/102
WIZARDS OF THE COAST`,
			wantCardNumber: "15",
			wantSetCode:    "base1",
			wantCardName:   "Venusaur",
			wantHP:         "100",
			wantIsWotC:     true,
			minConfidence:  0.7,
		},
		{
			name: "Jungle Scyther",
			input: `Scyther
HP 70
Basic Pokémon
Grass
Swords Dance
Slash 30
10/64
©1999 Wizards`,
			wantCardNumber: "10",
			wantSetCode:    "base2",
			wantCardName:   "Scyther",
			wantHP:         "70",
			wantIsWotC:     true,
			minConfidence:  0.7,
		},
		{
			name: "Fossil Aerodactyl",
			input: `Aerodactyl
HP 60
Stage 1 Evolves from Mysterious Fossil
Prehistoric Power
1/62
Fossil`,
			wantCardNumber: "1",
			wantSetCode:    "base3",
			wantCardName:   "Aerodactyl",
			wantHP:         "60",
			wantIsWotC:     true,
			minConfidence:  0.7,
		},
		{
			name: "Team Rocket Dark Charizard - noisy",
			input: `Dark Chari zard
HP 80
Stage 2
Fire
Team Rocket
4/82`,
			wantCardNumber: "4",
			wantSetCode:    "base5",
			wantCardName:   "Dark Charizard",
			wantHP:         "80",
			wantIsWotC:     true,
			minConfidence:  0.5,
		},
		{
			name: "Neo Genesis Lugia - clean",
			input: `Lugia
HP 90
Basic Pokémon
Psychic
Aeroblast
Neo Genesis
9/111`,
			wantCardNumber: "9",
			wantSetCode:    "neo1",
			wantCardName:   "Lugia",
			wantHP:         "90",
			wantIsWotC:     true,
			minConfidence:  0.7,
		},
		{
			name: "First Edition Base Set Alakazam",
			input: `Alakazarn
HP 80
Stage 2
Psychic
Damage Swap
1ST EDITION
1/102
Wizards`,
			wantCardNumber: "1",
			wantSetCode:    "base1",
			wantCardName:   "Alakazam",
			wantHP:         "80",
			wantIsWotC:     true,
			minConfidence:  0.5,
		},
		{
			name: "Gym Heroes Lt. Surge's Electabuzz",
			input: `Lt. Surge's Electabuzz
HP 70
Basic Pokémon
Lightning
Gym Heroes
27/132`,
			wantCardNumber: "27",
			wantSetCode:    "gym1",
			wantCardName:   "Lt. Surge's Electabuzz",
			wantHP:         "70",
			wantIsWotC:     true,
			minConfidence:  0.5,
		},
		{
			name: "Base Set Pikachu - poor quality scan",
			input: `P1kachu
w 40
Basic
Lightning
58/l02
W1ZARDS`,
			wantCardNumber: "58",
			wantSetCode:    "base1",
			wantCardName:   "Pikachu",
			wantIsWotC:     true,
			minConfidence:  0.3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseOCRText(tt.input, "pokemon")

			if result == nil {
				t.Fatal("Result should not be nil")
			}

			if tt.wantCardNumber != "" && result.CardNumber != tt.wantCardNumber {
				t.Errorf("CardNumber = %q, want %q", result.CardNumber, tt.wantCardNumber)
			}

			if tt.wantSetCode != "" && result.SetCode != tt.wantSetCode {
				t.Errorf("SetCode = %q, want %q", result.SetCode, tt.wantSetCode)
			}

			if tt.wantCardName != "" && result.CardName != tt.wantCardName {
				t.Errorf("CardName = %q, want %q", result.CardName, tt.wantCardName)
			}

			if tt.wantHP != "" && result.HP != tt.wantHP {
				t.Errorf("HP = %q, want %q", result.HP, tt.wantHP)
			}

			if result.IsWotCEra != tt.wantIsWotC {
				t.Errorf("IsWotCEra = %v, want %v", result.IsWotCEra, tt.wantIsWotC)
			}

			if tt.minConfidence > 0 && result.Confidence < tt.minConfidence {
				t.Errorf("Confidence = %v, want >= %v", result.Confidence, tt.minConfidence)
			}
		})
	}
}

// TestImageAnalysisConservativeFoil tests that image analysis uses conservative foil detection
func TestImageAnalysisConservativeFoil(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		analysis   *ImageAnalysis
		wantIsFoil bool
	}{
		{
			name:  "High confidence (0.85) image foil detection - SHOULD set foil",
			input: "Pikachu\nHP 60\n025/185",
			analysis: &ImageAnalysis{
				IsFoilDetected: true,
				FoilConfidence: 0.85,
			},
			wantIsFoil: true,
		},
		{
			name:  "Medium confidence (0.65) image foil detection - should NOT set foil",
			input: "Pikachu\nHP 60\n025/185",
			analysis: &ImageAnalysis{
				IsFoilDetected: true,
				FoilConfidence: 0.65,
			},
			wantIsFoil: false,
		},
		{
			name:  "Low confidence (0.3) image foil detection - should NOT set foil",
			input: "Pikachu\nHP 60\n025/185",
			analysis: &ImageAnalysis{
				IsFoilDetected: true,
				FoilConfidence: 0.3,
			},
			wantIsFoil: false,
		},
		{
			name:  "Exactly 0.8 confidence - SHOULD set foil",
			input: "Pikachu\nHP 60\n025/185",
			analysis: &ImageAnalysis{
				IsFoilDetected: true,
				FoilConfidence: 0.8,
			},
			wantIsFoil: true,
		},
		{
			name:  "Just under 0.8 (0.79) - should NOT set foil",
			input: "Pikachu\nHP 60\n025/185",
			analysis: &ImageAnalysis{
				IsFoilDetected: true,
				FoilConfidence: 0.79,
			},
			wantIsFoil: false,
		},
		{
			name:  "High confidence but IsFoilDetected is false",
			input: "Pikachu\nHP 60\n025/185",
			analysis: &ImageAnalysis{
				IsFoilDetected: false,
				FoilConfidence: 0.9,
			},
			wantIsFoil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseOCRTextWithAnalysis(tt.input, "pokemon", tt.analysis)

			if result.IsFoil != tt.wantIsFoil {
				t.Errorf("IsFoil = %v, want %v (FoilConfidence: %v, indicators: %v)",
					result.IsFoil, tt.wantIsFoil, result.FoilConfidence, result.FoilIndicators)
			}
		})
	}
}

// TestRealWorldBaseSetOCRFromImages tests OCR parsing with actual Tesseract output from card scans
func TestRealWorldBaseSetOCRFromImages(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantCardName   string
		wantCardNumber string
		wantSetCode    string
		wantIsWotC     bool
	}{
		{
			name: "Base Set Pikachu - realistic OCR output with artifacts",
			input: `Thunder Jolt Flip a coin. If
4) \¥) tails, Pikachu does 10 damage 30
to itself.

weakness resistance retreat cost

When several of these Pokémon gather, their electricity ean
cause lightning storm< IV. 12 #25

Mus. Mitsuhiro Arita ©1995, 96, 96 lic hSE crower: 581102 @`,
			wantCardName:   "Pikachu",
			wantCardNumber: "",   // Card number format is corrupted
			wantSetCode:    "",   // No clear set code
			wantIsWotC:     true, // Should detect ©1995
		},
		{
			name: "Base Set Pikachu - cleaned OCR output",
			input: `Thunder Jolt Flip a coin. If
tails, Pikachu does 10 damage 30
to itself.

weakness resistance retreat cost

When several of these Pokémon gather, their electricity can
cause lightning storms. IV. 12 #25

Illus. Mitsuhiro Arita ©1995, 96, 98 Nintendo. 58/102`,
			wantCardName:   "Pikachu",
			wantCardNumber: "58",
			wantSetCode:    "base1",
			wantIsWotC:     true,
		},
		{
			name: "Base Set Charizard - partial OCR from holo card",
			input: `rey a ae
rest of the turn. This power can't be used if Charizard
is Asleep, Confused, or Paralyzed.

Fire Spin Discard 2 Energy cards 100
attached to Charizard in order to use

4/102
©1999 Wizards`,
			wantCardName:   "Charizard",
			wantCardNumber: "4",
			wantSetCode:    "base1",
			wantIsWotC:     true,
		},
		{
			name: "Jungle Scyther - real Tesseract OCR output",
			input: `Basic Pokémon

Scyther

Mantis Pokémon. Length: 4' ||", Weight: 123 Ibs. G23

Swords Dance During your next
turn, Scyther's Slash attack's base damage
is 60 instead of 30.

Slash 30

weakness, resistance retreat cost

With ninja-like agility and speed, it can create the illusion
that there is more than one of it. LV.25 #123

Ken Sugimori ©1999 Nintendo. 10/64`,
			wantCardName:   "Scyther",
			wantCardNumber: "10",
			wantSetCode:    "base2", // Jungle
			wantIsWotC:     true,
		},
		{
			name: "Fossil Gengar - real Tesseract OCR output",
			input: `Shadow Pokémon. Length: 4' 11", Weight: 89 Ibs.
Pokémon Power: Curse Once during your turn
(before your attack), you may move 1 damage counter from 1
of your opponent's Pokémon to another (even if it would
Knock Out the other Pokémon). This power can't be used if
Gengar is Asleep, Confused, or Paralyzed.

Dark Mind If your opponent has any
Benched Pokémon, choose 1 of them and this 30
attack does 10 damage to it.
weakness resistance retreat cost

5/62
©1999 Wizards`,
			wantCardName:   "Gengar",
			wantCardNumber: "5",
			wantSetCode:    "base3", // Fossil
			wantIsWotC:     true,
		},
		{
			name: "Team Rocket Dark Charizard - real Tesseract OCR",
			input: `Dark Charizard
Nail Flick 10
Continuous Fireball Flip a number
of coins equal to the number of Fire Energy
cards attached to Dark Charizard

weakness resistance retreat cost

Seemingly peaceful, it can burn all it sees.
Ken Sugimori ©1995, 96, 98 Nintendo, Creatures, GAMEFREAK. ©1999 2000 Wizards 4/82`,
			wantCardName:   "Dark Charizard",
			wantCardNumber: "4",
			wantSetCode:    "base5", // Team Rocket
			wantIsWotC:     true,
		},
		{
			name: "Neo Genesis Lugia - real Tesseract OCR",
			input: `Elemental Blast Discard a
Water Energy card, a Fire Energy
card, and a Lightning Energy card 90
attached to Lugia in order to
use this attack.

weakness resistance retreat cost

It is said that it quietly spends its time deep at the bottom of
the sea, because its powers are too strong. LV. 45 #249

9/111
Neo Genesis`,
			wantCardName:   "Lugia",
			wantCardNumber: "9",
			wantSetCode:    "neo1", // Neo Genesis
			wantIsWotC:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseOCRText(tt.input, "pokemon")

			if result == nil {
				t.Fatal("Result should not be nil")
			}

			if tt.wantCardName != "" && result.CardName != tt.wantCardName {
				t.Errorf("CardName = %q, want %q", result.CardName, tt.wantCardName)
			}

			if tt.wantCardNumber != "" && result.CardNumber != tt.wantCardNumber {
				t.Errorf("CardNumber = %q, want %q", result.CardNumber, tt.wantCardNumber)
			}

			if tt.wantSetCode != "" && result.SetCode != tt.wantSetCode {
				t.Errorf("SetCode = %q, want %q", result.SetCode, tt.wantSetCode)
			}

			if result.IsWotCEra != tt.wantIsWotC {
				t.Errorf("IsWotCEra = %v, want %v", result.IsWotCEra, tt.wantIsWotC)
			}

			t.Logf("Result: CardName=%q, CardNumber=%q, SetCode=%q, IsWotCEra=%v, Confidence=%.2f",
				result.CardName, result.CardNumber, result.SetCode, result.IsWotCEra, result.Confidence)
		})
	}
}

// TestSetDetectionAllEras tests set detection across all Pokemon TCG eras
func TestSetDetectionAllEras(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantSetCode    string
		wantCardNumber string
		description    string
	}{
		// ==================== WotC Era (1999-2003) ====================
		{
			name:           "Base Set - by total 102",
			input:          "Charizard\nHP 120\n4/102\n©1999 Wizards",
			wantSetCode:    "base1",
			wantCardNumber: "4",
			description:    "Base Set via 102 total + Wizards copyright",
		},
		{
			name:           "Jungle - by total 64",
			input:          "Scyther\nHP 70\n10/64\n©1999 Wizards",
			wantSetCode:    "base2",
			wantCardNumber: "10",
			description:    "Jungle via 64 total + Wizards copyright",
		},
		{
			name:           "Fossil - by total 62",
			input:          "Gengar\nHP 80\n5/62\n©1999 Wizards",
			wantSetCode:    "base3",
			wantCardNumber: "5",
			description:    "Fossil via 62 total",
		},
		{
			name:           "Team Rocket - by total 82",
			input:          "Dark Charizard\nHP 80\n4/82\n©2000 Wizards",
			wantSetCode:    "base5",
			wantCardNumber: "4",
			description:    "Team Rocket via 82 total",
		},
		{
			name:           "Gym Heroes - by set name",
			input:          "Lt. Surge's Electabuzz\nHP 70\nGym Heroes\n27/132",
			wantSetCode:    "gym1",
			wantCardNumber: "27",
			description:    "Gym Heroes via set name",
		},
		{
			name:           "Neo Genesis - by set name",
			input:          "Lugia\nHP 90\nNeo Genesis\n9/111",
			wantSetCode:    "neo1",
			wantCardNumber: "9",
			description:    "Neo Genesis via set name",
		},
		{
			name:           "Neo Discovery - by set name",
			input:          "Umbreon\nHP 70\nNeo Discovery\n32/75",
			wantSetCode:    "neo2",
			wantCardNumber: "32",
			description:    "Neo Discovery via set name",
		},
		{
			name:           "Neo Destiny - by total 113",
			input:          "Shining Charizard\nHP 100\n107/113\n©2002 Wizards",
			wantSetCode:    "neo4",
			wantCardNumber: "107",
			description:    "Neo Destiny via 113 total",
		},

		// ==================== EX Era (2003-2007) ====================
		{
			name:           "EX Ruby & Sapphire - by set name",
			input:          "Blaziken\nHP 100\nRuby & Sapphire\n3/109",
			wantSetCode:    "ex1",
			wantCardNumber: "3",
			description:    "EX Ruby & Sapphire via set name",
		},
		{
			name:           "EX Dragon - by set name",
			input:          "Rayquaza ex\nHP 100\nEX Dragon\n97/97",
			wantSetCode:    "ex3",
			wantCardNumber: "97",
			description:    "EX Dragon via set name",
		},
		{
			name:           "EX FireRed LeafGreen - by set name",
			input:          "Charizard ex\nHP 160\nFireRed & LeafGreen\n105/112",
			wantSetCode:    "ex6",
			wantCardNumber: "105",
			description:    "EX FireRed LeafGreen via set name",
		},

		// ==================== Diamond & Pearl Era (2007-2011) ====================
		{
			name:           "Diamond & Pearl - by set name",
			input:          "Dialga\nHP 90\nDiamond & Pearl\n1/130",
			wantSetCode:    "dp1",
			wantCardNumber: "1",
			description:    "Diamond & Pearl via set name",
		},
		{
			name:           "Platinum - by set name",
			input:          "Giratina\nHP 100\nPlatinum\n10/127",
			wantSetCode:    "pl1",
			wantCardNumber: "10",
			description:    "Platinum via set name",
		},

		// ==================== HeartGold SoulSilver Era (2010-2011) ====================
		{
			name:           "HeartGold SoulSilver - by set name",
			input:          "Lugia\nHP 100\nHeartGold & SoulSilver\n2/123",
			wantSetCode:    "hgss1",
			wantCardNumber: "2",
			description:    "HGSS via set name",
		},

		// ==================== Black & White Era (2011-2014) ====================
		{
			name:           "Black & White - by set name",
			input:          "Reshiram\nHP 130\nBlack & White\n26/114",
			wantSetCode:    "bw1",
			wantCardNumber: "26",
			description:    "Black & White via set name",
		},
		{
			name:           "Boundaries Crossed - by set name",
			input:          "Charizard\nHP 160\nBoundaries Crossed\n20/149",
			wantSetCode:    "bw7",
			wantCardNumber: "20",
			description:    "Boundaries Crossed via set name",
		},

		// ==================== XY Era (2014-2017) ====================
		{
			name:           "XY Base - by set code",
			input:          "Xerneas EX\nHP 170\nXY\n97/146",
			wantSetCode:    "xy1",
			wantCardNumber: "97",
			description:    "XY via set code in text",
		},
		{
			name:           "Evolutions - by set name",
			input:          "Charizard EX\nHP 180\nEvolutions\n12/108",
			wantSetCode:    "xy12",
			wantCardNumber: "12",
			description:    "Evolutions via set name",
		},

		// ==================== Sun & Moon Era (2017-2020) ====================
		{
			name:           "Sun & Moon Base - by set name",
			input:          "Solgaleo GX\nHP 250\nSun & Moon\n89/149",
			wantSetCode:    "sm1",
			wantCardNumber: "89",
			description:    "Sun & Moon via set name",
		},
		{
			name:           "Team Up - by set name",
			input:          "Pikachu & Zekrom GX\nHP 240\nTeam Up\n33/181",
			wantSetCode:    "sm9",
			wantCardNumber: "33",
			description:    "Team Up via set name",
		},
		{
			name:           "Hidden Fates - by set name",
			input:          "Charizard GX\nHP 250\nHidden Fates\n9/68",
			wantSetCode:    "sm11pt5",
			wantCardNumber: "9",
			description:    "Hidden Fates via set name",
		},

		// ==================== Sword & Shield Era (2020-2023) ====================
		{
			name:           "Sword & Shield Base - by set code",
			input:          "Zacian V\nHP 220\nSWSH1\n138/202",
			wantSetCode:    "swsh1",
			wantCardNumber: "138",
			description:    "SWSH1 via explicit set code",
		},
		{
			name:           "Vivid Voltage - by total 185",
			input:          "Pikachu VMAX\nHP 310\n044/185",
			wantSetCode:    "swsh4",
			wantCardNumber: "44",
			description:    "Vivid Voltage via 185 total",
		},
		{
			name:           "Evolving Skies - by set name",
			input:          "Umbreon VMAX\nHP 320\nEvolving Skies\n215/203",
			wantSetCode:    "swsh7",
			wantCardNumber: "215",
			description:    "Evolving Skies via set name",
		},
		{
			name:           "Brilliant Stars - by set name",
			input:          "Charizard VSTAR\nHP 280\nBrilliant Stars\nTG03/TG30",
			wantSetCode:    "swsh9",
			wantCardNumber: "TG03",
			description:    "Brilliant Stars via set name",
		},
		{
			name:           "Crown Zenith - by set name",
			input:          "Arceus VSTAR\nHP 280\nCrown Zenith\nGG70/GG70",
			wantSetCode:    "swsh12pt5",
			wantCardNumber: "GG70",
			description:    "Crown Zenith via set name",
		},

		// ==================== Scarlet & Violet Era (2023+) ====================
		{
			name:           "Scarlet & Violet Base - by set code",
			input:          "Koraidon ex\nHP 230\nSV1\n125/198",
			wantSetCode:    "sv1",
			wantCardNumber: "125",
			description:    "SV1 via explicit set code",
		},
		{
			name:           "Paldea Evolved - by set name",
			input:          "Chien-Pao ex\nHP 220\nPaldea Evolved\n61/193",
			wantSetCode:    "sv2",
			wantCardNumber: "61",
			description:    "Paldea Evolved via set name",
		},
		{
			name:           "Obsidian Flames - by set name",
			input:          "Charizard ex\nHP 330\nObsidian Flames\n125/197",
			wantSetCode:    "sv3",
			wantCardNumber: "125",
			description:    "Obsidian Flames via set name",
		},
		{
			name:           "151 - by set name",
			input:          "Charizard ex\nHP 330\n151\n6/165",
			wantSetCode:    "sv3pt5",
			wantCardNumber: "6",
			description:    "151 via set name",
		},
		{
			name:           "Temporal Forces - by set name",
			input:          "Walking Wake ex\nHP 220\nTemporal Forces\n25/162",
			wantSetCode:    "sv5",
			wantCardNumber: "25",
			description:    "Temporal Forces via set name",
		},
		{
			name:           "Shrouded Fable - by set name",
			input:          "Kingdra ex\nHP 320\nShrouded Fable\n35/64",
			wantSetCode:    "sv6pt5",
			wantCardNumber: "35",
			description:    "Shrouded Fable via set name",
		},

		// ==================== Ambiguous Totals - WotC Era Priority ====================
		{
			name:           "Ambiguous 102 - Base Set with Wizards",
			input:          "Raichu\nHP 80\n14/102\n©1999 Wizards of the Coast",
			wantSetCode:    "base1", // Not HGSS Triumphant
			wantCardNumber: "14",
			description:    "102 total + Wizards = Base Set, not HGSS Triumphant",
		},
		{
			name:           "Ambiguous 102 - HGSS Triumphant without Wizards",
			input:          "Magnezone Prime\nHP 140\n96/102\nTriumphant",
			wantSetCode:    "hgss4",
			wantCardNumber: "96",
			description:    "102 total + Triumphant set name = HGSS Triumphant",
		},

		// ==================== PTCGO Codes ====================
		{
			name:           "PTCGO code BS - Base Set",
			input:          "Pikachu\nHP 40\nBS 58/102",
			wantSetCode:    "base1",
			wantCardNumber: "58",
			description:    "BS PTCGO code = Base Set",
		},
		{
			name:           "PTCGO code JU - Jungle",
			input:          "Jolteon\nHP 70\nJU 4/64",
			wantSetCode:    "base2",
			wantCardNumber: "4",
			description:    "JU PTCGO code = Jungle",
		},
		{
			name:           "PTCGO code FO - Fossil",
			input:          "Aerodactyl\nHP 60\nFO 1/62",
			wantSetCode:    "base3",
			wantCardNumber: "1",
			description:    "FO PTCGO code = Fossil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseOCRText(tt.input, "pokemon")

			if result == nil {
				t.Fatal("Result should not be nil")
			}

			if result.SetCode != tt.wantSetCode {
				t.Errorf("SetCode = %q, want %q (%s)", result.SetCode, tt.wantSetCode, tt.description)
			}

			if tt.wantCardNumber != "" && result.CardNumber != tt.wantCardNumber {
				t.Errorf("CardNumber = %q, want %q", result.CardNumber, tt.wantCardNumber)
			}

			t.Logf("[%s] SetCode=%q, CardNumber=%q, Confidence=%.2f",
				tt.description, result.SetCode, result.CardNumber, result.Confidence)
		})
	}
}

// TestFullCardOCRInputs tests complete OCR outputs from real card scans
// These simulate the full text you'd get from scanning an actual Pokemon card
func TestFullCardOCRInputs(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantCardName   string
		wantCardNumber string
		wantSetCode    string
		wantHP         string
		wantIsWotC     bool
		wantIsFoil     bool
		minConfidence  float64
	}{
		// ==================== Base Set Era Full Cards ====================
		{
			name: "Base Set Charizard - Complete Card Text",
			input: `Charizard
HP 120
Stage 2 Evolves from Charmeleon

Pokémon Power: Energy Burn
As often as you like during your turn (before your attack), you may turn all Energy attached to Charizard into Fire Energy for the rest of the turn. This power can't be used if Charizard is Asleep, Confused, or Paralyzed.

Fire Spin                                    100
Discard 2 Energy cards attached to Charizard in order to use this attack.

weakness        resistance       retreat cost
Water           Fighting -30     ★★★

Spits fire that is hot enough to melt boulders. Known to cause forest fires unintentionally. LV. 76 #6

Illus. Mitsuhiro Arita                       4/102
©1995, 96, 98, 99 Nintendo, Creatures, GAMEFREAK. ©1999 Wizards.`,
			wantCardName:   "Charizard",
			wantCardNumber: "4",
			wantSetCode:    "base1",
			wantHP:         "120",
			wantIsWotC:     true,
			wantIsFoil:     false,
			minConfidence:  0.9,
		},
		{
			name: "Jungle Scyther - Complete Card Text",
			input: `Scyther
HP 70
Basic Pokémon

Swords Dance
During your next turn, Scyther's Slash attack's base damage is 60 instead of 30.

Slash                                        30

weakness        resistance       retreat cost
Fire            Fighting -30

With ninja-like agility and speed, it can create the illusion that there is more than one of it. LV. 25 #123

Illus. Ken Sugimori                          10/64
©1995, 96, 98, 99 Nintendo, Creatures, GAMEFREAK. ©1999 Wizards.`,
			wantCardName:   "Scyther",
			wantCardNumber: "10",
			wantSetCode:    "base2",
			wantHP:         "70",
			wantIsWotC:     true,
			wantIsFoil:     false,
			minConfidence:  0.9,
		},
		{
			name: "Fossil Gengar - Complete Card Text",
			input: `Gengar
HP 80
Stage 2 Evolves from Haunter

Pokémon Power: Curse
Once during your turn (before your attack), you may move 1 damage counter from 1 of your opponent's Pokémon to another (even if it would Knock Out the other Pokémon). This power can't be used if Gengar is Asleep, Confused, or Paralyzed.

Dark Mind                                    30
If your opponent has any Benched Pokémon, choose 1 of them and this attack does 10 damage to it. (Don't apply Weakness and Resistance for Benched Pokémon.)

weakness        resistance       retreat cost
None            Fighting -30     ★

Under a full moon, this Pokémon likes to mimic the shadows of people and laugh at their fright. LV. 38 #94

Illus. Keiji Kinebuchi                       5/62
©1995, 96, 98, 99 Nintendo, Creatures, GAMEFREAK. ©1999 Wizards.`,
			wantCardName:   "Gengar",
			wantCardNumber: "5",
			wantSetCode:    "base3",
			wantHP:         "80",
			wantIsWotC:     true,
			wantIsFoil:     false,
			minConfidence:  0.9,
		},
		{
			name: "Team Rocket Dark Charizard - Complete Card Text",
			input: `Dark Charizard
HP 80
Stage 2 Evolves from Dark Charmeleon

Nail Flick                                   10

Continuous Fireball                          50×
Flip a number of coins equal to the number of Fire Energy cards attached to Dark Charizard. This attack does 50 damage times the number of heads. Discard a number of Fire Energy cards attached to Dark Charizard equal to the number of heads.

weakness        resistance       retreat cost
Water           Fighting -30     ★★★

Illus. Ken Sugimori                          4/82
©1995, 96, 98, 99 Nintendo, Creatures, GAMEFREAK. ©2000 Wizards.`,
			wantCardName:   "Dark Charizard",
			wantCardNumber: "4",
			wantSetCode:    "base5",
			wantHP:         "80",
			wantIsWotC:     true,
			wantIsFoil:     false,
			minConfidence:  0.9,
		},
		{
			name: "Neo Genesis Lugia - Complete Card Text",
			input: `Lugia
HP 90
Basic Pokémon

Elemental Blast                              90
Discard a Water Energy card, a Fire Energy card, and a Lightning Energy card attached to Lugia in order to use this attack.

weakness        resistance       retreat cost
Psychic         Fighting -30     ★★

It is said that it quietly spends its time deep at the bottom of the sea because its powers are too strong. LV. 45 #249

Illus. Hironobu Yoshida                      9/111
Neo Genesis`,
			wantCardName:   "Lugia",
			wantCardNumber: "9",
			wantSetCode:    "neo1",
			wantHP:         "90",
			wantIsWotC:     true,
			wantIsFoil:     false,
			minConfidence:  0.9,
		},
		{
			name: "1st Edition Base Set Alakazam",
			input: `Alakazam
HP 80
Stage 2 Evolves from Kadabra

Pokémon Power: Damage Swap
As often as you like during your turn (before your attack), you may move 1 damage counter from 1 of your Pokémon to another as long as you don't Knock Out that Pokémon. This power can't be used if Alakazam is Asleep, Confused, or Paralyzed.

Confuse Ray                                  30
Flip a coin. If heads, the Defending Pokémon is now Confused.

weakness        resistance       retreat cost
Psychic                          ★★★

Its brain can outperform a supercomputer. Its intelligence quotient is said to be 5000. LV. 42 #65

1ST EDITION
Illus. Ken Sugimori                          1/102
©1999 Wizards`,
			wantCardName:   "Alakazam",
			wantCardNumber: "1",
			wantSetCode:    "base1",
			wantHP:         "80",
			wantIsWotC:     true,
			wantIsFoil:     false,
			minConfidence:  0.9,
		},

		// ==================== Modern Era Full Cards ====================
		{
			name: "Vivid Voltage Pikachu VMAX - Complete Card Text",
			input: `Pikachu VMAX
HP 310
VMAX Evolves from Pikachu V

G-Max Volt Tackle                            120
You may discard all Lightning Energy from this Pokémon. If you do, this attack does 150 more damage.

VMAX rule
When your Pokémon VMAX is Knocked Out, your opponent takes 3 Prize cards.

weakness        resistance       retreat cost
Fighting ×2                      ★★★

Illus. aky CG Works                          044/185
SWSH Vivid Voltage
©2020 Pokémon`,
			wantCardName:   "Pikachu VMAX",
			wantCardNumber: "44",
			wantSetCode:    "swsh4",
			wantHP:         "310",
			wantIsWotC:     false,
			wantIsFoil:     false,
			minConfidence:  0.9,
		},
		{
			name: "Brilliant Stars Charizard VSTAR - Complete Card Text",
			input: `Charizard VSTAR
HP 280
VSTAR Evolves from Charizard V

Explosive Fire                               130+
This attack does 100 more damage for each of your Benched Pokémon V.

VSTAR Power
★ Star Blaze                                 320
Discard 2 Energy from this Pokémon.

VSTAR rule
When your Pokémon VSTAR is Knocked Out, your opponent takes 2 Prize cards.

weakness        resistance       retreat cost
Water ×2                         ★★★

Illus. 5ban Graphics                         TG03/TG30
SWSH Brilliant Stars
©2022 Pokémon`,
			wantCardName:   "Charizard VSTAR",
			wantCardNumber: "TG03",
			wantSetCode:    "swsh9",
			wantHP:         "280",
			wantIsWotC:     false,
			wantIsFoil:     false,
			minConfidence:  0.9,
		},
		{
			name: "Paldea Evolved Chien-Pao ex - Complete Card Text",
			input: `Chien-Pao ex
HP 220
Basic Pokémon

Ability: Shivery Chill
Once during your turn, if this Pokémon is in the Active Spot, you may search your deck for up to 2 Basic Water Energy cards, reveal them, and put them into your hand. Then, shuffle your deck.

Hail Blade                                   60×
You may discard any amount of Water Energy from your Pokémon. This attack does 60 damage for each card you discarded in this way.

Pokémon ex rule
When your Pokémon ex is Knocked Out, your opponent takes 2 Prize cards.

weakness        resistance       retreat cost
Metal ×2                         ★★

Illus. 5ban Graphics                         61/193
Paldea Evolved
©2023 Pokémon`,
			wantCardName:   "Chien-Pao ex",
			wantCardNumber: "61",
			wantSetCode:    "sv2",
			wantHP:         "220",
			wantIsWotC:     false,
			wantIsFoil:     false,
			minConfidence:  0.9,
		},
		{
			name: "151 Charizard ex - Complete Card Text",
			input: `Charizard ex
HP 330
Stage 2 Evolves from Charmeleon

Ability: Infernal Reign
When you play this Pokémon from your hand to evolve 1 of your Pokémon during your turn, you may search your deck for up to 3 Basic Fire Energy cards and attach them to your Pokémon in any way you like. Then, shuffle your deck.

Burning Dark                                 180
This attack does 30 more damage for each Prize card your opponent has taken.

Pokémon ex rule
When your Pokémon ex is Knocked Out, your opponent takes 2 Prize cards.

weakness        resistance       retreat cost
Water ×2                         ★★★

Illus. PLANETA Mochizuki                     6/165
151
©2023 Pokémon`,
			wantCardName:   "Charizard ex",
			wantCardNumber: "6",
			wantSetCode:    "sv3pt5",
			wantHP:         "330",
			wantIsWotC:     false,
			wantIsFoil:     false,
			minConfidence:  0.9,
		},

		// ==================== Middle Era Full Cards ====================
		{
			name: "EX Ruby Sapphire Blaziken - Complete Card Text",
			input: `Blaziken
HP 100
Stage 2 Evolves from Combusken

Poké-BODY: Blaze
As long as Blaziken's remaining HP is 40 or less, Blaziken does 40 more damage to the Defending Pokémon (before applying Weakness and Resistance).

Fire Stream                                  50
Discard a Fire Energy card attached to Blaziken. This attack does 10 damage to each of your opponent's Benched Pokémon. (Don't apply Weakness and Resistance for Benched Pokémon.)

weakness        resistance       retreat cost
Water           Psychic          ★★

Illus. Kouki Saitou                          3/109
EX Ruby & Sapphire`,
			wantCardName:   "Blaziken",
			wantCardNumber: "3",
			wantSetCode:    "ex1",
			wantHP:         "100",
			wantIsWotC:     false,
			wantIsFoil:     false,
			minConfidence:  0.9,
		},
		{
			name: "XY Evolutions Charizard EX - Complete Card Text",
			input: `Charizard EX
HP 180
Basic Pokémon

Stoke
Flip a coin. If heads, search your deck for up to 3 basic Energy cards and attach them to this Pokémon. Shuffle your deck afterward.

Fire Blast                                   120
Discard an Energy attached to this Pokémon.

Pokémon-EX rule
When a Pokémon-EX has been Knocked Out, your opponent takes 2 Prize cards.

weakness        resistance       retreat cost
Water ×2                         ★★★

Illus. Mitsuhiro Arita                       12/108
XY Evolutions
©2016 Pokémon`,
			wantCardName:   "Charizard EX",
			wantCardNumber: "12",
			wantSetCode:    "xy12",
			wantHP:         "180",
			wantIsWotC:     false,
			wantIsFoil:     false,
			minConfidence:  0.9,
		},
		{
			name: "Sun Moon Hidden Fates Charizard GX - Complete Card Text",
			input: `Charizard GX
HP 250
Stage 2 Evolves from Charmeleon

Flare Blitz                                  160
Discard all Fire Energy from this Pokémon.

Crimson Storm GX                             300
Discard 3 Fire Energy from this Pokémon. (You can't use more than 1 GX attack in a game.)

When your Pokémon-GX is Knocked Out, your opponent takes 2 Prize cards.

weakness        resistance       retreat cost
Water ×2                         ★★★

Illus. PLANETA Tsuji                         SV49/SV94
Hidden Fates Shiny Vault`,
			wantCardName:   "Charizard GX",
			wantCardNumber: "SV49",
			wantSetCode:    "sm11pt5",
			wantHP:         "250",
			wantIsWotC:     false,
			wantIsFoil:     false,
			minConfidence:  0.9,
		},

		// ==================== Edge Cases and Noisy OCR ====================
		{
			name: "Noisy OCR - Base Set Pikachu with artifacts",
			input: `P1kachu
HP 40
Bas1c Pokémon

Gnaw                                         l0

Thunder Jo1t                                 30
F1ip a coin. If tails, Pikachu does l0 damage to itself.

weakness        res1stance       retreat cost
F1ghting

When several of these Pokémon gather, their e1ectricity can cause l1ghtning storms. LV. l2 #25

Illus. M1tsuhiro Arita                       58/l02
©l999 W1zards`,
			wantCardName:   "Pikachu",
			wantCardNumber: "58",
			wantSetCode:    "base1",
			wantHP:         "40",
			wantIsWotC:     true,
			wantIsFoil:     false,
			minConfidence:  0.5,
		},
		{
			name: "Partial OCR - Missing card name line",
			input: `HP 120
Stage 2 Evolves from Charmeleon

Fire Spin                                    100
Discard 2 Energy cards attached to Charizard in order to use this attack.

weakness        resistance       retreat cost
Water           Fighting -30     ★★★

4/102
©1999 Wizards`,
			wantCardName:   "Charizard",
			wantCardNumber: "4",
			wantSetCode:    "base1",
			wantHP:         "120",
			wantIsWotC:     true,
			wantIsFoil:     false,
			minConfidence:  0.5,
		},
		{
			name: "Holo Card - Explicit Holo Rare marking",
			input: `Charizard
HP 120
Stage 2

Holo Rare

Fire Spin                                    100

4/102
©1999 Wizards`,
			wantCardName:   "Charizard",
			wantCardNumber: "4",
			wantSetCode:    "base1",
			wantHP:         "120",
			wantIsWotC:     true,
			wantIsFoil:     true, // Holo Rare should trigger foil
			minConfidence:  0.7,
		},
		{
			name: "Reverse Holo Modern Card",
			input: `Pikachu
HP 60
Basic Pokémon

Reverse Holo

Quick Attack                                 30+
Flip a coin. If heads, this attack does 30 more damage.

025/185
SWSH Vivid Voltage`,
			wantCardName:   "Pikachu",
			wantCardNumber: "25",
			wantSetCode:    "swsh4",
			wantHP:         "60",
			wantIsWotC:     false,
			wantIsFoil:     true, // Reverse Holo should trigger foil
			minConfidence:  0.7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseOCRText(tt.input, "pokemon")

			if result == nil {
				t.Fatal("Result should not be nil")
			}

			// Check card name
			if tt.wantCardName != "" && result.CardName != tt.wantCardName {
				t.Errorf("CardName = %q, want %q", result.CardName, tt.wantCardName)
			}

			// Check card number
			if tt.wantCardNumber != "" && result.CardNumber != tt.wantCardNumber {
				t.Errorf("CardNumber = %q, want %q", result.CardNumber, tt.wantCardNumber)
			}

			// Check set code
			if tt.wantSetCode != "" && result.SetCode != tt.wantSetCode {
				t.Errorf("SetCode = %q, want %q", result.SetCode, tt.wantSetCode)
			}

			// Check HP
			if tt.wantHP != "" && result.HP != tt.wantHP {
				t.Errorf("HP = %q, want %q", result.HP, tt.wantHP)
			}

			// Check WotC era
			if result.IsWotCEra != tt.wantIsWotC {
				t.Errorf("IsWotCEra = %v, want %v", result.IsWotCEra, tt.wantIsWotC)
			}

			// Check foil detection
			if result.IsFoil != tt.wantIsFoil {
				t.Errorf("IsFoil = %v, want %v (indicators: %v)", result.IsFoil, tt.wantIsFoil, result.FoilIndicators)
			}

			// Check confidence
			if tt.minConfidence > 0 && result.Confidence < tt.minConfidence {
				t.Errorf("Confidence = %.2f, want >= %.2f", result.Confidence, tt.minConfidence)
			}

			t.Logf("Result: Name=%q, Number=%q, Set=%q, HP=%q, WotC=%v, Foil=%v, Conf=%.2f",
				result.CardName, result.CardNumber, result.SetCode, result.HP,
				result.IsWotCEra, result.IsFoil, result.Confidence)
		})
	}
}
