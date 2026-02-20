package services

import (
	"testing"
	"time"
)

func TestIsFresh(t *testing.T) {
	svc := &PriceService{}

	// nil should not be fresh
	if svc.isFresh(nil) {
		t.Error("nil time should not be fresh")
	}

	// Time within threshold should be fresh
	recent := time.Now().Add(-1 * time.Hour)
	if !svc.isFresh(&recent) {
		t.Error("Time 1 hour ago should be fresh")
	}

	// Time at exactly threshold should be fresh
	threshold := time.Now().Add(-PriceStalenessThreshold + time.Minute)
	if !svc.isFresh(&threshold) {
		t.Error("Time just within threshold should be fresh")
	}

	// Time beyond threshold should not be fresh
	old := time.Now().Add(-PriceStalenessThreshold - time.Hour)
	if svc.isFresh(&old) {
		t.Error("Time beyond threshold should not be fresh")
	}
}

func TestGetJustTCGRequestsRemaining(t *testing.T) {
	// With nil JustTCG service
	svc := &PriceService{}
	if svc.GetJustTCGRequestsRemaining() != 0 {
		t.Error("Should return 0 when JustTCG service is nil")
	}

	// With JustTCG service (daily limit 100, monthly limit 1000)
	justTCG := NewJustTCGService("", 100, 1000)
	svc = &PriceService{justTCG: justTCG}
	remaining := svc.GetJustTCGRequestsRemaining()
	if remaining != 100 {
		t.Errorf("Expected 100 remaining (daily is tighter), got %d", remaining)
	}
}
