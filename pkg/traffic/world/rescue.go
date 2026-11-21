package world

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Rescue crews (the emergency add-on, part 1). An aircraft with priority
// (an emergency: one of ours declaring it, or the host's for the player)
// coming in to an airport of ours gets the airport's fire and rescue: as
// it comes within rescueDispatchNM, fire trucks drive from the fire
// station (the nearest vehicle depot) by the roads and taxiways to standby
// at the holding positions of its landing
// runway and wait; once it is down and slow (#1597) they come onto the
// runway by the exit or entry nearest it and drive to it (they wait at
// standby through its roll and a go-around), stay a while by where
// it stops, then drive back and go. A medical emergency gets
// an ambulance instead, following it in to its stand and staying there
// longer. The trucks are SimObjectRescue, placed every frame.

const (
	// rescueDispatchNM: the trucks leave as the aircraft comes this close.
	rescueDispatchNM = 20.0
	// rescueFireTrucks: fire trucks sent for anything but a medical.
	rescueFireTrucks = 2
	// rescueStay, rescueStayMedical: how long they stay with it after it
	// has stopped.
	rescueStay        = time.Minute
	rescueStayMedical = 5 * time.Minute
	// rescueStoppedKts: at or below this it has stopped.
	rescueStoppedKts = 1.0
	// rescueLostAfter: not seen this long, it is gone: the crew goes home.
	rescueLostAfter = 10 * time.Second
	// rescueTrail: each truck after the first follows this much farther
	// behind.
	rescueTrail = 40.0
)

// Fire and rescue vehicle titles: GSX's fire trucks first, MSFS's own;
// MSFS's ambulances.
var (
	rescueFirePrefixes   = []string{"FSDT_Firetruck_"}
	rescueStockFire      = []string{"Truck Fire Airport Medium", "Truck Fire Long 02", "Truck Fire Short", "Truck Fire Short 02", "ASO_Firetruck02"}
	rescueStockAmbulance = []string{"Truck Utility Europe Medic Box", "Truck Utility Europe Medic Flush"}
)

type rescueState int

const (
	rescueStandby   rescueState = iota // on its way to standby, or waiting there
	rescueFollowing                    // after the aircraft, it down and slow
	rescueHome                         // driving back
	rescueDown                         // it down and rolling: still at standby (#1597)
)

// rescueRunwayHoldNM: with the aircraft this close to the runway (or on
// it), vehicles on their way to standby stop short of the runway.
const rescueRunwayHoldNM = 5.0

// rescueCrew is the vehicles sent for one aircraft.
type rescueCrew struct {
	tail, icao, runway string
	medical            bool
	defBase            uint32
	trucks             []*traffic.SimObjectRescue
	state              rescueState
	graph              *airport.Graph
	// ahead: they came onto the runway ahead of the aircraft (stopped),
	// and wait in front of it.
	ahead     bool
	enteredAt time.Time               // since when they drive to it
	target    traffic.TrackedAircraft // the aircraft as last seen
	stoppedAt time.Time               // since when it has stood, zero moving
	frameAt   time.Time
	// kts is its speed: the simulator's, or from its positions when more
	// (AI on the ground report 0 kt while moving).
	kts float64
}

// observe takes the aircraft as seen now: its speed from how far it moved
// since it was last seen.
func (c *rescueCrew) observe(a traffic.TrackedAircraft) {
	prev := c.target
	c.target = a
	if dt := a.Updated.Sub(prev.Updated).Seconds(); prev.Tail == a.Tail && dt >= 0.5 {
		moved := calc.HaversineMeters(prev.Position.Lat, prev.Position.Lon, a.Position.Lat, a.Position.Lon) / dt * 3600 / 1852
		c.kts = math.Max(a.GroundKts, moved)
	} else if dt < 0 || prev.Tail != a.Tail {
		c.kts = a.GroundKts
	}
}

// nearRunway: a is within rescueRunwayHoldNM of the crew's runway.
func (c *rescueCrew) nearRunway(a traffic.TrackedAircraft) bool {
	if c.graph == nil {
		return false
	}
	_, end, ok := c.graph.Layout.RunwayEnd(c.runway)
	return ok && calc.HaversineNM(a.Position.Lat, a.Position.Lon, end.Threshold.Lat, end.Threshold.Lon) <= rescueRunwayHoldNM
}

// holdOffRunway keeps the vehicles on their way off the crew's runway, or
// lets them go on.
func (c *rescueCrew) holdOffRunway(hold bool) {
	var rwy *airport.Runway
	if c.graph != nil && hold {
		if r, _, ok := c.graph.Layout.RunwayEnd(c.runway); ok {
			rwy = &r
		}
	}
	for _, t := range c.trucks {
		t.HoldOffRunway(rwy)
	}
}

