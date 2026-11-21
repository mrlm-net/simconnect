package traffic

import (
	"math"
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// groundFrame is one frame of an injected departure as reported.
type groundFrame struct {
	state TaxiState
	pos   airport.LatLon
	hdg   float64
	kts   float64
}

// overflowQueue is the World's hold-short queue with nobody queued yet:
// the end entrances other than the first are overflow (holdShortQueue).
func overflowQueue(g *airport.Graph, end string) map[airport.NodeID]float64 {
	q := map[airport.NodeID]float64{}
	if es, _ := g.EndEntrances(end); len(es) > 1 {
		for _, e := range es[1:] {
			for _, h := range []airport.NodeID{e.HoldShort, e.ILSHoldShort} {
				if h >= 0 {
					q[h] += 1e6
				}
			}
		}
	}
	return q
}

// runDeparture flies an injected departure of model from stand to end at
// 20 frames a second until its take-off roll, and returns its frames and
// route.
func runDeparture(t *testing.T, g *airport.Graph, stand, end, model string, opts airport.RouteOptions) ([]groundFrame, *airport.Route) {
	t.Helper()
	si, err := g.Layout.ParkingIndex(stand)
	if err != nil {
		t.Fatal(err)
	}
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(inj))
	if err := ctl.Start(TaxiRequest{Graph: g, Parking: si, Runway: end, Model: model, Tail: "TEST1", Options: opts, RollingTakeoffChance: -1}); err != nil {
		t.Fatalf("%s %s: %v", stand, end, err)
	}
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultTaxiRequestBase+reqOffSpawn, 77))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	go func() {
		for range ctl.Events() {
		}
	}()
	var out []groundFrame
	p := g.Layout.Parking[si].Position
	for f := 0; f < 20*3600 && ctl.State() != TaxiDeparting && !ctl.State().Terminal(); f++ {
		now = now.Add(time.Second / 20)
		ctl.Handle(positionMsg(DefaultTaxiRequestBase+reqOffMonitor, 77, p, 0, 0, true))
		l := ctl.last
		out = append(out, groundFrame{ctl.State(), l.Position, l.Heading, l.GroundSpeed})
	}
	if ctl.State() != TaxiDeparting {
		t.Errorf("%s %s %s: ended %v, want the take-off roll", g.Layout.ICAO, stand, end, ctl.State())
	}
	return out, ctl.Route()
}

// forwardFaults checks frames moving under their own power (taxi and
// line-up): none moves against its heading, the speed given matches the
// distance the reference point moved, and over every 0.3 m moved the
// track of the main gear (back metres behind the reference point) is
// within maxSlip of the heading. It returns the worst slip and a
// description of the first fault, "" if none.
func forwardFaults(frames []groundFrame, maxSlip, back float64) (float64, string) {
	const dt = 1.0 / 20
	worst := 0.0
	own := func(s TaxiState) bool {
		return s == TaxiTaxiing || s == TaxiHoldingShort || s == TaxiLiningUp || s == TaxiLinedUp
	}
	fault := ""
	for i := 1; i < len(frames); i++ {
		a, b := frames[i-1], frames[i]
		if !own(a.state) || !own(b.state) {
			continue
		}
		d := localDist(a.pos, b.pos)
		if d < 1e-3 {
			continue
		}
		if math.Abs(headingDiff(b.hdg, localBearing(a.pos, b.pos))) > 90 && fault == "" {
			fault = "moved backwards " + b.state.String()
		}
		if v := d / dt / ktsToMS; math.Abs(v-b.kts) > math.Max(0.5, 0.15*v) && fault == "" {
			fault = "speed given off the distance moved " + b.state.String()
		}
	}
	for i := 0; i < len(frames); i++ {
		if !own(frames[i].state) {
			continue
		}
		for j := i + 1; j < len(frames) && own(frames[j].state); j++ {
			gi, gj := offsetHeading(frames[i].pos, frames[i].hdg+180, back), offsetHeading(frames[j].pos, frames[j].hdg+180, back)
			if localDist(gi, gj) < 0.3 {
				continue
			}
			trk := localBearing(gi, gj)
			mid := frames[(i+j)/2].hdg
			slip := math.Abs(headingDiff(mid, trk))
			if math.Abs(headingDiff(frames[i].hdg, frames[j].hdg)) < 20 {
				worst = math.Max(worst, slip)
				if slip > maxSlip && fault == "" {
					fault = "track off the heading " + frames[j].state.String()
				}
			}
			break
		}
	}
	return worst, fault
}

