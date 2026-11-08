package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// turnSample is one frame of an injected taxi-in.
type turnSample struct {
	pos       airport.LatLon
	hdg, kts  float64
	remaining float64 // nose metres to the path end
	join      float64 // stand join, metres before the end
}

// turnStats measures samples (60 Hz) for a gear geometry: the main gear's
// sideways slip and the radii at the main and nose gear, over the stand
// part (remaining <= join+20).
type turnStats struct {
	maxSlip, maxSlipDeg, minRMain, minRNose, maxYawDegS float64
}

func measureTurn(s []turnSample, noseM, mainM float64, standOnly bool) turnStats {
	st := turnStats{minRMain: math.Inf(1), minRNose: math.Inf(1)}
	const w = 6 // frames per difference (0.1 s)
	if len(s) <= w {
		return st
	}
	o := s[0].pos
	kx := metersPerDegree * math.Cos(o.Lat*math.Pi/180)
	xy := func(p airport.LatLon) (float64, float64) {
		return (p.Lon - o.Lon) * kx, (p.Lat - o.Lat) * metersPerDegree
	}
	gear := func(t turnSample, d float64) (float64, float64) {
		x, y := xy(t.pos)
		h := t.hdg * math.Pi / 180
		return x + math.Sin(h)*d, y + math.Cos(h)*d
	}
	dt := float64(w) / 60
	for i := w; i < len(s); i++ {
		a, b := s[i-w], s[i]
		if standOnly && b.remaining > b.join+20 {
			continue
		}
		hm := (a.hdg + headingDiff(a.hdg, b.hdg)/2) * math.Pi / 180
		ux, uy := math.Sin(hm), math.Cos(hm)
		rx, ry := math.Cos(hm), -math.Sin(hm)
		ax, ay := gear(a, -mainM)
		bx, by := gear(b, -mainM)
		vx, vy := (bx-ax)/dt, (by-ay)/dt
		along, lat := vx*ux+vy*uy, vx*rx+vy*ry
		nax, nay := gear(a, noseM)
		nbx, nby := gear(b, noseM)
		vn := math.Hypot(nbx-nax, nby-nay) / dt
		om := math.Abs(headingDiff(a.hdg, b.hdg)) * math.Pi / 180 / dt
		if math.Hypot(along, lat) < 0.3 {
			continue
		}
		st.maxSlip = math.Max(st.maxSlip, math.Abs(lat))
		st.maxSlipDeg = math.Max(st.maxSlipDeg, math.Abs(math.Atan2(lat, math.Abs(along)))*180/math.Pi)
		st.maxYawDegS = math.Max(st.maxYawDegS, om*180/math.Pi)
		if om > 0.005 {
			st.minRMain = math.Min(st.minRMain, math.Hypot(along, lat)/om)
			st.minRNose = math.Min(st.minRNose, vn/om)
		}
	}
	return st
}

// runStandArrival flies an injected arrival of model to stand from end and
// returns its taxi frames; ok false when none was planned or it did not park.
func runStandArrival(t *testing.T, g *airport.Graph, end string, stand int, model string) ([]turnSample, *eventClient, bool) {
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj))
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: end, Parking: stand, Model: model, InjectApproach: true,
		RollThroughChance: -1, AfterLandingDwell: time.Second}); err != nil {
		return nil, nil, false
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
	var out []turnSample
	for i := 0; i < 60*3600 && !ctl.State().Terminal(); i++ {
		now = now.Add(time.Second / 60)
		ctl.Handle(arrivalPositionMsg(mon, 77, p.Position, 0, 0, 0, false))
		if st := ctl.State(); (st == ArrivalTaxiing || st == ArrivalParking) && ctl.mover != nil {
			ps := ctl.mover.Pose()
			out = append(out, turnSample{pos: ps.Position, hdg: ps.Heading, kts: ps.GroundSpeedKts,
				remaining: ctl.mover.Path().Length() - ps.Distance, join: ctl.standJoin})
		}
	}
	return out, ec, ctl.State() == ArrivalParked
}

