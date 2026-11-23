package world

import (
	"testing"
	"time"
)

// TestGameWaitFromSTD: a crew calls DepartureLead before its STD; the wait
// counts from the later of the call and the STD, and a departure with no
// STD from its call (#1675).
func TestGameWaitFromSTD(t *testing.T) {
	std := time.Date(2026, 11, 22, 14, 5, 0, 0, time.UTC)
	for _, c := range []struct {
		std, now time.Time
		want     bool
	}{
		{std, std.Add(-5 * time.Minute), false},
		{std, std.Add(-time.Second), false},
		{std, std, true},
		{std, std.Add(time.Minute), true},
		{time.Time{}, std, true},
	} {
		if got := pastTime(c.std, c.now); got != c.want {
			t.Errorf("STD %v at %v: %v, want %v", c.std, c.now, got, c.want)
		}
	}
}
