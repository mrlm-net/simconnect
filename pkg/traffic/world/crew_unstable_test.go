package world

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// goAroundArr notes a go-around; no plan, no others to rejoin among.
type goAroundArr struct {
	arrivalCtl
	sent    int
	streams atomic.Int32
}

func (f *goAroundArr) GoAround() error                       { f.sent++; return nil }
func (f *goAroundArr) SetGoAroundStreams([][]airport.LatLon) { f.streams.Add(1) }
func (f *goAroundArr) Plan() *traffic.ArrivalPlan            { return nil }
func (f *goAroundArr) ProcedurePlan() []traffic.RoutePoint   { return nil }

// unstableBed is an arrival of ours and its controller, the controller's
// commands run.
func unstableBed(t *testing.T) (*controlCenter, *controlled, *goAroundArr) {
	t.Helper()
	w := New(Options{DataDir: t.TempDir()})
	cc := newControlCenter(nil, w.st.core)
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	go func() {
		for {
			select {
			case f := <-cc.cmds:
				f()
			case <-stop:
				return
			}
		}
	}()
	arr := &goAroundArr{}
	it := &controlled{ID: 1, Tail: "AAL432", ICAO: "TNCM", cc: cc, arr: arr, removed: make(chan struct{}), spoken: map[string]bool{}}
	return cc, it, arr
}

// Not stable at the gate, the crew goes around on its own (#1681): on a
// staffed station too, the crew's call and not the AI controller's
// (#1096); not the user's own with held gates on the map, the user the
// tower there, as for want of a landing clearance (crewDecides).
func TestCrewUnstablePlayers(t *testing.T) {
	for _, c := range []struct {
		name         string
		manual, sent bool
	}{{"staffed station", false, true}, {"held gates", true, false}} {
		t.Run(c.name, func(t *testing.T) {
			cc, it, arr := unstableBed(t)
			it.gates.Store(true)
			it.manualSet = c.manual
			cc.crewUnstable(it, false)
			if got := arr.sent > 0; got != c.sent {
				t.Errorf("sent round: %v, want %v", got, c.sent)
			}
		})
	}
}

// Never round for ever: crewUnstableLimit go-arounds, then it lands from
// the next; and not when the tower has sent it round already (#1681).
func TestCrewUnstableLimit(t *testing.T) {
	cc, it, arr := unstableBed(t)
	for range crewUnstableLimit + 2 {
		cc.crewUnstable(it, false)
	}
	if arr.sent != crewUnstableLimit {
		t.Errorf("%d go-arounds, want %d", arr.sent, crewUnstableLimit)
	}
	cc, it, arr = unstableBed(t)
	cc.crewGoAround = func(string) bool { return false }
	cc.crewUnstable(it, false)
	if arr.sent != 0 {
		t.Errorf("sent round again after the tower's go-around: %d", arr.sent)
	}
}

// Its crew may go round on its own at the gate (#1681): an arrival near
// its final has the others' streams for its circuit side up front
// (#1458), once per streamsAheadEvery, not on every frame; none high
// above its final yet.
func TestStreamsAheadOfCrewGoAround(t *testing.T) {
	given := func(agl float64, events int) int32 {
		_, it, arr := unstableBed(t)
		ev := &traffic.ArrivalEvent{State: traffic.ArrivalApproaching, AGL: agl}
		it.mu.Lock()
		for range events {
			it.streamsAhead(ev)
		}
		it.mu.Unlock()
		for end := time.Now().Add(time.Second); arr.streams.Load() == 0 && time.Now().Before(end); {
			time.Sleep(5 * time.Millisecond)
		}
		time.Sleep(20 * time.Millisecond)
		return arr.streams.Load()
	}
	if n := given(2000, 3); n != 1 {
		t.Errorf("2000 ft on its final, 3 events: streams given %d times, want once", n)
	}
	if n := given(9000, 1); n != 0 {
		t.Errorf("9000 ft, before its final: streams given %d times, want none", n)
	}
}
