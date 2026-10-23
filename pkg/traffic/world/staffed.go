package world

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
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
func (it *controlled) giveClearance() error {
	it.mu.Lock()
	due, arr := it.clearanceDue, it.arrivalDue
	it.clearanceDue, it.arrivalDue = nil, nil
	it.mu.Unlock()
	switch {
	case due != nil:
		it.clearance(traffic.ClearedDeparture(it.Tail, *due))
	case arr != nil:
		it.firstContact(*arr) // an arrival's STAR (#1096)
	default:
		return errors.New("no clearance asked for")
	}
	it.cc.changed("control")
	return nil
}

// amendClearance changes the departure clearance not given yet (#1164):
// re-planned off a.Runway with the SID a.SID (else the one off it from the
// same fix, as a runway change picks it), a.LevelFt the initial climb. The
// clearance then says what was applied.
func (it *controlled) amendClearance(a ClearanceAmend) error {
	cc := it.cc
	it.mu.Lock()
	due := it.clearanceDue
	rwy, oldSID := it.view.Runway, it.view.Procedure
	it.mu.Unlock()
	if due == nil {
		return errors.New("no clearance asked for")
	}
	dc := *due
	if a.Runway == "" {
		a.Runway = rwy
	}
	if a.Runway != rwy || (a.SID != "" && !strings.EqualFold(a.SID, oldSID)) {
		if cc.procedures == nil {
			return errors.New("procedures not available")
		}
		p, ok := cc.procedures(it.graph.Layout.ICAO)
		if !ok {
			return fmt.Errorf("procedures of %s not loaded (yet)", it.graph.Layout.ICAO)
		}
		name := a.SID
		if name == "" {
			if s := sameFix(p.SIDsFor(a.Runway), oldSID); s != nil {
				name = s.Name
			}
		}
		var pts []airport.NavPoint
		sid := ""
		if a.SID != "" || len(p.SIDsFor(a.Runway)) > 0 {
			got, n, _, err := cc.procedureFor(it.graph, SpawnRequest{Kind: "departure", Runway: a.Runway, ProcName: name})
			if err != nil {
				return err
			}
			pts, sid = got, n
		}
		if err := cc.do(func() error { return it.dep.ChangeRunway(a.Runway, "", pts) }); err != nil {
			return err
		}
		lim := cc.limitsOf(it.graph)
		it.mu.Lock()
		it.view.Runway, it.view.Procedure = a.Runway, sid
		it.keepSpot()
		it.procSaid, it.fixes = "", nil
		if sid != "" {
			it.procSaid = procedureSaid(sid, nil)
			for _, p := range pts {
				if p.Ident != "" {
					it.fixes = append(it.fixes, airFix{Ident: p.Ident, LatLon: p.Position})
				}
			}
		}
		it.climbSaid = initialClimbSaid(lim, sid)
		it.setRoute()
		dc.Runway, dc.SID, dc.Level = a.Runway, it.procSaid, it.climbSaid
		it.mu.Unlock()
		cc.log.printf("%-6s clearance amended: runway %s, SID %s", it.Tail, a.Runway, orNone(sid))
	}
	if a.LevelFt > 0 {
		if !it.dep.LimitClimb(a.LevelFt) {
			return errors.New("handed over already: no initial climb to amend")
		}
		said := traffic.LevelSaidAbove(a.LevelFt, cc.taOf(it.ICAO))
		it.mu.Lock()
		it.afterDepFt, it.climbSaid = a.LevelFt, said // tower keeps it, not one of its own
		dc.Level = said
		it.mu.Unlock()
		cc.log.printf("%-6s clearance amended: initial climb %s", it.Tail, said)
	}
	it.mu.Lock()
	if it.clearanceDue != nil {
		it.clearanceDue = &dc
	}
	it.mu.Unlock()
	return nil
}
