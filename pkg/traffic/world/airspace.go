package world

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Airspace classes (#570): the managed airports' control zones are of
// zoneClass (-airspace, D by default), out to zoneNM and up to zoneTopFt
// above the field; around them class E. Who is separated from whom
// follows traffic.SeparationRequired; where VFR traffic is not separated
// it is told of the other (traffic information) instead, and a loss of
// "separation" between them is no loss. The class is core.zone.

const (
	zoneNM    = 10.0
	zoneTopFt = 5000.0
	// trafficInfoEvery: the same pair is told of each other at most this
	// often.
	trafficInfoEvery = 3 * time.Minute
)

// parseAirspaceClass reads -airspace.
func parseAirspaceClass(s string) (traffic.AirspaceClass, error) {
	switch cl := traffic.AirspaceClass(strings.ToUpper(s)); cl {
	case traffic.ClassC, traffic.ClassD, traffic.ClassE, traffic.ClassG:
		return cl, nil
	}
	return "", fmt.Errorf("airspace class C, D, E or G, not %q", s)
}

// rulesOf is the flight rules an aircraft flies under: ours as spawned
// (ControlView.Rules). Others' the simulator does not give: unknown,
// treated as IFR, so separated (E22: guessed VFR from a light type's
// title, against showing only what the sim gives).
func (cc *controlCenter) rulesOf(a traffic.TrackedAircraft) string {
	if a.Ours {
		if it := cc.byTail(a.Tail); it != nil {
			it.mu.Lock()
			r := it.view.Rules
			it.mu.Unlock()
			if r != "" {
				return r
			}
		}
	}
	return "IFR"
}

// classAt is the class of the airspace an aircraft is in: a managed
// airport's zone (zoneClass) or class E around.
func (cc *controlCenter) classAt(a traffic.TrackedAircraft, airports []string) traffic.AirspaceClass {
	for _, icao := range airports {
		g, err := cc.graph(icao)
		if err != nil {
			continue
		}
		l := g.Layout
		if calc.HaversineNM(l.Latitude, l.Longitude, a.Position.Lat, a.Position.Lon) <= zoneNM && a.AltFt-l.Altitude/0.3048 <= zoneTopFt {
			return cc.core.zone
		}
	}
	return traffic.ClassE
}

// separationNeeded is a check of two call signs among aircraft: whether
// ATC separates them where they are (the stricter of their two classes'
// rules).
func (cc *controlCenter) separationNeeded(aircraft []traffic.TrackedAircraft, airports []string) func(a, b string) bool {
	by := map[string]traffic.TrackedAircraft{}
	for _, x := range aircraft {
		by[x.Tail] = x
	}
	return func(a, b string) bool {
		x, okA := by[a]
		y, okB := by[b]
		if !okA || !okB {
			return true // not known: as before
		}
		ra, rb := cc.rulesOf(x), cc.rulesOf(y)
		return traffic.SeparationRequired(cc.classAt(x, airports), ra, rb) || traffic.SeparationRequired(cc.classAt(y, airports), ra, rb)
	}
}

// tellTraffic gives each of ours in conflict c traffic information on the
// other, on its frequency (#570), at most every trafficInfoEvery.
func (w *conflictWatch) tellTraffic(now time.Time, c traffic.Conflict, aircraft []traffic.TrackedAircraft) {
	pair := c.A + "/" + c.B
	w.mu.Lock()
	if now.Sub(w.informed[pair]) < trafficInfoEvery {
		w.mu.Unlock()
		return
	}
	w.informed[pair] = now
	w.mu.Unlock()
	by := map[string]traffic.TrackedAircraft{}
	for _, x := range aircraft {
		by[x.Tail] = x
	}
	for _, cs := range []string{c.A, c.B} {
		me, other := by[cs], by[otherOf(c, cs)]
		it := w.s.cc.byTail(cs)
		if it == nil || me.OnGround || other.Tail == "" || me.AGLFt < trafficInfoMinAGLFt {
			continue // nothing to an aircraft taking off or landing (CAP 413 4.26)
		}
		it.mu.Lock()
		pos := it.atc
		it.mu.Unlock()
		if pos == "" {
			continue
		}
		clock, nm, dir := traffic.TrafficRelative(me.Position, me.Heading, other.Position, other.Heading)
		typ := typeSaid(traffic.ProfileFor(other.Title).Type)
		level := fmt.Sprintf("%.0f feet", math.Round(other.AltFt/100)*100)
		if ta := w.s.cc.taOf(it.ICAO); ta > 0 && other.AltFt >= ta || ta <= 0 && other.AltFt >= 5500 { // flight levels above the transition altitude (E23)
			level = "flight level " + fmt.Sprintf("%03.0f", math.Round(other.AltFt/100))
		}
		tx := traffic.TrafficInformation(pos, cs, clock, nm, dir, typ, level)
		it.call(pos, prioUrgent, func() {
			// Landing or taking off by the time it is said: dropped (live,
			// AFR1602 told of a DA62 climbing away 14 s before touchdown).
			it.mu.Lock()
			state := it.view.State
			it.mu.Unlock()
			if slices.Contains(finalStageStates, state) {
				return
			}
			it.say(tx)
		})
		w.s.cc.log.printf("%-6s traffic information on %s (no separation required here)", cs, other.Tail)
	}
}

// No traffic information to an aircraft "in the process of taking off or
// in the final stages of an approach and landing" (CAP 413 4.26): below
// trafficInfoMinAGLFt, or in one of finalStageStates when it would be said.
const trafficInfoMinAGLFt = 1000.0

var finalStageStates = []string{"lining up", "lined up", "departing", "landing", "rollout"}

// trafficInfoNearNM: traffic already at its closest point is told of
// only when nearer than this.
const trafficInfoNearNM = 1.5

// otherOf is the other aircraft of conflict c.
func otherOf(c traffic.Conflict, cs string) string {
	if c.A == cs {
		return c.B
	}
	return c.A
}
