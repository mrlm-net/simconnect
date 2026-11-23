package world

import (
	"maps"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// What the AI does on a staffed station (StaffedStation.Mode, #1096 step
// 3). Never a routine clearance, vector, speed or level: those are the
// player's in every mode. The modes differ in conflicts only.
const (
	// ModeAssist (the default, ""): a conflict of an aircraft on the
	// station is an alert, with the resolution the AI would give as a
	// suggestion; the AI does nothing.
	ModeAssist = "assist"
	// ModeSilent: nothing from the AI, not even an alert.
	ModeSilent = "silent"
	// ModeSafetyNet: an alert, and once the player's deadline passes with
	// the conflict still predicted, the AI resolves it itself, marked as
	// the safety net in the log, the resolutions and on the radio.
	ModeSafetyNet = "safetynet"
)

// stationMode is mode as a StaffedStation's Mode: lower case, "safety-net"
// and "safety_net" as "safetynet", "" and anything unknown as assist.
func stationMode(mode string) string {
	m := strings.NewReplacer("-", "", "_", "", " ", "").Replace(strings.ToLower(strings.TrimSpace(mode)))
	switch m {
	case ModeSilent, ModeSafetyNet:
		return m
	}
	return ModeAssist
}

// The safety net's deadline: the player has at least netGrace from the
// alert, and netAfterPlayer from their last instruction to the aircraft
// (time for it to work), but never less than netWithin before the
// predicted loss of separation.
const (
	netWithin      = 2 * time.Minute
	netGrace       = 30 * time.Second
	netAfterPlayer = time.Minute
)

// aiMode is what the AI does for it: "" when it is not the player's (the
// AI works it), else the mode of its staffed station. Taken over by its
// own manual toggle or a clearance from the user off the staffed stations,
// silent: the per-aircraft takeover has no mode.
func (it *controlled) aiMode() string {
	if !it.gates.Load() {
		return ""
	}
	it.mu.Lock()
	pos, manual := it.atc, it.manualSet
	it.mu.Unlock()
	if manual || pos == "" {
		return ModeSilent
	}
	at, _ := it.station(pos)
	s, ok := it.cc.core.staffedAt(it.ICAO, pos, at)
	if !ok {
		return ModeSilent
	}
	return stationMode(s.Mode)
}

// StaffedAlert is a predicted conflict of an aircraft on a staffed station
// (assist and safety net): the player's to resolve. Suggested is what the
// AI would say ("" when it has nothing to offer); Deadline is when the
// safety net acts (zero unless a safety net station is in it); SafetyNet
// that it has.
type StaffedAlert struct {
	Since     time.Time         `json:"since"`
	Conflict  traffic.Conflict  `json:"conflict"`
	Staffed   map[string]string `json:"staffed"` // callsign: its mode
	Suggested string            `json:"suggested,omitempty"`
	Deadline  time.Time         `json:"deadline,omitzero"`
	SafetyNet bool              `json:"safetyNet,omitempty"`
	// lossBy: the earliest netWithin before a predicted loss: on approach
	// the loss predicted slides on as they slow, a deadline taken from the
	// latest look with it, never reached.
	lossBy time.Time
}

// alertKey is the pair of c, in either order.
func alertKey(c traffic.Conflict) string {
	if c.B < c.A {
		return c.B + "/" + c.A
	}
	return c.A + "/" + c.B
}

// enrouteStaff is whose an en route aircraft of ours is (enrouteStaffed):
// the staffed station's airport, position and mode; mode "" the AI's.
type enrouteStaff struct {
	icao     string
	pos      traffic.Position
	mode     string
	playerAt time.Time // the player's last instruction to it
}

// enrouteStaffed is who works e, en route at at. It has no controlled item,
// so neither gates nor aiMode know it (#1662: the conflict watch steered an
// overflight off the descent the player had given it a second before). An
// overflight or transit is the player's while the centre station working
// it, the one InstructEnroute asks, is staffed; an arrival before its STAR
// once approach has told it something, while approach is staffed
// (enroute_told.go: untold, it flies as before).
func (s *scheduler) enrouteStaffed(e *enrouteAC, at airport.LatLon) enrouteStaff {
	s.mu.Lock()
	out := enrouteStaff{playerAt: e.playerAt}
	told := ""
	if e.told != nil {
		told = e.told.icao
	}
	focus := slices.Clone(s.focus)
	s.mu.Unlock()
	if len(s.cc.core.staff.list()) == 0 {
		return out
	}
	switch {
	case told != "":
		if st, ok := s.cc.approachStaffed(told); ok {
			out.icao, out.pos, out.mode = told, traffic.PosApproach, stationMode(st.Mode)
		}
	case e.f.Overflight():
		if icao, _, st, ok := s.centreStaffed(focus, at); ok {
			out.icao, out.pos, out.mode = icao, traffic.PosCenter, stationMode(st.Mode)
		}
	}
	return out
}

// centreStaffed is the staffed centre station working an en route
// aircraft at at: each focus airport's centre station there in turn, the
// first one a player staffs (#1684: the first focus airport's only, and a
// player on another's centre was not seen).
func (s *scheduler) centreStaffed(focus []string, at airport.LatLon) (string, traffic.Station, StaffedStation, bool) {
	for _, icao := range focus {
		on, ok := traffic.PickStation(s.cc.stations(icao), traffic.PosCenter, traffic.Where{At: &at})
		if !ok {
			continue
		}
		if st, ok := s.cc.core.staffedAt(icao, traffic.PosCenter, on.Name); ok {
			return icao, on, st, true
		}
	}
	return "", traffic.Station{}, StaffedStation{}, false
}

// staffing is whose a is: by its en route flight where it is one of those
// (e), else by its controlled item, as aiMode has it.
func (w *conflictWatch) staffing(a traffic.TrackedAircraft, e *enrouteAC) (st enrouteStaff, ours bool) {
	if e != nil {
		return w.s.enrouteStaffed(e, a.Position), true
	}
	it := w.s.cc.byTail(a.Tail)
	if it == nil {
		return st, false
	}
	st.mode = it.aiMode()
	if at := it.playerAt.Load(); at != 0 {
		st.playerAt = time.Unix(0, at)
	}
	if st.mode != "" {
		it.mu.Lock()
		st.icao, st.pos = it.ICAO, it.atc
		it.mu.Unlock()
	}
	return st, true
}

// maySteerEnroute reports whether the AI may steer e, en route at at: not
// the player's, or the safety net's due on it (mayAct for those en route).
func (w *conflictWatch) maySteerEnroute(e *enrouteAC, at airport.LatLon) bool {
	if w.s.enrouteStaffed(e, at).mode == "" {
		return true
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.net[e.f.Callsign]
}

// stopIsPlayers reports whether e, stopped for traffic by the watch (st),
// is no longer the watch's to clear on: the player has instructed it
// since, or it is the player's and the safety net did not stop it.
func (w *conflictWatch) stopIsPlayers(e *enrouteAC, at airport.LatLon, st stoppedLevel) bool {
	who := w.s.enrouteStaffed(e, at)
	if !who.playerAt.IsZero() && !who.playerAt.Before(st.since) {
		return true // their clearance stands
	}
	return who.mode != "" && !st.net
}

// departurePlayers reports whether departure it's climb is the player's
// (#1096): under their control, or with tower or departure while departure
// is staffed, gates not set yet (checkStaffed).
func (it *controlled) departurePlayers() bool {
	if it.gates.Load() {
		return true
	}
	it.mu.Lock()
	pos := it.atc
	it.mu.Unlock()
	return (pos == traffic.PosTower || pos == traffic.PosDeparture) && it.staffedNow(traffic.PosDeparture)
}

// holdStops leaves the departures the watch stopped to the player while
// they are the player's (#1665): the stop stays, its level held on in the
// route (the change's end climbed on by itself), and theirs names them, not
// to be cleared on. Off the staffed station the AI clears them on as
// before; a climb from the player since ends the stop. The safety net's
// own stop too: the net resolves, the climb after it is the player's.
func (w *conflictWatch) holdStops(aircraft []traffic.TrackedAircraft) (theirs map[string]bool) {
	w.mu.Lock()
	stops := maps.Clone(w.stopped)
	w.mu.Unlock()
	theirs = map[string]bool{}
	for _, a := range aircraft {
		st, ok := stops[a.Tail]
		if !ok || !a.Ours || a.OnGround || st.pos != traffic.PosDeparture {
			continue
		}
		it := w.s.cc.byTail(a.Tail)
		if it == nil || it.dep == nil || it.object() != a.ObjectID {
			continue
		}
		it.mu.Lock()
		climbed := it.climbClearedAt
		it.mu.Unlock()
		if !climbed.IsZero() && !climbed.Before(st.since) {
			w.mu.Lock()
			delete(w.stopped, a.Tail)
			w.mu.Unlock()
			w.s.cc.log.printf("%-6s conflict stop: over, the player cleared its climb", a.Tail)
			continue
		}
		if !it.departurePlayers() {
			continue
		}
		theirs[a.Tail] = true
		if st.held || st.atFt <= 0 {
			continue
		}
		route := stopHeld(a, st, it.dep.ClimbPlan(a.Position))
		if err := w.s.cc.do(func() error { return it.dep.Reroute(route) }); err != nil {
			w.s.cc.log.printf("%-6s conflict stop: not held for the player: %v", a.Tail, err)
			continue
		}
		it.mu.Lock()
		it.clearedFt, it.higherAt, it.levelSet = st.atFt, time.Time{}, true
		it.mu.Unlock()
		w.mu.Lock()
		if cur, ok := w.stopped[a.Tail]; ok && cur.since.Equal(st.since) {
			cur.held = true
			w.stopped[a.Tail] = cur
		}
		w.mu.Unlock()
		w.s.cc.log.printf("%-6s conflict stop: the player's, held at %.0f ft", a.Tail, st.atFt)
	}
	return theirs
}

// stopHeld is a's route ahead with the level of its stop st to the end.
func stopHeld(a traffic.TrackedAircraft, st stoppedLevel, plan []traffic.RoutePoint) []traffic.RoutePoint {
	out := []traffic.RoutePoint{{Position: a.Position, AltFt: a.AltFt, Kts: a.GroundKts}}
	if len(plan) == 0 {
		return out
	}
	for _, p := range traffic.ProfileAhead(a.Position, plan) {
		if p.AltFt == 0 || st.climb && p.AltFt > st.atFt || !st.climb && p.AltFt < st.atFt {
			p.AltFt = st.atFt
		}
		out = append(out, p)
	}
	return out
}

// staffedConflict looks at conflict c for aircraft on staffed stations: an
// alert for those in assist or safety net, kept by pair, and the safety net
// given the go (w.net) once its deadline passes. False when the AI has
// nothing to do in it: everything ours in it is staffed and no safety net
// is due. who is whose each call sign is (staffing).
func (w *conflictWatch) staffedConflict(now time.Time, c traffic.Conflict, who func(cs string) (enrouteStaff, bool), suggest func(cs string) string) bool {
	staffed := map[string]string{}
	var lastPlayer time.Time
	others := false
	for _, cs := range []string{c.A, c.B} {
		st, ours := who(cs)
		if !ours {
			continue
		}
		if st.mode == "" {
			others = true
			continue
		}
		staffed[cs] = st.mode
		if st.playerAt.After(lastPlayer) {
			lastPlayer = st.playerAt
		}
	}
	if len(staffed) == 0 {
		return true
	}
	pair := alertKey(c)
	alerted := false
	net := false
	for _, m := range staffed {
		alerted = alerted || m != ModeSilent
		net = net || m == ModeSafetyNet
	}
	if !alerted {
		return others
	}
	w.mu.Lock()
	if w.alerts == nil {
		w.alerts = map[string]*StaffedAlert{}
	}
	a := w.alerts[pair]
	first := a == nil
	if first {
		a = &StaffedAlert{Since: now}
		w.alerts[pair] = a
	}
	a.Conflict, a.Staffed = c, staffed
	acted := a.SafetyNet
	w.mu.Unlock()
	cs := slices.Sorted(maps.Keys(staffed))
	if first {
		w.s.cc.log.printf("alert: %s and %s lose separation in %s at %.1f NM, %.0f ft: %s the player's", c.A, c.B, c.In.Round(time.Second), c.LossNM, c.LossFt, strings.Join(cs, ", "))
	}
	var said string
	for _, x := range cs {
		if s := suggest(x); s != "" {
			said = s
			break
		}
	}
	var deadline time.Time
	if net {
		w.mu.Lock()
		if d := now.Add(c.In - netWithin); a.lossBy.IsZero() || d.Before(a.lossBy) {
			a.lossBy = d
		}
		deadline = a.Since.Add(netGrace)
		if a.lossBy.After(deadline) {
			deadline = a.lossBy
		}
		w.mu.Unlock()
		if !lastPlayer.IsZero() && lastPlayer.Add(netAfterPlayer).After(deadline) {
			deadline = lastPlayer.Add(netAfterPlayer)
		}
	}
	due := net && (acted || !now.Before(deadline))
	w.mu.Lock()
	a.Suggested, a.Deadline = said, deadline
	if due {
		a.SafetyNet = true
		for x, m := range staffed {
			if m == ModeSafetyNet {
				w.net[x] = true
			}
		}
	}
	w.mu.Unlock()
	if due && !acted {
		w.s.cc.log.printf("alert: %s and %s not resolved by the player in time: the safety net acts", c.A, c.B)
	}
	return others || due
}

// arrivalSuggestion is what resolveArrivals would say first to cs, the
// trailer of conflict c on its STAR ("" when it is not): its descent
// stopped above the other well out, else its speed for spacing, as much
// as the speed takes of the time to lose (the rest by vectors).
func (w *conflictWatch) arrivalSuggestion(c traffic.Conflict, aircraft []traffic.TrackedAircraft, cs string) string {
	_, trailer := w.trailerOf(c)
	if trailer == nil || trailer.it.Tail != cs {
		return ""
	}
	var me, them *traffic.TrackedAircraft
	for i := range aircraft {
		switch aircraft[i].Tail {
		case cs:
			me = &aircraft[i]
		case other(c, cs):
			them = &aircraft[i]
		}
	}
	ta := w.s.cc.taOf(trailer.it.ICAO)
	if me != nil && them != nil && trailer.e.DistanceToGoNM > arrivalLevelFromNM {
		if level := math.Ceil((them.AltFt+arrivalLevelAboveFt)/500) * 500; me.AltFt >= level-200 {
			return w.worded(trailer.it.ICAO, traffic.StopDescent(traffic.PosApproach, cs, level, them.Tail, ta))
		}
		if hold := math.Floor(me.AltFt/500) * 500; me.AltFt >= them.AltFt-arrivalLevelWithFt && hold >= arrivalDescendMinFt {
			return w.worded(trailer.it.ICAO, traffic.StopDescent(traffic.PosApproach, cs, hold, them.Tail, ta))
		}
	}
	kts := min(spacingKts(me), traffic.ProcedureSpeedKts)
	floor := traffic.ProcedureApproachSpeedKts
	if traffic.ProfileFor(trailer.it.view.Model).Category == traffic.CategoryTurboprop {
		floor = min(floor, traffic.MinProcedureSpeedTurbopropKts)
	}
	a := traffic.PlanAbsorption(spacingDelay(c, me), spacingWithinNM(c, me), kts, floor)
	if a.SpeedKts <= 0 || a.SpeedKts >= kts {
		return ""
	}
	return w.worded(trailer.it.ICAO, traffic.Sequenced(cs, trailer.e.Number, 0, traffic.Absorption{SpeedKts: a.SpeedKts}))
}

// worded is tx's text as the radio would say it at icao (its callsign
// spoken, the airport's phraseology), as World.Phrase gives it.
func (w *conflictWatch) worded(icao string, tx traffic.Transmission) string {
	if w.s.cc.radio == nil {
		return tx.Text
	}
	said, _, _ := w.s.cc.radio.Worded(icao, tx)
	return said.Text
}

// mayAct reports whether the AI may give it a conflict instruction: not
// the player's, or the safety net's due on it.
func (w *conflictWatch) mayAct(it *controlled) bool {
	if !it.gates.Load() {
		return true
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.net[it.Tail]
}

// marked is tx said by the safety net when it is cs's (Params "by":
// "safetynet"), so a game shows who said it.
func (w *conflictWatch) marked(cs string, tx traffic.Transmission) traffic.Transmission {
	w.mu.Lock()
	net := w.net[cs]
	w.mu.Unlock()
	if !net {
		return tx
	}
	p := maps.Clone(tx.Params)
	if p == nil {
		p = map[string]string{}
	}
	p["by"] = ModeSafetyNet
	tx.Params = p
	return tx
}

// Alerts are the conflicts of aircraft on staffed stations now (assist and
// safety net), the earliest first.
func (w *World) Alerts() []StaffedAlert {
	w.st.mu.Lock()
	cw := w.st.conflicts
	w.st.mu.Unlock()
	if cw == nil {
		return []StaffedAlert{}
	}
	return cw.alertList()
}

func (w *conflictWatch) alertList() []StaffedAlert {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]StaffedAlert, 0, len(w.alerts))
	for _, a := range w.alerts {
		x := *a
		x.Staffed = maps.Clone(a.Staffed)
		out = append(out, x)
	}
	slices.SortFunc(out, func(a, b StaffedAlert) int { return a.Since.Compare(b.Since) })
	return out
}
