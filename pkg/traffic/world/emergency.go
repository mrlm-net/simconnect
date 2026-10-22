package world

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"strings"
	"sync"
	"time"

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
	mu    sync.Mutex
	by    map[string]bool                  // call sign → priority
	kinds map[string]traffic.EmergencyKind // call sign → its emergency, when told
}

// setKind notes tail's emergency (its rescue: an ambulance for a medical).
func (p *priorities) setKind(tail string, kind traffic.EmergencyKind) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.kinds == nil {
		p.kinds = map[string]traffic.EmergencyKind{}
	}
	p.kinds[tail] = kind
}

func (p *priorities) kind(tail string) traffic.EmergencyKind {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.kinds[tail]
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

// PriorityFor is Priority given for an emergency of kind: the airport's
// rescue sent for it is the kind's (an ambulance for a medical, else fire
// trucks).
func (w *World) PriorityFor(tail string, kind traffic.EmergencyKind) {
	if tail == "" {
		tail = playerTail
	}
	w.st.core.prio.setKind(tail, kind)
	w.Priority(tail, true)
}

// declareEmergency has one of ours declare an emergency of kind (level ""
// by the kind): squawk 7700, its MAYDAY or PAN PAN, approach's answer,
// and priority. Arrivals on their way in only, for now. quiet: nothing
// said (a scene saying its own words), the rest as ever.
func (cc *controlCenter) declareEmergency(it *controlled, kind traffic.EmergencyKind, level string, quiet bool) error {
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
	if it.view.Squawk != traffic.EmergencySquawk {
		it.preEmergencySquawk = it.view.Squawk
	}
	it.view.Squawk, it.emergencyLevel = traffic.EmergencySquawk, level
	pos, rwy, tail := it.atc, it.view.Runway, it.Tail
	it.mu.Unlock()
	if pos == "" {
		pos = traffic.PosApproach
	}
	station, _ := it.station(pos)
	cc.core.prio.setKind(tail, kind)
	cc.core.prio.set(tail, true)
	cc.log.printf("%-6s emergency: %s (%s), squawk %s, priority", tail, kind, level, traffic.EmergencySquawk)
	if quiet {
		return nil
	}
	it.say(traffic.DeclareEmergency(pos, station, tail, kind, level, rwy))
	it.say(traffic.AcknowledgeEmergency(pos, tail, level, rwy))
	// What the message should carry, asked and answered: its own
	// plausible persons on board and endurance (the app's player answers
	// with the real ones).
	it.mu.Lock()
	model := it.view.Model
	it.mu.Unlock()
	persons, endurance := plausibleOnBoard(model, tail)
	it.say(traffic.EmergencyQuestions(pos, tail))
	it.say(traffic.EmergencyDetails(pos, tail, persons, endurance, traffic.IntentionsLand(rwy)))
	return nil
}

// plausibleOnBoard is a cast aircraft's persons on board by its size
// (wake category) and an endurance of 1 to 3 hours, the same for one call
// sign every time.
func plausibleOnBoard(model, tail string) (int, time.Duration) {
	h := 0
	for _, c := range tail {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	persons := 120 + h%60 // a single aisle
	switch traffic.WakeFor(model).ICAO {
	case 'L':
		persons = 2 + h%6
	case 'H', 'J':
		persons = 240 + h%100
	}
	return persons, time.Hour + time.Duration(h%120)*time.Minute
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

// cancelEmergency has one of ours cancel its emergency: its call, ATC's
// answer with its code back, priority taken away.
func (cc *controlCenter) cancelEmergency(it *controlled) error {
	it.mu.Lock()
	level, squawk := it.emergencyLevel, it.preEmergencySquawk
	if level == "" {
		it.mu.Unlock()
		return errors.New("emergency: none to cancel")
	}
	it.emergencyLevel = ""
	if squawk != "" {
		it.view.Squawk = squawk
	}
	pos, tail := it.atc, it.Tail
	it.mu.Unlock()
	if pos == "" {
		pos = traffic.PosApproach
	}
	station, _ := it.station(pos)
	cc.core.prio.set(tail, false)
	cc.log.printf("%-6s emergency: cancelled, priority taken away", tail)
	it.say(traffic.CancelEmergency(pos, station, tail, level))
	it.say(traffic.AcknowledgeCancelEmergency(pos, tail, level, squawk))
	return nil
}

// registerEmergency serves POST /api/emergency ({"tail", "kind", "level",
// "on"}): one of ours declares an emergency of kind (on false: cancels
// it), as a scenario's "emergency" action does; a debug trigger for the
// rescue vehicles and the sequence around it.
func registerEmergency(mux *http.ServeMux, st *state) {
	mux.HandleFunc("POST /api/emergency", func(rw http.ResponseWriter, r *http.Request) {
		var req struct {
			Tail  string `json:"tail"`
			Kind  string `json:"kind"`
			Level string `json:"level"`
			On    *bool  `json:"on"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		st.mu.Lock()
		cc := st.control
		st.mu.Unlock()
		if cc == nil {
			http.Error(rw, "simulator not connected", http.StatusServiceUnavailable)
			return
		}
		it := cc.byTail(strings.ToUpper(strings.TrimSpace(req.Tail)))
		if it == nil {
			http.Error(rw, "no flight of ours "+req.Tail, http.StatusNotFound)
			return
		}
		var err error
		if req.On != nil && !*req.On {
			err = cc.cancelEmergency(it)
		} else {
			if req.Kind == "" {
				req.Kind = string(traffic.EmergencyEngineFailure)
			}
			err = cc.declareEmergency(it, traffic.EmergencyKind(req.Kind), req.Level, false)
		}
		if err != nil {
			http.Error(rw, err.Error(), http.StatusConflict)
			return
		}
		writeJSON(rw, map[string]any{"tail": it.Tail, "kind": req.Kind, "on": req.On == nil || *req.On})
	})
}
