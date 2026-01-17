package services

import (
	"regexp"
	"strings"
)

// OCRResult contains parsed information from OCR text
type OCRResult struct {
	RawText           string   `json:"raw_text"`
	CardName          string   `json:"card_name"`
	CardNumber        string   `json:"card_number"`         // e.g., "25" from "025/185"
	SetTotal          string   `json:"set_total"`           // e.g., "185" from "025/185"
	SetCode           string   `json:"set_code"`            // e.g., "SWSH4" if detected
	SetName           string   `json:"set_name"`            // e.g., "Vivid Voltage" if detected
	HP                string   `json:"hp"`                  // e.g., "170" from "HP 170"
	Rarity            string   `json:"rarity"`              // if detected
	IsFoil            bool     `json:"is_foil"`             // detected foil indicators
	FoilIndicators    []string `json:"foil_indicators"`     // what triggered foil detection
	AllLines          []string `json:"all_lines"`
	Confidence        float64  `json:"confidence"`          // 0-1 based on how much we extracted
	ConditionHints    []string `json:"condition_hints"`     // hints about card condition
}

// Maximum allowed OCR text length to prevent regex DoS
const maxOCRTextLength = 10000

// ParseOCRText extracts card information from OCR text
func ParseOCRText(text string, game string) *OCRResult {
	// Truncate overly long text to prevent regex DoS
	if len(text) > maxOCRTextLength {
		text = text[:maxOCRTextLength]
	}

	result := &OCRResult{
		RawText: text,
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

	// Calculate confidence based on what we extracted
	result.Confidence = calculateConfidence(result)

	return result
}

func parsePokemonOCR(result *OCRResult) {
	text := result.RawText
	upperText := strings.ToUpper(text)

	// Extract card number pattern: XXX/YYY (e.g., "025/185", "TG17/TG30")
	cardNumRegex := regexp.MustCompile(`(?:^|\s)(\d{1,3})\s*/\s*(\d{1,3})(?:\s|$|[^0-9])`)
	if matches := cardNumRegex.FindStringSubmatch(text); len(matches) >= 3 {
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

	// Extract HP: "HP 170" or "170 HP" or just "D170" pattern
	hpRegex := regexp.MustCompile(`(?:HP\s*(\d+)|(\d{2,3})\s*HP|[A-Z](\d{2,3})\s*[&@])`)
	if matches := hpRegex.FindStringSubmatch(text); len(matches) >= 2 {
		for _, m := range matches[1:] {
			if m != "" {
				result.HP = m
				break
			}
		}
	}

	// Extract set code patterns with full set code (e.g., SWSH4, SV1, XY12)
	// More comprehensive list of Pokemon TCG set prefixes
	setCodeRegex := regexp.MustCompile(`\b(SWSH\d{1,2}|SV\d{1,2}|XY\d{1,2}|SM\d{1,2}|BW\d{1,2}|DP\d?|EX\d{1,2}|RS|LC|BS\d?|PGO|CEL25|PR-SW|PR-SV)\b`)
	if matches := setCodeRegex.FindStringSubmatch(upperText); len(matches) >= 1 {
		result.SetCode = strings.ToLower(matches[0])
	}

	// Detect foil/holo indicators
	detectFoilIndicators(result, upperText)

	// Detect rarity
	detectPokemonRarity(result, upperText)

	// Detect condition hints (grading labels, damage indicators)
	detectConditionHints(result, upperText)

	// Extract card name - usually first substantial line or after HP
	result.CardName = extractPokemonCardName(result.AllLines)
}

// detectFoilIndicators checks for foil/holographic card indicators
func detectFoilIndicators(result *OCRResult, upperText string) {
	foilPatterns := map[string]string{
		"HOLO":           "Holographic text detected",
		"HOLOFOIL":       "Holofoil text detected",
		"REVERSE HOLO":   "Reverse holo text detected",
		"REVERSE":        "Reverse holo indicator",
		"SHINY":          "Shiny variant text",
		"GOLD":           "Gold card indicator",
		"RAINBOW":        "Rainbow rare indicator",
		"FULL ART":       "Full art card",
		"ALT ART":        "Alternate art card",
		"ALTERNATE ART":  "Alternate art card",
		"SECRET":         "Secret rare indicator",
		"ILLUSTRATION":   "Special illustration rare",
		"SPECIAL ART":    "Special art rare",
		"CROWN ZENITH":   "Crown Zenith (often special)",
	}

	for pattern, hint := range foilPatterns {
		if strings.Contains(upperText, pattern) {
			result.IsFoil = true
			result.FoilIndicators = append(result.FoilIndicators, hint)
		}
	}

	// Check for V, VMAX, VSTAR, EX, GX patterns which are typically holo
	specialPatterns := regexp.MustCompile(`\b(VMAX|VSTAR|V|GX|EX|MEGA|PRIME|LV\.?\s*X)\b`)
	if specialPatterns.MatchString(upperText) {
		result.IsFoil = true
		result.FoilIndicators = append(result.FoilIndicators, "Special card type (typically holographic)")
	}
}

// detectPokemonRarity detects card rarity from text
func detectPokemonRarity(result *OCRResult, upperText string) {
	// Rarity symbols often appear as text in OCR
	rarityPatterns := map[string]string{
		"ILLUSTRATION RARE":     "Illustration Rare",
		"SPECIAL ART RARE":      "Special Art Rare",
		"HYPER RARE":            "Hyper Rare",
		"SECRET RARE":           "Secret Rare",
		"ULTRA RARE":            "Ultra Rare",
		"DOUBLE RARE":           "Double Rare",
		"RARE HOLO":             "Rare Holo",
		"RARE":                  "Rare",
		"UNCOMMON":              "Uncommon",
		"COMMON":                "Common",
		"PROMO":                 "Promo",
	}

	for pattern, rarity := range rarityPatterns {
		if strings.Contains(upperText, pattern) {
			result.Rarity = rarity
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

func extractPokemonCardName(lines []string) string {
	// Common patterns to skip
	skipPatterns := []string{
		"basic", "stage", "pokemon", "trainer", "energy",
		"once during", "when you", "attack", "weakness",
		"resistance", "retreat", "illus", "©", "nintendo",
	}

	for _, line := range lines {
		lower := strings.ToLower(line)

		// Skip short lines
		if len(line) < 3 {
			continue
		}

		// Skip lines that are just numbers
		if regexp.MustCompile(`^[\d\s/]+$`).MatchString(line) {
			continue
		}

		// Skip common non-name patterns
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

		// Skip lines with too many special characters
		specialCount := len(regexp.MustCompile(`[^a-zA-Z0-9\s'-]`).FindAllString(line, -1))
		if specialCount > 3 {
			continue
		}

		// This might be the card name
		// Clean it up - remove HP values, etc.
		name := regexp.MustCompile(`\s*HP\s*\d+`).ReplaceAllString(line, "")
		name = regexp.MustCompile(`\s*\d{2,3}\s*HP`).ReplaceAllString(name, "")
		name = strings.TrimSpace(name)

		if len(name) >= 3 {
			return name
		}
	}

	// Fallback: return first line with letters
	for _, line := range lines {
		if regexp.MustCompile(`[a-zA-Z]{3,}`).MatchString(line) {
			return strings.TrimSpace(line)
		}
	}

	return ""
}

func parseMTGOCR(result *OCRResult) {
	text := result.RawText

	// MTG collector number pattern: e.g., "123/456" or "123"
	collectorRegex := regexp.MustCompile(`(?:^|\s)(\d{1,4})\s*/\s*(\d{1,4})(?:\s|$)`)
	if matches := collectorRegex.FindStringSubmatch(text); len(matches) >= 3 {
		result.CardNumber = matches[1]
		result.SetTotal = matches[2]
	}

	// MTG set codes are 3-4 letters: MKM, ONE, DMU, etc.
	setCodeRegex := regexp.MustCompile(`\b([A-Z]{3,4})\b`)
	upperText := strings.ToUpper(text)
	for _, match := range setCodeRegex.FindAllStringSubmatch(upperText, -1) {
		code := match[1]
		// Skip common false positives
		if code != "THE" && code != "AND" && code != "FOR" && code != "YOU" {
			result.SetCode = code
			break
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

	// Card name is typically the first line
	result.CardName = extractMTGCardName(result.AllLines)
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