// enter sends the vehicles to the aircraft, now slow: by the exit or entry
// nearest it (not ahead of it while it moves), one after another (#1597).
func (c *rescueCrew) enter() {
	c.holdOffRunway(false)
	if c.graph == nil {
		return
	}
	way, ok := traffic.RescueAccess(c.graph, c.runway, c.target.Position, c.kts > rescueStoppedKts)
	if !ok {
		return
	}
	c.ahead = way.Ahead
	for _, t := range c.trucks {
		_ = t.Enter(way)
	}
}

// touchedDown: a is on the runway, its wheels on (not in the flare, low).
func touchedDown(a traffic.TrackedAircraft) bool {
	return a.OnGround || a.AGLFt > 0 && a.AGLFt < 3
}

// track is where the crew's vehicles are, for the map.
func (c *rescueCrew) track() []tugView {
	var out []tugView
	for _, t := range c.trucks {
		if p, route, ok := t.Track(); ok {
			out = append(out, tugView{Position: p.Position, Heading: p.Heading, Route: route})
		}
	}
	return out
}

// rescueCrews are the crews out.
type rescueCrews struct {
	mu   sync.Mutex
	by   map[string]*rescueCrew // by the aircraft's name
	sent map[string]bool        // names already given a crew: one each
}

// rescueTitles are the vehicles of a crew: an ambulance for a medical, else
// n fire trucks (GSX's by preference); none when the
// simulator offers none.
func (cc *controlCenter) rescueTitles(medical bool, n int) []string {
	cc.mu.Lock()
	titles := cc.groundTitles
	cc.mu.Unlock()
	has := func(t string) bool {
		for _, x := range titles {
			if x == t {
				return true
			}
		}
		return false
	}
	pick := func(list []string, n int) []string {
		list = traffic.UsableModels(list)
		var out []string
		for i := 0; i < n && len(list) > 0; i++ {
			out = append(out, list[i%len(list)])
		}
		return out
	}
	if medical {
		var amb []string
		for _, t := range rescueStockAmbulance {
			if has(t) {
				amb = append(amb, t)
			}
		}
		return pick(amb, 1)
	}
	var gsx, stock []string
	for _, t := range titles {
		for _, p := range rescueFirePrefixes {
			if strings.HasPrefix(t, p) {
				gsx = append(gsx, t)
			}
		}
	}
	for _, t := range rescueStockFire {
		if has(t) {
			stock = append(stock, t)
		}
	}
	if len(gsx) > 0 {
		return pick(gsx, n)
	}
	return pick(stock, n)
}

// rescueName is a's name among those with priority (as priorityClose).
func (cc *controlCenter) rescueName(a traffic.TrackedAircraft, prio map[string]bool) string {
	if !a.User {
		return a.Tail
	}
	if p, ok := cc.core.playerLanding(); ok && p.Callsign != "" && prio[p.Callsign] {
		return p.Callsign
	}
	return playerTail
}

// rescueRunway is the runway end an aircraft with priority lands on: its
// clearance's, else the arrival runway in use.
func (cc *controlCenter) rescueRunway(name string, user bool, icao string) string {
	if user {
		if p, ok := cc.core.playerLanding(); ok && p.Runway != "" && (p.ICAO == "" || p.ICAO == icao) {
			return p.Runway
		}
	} else if it := cc.byTail(name); it != nil {
		it.mu.Lock()
		rwy := it.view.Runway
		it.mu.Unlock()
		if rwy != "" {
			return rwy
		}
	}
	g, err := cc.graph(icao)
	if err != nil {
		return ""
	}
	return cc.runwayFor(g, true, -1)
}

// rescueTick (each second) sends crews to aircraft with priority coming
// in, starts them following once it is down and sends them home after.
func (cc *controlCenter) rescueTick(now time.Time) {
	prio := cc.core.prio.all()
	cc.rescue.mu.Lock()
	idle := len(prio) == 0 && len(cc.rescue.by) == 0
	cc.rescue.mu.Unlock()
	if idle {
		return
	}
	cc.rescueStep(now, prio, cc.world.Aircraft())
}

