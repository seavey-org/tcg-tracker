package services

import (
	"os"
	"testing"
)

func TestNewServerOCRService(t *testing.T) {
	service := NewServerOCRService()
	if service == nil {
		t.Fatal("NewServerOCRService returned nil")
	}

	if service.language != "eng" {
		t.Errorf("Expected language 'eng', got '%s'", service.language)
	}
}

func TestServerOCRServiceIsAvailable(t *testing.T) {
	service := NewServerOCRService()

	if service.identifierURL == "" {
		t.Fatal("Expected identifierURL to be set")
	}
}

func TestSplitAndCleanLines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Basic text",
			input:    "Line 1\nLine 2\nLine 3",
			expected: []string{"Line 1", "Line 2", "Line 3"},
		},
		{
			name:     "Text with empty lines",
			input:    "Line 1\n\nLine 2\n\n\nLine 3",
			expected: []string{"Line 1", "Line 2", "Line 3"},
		},
		{
			name:     "Text with whitespace lines",
			input:    "Line 1\n   \nLine 2\n\t\nLine 3",
			expected: []string{"Line 1", "Line 2", "Line 3"},
		},
		{
			name:     "Text with leading/trailing whitespace",
			input:    "  Line 1  \n  Line 2  ",
			expected: []string{"Line 1", "Line 2"},
		},
		{
			name:     "Empty text",
			input:    "",
			expected: nil,
		},
		{
			name:     "Only whitespace",
			input:    "   \n\n\t  ",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitAndCleanLines(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d lines, got %d", len(tt.expected), len(result))
				return
			}

			for i, line := range result {
				if line != tt.expected[i] {
					t.Errorf("Line %d: expected %q, got %q", i, tt.expected[i], line)
				}
			}
		})
	}
}

func TestEstimateConfidence(t *testing.T) {
	tests := []struct {
		name          string
		lines         []string
		minConfidence float64
		maxConfidence float64
	}{
		{
			name:          "Good quality text",
			lines:         []string{"Pikachu", "HP 60", "Lightning", "025/185"},
			minConfidence: 0.8,
			maxConfidence: 1.0,
		},
		{
			name:          "Single line",
			lines:         []string{"Charizard"},
			minConfidence: 0.5,
			maxConfidence: 1.0,
		},
		{
			name:          "Text with special characters",
			lines:         []string{"@#$%^&*()"},
			minConfidence: 0.0,
			maxConfidence: 0.3,
		},
		{
			name:          "Mixed quality",
			lines:         []string{"Pikachu @#$", "HP 60"},
			minConfidence: 0.4,
			maxConfidence: 0.9,
		},
		{
			name:          "Empty lines",
			lines:         []string{},
			minConfidence: 0.0,
			maxConfidence: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confidence := estimateConfidence(tt.lines)

			if confidence < tt.minConfidence {
				t.Errorf("Confidence %v is less than minimum expected %v", confidence, tt.minConfidence)
			}

			if confidence > tt.maxConfidence {
				t.Errorf("Confidence %v is greater than maximum expected %v", confidence, tt.maxConfidence)
			}
		})
	}
}

func TestProcessBase64ImageInvalid(t *testing.T) {
	service := NewServerOCRService()

	// Test with invalid base64
	result, err := service.ProcessBase64Image("not-valid-base64!@#$")
	if err == nil {
		t.Error("Expected error for invalid base64")
	}
	if result.Error == "" {
		t.Error("Expected error message in result")
	}
}

func TestProcessImageBytesInvalid(t *testing.T) {
	service := NewServerOCRService()

	// Test with invalid image data
	result, err := service.ProcessImageBytes([]byte("not an image"))
	if err == nil {
		t.Error("Expected error for invalid image data")
	}
	if result.Error == "" {
		t.Error("Expected error message in result")
	}
}

func TestProcessImageFileNotFound(t *testing.T) {
	service := NewServerOCRService()

	if !service.IsAvailable() {
		t.Skip("Identifier OCR service not available")
	}

	// Test with non-existent file
	result, err := service.ProcessImage("/nonexistent/path/to/image.png")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if result.Error == "" {
		t.Error("Expected error message in result")
	}
}

// Integration test that requires actual image file
func TestProcessImageIntegration(t *testing.T) {
	service := NewServerOCRService()

	if !service.IsAvailable() {
		t.Skip("Identifier OCR service not available")
	}

	// Create a simple test image with text
	// This test would require creating an actual test image
	// For now, skip if no test images directory exists
	testImagePath := "testdata/pokemon_card.jpg"
	if _, err := os.Stat(testImagePath); os.IsNotExist(err) {
		t.Skip("Test image not found at " + testImagePath)
	}

	result, err := service.ProcessImage(testImagePath)
	if err != nil {
		t.Fatalf("ProcessImage failed: %v", err)
	}

	if result.Text == "" {
		t.Error("Expected non-empty OCR text")
	}

	if len(result.Lines) == 0 {
		t.Error("Expected non-empty lines")
	}

	if result.Confidence <= 0 {
		t.Error("Expected positive confidence")
	}
}
