package world

import (
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// A number with nothing to do is not a call, and a sequence call with a
// vector says "for spacing" once, at the end (#1428; live, LKPR: TVS149
// told "Skytravel 149, number 2" alone, and "number 3, for spacing reduce
// speed to 180 knots, vectoring …, fly heading 125, for spacing").
func TestSequencePhrases1428(t *testing.T) {
	if !numberOnly(traffic.Sequenced("TVS149", 2, 0, traffic.Absorption{})) {
		t.Error("a bare number not seen as one")
	}
	if numberOnly(traffic.Sequenced("TVS149", 2, 0, traffic.Absorption{SpeedKts: 180})) {
		t.Error("a number with a speed seen as bare")
	}
	if numberOnly(traffic.Sequenced("TVS149", 0, 0, traffic.Absorption{SpeedKts: 180})) {
		t.Error("a speed alone seen as a bare number")
	}
	tx := traffic.SpacingTold(traffic.Sequenced("TVS149", 3, 0, traffic.Absorption{SpeedKts: 180}))
	vx := traffic.Vectored("TVS149", traffic.Vector{HeadingDeg: 125, For: "spacing"}, 0)
	got := traffic.Joined(tx, vx).Text
	if n := strings.Count(got, "for spacing"); n != 1 || !strings.HasSuffix(got, "for spacing") {
		t.Errorf("%q: \"for spacing\" %d times, want once at the end", got, n)
	}
	_ = time.Second
}
