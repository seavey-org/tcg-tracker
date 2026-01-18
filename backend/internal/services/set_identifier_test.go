package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSetIdentifierService_IsConfigured(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     bool
	}{
		{
			name:     "configured when env is set",
			envValue: "http://localhost:8099",
			want:     true,
		},
		{
			name:     "not configured when env is empty",
			envValue: "",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SET_ID_SERVICE_URL", tt.envValue)
			svc := NewSetIdentifierService()
			if got := svc.IsConfigured(); got != tt.want {
				t.Errorf("IsConfigured() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetIdentifierService_checkHealth(t *testing.T) {
	t.Run("healthy when service responds with games", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				json.NewEncoder(w).Encode(map[string]any{
					"status":       "ok",
					"index_dir":    "/opt/tcg-tracker/setid/indexes",
					"games_loaded": []string{"pokemon", "mtg"},
				})
			}
		}))
		defer server.Close()

		t.Setenv("SET_ID_SERVICE_URL", server.URL)
		svc := NewSetIdentifierService()

		// Wait for background health check to complete
		time.Sleep(100 * time.Millisecond)

		if !svc.IsHealthy() {
			t.Error("Expected IsHealthy() = true")
		}

		games := svc.GamesLoaded()
		if len(games) != 2 {
			t.Errorf("Expected 2 games loaded, got %d", len(games))
		}
	})

	t.Run("unhealthy when service has no games", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				json.NewEncoder(w).Encode(map[string]any{
					"status":       "ok",
					"index_dir":    "/opt/tcg-tracker/setid/indexes",
					"games_loaded": []string{},
				})
			}
		}))
		defer server.Close()

		t.Setenv("SET_ID_SERVICE_URL", server.URL)
		svc := NewSetIdentifierService()

		time.Sleep(100 * time.Millisecond)

		if svc.IsHealthy() {
			t.Error("Expected IsHealthy() = false when no games loaded")
		}
	})

	t.Run("unhealthy when service is down", func(t *testing.T) {
		t.Setenv("SET_ID_SERVICE_URL", "http://localhost:12345")
		svc := NewSetIdentifierService()

		time.Sleep(100 * time.Millisecond)

		if svc.IsHealthy() {
			t.Error("Expected IsHealthy() = false when service is down")
		}
	})
}

func TestSetIdentifierService_IdentifyFromImageBytes(t *testing.T) {
	t.Run("returns result on success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				json.NewEncoder(w).Encode(map[string]any{
					"status":       "ok",
					"games_loaded": []string{"pokemon"},
				})
				return
			}
			if r.URL.Path == "/identify-set" {
				json.NewEncoder(w).Encode(map[string]any{
					"best_set_id":    "swsh4",
					"confidence":     0.85,
					"low_confidence": false,
					"candidates": []map[string]any{
						{"set_id": "swsh4", "score": 0.85},
						{"set_id": "swsh5", "score": 0.65},
					},
					"timings_ms": map[string]int{
						"total": 150,
						"crop":  20,
						"embed": 100,
					},
				})
			}
		}))
		defer server.Close()

		t.Setenv("SET_ID_SERVICE_URL", server.URL)
		svc := NewSetIdentifierService()

		// Small test image (1x1 pixel JPEG-like bytes)
		img := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}

		result, err := svc.IdentifyFromImageBytes(context.Background(), "pokemon", img)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if result.BestSetID != "swsh4" {
			t.Errorf("BestSetID = %q, want %q", result.BestSetID, "swsh4")
		}

		if result.Confidence != 0.85 {
			t.Errorf("Confidence = %f, want 0.85", result.Confidence)
		}

		if len(result.Candidates) != 2 {
			t.Errorf("len(Candidates) = %d, want 2", len(result.Candidates))
		}
	})

	t.Run("returns error on failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				json.NewEncoder(w).Encode(map[string]any{
					"status":       "ok",
					"games_loaded": []string{"pokemon"},
				})
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		t.Setenv("SET_ID_SERVICE_URL", server.URL)
		svc := NewSetIdentifierService()

		img := []byte{0xFF, 0xD8, 0xFF}

		_, err := svc.IdentifyFromImageBytes(context.Background(), "pokemon", img)
		if err == nil {
			t.Error("Expected error, got nil")
		}
	})
}
