package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestPickedJoin (#1529): an arrival on its way in from the start joins at
// its plan's next STAR point, of its own STAR where that has the point,
// not the cheapest join from where it appeared.
func TestPickedJoin(t *testing.T) {
	join := func(star, ident string, rest float64) starJoin {
		return starJoin{pts: []airport.NavPoint{{Ident: ident}}, name: star, restNM: rest}
	}
	joins := []starJoin{join("GOLO4S", "ERASU", 20), join("LOMK8S", "PR511", 60), join("GOLO4S", "PR511", 50), join("LOMK8S", "ERASU", 20)}
	if j, ok := pickedJoin(joins, starPick{star: "LOMK8S", ident: "PR511"}); !ok || j.name != "LOMK8S" || j.pts[0].Ident != "PR511" {
		t.Errorf("picked %s at %s (%v), want LOMK8S at PR511", j.name, j.pts[0].Ident, ok)
	}
	if j, ok := pickedJoin(joins, starPick{star: "ABCD1S", ident: "PR511"}); !ok || j.pts[0].Ident != "PR511" {
		t.Errorf("another STAR's PR511 not picked: %v", ok)
	}
	if _, ok := pickedJoin(joins, starPick{star: "LOMK8S", ident: "NOPE"}); ok {
		t.Error("a point on no STAR picked")
	}
	if _, ok := pickedJoin(joins, starPick{}); ok {
		t.Error("nothing to pick, yet picked")
	}
}
