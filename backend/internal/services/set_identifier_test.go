package services

import (
	"testing"
)

func TestSetIdentifierService_IsConfigured(t *testing.T) {
	// Service is now stubbed - always returns false
	svc := NewSetIdentifierService()
	if svc.IsConfigured() {
		t.Error("Expected IsConfigured() = false (service removed)")
	}
}

func TestSetIdentifierService_IsHealthy(t *testing.T) {
	// Service is now stubbed - always returns false
	svc := NewSetIdentifierService()
	if svc.IsHealthy() {
		t.Error("Expected IsHealthy() = false (service removed)")
	}
}

func TestSetIdentifierService_GamesLoaded(t *testing.T) {
	// Service is now stubbed - always returns nil
	svc := NewSetIdentifierService()
	games := svc.GamesLoaded()
	if games != nil {
		t.Errorf("Expected GamesLoaded() = nil, got %v", games)
	}
}
