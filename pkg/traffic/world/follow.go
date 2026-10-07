package world

import (
	"math"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Follow on the ground: told with its taxi clearance when another of ours
// taxis ahead on the same way ("…via A, Z, follow the company Airbus
// A320"), or given on the map. The follower stays behind its leader
// wherever their ways meet (GroundPicture.Follow).

// A leader taxis on the follower's route ahead: within followOnRouteM of
// it, along it within followAheadM, heading its way within followHdgDeg.
const (
	followOnRouteM = 40.0
	followAheadM   = 800.0
	followHdgDeg   = 45.0
)

// leaderFor is the aircraft of ours taxiing ahead of it on route r, the
// nearest along the route; nil for none.
func (cc *controlCenter) leaderFor(it *controlled, r *airport.Route) *controlled {
	if r == nil || len(r.Points) < 2 {
		return nil
	}
	cum := make([]float64, len(r.Points))
	for i := 1; i < len(r.Points); i++ {
		a, b := r.Points[i-1], r.Points[i]
		cum[i] = cum[i-1] + calc.HaversineMeters(a.Lat, a.Lon, b.Lat, b.Lon)
	}
	var best *controlled
	bestAt := math.Inf(1)
	for _, o := range cc.snapshotItems() {
		if o == it || o.ICAO != it.ICAO || (o.dep == nil && o.arr == nil) {
			continue
		}
		o.mu.Lock()
		pos, hdg, state, ground := o.view.Position, o.view.Heading, o.view.State, o.view.OnGround
		o.mu.Unlock()
		if !ground || (state != traffic.TaxiTaxiing.String() && state != traffic.ArrivalTaxiing.String()) {
			continue
		}
		for i := 1; i < len(r.Points); i++ {
			a, b := r.Points[i-1], r.Points[i]
			leg := calc.BearingDegrees(a.Lat, a.Lon, b.Lat, b.Lon)
			d, along := nearLeg(a, b, pos)
			at := cum[i-1] + along
			if d <= followOnRouteM && at > 0 && at <= followAheadM && math.Abs(angleDiff(hdg, leg)) <= followHdgDeg {
				if at < bestAt {
					best, bestAt = o, at
				}
				break
			}
		}
	}
	return best
}

// nearLeg is how far p lies from leg a–b and how far along it.
func nearLeg(a, b, p airport.LatLon) (dist, along float64) {
	l := calc.HaversineMeters(a.Lat, a.Lon, b.Lat, b.Lon)
	if l == 0 {
		return calc.HaversineMeters(a.Lat, a.Lon, p.Lat, p.Lon), 0
	}
	h := calc.BearingDegrees(a.Lat, a.Lon, b.Lat, b.Lon)
	hp := calc.BearingDegrees(a.Lat, a.Lon, p.Lat, p.Lon)
	r := calc.HaversineMeters(a.Lat, a.Lon, p.Lat, p.Lon)
	along = r * math.Cos((hp-h)*math.Pi/180)
	t := math.Max(0, math.Min(l, along))
	lat, lon := calc.DisplaceByHeading(a.Lat, a.Lon, h, t)
	return calc.HaversineMeters(lat, lon, p.Lat, p.Lon), along
}

func angleDiff(a, b float64) float64 {
	return math.Mod(a-b+540, 360) - 180
}

// followSaid describes leader to it as ground does: its operator ("the
// company" for its own airline's) and type — "company Airbus A320",
// "Lufthansa Boeing 737".
func (cc *controlCenter) followSaid(it, leader *controlled) string {
	leader.mu.Lock()
	model := leader.view.Model
	leader.mu.Unlock()
	what := typeSaid(traffic.ProfileFor(strings.SplitN(model, liverySep, 2)[0]).Type)
	op := airlineOf(leader.Tail)
	switch {
	case op != "" && op == airlineOf(it.Tail):
		return "company " + what
	case op != "":
		if _, name, ok := traffic.Telephony(op); ok && name != "" {
			return name + " " + what
		}
	}
	return what
}

// follow has it follow leader on the ground from now on (the picture's
// give-way), told ground's way; false when either has no aircraft yet.
func (cc *controlCenter) follow(it, leader *controlled) bool {
	it.mu.Lock()
	id := it.objectID
	it.mu.Unlock()
	leader.mu.Lock()
	lid := leader.objectID
	leader.mu.Unlock()
	if id == 0 || lid == 0 || cc.world == nil {
		return false
	}
	cc.world.Ground(it.ICAO).Follow(id, lid)
	tlog.printf("%-6s ground: follows %s", it.Tail, leader.Tail)
	return true
}
