package world

import (
	"math"
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
// station (the nearest vehicle depot) to standby points beside its landing
// runway and wait; once it is down they follow it down the runway to where
// it stops, stay a while, then drive back and go. A medical emergency gets
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
	rescueFollowing                    // after the aircraft, it down
	rescueHome                         // driving back
)

// rescueCrew is the vehicles sent for one aircraft.
type rescueCrew struct {
	tail, icao, runway string
	medical            bool
	defBase            uint32
	trucks             []*traffic.SimObjectRescue
	state              rescueState
	target             traffic.TrackedAircraft // the aircraft as last seen
	stoppedAt          time.Time               // since when it has stood, zero moving
	frameAt            time.Time
}

// rescueCrews are the crews out.
type rescueCrews struct {
	mu   sync.Mutex
	by   map[string]*rescueCrew // by the aircraft's name
	sent map[string]bool        // names already given a crew: one each
}

// rescueTitles are the vehicles of a crew: an ambulance for a medical, else
// rescueFireTrucks fire trucks (GSX's by preference); none when the
// simulator offers none.
func (cc *controlCenter) rescueTitles(medical bool) []string {
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
		return pick(gsx, rescueFireTrucks)
	}
	return pick(stock, rescueFireTrucks)
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
			c.target = a
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
		case c.state == rescueStandby && !prio[name]:
			cc.rescueHome(c, "priority taken away")
		case c.state == rescueStandby && a.Grounded() && a.Airport == c.icao:
			c.state = rescueFollowing
			cc.log.printf("%-6s rescue: down, the vehicles follow it", name)
		case c.state == rescueFollowing:
			if a.GroundKts > rescueStoppedKts {
				c.stoppedAt = time.Time{}
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
	titles := cc.rescueTitles(medical)
	if rwy == "" || len(titles) == 0 {
		cc.log.printf("%-6s rescue: none to send (runway %q, %d vehicles offered)", name, rwy, len(titles))
		return
	}
	spots := traffic.RescueStandby(g.Layout, rwy, len(titles))
	if len(spots) < len(titles) {
		cc.log.printf("%-6s rescue: runway %s unknown at %s", name, rwy, a.Airport)
		return
	}
	defBase, reqBase, err := cc.ids.Acquire()
	if err != nil {
		cc.log.printf("%-6s rescue: %v", name, err)
		return
	}
	c := &rescueCrew{tail: name, icao: a.Airport, runway: rwy, medical: medical, defBase: defBase, target: a}
	for i, title := range titles {
		t := traffic.NewSimObjectRescue(cc.client, cc.inj, title, reqBase+uint32(i))
		t.Layout = g.Layout
		if err := t.Standby(spots[i]); err != nil {
			cc.log.printf("%-6s rescue: %s: %v", name, title, err)
			continue
		}
		c.trucks = append(c.trucks, t)
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
	cc.log.printf("%-6s rescue: %d %s to standby beside %s %s", name, len(c.trucks), what, a.Airport, rwy)
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
		for i, t := range c.trucks {
			var err error
			if c.state == rescueFollowing {
				a := c.target
				ahead := a.GroundKts * 1852 / 3600 * math.Min(2, now.Sub(a.Updated).Seconds())
				lat, lon := calc.DisplaceByHeading(a.Position.Lat, a.Position.Lon, a.Heading, ahead-rescueTrail*float64(i))
				pos := airport.LatLon{Lat: lat, Lon: lon}
				err = t.Follow(traffic.GroundPose{Position: pos, Heading: a.Heading}, a.GroundKts, dt)
			} else {
				err = t.Update(dt)
			}
			if err != nil {
				cc.log.printf("%-6s rescue: %v", c.tail, err)
			}
		}
	}
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
