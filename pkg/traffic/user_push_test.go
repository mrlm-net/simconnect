package traffic

import (
	"math"
	"strings"
	"testing"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

func userPushMsg(req uint32, p airport.LatLon, hdg float64, pushState float64) engine.Message {
	var hdr types.SIMCONNECT_RECV_SIMOBJECT_DATA
	off := int(unsafe.Offsetof(hdr.DwData))
	buf := make([]byte, off+int(unsafe.Sizeof(userPushData{})))
	h := (*types.SIMCONNECT_RECV_SIMOBJECT_DATA)(unsafe.Pointer(&buf[0]))
	h.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA)
	h.DwRequestID = types.DWORD(req)
	*(*userPushData)(unsafe.Pointer(&buf[off])) = userPushData{p.Lat, p.Lon, hdg, pushState, 1}
	return engine.Message{SIMCONNECT_RECV: (*types.SIMCONNECT_RECV)(unsafe.Pointer(&buf[0]))}
}

// TestUserPushDriver: a tug that goes tail first along the heading it is
// given follows LKPR's standard push from a stand to its end, started and
// stopped with TOGGLE_PUSHBACK.
func TestUserPushDriver(t *testing.T) {
	g := lkprGraph(t)
	var r PushRoute
	for i := range g.Layout.Parking {
		if p, ok := StandardPush(g, i, "FSLTL_B738_RYR"); ok && len(p.Tow) == 0 && pathLen(p.Points) > 60 {
			r = p
			break
		}
	}
	if len(r.Points) == 0 {
		t.Fatal("no push to drive")
	}
	ec := &eventClient{}
	d := NewUserPushDriver(ec, 0, nil)
	if err := d.Start(r); err != nil {
		t.Fatal(err)
	}
	pos := r.Points[0]
	hdg := math.Mod(localBearing(r.Points[1], r.Points[0])+360, 360) // nose away from the way back
	push := 3.0
	worst := 0.0
	for step := 0; step < 5000 && d.State() == UserPushPushing; step++ {
		d.Handle(userPushMsg(d.reqID, pos, hdg, push))
		if d.started {
			push = 0
		}
		if d.lastHdg >= 0 {
			// The tug turns the aircraft toward the heading asked, 2° a step.
			hdg = math.Mod(hdg+math.Max(-2, math.Min(2, headingDiff(hdg, d.lastHdg)))+360, 360)
		}
		pos = offsetHeading(pos, hdg+180, 0.3) // tail first
		worst = math.Max(worst, distToPath(pos, r.Points))
	}
	if d.State() != UserPushDone {
		t.Fatalf("state %s", d.State())
	}
	end := r.Points[len(r.Points)-1]
	if dd := localDist(pos, end); dd > 6 {
		t.Errorf("stopped %.1f m from the end", dd)
	}
	if worst > 6 {
		t.Errorf("strayed %.1f m off the route", worst)
	}
	if n := strings.Count(strings.Join(ec.events, ","), "TOGGLE_PUSHBACK"); n != 2 {
		t.Errorf("TOGGLE_PUSHBACK %d times, want start and stop: %v", n, ec.events)
	}
	t.Logf("pushed %.0f m, at most %.1f m off the route", pathLen(r.Points), worst)
}

// distToPath is the distance from p to the nearest segment of path.
func distToPath(p airport.LatLon, path []airport.LatLon) float64 {
	best := math.Inf(1)
	for i := 0; i+1 < len(path); i++ {
		a, b := path[i], path[i+1]
		seg := localDist(a, b)
		along := 0.0
		if seg > 0 {
			ax, ay := (p.Lon-a.Lon)*math.Cos(a.Lat*math.Pi/180), p.Lat-a.Lat
			bx, by := (b.Lon-a.Lon)*math.Cos(a.Lat*math.Pi/180), b.Lat-a.Lat
			along = math.Max(0, math.Min(1, (ax*bx+ay*by)/(bx*bx+by*by)))
		}
		q := airport.LatLon{Lat: a.Lat + (b.Lat-a.Lat)*along, Lon: a.Lon + (b.Lon-a.Lon)*along}
		best = math.Min(best, localDist(p, q))
	}
	return best
}
