package world

import (
	"fmt"
	"hash/fnv"
	"math"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// What the clearances say, as docs/traffic-phraseology.md has it (#462):
// destinations by name, procedures by their fix, levels against the
// transition altitude, the wind in magnetic degrees, a squawk code.

// airportName is icao as a clearance names it ("Frankfurt").
func (cc *controlCenter) airportName(icao string) string {
	if f := cc.namedAirport; f != nil {
		return f(icao)
	}
	return defaultSchedule.AirportName(icao)
}

// procedureSaid is a SID or STAR as said, with the fix it is named after
// written out when a point of the flight plan or procedure starts with its
// letters ("BALT7D" and BALTU → "BALTU 7D").
func procedureSaid(name string, p *planned) string {
	prefix := name
	if i := strings.IndexAny(name, "0123456789"); i > 0 {
		prefix = name[:i]
	}
	var idents []string
	if p != nil {
		for _, pt := range p.route {
			idents = append(idents, pt.Ident)
		}
		if p.plan != nil {
			for _, w := range p.plan.Waypoints {
				idents = append(idents, w.Ident)
			}
		}
	}
	for _, id := range idents {
		if len(id) > len(prefix) && strings.HasPrefix(strings.ToUpper(id), strings.ToUpper(prefix)) {
			return traffic.SaidProcedure(name, id)
		}
	}
	return traffic.SaidProcedure(name, "")
}

// squawkFor is the SSR code a departure is given: the same for a call sign,
// four octal digits from 4001 to 4777, never an emergency or conspicuity
// code.
func squawkFor(cs string) string {
	h := fnv.New32a()
	h.Write([]byte(cs))
	n := 1 + int(h.Sum32()%511) // 1–511: 4001–4777 in octal
	return fmt.Sprintf("4%03o", n)
}

// validSquawk: four octal digits, not an emergency code (7500 unlawful
// interference, 7600 radio failure, 7700 emergency).
func validSquawk(s string) bool {
	if len(s) != 4 || s == "7500" || s == "7600" || s == "7700" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '7' {
			return false
		}
	}
	return true
}

// initialClimbSaid is the initial climb of a departure on sid with limits
// lim as said (Limits.InitialClimbFor: by SID, else the airport's, else
// FL100), a level against the transition altitude.
func initialClimbSaid(lim airport.Limits, sid string) string {
	return traffic.LevelSaidAbove(lim.InitialClimbFor(sid), lim.TransitionAltitudeFt)
}

// windSaid is the surface wind at icao as the tower says it, magnetic
// (airport.Procedures.MagVar: magnetic = true + MagVar); "" without weather.
func (cc *controlCenter) windSaid(icao string) string {
	if cc.weather == nil {
		return ""
	}
	w := cc.weather()
	if w == nil {
		return ""
	}
	mv := 0.0
	if cc.procedures != nil {
		if p, ok := cc.procedures(icao); ok {
			mv = p.MagVar
		}
	}
	dir := math.Mod(w.WindDirTrue+mv+720, 360)
	if w.Variable {
		dir = math.NaN() // said variable (#753)
	}
	return traffic.WindSaidAs(traffic.PhraseologyFor(icao), dir, w.WindKts, w.GustKts)
}

// magVar is icao's magnetic variation (airport.Procedures.MagVar), 0 unknown.
// taOf is the transition altitude at icao (ft), 0 unknown: the levels
// said by it (#101).
func (cc *controlCenter) taOf(icao string) float64 {
	var g *airport.Graph
	cc.mu.Lock()
	for _, it := range cc.items {
		if it.ICAO == icao && it.graph != nil {
			g = it.graph // the airport's graph, from one of ours there
			break
		}
	}
	cc.mu.Unlock()
	if g == nil {
		return 0
	}
	return cc.limitsOf(g).TransitionAltitudeFt
}

func (cc *controlCenter) magVar(icao string) float64 {
	if cc.procedures != nil {
		if p, ok := cc.procedures(icao); ok {
			return p.MagVar
		}
	}
	return 0
}

// oneDesignator is one runway designator of a runway named "12/30": a crossing
// clearance names one (Doc 4444 12.3.4.9).
func oneDesignator(name string) string {
	first, _, _ := strings.Cut(name, "/")
	return first
}

// goAround is the tower's go-around for it: an IFR arrival told its climb
// and heading (Doc 4444 12.3.4.18 with the missed approach), the reason
// said as traffic, not a call sign ("CSA821 on the runway" → "traffic on
// the runway"); a VFR circuit arrival goes round its circuit, no climb.
func (it *controlled) goAround(why string) traffic.Transmission {
	if strings.HasSuffix(why, " on the runway") {
		why = "traffic on the runway"
	}
	if it.circuit != nil {
		return traffic.GoAround(it.Tail, why)
	}
	return traffic.GoAroundWith(it.Tail, why, it.goAroundLevel(), "runway heading")
}

// goAroundAck is the tower's answer to a crew going around on its own: its
// climb and heading for an IFR arrival, "roger" for a circuit one.
func (it *controlled) goAroundAck() traffic.Transmission {
	if it.circuit != nil {
		return traffic.Acknowledge(traffic.PosTower, it.Tail)
	}
	return traffic.GoAroundAcknowledged(it.Tail, it.goAroundLevel(), "runway heading")
}

// goAroundLevel is the altitude a go-around climbs to, as said: its
// circuit's once it flies one, else traffic.GoAroundHeightFt above the
// field, to the hundred feet.
func (it *controlled) goAroundLevel() string {
	ft := it.graph.Layout.Altitude/0.3048 + traffic.GoAroundHeightFt
	if it.arr != nil {
		if fx := it.arr.CircuitFixes(); len(fx) > 0 && fx[0].AltMin > 0 {
			ft = fx[0].AltMin / 0.3048
		}
	}
	return traffic.LevelSaidAbove(math.Round(ft/100)*100, it.cc.taOf(it.ICAO))
}