// TestStandTurnInNoDrift: an arrival turns off the taxilane onto its stand
// rolling on its own main gear, never sliding it sideways, and on a radius
// fit for its size: at least 1.3 wheelbases at the nose gear (about 50° of
// nose wheel steering; full lock is about wheelbase / sin 65-70°, roughly
// 33 m on a B77W, 30 m A359, 28 m B789, 27 m A333). Before: B77W, A359,
// B789 and A333 turned onto LKPR E6, E7 and B14 on 27-35 m, and the real
// main gear of a B789 slid 11-14° sideways (B738 8°): it looked like drift
// (the user, live LKPR 2026-11-07). Each still parks on its mark, aligned,
// without a jump.
func TestStandTurnInNoDrift(t *testing.T) {
	type run struct {
		icao, end, stand string
		wide             bool // the stand's lead-in leaves room for the wide turn
	}
	cases := []struct {
		model string
		runs  []run
	}{
		{"B77W", []run{{"LKPR", "24", "E6", true}, {"LKPR", "24", "E7", true}, {"LKPR", "24", "B14", true}, {"LKPR", "06", "E6", true}, {"EDDF", "25C", "A62", true}}},
		{"A359", []run{{"LKPR", "24", "E6", true}, {"LKPR", "24", "B14", true}, {"LKPR", "06", "E7", true}, {"EDDF", "07C", "A58", true}}},
		{"B789", []run{{"LKPR", "24", "E6", true}, {"LKPR", "24", "B14", true}, {"LKPR", "06", "E7", true}, {"EDDF", "25C", "A54", true}}},
		{"A333", []run{{"LKPR", "24", "E7", true}, {"LKPR", "24", "B14", true}, {"EDDF", "25C", "A66", true}}},
		{"B738", []run{{"LKPR", "24", "A1", true}, {"LKPR", "06", "A5", false}, {"EDDF", "25C", "A14", false}}},
		{"A321", []run{{"LKPR", "24", "A3", true}, {"EDDF", "25C", "A52", false}}},
		{"A320", []run{{"LKPR", "24", "A1", false}, {"LKPR", "06", "C22", false}}},
	}
	graphs := map[string]*airport.Graph{}
	for _, c := range cases {
		geo, ok := gearFor(c.model)
		if !ok {
			t.Fatalf("%s: no gear geometry", c.model)
		}
		wb := moverGear(ProfileFor(c.model).Motion, c.model).WheelbaseMeters
		for _, r := range c.runs {
			g := graphs[r.icao]
			if g == nil {
				g = airportGraph(t, r.icao)
				graphs[r.icao] = g
			}
			st, err := g.Layout.ParkingIndex(r.stand)
			if err != nil {
				t.Fatalf("%s: no stand %s", r.icao, r.stand)
			}
			name := c.model + " " + r.icao + " " + r.end + " " + r.stand
			s, ec, parked := runStandArrival(t, g, r.end, st, c.model)
			if s == nil || !parked {
				t.Errorf("%s: not parked", name)
				continue
			}
			m := measureTurn(s, geo.NoseM, geo.MainM, true)
			t.Logf("%s: main gear slip %.2f m/s (%.1f°), radius %.1f m at the main gear, %.1f at the nose gear (%.2f wheelbases), yaw up to %.1f°/s",
				name, m.maxSlip, m.maxSlipDeg, m.minRMain, m.minRNose, m.minRNose/wb, m.maxYawDegS)
			if m.maxSlipDeg > 1.5 {
				t.Errorf("%s: the main gear slid %.1f° sideways onto the stand", name, m.maxSlipDeg)
			}
			if r.wide && m.minRNose < 1.3*wb {
				t.Errorf("%s: turned onto the stand on %.1f m at the nose gear, want at least %.1f (1.3 wheelbases)", name, m.minRNose, 1.3*wb)
			}
			all := placements(ec)
			for i := 1; i < len(all); i++ {
				a, b := all[i-1], all[i]
				if d := localDist(airport.LatLon{Lat: a.Latitude, Lon: a.Longitude}, airport.LatLon{Lat: b.Latitude, Lon: b.Longitude}); d > 1.5 {
					t.Errorf("%s: jumped %.2f m at placement %d/%d", name, d, i, len(all))
					break
				}
			}
			for i := 1; i < len(s); i++ {
				if d := math.Abs(headingDiff(s[i-1].hdg, s[i].hdg)); d > 0.5 {
					t.Errorf("%s: heading stepped %.2f° in a frame on the ground", name, d)
					break
				}
			}
			last := all[len(all)-1]
			if hd := math.Abs(headingDiff(last.Heading, g.Layout.Parking[st].Heading)); hd > 3 {
				t.Errorf("%s: parked %.1f° off the stand heading", name, hd)
			}
		}
	}
}
