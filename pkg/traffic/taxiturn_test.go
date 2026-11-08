package traffic

import (
	"math"

	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// groundRun is one injected ground move's frames (60 Hz) and placements.
type groundRun struct {
	s      []turnSample
	ec     *eventClient
	ok     bool
	graph  *airport.Graph
	center airport.LatLon
}

// runTaxiIn flies an injected arrival of model to stand from end and keeps
// every ground frame from the rollout under 60 kt on: the runway exit, the
// taxi-in and the stand.
func runTaxiIn(t *testing.T, g *airport.Graph, end string, stand int, model string) groundRun {
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj))
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: end, Parking: stand, Model: model, InjectApproach: true,
		RollThroughChance: -1, AfterLandingDwell: time.Second}); err != nil {
		return groundRun{}
	}
	go func() {
		for range ctl.Events() {
		}
	}()
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	mon := DefaultArrivalRequestBase + arrReqMonitor
	p := g.Layout.Parking[stand]
	r := groundRun{ec: ec, graph: g, center: p.Position}
	for i := 0; i < 60*3600 && !ctl.State().Terminal(); i++ {
		now = now.Add(time.Second / 60)
		ctl.Handle(arrivalPositionMsg(mon, 77, p.Position, 0, 0, 0, false))
		if st := ctl.State(); st >= ArrivalRollout && st <= ArrivalParked && ctl.mover != nil {
			if ps := ctl.mover.Pose(); ps.GroundSpeedKts < 60 {
				r.s = append(r.s, turnSample{pos: ps.Position, hdg: ps.Heading, kts: ps.GroundSpeedKts})
			}
		}
	}
	r.ok = ctl.State() == ArrivalParked
	return r
}

// runTaxiOut runs an injected departure of model from stand to end and keeps
// every frame from the end of the push to the line-up: the taxi-out and the
// runway entry.
func runTaxiOut(t *testing.T, g *airport.Graph, end string, stand int, model string) groundRun {
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(inj))
	if err := ctl.Start(TaxiRequest{Graph: g, Runway: end, Parking: stand, Model: model, Tail: "OKTST", RollingTakeoffChance: -1}); err != nil {
		return groundRun{}
	}
	go func() {
		for range ctl.Events() {
		}
	}()
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultTaxiRequestBase+reqOffSpawn, 77))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	mon := DefaultTaxiRequestBase + reqOffMonitor
	p := g.Layout.Parking[stand]
	r := groundRun{ec: ec, graph: g, center: p.Position}
	for i := 0; i < 60*1800 && ctl.State() != TaxiLinedUp && !ctl.State().Terminal(); i++ {
		now = now.Add(time.Second / 60)
		ctl.Handle(positionMsg(mon, 77, p.Position, 0, 0, true))
		if st := ctl.State(); st >= TaxiTaxiing && st <= TaxiLinedUp && ctl.mover != nil {
			ps := ctl.mover.Pose()
			r.s = append(r.s, turnSample{pos: ps.Position, hdg: ps.Heading, kts: ps.GroundSpeedKts})
		}
	}
	r.ok = ctl.State() == TaxiLinedUp
	return r
}

// groundCase is a type's moves at an airport, both ways.
type groundCase struct {
	icao       string
	ends       [2]string
	model      string
	stands     []string
	wantRadius bool // every turn at least groundTurnWheelbases at the nose gear
}

// groundTurnWheelbases: no turn on the way at groundTurnKts or more is
// tighter than this many wheelbases at the nose gear (full nose wheel lock
// is roughly 1.05-1.1, approximate). Slower, near full lock is how a
// line-up from a stop or a turn pad goes.
const (
	groundTurnWheelbases = 1.0
	groundTurnKts        = 8.0
)

// tightAbove is the tightest nose gear radius (real gear geometry noseM,
// mainM ahead of and behind the reference point) at kts or more, and the
// seconds spent turning tighter than wide at that speed.
func tightAbove(s []turnSample, noseM, mainM, kts, wide float64) (tightest, secs float64) {
	tightest = math.Inf(1)
	const w = 6
	for i := w; i < len(s); i++ {
		a, b := s[i-w], s[i]
		if b.kts < kts || a.kts < kts {
			continue
		}
		om := math.Abs(headingDiff(a.hdg, b.hdg)) * math.Pi / 180 / (float64(w) / 60)
		if om < 0.005 {
			continue
		}
		na := offsetHeading(a.pos, a.hdg, noseM)
		nb := offsetHeading(b.pos, b.hdg, noseM)
		r := localDist(na, nb) / (float64(w) / 60) / om
		tightest = math.Min(tightest, r)
		if r < wide {
			secs += 1.0 / 60
		}
	}
	return tightest, secs
}

