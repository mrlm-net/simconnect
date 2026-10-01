//go:build windows

package main

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
	return traffic.WindSaid(math.Mod(w.WindDirTrue+mv+720, 360), w.WindKts, w.GustKts)
}

// oneDesignator is one runway designator of a runway named "12/30": a crossing
// clearance names one (Doc 4444 12.3.4.9).
func oneDesignator(name string) string {
	first, _, _ := strings.Cut(name, "/")
	return first
}

// aipUnitNames are unit call signs from the AIP where the scenery's
// frequency names do not give the unit (LKPR AD 2.18: approach and
// departure are "Ruzyně Radar"; the scenery names them "RUZYNE"). By
// airport, then frequency as the radio writes it.
var aipUnitNames = map[string]map[string]string{
	"LKPR": {
		"120.06": "Ruzyne Delivery", "121.91": "Ruzyne Ground", "134.56": "Ruzyne Tower",
		"118.31": "Ruzyne Radar", "119.01": "Ruzyne Radar", "120.53": "Praha Radar", "127.58": "Praha Radar",
	},
}

// aipUnitName is the AIP's call sign of the unit on freq at icao, "" none.
func aipUnitName(icao, freq string) string {
	return aipUnitNames[icao][freq]
}
