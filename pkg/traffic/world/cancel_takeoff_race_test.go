package world

import (
	"strings"
	"sync"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// raceDep is a departure whose roll began between the tower's look at its
// view ("lined up") and the cancel reaching it (#1685).
type raceDep struct {
	departureCtl
	mu       sync.Mutex
	rejected bool
}

// AbortTakeoff on a roll under way rejects it.
func (d *raceDep) AbortTakeoff() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.rejected = true
	return nil
}

// CancelTakeoffClearance on a roll under way: too late, it rolls on.
func (d *raceDep) CancelTakeoffClearance() error { return traffic.ErrTooLate }

// TestCancelTakeoffNeverRejects (#1685): the tower's clearance cancel for
// traffic on the runway never rejects a roll that began after it looked;
// the roll goes on, no cancel is said, and the clearance stays given.
func TestCancelTakeoffNeverRejects(t *testing.T) {
	w := New(Options{DataDir: t.TempDir()})
	cc := newControlCenter(nil, w.st.core)
	var said []string
	var saidMu sync.Mutex
	cc.radio = traffic.NewRadio(traffic.RadioOptions{Now: cc.clock.Now, OnTransmission: func(x traffic.Transmission) {
		saidMu.Lock()
		said = append(said, x.Text)
		saidMu.Unlock()
	}})
	stop := make(chan struct{})
	defer close(stop)
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
	dep := &raceDep{}
	it := &controlled{ID: 1, Tail: "TST1", ICAO: "LKPR", cc: cc, dep: dep, removed: make(chan struct{}), spoken: map[string]bool{}}
	it.view.State, it.view.Runway = "lined up", "24"
	tw := newTowers(cc, nil)
	tw.given["TST1 takeoff"] = true
	tw.apply("LKPR", "24", traffic.RunwayClearances{Waiting: map[string]string{"TST1": "CSA2 on the runway"}}, map[string]*controlled{"TST1": it}, nil)
	dep.mu.Lock()
	rejected := dep.rejected
	dep.mu.Unlock()
	if rejected {
		t.Errorf("the cancel rejected a roll under way")
	}
	saidMu.Lock()
	defer saidMu.Unlock()
	for _, s := range said {
		if strings.Contains(strings.ToLower(s), "cancel") {
			t.Errorf("said %q to a departure rolling on", s)
		}
	}
	tw.mu.Lock()
	given := tw.given["TST1 takeoff"]
	tw.mu.Unlock()
	if !given {
		t.Errorf("take-off clearance no longer given while it rolls on it")
	}
}
