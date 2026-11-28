package world

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// An arrival's way-off planning (#1640, #1641) with the director split from
// the actuator (#1655). The plan is searched where its controller is, off
// that connection's goroutine, and put in place on it; the director alone
// decides, so a player's arrival (#1096) is never asked about. It looks off
// its own loop too (lookOverLink): a call takes as long as the link takes,
// up to wireCallTimeout, and all else it runs would wait. An actuator
// from before says nothing of it at the arrival's start and is never asked
// either: no planning there, as before.

var (
	_ wayOffCalls   = (*remoteWayOff)(nil)
	_ wayOffCalls   = (*actuatorWayOff)(nil)
	_ wayOffPlanner = (*remotePlanner)(nil)
)

// wayOffSuffix ends the target of an arrival's way-off planning.
const wayOffSuffix = "/wayoff"

// wayOffSteps numbers the searches: the director's numbers, so that the
// same one means the same on every actuator of a fan.
var wayOffSteps atomic.Uint64

// remoteWayOff is the director's stand-in for an arrival's wayOffCalls
// (remote_gen.go).
type remoteWayOff struct {
	c *wireClient
	t string
}

// wayOffPlannerOf is arrival arr's way-off planning; false: it has none.
func wayOffPlannerOf(arr arrivalCtl) (wayOffPlanner, bool) {
	if p, ok := arr.(wayOffPlanner); ok {
		return p, true
	}
	if r, ok := arr.(*remoteArr); ok && r.plansWayOff {
		return &remotePlanner{arr: r, w: remoteWayOff{c: r.c, t: r.t + wayOffSuffix}}, true
	}
	return nil, false
}

// remotePlanner is one look at an actuator arrival's way off: a new one
// each time, so that a call lost spoils that look only.
type remotePlanner struct {
	arr    *remoteArr
	w      remoteWayOff
	failed atomic.Bool
}

// lost is whether a call of this look failed: what it found means nothing.
func (p *remotePlanner) lost() bool { return p.failed.Load() }

func (p *remotePlanner) fail(err error) {
	p.failed.Store(true)
	p.w.c.dropped(err)
}

// WayOffClear asks the controller; with nothing taken there is nothing to
// ask. True when the call is lost: nothing is done on a guess.
func (p *remotePlanner) WayOffClear(occ []airport.Occupied) bool {
	if len(occ) == 0 || p.lost() {
		return true
	}
	clear, err := p.w.CheckWayOff(occ)
	if err != nil {
		p.fail(err)
		return true
	}
	return clear
}

// PlanWayOff searches on the actuator: nil none clear or the call lost
// (lost), else the step that puts what it found in place there.
func (p *remotePlanner) PlanWayOff(parking int, occ []airport.Occupied) func() bool {
	if p.lost() {
		return nil
	}
	step := wayOffSteps.Add(1)
	found, err := p.w.SearchWayOff(step, parking, occ)
	if err != nil {
		p.fail(err)
		return nil
	}
	if !found {
		return nil
	}
	return func() bool {
		// Its plan changes: the director's snapshot of it is no longer true.
		defer p.arr.c.commanding(p.arr.t, "TakeWayOff")()
		took, err := p.w.TakeWayOff(step, parking, occ)
		if err != nil {
			p.fail(err)
			return false
		}
		return took
	}
}

// actuatorWayOff serves an arrival's wayOffCalls. Its methods are called
// off the connection's goroutine (wireServer.answer).
type actuatorWayOff struct {
	p wayOffPlanner
	// do runs on the connection's goroutine; nil: at once.
	do func(func() error) error

	mu      sync.Mutex
	step    uint64
	parking int
	apply   func() bool
}

// CheckWayOff is whether its way off is clear of occ.
func (a *actuatorWayOff) CheckWayOff(occ []airport.Occupied) (bool, error) {
	return a.p.WayOffClear(occ), nil
}

// SearchWayOff is whether a way off clear of occ to parking is found; kept
// as step.
func (a *actuatorWayOff) SearchWayOff(step uint64, parking int, occ []airport.Occupied) (bool, error) {
	apply := a.p.PlanWayOff(parking, occ)
	if apply == nil {
		return false, nil
	}
	a.mu.Lock()
	a.step, a.parking, a.apply = step, parking, apply
	a.mu.Unlock()
	return true, nil
}

// TakeWayOff puts step in place. One not kept is searched again: a
// follower's of a fan, whose search the director does not wait for.
func (a *actuatorWayOff) TakeWayOff(step uint64, parking int, occ []airport.Occupied) (bool, error) {
	a.mu.Lock()
	var apply func() bool
	if a.step == step && a.parking == parking {
		apply, a.apply = a.apply, nil
	}
	a.mu.Unlock()
	if apply == nil {
		if apply = a.p.PlanWayOff(parking, occ); apply == nil {
			return false, nil
		}
	}
	if a.do == nil {
		return apply(), nil
	}
	var took bool
	if err := a.do(func() error { took = apply(); return nil }); err != nil {
		return false, err
	}
	return took, nil
}

// lookOverLink is keepWayOffClear's look at an actuator arrival's way off,
// off the director's loop, every call of it. Its answer is dropped when what
// it was asked for holds no longer; none is never "no way off", which would
// send the arrival round. On the loop only what the answer says is put in
// place, asking nothing.
func (cc *controlCenter) lookOverLink(it *controlled, p *remotePlanner, items []*controlled) {
	defer it.wayOffLooking.Store(false)
	if !lookable(it) {
		it.wayOffBlocked.Store(false) // down, or not answered
		return
	}
	l := cc.lookWayOff(it, p, items)
	if p.lost() {
		it.wayOffBlocked.Store(false)
		return
	}
	if l.backtracks || l.exitOnly {
		now := cc.snapshotItems()
		if cc.wayOffStale(it, l.stand, l.occ) != "" || l.clear && l.apron != apronPlacesFor(it, now).key {
			return // looked at afresh a second later
		}
	}
	if err := cc.do(func() error {
		if it.gates.Load() || it.standNow() != l.stand {
			return nil // a player's or given another stand since
		}
		cc.takeLook(it, p, l)
		return nil
	}); err != nil {
		cc.log.printf("%-6s ground: way off the runway: %v", it.Tail, err)
	}
}

// answer is call m's reply: run on the connection's goroutine by do, but
// for the way-off planning, whose route searches would hold up the frames.
func (s *wireServer) answer(m wireMsg, do func(func() error) error) (r wireMsg) {
	if strings.HasSuffix(m.Target, wayOffSuffix) {
		defer func() {
			if e := recover(); e != nil {
				r = wireMsg{Kind: wireReply, ID: m.ID, Err: fmt.Sprintf("world: wire: %s.%s panicked: %v", m.Target, m.Method, e)}
			}
		}()
		return s.dispatch(m)
	}
	if err := do(func() error { r = s.dispatch(m); return nil }); err != nil {
		r = wireMsg{Kind: wireReply, ID: m.ID, Err: err.Error()}
	}
	return r
}
