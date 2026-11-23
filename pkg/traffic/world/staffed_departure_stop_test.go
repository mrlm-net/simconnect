package world

import (
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// climbDep is a departure handed over that only keeps its climb.
type climbDep struct {
	departureCtl
	mu     sync.Mutex
	route  []traffic.RoutePoint
	climbs []float64
}

func (d *climbDep) ClimbPlan(airport.LatLon) []traffic.RoutePoint {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.route)
}

func (d *climbDep) ClimbRoute(airport.LatLon) []airport.LatLon {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []airport.LatLon
	for _, p := range d.route {
		out = append(out, p.Position)
	}
	return out
}

func (d *climbDep) Reroute(r []traffic.RoutePoint) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.route = slices.Clone(r)
	return nil
}

func (d *climbDep) ClimbTo(pos airport.LatLon, altFt, ft float64) error {
	return d.ClimbExpedited(pos, altFt, ft, 0)
}

func (d *climbDep) ClimbExpedited(_ airport.LatLon, _, ft, _ float64) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.climbs = append(d.climbs, ft)
	for i := range d.route {
		if d.route[i].AltFt < ft {
			d.route[i].AltFt = ft
		}
	}
	return nil
}

func (d *climbDep) cleared() []float64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.climbs)
}

const stopFt, onFt = 37000.0, 39000.0

// stoppedDeparture is the meeting bed's western aircraft as a departure of
// ours climbing through 35,000 ft to onFt, cleared there by departure, and
// DLH836 level at FL380 20 NM ahead head on, not to be steered: the watch's own look, departure
// the AI's, stops its climb at stopFt for it. Then the traffic is gone
// (#1665).
func stoppedDeparture(t *testing.T) (*meetingBed, *controlled, *climbDep) {
	t.Helper()
	b := newMeetingBed(t)
	delete(b.s.enroute, "KLM221")
	delete(b.s.enroute, "DLH836")
	start, end := b.west.route[0], b.west.route[1]
	start.AltFt, end.AltFt = 35000, onFt
	// Its route climbing to onFt 10 NM on: through DLH836's level.
	top := end
	top.Position.Lat, top.Position.Lon = calc.DisplaceByHeading(start.Position.Lat, start.Position.Lon, 90, 10*1852)
	d := &climbDep{route: []traffic.RoutePoint{start, top, end}}
	cc := b.s.cc
	west, east := &b.air[0], &b.air[1]
	west.AltFt, west.AGLFt, west.VSFpm, west.Phase = 35000, 34000, 2000, traffic.PhaseClimbing
	east.AltFt, east.AGLFt = stopFt+1000, stopFt // ours, not steered: no longer en route
	east.Position.Lat, east.Position.Lon = calc.DisplaceByHeading(west.Position.Lat, west.Position.Lon, 90, 20*1852)
	it := &controlled{cc: cc, ICAO: "LKPR", Tail: "KLM221", dep: d}
	it.objectID, it.atc, it.identified = westID, traffic.PosDeparture, true
	it.radarFt, it.clearedFt = onFt, 5000
	it.view.State = traffic.TaxiComplete.String()
	cc.items[1] = it
	cc.climbStopped = b.cw.isStopped
	cc.world.Observe(b.now, []traffic.Observation{{ObjectID: 1, Tail: "USER", Position: west.Position, OnGround: true, User: true}, west.Observation, east.Observation})
	b.cw.tick(b.now.Add(-conflictEvery), b.air) // a look before the tests' own
	_, done := b.cw.view()
	if len(done) != 1 || done[0].Resolution.Kind != traffic.ResolveLevel || !done[0].Resolution.Stop || done[0].Resolution.AltFt != stopFt || !b.cw.isStopped("KLM221") {
		t.Fatalf("the watch's look: resolutions %+v, stopped %v; want its climb stopped at %.0f ft", done, b.cw.isStopped("KLM221"), stopFt)
	}
	d.mu.Lock()
	d.climbs = nil // the stop is not a climb
	d.mu.Unlock()
	// The traffic gone: nothing for its climb to meet.
	b.air = b.air[:1]
	cc.world = traffic.NewTrafficPicture(traffic.PictureOptions{Centre: traffic.Centre{FollowUser: true}})
	cc.world.Observe(b.now, []traffic.Observation{{ObjectID: 1, Tail: "USER", Position: west.Position, OnGround: true, User: true}, b.air[0].Observation})
	return b, it, d
}

// Not staffed, the watch clears it on: the bed has the case.
func TestDepartureStopClearedOnUnstaffed(t *testing.T) {
	b, _, d := stoppedDeparture(t)
	b.look(0)
	if got := d.cleared(); !slices.Equal(got, []float64{onFt}) || b.cw.isStopped("KLM221") {
		t.Fatalf("climbs %v, stopped %v; want cleared on to %.0f", got, b.cw.isStopped("KLM221"), onFt)
	}
}

