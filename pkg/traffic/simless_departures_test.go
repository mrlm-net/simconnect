package traffic

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// TestSimlessDeparturesFlyTheirSIDs: every LKPR SID off each runway end,
// an injected take-off from C22, then flown by SimlessAI: it passes every
// SID fix (a fly-by corner within its turn's cut), never descends, jumps
// or orbits. Before, a departure handed back after its injected climb
// was never flown (the unfreeze ignored), and a SID turning back (24
// ARTU 6M: climb to 1700 ft, direct PR407) lost all its fixes.
func TestSimlessDeparturesFlyTheirSIDs(t *testing.T) {
	g := lkprGraph(t)
	procs := lkprProcedures(t)
	c22, _ := g.Layout.ParkingIndex("C22")
	for _, rwy := range []string{"24", "06", "30", "12"} {
		r, end, _ := g.Layout.RunwayEnd(rwy)
		far := r.Primary.Threshold
		if end.Name == r.Primary.Name {
			far = r.Secondary.Threshold
		}
		for _, d := range procs.Departures {
			ok := false
			for _, rr := range d.Runways() {
				if rr == rwy {
					ok = true
				}
			}
			if !ok {
				continue
			}
			sid, err := procs.ResolveSID(d.Name, rwy, "", far, g.Layout.Altitude)
			if err != nil || len(sid) == 0 {
				t.Logf("%s %s: resolve: %v", rwy, d.Name, err)
				continue
			}
			name := rwy + " " + d.Name
			ec := &eventClient{}
			ai := NewSimlessAI(ec)
			inj := NewInjector(ai)
			ctl := NewTaxiController(NewFleet(ai), TaxiWithInjector(inj))
			if err := ctl.Start(TaxiRequest{Graph: g, Parking: c22, Runway: rwy, Model: "FSLTL A320 Air France SL", Tail: "CSA8",
				RollingTakeoffChance: -1, Departure: sid}); err != nil {
				t.Errorf("%s: start: %v", name, err)
				continue
			}
			go func() {
				for range ctl.Events() {
				}
			}()
			now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
			ctl.now = func() time.Time { return now }
			ctl.Handle(assignedMsg(DefaultTaxiRequestBase+reqOffSpawn, 77))
			inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
			mon := DefaultTaxiRequestBase + reqOffMonitor
			stand := g.Layout.Parking[c22]
			for i := 0; i < 60*3600 && !ctl.State().Terminal(); i++ {
				now = now.Add(time.Second / 60)
				ctl.Handle(positionMsg(mon, 77, stand.Position, 0, 0, true))
			}
			if ctl.State() != TaxiComplete {
				t.Errorf("%s: ended %v", name, ctl.State())
				continue
			}
			if !ai.Flying(77) {
				t.Errorf("%s: not flown by SimlessAI after the hand-over", name)
				continue
			}
			ai.mu.Lock()
			wps := ai.objects[77].wps
			ai.mu.Unlock()
			// Fly 30 minutes at 60 Hz traffic time.
			passed := make([]float64, len(sid))
			for i := range passed {
				passed[i] = math.Inf(1)
			}
			var prev airport.LatLon
			prevAlt, minAlt, maxJump, turned, lastHdg := 0.0, 0.0, 0.0, 0.0, -1.0
			descents := 0
			for i := 0; i < 60*1800 && ai.Flying(77); i++ {
				now = now.Add(time.Second / 60)
				ai.Step(now)
				ai.mu.Lock()
				o := ai.objects[77]
				pos, alt, hdg, kts := o.pos, o.altFt, o.hdg, o.kts
				ai.mu.Unlock()
				if i > 0 {
					step := calc.HaversineMeters(prev.Lat, prev.Lon, pos.Lat, pos.Lon)
					maxJump = math.Max(maxJump, step-kts*1852/3600/60)
					if alt < prevAlt-0.5 {
						descents++
						minAlt = math.Min(minAlt, alt-prevAlt)
					}
				}
				if lastHdg >= 0 {
					turned += math.Abs(headingDiff(lastHdg, hdg))
				}
				prev, prevAlt, lastHdg = pos, alt, hdg
				for k, n := range sid {
					if n.Position.Lat == 0 && n.Position.Lon == 0 {
						continue
					}
					passed[k] = math.Min(passed[k], calc.HaversineMeters(pos.Lat, pos.Lon, n.Position.Lat, n.Position.Lon))
				}
			}
			missed := ""
			for k, n := range sid {
				if !math.IsInf(passed[k], 1) && passed[k] > flyByAllowM(sid, k, far) {
					missed += fmt.Sprintf("%s (%.0f m, allowed %.0f) ", n.Ident, passed[k], flyByAllowM(sid, k, far))
				}
			}
			ai.mu.Lock()
			o := ai.objects[77]
			ai.mu.Unlock()
			t.Logf("%-12s %2d wps, %2d sid pts, end %5.0f ft %3.0f kt, turned %4.0f°, max extra step %.2f m, descents %d, missed [%s]",
				name, len(wps), len(sid), o.altFt, o.kts, turned, maxJump, descents, missed)
			if missed != "" {
				t.Errorf("%s: missed SID fixes: %s", name, missed)
			}
			if maxJump > 1 {
				t.Errorf("%s: jumped %.1f m beyond its speed", name, maxJump)
			}
			if descents > 0 {
				t.Errorf("%s: descended %d frames (worst %.1f ft/frame)", name, descents, minAlt)
			}
			if turned > 1080 {
				t.Errorf("%s: turned %.0f° in total: orbiting", name, turned)
			}
		}
	}
}

