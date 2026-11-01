package traffic

import (
	"math"
	"testing"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// TestGroundSpikeIsNotFollowed: the simulator reports a wrong ground for one
// frame; the height sent stays level, and the spike stays out of the map the
// next aircraft reads there (live, LKPR: 0.39 ft up in the frame after it). A
// real change of the ground is followed a sample later.
func TestGroundSpikeIsNotFollowed(t *testing.T) {
	c := &eventClient{}
	inj := NewInjector(c)
	const obj = 42
	if err := inj.Takeover(obj); err != nil {
		t.Fatal(err)
	}
	at := airport.LatLon{Lat: 50.1, Lon: 14.2}
	sent := func(ft float64) float64 {
		t.Helper()
		at = offsetHeading(at, 90, 0.5)
		m := groundMsg(DefaultInjectRequestBase+1, obj, ft, 8.48)
		g := (*injectGround)(unsafe.Add(unsafe.Pointer(m.SIMCONNECT_RECV), unsafe.Offsetof(types.SIMCONNECT_RECV_SIMOBJECT_DATA{}.DwData)))
		g.Lat, g.Lon, g.GS = at.Lat, at.Lon, 20
		inj.Handle(m)
		got, _ := inj.GroundFt(obj)
		return got
	}
	for range 3 {
		sent(1174.15)
	}
	spike := offsetHeading(at, 90, 0.5)
	if got := sent(1174.54); math.Abs(got-1174.15) > 0.001 {
		t.Errorf("at the spike: ground %.2f, want 1174.15", got)
	}
	if ft, ok := inj.ground.at(spike); ok {
		t.Errorf("the spike in the map: %.2f", ft)
	}
	if got := sent(1174.15); math.Abs(got-1174.15) > 0.001 {
		t.Errorf("after the spike: ground %.2f, want 1174.15", got)
	}
	// Up for good: followed.
	sent(1175)
	if got := sent(1175); math.Abs(got-1175) > 0.001 {
		t.Errorf("a real step: ground %.2f, want 1175", got)
	}
	// Placed far away: the new ground at once.
	at = offsetHeading(at, 90, 500)
	if got := sent(1300); got != 1300 {
		t.Errorf("elsewhere: ground %.2f, want 1300", got)
	}
}
