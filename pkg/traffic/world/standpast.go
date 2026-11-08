package world

import (
	"fmt"
	"strings"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// standPastWarned: the stands and models already logged by standPastCentre
// ("ICAO stand model"), once each per run.
var standPastWarned sync.Map

// standPastCentre logs, once per stand and model, a stand offset past the
// parking point: traffic.StandPoint stops at the point instead, short of
// where the offset wants (live, LKPR B14: an Emirates 777 by the B748
// helper's door 50.5 m in on a 35 m circle, stopped at the point, the
// jetway 15.5 m aft of L1, nothing logged). It reports whether it is past.
func (cc *controlCenter) standPastCentre(g *airport.Graph, stand int, model string, nose float64) bool {
	if g == nil || stand < 0 || stand >= len(g.Layout.Parking) {
		return false
	}
	p := g.Layout.Parking[stand]
	if p.Radius <= 0 || nose <= p.Radius {
		return false
	}
	key := strings.ToUpper(fmt.Sprint(g.Layout.ICAO, " ", stand, " ", model))
	if _, seen := standPastWarned.LoadOrStore(key, true); !seen && cc != nil && cc.log != nil {
		cc.log.printf("stand %s %s: %s would stop %.2f m past the parking point (offset %.2f m, circle %.2f m); stops on the point — STAND PAST CENTRE",
			g.Layout.ICAO, p.Label(), model, nose-p.Radius, nose, p.Radius)
	}
	return true
}
