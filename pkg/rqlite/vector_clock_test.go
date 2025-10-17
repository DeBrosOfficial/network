package rqlite

import (
	"testing"
)

func TestVectorClock_Increment(t *testing.T) {
	vc := NewVectorClock()

	// Initial state
	if vc["nodeA"] != 0 {
		t.Errorf("Expected initial value 0, got %d", vc["nodeA"])
	}

	// First increment
	vc.Increment("nodeA")
	if vc["nodeA"] != 1 {
		t.Errorf("Expected value 1 after first increment, got %d", vc["nodeA"])
	}

	// Second increment
	vc.Increment("nodeA")
	if vc["nodeA"] != 2 {
		t.Errorf("Expected value 2 after second increment, got %d", vc["nodeA"])
	}

	// Different node
	vc.Increment("nodeB")
	if vc["nodeB"] != 1 {
		t.Errorf("Expected nodeB value 1, got %d", vc["nodeB"])
	}
	if vc["nodeA"] != 2 {
		t.Errorf("Expected nodeA value still 2, got %d", vc["nodeA"])
	}
}

func TestVectorClock_Merge(t *testing.T) {
	vc1 := NewVectorClock()
	vc1["nodeA"] = 5
	vc1["nodeB"] = 3

	vc2 := NewVectorClock()
	vc2["nodeB"] = 7
	vc2["nodeC"] = 2

	vc1.Merge(vc2)

	// Should take max values
	if vc1["nodeA"] != 5 {
		t.Errorf("Expected nodeA=5, got %d", vc1["nodeA"])
	}
	if vc1["nodeB"] != 7 {
		t.Errorf("Expected nodeB=7 (max of 3 and 7), got %d", vc1["nodeB"])
	}
	if vc1["nodeC"] != 2 {
		t.Errorf("Expected nodeC=2, got %d", vc1["nodeC"])
	}
}

func TestVectorClock_Compare_StrictlyLess(t *testing.T) {
	vc1 := NewVectorClock()
	vc1["nodeA"] = 1
	vc1["nodeB"] = 2

	vc2 := NewVectorClock()
	vc2["nodeA"] = 2
	vc2["nodeB"] = 3

	result := vc1.Compare(vc2)
	if result != -1 {
		t.Errorf("Expected -1 (strictly less), got %d", result)
	}
}

func TestVectorClock_Compare_StrictlyGreater(t *testing.T) {
	vc1 := NewVectorClock()
	vc1["nodeA"] = 5
	vc1["nodeB"] = 4

	vc2 := NewVectorClock()
	vc2["nodeA"] = 3
	vc2["nodeB"] = 2

	result := vc1.Compare(vc2)
	if result != 1 {
		t.Errorf("Expected 1 (strictly greater), got %d", result)
	}
}

func TestVectorClock_Compare_Concurrent(t *testing.T) {
	vc1 := NewVectorClock()
	vc1["nodeA"] = 5
	vc1["nodeB"] = 2

	vc2 := NewVectorClock()
	vc2["nodeA"] = 3
	vc2["nodeB"] = 4

	result := vc1.Compare(vc2)
	if result != 0 {
		t.Errorf("Expected 0 (concurrent), got %d", result)
	}
}

func TestVectorClock_Compare_Identical(t *testing.T) {
	vc1 := NewVectorClock()
	vc1["nodeA"] = 5
	vc1["nodeB"] = 3

	vc2 := NewVectorClock()
	vc2["nodeA"] = 5
	vc2["nodeB"] = 3

	result := vc1.Compare(vc2)
	if result != 0 {
		t.Errorf("Expected 0 (identical), got %d", result)
	}
}

func TestVectorClock_String(t *testing.T) {
	vc := NewVectorClock()
	vc["nodeA"] = 5
	vc["nodeB"] = 3

	str := vc.String()
	// Should contain both nodes
	if str == "" {
		t.Error("Expected non-empty string representation")
	}
	// Basic format check
	if str[0] != '{' || str[len(str)-1] != '}' {
		t.Errorf("Expected string wrapped in braces, got %s", str)
	}
}
