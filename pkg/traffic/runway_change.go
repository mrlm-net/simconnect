package traffic

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Runway change (#456): when the runway in use changes, ATC re-plans the
// departures not yet lined up — the new runway, its SID, a new taxi route
// from where each aircraft is. ChangeRunway is that re-plan for one
// injected departure; the caller (a tower) decides who gets it and says
// the re-clearance.

// ChangeEntry has the departure take its runway from entry instead (the
// entry taxiway, "" full length): planned again as for a runway change
// (ChangeRunway), on the stand or from where it is. An entry leaving less
// runway than the type needs is ErrEntryTooShort.
func (c *TaxiController) ChangeEntry(entry string) error {
	c.mu.Lock()
	req := c.req
	c.mu.Unlock()
	if entry != "" {
		entries, err := req.Graph.RunwayEntries(req.Runway)
		if err != nil {
			return err
		}
		found := false
		for _, e := range entries {
			if !strings.EqualFold(e.Taxiway, entry) {
				continue
			}
			found = true
			req.Entry = e.Taxiway
			last := e.Node
			if e.HoldShort >= 0 {
				last = e.HoldShort
			}
			if err := entryLongEnough(req, &airport.Route{Nodes: []airport.NodeID{last}}); err != nil {
				return err
			}
			entry = e.Taxiway
			break
		}
		if !found {
			return fmt.Errorf("%w: no entry %q onto runway %s", ErrBadTaxiRequest, entry, req.Runway)
		}
	}
	return c.ChangeRunway(req.Runway, entry, nil)
}

// ChangeRunway re-plans the departure for runway (entry an intersection,
// "" full length) and departure, its new SID and route (nil keeps the
// request's). On the stand the route and the pushback are planned again;
// pushed back or waiting for the taxi, the taxi-out is planned from where
// the aircraft stands when it starts; taxiing, from where it is, at the
// speed it has; holding short, from there, and it waits for a new taxi
// clearance. Lining up or later it is ErrTooLate: the tower lets it go or
// cancels its take-off.
func (c *TaxiController) ChangeRunway(runway, entry string, departure []airport.NavPoint) error {
	return c.changeRoute(runway, entry, departure, nil)
}

// TaxiVia re-plans a departure's taxi-out to its runway through via, graph
// nodes picked on the map in order (#443), as a runway change re-plans it:
// on the stand all of it, pushed back or taxiing from where it is.
func (c *TaxiController) TaxiVia(via []airport.NodeID) error {
	if len(via) == 0 {
		return fmt.Errorf("%w: no via point", ErrBadTaxiRequest)
	}
	c.mu.Lock()
	runway, entry := c.req.Runway, c.req.Entry
	c.mu.Unlock()
	return c.changeRoute(runway, entry, nil, via)
}

// changeRoute is ChangeRunway, through via when set (TaxiVia).
func (c *TaxiController) changeRoute(runway, entry string, departure []airport.NavPoint, via []airport.NodeID) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.inj == nil {
		return fmt.Errorf("%w: only an injected departure re-plans its runway", ErrNotApplicable)
	}
	g := c.req.Graph
	rwy, end, ok := g.Layout.RunwayEnd(runway)
	if !ok {
		return fmt.Errorf("%w: %q at %s", airport.ErrUnknownRunway, runway, g.Layout.ICAO)
	}
	switch c.state {
	case TaxiLiningUp, TaxiLinedUp, TaxiDeparting, TaxiComplete, TaxiFailed, TaxiCancelled:
		return ErrTooLate
	}
	req := c.req
	req.Runway, req.Entry = runway, entry
	if departure != nil {
		req.Departure = departure
	}
	c.hereVia = via
	if via != nil {
		req.Options.Via, req.Options.Taxiways = append([]airport.NodeID(nil), via...), nil
	}
	switch c.state {
	case TaxiIdle, TaxiSpawning, TaxiAwaitingPushback:
		// Still on the stand: plan it all again, the push included.
		if err := c.planFromStand(req, rwy, end); err != nil {
			return err
		}
		c.planPushback()
	case TaxiAwaitingTaxi:
		// Waiting for the taxi on a stand it leaves without a push (no
		// taxiway under it to plan from; live, SPDPS PC12 on N63 asking for
		// B): the taxi-out from the stand, as before the push.
		if c.mover != nil && c.onStand() {
			if err := c.planFromStand(req, rwy, end); err != nil {
				return err
			}
			c.reroute, c.fromHere = false, false
			break
		}
		fallthrough
	case TaxiPushback:
		// The taxi-out, from where the aircraft stands when it starts;
		// standing already (waiting for the taxi), planned now as well, so
		// the taxi clearance names the new route (a crew's intersection
		// request, #621).
		c.req, c.runway, c.end, c.runwayLength = req, rwy, end, rwy.Length
		c.reroute = true
		if c.state == TaxiAwaitingTaxi && c.mover != nil {
			c.routeFromHere() // else planned when it starts, as before
		}
	case TaxiTaxiing, TaxiHoldingShort:
		c.req, c.runway, c.end, c.runwayLength = req, rwy, end, rwy.Length
		if err := c.routeFromHere(); err != nil {
			return err
		}
		if c.state == TaxiHoldingShort {
			// Held short (of the old runway, or a crossing): a new taxi
			// clearance first.
			c.taxiCleared, c.moveAt = false, time.Time{}
			c.setState(TaxiAwaitingTaxi, nil)
			c.reroute = true
			break
		}
		if err := c.startTaxiOut(); err != nil {
			return err
		}
	}
	c.lineUpCleared, c.takeoffCleared, c.hasLimit, c.hasPendingLimit = false, false, false, false
	c.note("runway changed to "+runway, nil)
	c.emit(nil, true)
	return nil
}