// flyByAllowM is how far from SID fix k a departure may pass: 2.5 km, and
// at a fly-by corner what its turn cuts off at 250 kt and 25° of bank, a
// kilometre to spare (06 VOZ 4E at PR633: about 110°, 2.7 km inside).
func flyByAllowM(sid []airport.NavPoint, k int, from airport.LatLon) float64 {
	allow := 2500.0
	if k+1 >= len(sid) || sid[k].FlyOver {
		return allow
	}
	// The legs in and out: from the last charted point before (from, the
	// runway end, for the first), to the next.
	a := from
	for i := k - 1; i >= 0; i-- {
		if p := sid[i].Position; p != (airport.LatLon{}) && calc.HaversineMeters(p.Lat, p.Lon, sid[k].Position.Lat, sid[k].Position.Lon) > 500 {
			a = p
			break
		}
	}
	b, c := sid[k].Position, sid[k+1].Position
	if a == (airport.LatLon{}) || c == (airport.LatLon{}) {
		return allow
	}
	turn := math.Abs(headingDiff(calc.BearingDegrees(a.Lat, a.Lon, b.Lat, b.Lon), calc.BearingDegrees(b.Lat, b.Lon, c.Lat, c.Lon)))
	if turn >= 170 {
		return allow
	}
	v := 250 * 1852.0 / 3600
	r := v * v / (9.81 * math.Tan(25*math.Pi/180))
	return math.Max(allow, r*(1/math.Cos(turn/2*math.Pi/180)-1)+1000)
}

// TestSimlessEnrouteFliesItsPlan: an en-route plan from EnrouteStart,
// spawned at its first point and released to the AI, is flown down to its
// last altitude and on along the last track.
func TestSimlessEnrouteFliesItsPlan(t *testing.T) {
	a := airport.LatLon{Lat: 49.0, Lon: 11.0}
	b := airport.LatLon{Lat: 49.5, Lon: 12.5}
	c := airport.LatLon{Lat: 50.0, Lon: 13.8}
	spawn, wps, err := EnrouteStart([]RoutePoint{{a, 36000, 450}, {b, 30000, 420}, {c, 12000, 280}})
	if err != nil {
		t.Fatal(err)
	}
	ec := &eventClient{}
	ai := NewSimlessAI(ec)
	ai.AddToDataDefinition(1, "Initial Position", "", 0, 0, 0)
	_ = spawn
	ai.mu.Lock()
	o := ai.object(9)
	o.wps = wps
	o.posed, o.pos, o.altFt, o.kts = true, a, spawn.Altitude, float64(spawn.Airspeed)
	o.hdg = spawn.Heading
	o.released = true
	ai.mu.Unlock()
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	best := math.Inf(1)
	for i := 0; i < 60*3600; i++ {
		now = now.Add(time.Second / 60)
		ai.Step(now)
		ai.mu.Lock()
		p := o.pos
		ai.mu.Unlock()
		best = math.Min(best, calc.HaversineMeters(p.Lat, p.Lon, c.Lat, c.Lon))
	}
	t.Logf("closest to the last point %.0f m, end %.0f ft %.0f kt", best, o.altFt, o.kts)
	if best > 2500 {
		t.Errorf("missed the last point by %.0f m", best)
	}
	if math.Abs(o.altFt-12000) > 100 {
		t.Errorf("ended at %.0f ft, want 12000", o.altFt)
	}
}
