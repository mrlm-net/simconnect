package traffic

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// A published missed approach point straight ahead but short of the
// go-around's climb-out goes to the climb-out's end (#1573: LKPR's FM leg
// from RW06 ends 2.7 NM out once the runway fix is on the threshold);
// points off the centreline, beyond the climb-out or behind the threshold
// stay, and so does their number.
func TestClimbOutMissed(t *testing.T) {
	thr := airport.LatLon{Lat: 50.1, Lon: 14.2}
	const hdg, climbM = 60.0, 3.5 * 1852
	short := offsetHeading(thr, hdg, 2.7*1852)
	beyond := offsetHeading(thr, hdg, 5*1852)
	aside := offsetHeading(offsetHeading(thr, hdg, 2*1852), hdg+90, 2*1852)
	behind := offsetHeading(thr, hdg, -1852)
	in := []airport.NavPoint{{Position: short}, {Ident: "A", Position: aside}, {Ident: "B", Position: beyond}, {Ident: "C", Position: behind}, {Ident: "V"}}
	got := climbOutMissed(in, thr, hdg, climbM)
	if len(got) != len(in) {
		t.Fatalf("%d points, want %d", len(got), len(in))
	}
	if a := alongHeading(thr, hdg, got[0].Position); math.Abs(a-climbM) > 1 {
		t.Errorf("short point %.0f m out, want %.0f", a, climbM)
	}
	for i := 1; i < len(in); i++ {
		if got[i] != in[i] {
			t.Errorf("point %d moved: %+v", i, got[i])
		}
	}
	if in[0].Position != short {
		t.Error("input changed")
	}
}
