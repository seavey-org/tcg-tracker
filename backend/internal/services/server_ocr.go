package services

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ServerOCRService provides server-side OCR processing using Tesseract
type ServerOCRService struct {
	tesseractPath string
	language      string
}

// ServerOCRResult contains the result of server-side OCR processing
type ServerOCRResult struct {
	Text       string   `json:"text"`
	Lines      []string `json:"lines"`
	Confidence float64  `json:"confidence"`
	Error      string   `json:"error,omitempty"`
}

// NewServerOCRService creates a new server OCR service
func NewServerOCRService() *ServerOCRService {
	// Find tesseract in PATH
	tesseractPath, err := exec.LookPath("tesseract")
	if err != nil {
		tesseractPath = "tesseract" // Will fail at runtime if not found
	}

	return &ServerOCRService{
		tesseractPath: tesseractPath,
		language:      "eng", // English by default
	}
}

// IsAvailable checks if Tesseract is available on the system
func (s *ServerOCRService) IsAvailable() bool {
	cmd := exec.Command(s.tesseractPath, "--version")
	err := cmd.Run()
	return err == nil
}

// ProcessImage processes an image file and returns OCR text.
// The image path is validated and sanitized to prevent path traversal attacks.
func (s *ServerOCRService) ProcessImage(imagePath string) (*ServerOCRResult, error) {
	// Sanitize and validate the image path to prevent command injection
	cleanPath, err := s.validateImagePath(imagePath)
	if err != nil {
		return &ServerOCRResult{
			Error: fmt.Sprintf("invalid image path: %v", err),
		}, err
	}

	// Run tesseract with custom config for card text
	// Use PSM 6 (Assume a single uniform block of text) or PSM 3 (Fully automatic page segmentation)
	cmd := exec.Command(
		s.tesseractPath,
		cleanPath,
		"stdout", // Output to stdout
		"-l", s.language,
		"--psm", "3", // Fully automatic page segmentation
		"--oem", "3", // Default OCR Engine Mode (LSTM + Legacy)
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		return &ServerOCRResult{
			Error: fmt.Sprintf("tesseract error: %v - %s", err, stderr.String()),
		}, err
	}

	text := stdout.String()
	lines := splitAndCleanLines(text)

	return &ServerOCRResult{
		Text:       text,
		Lines:      lines,
		Confidence: estimateConfidence(lines),
	}, nil
}

// ProcessImageBytes processes image data directly without saving to file
// Uses region-based OCR extraction for better accuracy with trading cards
func (s *ServerOCRService) ProcessImageBytes(imageData []byte) (*ServerOCRResult, error) {
	// Decode the image
	img, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return &ServerOCRResult{
			Error: fmt.Sprintf("invalid image data: %v", err),
		}, err
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Define regions of interest for trading cards
	// Trading cards have standard layouts - we extract text from specific regions
	regions := []struct {
		name                           string
		x1Pct, y1Pct, x2Pct, y2Pct     float64 // Percentages of image dimensions
		psm                            string  // Tesseract page segmentation mode
		priority                       int     // Higher = more important for card identification
	}{
		// Top region: Card name and HP (Pokemon) or name/mana (MTG)
		{"top_header", 0.05, 0.02, 0.95, 0.12, "7", 10},
		// Bottom region: Card number, set info, illustrator
		{"bottom_info", 0.0, 0.85, 1.0, 1.0, "6", 9},
		// Bottom third: Stats, attacks, card number
		{"bottom_third", 0.0, 0.65, 1.0, 1.0, "6", 7},
		// Full card with different PSM modes
		{"full_sparse", 0.0, 0.0, 1.0, 1.0, "11", 5},
		{"full_block", 0.0, 0.0, 1.0, 1.0, "6", 4},
		// Middle-bottom: Attacks/abilities text
		{"mid_bottom", 0.05, 0.50, 0.95, 0.85, "6", 3},
	}

	var allResults []struct {
		lines    []string
		priority int
		region   string
	}

	for _, region := range regions {
		// Calculate pixel coordinates from percentages
		x1 := int(float64(width) * region.x1Pct)
		y1 := int(float64(height) * region.y1Pct)
		x2 := int(float64(width) * region.x2Pct)
		y2 := int(float64(height) * region.y2Pct)

		// Extract and process the region
		cropped := cropImage(img, x1, y1, x2, y2)
		processed := preprocessRegionForOCR(cropped)

		// Run OCR on the region
		lines := s.runTesseract(processed, region.psm)
		if len(lines) > 0 {
			allResults = append(allResults, struct {
				lines    []string
				priority int
				region   string
			}{lines, region.priority, region.name})
		}
	}

	// Try multiple binarization approaches for different card backgrounds
	// Otsu's method works well for high contrast cards but fails on dark cards
	// Fixed thresholds help with specific card types (dark backgrounds, foils, etc.)
	thresholds := []struct {
		name     string
		pct      float64
		useOtsu  bool
		priority int
	}{
		{"binarized_otsu", 0, true, 6},
		{"binarized_35pct", 35, false, 5},
		{"binarized_40pct", 40, false, 5},
		{"binarized_45pct", 45, false, 5},
		{"binarized_50pct", 50, false, 4},
	}

	for _, thresh := range thresholds {
		var binarized image.Image
		if thresh.useOtsu {
			binarized = binarizeImage(img)
		} else {
			binarized = binarizeImageWithThreshold(img, thresh.pct)
		}
		binarizedData := encodeImagePNG(binarized)
		binarizedLines := s.runTesseract(binarizedData, "6")
		if len(binarizedLines) > 0 {
			allResults = append(allResults, struct {
				lines    []string
				priority int
				region   string
			}{binarizedLines, thresh.priority, thresh.name})
		}
	}

	// Combine results: merge unique lines, prioritize by region importance
	combinedLines := s.combineOCRResults(allResults)

	// Post-process to clean up common OCR errors
	cleanedLines := postProcessOCRLines(combinedLines)

	if len(cleanedLines) == 0 {
		return &ServerOCRResult{
			Error: "no text extracted from image",
		}, fmt.Errorf("no text extracted from image")
	}

	confidence := estimateCardConfidence(cleanedLines)
	text := strings.Join(cleanedLines, "\n")

	return &ServerOCRResult{
		Text:       text,
		Lines:      cleanedLines,
		Confidence: confidence,
	}, nil
}

