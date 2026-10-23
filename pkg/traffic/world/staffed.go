package world

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Staffed stations (#1096): a human controller works chosen stations of an
// airport (an ATC game: Vector, any game on the core). On those the AI
// gives nothing: an aircraft on a staffed station is under the player's
// control (manual, as the per-aircraft toggle makes it), its crew calling
// on its own and flying its last clearance; everything else stays
// automatic. An aircraft's own manual toggle still wins.

// StaffedStation is a station a player works: a whole position ("tower",
// "ground", "approach", …) or one station of it by name ("Ruzyne Ground
// North"); several make a band-box. Mode is what the AI does there
// (assist, silent, safetynet: #1096's later steps; "" assist).
type StaffedStation struct {
	ICAO        string `json:"icao"`
	Position    string `json:"position,omitempty"`
	Station     string `json:"station,omitempty"`
	Mode        string `json:"mode,omitempty"`
	AutoHandoff bool   `json:"autoHandoff,omitempty"`
}

// staffing is the stations staffed, by airport.
type staffing struct {
	mu  sync.Mutex
	set []StaffedStation
}

func (s *staffing) list() []StaffedStation {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]StaffedStation(nil), s.set...)
}

// SetStaffed sets the stations players work (none: all automatic). The
// aircraft on them are the player's from the next look; those on a
// station no longer staffed go back to the AI from their current state.
func (w *World) SetStaffed(list []StaffedStation) {
	for i := range list {
		list[i].ICAO = strings.ToUpper(strings.TrimSpace(list[i].ICAO))
		list[i].Position = strings.ToLower(strings.TrimSpace(list[i].Position))
	}
	w.st.core.staff.mu.Lock()
	w.st.core.staff.set = append([]StaffedStation(nil), list...)
	w.st.core.staff.mu.Unlock()
	w.st.core.log.printf("staffed stations: %d", len(list))
}

// Staffed are the stations players work.
func (w *World) Staffed() []StaffedStation { return w.st.core.staff.list() }

// staffedAt reports whether position pos (and its station name, as said)
// at icao is staffed.
func (k *core) staffedAt(icao string, pos traffic.Position, station string) (StaffedStation, bool) {
	for _, s := range k.staff.list() {
		if s.ICAO != icao {
			continue
		}
		if s.Station != "" && strings.EqualFold(s.Station, station) {
			return s, true
		}
		if s.Station == "" && s.Position != "" && s.Position == string(pos) {
			return s, true
		}
	}
	return StaffedStation{}, false
}

// staffEvery: how often who is on a staffed station is looked at.
const staffEvery = time.Second

// checkStaffed puts the aircraft on a staffed station under the player's
// control and gives those leaving one back to the AI, unless their own
// manual toggle (manualSet) decides.
func (cc *controlCenter) checkStaffed(now time.Time) {
	if now.Sub(cc.staffedAt) < staffEvery {
		return
	}
	cc.staffedAt = now
	if len(cc.core.staff.list()) == 0 && !cc.staffedAny {
		return
	}
	cc.mu.Lock()
	items := make([]*controlled, 0, len(cc.items))
	for _, it := range cc.items {
		items = append(items, it)
	}
	cc.mu.Unlock()
	any := false
	for _, it := range items {
		it.mu.Lock()
		pos, explicit, done := it.atc, it.manualSet, it.view.Done
		it.mu.Unlock()
		if done || explicit || pos == "" {
			continue
		}
		station, _ := it.station(pos)
		_, want := cc.core.staffedAt(it.ICAO, pos, station)
		any = any || want
		if it.gates.Load() == want {
			continue
		}
		it.gates.Store(want)
		it.mu.Lock()
		it.view.Manual = want
		req := it.request
		it.mu.Unlock()
		if want {
			cc.log.printf("%-6s on a staffed station (%s): the player's", it.Tail, station)
			continue
		}
		cc.log.printf("%-6s off the staffed stations: automatic", it.Tail)
		if req != "" {
			it.askGround(req) // a request still waiting is answered
		}
		it.giveClearance() // a clearance still waiting too (#1156)
	}
	cc.staffedAny = any
}

