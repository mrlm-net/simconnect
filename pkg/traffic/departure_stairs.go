package traffic

import (
	"errors"
	"time"
)

// Stand services (#831, #832): stairs at the front left door and a ground
// power unit at the nose, on a remote stand. Each is sent once the aircraft
// waits on its stand (StandServiceStartDelay), stays until
// StandServiceClearMargin before the tug comes (TugLeadTime before the
// push) or the push is cleared, and the push waits for it to leave. With
// none free in the fleet, none.
const (
	StandServiceStartDelay   = 10 * time.Second
	StandServiceClearMargin  = 2 * time.Minute
	StandServiceClearTimeout = 2 * time.Minute
)

// Deprecated names, kept for v0.20 callers: the stairs' timings.
const (
	StairsStartDelay   = StandServiceStartDelay
	StairsClearMargin  = StandServiceClearMargin
	StairsClearTimeout = StandServiceClearTimeout
)

// standService is a departure's stairs or GPU and where it is.
type standService struct {
	kind      VehicleKind
	what      string // "stairs", "GPU": for errors
	attached  bool
	waitFrom  time.Time // first frame waiting on the stand
	clearFrom time.Time // the push first waited for it to leave
}

// standServices are the departure's stand services with their vehicles.
func (c *TaxiController) standServices() []struct {
	v FuelService
	s *standService
} {
	c.stairsSvc.kind, c.stairsSvc.what = VehicleStairs, "stairs"
	c.gpuSvc.kind, c.gpuSvc.what = VehicleGPU, "GPU"
	return []struct {
		v FuelService
		s *standService
	}{{c.req.Stairs, &c.stairsSvc}, {c.req.GPU, &c.gpuSvc}}
}

// updateStairs drives the stand services (stairs, GPU) each frame.
func (c *TaxiController) updateStairs(dt float64) {
	for _, x := range c.standServices() {
		c.updateStandService(x.v, x.s, dt)
	}
}

func (c *TaxiController) updateStandService(v FuelService, st *standService, dt float64) {
	if v == nil {
		return
	}
	if v.Done() {
		c.give(st.kind)
		return
	}
	now := c.now()
	stand := c.req.Graph.Layout.Parking[c.req.Parking]
	pose := GroundPose{Position: StandPoint(stand, c.req.NoseOffset), Heading: stand.Heading}
	if c.mover != nil {
		pose = c.mover.Pose()
	}
	leaveBy := c.gateAt.Add(-TugLeadTime - StandServiceClearMargin)
	if !st.attached {
		if c.state != TaxiAwaitingPushback || c.pushCleared || !c.pushAt.IsZero() || !now.Before(leaveBy) {
			return
		}
		if st.waitFrom.IsZero() {
			st.waitFrom = now
		}
		if now.Before(st.waitFrom.Add(StandServiceStartDelay)) || !c.take(st.kind) {
			return
		}
		st.attached = true
		c.giveTraffic(v)
		c.note(st.what, nil)
		if err := v.Attach(pose); err != nil {
			c.fuelErr(err)
			c.fuelErr(v.Remove())
		}
		return
	}
	leave := c.state != TaxiAwaitingPushback || c.pushCleared || !c.pushAt.IsZero() || !now.Before(leaveBy)
	c.fuelErr(v.Update(pose, leave, dt))
}

// stairsClear reports that no stand service is at the aircraft: none,
// never sent, gone or leaving. One still there after
// StandServiceClearTimeout is removed.
func (c *TaxiController) stairsClear() bool {
	clear := true
	for _, x := range c.standServices() {
		v, st := x.v, x.s
		if v == nil || !st.attached || v.Done() || v.Clear() {
			continue
		}
		now := c.now()
		if st.clearFrom.IsZero() {
			st.clearFrom = now
		}
		if now.Sub(st.clearFrom) < StandServiceClearTimeout {
			clear = false
			continue
		}
		c.fuelErr(errors.New("the " + st.what + " did not leave: removed"))
		c.fuelErr(v.Remove())
	}
	return clear
}

// stairsDriving reports that a stand service is on its way in or out.
func (c *TaxiController) stairsDriving() bool {
	for _, x := range c.standServices() {
		if x.v != nil && x.s.attached && !x.v.Done() && !x.v.Fuelling() {
			return true
		}
	}
	return false
}

// removeStairs takes the stand services away (cancel, failure).
func (c *TaxiController) removeStairs() {
	for _, x := range c.standServices() {
		if x.v != nil {
			c.note(x.s.what, x.v.Remove())
		}
		c.give(x.s.kind)
	}
}