// runTesseract runs Tesseract OCR on image data with specified PSM mode
func (s *ServerOCRService) runTesseract(imageData []byte, psm string) []string {
	cmd := exec.Command(
		s.tesseractPath,
		"stdin",
		"stdout",
		"-l", s.language,
		"--psm", psm,
		"--oem", "3",
	)

	cmd.Stdin = bytes.NewReader(imageData)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil
	}

	return splitAndCleanLines(stdout.String())
}

// cropImage extracts a rectangular region from an image
func cropImage(img image.Image, x1, y1, x2, y2 int) image.Image {
	bounds := img.Bounds()

	// Clamp to image bounds
	if x1 < bounds.Min.X {
		x1 = bounds.Min.X
	}
	if y1 < bounds.Min.Y {
		y1 = bounds.Min.Y
	}
	if x2 > bounds.Max.X {
		x2 = bounds.Max.X
	}
	if y2 > bounds.Max.Y {
		y2 = bounds.Max.Y
	}

	// Create new image for the cropped region
	rect := image.Rect(0, 0, x2-x1, y2-y1)
	cropped := image.NewRGBA(rect)

	draw.Draw(cropped, rect, img, image.Point{x1, y1}, draw.Src)
	return cropped
}

// preprocessRegionForOCR applies preprocessing optimized for OCR
func preprocessRegionForOCR(img image.Image) []byte {
	bounds := img.Bounds()
	gray := image.NewGray(bounds)

	// Convert to grayscale with luminosity formula
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			lum := uint8((0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 256)
			gray.SetGray(x, y, color.Gray{Y: lum})
		}
	}

	// Apply adaptive contrast enhancement
	enhanced := enhanceContrast(gray)

	return encodeImagePNG(enhanced)
}

// binarizeImage converts image to black and white using Otsu's method
func binarizeImage(img image.Image) image.Image {
	bounds := img.Bounds()
	gray := image.NewGray(bounds)

	// First pass: convert to grayscale and build histogram
	histogram := make([]int, 256)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			lum := uint8((0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 256)
			gray.SetGray(x, y, color.Gray{Y: lum})
			histogram[lum]++
		}
	}

	// Otsu's method to find optimal threshold
	threshold := otsuThreshold(histogram, bounds.Dx()*bounds.Dy())

	// Apply threshold
	binary := image.NewGray(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if gray.GrayAt(x, y).Y > threshold {
				binary.SetGray(x, y, color.Gray{Y: 255})
			} else {
				binary.SetGray(x, y, color.Gray{Y: 0})
			}
		}
	}

	return binary
}

