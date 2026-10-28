package world

import (
	"maps"
	"slices"
	"strings"
	"time"

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

// staffedConflict looks at conflict c for aircraft on staffed stations: an
// alert for those in assist or safety net, kept by pair, and the safety net
// given the go (w.net) once its deadline passes. False when the AI has
// nothing to do in it: everything ours in it is staffed and no safety net
// is due.
func (w *conflictWatch) staffedConflict(now time.Time, c traffic.Conflict, suggest func(cs string) string) bool {
	staffed := map[string]string{}
	var lastPlayer time.Time
	others := false
	for _, cs := range []string{c.A, c.B} {
		it := w.s.cc.byTail(cs)
		if it == nil {
			continue
		}
		m := it.aiMode()
		if m == "" {
			others = true
			continue
		}
		staffed[cs] = m
		if at := it.playerAt.Load(); at != 0 && time.Unix(0, at).After(lastPlayer) {
			lastPlayer = time.Unix(0, at)
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
