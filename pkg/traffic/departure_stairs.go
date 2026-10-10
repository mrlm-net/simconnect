package traffic

import (
	"errors"
	"time"
)

// Stand services (#831, #832, #887): stairs at the front left door, a ground
// power unit at the nose and passenger buses beside the stairs, on a remote
// stand. Each is sent once the aircraft waits on its stand
// (StandServiceStartDelay), stays until StandServiceClearMargin before the
// tug comes (TugLeadTime before the push) or the push is cleared, and the
// push waits for it to leave. With none free in the fleet, none. The buses
// come for the boarding only (BusBoardingTime, after the stairs) and leave
// before the stairs (BusLeaveBeforeStairs). The GPU stays longer: until
// the APU is on, APUStartBeforeTug before the tug comes (#1025, the user:
// it went long before the push).
const (
	APUStartBeforeTug        = time.Minute
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

// standService is a departure's stairs, GPU or bus and where it is.
type standService struct {
	kind       VehicleKind
	what       string // "stairs", "GPU", "bus": for errors
	owner      string // who holds it in the fleet ("": the call sign)
	attached   bool
	attachedAt time.Time
	waitFrom   time.Time // first frame waiting on the stand
	clearFrom  time.Time // the push first waited for it to leave
	// window: it comes no earlier than this long before it must leave (0:
	// any time); after: only once that one has been sent afterDelay ago;
	// behind: only once that vehicle stands at the aircraft (or is gone);
	// leaveEarly: it leaves this long before the others.
	window     time.Duration
	after      *standService
	afterDelay time.Duration
	behind     FuelService
	leaveEarly time.Duration
	// stay: it leaves this long after it got to the aircraft (arrivedAt; 0:
	// it stays); home: it comes only once that vehicle has left for good.
	stay      time.Duration
	arrivedAt time.Time
	home      FuelService
	// margin: it leaves this long before the tug comes (0:
	// StandServiceClearMargin).
	margin time.Duration
}

type standVehicle struct {
	v FuelService
	s *standService
}

// standServices are the departure's stand services with their vehicles.
func (c *TaxiController) standServices() []standVehicle {
	c.stairsSvc.kind, c.stairsSvc.what = VehicleStairs, "stairs"
	c.gpuSvc.kind, c.gpuSvc.what, c.gpuSvc.margin = VehicleGPU, "GPU", APUStartBeforeTug
	out := []standVehicle{{c.req.Stairs, &c.stairsSvc}, {c.req.GPU, &c.gpuSvc}}
	if c.req.Stairs == nil {
		return out // buses only with stairs
	}
	if len(c.deboardSvc) != len(c.req.Deboard) {
		c.deboardSvc = make([]standService, len(c.req.Deboard))
	}
	for i, b := range c.req.Deboard {
		s := &c.deboardSvc[i]
		s.kind, s.what, s.owner = VehicleBus, "bus", busOwner(c.req.Tail, i)
		s.after, s.afterDelay, s.leaveEarly, s.stay = &c.stairsSvc, BusAfterStairs, BusLeaveBeforeStairs, BusDeboardTime
		if i > 0 {
			s.behind = c.req.Deboard[i-1] // drives past its spot: in once it is there
		}
		out = append(out, standVehicle{b, s})
	}
	if len(c.busSvc) != len(c.req.Buses) {
		c.busSvc = make([]standService, len(c.req.Buses))
	}
	for i, b := range c.req.Buses {
		s := &c.busSvc[i]
		s.kind, s.what, s.owner = VehicleBus, "bus", busOwner(c.req.Tail, i)
		s.window, s.after, s.afterDelay, s.leaveEarly = BusBoardingTime, &c.stairsSvc, BusAfterStairs, BusLeaveBeforeStairs
		if i > 0 {
			s.behind = c.req.Buses[i-1]
		}
		if i < len(c.req.Deboard) {
			s.home = c.req.Deboard[i] // its spot and its request ID
		}
		out = append(out, standVehicle{b, s})
	}
	return out
}

// updateStairs drives the stand services (stairs, GPU, buses) each frame.
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
		c.giveService(st)
		return
	}
	now := c.now()
	stand := c.req.Graph.Layout.Parking[c.req.Parking]
	pose := GroundPose{Position: StandPoint(stand, c.req.NoseOffset), Heading: stand.Heading}
	if c.mover != nil {
		pose = c.mover.Pose()
	}
	margin := StandServiceClearMargin
	if st.margin > 0 {
		margin = st.margin
	}
	leaveBy := c.gateAt.Add(-TugLeadTime - margin - st.leaveEarly)
	if !st.attached {
		if c.state != TaxiAwaitingPushback || c.pushCleared || !c.pushAt.IsZero() || !now.Before(leaveBy) {
			return
		}
		if st.waitFrom.IsZero() {
			st.waitFrom = now
		}
		if now.Before(st.waitFrom.Add(StandServiceStartDelay)) {
			return
		}
		if st.window > 0 && now.Before(leaveBy.Add(-st.window)) {
			return
		}
		if st.after != nil && (!st.after.attached || now.Before(st.after.attachedAt.Add(st.afterDelay))) {
			return
		}
		if st.behind != nil && !st.behind.Fuelling() && !st.behind.Done() {
			return
		}
		if st.home != nil && !st.home.Done() {
			return
		}
		if !c.takeService(st) {
			return
		}
		st.attached, st.attachedAt = true, now
		c.giveTraffic(v)
		c.note(st.what, nil)
		if err := v.Attach(pose); err != nil {
			c.fuelErr(err)
			c.fuelErr(v.Remove())
		}
		return
	}
	if st.arrivedAt.IsZero() && v.Fuelling() {
		st.arrivedAt = now
	}
	leave := c.state != TaxiAwaitingPushback || c.pushCleared || !c.pushAt.IsZero() || !now.Before(leaveBy) ||
		st.stay > 0 && !st.arrivedAt.IsZero() && !now.Before(st.arrivedAt.Add(st.stay))
	c.fuelErr(v.Update(pose, leave, dt))
}

// takeService reserves the service's vehicle from the fleet.
func (c *TaxiController) takeService(st *standService) bool {
	if st.owner == "" {
		return c.take(st.kind)
	}
	return c.services == nil || c.services.Take(st.kind, st.owner)
}

// giveService returns the service's vehicle to the fleet.
func (c *TaxiController) giveService(st *standService) {
	if st.owner == "" {
		c.give(st.kind)
	} else if c.services != nil {
		c.services.Give(st.kind, st.owner)
	}
}

// giveBuses returns all the departure's buses to the fleet.
func (c *TaxiController) giveBuses() {
	if c.services == nil {
		return
	}
	for i := range max(len(c.req.Buses), len(c.req.Deboard)) {
		c.services.Give(VehicleBus, busOwner(c.req.Tail, i))
	}
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
		c.giveService(x.s)
	}
}
