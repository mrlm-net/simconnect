//go:build windows

package simconnect

import (
	"math"
	"testing"
)

// A double goes to the DLL as its IEEE-754 bits, not as a pointer.
func TestFloat64Arg(t *testing.T) {
	for _, v := range []float64{0, 0.5, 1, -3.25, math.MaxFloat64} {
		if got := float64Arg(v); math.Float64frombits(uint64(got)) != v {
			t.Errorf("float64Arg(%v) = %#x", v, got)
		}
	}
}
