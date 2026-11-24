package world

import "testing"

// A crew's own go-around never keeps the old landing clearance (#1680):
// its crew round already, not stable at the gate (#1681's crewRound), its
// landing clearance and a held one are dropped and it is sequenced again
// (rejoin), as after the tower's go-around; not sent round a second time.
func TestCrewRoundDropsLandingClearance(t *testing.T) {
	crewRoundDropsLandingClearance(t, func(cc *controlCenter, it *controlled) { cc.crewUnstable(it, true) })
}

// crewRoundDropsLandingClearance checks round, a crew's own go-around
// flown already, on an arrival cleared to land.
func crewRoundDropsLandingClearance(t *testing.T, round func(cc *controlCenter, it *controlled)) {
	t.Helper()
	cc, it, arr := unstableBed(t)
	rejoined := ""
	cc.rejoin = func(_, tail string) { rejoined = tail }
	it.mu.Lock()
	it.landCleared, it.landHeld, it.view.LandHeld = true, "N123AB", "N123AB"
	it.mu.Unlock()
	round(cc, it)
	it.mu.Lock()
	cleared, held, view := it.landCleared, it.landHeld, it.view.LandHeld
	it.mu.Unlock()
	if cleared {
		t.Error("still cleared to land after its crew's go-around")
	}
	if held != "" || view != "" {
		t.Errorf("a held landing clearance kept: %q (view %q)", held, view)
	}
	if rejoined != it.Tail {
		t.Errorf("not sequenced again: rejoined %q", rejoined)
	}
	if arr.sent != 0 {
		t.Errorf("sent round again %d times: its crew had gone round already", arr.sent)
	}
}