// binarizeImageWithThreshold converts image to black and white using a fixed threshold percentage
func binarizeImageWithThreshold(img image.Image, thresholdPct float64) image.Image {
	bounds := img.Bounds()
	threshold := uint8(255 * thresholdPct / 100)

	binary := image.NewGray(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			lum := uint8((0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 256)
			if lum > threshold {
				binary.SetGray(x, y, color.Gray{Y: 255})
			} else {
				binary.SetGray(x, y, color.Gray{Y: 0})
			}
		}
	}

	return binary
}

// otsuThreshold calculates the optimal threshold using Otsu's method
func otsuThreshold(histogram []int, totalPixels int) uint8 {
	var sum float64
	for i := 0; i < 256; i++ {
		sum += float64(i * histogram[i])
	}

	var sumB float64
	var wB, wF int
	var maxVariance float64
	var threshold uint8

	for t := 0; t < 256; t++ {
		wB += histogram[t]
		if wB == 0 {
			continue
		}
		wF = totalPixels - wB
		if wF == 0 {
			break
		}

		sumB += float64(t * histogram[t])
		mB := sumB / float64(wB)
		mF := (sum - sumB) / float64(wF)

		variance := float64(wB) * float64(wF) * (mB - mF) * (mB - mF)
		if variance > maxVariance {
			maxVariance = variance
			threshold = uint8(t)
		}
	}

	return threshold
}

// enhanceContrast applies contrast stretching to a grayscale image
func enhanceContrast(gray *image.Gray) *image.Gray {
	bounds := gray.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	// Build histogram
	histogram := make([]int, 256)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			histogram[gray.GrayAt(x, y).Y]++
		}
	}

	// Find 1% and 99% percentile values for contrast stretching
	total := width * height
	threshold := total / 100
	minVal, maxVal := 0, 255

	count := 0
	for i := 0; i < 256; i++ {
		count += histogram[i]
		if count >= threshold {
			minVal = i
			break
		}
	}

	count = 0
	for i := 255; i >= 0; i-- {
		count += histogram[i]
		if count >= threshold {
			maxVal = i
			break
		}
	}

	// Apply contrast stretching
	enhanced := image.NewGray(bounds)
	if maxVal > minVal {
		scale := 255.0 / float64(maxVal-minVal)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				oldVal := gray.GrayAt(x, y).Y
				newVal := int(float64(int(oldVal)-minVal) * scale)
				if newVal < 0 {
					newVal = 0
				}
				if newVal > 255 {
					newVal = 255
				}
				enhanced.SetGray(x, y, color.Gray{Y: uint8(newVal)})
			}
		}
	} else {
		draw.Draw(enhanced, bounds, gray, bounds.Min, draw.Src)
	}

	return enhanced
}

// encodeImagePNG encodes an image as PNG bytes
func encodeImagePNG(img image.Image) []byte {
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

// combineOCRResults merges OCR results from different regions
func (s *ServerOCRService) combineOCRResults(results []struct {
	lines    []string
	priority int
	region   string
}) []string {
	// Sort by priority (highest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].priority > results[j].priority
	})

	// Track seen lines to avoid duplicates
	seen := make(map[string]bool)
	var combined []string

	// Patterns that indicate important card information
	cardNumberPattern := regexp.MustCompile(`\d{1,4}\s*/\s*\d{1,4}`)
	hpPattern := regexp.MustCompile(`(?i)\d{2,3}\s*HP|HP\s*\d{2,3}`)
	setCodePattern := regexp.MustCompile(`(?i)^[A-Z]{2,4}\d{1,2}$|SWSH|SV\d|XY\d|SM\d|BW\d`)

	// First pass: extract high-value lines (card numbers, HP, etc.)
	var highValueLines []string
	var normalLines []string

	for _, result := range results {
		for _, line := range result.lines {
			normalized := strings.ToLower(strings.TrimSpace(line))
			if seen[normalized] || len(line) < 2 {
				continue
			}

			// Check if line contains important card information
			isHighValue := cardNumberPattern.MatchString(line) ||
				hpPattern.MatchString(line) ||
				setCodePattern.MatchString(line)

			if isHighValue {
				highValueLines = append(highValueLines, line)
			} else {
				normalLines = append(normalLines, line)
			}
			seen[normalized] = true
		}
	}

	// Combine: high-value lines first, then others
	combined = append(combined, highValueLines...)
	combined = append(combined, normalLines...)

	return combined
}

