package traffic

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// offWayEndMeters: it stands this near its point at the end, as one
// reached by the ways does (5 m on every LROP standby, the pose short of
// the way's end), and a metre.
const offWayEndMeters = 6.0

// TestRescueStandbyOffWay (#1656): a standby point with no way to it (no
// taxiway node within reach, LROP 08L's beside the runway was one) is
// still driven to from the depot: by the ways to where they pass nearest
// it, then across, every step smooth, no turning back, ending on it.
func TestRescueStandbyOffWay(t *testing.T) {
	n := 0
	for _, icao := range homeAirports {
		l := smoothLayout(t, icao)
		var spots []GroundPose
		homeEnds(l, func(rwy airport.Runway, end airport.RunwayEnd) {
			for _, side := range []float64{90, -90} {
				for a := rwy.Length / 3; a <= rwy.Length*2/3 && len(spots) < 4; a += 150 {
					p := offsetHeading(offsetHeading(end.Threshold, end.Heading, a), end.Heading+side, 105)
					d, ok := nearestDepot(l, p)
					if _, found := RescueStandbyWay(l, d, p); ok && !found {
						spots = append(spots, GroundPose{Position: p, Heading: end.Heading - side})
						break
					}
				}
			}
		})
		for k, spot := range spots {
			n++
			c := &tugClient{}
			r := NewSimObjectRescue(c, NewInjector(c), "Truck Fire Airport Medium", 9200)
			r.Layout = l
			if err := r.Standby(spot); err != nil {
				t.Fatal(err)
			}
			if !r.hasDepot {
				t.Errorf("%s %d: no way to %.6f,%.6f", icao, k, spot.Position.Lat, spot.Position.Lon)
				continue
			}
			if at := hairpin(r.mover.Path()); at >= 0 {
				t.Errorf("%s %d: turns back %.0f m out", icao, k, at)
			}
			r.Handle(assignedMsg(9200, 501))
			for i := 0; i < 30*60*10 && r.mover != nil; i++ {
				last := r.pose
				if err := r.Update(homeStep); err != nil {
					t.Fatal(err)
				}
				if bad := homeStepFault(last, r.pose); bad != "" {
					t.Errorf("%s %d: %s %.1f s out", icao, k, bad, float64(i)*homeStep)
					break
				}
			}
			if r.mover != nil {
				t.Errorf("%s %d: never there", icao, k)
			} else if d := localDist(r.pose.Position, spot.Position); d > offWayEndMeters {
				t.Errorf("%s %d: stopped %.1f m from its point", icao, k, d)
			}
		}
	}
	if n == 0 {
		t.Fatal("no point without a way: nothing checked")
	}
}
