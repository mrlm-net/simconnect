package worldtest

import (
	"math"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// TestNeighbourPushNoDeadlock: departures from LKPR's neighbouring stands
// C26 and C27, both with a tug and a fuel truck, the C27 one due half an
// hour later (live: DLH1560 held its push "for TVS1517 behind the stand",
// the parked neighbour, while TVS1517's tug and fuel truck gave way to
// DLH1560's push: 40 minutes and more, neither ever pushed). The C26 one
// pushes past its parked neighbour and taxis out; then the other does.
func TestNeighbourPushNoDeadlock(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	o := Options{DataDir: dataDir(t), ICAO: "LKPR", Manual: true, FrameRate: 10}
	if testing.Verbose() {
		o.World.Output = os.Stderr
	}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	stand := func(label string) int {
		for i, p := range h.Layout.Parking {
			if p.Label() == label {
				return i
			}
		}
		t.Fatalf("no stand %s", label)
		return -1
	}
	for _, r := range []world.SpawnRequest{
		{Kind: "departure", ICAO: "LKPR", Stand: stand("C26"), Runway: "24", Model: "FSLTL A320 DLH Lufthansa Star Alliance", Tail: "DLH1560", Tug: true, Fuel: true, PushInMin: 5},
		{Kind: "departure", ICAO: "LKPR", Stand: stand("C27"), Runway: "24", Model: "FSLTL_FAIB_B738_TVS-Skytravel_TSOC", Tail: "TVS1517", Tug: true, Fuel: true, PushInMin: 35},
	} {
		if _, err := h.World.Do(http.MethodPost, "/api/control", r); err != nil {
			t.Fatal(err)
		}
	}
	gone := func(state string) bool {
		return state != "" && state != "awaiting pushback" && state != "pushback" && state != "spawning"
	}
	state := map[string]string{}
	first := ""
	clear := math.Inf(1) // DLH1560's wing from TVS1517's fuselage while it pushes
	for range 60 * 60 {
		h.Advance(time.Second)
		var dlh, tvs world.ControlView
		for _, v := range h.World.Snapshot().Aircraft {
			state[v.Tail] = v.State
			switch v.Tail {
			case "DLH1560":
				dlh = v
			case "TVS1517":
				tvs = v
			}
		}
		if state["DLH1560"] == "pushback" && state["TVS1517"] == "awaiting pushback" {
			clear = min(clear, wingFromFuselage(dlh, 17.1, tvs, 18, 21))
		}
		if first == "" && gone(state["DLH1560"]) {
			first = "DLH1560"
		} else if first == "" && gone(state["TVS1517"]) {
			first = "TVS1517"
		}
		if gone(state["DLH1560"]) && gone(state["TVS1517"]) {
			break
		}
	}
	if first != "DLH1560" || !gone(state["TVS1517"]) {
		t.Fatalf("in an hour: first off %q, DLH1560 %s, TVS1517 %s; want DLH1560 first, both off", first, state["DLH1560"], state["TVS1517"])
	}
	if clear < 1 {
		t.Errorf("DLH1560's wing came %.1f m from TVS1517's fuselage pushing past it", clear)
	}
	t.Logf("wing clear of the neighbour's fuselage by %.1f m at the closest", clear)
}

// wingFromFuselage is how far the wing of a (half span halfM, across its
// position) is from b's fuselage (noseM ahead of its position to tailM
// behind), metres; negative through it.
func wingFromFuselage(a world.ControlView, halfM float64, b world.ControlView, noseM, tailM float64) float64 {
	local := func(lat, lon float64) (float64, float64) {
		return (lon - b.Position.Lon) * 111320 * math.Cos(b.Position.Lat*math.Pi/180), (lat - b.Position.Lat) * 110540
	}
	at := func(x, y, hdg, d float64) (float64, float64) {
		r := hdg * math.Pi / 180
		return x + d*math.Sin(r), y + d*math.Cos(r)
	}
	ax, ay := local(a.Position.Lat, a.Position.Lon)
	w1x, w1y := at(ax, ay, a.Heading+90, halfM)
	w2x, w2y := at(ax, ay, a.Heading-90, halfM)
	f1x, f1y := at(0, 0, b.Heading, noseM)
	f2x, f2y := at(0, 0, b.Heading, -tailM)
	return segmentDistance(w1x, w1y, w2x, w2y, f1x, f1y, f2x, f2y)
}

// segmentDistance is the distance between segments p1-p2 and q1-q2 in a
// plane (0 crossing).
func segmentDistance(p1x, p1y, p2x, p2y, q1x, q1y, q2x, q2y float64) float64 {
	cross := func(ax, ay, bx, by, cx, cy float64) float64 { return (bx-ax)*(cy-ay) - (by-ay)*(cx-ax) }
	if cross(p1x, p1y, p2x, p2y, q1x, q1y)*cross(p1x, p1y, p2x, p2y, q2x, q2y) < 0 &&
		cross(q1x, q1y, q2x, q2y, p1x, p1y)*cross(q1x, q1y, q2x, q2y, p2x, p2y) < 0 {
		return 0
	}
	toSeg := func(px, py, ax, ay, bx, by float64) float64 {
		dx, dy := bx-ax, by-ay
		t := math.Max(0, math.Min(1, ((px-ax)*dx+(py-ay)*dy)/(dx*dx+dy*dy)))
		return math.Hypot(px-ax-t*dx, py-ay-t*dy)
	}
	return min(toSeg(p1x, p1y, q1x, q1y, q2x, q2y), toSeg(p2x, p2y, q1x, q1y, q2x, q2y),
		toSeg(q1x, q1y, p1x, p1y, p2x, p2y), toSeg(q2x, q2y, p1x, p1y, p2x, p2y))
}
