package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type SetIDCandidate struct {
	SetID string  `json:"set_id"`
	Score float64 `json:"score"`
}

type SetIDResult struct {
	BestSetID     string           `json:"best_set_id"`
	Confidence    float64          `json:"confidence"`
	LowConfidence bool             `json:"low_confidence"`
	Candidates    []SetIDCandidate `json:"candidates"`
	TimingsMS     map[string]int   `json:"timings_ms"`
}

type SetIDHealthResponse struct {
	Status      string   `json:"status"`
	IndexDir    string   `json:"index_dir"`
	GamesLoaded []string `json:"games_loaded"`
}

type SetIdentifierService struct {
	baseURL string
	client  *http.Client

	// Cached health status
	mu              sync.RWMutex
	lastHealthCheck time.Time
	cachedHealthy   bool
	cachedGames     []string
}

func NewSetIdentifierService() *SetIdentifierService {
	baseURL := os.Getenv("SET_ID_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8099"
	}

	svc := &SetIdentifierService{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 25 * time.Second,
		},
	}

	// Run initial health check in background
	go svc.checkHealth()

	return svc
}

func (s *SetIdentifierService) IsConfigured() bool {
	if s.baseURL == "" {
		return false
	}
	// Default is local dev; treat as "not configured" unless explicitly set.
	return os.Getenv("SET_ID_SERVICE_URL") != ""
}

// IsHealthy returns true if the identifier service is reachable and has indexes loaded.
// Uses cached result (refreshed every 60 seconds) to avoid blocking on every request.
func (s *SetIdentifierService) IsHealthy() bool {
	s.mu.RLock()
	if time.Since(s.lastHealthCheck) < 60*time.Second {
		healthy := s.cachedHealthy
		s.mu.RUnlock()
		return healthy
	}
	s.mu.RUnlock()

	return s.checkHealth()
}

// GamesLoaded returns the list of games with loaded indexes.
func (s *SetIdentifierService) GamesLoaded() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cachedGames
}

func (s *SetIdentifierService) checkHealth() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/health", nil)
	if err != nil {
		s.updateHealthCache(false, nil)
		return false
	}

	resp, err := s.client.Do(req)
	if err != nil {
		log.Printf("[SetIdentifier] health check failed: %v", err)
		s.updateHealthCache(false, nil)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.updateHealthCache(false, nil)
		return false
	}

	var health SetIDHealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		s.updateHealthCache(false, nil)
		return false
	}

	healthy := health.Status == "ok" && len(health.GamesLoaded) > 0
	s.updateHealthCache(healthy, health.GamesLoaded)
	return healthy
}

func (s *SetIdentifierService) updateHealthCache(healthy bool, games []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastHealthCheck = time.Now()
	s.cachedHealthy = healthy
	s.cachedGames = games
}

func (s *SetIdentifierService) IdentifyFromImageBytes(ctx context.Context, game string, img []byte) (*SetIDResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	if err := w.WriteField("game", game); err != nil {
		return nil, fmt.Errorf("write game field: %w", err)
	}

	fw, err := w.CreateFormFile("image", filepath.Base("card.jpg"))
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := fw.Write(img); err != nil {
		return nil, fmt.Errorf("write image: %w", err)
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/identify-set", &buf)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("identify set request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var payload map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&payload)
		return nil, fmt.Errorf("identify set failed status=%d", resp.StatusCode)
	}

	var out SetIDResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode identify set response: %w", err)
	}

	return &out, nil
}
