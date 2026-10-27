package world

import (
	"strings"
	"testing"
)

// Behind one of ours early on its roll, the line-up is conditional; late
// in it (about to lift off) or off the runway, a plain line-up (the user,
// live: "behind the departing Boeing 737, line up and wait runway 24,
// behind" with the 737 about to lift off). Ahead of its way in (an
// intersection departure), never conditional (Doc 4444 12.2.7; live,
// TVS737 behind the Cirrus rolling from B).
func TestLineUpBehindDepartingSaid(t *testing.T) {
	for _, c := range []struct {
		kts     float64
		rolling bool
		ahead   bool
		behind  bool
	}{{30, true, false, true}, {79, true, false, true}, {80, true, false, false}, {140, true, false, false}, {0, false, false, false}, {30, true, true, false}} {
		tx := lineUpBehindDepartingSaid("OKKBC", "Boeing 737", "24", c.kts, c.rolling, c.ahead)
		if got := strings.Contains(tx.Text, "behind"); got != c.behind {
			t.Errorf("%v kt (rolling %v): %q, behind %v, want %v", c.kts, c.rolling, tx.Text, got, c.behind)
		}
		if !strings.Contains(tx.Text, "line up and wait") {
			t.Errorf("%v kt: %q, no line-up", c.kts, tx.Text)
		}
	}
}