// rescueStep is rescueTick on the picture's aircraft.
func (cc *controlCenter) rescueStep(now time.Time, prio map[string]bool, aircraft []traffic.TrackedAircraft) {
	seen := map[string]traffic.TrackedAircraft{}
	for _, a := range aircraft {
		name := cc.rescueName(a, prio)
		seen[name] = a
		if !prio[name] || a.Grounded() || a.Airport == "" {
			continue
		}
		cc.rescue.mu.Lock()
		sent := cc.rescue.sent[name]
		cc.rescue.mu.Unlock()
		if sent {
			continue
		}
		g, err := cc.graph(a.Airport)
		if err != nil {
			continue
		}
		l := g.Layout
		if calc.HaversineNM(a.Position.Lat, a.Position.Lon, l.Latitude, l.Longitude) > rescueDispatchNM {
			continue
		}
		cc.sendRescue(name, a, cc.core.prio.kind(name) == traffic.EmergencyMedical)
	}
	cc.rescue.mu.Lock()
	defer cc.rescue.mu.Unlock()
	for name, c := range cc.rescue.by {
		a, ok := seen[name]
		if ok {
			c.observe(a)
		}
		done := true
		for _, t := range c.trucks {
			done = done && t.Done()
		}
		switch {
		case done:
			cc.ids.Release(c.defBase)
			delete(cc.rescue.by, name)
			cc.log.printf("%-6s rescue: back at the station", name)
			continue
		case c.state == rescueHome:
			continue
		case !ok && now.Sub(c.target.Updated) > rescueLostAfter:
			cc.rescueHome(c, "it is gone")
		case !ok:
			continue
		case c.state == rescueStandby && !prio[name]:
			cc.rescueHome(c, "priority taken away")
		case c.state == rescueStandby && touchedDown(a) && a.Airport == c.icao:
			c.state = rescueDown
			c.holdOffRunway(true)
			cc.log.printf("%-6s rescue: down, the vehicles wait at standby until it slows", name)
		case c.state == rescueStandby:
			c.holdOffRunway(c.nearRunway(a))
		case c.state == rescueDown && !touchedDown(a):
			c.state = rescueStandby
			cc.log.printf("%-6s rescue: off again (go-around), the vehicles stay at standby", name)
		case c.state == rescueDown && c.kts <= traffic.TaxiSpeedKts:
			c.state, c.enteredAt = rescueFollowing, now
			c.enter()
			cc.log.printf("%-6s rescue: slow, the vehicles drive to it", name)
		case c.state == rescueFollowing:
			if c.kts > rescueStoppedKts {
				c.stoppedAt = time.Time{}
				break
			}
			// Its stay counts once a vehicle is with it (they drive from
			// standby by the taxiways), or rescueReachWithin after they set off.
			if c.stoppedAt.IsZero() && !c.with() && now.Sub(c.enteredAt) < rescueReachWithin {
				break
			}
			if c.stoppedAt.IsZero() {
				c.stoppedAt = now
			}
			stay := rescueStay
			if c.medical {
				stay = rescueStayMedical
			}
			if now.Sub(c.stoppedAt) >= stay {
				cc.rescueHome(c, "stopped, attended")
			}
		}
	}
}

// sendRescue creates the crew for name, a coming in, and sends it to
// standby beside its runway.
func (cc *controlCenter) sendRescue(name string, a traffic.TrackedAircraft, medical bool) {
	cc.rescue.mu.Lock()
	if cc.rescue.sent == nil {
		cc.rescue.sent, cc.rescue.by = map[string]bool{}, map[string]*rescueCrew{}
	}
	cc.rescue.sent[name] = true
	cc.rescue.mu.Unlock()
	g, err := cc.graph(a.Airport)
	if err != nil {
		return
	}
	rwy := cc.rescueRunway(name, a.User, a.Airport)
	var legs []rescueLeg
	if rwy != "" {
		legs = cc.rescuePlan(g.Layout, rwy, medical)
	}
	if rwy == "" || len(legs) == 0 {
		cc.log.printf("%-6s rescue: none to send (runway %q, %d vehicles offered)", name, rwy, len(legs))
		return
	}
	spots := traffic.RescueStandby(g.Layout, rwy, len(legs))
	if len(spots) < len(legs) {
		cc.log.printf("%-6s rescue: runway %s unknown at %s", name, rwy, a.Airport)
		return
	}
	// In the order they will reach them: none waiting on the way of
	// another to its point (#1622).
	depots := make([]*airport.LatLon, len(legs))
	for i, leg := range legs {
		depots[i] = leg.depot
	}
	spots = traffic.RescueAssign(g.Layout, depots, spots)
	defBase, reqBase, err := cc.ids.Acquire()
	if err != nil {
		cc.log.printf("%-6s rescue: %v", name, err)
		return
	}
	c := &rescueCrew{tail: name, icao: a.Airport, runway: rwy, medical: medical, defBase: defBase, target: a, graph: g}
	stations := map[airport.LatLon]bool{}
	var prev *traffic.SimObjectRescue
	for i, leg := range legs {
		t := traffic.NewSimObjectRescue(cc.client, cc.inj, leg.title, reqBase+uint32(i))
		t.Layout = g.Layout
		// From one depot one after another, not on top of one another
		// (the default crew too: both from the depot nearest, #1597).
		t.After = prev
		if leg.depot != nil {
			t.Depot, t.After = leg.depot, prev
			stations[*leg.depot] = true
		}
		if err := t.Standby(spots[i]); err != nil {
			cc.log.printf("%-6s rescue: %s: %v", name, leg.title, err)
			continue
		}
		c.trucks = append(c.trucks, t)
		prev = t
	}
	if len(c.trucks) == 0 {
		cc.ids.Release(defBase)
		return
	}
	cc.rescue.mu.Lock()
	cc.rescue.by[name] = c
	cc.rescue.mu.Unlock()
	what := "fire trucks"
	if medical {
		what = "ambulance"
	}
	from := ""
	if len(stations) > 0 {
		from = fmt.Sprintf(" from %d fire station(s)", len(stations))
	}
	cc.log.printf("%-6s rescue: %d %s%s to standby beside %s %s", name, len(c.trucks), what, from, a.Airport, rwy)
}

