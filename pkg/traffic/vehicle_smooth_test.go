package traffic

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Smooth service vehicles (#1630): at every stand an airliner uses, the
// fuel truck, the stairs, the bus and the tug drive in and home without
// turning back on themselves (a hairpin within smoothHairpinMeters of
// way), without driving under their own aircraft's fuselage, and without
// passing over another stand's parking point (where an aircraft may
// stand). LKPR (roads at the stands' heads) and LROP (roads behind the
// tails).

// smoothHairpinMeters: a way turning more than smoothHairpinDeg within
// this much of it doubles back.
const (
	smoothHairpinMeters = 12.0
	smoothHairpinDeg    = 150.0
	smoothStandMeters   = 3.0
	smoothSpotMeters    = 8.0
)

// smoothLayout is icao's layout from the test data.
func smoothLayout(t *testing.T, icao string) *airport.Layout {
	t.Helper()
	if icao == "LKPR" {
		return lkprGraph(t).Layout
	}
	b, err := os.ReadFile("../airport/testdata/" + icao + "-layout.json")
	if err != nil {
		t.Fatal(err)
	}
	var l airport.Layout
	if err := json.Unmarshal(b, &l); err != nil {
		t.Fatal(err)
	}
	return &l
}

// smoothStands are the stands of l an A320 uses, with a depot to come from.
func smoothStands(l *airport.Layout, prof MotionProfile) []int {
	var out []int
	for i, p := range l.Parking {
		if p.Type == types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_VEHICLE || p.Size() == airport.StandNone || p.Radius*2 < prof.SpanMeters {
			continue
		}
		out = append(out, i)
	}
	return out
}

// hairpin is the first place along path where it turns more than
// smoothHairpinDeg within smoothHairpinMeters; -1 when none.
func hairpin(path *GroundPath) float64 {
	var heads []float64
	for d := 0.0; d+1 <= path.Length(); d++ {
		heads = append(heads, localBearing(path.PointAt(d), path.PointAt(d+1)))
		k := len(heads) - 1
		for j := max(0, k-int(smoothHairpinMeters)); j < k; j++ {
			if math.Abs(headingDiff(heads[j], heads[k])) > smoothHairpinDeg {
				return d
			}
		}
	}
	return -1
}

// underFuselage is the first place along path under the fuselage of the
// aircraft at pose (nose gear back to the tail, within its half-width),
// but for its last smoothSpotMeters (in: the front axle past a tug's spot
// at the nose gear) or first (home); -1 when none.
func underFuselage(path *GroundPath, pose GroundPose, prof MotionProfile, in bool) float64 {
	f := aircraftFrame{nose: NoseGear(pose.Position, pose.Heading, prof), hdg: pose.Heading, prof: prof}
	from, to := smoothSpotMeters, path.Length()
	if in {
		from, to = 0, path.Length()-smoothSpotMeters
	}
	for d := from; d <= to; d++ {
		x, y := f.to(path.PointAt(d))
		if math.Abs(x) < 2.5 && y < -1 && y > -(prof.WheelbaseMeters+prof.TailMeters) {
			return d
		}
	}
	return -1
}

// overStand is the first other stand whose parking point path passes
// (not one sharing the own stand's space); -1 when none.
func overStand(path *GroundPath, l *airport.Layout, own int) int {
	shared := map[int]bool{own: true}
	for _, k := range l.ParkingConflicts(own) {
		shared[k] = true
	}
	for d := 0.0; d <= path.Length(); d++ {
		p := path.PointAt(d)
		for i, s := range l.Parking {
			if shared[i] || s.Type == types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_VEHICLE || s.Size() == airport.StandNone {
				continue
			}
			if localDist(p, s.Position) < smoothStandMeters {
				return i
			}
		}
	}
	return -1
}

// smoothVehicle drives one kind of vehicle at a stand: in, then home,
// reporting each way it drives (once).
type smoothVehicle struct {
	kind string
	run  func(t *testing.T, l *airport.Layout, pose GroundPose, prof MotionProfile, way func(phase string, p *GroundPath, at GroundPose))
}

var smoothVehicles = []smoothVehicle{
	{"fuel", func(t *testing.T, l *airport.Layout, pose GroundPose, prof MotionProfile, way func(string, *GroundPath, GroundPose)) {
		c := &tugClient{}
		inj := NewInjector(c)
		f := NewSimObjectFuelTruck(c, inj, "FSDT_FuelTruck_7296", 9002, prof)
		f.Layout = l
		if f.Attach(pose) != nil || f.arrive == nil {
			return
		}
		way("in", f.arrive.Path(), pose)
		f.Handle(assignedMsg(9002, 56))
		inj.Handle(groundMsg(DefaultInjectRequestBase+1, 56, 1200, 3))
		for k := 0; k < 9000 && !f.Fuelling(); k++ {
			f.Update(pose, false, 0.1)
		}
		f.Update(pose, true, 0.1)
		if f.away != nil {
			way("home", f.away.Path(), pose)
		}
	}},
	{"stairs", func(t *testing.T, l *airport.Layout, pose GroundPose, prof MotionProfile, way func(string, *GroundPath, GroundPose)) {
		c := &tugClient{}
		inj := NewInjector(c)
		s := NewSimObjectStairs(c, inj, "x", 9003, prof)
		s.Layout = l
		if s.Attach(pose) != nil || s.arrive == nil {
			return
		}
		way("in", s.arrive.Path(), pose)
		s.Handle(assignedMsg(9003, 57))
		inj.Handle(groundMsg(DefaultInjectRequestBase+1, 57, 1200, 3))
		for k := 0; k < 9000 && s.arrive != nil; k++ {
			s.Update(pose, false, 0.1)
		}
		for k := 0; k < 9000 && s.away == nil && !s.done; k++ {
			s.Update(pose, true, 0.1)
		}
		if s.away != nil {
			way("home", s.away.Path(), pose)
		}
	}},
	{"bus", func(t *testing.T, l *airport.Layout, pose GroundPose, prof MotionProfile, way func(string, *GroundPath, GroundPose)) {
		c := &tugClient{}
		inj := NewInjector(c)
		b := NewSimObjectBus(c, inj, "x", 9004, 0, prof)
		b.Layout = l
		if b.Attach(pose) != nil || b.arrive == nil {
			return
		}
		way("in", b.arrive.Path(), pose)
		b.Handle(assignedMsg(9004, 58))
		inj.Handle(groundMsg(DefaultInjectRequestBase+1, 58, 1200, 3))
		for k := 0; k < 9000 && b.arrive != nil; k++ {
			b.Update(pose, false, 0.1)
		}
		b.Update(pose, true, 0.1)
		if b.away != nil {
			way("home", b.away.Path(), pose)
		}
	}},
	{"tug", func(t *testing.T, l *airport.Layout, pose GroundPose, prof MotionProfile, way func(string, *GroundPath, GroundPose)) {
		c := &tugClient{}
		inj := NewInjector(c)
		tug := NewSimObjectTug(c, inj, DefaultTugTitle, 9001, prof)
		tug.Layout = l
		if tug.Attach(pose) != nil || tug.arrive == nil {
			return
		}
		way("in", tug.arrive.Path(), pose)
		tug.Handle(assignedMsg(9001, 55))
		inj.Handle(groundMsg(DefaultInjectRequestBase+1, 55, 1200, 3))
		for k := 0; k < 9000 && !tug.Connected(); k++ {
			tug.Update(pose, true, 0.1)
		}
		// Pushed to the stand's standard push pose (else straight back
		// 45 m), a step a frame.
		end := GroundPose{Position: offsetHeading(pose.Position, pose.Heading+180, 45), Heading: pose.Heading}
		if smoothPush != nil {
			if e, ok := smoothPush(); ok {
				end = e
			}
		}
		pushed := pose
		for k := 1; k <= 60; k++ {
			f := float64(k) / 60
			pushed = GroundPose{Position: airport.LatLon{Lat: pose.Position.Lat + (end.Position.Lat-pose.Position.Lat)*f, Lon: pose.Position.Lon + (end.Position.Lon-pose.Position.Lon)*f},
				Heading: normDeg(pose.Heading + headingDiff(end.Heading, pose.Heading)*f)}
			tug.Update(pushed, true, 0.1)
		}
		var seen *GroundMover
		for k := 0; k < 9000 && !tug.Done(); k++ {
			tug.Update(pushed, false, 0.1)
			if tug.away != nil && tug.away != seen && tug.homing {
				seen = tug.away
				way("home", tug.away.Path(), pushed)
			}
		}
	}},
}

// smoothPush is where the aircraft on the stand being swept ends its
// standard push (StandardPush), for the tug's way home.
var smoothPush func() (GroundPose, bool)

// TestServiceVehiclesSmooth (#1630).
func TestServiceVehiclesSmooth(t *testing.T) {
	prof := ProfileFor("A320").Motion
	for _, icao := range []string{"LKPR", "LROP"} {
		l := smoothLayout(t, icao)
		if len(l.VehicleDepots()) == 0 {
			t.Fatalf("%s: no vehicle depot", icao)
		}
		stands := smoothStands(l, prof)
		g, err := airport.BuildGraph(l)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range smoothVehicles {
			t.Run(icao+"/"+v.kind, func(t *testing.T) {
				bad, ways := map[string]int{}, 0
				for _, i := range stands {
					stand := l.Parking[i]
					pose := GroundPose{Position: StandPoint(stand, 0), Heading: stand.Heading}
					smoothPush = func() (GroundPose, bool) {
						r, ok := StandardPush(g, i, "A320")
						if !ok {
							return GroundPose{}, false
						}
						back := localDist(NoseGear(pose.Position, pose.Heading, prof), pose.Position)
						return GroundPose{Position: offsetHeading(r.Nose, r.Facing+180, back), Heading: r.Facing}, true
					}
					came, left := false, false
					v.run(t, l, pose, prof, func(phase string, p *GroundPath, at GroundPose) {
						ways++
						came, left = came || phase == "in", left || phase == "home"
						where := fmt.Sprintf("%s %s (%d) %s", v.kind, stand.Label(), i, phase)
						if d := hairpin(p); d >= 0 {
							bad["hairpin"]++
							t.Errorf("%s: turns back on itself %.0f m along its way", where, d)
						}
						if d := underFuselage(p, at, prof, phase == "in"); d >= 0 {
							bad["fuselage"]++
							t.Errorf("%s: under its aircraft's fuselage %.0f m along its way", where, d)
						}
						if o := overStand(p, l, i); o >= 0 {
							bad["stand"]++
							t.Errorf("%s: over stand %s's parking point", where, l.Parking[o].Label())
						}
					})
					if came && !left {
						bad["nohome"]++
						t.Errorf("%s %s (%d): came in and has no way home", v.kind, stand.Label(), i)
					}
				}
				t.Logf("%s %s: %d stands, %d ways, %v", icao, v.kind, len(stands), ways, bad)
				if ways == 0 {
					t.Error("no way driven")
				}
			})
		}
	}
}
