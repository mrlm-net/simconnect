package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestParkedAircraftDoesNotBlockTheTaxilane: an A321 parked on LKPR C27
// reaches with its tail within a wing of H1 behind it; taxiing traffic on
// H1 passes behind it (the user, live: AFR1002 stopped 18 min for WZZ1469
// on C27, whose pushback waited for AFR1002). Once it pushes, it counts.
func TestParkedAircraftDoesNotBlockTheTaxilane(t *testing.T) {
	g := lkprGraph(t)
	i, err := g.Layout.ParkingIndex("C27")
	if err != nil {
		t.Fatal(err)
	}
	stand := g.Layout.Parking[i]
	// The H1 segment nearest behind the stand, as a straight taxi path.
	f := newFlat(stand.Position)
	best, a, b := math.Inf(1), airport.LatLon{}, airport.LatLon{}
	for id, es := range g.Adj {
		for _, e := range es {
			if e.Name != "H1" {
				continue
			}
			p, q := g.Nodes[id].Position, g.Nodes[e.To].Position
			mx, my := f.xy(airport.LatLon{Lat: (p.Lat + q.Lat) / 2, Lon: (p.Lon + q.Lon) / 2})
			if d := math.Hypot(mx, my); d < best && localDist(p, q) > 20 {
				best, a, b = d, p, q
			}
		}
	}
	h := localBearing(a, b)
	mid := airport.LatLon{Lat: (a.Lat + b.Lat) / 2, Lon: (a.Lon + b.Lon) / 2}
	prof := ProfileFor("A320")
	path, err := NewGroundPath([]airport.LatLon{offsetHeading(mid, h, -150), offsetHeading(mid, h, 150)}, prof.Motion)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	pic := NewGroundPicture()
	const parked, taxiing = 1, 2
	pic.Report(parked, stand.Position, stand.Heading, ProfileFor("A321").Motion, now)
	half := prof.WingspanM / 2
	if body, _ := pic.blocking(taxiing, path, 0, 300, half, now); math.IsInf(body, 1) {
		t.Fatal("the A321's tail is not within a wing of H1: the case is gone")
	}
	pic.ReportParked(parked, true)
	if body, who := pic.blocking(taxiing, path, 0, 300, half, now); !math.IsInf(body, 1) {
		t.Errorf("stopped %.0f m on behind #%d, parked on C27", body, who.id)
	}
	pic.ReportParked(parked, false) // pushing: in the way again
	if body, _ := pic.blocking(taxiing, path, 0, 300, half, now); math.IsInf(body, 1) {
		t.Error("not stopped for it once it moves")
	}
}