// rescueLeg is one vehicle of a crew: its title and the fire station it
// leaves from (nil: the depot nearest its standby point).
type rescueLeg struct {
	title string
	depot *airport.LatLon
}

// rescueMaxVehicles caps a crew at its block of request IDs.
const rescueMaxVehicles = int(controlIDBlock)

// rescuePlan is the crew for an aircraft landing on rwy (#1587). Without
// fire stations in the layout (no depot with RescueVehicles), as before:
// rescueFireTrucks fire trucks, or an ambulance for a medical. With them,
// for a crash or fire each station sends all its vehicles (the station
// nearest the landing threshold first, capped at rescueMaxVehicles); for
// a medical the nearest station sends the reduced set: an ambulance, or
// one fire truck when the simulator offers no ambulance.
func (cc *controlCenter) rescuePlan(l *airport.Layout, rwy string, medical bool) []rescueLeg {
	stations := l.FireStations()
	var legs []rescueLeg
	if len(stations) == 0 {
		for _, t := range cc.rescueTitles(medical, rescueFireTrucks) {
			legs = append(legs, rescueLeg{title: t})
		}
		return legs
	}
	if _, end, ok := l.RunwayEnd(rwy); ok {
		at := func(i int) float64 {
			p := l.Parking[i].Position
			return calc.HaversineMeters(p.Lat, p.Lon, end.Threshold.Lat, end.Threshold.Lon)
		}
		sort.SliceStable(stations, func(i, j int) bool { return at(stations[i]) < at(stations[j]) })
	}
	depot := func(i int) *airport.LatLon {
		p := l.Parking[i].Position
		return &p
	}
	if medical {
		titles := cc.rescueTitles(true, 1)
		if len(titles) == 0 {
			titles = cc.rescueTitles(false, 1)
		}
		for _, t := range titles {
			legs = append(legs, rescueLeg{title: t, depot: depot(stations[0])})
		}
		return legs
	}
	total := 0
	for _, s := range stations {
		total += l.Parking[s].RescueVehicles
	}
	titles := cc.rescueTitles(false, min(total, rescueMaxVehicles))
	for _, s := range stations {
		for k := 0; k < l.Parking[s].RescueVehicles && len(titles) > 0; k++ {
			legs = append(legs, rescueLeg{title: titles[0], depot: depot(s)})
			titles = titles[1:]
		}
	}
	return legs
}

// rescueHome sends c's vehicles home. cc.rescue.mu held.
func (cc *controlCenter) rescueHome(c *rescueCrew, why string) {
	c.state = rescueHome
	for _, t := range c.trucks {
		if err := t.Home(); err != nil {
			cc.log.printf("%-6s rescue: home: %v", c.tail, err)
		}
	}
	cc.log.printf("%-6s rescue: back to the station (%s)", c.tail, why)
}

