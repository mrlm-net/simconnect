package world

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// countPlanner counts its searches and the steps taken.
type countPlanner struct {
	searched, took int
	boom           bool
}

func (p *countPlanner) WayOffClear([]airport.Occupied) bool {
	if p.boom {
		panic("boom")
	}
	return false
}

func (p *countPlanner) PlanWayOff(int, []airport.Occupied) func() bool {
	p.searched++
	return func() bool {
		p.took++
		return true
	}
}

// TestWayOffSearchOffConnection (#1655): the actuator searches off its
// connection's goroutine, where a search would hold up the frames, and
// puts what it found in place on it; every other call as before.
func TestWayOffSearchOffConnection(t *testing.T) {
	p := &countPlanner{}
	applied, onLoop := 0, 0
	busy := func(func() error) error {
		onLoop++
		return errors.New("simulator connection busy")
	}
	srv := newWireServer()
	srv.add("arr/1"+wayOffSuffix, &actuatorWayOff{p: p, do: func(f func() error) error {
		applied++
		return f()
	}})
	call := func(target, method string, args ...any) wireMsg {
		m := wireMsg{Kind: wireCall, Target: target, Method: method}
		for _, a := range args {
			b, err := json.Marshal(a)
			if err != nil {
				t.Fatal(err)
			}
			m.Args = append(m.Args, b)
		}
		return srv.answer(m, busy)
	}
	target := "arr/1" + wayOffSuffix
	var none []airport.Occupied
	if r := call(target, "SearchWayOff", uint64(1), 0, none); r.Err != "" || p.searched != 1 || onLoop != 0 || applied != 0 {
		t.Fatalf("search: %q, searched %d, on the connection %d and %d", r.Err, p.searched, onLoop, applied)
	}
	if r := call(target, "TakeWayOff", uint64(1), 0, none); r.Err != "" || p.searched != 1 || p.took != 1 || applied != 1 {
		t.Errorf("take: %q, searched %d, took %d, on the connection %d", r.Err, p.searched, p.took, applied)
	}
	// A follower of a fan that hasn't that step searches it itself.
	if r := call(target, "TakeWayOff", uint64(2), 0, none); r.Err != "" || p.searched != 2 || p.took != 2 {
		t.Errorf("a step not kept: %q, searched %d, took %d", r.Err, p.searched, p.took)
	}
	if r := call("arr/1", "State"); r.Err == "" || onLoop != 1 {
		t.Errorf("another call not on the connection: %q", r.Err)
	}
	p.boom = true
	if r := call(target, "CheckWayOff", none); r.Err == "" {
		t.Error("a panic answered as if all was well")
	}
}
