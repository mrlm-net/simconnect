package traffic

import (
	"errors"
	"math"
	"time"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// The follow-me car of an injected arrival (#890, ArrivalRequest.FollowMe):
// sent once the aircraft is down, from the fleet, to wait
// FollowMeLeadMeters past its vacate stop; the aircraft keeps behind it
// (groundDrive.lead) until it pulls aside before the stand, and the
// aircraft's frames drive it home after it has parked.

// updateFollowMe sends the car when it is due and drives it, after the
// aircraft has stepped to pose.
func (c *ArrivalController) updateFollowMe(pose GroundPose) {
	if c.req.FollowMe == nil || c.mover == nil {
		return
	}
	path := c.mover.Path()
	if !c.fmSent {
		switch c.state {
		case ArrivalRollout, ArrivalVacating, ArrivalAwaitingTaxi:
		default:
			return // not down yet, or taxiing already: too late for it
		}
		c.fmSent = true
		meet := c.vacateDist + FollowMeLeadMeters
		if meet+FollowMeLeaveMeters+FollowMeGapMeters > path.Length() || !c.takeFollowMe() {
			return // a short taxi-in, or none free
		}
		c.fm = c.req.FollowMe
		if f, ok := c.fm.(trafficAware); ok && c.picture != nil {
			f.SetTraffic(c.picture, c.objectID, c.now)
		}
		c.note("follow-me", nil)
		if err := c.fm.Attach(path, meet); err != nil {
			c.note("follow-me", err)
			c.dropFollowMe()
			return
		}
		c.lead = c.fm.Lead
	}
	if c.fm == nil {
		return
	}
	c.note("follow-me", c.fm.Update(path, pose.Distance, c.frameDt))
	// Held by a car that does not come: on without it.
	if at, ok := c.fm.Lead(); ok && pose.Stopped && pose.Distance >= at-2 && c.state == ArrivalTaxiing {
		if c.fmStopped.IsZero() {
			c.fmStopped = c.now()
		}
		if c.now().Sub(c.fmStopped) >= FollowMeWaitMax {
			c.note("follow-me did not come: removed", errors.New("follow-me timeout"))
			c.dropFollowMe()
			return
		}
	} else {
		c.fmStopped = time.Time{}
	}
	if c.fm.Done() {
		c.lead = nil
		c.giveFollowMe()
		c.fm = nil
	}
}

// followMeDriving reports that the car is on its way: the frames come at
// the full rate meanwhile.
func (c *ArrivalController) followMeDriving() bool {
	f, ok := c.fm.(interface{ Driving() bool })
	return ok && f.Driving()
}

// handleParked drives the car home on the parked aircraft's frames (#890),
// then stops the frames.
func (c *ArrivalController) handleParked(msg engine.Message) bool {
	if c.fm.Handle(msg) {
		return true
	}
	if types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA {
		return false
	}
	m := msg.AsSimObjectData()
	if uint32(m.DwRequestID) != c.reqBase+arrReqMonitor || uint32(m.DwObjectID) != c.objectID {
		return false
	}
	now := c.now()
	dt := math.Max(0, math.Min(now.Sub(c.lastStep).Seconds(), MaxFrameStepSeconds))
	c.lastStep = now
	var path *GroundPath
	s := 0.0
	if c.mover != nil {
		path, s = c.mover.Path(), c.mover.Pose().Distance
	}
	c.note("follow-me", c.fm.Update(path, s, dt))
	if c.fm.Done() {
		c.giveFollowMe()
		c.fm, c.lead = nil, nil
		c.stopMonitor()
	}
	return true
}

// dropFollowMe takes the car away at once.
func (c *ArrivalController) dropFollowMe() {
	if c.fm != nil {
		c.note("follow-me", c.fm.Remove())
	}
	c.giveFollowMe()
	c.fm, c.lead = nil, nil
}

func (c *ArrivalController) takeFollowMe() bool {
	return c.services == nil || c.services.Take(VehicleFollowMe, c.req.Tail)
}

func (c *ArrivalController) giveFollowMe() {
	if c.services != nil {
		c.services.Give(VehicleFollowMe, c.req.Tail)
	}
}