// Staffed, in every mode, the stop and the climb after it are the
// player's: not cleared on, not climbing on as the change ends.
func TestDepartureStopIsPlayers(t *testing.T) {
	for _, mode := range []string{ModeAssist, ModeSilent, ModeSafetyNet} {
		for _, gated := range []bool{false, true} { // before and after checkStaffed's look
			b, it, d := stoppedDeparture(t)
			b.staff(StaffedStation{Position: string(traffic.PosDeparture), Mode: mode})
			it.gates.Store(gated)
			b.look(0)
			b.look(conflictLookAhead + time.Minute)
			if got := d.cleared(); len(got) != 0 || !b.cw.isStopped("KLM221") {
				t.Errorf("%s, gated %v: climbs %v, stopped %v; want the stop left to the player", mode, gated, got, b.cw.isStopped("KLM221"))
			}
			for _, p := range d.ClimbPlan(airport.LatLon{}) {
				if p.AltFt > stopFt {
					t.Errorf("%s, gated %v: its route climbs on to %.0f ft by itself", mode, gated, p.AltFt)
					break
				}
			}
		}
	}
}

// The safety net's own stop is the player's as well: it stays until they
// climb it, and the AI clears it on only once departure is not staffed.
func TestDepartureNetStopIsPlayers(t *testing.T) {
	b, it, d := stoppedDeparture(t)
	st := b.cw.stopped["KLM221"]
	st.net = true
	b.cw.stopped["KLM221"] = st
	b.staff(StaffedStation{Position: string(traffic.PosDeparture), Mode: ModeSafetyNet})
	it.gates.Store(true)
	b.look(0)
	b.look(conflictLookAhead + time.Minute)
	if got := d.cleared(); len(got) != 0 || !b.cw.isStopped("KLM221") {
		t.Fatalf("staffed: climbs %v, stopped %v; want the net's stop left to the player", got, b.cw.isStopped("KLM221"))
	}
	for _, p := range d.ClimbPlan(airport.LatLon{}) {
		if p.AltFt > stopFt {
			t.Fatalf("staffed: its route climbs on to %.0f ft by itself", p.AltFt)
		}
	}
	b.staff()
	it.gates.Store(false)
	b.look(conflictLookAhead + 2*time.Minute)
	if got := d.cleared(); !slices.Equal(got, []float64{onFt}) {
		t.Errorf("unstaffed: climbs %v, want cleared on to %.0f", got, onFt)
	}
}

// The crew's levelling at the clearance's level doesn't undo the stop.
func TestDepartureStopNotRaisedByCrew(t *testing.T) {
	b, it, d := stoppedDeparture(t)
	it.clearedFt = onFt
	b.staff(StaffedStation{Position: string(traffic.PosDeparture)})
	it.gates.Store(true)
	b.s.cc.crewAsksHigher([]*controlled{it}, b.now)
	time.Sleep(50 * time.Millisecond) // it levels on a goroutine
	if got := d.cleared(); len(got) != 0 {
		t.Errorf("climbs %v over the stop", got)
	}
}

// The player's climb ends the stop: the AI has nothing more to say to it,
// staffed or after.
func TestDepartureStopPlayersClimbStands(t *testing.T) {
	b, it, d := stoppedDeparture(t)
	b.staff(StaffedStation{Position: string(traffic.PosDeparture)})
	it.gates.Store(true)
	b.look(0)
	if err := it.climbTo(stopFt+1000, 0, false); err != nil {
		t.Fatal(err)
	}
	b.staff()
	it.gates.Store(false)
	b.look(time.Minute)
	if got := d.cleared(); !slices.Equal(got, []float64{stopFt + 1000}) || b.cw.isStopped("KLM221") {
		t.Errorf("climbs %v, stopped %v; want the player's clearance alone", got, b.cw.isStopped("KLM221"))
	}
}

// Off the staffed station again, the AI clears it on: not stopped for good.
func TestDepartureStopBackToAI(t *testing.T) {
	b, it, d := stoppedDeparture(t)
	b.staff(StaffedStation{Position: string(traffic.PosDeparture)})
	it.gates.Store(true)
	b.look(0)
	if got := d.cleared(); len(got) != 0 {
		t.Fatalf("staffed: climbs %v, want none", got)
	}
	b.staff()
	it.gates.Store(false)
	b.look(time.Minute)
	if got := d.cleared(); !slices.Equal(got, []float64{onFt}) {
		t.Errorf("unstaffed again: climbs %v, want cleared on to %.0f", got, onFt)
	}
}

// A climb held for crossing traffic waits while departure is the player's.
func TestHeldClimbWaitsForPlayer(t *testing.T) {
	b, it, d := stoppedDeparture(t)
	b.staff(StaffedStation{Position: string(traffic.PosDeparture)})
	it.gates.Store(true)
	it.climbWhenClear()
	if got := d.cleared(); len(got) != 0 {
		t.Errorf("climbs %v while the player's", got)
	}
}
