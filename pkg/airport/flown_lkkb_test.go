package airport

import (
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/calc"
)

func loadJSON(t *testing.T, file string, v any) {
	t.Helper()
	b, err := os.ReadFile("testdata/" + file)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(err)
	}
}

// LKPR to LKKB, ILS 24 via SULOV (MyCrew, the user: "there should be a
// normal entry curve"): drawn as flown, every turn a curve, never a loop:
// with VENO4M and GOLO4W round the field, and on a short hop without them
// (no hairpin after the climb-out).
func TestFlownLKKBNoLoops(t *testing.T) {
	var arr Layout
	var dp, ap Procedures
	loadJSON(t, "LKKB-layout.json", &arr)
	loadJSON(t, "LKPR-procedures.json", &dp)
	loadJSON(t, "LKKB-procedures.json", &ap)
	dep := loadLayout(t, "LKPR")
	for _, c := range []struct{ sid, star string }{{"VENO4M", "GOLO4W"}, {"", ""}} {
		fr, err := FlownRouteFor(FlownRequest{Departure: dep, Arrival: &arr, DepartureRunway: "24", ArrivalRunway: "24",
			DepartureProcedures: dp, ArrivalProcedures: ap, SID: c.sid, STAR: c.star, Approach: "ILS 24", ApproachTransition: "SULOV"})
		if err != nil {
			t.Fatal(err)
		}
		pts := fr.Smoothed(TurnRadiusEnroute)
		var hs, ds []float64
		for i := 1; i < len(pts); i++ {
			a, b := pts[i-1], pts[i]
			if d := calc.HaversineMeters(a.Lat, a.Lon, b.Lat, b.Lon); d >= 0.5 {
				hs, ds = append(hs, calc.BearingDegrees(a.Lat, a.Lon, b.Lat, b.Lon)), append(ds, d)
			}
		}
		corner, loop := 0.0, 0.0
		for i := 1; i < len(hs); i++ {
			corner = math.Max(corner, math.Abs(math.Mod(hs[i]-hs[i-1]+540, 360)-180))
			sum, along := 0.0, 0.0
			for j := i; j < len(hs) && along < 3000; j++ {
				sum += math.Mod(hs[j]-hs[j-1]+540, 360) - 180
				along += ds[j]
				loop = math.Max(loop, math.Abs(sum))
			}
		}
		if corner > 10 || loop > 150 {
			t.Errorf("SID %q STAR %q: a corner of %.0f°, %.0f° of turning within 3 km", c.sid, c.star, corner, loop)
		}
	}
}
