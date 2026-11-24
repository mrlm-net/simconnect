package world

import (
	"encoding/json"
	"math"
	"os"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// testLayout is a test airport's layout.
func testLayout(t *testing.T, icao string) *airport.Layout {
	t.Helper()
	b, err := os.ReadFile("../../airport/testdata/" + icao + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var raw airport.RawAirport
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	l, err := airport.BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// segDist is how far p is from the segment ab, in metres.
func segDist(p, a, b airport.LatLon) float64 {
	kx := 111320 * math.Cos(p.Lat*math.Pi/180)
	ax, ay := (a.Lon-p.Lon)*kx, (a.Lat-p.Lat)*111320
	bx, by := (b.Lon-p.Lon)*kx, (b.Lat-p.Lat)*111320
	dx, dy := bx-ax, by-ay
	f := 0.0
	if l2 := dx*dx + dy*dy; l2 > 0 {
		f = math.Max(0, math.Min(1, -(ax*dx+ay*dy)/l2))
	}
	return math.Hypot(ax+dx*f, ay+dy*f)
}

// rescueLanding flies an aircraft with a fire in to the arrival runway at
// icao and down (or, goAround, touching down and climbing away again),
// driving the crew every frame. It checks that no vehicle is ever on the
// runway while the aircraft is airborne close in or rolling above taxi
// speed, that none jumps or stands on another, and returns the crew and
// the vehicles' first points on the runway. Since #1622 also that each
// reaches its own standby point before the touchdown, never overshooting
// it nor reversing, and that its way ahead (Track) never runs empty
// before its end.
func rescueLanding(t *testing.T, icao string, goAround bool) {
	rescueLandingOn(t, icao, testLayout(t, icao), goAround)
}

// rescueLandingOn is rescueLanding on layout l of icao.
func rescueLandingOn(t *testing.T, icao string, l *airport.Layout, goAround bool) {
	t.Helper()
	cc, c, w := rescueBed(t, l, rescueTestTitles)
	g, _ := cc.graph(icao)
	rwyName := cc.runwayFor(g, true, -1)
	rwy, end, ok := l.RunwayEnd(rwyName)
	if !ok {
		t.Fatalf("%s: no arrival runway", icao)
	}
	w.PriorityFor("CSA1", traffic.EmergencyFire)
	prio := map[string]bool{"CSA1": true}
	// along: metres past the landing threshold, from where the crew is
	// sent (#1622: rescueDispatchNM from the airport, a mile in hand);
	// kts its speed; agl feet.
	along, kts, agl := -(rescueDispatchNM-1)*1852.0, 140.0, 0.0
	aircraft := func(now time.Time) traffic.TrackedAircraft {
		lat, lon := calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, end.Heading, along)
		a := traffic.TrackedAircraft{Airport: icao, Updated: now}
		a.Tail, a.Position, a.Heading, a.GroundKts = "CSA1", airport.LatLon{Lat: lat, Lon: lon}, end.Heading, kts
		a.OnGround, a.AGLFt = agl == 0 && along >= 0, agl
		return a
	}
	turnoffs := map[[2]airport.LatLon]bool{}
	for _, name := range []string{rwy.Primary.Name, rwy.Secondary.Name} {
		xs, _ := g.RunwayExits(name)
		for _, x := range xs {
			for i := 1; i < len(x.Path); i++ {
				turnoffs[[2]airport.LatLon{g.Nodes[x.Path[i-1]].Position, g.Nodes[x.Path[i]].Position}] = true
			}
		}
	}
	onRunway := func(p airport.LatLon) bool {
		r, on := l.RunwayAt(p, 0)
		return on && r.Index == rwy.Index
	}
	now := time.Now()
	cc.rescueStep(now, prio, []traffic.TrackedAircraft{aircraft(now)})
	crew := cc.rescue.by["CSA1"]
	if crew == nil || len(crew.trucks) == 0 {
		t.Fatalf("%s: no crew sent", icao)
	}
	spots := traffic.RescueStandby(l, rwyName, len(crew.trucks))
	handled := 0
	const dt = 0.1
	prev := map[int]airport.LatLon{}
	speed := map[int]float64{}
	entered := map[int]bool{}
	touched, lifted, slowAt, withSeen := false, false, time.Time{}, false
	var downAt time.Time
	own, minD := map[int]airport.LatLon{}, map[int]float64{}
	checkedDown := false
	for step := 0; step < 30*60*10; step++ {
		for ; handled < len(c.req); handled++ {
			cc.rescueHandle(assignedObject(c.req[handled], 800+c.req[handled]))
		}
		now = now.Add(100 * time.Millisecond)
		// The aircraft: the approach at 140 kt on a 3° path, touchdown
		// 400 m in at 130 kt, then braking at 1.5 m/s² to a stop; or
		// climbing away after 3 s on the runway.
		switch {
		case along < 400:
			agl = math.Max(0, (400-along)*0.0524*3.28)
		case !touched:
			touched, kts, agl, downAt = true, 130, 0, now
		case goAround && now.Sub(downAt) > 3*time.Second:
			lifted = true
			kts = math.Min(180, kts+2*dt)
			agl += 15 * dt
		case !goAround:
			kts = math.Max(0, kts-1.5/0.5144*dt)
		}
		along += kts * 0.5144 * dt
		if touched && !checkedDown {
			checkedDown = true
			for i := range crew.trucks {
				d, seen := minD[i]
				if !seen || d > 15 {
					t.Errorf("%s: vehicle %d never within 15 m of its standby point before the touchdown (nearest %.0f m, seen %v)", icao, i, d, seen)
				}
			}
		}
		if step%10 == 0 {
			cc.rescueStep(now, prio, []traffic.TrackedAircraft{aircraft(now)})
		}
		cc.rescueFrame(now)
		fast := kts > traffic.TaxiSpeedKts && (touched || along > -2*1852)
		if !goAround && touched && kts == 0 && slowAt.IsZero() {
			slowAt = now
		}
		var pts []airport.LatLon
		for i, tr := range crew.trucks {
			p, route, ok := tr.Track()
			if !ok {
				continue
			}
			if route != nil && len(route) < 2 {
				t.Fatalf("%s: vehicle %d: way ahead empty %.1f m from its end (state %d)", icao, i, calc.HaversineMeters(p.Position.Lat, p.Position.Lon, own[i].Lat, own[i].Lon), crew.state)
			}
			if _, seen := own[i]; !seen && len(route) >= 2 {
				own[i], minD[i] = route[len(route)-1], math.Inf(1)
				if turn := endTurn(route, 30); turn > 135 {
					t.Fatalf("%s: vehicle %d: way to standby turns %.0f° in its last 30 m: past its point and back", icao, i, turn)
				}
			} else if !seen && route == nil {
				// No way from a depot: it appears at its standby point.
				own[i], minD[i] = p.Position, math.Inf(1)
			}
			if o, seen := own[i]; seen && (crew.state == rescueStandby || crew.state == rescueDown) {
				d := calc.HaversineMeters(p.Position.Lat, p.Position.Lon, o.Lat, o.Lon)
				if minD[i] < 15 && d > minD[i]+2 {
					t.Fatalf("%s: vehicle %d overshot its standby point: %.1f m from it after %.1f m", icao, i, d, minD[i])
				}
				minD[i] = math.Min(minD[i], d)
				if q, moved := prev[i]; moved && p.GroundSpeedKts > 0.5 {
					dir := calc.BearingDegrees(q.Lat, q.Lon, p.Position.Lat, p.Position.Lon)
					if calc.HaversineMeters(q.Lat, q.Lon, p.Position.Lat, p.Position.Lon) > 0.05 && math.Abs(math.Remainder(dir-p.Heading, 360)) > 90 {
						t.Fatalf("%s: vehicle %d reversing to standby (heading %.0f, moving %.0f)", icao, i, p.Heading, dir)
					}
				}
			}
			if q, seen := prev[i]; seen && calc.HaversineMeters(p.Position.Lat, p.Position.Lon, q.Lat, q.Lon) > traffic.RescueFollowKts*0.5144*dt+0.5 {
				t.Fatalf("%s: vehicle %d jumped %.1f m", icao, i, calc.HaversineMeters(p.Position.Lat, p.Position.Lon, q.Lat, q.Lon))
			}
			if v, seen := speed[i]; seen && math.Abs(p.GroundSpeedKts-v)*0.5144 > 0.5 {
				t.Fatalf("%s: vehicle %d went from %.1f to %.1f kt in a step (state %d, #1643)", icao, i, v, p.GroundSpeedKts, crew.state)
			}
			speed[i] = p.GroundSpeedKts
			prev[i] = p.Position
			for _, o := range pts {
				if d := calc.HaversineMeters(p.Position.Lat, p.Position.Lon, o.Lat, o.Lon); d < 10 {
					t.Fatalf("%s: vehicle %d %.1f m from another", icao, i, d)
				}
			}
			pts = append(pts, p.Position)
			if onRunway(p.Position) {
				if fast {
					t.Fatalf("%s: vehicle %d on runway %s with the aircraft at %.0f kt, %.0f m along", icao, i, rwyName, kts, along)
				}
				if !entered[i] {
					entered[i] = true
					via := false
					for s := range turnoffs {
						via = via || segDist(p.Position, s[0], s[1]) < 15
					}
					if !via {
						t.Errorf("%s: vehicle %d came onto the runway not by an exit or entry", icao, i)
					}
				}
			}
		}
		if goAround && lifted && agl > 1500 {
			break
		}
		if !slowAt.IsZero() && now.Sub(slowAt) > 6*time.Minute {
			break
		}
		if _, with := cc.rescueStatus("CSA1"); with {
			withSeen = true
		}
	}
	if goAround {
		if crew.state != rescueStandby {
			t.Errorf("%s: after the go-around, state %d, want standby", icao, crew.state)
		}
		for i, tr := range crew.trucks {
			p, _, ok := tr.Track()
			if !ok {
				t.Errorf("%s: vehicle %d not out", icao, i)
				continue
			}
			best := math.Inf(1)
			for _, s := range spots {
				best = math.Min(best, calc.HaversineMeters(p.Position.Lat, p.Position.Lon, s.Position.Lat, s.Position.Lon))
			}
			if best > 10 {
				t.Errorf("%s: vehicle %d %.0f m off any standby point after the go-around", icao, i, best)
			}
		}
		return
	}
	if !withSeen {
		t.Errorf("%s: never with the aircraft after it stopped (state %d)", icao, crew.state)
	}
	if len(entered) == 0 {
		t.Errorf("%s: no vehicle came onto the runway to it", icao)
	}
}

// TestRescueWaitsThenEnters (#1597): the fire trucks wait at standby while
// the aircraft lands and rolls out, never on the runway while it rolls
// faster than taxi speed; once it is slow they come onto the runway by an
// exit or entry and drive to it; never jumping nor standing on one
// another. LKPR and EDDF.
func TestRescueWaitsThenEnters(t *testing.T) {
	for _, icao := range []string{"LKPR", "EDDF"} {
		t.Run(icao, func(t *testing.T) { rescueLanding(t, icao, false) })
	}
}

// TestRescueGoAround (#1597): an aircraft touching down and climbing away
// again leaves the trucks at standby; they never drive onto the runway.
func TestRescueGoAround(t *testing.T) {
	for _, icao := range []string{"LKPR", "EDDF"} {
		t.Run(icao, func(t *testing.T) { rescueLanding(t, icao, true) })
	}
}

// TestRescueStandbyOnPavement (#1597): every standby point (2 to 5
// vehicles, every runway end) lies on a taxiway, more than
// RescueStripHalfMeters (75 m for these runways) from the centreline, and
// no two closer than RescueSpacingMeters. LKPR and EDDF.
func TestRescueStandbyOnPavement(t *testing.T) {
	for _, icao := range []string{"LKPR", "EDDF"} {
		l := testLayout(t, icao)
		onPavement := func(p airport.LatLon) bool {
			for _, tp := range l.TaxiPaths {
				if tp.Type == types.SIMCONNECT_FACILITY_TAXI_PATH_TYPE_RUNWAY || tp.EndsAtParking() {
					continue
				}
				a, b, ok := l.PathEndpoints(tp)
				if ok && segDist(p, a, b) <= tp.Width/2+1 {
					return true
				}
			}
			return false
		}
		for _, r := range l.Runways {
			strip := traffic.RescueStripHalfMeters(r.Width)
			for _, name := range []string{r.Primary.Name, r.Secondary.Name} {
				_, end, _ := l.RunwayEnd(name)
				for n := 2; n <= 5; n++ {
					spots := traffic.RescueStandby(l, name, n)
					if len(spots) != n {
						t.Fatalf("%s %s: %d standby points, want %d", icao, name, len(spots), n)
					}
					for i, s := range spots {
						along, cross := axis(end, s.Position)
						if math.Abs(cross) <= strip || along < 0 || along > r.Length {
							t.Errorf("%s %s n=%d: point %d %.0f m along, %.0f m across: in the strip (half-width %.0f m, runway %.0f m wide)", icao, name, n, i, along, cross, strip, r.Width)
						}
						if !onPavement(s.Position) {
							t.Errorf("%s %s n=%d: point %d (%.0f m along, %.0f m across) not on a taxiway", icao, name, n, i, along, cross)
						}
						for _, o := range spots[:i] {
							if d := calc.HaversineMeters(s.Position.Lat, s.Position.Lon, o.Position.Lat, o.Position.Lon); d < traffic.RescueSpacingMeters-0.5 {
								t.Errorf("%s %s n=%d: points %.1f m apart", icao, name, n, d)
							}
						}
					}
				}
			}
		}
	}
}

// TestRescueStandbyGiven (#1597): standby positions a layout gives are
// taken for their runway end, in place of the derived ones.
func TestRescueStandbyGiven(t *testing.T) {
	l := testLayout(t, "LKPR")
	_, end, _ := l.RunwayEnd("24")
	var given []airport.RescueStandbyPoint
	for _, d := range []float64{1200, 1900} {
		lat, lon := calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, end.Heading, d)
		lat, lon = calc.DisplaceByHeading(lat, lon, end.Heading+90, 120)
		given = append(given, airport.RescueStandbyPoint{Runway: "24", Position: airport.LatLon{Lat: lat, Lon: lon}, Heading: end.Heading - 90})
	}
	l.RescueStandby = append(given, airport.RescueStandbyPoint{Runway: "06", Position: given[0].Position})
	spots := traffic.RescueStandby(l, "24", 2)
	if len(spots) != 2 {
		t.Fatalf("%d points", len(spots))
	}
	for _, s := range spots {
		if s.Position != given[0].Position && s.Position != given[1].Position {
			t.Errorf("point %v not one given", s.Position)
		}
	}
	if spots[0].Position != given[1].Position {
		t.Errorf("first %v, want the one nearest the expected stop %v", spots[0].Position, given[1].Position)
	}
	got := traffic.RescueStandby(l, "06", 3)
	has := false
	for _, s := range got {
		has = has || s.Position == given[0].Position
	}
	if len(got) != 3 || !has {
		t.Errorf("06: %v, want the given one and two derived", got)
	}
}

// endTurn is how far (degrees) the way's direction in its last within
// metres strays from its final one: near 180 for a way going past its end
// and back.
func endTurn(way []airport.LatLon, within float64) float64 {
	final, has := 0.0, false
	worst, walked := 0.0, 0.0
	for i := len(way) - 1; i > 0 && walked < within; i-- {
		a, b := way[i-1], way[i]
		d := calc.HaversineMeters(a.Lat, a.Lon, b.Lat, b.Lon)
		if d < 0.2 {
			continue
		}
		brg := calc.BearingDegrees(a.Lat, a.Lon, b.Lat, b.Lon)
		if !has {
			final, has = brg, true
		}
		worst = math.Max(worst, math.Abs(math.Remainder(brg-final, 360)))
		walked += d
	}
	return worst
}

// axis is p along the runway end's direction from its threshold and
// across it, in metres.
func axis(end airport.RunwayEnd, p airport.LatLon) (along, cross float64) {
	d := calc.HaversineMeters(end.Threshold.Lat, end.Threshold.Lon, p.Lat, p.Lon)
	a := (calc.BearingDegrees(end.Threshold.Lat, end.Threshold.Lon, p.Lat, p.Lon) - end.Heading) * math.Pi / 180
	return d * math.Cos(a), d * math.Sin(a)
}
