package world

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
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
		// Handed to the simulator after take-off (done), a departure is
		// still the staffed departure's to climb on (#1096).
		climbing := it.dep != nil && pos == traffic.PosDeparture && it.view.State == traffic.TaxiComplete.String()
		it.mu.Unlock()
		if done && !climbing || explicit || pos == "" {
			continue
		}
		station, _ := it.station(pos)
		_, want := cc.core.staffedAt(it.ICAO, pos, station)
		any = any || want
		if climbing {
			it.mu.Lock()
			has := slices.Contains(it.view.Actions, "climb")
			if want && !has {
				it.view.Actions = append(it.view.Actions, "climb") // ?level=feet
			} else if !want && has {
				it.view.Actions = slices.DeleteFunc(it.view.Actions, func(a string) bool { return a == "climb" })
			}
			it.mu.Unlock()
		}
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
		go it.releaseLimitHold()
	}
	cc.staffedAny = any
	cc.crewAsksHigher(items, now)
	cc.crewAtLimit(items)
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
		it.climbSaid, it.clearedFt = initialClimbSaid(lim, sid), lim.InitialClimbFor(sid)
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
		it.afterDepFt, it.climbSaid, it.clearedFt = a.LevelFt, said, a.LevelFt // tower keeps it, not one of its own
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

// climbTo clears a departure handed over on to ft (the player's "climb",
// #1096): said against the transition altitude, flown from where it is.
func (it *controlled) climbTo(ft float64) error {
	it.mu.Lock()
	pos, alt, id, done := it.view.Position, it.heightFt, it.objectID, it.dep != nil && it.view.State == traffic.TaxiComplete.String()
	it.mu.Unlock()
	if !done {
		return errors.New("not airborne and handed over yet")
	}
	if it.graph != nil {
		alt += it.graph.Layout.Altitude / 0.3048
	}
	for _, a := range it.cc.world.Aircraft() {
		if a.ObjectID == id && id != 0 {
			pos, alt = a.Position, a.AltFt
		}
	}
	if ft <= alt+100 {
		return fmt.Errorf("at %.0f ft: %.0f ft is no climb", alt, ft)
	}
	if err := it.cc.do(func() error { return it.dep.ClimbTo(pos, alt, ft) }); err != nil {
		return err
	}
	it.mu.Lock()
	it.clearedFt, it.higherAt = ft, time.Time{}
	it.mu.Unlock()
	ta := it.cc.taOf(it.ICAO)
	it.say(traffic.Climb(traffic.PosDeparture, it.Tail, ft, ta))
	it.cc.log.printf("%-6s departure: climb to %s (the player)", it.Tail, traffic.LevelSaidAbove(ft, ta))
	return nil
}

// higherAfter: level at its cleared level this long on a staffed
// departure, the crew asks for its cruising level; again every
// higherAgain until cleared.
const (
	higherAfter = 30 * time.Second
	higherAgain = 3 * time.Minute
)

// crewAsksHigher has the crews of our departures held at their cleared
// level on a staffed departure ask for higher (#1096).
func (cc *controlCenter) crewAsksHigher(items []*controlled, now time.Time) {
	for _, it := range items {
		if it.dep == nil || !it.gates.Load() {
			continue
		}
		it.mu.Lock()
		on := it.atc == traffic.PosDeparture && it.view.State == traffic.TaxiComplete.String()
		cleared, cruise, at, id, asked := it.clearedFt, it.cruiseFt, it.view.AltFt, it.objectID, it.higherAt
		level := on && cleared > 0 && !it.levelSet
		if level {
			it.levelSet = true
		}
		it.mu.Unlock()
		if level {
			// Handed to the simulator: it levels there, not on to its
			// cruise as the simulator's own flight would.
			go it.climbOnTo(cleared)
		}
		if !on || cleared <= 0 || cruise <= cleared {
			continue
		}
		for _, a := range cc.world.Aircraft() {
			if a.ObjectID == id && id != 0 {
				at = a.AltFt
			}
		}
		it.mu.Lock()
		switch {
		case at < cleared-300: // still climbing to it
			it.higherAt = time.Time{}
		case asked.IsZero(): // level: asks a while on
			it.higherAt = now.Add(higherAfter)
		case !now.Before(asked):
			it.higherAt = now.Add(higherAgain)
		}
		ask := !asked.IsZero() && !now.Before(asked) && at >= cleared-300
		it.mu.Unlock()
		if !ask {
			continue
		}
		it.say(traffic.RequestClimb(traffic.PosDeparture, it.Tail, traffic.LevelSaidAbove(cruise, cc.taOf(it.ICAO))))
	}
}

