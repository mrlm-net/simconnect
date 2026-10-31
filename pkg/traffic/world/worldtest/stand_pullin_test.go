package worldtest

import (
	"math"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TestFaceOutStandPullsIn: LOWI E1 faces out and taxiway L passes it, up from
// behind and on to the lead-in ahead. Off 26, coming from behind, an arrival pulls
// straight in, not on past the stand and back round. Off 08, coming from ahead, it
// has to turn: on L's side of the stand, not over the ground on the other (#1339).
func TestFaceOutStandPullsIn(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	for _, tc := range []struct {
		rwy      string
		maxAhead float64
	}{{"26", 6}, {"08", 70}} {
		t.Run(tc.rwy, func(t *testing.T) {
			o := Options{DataDir: dataDir(t), ICAO: "LOWI", Manual: true, FrameRate: 10}
			if testing.Verbose() {
				o.World.Output = os.Stderr
			}
			h, err := Start(o)
			if err != nil {
				t.Fatal(err)
			}
			defer h.Close()
			h.World.UseRunway("LOWI", tc.rwy)
			stand := -1
			for i, p := range h.Layout.Parking {
				if p.Label() == "E1" {
					stand = i
				}
			}
			if stand < 0 {
				t.Fatal("no stand E1")
			}
			p := h.Layout.Parking[stand]
			if _, err := h.World.Do(http.MethodPost, "/api/control", map[string]any{"kind": "arrival", "icao": "LOWI", "tail": "PULL1", "runway": tc.rwy, "stand": stand, "model": "A320"}); err != nil {
				t.Fatal(err)
			}
			ahead, right := math.Inf(-1), math.Inf(-1)
			if !h.Until(45*time.Minute, func() bool {
				v, ok := h.Aircraft("PULL1")
				if !ok {
					return false
				}
				if v.Stand != "E1" {
					t.Fatalf("given stand %q", v.Stand)
				}
				d := calc.HaversineMeters(p.Position.Lat, p.Position.Lon, v.Position.Lat, v.Position.Lon)
				if v.OnGround && d < 100 {
					off := (calc.BearingDegrees(p.Position.Lat, p.Position.Lon, v.Position.Lat, v.Position.Lon) - p.Heading) * math.Pi / 180
					ahead = max(ahead, d*math.Cos(off))
					if d*math.Cos(off) < -5 {
						right = max(right, d*math.Sin(off))
					}
				}
				return v.State == traffic.ArrivalParked.String()
			}) {
				v, _ := h.Aircraft("PULL1")
				t.Fatalf("not parked: %s", v.State)
			}
			t.Logf("runway %s: at most %.0f m ahead of the stand, %.0f m right of it behind", tc.rwy, ahead, right)
			if ahead > tc.maxAhead {
				t.Errorf("%.0f m past the stand before coming back, want at most %.0f", ahead, tc.maxAhead)
			}
			if right > 5 {
				t.Errorf("%.0f m off to the side away from taxiway L", right)
			}
		})
	}
}
