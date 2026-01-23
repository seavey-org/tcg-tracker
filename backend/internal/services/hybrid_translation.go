package services

import (
	"context"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/codyseavey/tcg-tracker/backend/internal/metrics"
)

const (
	// DefaultConfidenceThreshold is the score below which we attempt translation
	// Scoring: name_exact=1000, name_partial=500, attack=200, number=300
	DefaultConfidenceThreshold = 800
)

// HybridTranslationService orchestrates static translation, caching, and API calls
type HybridTranslationService struct {
	cache               *TranslationCacheService
	api                 *TranslationService
	confidenceThreshold int
}

// NewHybridTranslationService creates a new hybrid translation service
func NewHybridTranslationService(db *gorm.DB) *HybridTranslationService {
	threshold := DefaultConfidenceThreshold
	if v := os.Getenv("TRANSLATION_CONFIDENCE_THRESHOLD"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			threshold = parsed
		}
	}

	svc := &HybridTranslationService{
		cache:               NewTranslationCacheService(db),
		api:                 NewTranslationService(),
		confidenceThreshold: threshold,
	}

	log.Printf("Hybrid translation service initialized: threshold=%d, api_enabled=%v",
		threshold, svc.api.IsEnabled())

	return svc
}

// IsAPIEnabled returns whether the translation API is available
func (s *HybridTranslationService) IsAPIEnabled() bool {
	return s.api.IsEnabled()
}

// GetConfidenceThreshold returns the current confidence threshold
func (s *HybridTranslationService) GetConfidenceThreshold() int {
	return s.confidenceThreshold
}

// TranslateForMatching attempts to translate text for card matching.
// It follows this priority:
// 1. If score >= threshold, return original text (no translation needed)
// 2. If language is not Japanese, return original text
// 3. Apply static map translation
// 4. Check translation cache
// 5. Call translation API if cache miss
// 6. Cache the result
//
// Returns: translated text, whether API was used, error (if any)
func (s *HybridTranslationService) TranslateForMatching(
	ctx context.Context,
	text string,
	detectedLanguage string,
	currentScore int,
) (string, bool, error) {
	// Check if translation is needed based on confidence score
	if currentScore >= s.confidenceThreshold {
		return text, false, nil
	}

	// Only translate Japanese text
	if detectedLanguage != "Japanese" {
		return text, false, nil
	}

	// Step 1: Apply static map translation first
	staticTranslated := TranslateTextWithStaticMap(text)
	if staticTranslated != text {
		metrics.TranslationRequestsTotal.WithLabelValues("static").Inc()
	}

	// If API is not enabled, return static translation result
	if !s.api.IsEnabled() {
		return staticTranslated, false, nil
	}

	// Step 2: Check translation cache (using original text as key)
	if cached, found := s.cache.Get(text); found {
		return cached, false, nil
	}

	// Step 3: Call translation API
	translated, err := s.api.Translate(ctx, text, "ja", "en")
	if err != nil {
		log.Printf("Translation API error (returning static result): %v", err)
		// Return static translation on API error
		return staticTranslated, false, err
	}

	// Step 4: Cache the result
	if err := s.cache.Set(text, translated, "ja"); err != nil {
		log.Printf("Failed to cache translation: %v", err)
		// Don't fail the request, just log
	}

	return translated, true, nil
}

// sortedJapaneseKeys holds Japanese keys sorted by length (longest first)
// This ensures longer matches are replaced before shorter ones
// (e.g., リザードン before リザード)
var sortedJapaneseKeys []string
var staticReplacer *strings.Replacer

func init() {
	// Build sorted list of Japanese keys by length (longest first)
	sortedJapaneseKeys = make([]string, 0, len(JapaneseToEnglishNames))
	for japanese := range JapaneseToEnglishNames {
		sortedJapaneseKeys = append(sortedJapaneseKeys, japanese)
	}
	// Sort by length descending (use sort.Slice instead of O(n²) bubble sort)
	sort.Slice(sortedJapaneseKeys, func(i, j int) bool {
		return len(sortedJapaneseKeys[i]) > len(sortedJapaneseKeys[j])
	})

	// Build strings.Replacer for efficient multi-pattern replacement
	// Note: Replacer uses Aho-Corasick-like algorithm for O(n) replacement
	pairs := make([]string, 0, len(JapaneseToEnglishNames)*2)
	for _, jp := range sortedJapaneseKeys {
		pairs = append(pairs, jp, JapaneseToEnglishNames[jp])
	}
	staticReplacer = strings.NewReplacer(pairs...)
}

// TranslateTextWithStaticMap applies the static Japanese-to-English name map
// to translate known words in the text. This is useful for translating
// full OCR text where Japanese words may appear anywhere.
// Uses strings.Replacer for efficient O(n) multi-pattern replacement.
func TranslateTextWithStaticMap(text string) string {
	if text == "" {
		return text
	}
	return staticReplacer.Replace(text)
}
