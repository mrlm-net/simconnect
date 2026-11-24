package traffic

import (
	"sync"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// The rescue vehicles' route searches off the frame's goroutine (#1670):
// held, the frames go on, the vehicle stands or drives on as it was; once
// a search ends, the next frame puts its way in place, no step in it.

// searchGate holds the searches given to it (run, as Search) until
// opened, counting them.
type searchGate struct {
	mu    sync.Mutex
	asked int
	open  chan struct{}
	wg    sync.WaitGroup
}

func newSearchGate() *searchGate { return &searchGate{open: make(chan struct{})} }

func (g *searchGate) run(f func()) {
	g.mu.Lock()
	g.asked++
	g.mu.Unlock()
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		<-g.open
		f()
	}()
}

func (g *searchGate) count() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.asked
}

// release opens the gate and waits for the searches given to end.
func (g *searchGate) release() {
	close(g.open)
	g.wg.Wait()
}

// searchHeldFrames is how many frames each search is held for.
const searchHeldFrames = 20

// searchSpot is LKPR's first standby point and the way in to an aircraft
// half way down that runway, there.
func searchSpot(t *testing.T) (*airport.Layout, GroundPose, RescueWay, GroundPose) {
	t.Helper()
	l := smoothLayout(t, "LKPR")
	g, err := airport.BuildGraph(l)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range l.Runways {
		rwy, end, _ := l.RunwayEnd(r.Primary.Name)
		spots := RescueStandby(l, end.Name, 1)
		at := offsetHeading(end.Threshold, end.Heading, rwy.Length/2)
		if way, ok := RescueAccess(g, end.Name, at, false); ok && len(spots) > 0 {
			return l, spots[0], way, GroundPose{Position: at, Heading: end.Heading}
		}
	}
	t.Fatal("no standby point with a way in at LKPR")
	return nil, GroundPose{}, RescueWay{}, GroundPose{}
}

// TestRescueEnterSearchOffFrame: sent in from standing at standby, its way
// in looked for while frames go on, it stands until the search ends, then
// drives in from there.
func TestRescueEnterSearchOffFrame(t *testing.T) {
	l, spot, way, place := searchSpot(t)
	crew := homeCrew(t, l, []GroundPose{spot}, nil)
	if crew == nil {
		t.Fatal("never at standby: nothing checked")
	}
	r := crew[0]
	gate := newSearchGate()
	r.Search = gate.run
	stood := r.pose
	if err := r.Enter(way); err != nil {
		t.Fatal(err)
	}
	if n := gate.count(); n != 1 {
		t.Fatalf("%d searches as it was sent in, want 1", n)
	}
	for i := 0; i < searchHeldFrames; i++ {
		if err := r.Follow(place, 0, homeStep); err != nil {
			t.Fatal(err)
		}
		if _, route, _ := r.Track(); r.pose.Position != stood.Position || len(route) > 0 {
			t.Fatalf("frame %d of the search: moved %.1f m, %d points ahead; want it standing", i, localDist(stood.Position, r.pose.Position), len(route))
		}
	}
	gate.release()
	if err := r.Follow(place, 0, homeStep); err != nil {
		t.Fatal(err)
	}
	if _, route, _ := r.Track(); len(route) < 2 {
		t.Fatalf("%d points ahead the frame after its way in was found, want its way", len(route))
	}
	if bad := convoySteps(t, r, 60, func(float64) error { return r.Follow(place, 0, homeStep) }); bad != "" {
		t.Errorf("driving in after the search: %s", bad)
	}
	if moved := localDist(stood.Position, r.pose.Position); moved < 5 {
		t.Errorf("%.1f m in a minute after its way in was found, want it driving in", moved)
	}
}

// TestRescueHomeSearchOffFrame: sent home standing at standby, and sent
// home driving out to it, its way home looked for while frames go on: it
// stands, or drives on its way, until the search ends; then it goes home
// from where it is, no step in it, and gets there.
func TestRescueHomeSearchOffFrame(t *testing.T) {
	l, spot, _, _ := searchSpot(t)
	for _, driving := range []bool{false, true} {
		var crew []*SimObjectRescue
		if driving {
			crew = homeCrew(t, l, []GroundPose{spot}, func(sec float64) bool { return sec >= 30 })
		} else {
			crew = homeCrew(t, l, []GroundPose{spot}, nil)
		}
		if crew == nil {
			t.Fatalf("driving %v: no vehicle: nothing checked", driving)
		}
		r := crew[0]
		if driving && r.pose.GroundSpeedKts < 5 {
			t.Fatalf("%.1f kt 30 s out: not driving: nothing checked", r.pose.GroundSpeedKts)
		}
		gate := newSearchGate()
		r.Search = gate.run
		if err := r.Home(); err != nil {
			t.Fatal(err)
		}
		if n := gate.count(); n != 1 {
			t.Fatalf("driving %v: %d searches as it was sent home, want 1", driving, n)
		}
		was := r.pose
		for i := 0; i < searchHeldFrames; i++ {
			last := r.pose
			if err := r.Update(homeStep); err != nil {
				t.Fatal(err)
			}
			if r.Done() {
				t.Fatalf("driving %v: gone before its way home was found", driving)
			}
			if bad := homeStepFault(last, r.pose); bad != "" {
				t.Fatalf("driving %v, frame %d of the search: %s", driving, i, bad)
			}
		}
		if moved := localDist(was.Position, r.pose.Position); driving && moved < 1 || !driving && moved > 0 {
			t.Errorf("driving %v: %.1f m on during the search; want it standing, or driving on", driving, moved)
		}
		gate.release()
		if bad := convoySteps(t, r, 30*60, func(float64) error { return r.Update(homeStep) }); bad != "" {
			t.Errorf("driving %v, home after the search: %s", driving, bad)
		}
		if !r.Done() || !homeArrived(r) {
			t.Errorf("driving %v: done %v, %.0f m from the depot; want it home", driving, r.Done(), localDist(r.pose.Position, r.depot))
		}
	}
}
