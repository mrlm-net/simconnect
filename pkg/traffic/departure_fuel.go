//go:build windows
// +build windows

package traffic

import (
	"errors"
	"fmt"
	"time"
)

// Refuelling on the stand (#582): TaxiRequest.Fuel comes FuelStartDelay
// after the departure is waiting on its stand, when there is at least
// FuelMinService before it must be off the wing (fuelLeaveBy), refuels for
// FuelServiceTime (twice for a widebody) and leaves; the pushback, or the
// taxi out of a self-manoeuvring stand, waits until it is off the wing.

// widebodySpanMeters: an aircraft this wide (ICAO code E and up) takes
// twice as long to refuel.
const widebodySpanMeters = 52.0

// fuelLeaveBy is when the fuel vehicle must be off the wing:
// FuelClearMargin before the tug comes (TugLeadTime before the crew asks
// for the push).
func (c *TaxiController) fuelLeaveBy() time.Time {
	if c.gateAt.IsZero() {
		return time.Time{}
	}
	return c.gateAt.Add(-TugLeadTime - FuelClearMargin)
}

// updateFuel sends the fuel vehicle while the aircraft waits on its stand
// and drives it until it has left.
func (c *TaxiController) updateFuel(dt float64) {
	f := c.req.Fuel
	if f == nil || f.Done() {
		return
	}
	now := c.now()
	stand := c.req.Graph.Layout.Parking[c.req.Parking]
	pose := GroundPose{Position: StandPoint(stand, c.req.NoseOffset), Heading: stand.Heading}
	if c.mover != nil {
		pose = c.mover.Pose()
	}
	if !c.fuelAttached {
		if c.state != TaxiAwaitingPushback || c.pushCleared || !c.pushAt.IsZero() {
			return
		}
		if c.fuelWaitFrom.IsZero() {
			c.fuelWaitFrom = now
		}
		if now.Before(c.fuelWaitFrom.Add(FuelStartDelay)) || c.fuelLeaveBy().Sub(now) < FuelMinService {
			return
		}
		c.fuelAttached = true
		c.giveTraffic(f)
		c.note("fuel truck", nil)
		if err := f.Attach(pose); err != nil {
			c.fuelErr(err)
			c.fuelErr(f.Remove())
		}
		return
	}
	if c.fuelUntil.IsZero() && f.Fuelling() {
		d := FuelServiceTime
		if c.profile().SpanMeters >= widebodySpanMeters {
			d *= 2
		}
		d = time.Duration(float64(d) * (1 + DwellJitter*(2*c.rng.Float64()-1)))
		c.fuelUntil = now.Add(d)
		if by := c.fuelLeaveBy(); by.Before(c.fuelUntil) {
			c.fuelUntil = by
		}
	}
	leave := c.state != TaxiAwaitingPushback || c.pushCleared || !c.pushAt.IsZero() ||
		!now.Before(c.fuelLeaveBy()) || !c.fuelUntil.IsZero() && !now.Before(c.fuelUntil)
	c.fuelErr(f.Update(pose, leave, dt))
}

// fuelClear reports that no fuel vehicle is at the wing: none, never sent,
// gone, or driving off. One still there after FuelClearTimeout is removed.
func (c *TaxiController) fuelClear() bool {
	f := c.req.Fuel
	if f == nil || !c.fuelAttached || f.Done() || f.Clear() {
		return true
	}
	now := c.now()
	if c.fuelClearFrom.IsZero() {
		c.fuelClearFrom = now
	}
	if now.Sub(c.fuelClearFrom) < FuelClearTimeout {
		return false
	}
	c.fuelErr(errors.New("the fuel truck did not leave: removed"))
	c.fuelErr(f.Remove())
	return true
}

// giveTraffic lets a service vehicle give way to the aircraft around
// (trafficAware): the departure's ground picture, without itself.
func (c *TaxiController) giveTraffic(v any) {
	if a, ok := v.(trafficAware); ok && c.picture != nil {
		a.SetTraffic(c.picture, c.objectID, c.now)
	}
}

// fuelDriving reports that the fuel vehicle is on its way in or out: the
// aircraft's frames move it, so they come at the full rate meanwhile.
func (c *TaxiController) fuelDriving() bool {
	f := c.req.Fuel
	return f != nil && c.fuelAttached && !f.Done() && !f.Fuelling()
}

// removeFuel takes the fuel vehicle away (cancel, failure).
func (c *TaxiController) removeFuel() {
	if c.req.Fuel != nil {
		c.note("fuel truck", c.req.Fuel.Remove())
	}
}

// fuelErr reports a fuel vehicle error as an event; the departure goes on
// without it.
func (c *TaxiController) fuelErr(err error) {
	if err != nil {
		c.emit(fmt.Errorf("traffic: fuel truck: %w", err), true)
	}
}