// postProcessOCRLines cleans up common OCR errors
func postProcessOCRLines(lines []string) []string {
	var cleaned []string

	for _, line := range lines {
		// Skip very short lines or lines that are just noise
		if len(line) < 2 {
			continue
		}

		// Apply common OCR corrections
		corrected := correctOCRErrors(line)

		// Skip lines that are mostly special characters
		alphaCount := 0
		for _, c := range corrected {
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
				alphaCount++
			}
		}
		if alphaCount < len(corrected)/3 {
			continue
		}

		cleaned = append(cleaned, corrected)
	}

	return cleaned
}

// correctOCRErrors fixes common OCR misreads
func correctOCRErrors(text string) string {
	// Common OCR error corrections
	corrections := map[string]string{
		// Illustrator misspellings
		"lllus":     "Illus",
		"llus":      "Illus",
		"1llus":     "Illus",
		"|llus":     "Illus",
		// Company names
		"Wisards":   "Wizards",
		"Wizarcls":  "Wizards",
		"Nintenclo": "Nintendo",
		// Pokemon misspellings
		"Pokérnon":  "Pokémon",
		"Pokermon":  "Pokémon",
		"Pokernon":  "Pokémon",
		"Pokéman":   "Pokémon",
		// HP misreads (common on dark cards)
		" HED ":    " HP ",
		" HEP ":    " HP ",
		" HB ":     " HP ",
		" H P ":    " HP ",
		"HED ":     "HP ",
		"HEP ":     "HP ",
		" HED":     " HP",
		" HEP":     " HP",
		// Number corrections - l/1 and O/0 confusion
		" l ":      " 1 ",
		" O ":      " 0 ",
		"l/":       "1/",
		"/l":       "/1",
		"O/":       "0/",
		"/O":       "/0",
		// Modern card HP patterns: "~3l0@" → "~310@", "2l0" → "210"
		"0l0":      "010",
		"1l0":      "110",
		"2l0":      "210",
		"3l0":      "310",
		"4l0":      "410",
		// Also O → 0 in number contexts
		"0O0":      "000",
		"1O0":      "100",
		"2O0":      "200",
		"3O0":      "300",
	}

	result := text
	for wrong, correct := range corrections {
		result = strings.ReplaceAll(result, wrong, correct)
	}

	return result
}

// estimateCardConfidence estimates how confident we are this is valid card OCR
func estimateCardConfidence(lines []string) float64 {
	if len(lines) == 0 {
		return 0.0
	}

	confidence := 0.3 // Base confidence

	text := strings.Join(lines, " ")
	upper := strings.ToUpper(text)

	// Check for Pokemon card indicators
	if strings.Contains(upper, "HP") {
		confidence += 0.15
	}
	if strings.Contains(upper, "POKEMON") || strings.Contains(upper, "POKÉMON") {
		confidence += 0.1
	}
	if strings.Contains(upper, "WEAKNESS") || strings.Contains(upper, "RESISTANCE") {
		confidence += 0.1
	}
	if strings.Contains(upper, "RETREAT") {
		confidence += 0.05
	}
	if strings.Contains(upper, "ILLUS") || strings.Contains(upper, "ILLUSTRATOR") {
		confidence += 0.1
	}

	// Check for MTG card indicators
	if strings.Contains(upper, "CREATURE") || strings.Contains(upper, "INSTANT") ||
		strings.Contains(upper, "SORCERY") || strings.Contains(upper, "ENCHANTMENT") ||
		strings.Contains(upper, "ARTIFACT") {
		confidence += 0.2
	}

	// Check for card number pattern (XX/YY)
	cardNumPattern := regexp.MustCompile(`\d{1,4}\s*/\s*\d{1,4}`)
	if cardNumPattern.MatchString(text) {
		confidence += 0.2
	}

	// Check for copyright/trademark
	if strings.Contains(text, "©") || strings.Contains(upper, "NINTENDO") ||
		strings.Contains(upper, "WIZARDS") {
		confidence += 0.1
	}

	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// preprocessImageForOCR applies image preprocessing to improve OCR quality
// - Converts to grayscale
// - Enhances contrast
// - Returns PNG-encoded bytes
func preprocessImageForOCR(img image.Image) ([]byte, error) {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	// Create grayscale image with enhanced contrast
	gray := image.NewGray(bounds)

	// Convert to grayscale and collect histogram
	histogram := make([]int, 256)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			// Use luminosity formula for grayscale conversion
			lum := uint8((0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 256)
			gray.SetGray(x, y, color.Gray{Y: lum})
			histogram[lum]++
		}
	}

	// Find min/max for contrast stretching (ignore bottom/top 1%)
	total := width * height
	threshold := total / 100
	minVal, maxVal := 0, 255

	count := 0
	for i := 0; i < 256; i++ {
		count += histogram[i]
		if count >= threshold {
			minVal = i
			break
		}
	}

	count = 0
	for i := 255; i >= 0; i-- {
		count += histogram[i]
		if count >= threshold {
			maxVal = i
			break
		}
	}

	// Apply contrast stretching
	if maxVal > minVal {
		scale := 255.0 / float64(maxVal-minVal)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				oldVal := gray.GrayAt(x, y).Y
				newVal := int(float64(int(oldVal)-minVal) * scale)
				if newVal < 0 {
					newVal = 0
				}
				if newVal > 255 {
					newVal = 255
				}
				gray.SetGray(x, y, color.Gray{Y: uint8(newVal)})
			}
		}
	}

	// Encode as PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, gray); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// hasCardPatterns checks if OCR output contains patterns typical of trading cards
