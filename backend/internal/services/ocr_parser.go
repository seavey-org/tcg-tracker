package services

import (
	"regexp"
	"strings"
)

// OCRResult contains parsed information from OCR text
type OCRResult struct {
	RawText      string   `json:"raw_text"`
	CardName     string   `json:"card_name"`
	CardNumber   string   `json:"card_number"`   // e.g., "25" from "025/185"
	SetTotal     string   `json:"set_total"`     // e.g., "185" from "025/185"
	SetCode      string   `json:"set_code"`      // e.g., "SWSH" if detected
	HP           string   `json:"hp"`            // e.g., "170" from "HP 170"
	Rarity       string   `json:"rarity"`        // if detected
	AllLines     []string `json:"all_lines"`
	Confidence   float64  `json:"confidence"`    // 0-1 based on how much we extracted
}

// ParseOCRText extracts card information from OCR text
func ParseOCRText(text string, game string) *OCRResult {
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

	// Extract set code patterns (SWSH, SV, XY, SM, etc.)
	setCodeRegex := regexp.MustCompile(`\b(SWSH|SV|XY|SM|BW|DP|EX|RS|LC|BS)\d*\b`)
	if matches := setCodeRegex.FindStringSubmatch(strings.ToUpper(text)); len(matches) >= 1 {
		result.SetCode = matches[0]
	}

	// Extract card name - usually first substantial line or after HP
	result.CardName = extractPokemonCardName(result.AllLines)
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
	for _, match := range setCodeRegex.FindAllStringSubmatch(strings.ToUpper(text), -1) {
		code := match[1]
		// Skip common false positives
		if code != "THE" && code != "AND" && code != "FOR" && code != "YOU" {
			result.SetCode = code
			break
		}
	}

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
