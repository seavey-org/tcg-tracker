package services

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// ImageStorageService handles storing and retrieving scanned card images
type ImageStorageService struct {
	storageDir string
}

// NewImageStorageService creates a new image storage service
func NewImageStorageService() *ImageStorageService {
	storageDir := os.Getenv("SCANNED_IMAGES_DIR")
	if storageDir == "" {
		storageDir = "./data/scanned_images"
	}

	// Ensure the storage directory exists
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		// Log error but don't fail - will fail on actual writes
		fmt.Printf("Warning: could not create scanned images directory: %v\n", err)
	}

	return &ImageStorageService{
		storageDir: storageDir,
	}
}

// SaveImage saves image data to disk and returns the filename
func (s *ImageStorageService) SaveImage(imageData []byte) (string, error) {
	if len(imageData) == 0 {
		return "", fmt.Errorf("empty image data")
	}

	// Generate a unique filename
	filename := uuid.New().String() + ".jpg"
	filePath := filepath.Join(s.storageDir, filename)

	// Write the file
	if err := os.WriteFile(filePath, imageData, 0644); err != nil {
		return "", fmt.Errorf("failed to save image: %w", err)
	}

	return filename, nil
}

// GetStorageDir returns the storage directory path
func (s *ImageStorageService) GetStorageDir() string {
	return s.storageDir
}