func hasCardPatterns(lines []string) bool {
	for _, line := range lines {
		upper := strings.ToUpper(line)
		// Check for HP pattern (Pokemon)
		if strings.Contains(upper, "HP") {
			return true
		}
		// Check for card number pattern (XXX/YYY)
		if strings.Contains(line, "/") {
			for i, c := range line {
				if c == '/' && i > 0 && i < len(line)-1 {
					// Check if there are digits around the slash
					if line[i-1] >= '0' && line[i-1] <= '9' && line[i+1] >= '0' && line[i+1] <= '9' {
						return true
					}
				}
			}
		}
		// Check for common MTG types
		if strings.Contains(upper, "CREATURE") || strings.Contains(upper, "INSTANT") ||
			strings.Contains(upper, "SORCERY") || strings.Contains(upper, "ARTIFACT") ||
			strings.Contains(upper, "ENCHANTMENT") {
			return true
		}
	}
	return false
}

// ProcessBase64Image processes a base64-encoded image
func (s *ServerOCRService) ProcessBase64Image(base64Data string) (*ServerOCRResult, error) {
	// Remove data URL prefix if present
	if idx := strings.Index(base64Data, ","); idx != -1 {
		base64Data = base64Data[idx+1:]
	}

	imageData, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return &ServerOCRResult{
			Error: fmt.Sprintf("invalid base64 data: %v", err),
		}, err
	}

	return s.ProcessImageBytes(imageData)
}

// splitAndCleanLines splits text into lines and removes empty/whitespace lines
func splitAndCleanLines(text string) []string {
	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return cleaned
}

// estimateConfidence estimates OCR confidence based on extracted text quality
func estimateConfidence(lines []string) float64 {
	if len(lines) == 0 {
		return 0.0
	}

	totalChars := 0
	alphanumericChars := 0

	for _, line := range lines {
		for _, c := range line {
			totalChars++
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == ' ' {
				alphanumericChars++
			}
		}
	}

	if totalChars == 0 {
		return 0.0
	}

	// Higher ratio of alphanumeric characters indicates cleaner OCR
	ratio := float64(alphanumericChars) / float64(totalChars)

	// Scale confidence based on ratio and number of lines
	confidence := ratio * 0.8
	if len(lines) >= 3 {
		confidence += 0.2
	} else if len(lines) >= 1 {
		confidence += 0.1
	}

	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// validateImagePath validates and sanitizes an image path to prevent command injection
// and path traversal attacks.
func (s *ServerOCRService) validateImagePath(imagePath string) (string, error) {
	// Clean the path to remove any ".." or other traversal attempts
	cleanPath := filepath.Clean(imagePath)

	// Convert to absolute path
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	// Verify the file exists and is a regular file (not a directory, symlink, etc.)
	info, err := os.Lstat(absPath)
	if err != nil {
		return "", fmt.Errorf("file not found: %w", err)
	}

	// Only allow regular files, not symlinks (to prevent symlink attacks)
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("symbolic links are not allowed")
	}

	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("path is not a regular file")
	}

	// Verify it's likely an image file by checking extension
	ext := strings.ToLower(filepath.Ext(absPath))
	allowedExtensions := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".bmp":  true,
		".tiff": true,
		".tif":  true,
		".webp": true,
	}
	if !allowedExtensions[ext] {
		return "", fmt.Errorf("unsupported image format: %s", ext)
	}

	return absPath, nil
}
