package world

import (
	"math"
	"math/rand/v2"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// A departure's crew now and then asks, with its taxi request, to depart
// from another runway (#621): crewRunwayShare of them, for a runway end with
// a SID to the same fix whose tailwind is crewRunwayMaxTailKts at most;
// ground gives it when that runway crosses no runway in use and takes no
// arrivals, else "unable".
const (
	crewRunwayShare      = 0.02
	crewRunwayMaxTailKts = 5.0
)

// crewRunway is the runway the crew of it asks for instead of its own, ""
// when it does not ask (most) or there is none to ask for. it.mu is held.
func (it *controlled) crewRunway() string {
	if it.dep == nil || it.view.Procedure == "" || it.cc.procedures == nil || rand.Float64() >= crewRunwayShare {
		return ""
	}
	p, ok := it.cc.procedures(it.ICAO)
	if !ok {
		return ""
	}
	var w *nav.Weather
	if it.cc.weather != nil {
		w = it.cc.weather()
	}
	best, bestD := "", math.Inf(1)
	for _, r := range it.graph.Layout.Runways {
		for _, end := range []airport.RunwayEnd{r.Primary, r.Secondary} {
			if end.Name == "" || end.Name == it.view.Runway || sameFix(p.SIDsFor(end.Name), it.view.Procedure) == nil {
				continue
			}
			if w != nil && w.WindKts*math.Cos((w.WindDirTrue-end.Heading)*math.Pi/180) < -crewRunwayMaxTailKts {
				continue // a tailwind too strong
			}
			d := calc.HaversineMeters(it.view.Position.Lat, it.view.Position.Lon, end.Threshold.Lat, end.Threshold.Lon)
			if d < bestD {
				best, bestD = end.Name, d
			}
		}
	}
	return best
}

// grantRunway answers the crew's runway request before the taxi clearance:
// the departure re-cleared to it (changeDepartureRunway says so) when it
// crosses no runway in use and takes no arrivals; else "unable".
func (it *controlled) grantRunway() {
	it.mu.Lock()
	rwy, own := it.askedRunway, it.view.Runway
	it.askedRunway = ""
	it.mu.Unlock()
	if rwy == "" || it.dep == nil {
		return
	}
	g := it.graph
	asked, _, free := g.Layout.RunwayEnd(rwy)
	for _, arrival := range []bool{true, false} {
		for _, end := range it.cc.runwaysInUse(g, arrival) {
			inUse, _, found := g.Layout.RunwayEnd(end.Name)
			switch {
			case !found:
			case inUse.Index == asked.Index:
				if arrival {
					free = false // it takes arrivals
				}
			case runwaysCross(asked, inUse):
				free = false // a take-off through a runway in use: not coordinated
			}
		}
	}
	if !free {
		it.say(traffic.UnableRunway(it.Tail, own))
		it.cc.log.printf("%-6s crew: runway %s asked, unable (%s in use)", it.Tail, rwy, own)
		return
	}
	it.cc.log.printf("%-6s crew: runway %s asked, given", it.Tail, rwy)
	it.cc.changeDepartureRunway(g, it, rwy)
}

// runwaysCross reports whether runways a and b cross or touch (their
// centrelines, threshold to threshold).
func runwaysCross(a, b airport.Runway) bool {
	o := a.Primary.Threshold
	xy := func(p airport.LatLon) (float64, float64) {
		return (p.Lon - o.Lon) * math.Cos(o.Lat*math.Pi/180), p.Lat - o.Lat
	}
	ax1, ay1 := xy(a.Primary.Threshold)
	ax2, ay2 := xy(a.Secondary.Threshold)
	bx1, by1 := xy(b.Primary.Threshold)
	bx2, by2 := xy(b.Secondary.Threshold)
	cross := func(x1, y1, x2, y2, x3, y3 float64) float64 { return (x2-x1)*(y3-y1) - (y2-y1)*(x3-x1) }
	d1, d2 := cross(bx1, by1, bx2, by2, ax1, ay1), cross(bx1, by1, bx2, by2, ax2, ay2)
	d3, d4 := cross(ax1, ay1, ax2, ay2, bx1, by1), cross(ax1, ay1, ax2, ay2, bx2, by2)
	return (d1 > 0) != (d2 > 0) && (d3 > 0) != (d4 > 0)
}