// rescueFrame drives the crews out by a frame: on their way, or after
// their aircraft (where it was last seen, moved on at its speed since).
func (cc *controlCenter) rescueFrame(now time.Time) {
	cc.rescue.mu.Lock()
	defer cc.rescue.mu.Unlock()
	for _, c := range cc.rescue.by {
		dt := now.Sub(c.frameAt).Seconds()
		c.frameAt = now
		if dt <= 0 || dt > 0.5 {
			continue
		}
		var first []airport.LatLon // where the vehicles going first are
		for i, t := range c.ordered() {
			t.KeepBehind(first)
			if p, _, ok := t.Track(); ok {
				first = append(first, p.Position)
			}
			var err error
			if c.state == rescueFollowing {
				a := c.target
				ahead := c.kts * 1852 / 3600 * math.Min(2, now.Sub(a.Updated).Seconds())
				off, hdg := ahead-rescueTrail*float64(i), a.Heading
				if c.ahead {
					// In front of it: its place RescueGapMeters ahead.
					off, hdg = ahead+rescueTrail*float64(i), a.Heading+180
				}
				lat, lon := calc.DisplaceByHeading(a.Position.Lat, a.Position.Lon, a.Heading, off)
				pos := airport.LatLon{Lat: lat, Lon: lon}
				err = t.Follow(traffic.GroundPose{Position: pos, Heading: math.Mod(hdg, 360)}, c.kts, dt)
			} else {
				err = t.Update(dt)
			}
			if err != nil {
				cc.log.printf("%-6s rescue: %v", c.tail, err)
			}
		}
	}
}

// ordered are the crew's vehicles in the order they go: as sent, and
// driving to the aircraft the nearest it first, which takes the place
// nearest it; the others keep behind (#1597: one arriving later never
// drives through one already there).
func (c *rescueCrew) ordered() []*traffic.SimObjectRescue {
	if c.state != rescueFollowing {
		return c.trucks
	}
	out := append([]*traffic.SimObjectRescue(nil), c.trucks...)
	at := c.target.Position
	dist := map[*traffic.SimObjectRescue]float64{}
	for _, t := range out {
		dist[t] = math.Inf(1)
		if p, _, ok := t.Track(); ok {
			dist[t] = calc.HaversineMeters(p.Position.Lat, p.Position.Lon, at.Lat, at.Lon)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return dist[out[i]] < dist[out[j]] })
	return out
}

// rescueWithM: a rescue vehicle this close to the aircraft it follows is
// with it (an emergency arrival rolls out and vacates as any other; they
// catch it up on the runway or the taxiway).
const rescueWithM = traffic.RescueGapMeters + 2*traffic.RescueStopMeters + 10

// rescueStatus reports whether vehicles were sent for tail, and whether
// one is with it (down, a vehicle within rescueWithM): a scene's
// "ROLE:rescue" and "ROLE:rescue:with".
func (cc *controlCenter) rescueStatus(tail string) (sent, with bool) {
	cc.rescue.mu.Lock()
	defer cc.rescue.mu.Unlock()
	c := cc.rescue.by[tail]
	if c == nil {
		return cc.rescue.sent[tail], false
	}
	return true, c.state == rescueFollowing && c.with()
}

// with: a vehicle of the crew is within rescueWithM of the aircraft.
func (c *rescueCrew) with() bool {
	at := c.target.Position
	for _, t := range c.trucks {
		if p, _, ok := t.Track(); ok && calc.HaversineMeters(p.Position.Lat, p.Position.Lon, at.Lat, at.Lon) <= rescueWithM {
			return true
		}
	}
	return false
}

// rescueReachWithin: the aircraft's stay counts from this long after the
// vehicles set off to it, should none get to it before.
const rescueReachWithin = 10 * time.Minute

// rescueTrack is where the vehicles out for tail are, for the map.
func (cc *controlCenter) rescueTrack(tail string) []tugView {
	cc.rescue.mu.Lock()
	defer cc.rescue.mu.Unlock()
	if c := cc.rescue.by[tail]; c != nil {
		return c.track()
	}
	return nil
}

// rescueHandle takes the crews' creation answers.
func (cc *controlCenter) rescueHandle(msg engine.Message) bool {
	cc.rescue.mu.Lock()
	defer cc.rescue.mu.Unlock()
	for _, c := range cc.rescue.by {
		for _, t := range c.trucks {
			if t.Handle(msg) {
				return true
			}
		}
	}
	return false
}

// removeRescue takes every crew away at once (stopping).
func (cc *controlCenter) removeRescue() {
	cc.rescue.mu.Lock()
	defer cc.rescue.mu.Unlock()
	for name, c := range cc.rescue.by {
		for _, t := range c.trucks {
			_ = t.Remove()
		}
		cc.ids.Release(c.defBase)
		delete(cc.rescue.by, name)
	}
}
