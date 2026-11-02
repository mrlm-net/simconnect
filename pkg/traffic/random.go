package traffic

import (
	"math/rand/v2"
	"sync"
)

// The traffic's random choices, replayable: seeded (SetRandomSeed), a test's
// run draws the same numbers again; otherwise the process's random numbers.
var randomSource struct {
	mu      sync.Mutex
	r       *rand.Rand
	seed    uint64
	streams uint64
}

// SetRandomSeed seeds the traffic's random choices process-wide, as
// SetTimeSource sets its clock; 0 puts them back to unseeded. For tests.
func SetRandomSeed(seed uint64) {
	s := &randomSource
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seed, s.streams, s.r = seed, 0, nil
	if seed != 0 {
		s.r = rand.New(rand.NewPCG(seed, 0x7a11))
	}
}

// RandomFloat64 is rand.Float64 from the traffic's source.
func RandomFloat64() float64 {
	s := &randomSource
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.r != nil {
		return s.r.Float64()
	}
	return rand.Float64()
}

// RandomIntN is rand.IntN from the traffic's source.
func RandomIntN(n int) int {
	s := &randomSource
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.r != nil {
		return s.r.IntN(n)
	}
	return rand.IntN(n)
}

// RandomInt64N is rand.Int64N from the traffic's source.
func RandomInt64N(n int64) int64 {
	s := &randomSource
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.r != nil {
		return s.r.Int64N(n)
	}
	return rand.Int64N(n)
}

// NewRandom is a generator of its own for a controller or a service: from the
// clock, or, seeded, the next stream of the seed.
func NewRandom(stream uint64) *rand.Rand {
	s := &randomSource
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.r == nil {
		return rand.New(rand.NewPCG(uint64(Now().UnixNano()), stream))
	}
	s.streams++
	return rand.New(rand.NewPCG(s.seed+s.streams, stream))
}
