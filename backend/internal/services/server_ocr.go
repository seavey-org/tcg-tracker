package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	_ "image/jpeg"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ServerOCRService provides server-side OCR processing using the identifier service
type ServerOCRService struct {
	language      string
	identifierURL string
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
	identifierURL := strings.TrimSpace(os.Getenv("IDENTIFIER_SERVICE_URL"))
	if identifierURL == "" {
		identifierURL = "http://127.0.0.1:8099"
	}

	return &ServerOCRService{
		identifierURL: identifierURL,
		language:      "eng",
	}
}

// IsAvailable checks if the identifier OCR service is reachable
func (s *ServerOCRService) IsAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(s.identifierURL, "/")+"/health", nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
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

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return &ServerOCRResult{
			Error: fmt.Sprintf("failed to read image: %v", err),
		}, err
	}

	return s.ProcessImageBytes(data)
}

// ProcessImageBytes processes image data directly without saving to file
// Uses identifier service OCR (EasyOCR) for better accuracy
func (s *ServerOCRService) ProcessImageBytes(imageData []byte) (*ServerOCRResult, error) {
	payload := map[string]string{
		"image_b64": base64.StdEncoding.EncodeToString(imageData),
		"backend":   "easyocr",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return &ServerOCRResult{
			Error: fmt.Sprintf("failed to serialize OCR request: %v", err),
		}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	url := strings.TrimRight(s.identifierURL, "/") + "/ocr"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return &ServerOCRResult{
			Error: fmt.Sprintf("failed to create OCR request: %v", err),
		}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return &ServerOCRResult{
			Error: fmt.Sprintf("OCR service request failed: %v", err),
		}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return &ServerOCRResult{
			Error: fmt.Sprintf("OCR service error: %s", strings.TrimSpace(string(data))),
		}, fmt.Errorf("ocr service status %d", resp.StatusCode)
	}

	var ocrResp struct {
		Text       string   `json:"text"`
		Lines      []string `json:"lines"`
		Confidence float64  `json:"confidence"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ocrResp); err != nil {
		return &ServerOCRResult{
			Error: fmt.Sprintf("failed to decode OCR response: %v", err),
		}, err
	}

	lines := ocrResp.Lines
	if len(lines) == 0 && strings.TrimSpace(ocrResp.Text) != "" {
		lines = splitAndCleanLines(ocrResp.Text)
	}

	return &ServerOCRResult{
		Text:       ocrResp.Text,
		Lines:      lines,
		Confidence: ocrResp.Confidence,
	}, nil
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
