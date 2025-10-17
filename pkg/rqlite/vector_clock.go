package rqlite

// VectorClock represents a vector clock for distributed consistency
type VectorClock map[string]uint64

// NewVectorClock creates a new vector clock
func NewVectorClock() VectorClock {
	return make(VectorClock)
}

// Increment increments the clock for a given node
func (vc VectorClock) Increment(nodeID string) {
	vc[nodeID]++
}

// Update updates the vector clock with values from another clock
func (vc VectorClock) Update(other VectorClock) {
	for nodeID, value := range other {
		if existing, exists := vc[nodeID]; !exists || value > existing {
			vc[nodeID] = value
		}
	}
}

// Copy creates a copy of the vector clock
func (vc VectorClock) Copy() VectorClock {
	copy := make(VectorClock, len(vc))
	for k, v := range vc {
		copy[k] = v
	}
	return copy
}

// Compare compares two vector clocks
// Returns: -1 if vc < other, 0 if concurrent, 1 if vc > other
func (vc VectorClock) Compare(other VectorClock) int {
	less := false
	greater := false

	// Check all keys in both clocks
	allKeys := make(map[string]bool)
	for k := range vc {
		allKeys[k] = true
	}
	for k := range other {
		allKeys[k] = true
	}

	for k := range allKeys {
		v1 := vc[k]
		v2 := other[k]

		if v1 < v2 {
			less = true
		} else if v1 > v2 {
			greater = true
		}
	}

	if less && !greater {
		return -1 // vc < other
	} else if greater && !less {
		return 1 // vc > other
	}
	return 0 // concurrent
}

// HappensBefore checks if this clock happens before another
func (vc VectorClock) HappensBefore(other VectorClock) bool {
	return vc.Compare(other) == -1
}

// HappensAfter checks if this clock happens after another
func (vc VectorClock) HappensAfter(other VectorClock) bool {
	return vc.Compare(other) == 1
}

// IsConcurrent checks if two clocks are concurrent (neither happens before the other)
func (vc VectorClock) IsConcurrent(other VectorClock) bool {
	return vc.Compare(other) == 0
}
