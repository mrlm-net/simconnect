package world

import (
	"fmt"
	"math"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// The game never hands the player an arrival already short of spacing
// (TNCM, seed 7911933624517526090: DAL1129 on ZPATA 50 NM to go and
// UAL1170 on MNOLO 49 NM to go, spawned 30 s apart, met at 6,600 ft 1.2 NM
// apart before the player could act). Its flights are the player's (#1096):
// the AI gives them no speed or vector, so the game spaces them where it
// draws them, by their distance to go to the runway against that of the
// arrivals already in its sequence (flown at like speeds, their times at
// the runway as far apart). Short, another of the runway's procedures is
// drawn; none spaced, it is not spawned now (ErrSpawnBlocked), and the
// game draws again at its next interval.

// gameSpacingMarginNM: what the distances to go may be off by as times at
// the runway (speeds, turns), on top of the spacing.
const gameSpacingMarginNM = 4.0

// gameSpawnTries: how many of the runway's procedures are drawn before the
// spawn waits.
const gameSpawnTries = 6

// gameSpaced is the procedure a game arrival r flies (route, its name and
// approach): the one drawn, or another of its runway's when that one lands
// short of spacing behind or ahead of an arrival in the sequence; an
// ErrSpawnBlocked error when none is spaced.
func (cc *controlCenter) gameSpaced(g *airport.Graph, r SpawnRequest, route []airport.NavPoint, name, expect string) ([]airport.NavPoint, string, string, error) {
	who := ""
	for try := 0; ; try++ {
		if who = cc.gameShort(g, r.Runway, route); who == "" {
			return route, name, expect, nil
		}
		if try+1 >= gameSpawnTries || r.ProcName != "" {
			break
		}
		next, n, e, err := cc.procedureFor(g, r)
		if err != nil || len(next) == 0 {
			break
		}
		route, name, expect = next, n, e
	}
	return nil, "", "", fmt.Errorf("%w: it would land short of spacing with %s; the game draws again later", traffic.ErrSpawnBlocked, who)
}

// gameShort is the arrival in runway's sequence that one flying route
// from its first point would land short of spacing with, by their
// distances to go to the runway; "" none.
func (cc *controlCenter) gameShort(g *airport.Graph, runway string, route []airport.NavPoint) string {
	if cc.sequencesAt == nil || len(route) == 0 {
		return ""
	}
	_, end, ok := g.Layout.RunwayEnd(runway)
	if !ok {
		return ""
	}
	nm := 0.0
	for i := 1; i < len(route); i++ {
		a, b := route[i-1].Position, route[i].Position
		nm += calc.HaversineNM(a.Lat, a.Lon, b.Lat, b.Lon)
	}
	last, thr := route[len(route)-1].Position, end.LandingThreshold()
	nm += calc.HaversineNM(last.Lat, last.Lon, thr.Lat, thr.Lon)
	for _, e := range cc.sequencesAt(g.Layout.ICAO)[runway] {
		if e.Runway != "" || e.DistanceToGoNM <= 0 {
			continue // the adjacent final's, or not measured yet
		}
		gap := math.Max(e.SpacingNM, traffic.DefaultMergeSpacingNM) + gameSpacingMarginNM
		if math.Abs(nm-e.DistanceToGoNM) < gap {
			return e.Callsign
		}
	}
	return ""
}
