package traffic

// AddStandServices gives a departure still waiting on its stand the stand
// services of a remote stand it was started without: stairs, a GPU (nil
// none) and buses, driven as if given at the start (StandServiceStartDelay
// on). For a gate whose jetway does not come (live, LKPR C21 2026-11-08: an
// A321 waited for a jetway that never moved). False when it no longer
// waits, its push is under way, it has stairs already, or stairs is nil.
func (c *TaxiController) AddStandServices(stairs, gpu FuelService, buses []FuelService) bool {
	if stairs == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state != TaxiAwaitingPushback || c.pushCleared || !c.pushAt.IsZero() || c.req.Stairs != nil {
		return false
	}
	c.req.Stairs = stairs
	if c.req.GPU == nil {
		c.req.GPU = gpu
	}
	c.req.Buses = buses
	return true
}