// registerStaffed serves GET /api/stations/staffed (the list) and PUT
// /api/stations/staffed (a []StaffedStation, replacing it).
func registerStaffed(mux *http.ServeMux, w *World) {
	mux.HandleFunc("GET /api/stations/staffed", func(rw http.ResponseWriter, r *http.Request) {
		writeJSON(rw, w.Staffed())
	})
	mux.HandleFunc("PUT /api/stations/staffed", func(rw http.ResponseWriter, r *http.Request) {
		var list []StaffedStation
		if err := json.NewDecoder(r.Body).Decode(&list); err != nil {
			http.Error(rw, "a list of staffed stations: "+err.Error(), http.StatusBadRequest)
			return
		}
		w.SetStaffed(list)
		writeJSON(rw, w.Staffed())
	})
}

// giveClearance gives the departure clearance it asked for on a staffed
// delivery (#1156): the player's action "clearance", or the World once
// delivery is no longer staffed.
// amendClearance changes a held departure clearance before it is given
// (#1164): a different runway, a different SID off it, a lower initial
// climb. Empty values leave that part of the clearance as filed.
//
// A DELIVERY CONTROLLER IS NOT A TAPE RECORDER. The clearance may go off
// another runway, with the SID that serves it, or with a lower initial
// climb than the SID's own top to keep a departure under arriving traffic.
//
// THE AIRCRAFT AND NOT ONLY THE WORDS. ChangeRunway re-plans the taxi and
// the departure route and LimitClimb caps the climb, so what is said is
// what is flown; saying a runway the aeroplane is not going to would be
// worse than not offering the choice at all.
//
// Refusals are the engine's: no such SID for that runway, a runway that
// publishes nothing, too late to change runway (it is lined up).
func (it *controlled) amendClearance(runway, sid, level string) error {
	if runway == "" && sid == "" && level == "" {
		return nil
	}
	if it.dep == nil {
		return errors.New("only a departure has a departure clearance")
	}
	it.mu.Lock()
	due, had, g := it.clearanceDue, it.view.Procedure, it.graph
	if runway == "" {
		runway = it.view.Runway
	}
	it.mu.Unlock()
	if due == nil {
		return errors.New("no clearance asked for")
	}
	if g == nil {
		return errors.New("no airport")
	}
	amended := *due
	// The runway, and the SID off it: named, or the one off the new runway
	// nearest the fix it had (what changeDepartureRunway picks).
	if runway != "" || sid != "" {
		name := sid
		if name == "" && had != "" && it.cc.procedures != nil {
			if p, ok := it.cc.procedures(g.Layout.ICAO); ok {
				if x := sameFix(p.SIDsFor(runway), had); x != nil {
					name = x.Name
				}
			}
		}
		pts, said, _, err := it.cc.procedureFor(g, SpawnRequest{Kind: "departure", Runway: runway, ProcName: name})
		if err != nil {
			return err
		}
		if err := it.cc.do(func() error { return it.dep.ChangeRunway(runway, "", pts) }); err != nil {
			return err
		}
		it.mu.Lock()
		it.view.Runway, it.view.Procedure = runway, said
		it.procSaid = procedureSaid(said, nil)
		amended.Params[traffic.ParamRunway], amended.Params[traffic.ParamSID] = runway, it.procSaid
		it.mu.Unlock()
	}
	// A lower initial climb than the SID's: a restriction, not a re-route.
	if level != "" {
		ft, err := strconv.ParseFloat(level, 64)
		if err != nil || ft <= 0 {
			return errors.New("level must be an altitude in feet")
		}
		if !it.dep.LimitClimb(ft) {
			return errors.New("the climb cannot be limited now")
		}
		said := traffic.LevelSaidAbove(ft, it.cc.limitsOf(g).TransitionAltitudeFt)
		it.mu.Lock()
		it.climbSaid = said
		amended.Params[traffic.ParamLevel] = said
		it.mu.Unlock()
	}
	// Said again from the amended parameters, so the words match the plan.
	said := traffic.Say(amended)
	it.mu.Lock()
	it.clearanceDue = &said
	it.mu.Unlock()
	return nil
}

func (it *controlled) giveClearance() error {
	it.mu.Lock()
	due := it.clearanceDue
	it.clearanceDue = nil
	it.mu.Unlock()
	if due == nil {
		return errors.New("no clearance asked for")
	}
	it.clearance(*due)
	it.cc.changed("control")
	return nil
}