// staffedNow reports whether pos's station where it is now is staffed:
// the look itself, not gates, which checkStaffed sets a moment later.
func (it *controlled) staffedNow(pos traffic.Position) bool {
	at, _ := it.station(pos)
	_, staffed := it.cc.core.staffedAt(it.ICAO, pos, at)
	return staffed
}

// limitWithin: an arrival this many minutes from its clearance limit
// without an approach clearance asks for one and plans to hold there
// (FAA AIP ENR 1.5: within 3 minutes of a clearance limit with no
// clearance beyond it, slow to hold there, in a standard pattern on the
// course it approached it by, and request further clearance).
const limitWithin = 3.0

// crewAtLimit has the crews of our arrivals on a staffed approach without
// an approach clearance ask for one near the end of their STAR, their
// clearance limit, and hold there until the player gives it (#1096).
func (cc *controlCenter) crewAtLimit(items []*controlled) {
	for _, it := range items {
		if it.arr == nil || it.circuit != nil || !it.gates.Load() || it.arr.State() != traffic.ArrivalApproaching {
			continue
		}
		it.mu.Lock()
		skip := it.manualSet || it.approachCleared || it.limitHeld || it.arrivalDue != nil || it.atc != traffic.PosApproach
		star, _, _ := strings.Cut(it.view.Procedure, " → ")
		rwy, fixes, id := it.view.Runway, it.fixes, it.objectID
		it.mu.Unlock()
		if skip {
			continue
		}
		fix, ok := cc.starEnd(it.ICAO, rwy, star, fixes)
		if !ok {
			continue
		}
		var a traffic.TrackedAircraft
		for _, o := range cc.world.Aircraft() {
			if o.ObjectID == id && id != 0 {
				a = o
			}
		}
		if a.ObjectID == 0 || a.GroundKts < 60 {
			continue
		}
		nm := calc.HaversineNM(a.Position.Lat, a.Position.Lon, fix.Lat, fix.Lon)
		if nm/a.GroundKts*60 > limitWithin {
			continue
		}
		h := traffic.Hold{Ident: fix.Ident, Fix: fix.LatLon, InboundTrue: calc.BearingDegrees(a.Position.Lat, a.Position.Lon, fix.Lat, fix.Lon)}
		alt := math.Round(a.AltFt/1000) * 1000
		it.mu.Lock()
		it.limitHeld = true
		it.mu.Unlock()
		it.say(traffic.RequestApproachClearance(traffic.PosApproach, it.Tail, fix.Ident))
		// On the connection's goroutine (this is its tick): not waited for.
		go func(it *controlled, ident string) {
			if err := cc.do(func() error { _, err := it.arr.EnterHold(h, alt); return err }); err != nil {
				cc.log.printf("%-6s at its clearance limit %s: no hold: %v", it.Tail, ident, err)
				return
			}
			cc.log.printf("%-6s no approach clearance: holds at %s, %.0f ft, its clearance limit", it.Tail, ident, alt)
		}(it, fix.Ident)
	}
}

// starEnd is the last fix of star (with its transition to runway): the
// clearance limit of an arrival cleared that STAR, where it is among fixes.
func (cc *controlCenter) starEnd(icao, runway, star string, fixes []airFix) (airFix, bool) {
	if cc.procedures == nil || star == "" {
		return airFix{}, false
	}
	p, ok := cc.procedures(icao)
	if !ok {
		return airFix{}, false
	}
	for _, s := range p.STARsFor(runway) {
		if s.Name != star {
			continue
		}
		legs := slices.Clone(s.Legs)
		for _, t := range s.RunwayTransitions {
			if strings.TrimLeft(t.Runway, "0") == strings.TrimLeft(runway, "0") || t.Runway == "ALL" {
				legs = append(legs, t.Legs...)
				break
			}
		}
		last := ""
		for _, l := range legs {
			if l.HasFix() {
				last = l.Fix
			}
		}
		for i := len(fixes) - 1; i >= 0; i-- {
			if fixes[i].Ident == last {
				return fixes[i], true
			}
		}
	}
	return airFix{}, false
}

// releaseLimitHold takes it out of the hold at its clearance limit (back
// to the AI, which clears its approach itself).
func (it *controlled) releaseLimitHold() {
	it.mu.Lock()
	held := it.limitHeld
	it.limitHeld = false
	it.mu.Unlock()
	if !held || it.arr == nil {
		return
	}
	if _, _, holding := it.arr.Holding(); holding {
		if err := it.cc.do(it.arr.LeaveHold); err != nil {
			it.cc.log.printf("%-6s leaving the hold at its clearance limit: %v", it.Tail, err)
		}
	}
}
