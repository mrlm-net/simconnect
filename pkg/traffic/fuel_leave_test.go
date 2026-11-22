package traffic

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// TestFuelTruckLeavesClearOfItsAircraft (#1630): at every LKPR stand an
// airliner uses, the fuel truck's way home from the wing never runs
// through its own aircraft (live and offline, it drove on past the nose,
// turned round on the spot and back under the fuselage to the road behind
// the tail) and never turns back on itself.
func TestFuelTruckLeavesClearOfItsAircraft(t *testing.T) {
	g := lkprGraph(t)
	l := g.Layout
	prof := ProfileFor("A320").Motion
	bad := 0
	for i, stand := range l.Parking {
		if stand.Type == types.SIMCONNECT_FACILITY_TAXI_PARKING_TYPE_VEHICLE || stand.Radius < 15 {
			continue
		}
		c := &tugClient{}
		inj := NewInjector(c)
		f := NewSimObjectFuelTruck(c, inj, "FSDT_FuelTruck_7296", 9002, prof)
		f.Layout = l
		pose := GroundPose{Position: StandPoint(stand, 0), Heading: stand.Heading}
		if err := f.Attach(pose); err != nil {
			t.Fatal(err)
		}
		if !f.hasDepot {
			continue
		}
		f.Handle(assignedMsg(9002, 56))
		inj.Handle(groundMsg(DefaultInjectRequestBase+1, 56, 1200, 3))
		for k := 0; k < 10*900 && !f.Fuelling(); k++ {
			f.Update(pose, false, 0.1)
		}
		if !f.Fuelling() {
			continue
		}
		frame := aircraftFrame{nose: NoseGear(pose.Position, pose.Heading, prof), hdg: pose.Heading, prof: prof}
		// Headings every metre driven: no turn back on itself within
		// hairpinMeters.
		const hairpinMeters = 12.0
		var heads []float64
		var last *airport.LatLon
		hairpin := false
		var marks []airport.LatLon
		for k := 0; k < 10*900 && !f.Done(); k++ {
			f.Update(pose, true, 0.1)
			p := f.pose
			x, y := frame.to(p.Position)
			if math.Abs(x) < 2.5 && y < 0 && y > -(prof.WheelbaseMeters+prof.TailMeters) {
				marks = append(marks, p.Position)
			}
			if last == nil || localDist(*last, p.Position) >= 1 {
				q := p.Position
				last = &q
				heads = append(heads, p.Heading)
				for j := max(0, len(heads)-int(hairpinMeters)); j < len(heads)-1 && !hairpin; j++ {
					if math.Abs(headingDiff(heads[j], p.Heading)) > 150 {
						hairpin = true
						t.Errorf("%s (%d): the fuel truck turned back on itself (%.0f° to %.0f° within %.0f m)", stand.Label(), i, heads[j], p.Heading, hairpinMeters)
					}
				}
			}
		}
		if len(marks) > 0 {
			bad++
			t.Errorf("%s (%d): the fuel truck drove home under its aircraft's fuselage (%d samples)", stand.Label(), i, len(marks))
		}
	}
	t.Logf("%d stands with the truck through the fuselage", bad)
}