var groundCases = []groundCase{
	{"LKPR", [2]string{"24", "06"}, "B77W", []string{"E6", "B14"}, true},
	{"LKPR", [2]string{"24", "06"}, "B789", []string{"E7"}, true},
	{"LKPR", [2]string{"24", "06"}, "A333", []string{"B14"}, true},
	{"LKPR", [2]string{"24", "06"}, "B738", []string{"A1", "C22"}, false},
	{"LKPR", [2]string{"24", "06"}, "A320", []string{"C22"}, false},
	{"EDDF", [2]string{"25C", "07C"}, "B77W", []string{"A62"}, true},
	{"EDDF", [2]string{"25C", "07C"}, "A359", []string{"A58"}, true},
	{"EDDF", [2]string{"25C", "07C"}, "B789", []string{"A54"}, true},
	{"EDDF", [2]string{"25C", "07C"}, "A321", []string{"A52"}, false},
}

// TestGroundTurnsNoDrift: on every ground move (runway exit, taxi-in, onto
// the stand; off the stand after the push, taxi-out, runway entry) the
// aircraft rolls on its own main gear, never sliding it sideways, its
// widebody turns no tighter than groundTurnWheelbases at the nose gear,
// without a jump or a heading step, at LKPR and EDDF, both runway
// directions. Before: on the taxi-out the movers pivoted on the profile's
// wheelbase, not the model's main gear, and the main gear slid up to 23°
// sideways (B789 EDDF) through the turns (live LKPR 2026-11-07, the user:
// big planes look to drift in turns).
func TestGroundTurnsNoDrift(t *testing.T) {
	graphs := map[string]*airport.Graph{}
	for _, c := range groundCases {
		g := graphs[c.icao]
		if g == nil {
			g = airportGraph(t, c.icao)
			graphs[c.icao] = g
		}
		geo, ok := gearFor(c.model)
		if !ok {
			t.Fatalf("%s: no gear geometry", c.model)
		}
		wb := moverGear(ProfileFor(c.model).Motion, c.model).WheelbaseMeters
		for _, label := range c.stands {
			st, err := g.Layout.ParkingIndex(label)
			if err != nil {
				t.Fatalf("%s: no stand %s", c.icao, label)
			}
			for _, end := range c.ends {
				for _, way := range []string{"in", "out"} {
					var r groundRun
					if way == "in" {
						r = runTaxiIn(t, g, end, st, c.model)
					} else {
						r = runTaxiOut(t, g, end, st, c.model)
					}
					name := c.model + " " + c.icao + " " + end + " " + label + " " + way
					if !r.ok || len(r.s) == 0 {
						t.Errorf("%s: did not finish", name)
						continue
					}
					m := measureTurn(r.s, geo.NoseM, geo.MainM, false)
					t.Logf("%s: main gear slip %.2f m/s (%.1f°), tightest %.1f m at the main gear, %.1f at the nose gear (%.2f wheelbases), yaw up to %.1f°/s",
						name, m.maxSlip, m.maxSlipDeg, m.minRMain, m.minRNose, m.minRNose/wb, m.maxYawDegS)
					if m.maxSlipDeg > 1.5 {
						t.Errorf("%s: the main gear slid %.1f° sideways", name, m.maxSlipDeg)
					}
					fast, secs := tightAbove(r.s, geo.NoseM, geo.MainM, groundTurnKts, 1.3*wb)
					t.Logf("%s: at %.0f kt or more tightest %.1f m at the nose gear (%.2f wheelbases), %.0f s under 1.3 wheelbases", name, groundTurnKts, fast, fast/wb, secs)
					if c.wantRadius && fast < groundTurnWheelbases*wb {
						t.Errorf("%s: turned on %.1f m at the nose gear at %.0f kt or more, want at least %.1f (%.2f wheelbases)", name, fast, groundTurnKts, groundTurnWheelbases*wb, groundTurnWheelbases)
					}
					for i := 1; i < len(r.s); i++ {
						if d := localDist(r.s[i-1].pos, r.s[i].pos); d > 1.5 {
							t.Errorf("%s: jumped %.2f m at frame %d", name, d, i)
							break
						}
						if d := math.Abs(headingDiff(r.s[i-1].hdg, r.s[i].hdg)); d > 0.5 {
							t.Errorf("%s: heading stepped %.2f° at frame %d", name, d, i)
							break
						}
					}
				}
			}
		}
	}
}
