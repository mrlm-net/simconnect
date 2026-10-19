package world

import (
	"errors"
	"fmt"
	"maps"
	"strings"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Emergencies (the emergency add-on, part 2): an aircraft with priority
// lands at its own time, number one, the other arrivals fitted around it,
// and no departure takes off toward it while it is close in. Ours declare
// it on the radio (a scenario's "emergency" action); the player's is the
// host's own ATC (Priority with no call sign).

// priorityHoldNM: while an aircraft with priority is this close to the
// airport on its way in, departures wait.
const priorityHoldNM = 15.0

// playerTail is the user aircraft's name in the sequence when the host
// gives none.
const playerTail = "Player"

// priorities are the call signs with priority, by airport.
type priorities struct {
	mu sync.Mutex
	by map[string]bool // call sign → priority
}

func (p *priorities) set(tail string, on bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.by == nil {
		p.by = map[string]bool{}
	}
	if on {
		p.by[tail] = true
	} else {
		delete(p.by, tail)
	}
}

func (p *priorities) all() map[string]bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return maps.Clone(p.by)
}

// Priority gives tail priority (an emergency) or takes it away: it lands
// number one at its own time, the other arrivals are fitted around it,
// and departures wait while it is close in. tail "" is the user aircraft
// (the host's ATC handles its emergency: its call and the words).
func (w *World) Priority(tail string, on bool) {
	if tail == "" {
		tail = playerTail
	}
	w.st.core.prio.set(tail, on)
	state := "given"
	if !on {
		state = "taken away"
	}
	w.st.core.log.printf("%-6s priority %s", tail, state)
}

// declareEmergency has one of ours declare an emergency of kind (level ""
// by the kind): squawk 7700, its MAYDAY or PAN PAN, approach's answer,
// and priority. Arrivals on their way in only, for now.
func (cc *controlCenter) declareEmergency(it *controlled, kind traffic.EmergencyKind, level string) error {
	if it.arr == nil || it.arr.State() != traffic.ArrivalApproaching {
		return errors.New("emergency: an arrival on its way in only, for now")
	}
	if level == "" {
		level = traffic.EmergencyLevel(kind)
	}
	level = strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(level, " ", ""), "-", ""))
	if level != traffic.LevelMayday && level != traffic.LevelPanPan {
		return fmt.Errorf("emergency: level %q, want mayday or panpan", level)
	}
	it.mu.Lock()
	it.view.Squawk = traffic.EmergencySquawk
	pos, rwy, tail := it.atc, it.view.Runway, it.Tail
	it.mu.Unlock()
	if pos == "" {
		pos = traffic.PosApproach
	}
	station, _ := it.station(pos)
	cc.core.prio.set(tail, true)
	cc.log.printf("%-6s emergency: %s (%s), squawk %s, priority", tail, kind, level, traffic.EmergencySquawk)
	it.say(traffic.DeclareEmergency(pos, station, tail, kind, level, rwy))
	it.say(traffic.AcknowledgeEmergency(pos, tail, level, rwy))
	return nil
}

// priorityClose names one with priority within priorityHoldNM of icao on
// its way in ("" none): departures wait for it.
func (cc *controlCenter) priorityClose(icao string) string {
	prio := cc.core.prio.all()
	if len(prio) == 0 {
		return ""
	}
	g, err := cc.graph(icao)
	if err != nil {
		return ""
	}
	l := g.Layout
	for _, a := range cc.world.Aircraft() {
		name := a.Tail
		if a.User {
			name = playerTail
			if p, ok := cc.core.playerLanding(); ok && p.Callsign != "" && prio[p.Callsign] {
				name = p.Callsign
			}
		}
		if !prio[name] || a.Grounded() {
			continue
		}
		if calc.HaversineNM(a.Position.Lat, a.Position.Lon, l.Latitude, l.Longitude) <= priorityHoldNM {
			return name
		}
	}
	return ""
}

// playerPriorityNM: the user aircraft with priority is sequenced at the
// nearest airport of ours within this.
const playerPriorityNM = 60.0