// TestLineUpNeverBacksUp (#1627): with the World's queue sending
// departures to the first of a runway's end entrances (EGLL 27L: N1B,
// north of it), a departure from the other side holds short on its own
// side and lines up forward: no route over its departure runway to a hold
// beyond it, no frame against its heading. Live, EZY1230 (A20N) crossed
// 27L to N1B and backed 94 m onto it at gs 0.
func TestLineUpNeverBacksUp(t *testing.T) {
	g := airportGraph(t, "EGLL")
	for _, c := range []struct{ stand, end, model string }{
		{"457", "09R", "FSLTL A20N easyJet"},
		{"429", "09R", "FSLTL A320 British Airways"},
		{"457", "27L", "FSLTL A20N easyJet"},
		{"429", "27L", "FSLTL B738 Ryanair"},
	} {
		frames, r := runDeparture(t, g, c.stand, c.end, c.model, airport.RouteOptions{HoldShortQueue: overflowQueue(g, c.end)})
		rw, _, _ := g.Layout.RunwayEnd(c.end)
		if slices.Contains(r.RunwayCrossings, rw.Name()) {
			t.Errorf("%s → %s: route via %v crosses %s to its holding point", c.stand, c.end, r.Taxiways, rw.Name())
		}
		worst, fault := forwardFaults(frames, 180, 0)
		t.Logf("%s → %s %s via %v: worst slip %.1f°", c.stand, c.end, c.model, r.Taxiways, worst)
		if fault != "" {
			t.Errorf("%s → %s %s: %s", c.stand, c.end, c.model, fault)
		}
	}
}

// TestGroundTurnTrackFollowsHeading (#1628): taxiing and lining up, the
// speed given is the speed the reference point moves at, and the main
// gear's track follows the heading through every turn (within 4°): the
// reference point ahead of it swings out in a tight turn as on a real
// aircraft, the main gear does not slide (TestGroundTurnsNoDrift). For
// several types at several airports and runway ends. Live, DAL915 at KJFK
// moved 0.8 m/s at a given 5 kt in its line-up turn.
func TestGroundTurnTrackFollowsHeading(t *testing.T) {
	for _, c := range []struct{ icao, stand, end, model string }{
		{"KJFK", "", "04L", "FSLTL B739 Delta"},
		{"KJFK", "", "31L", "FSLTL A321 Delta"},
		{"KJFK", "", "22R", "FSLTL B772 Delta"},
		{"EGLL", "457", "27L", "FSLTL A20N easyJet"},
		{"EGLL", "429", "09R", "FSLTL A320 British Airways"},
		{"LKPR", "C22", "24", "FSLTL B738 Ryanair"},
		{"LKPR", "C22", "06", "FSLTL A20N easyJet"},
	} {
		g := airportGraph(t, c.icao)
		stand := c.stand
		if stand == "" {
			s := g.Layout.SuitableStands(18)
			stand = g.Layout.Parking[s[len(s)/3]].Label()
		}
		frames, r := runDeparture(t, g, stand, c.end, c.model, airport.RouteOptions{})
		back := 1.0
		if gg, ok := gearFor(c.model); ok {
			back = gg.MainM
		}
		worst, fault := forwardFaults(frames, 4, back)
		t.Logf("%s %s → %s %s via %v: worst slip %.1f°", c.icao, stand, c.end, c.model, r.Taxiways, worst)
		if fault != "" {
			t.Errorf("%s %s → %s %s: %s (worst %.1f°)", c.icao, stand, c.end, c.model, fault, worst)
		}
	}
}
