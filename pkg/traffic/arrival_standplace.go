package traffic

import "math"

// SetStandPlace moves the arrival's stop on its stand to nose (NoseOffset)
// and side (StandSide), until it touches down: the taxi-in planned again to
// the same stand, as ChangeStand does. changed false when the stop is
// already there (within 5 cm). An arrival's place is worked out when it
// spawns, 20–40 minutes before it parks; at a jetway gate it depends on the
// helper that will dock it, not known before the jetways are ready (live,
// LKPR C22: CSA1894 spawned 1 min before the gates filled, stopped as with
// no jetway, 9 m short of the door the "B748 D-A320" helper's jetway went
// to).
func (c *ArrivalController) SetStandPlace(nose, side float64) (changed bool, err error) {
	c.mu.Lock()
	if c.plan == nil || c.state > ArrivalLanding {
		c.mu.Unlock()
		return false, ErrStandFixed
	}
	if math.Abs(c.req.NoseOffset-nose) < 0.05 && math.Abs(c.req.StandSide-side) < 0.05 {
		c.mu.Unlock()
		return false, nil
	}
	oldNose, oldSide, parking := c.req.NoseOffset, c.req.StandSide, c.req.Parking
	c.req.NoseOffset, c.req.StandSide = nose, side
	c.mu.Unlock()
	if err := c.ChangeStand(parking); err != nil {
		c.mu.Lock()
		c.req.NoseOffset, c.req.StandSide = oldNose, oldSide
		c.mu.Unlock()
		return false, err
	}
	return true, nil
}
