package world

import (
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// rejoinArr is an arrival coming in on plan, noting what it is told.
type rejoinArr struct {
	arrivalCtl
	plan    *traffic.ArrivalPlan
	proc    []traffic.RoutePoint
	fixes   []airport.NavPoint
	absorbs []time.Duration
	stops   []float64
}

func (f *rejoinArr) Plan() *traffic.ArrivalPlan          { return f.plan }
func (f *rejoinArr) ProcedurePlan() []traffic.RoutePoint { return f.proc }
func (f *rejoinArr) State() traffic.ArrivalState         { return traffic.ArrivalApproaching }
func (f *rejoinArr) CircuitFixes() []airport.NavPoint    { return f.fixes }
func (f *rejoinArr) StopDescent(altFt, forNM float64) error {
	f.stops = append(f.stops, altFt)
	return nil
}
func (f *rejoinArr) AbsorbDelay(d time.Duration) (traffic.Absorption, error) {
	f.absorbs = append(f.absorbs, d)
	return traffic.Absorption{}, nil
}

// newRejoinBed is a go-around (JBU1547) 15 NM from its final's join and
// another arrival of ours 20 s behind it there, far apart and 7000 ft
// above: its slot is behind that one, and it has time to lose.
func newRejoinBed(t *testing.T) (*controlCenter, *controlled, *rejoinArr) {
	t.Helper()
	w := New(Options{DataDir: t.TempDir()})
	cc := newControlCenter(nil, w.st.core)
	thr := airport.LatLon{Lat: 18.04, Lon: -63.11}
	at := func(brg, nm float64) airport.LatLon {
		lat, lon := calc.DisplaceByHeading(thr.Lat, thr.Lon, brg, nm*1852)
		return airport.LatLon{Lat: lat, Lon: lon}
	}
	end := airport.RunwayEnd{Name: "10", Heading: 90}
	mine := &rejoinArr{plan: &traffic.ArrivalPlan{End: end}, proc: []traffic.RoutePoint{{Position: at(270, 5), AltFt: 3000, Kts: 180}}}
	other := &rejoinArr{plan: &traffic.ArrivalPlan{End: end}, proc: []traffic.RoutePoint{{Position: at(0, 25), AltFt: 10000, Kts: 180}}}
	me := &controlled{ID: 1, Tail: "JBU1547", ICAO: "TNCM", cc: cc, arr: mine, objectID: 11, removed: make(chan struct{})}
	cc.items[1] = me
	cc.items[2] = &controlled{ID: 2, Tail: "JBU1536", ICAO: "TNCM", cc: cc, arr: other, objectID: 12, removed: make(chan struct{})}
	cc.world.Observe(cc.clock.Now(), []traffic.Observation{
		{ObjectID: 1, Tail: "USER", Position: thr, OnGround: true, User: true},
		{ObjectID: 11, Tail: "JBU1547", Position: at(270, 20), AltFt: 3000, AGLFt: 3000, GroundKts: 180, Heading: 90},
		{ObjectID: 12, Tail: "JBU1536", Position: at(0, 41), AltFt: 10000, AGLFt: 10000, GroundKts: 180, Heading: 180},
	})
	return cc, me, mine
}

// A player's go-around is not given time to lose by the AI (#1096): in
// game mode JBU1547 was told to lose 58 s behind JBU1536 (#1648). The
// AI's own is.
func TestGoAroundRejoinPlayers(t *testing.T) {
	cc, me, mine := newRejoinBed(t)
	cc.planRejoin(me)
	if len(mine.absorbs) == 0 {
		t.Fatalf("the AI's go-around: no time to lose (the bed gives it a slot behind JBU1536)")
	}
	cc, me, mine = newRejoinBed(t)
	me.gates.Store(true)
	cc.planRejoin(me)
	if len(mine.absorbs) != 0 {
		t.Errorf("the player's go-around told to lose %v", mine.absorbs)
	}
}

// A go-around checks in with approach passing its height, whatever its
// missed approach climbs to: "passing 0 feet climbing flight level 140"
// (#1648; a departure's height was read).
func TestGoAroundCheckInPassing(t *testing.T) {
	for _, missedM := range []float64{0, 1219.2, 4267.2} {
		w := New(Options{DataDir: t.TempDir()})
		cc := newControlCenter(nil, w.st.core)
		arr := &rejoinArr{fixes: []airport.NavPoint{{AltMin: missedM}}}
		it := &controlled{Tail: "JBU1547", ICAO: "TNCM", cc: cc, arr: arr, graph: &airport.Graph{Layout: &airport.Layout{ICAO: "TNCM", Altitude: 3}}}
		it.aglFt = 1500
		got := it.checkInReport(traffic.PosApproach)
		if want := "going around, passing 1500 feet climbing "; !strings.HasPrefix(got, want) {
			t.Errorf("missed approach at %.0f m: %q, want %q…", missedM, got, want)
		}
	}
}