// planFromStand plans the departure from its stand again for req (rwy, end
// its runway): the route, and how it leaves the stand. c.mu is held.
func (c *TaxiController) planFromStand(req TaxiRequest, rwy airport.Runway, end airport.RunwayEnd) error {
	route, err := req.Graph.RouteToRunwayEntry(req.Parking, req.Runway, req.Entry, req.Options)
	if err != nil {
		return err
	}
	if err := entryLongEnough(req, route); err != nil {
		return err
	}
	c.req, c.runway, c.end, c.runwayLength = req, rwy, end, rwy.Length
	c.route, c.origRoute, c.pushJunction, c.pushPlanned = route, nil, 1, nil
	c.faceOut, c.powerOut = c.standFacesOut(), nil
	if c.req.PowerOut && !c.faceOut {
		if loop, ok := c.planPowerOut(); ok {
			c.powerOut, c.faceOut = loop, true
		}
	}
	c.track = newRouteTracker(c.route)
	return nil
}

// onStand: the aircraft stands where it was parked (within onStandMeters),
// not pushed back. c.mu is held.
func (c *TaxiController) onStand() bool {
	stand := c.req.Graph.Layout.Parking[c.req.Parking]
	return localDist(c.mover.Pose().Position, StandPoint(stand, c.req.NoseOffset)) <= onStandMeters
}

const onStandMeters = 5.0

// routeFromHere plans the route to the request's runway from where the
// aircraft is: from the taxiway edge under its nose gear that runs its way
// (within routeHereDeg), not turning back.
func (c *TaxiController) routeFromHere() error {
	g, prof := c.req.Graph, c.profile()
	pose := c.mover.Pose()
	nose := NoseGear(pose.Position, pose.Heading, prof)
	from, to, best := airport.NodeID(-1), airport.NodeID(-1), math.Inf(1)
	for a := range g.Adj {
		pa := g.Nodes[a].Position
		if g.Nodes[a].Kind == airport.NodeParking || localDist(pa, nose) > 400 {
			continue
		}
		for _, e := range g.Adj[a] {
			if g.Nodes[e.To].Kind == airport.NodeParking {
				continue
			}
			pb := g.Nodes[e.To].Position
			h, l := localBearing(pa, pb), localDist(pa, pb)
			if math.Abs(headingDiff(h, pose.Heading)) > routeHereDeg {
				continue
			}
			along := alongHeading(pa, h, nose)
			if along < -5 || along > l {
				continue
			}
			d := localDist(nose, offsetHeading(pa, h, math.Max(0, along)))
			if d < best {
				from, to, best = airport.NodeID(a), e.To, d
			}
		}
	}
	if from < 0 || best > routeHereMeters {
		return fmt.Errorf("%w: not on a taxiway to re-plan from", airport.ErrNoRoute)
	}
	opts := c.req.Options
	opts.Via, opts.Taxiways = c.hereVia, nil
	r, err := g.RouteToRunwayFrom(to, from, c.req.Runway, c.req.Entry, opts)
	if err != nil {
		return err
	}
	full, err := g.RouteFromNodes(append([]airport.NodeID{from}, r.Nodes...))
	if err != nil {
		return err
	}
	full.Runway, full.RunwayEnd, full.Entry, full.HoldShort, full.Tight = r.Runway, r.RunwayEnd, r.Entry, r.HoldShort, r.Tight
	c.route, c.pushJunction, c.fromHere = full, 0, true
	c.track = newRouteTracker(c.route)
	return nil
}

