package node

import (
	"testing"
	"time"
)

func TestCalculateNextBackoff(t *testing.T) {
	if got := calculateNextBackoff(10 * time.Second); got <= 10*time.Second || got > 15*time.Second {
		t.Fatalf("unexpected next: %v", got)
	}
	if got := calculateNextBackoff(10 * time.Minute); got != 10*time.Minute {
		t.Fatalf("cap not applied: %v", got)
	}
}

func TestAddJitter(t *testing.T) {
	base := 10 * time.Second
	min := base - time.Duration(0.2*float64(base))
	max := base + time.Duration(0.2*float64(base))
	for i := 0; i < 100; i++ {
		got := addJitter(base)
		if got < time.Second || got < min || got > max {
			t.Fatalf("jitter out of range: %v", got)
		}
	}
}