// A route re-planned from where the aircraft is starts on a taxiway edge
// within routeHereMeters of the nose gear, running within routeHereDeg of
// its heading.
const (
	routeHereMeters = 30.0
	routeHereDeg    = 60.0
)

// ChangeRunway re-plans an arrival for runway (#456): one still on its way
// in — spawned, or flying its STAR and approach (procedure, for the new
// runway) by MSFS AI — gets the new arrival plan (exit, taxi-in) and flies
// the new procedure from where it is. On the injected final or later it is
// ErrTooLate: it lands on the old runway, or the tower sends it around.
// missed is the new runway's missed approach (nil: a circuit on a
// go-around).
func (c *ArrivalController) ChangeRunway(runway string, procedure, missed []airport.NavPoint) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.req.InjectApproach || len(procedure) == 0 || c.plan == nil {
		return fmt.Errorf("%w: an arrival re-plans its runway with a procedure to fly (InjectApproach)", ErrNotApplicable)
	}
	if c.state > ArrivalApproaching || c.approach != nil {
		return ErrTooLate
	}
	g, req := c.req.Graph, c.req
	plan, err := PlanArrival(g, runway, req.Parking, ArrivalOptions{
		SpawnNm: req.SpawnNm, Route: req.Options, GroundAGL: req.GroundAGL, NoseOffset: req.NoseOffset,
		TouchdownKts: approachProfileOf(req).TouchdownKts, BrakeDecel: req.Rollout.BrakeDecel,
	})
	if err != nil {
		return err
	}
	join := math.Max(plan.SpawnNm, ProcedureJoinNm) * 1852
	jp := NewApproachMover(plan.End.Threshold, plan.End.Heading, join, approachProfileOf(req)).Pose()
	proc, err := PlanArrivalProcedure(procedure, plan.End, join, g.Layout.Altitude/0.3048+jp.HeightFt)
	if err != nil {
		return err
	}
	// From where it is now (the spawn point while it is still appearing).
	from := airport.LatLon{Lat: proc.Spawn.Latitude, Lon: proc.Spawn.Longitude}
	if c.last.Position.Lat != 0 || c.last.Position.Lon != 0 {
		from = c.last.Position
	}
	c.setCorners(proc.Waypoints, proc.Names)
	proc.Waypoints = roundedChain(from, proc.Waypoints, MaxBankDeg(*req.Aircraft))
	plan.Spawn = c.plan.Spawn
	req.Runway, req.Procedure, req.MissedApproach, req.Exit = runway, procedure, missed, nil
	c.req, c.plan, c.proc, c.procNext, c.circuit = req, plan, proc, -1, false
	c.track = newRouteTracker(plan.Route)
	c.exitAlong = c.track.cum[len(plan.Exit.Path)-1]
	c.vacateAlong = c.track.cum[plan.VacateIndex]
	if c.flyingProc && c.objectID != 0 {
		if err := c.fleet.SetWaypoints(c.objectID, c.defBase+arrDefWaypoints, c.proc.Waypoints); err != nil {
			return err
		}
	}
	c.note("runway changed to "+runway, nil)
	c.emit(nil, true)
	return nil
}
