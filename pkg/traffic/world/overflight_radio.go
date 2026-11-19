package world

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Overflights on the centre frequency (#1487), with
// OverflightTraffic.OverflightRadio. The overflights and transits are
// worked by the centre station whose sector holds them (traffic.PickStation
// over the focus airport's stations, PosCenter: the scenery's centre
// frequency, or a local station set with PUT /api/stations, an Area
// polygon for a sector): on entering the area its crew checks in ("Praha
// Radar, CSA123, flight level 370"; a climbing or descending transit
// "passing flight level 183, climbing flight level 240"); on leaving it or
// its sector, the station hands it to the next ("contact <the next
// sector's station>"; past the area: the next unit, not named), the crew
// reading back and checking in with a next station of ours. Virtual tracks
// (#1482) talk as the aircraft do. Said only on the frequency the user has
// tuned (COM1; with no user aircraft, all), and no more than
// CentreCallsPerMinute transmissions a minute on a centre frequency: a call
// over the cap waits for the next look.
//
// A centre station staffed by a player (#1096) is theirs: the crews still
// check in, the AI hands nobody off (unless AutoHandoff), and the player
// instructs the en-route aircraft with World.InstructEnroute (POST
// /api/enroute/instruct): a level, heading, speed or direct-to, read back
// and flown through its waypoint chain (SetRoute) or its virtual track,
// re-planned from where it is: no jump. An en-route arrival before its
// STAR is approach's (enroute_told.go, #1419).

const (
	// radioEvery: how often the crossing traffic's stations are looked at.
	radioEvery = 5 * time.Second
	// centreCallsPerMinute: the cap's default.
	centreCallsPerMinute = 6
	// instructHeadingFor: a heading is flown this long (at its speed, then
	// back onto the way; traffic.ResolvedRoute flies it for half its
	// look-ahead).
	instructHeadingFor = 30 * time.Minute
	// instructRestOf: a level or speed holds for the rest of the way.
	instructRestOf = 6 * time.Hour
	// climbingFpm: a vertical speed past this is a climb or descent said.
	climbingFpm = 300.0
)

// centreContact is the centre station a crossing flight talks to.
type centreContact struct {
	station traffic.Station
	// handed: told to contact the next unit (gone from ours).
	handed bool
}

// centreRadio is the overflights' radio state. s.mu held.
type centreRadio struct {
	at time.Time
	on map[string]*centreContact
	// sent: the transmissions on each centre frequency, for the cap.
	sent map[string][]time.Time
}

// crossingPose is where a crossing flight is now: its aircraft's, or its
// virtual track's.
type crossingPose struct {
	cs   string
	e    *enrouteAC
	a    traffic.TrackedAircraft
	vsFm float64
}

// radioTick lets the crossing traffic talk (scheduler goroutine).
func (s *scheduler) radioTick(now time.Time) {
	s.mu.Lock()
	if !s.over.radio || now.Sub(s.over.cr.at) < radioEvery {
		s.mu.Unlock()
		return
	}
	s.over.cr.at = now
	if s.over.cr.on == nil {
		s.over.cr.on, s.over.cr.sent = map[string]*centreContact{}, map[string][]time.Time{}
	}
	var list []crossingPose
	byID := map[uint32]*enrouteAC{}
	for cs, e := range s.enroute {
		if _, ok := s.over.crossing[cs]; !ok || e.gone {
			continue
		}
		switch {
		case e.vt != nil && !e.materialising:
			p := e.vt.Pose()
			list = append(list, crossingPose{cs: cs, e: e, vsFm: p.VSFpm, a: traffic.TrackedAircraft{Observation: traffic.Observation{
				Tail: cs, Position: p.Position, AltFt: p.AltFt, Heading: p.Heading, GroundKts: p.Kts}}})
		case e.objectID != 0:
			byID[e.objectID] = e
		}
	}
	// Those gone: off the frequency.
	for cs := range s.over.cr.on {
		if s.enroute[cs] == nil {
			delete(s.over.cr.on, cs)
		}
	}
	focus := append([]string(nil), s.focus...)
	s.mu.Unlock()
	if len(focus) == 0 {
		return
	}
	if len(byID) > 0 {
		for _, a := range s.cc.world.Aircraft() {
			if e := byID[a.ObjectID]; e != nil {
				list = append(list, crossingPose{cs: e.f.Callsign, e: e, a: a, vsFm: a.VSFpm})
			}
		}
	}
	c, ok := s.cc.world.Centre()
	if !ok {
		return
	}
	icao := focus[0]
	stations := s.cc.stations(icao)
	for _, p := range list {
		s.radioLook(now, icao, stations, c, p)
	}
}

// radioLook is one crossing flight's call due, if any.
func (s *scheduler) radioLook(now time.Time, icao string, stations []traffic.Station, c airport.LatLon, p crossingPose) {
	pos := p.a.Position
	in := calc.HaversineNM(c.Lat, c.Lon, pos.Lat, pos.Lon) <= overflightRadiusNM
	st, ok := traffic.PickStation(stations, traffic.PosCenter, traffic.Where{At: &pos})
	ok = ok && st.Freq != ""
	s.mu.Lock()
	on := s.over.cr.on[p.cs]
	s.mu.Unlock()
	switch {
	case on == nil && in && ok:
		// Into the area: its check-in.
		t := traffic.CheckIn(traffic.PosCenter, st.Name, p.cs, s.levelReport(icao, p), "")
		if s.centreSay(now, icao, st, t, 1) {
			s.mu.Lock()
			s.over.cr.on[p.cs] = &centreContact{station: st}
			s.mu.Unlock()
		}
	case on != nil && !on.handed && (!in || !ok || st.Name != on.station.Name || st.Freq != on.station.Freq):
		// Out of its sector or the area: handed on.
		if staffed, ok := s.cc.core.staffedAt(icao, traffic.PosCenter, on.station.Name); ok && !staffed.AutoHandoff {
			return // the player's to hand off
		}
		next := traffic.Station{Position: traffic.PosCenter}
		if in && ok {
			next = st
		}
		if next.Freq != "" && s.tunedTo(next.Freq) && !s.centreRoom(now, icao, next.Freq, 1) {
			return // its check-in there over the cap: the hand-off waits
		}
		t := traffic.Handoff(p.cs, traffic.PosCenter, traffic.PosCenter, next.Name, next.Freq)
		if !s.centreSay(now, icao, on.station, t, 2) {
			return
		}
		s.mu.Lock()
		on.handed = true
		s.mu.Unlock()
		if next.Freq != "" {
			// Checking in with the next sector's station.
			t := traffic.CheckIn(traffic.PosCenter, next.Name, p.cs, s.levelReport(icao, p), "")
			s.centreSay(now, icao, next, t, 1)
			s.mu.Lock()
			s.over.cr.on[p.cs] = &centreContact{station: next}
			s.mu.Unlock()
		}
	}
}

// levelReport is a crossing flight's level as its crew reports it: level,
// or passing one climbing or descending to its top (or the bottom of its
// descent) ahead.
func (s *scheduler) levelReport(icao string, p crossingPose) string {
	ta := s.cc.taOf(icao)
	at := traffic.LevelSaidAbove(math.Round(p.a.AltFt/100)*100, ta)
	if math.Abs(p.vsFm) < climbingFpm {
		return at
	}
	s.mu.Lock()
	route := p.e.route
	s.mu.Unlock()
	to := p.a.AltFt
	for _, w := range traffic.ProfileAhead(p.a.Position, route) {
		if p.vsFm > 0 {
			to = math.Max(to, w.AltFt)
		} else if w.AltFt > 0 {
			to = math.Min(to, w.AltFt)
		}
	}
	if math.Abs(to-p.a.AltFt) < 500 {
		return at
	}
	way := "climbing"
	if p.vsFm < 0 {
		way = "descending"
	}
	return "passing " + at + ", " + way + " " + traffic.LevelSaidAbove(to, ta)
}

// tunedTo reports whether the user listens on freq: COM1 on it ("127.00"
// is "127.000"), or no user aircraft (all heard).
func (s *scheduler) tunedTo(freq string) bool {
	p := s.cc.core.com1.Load()
	if p == nil || *p == "" {
		return true
	}
	a, err1 := strconv.ParseFloat(*p, 64)
	b, err2 := strconv.ParseFloat(freq, 64)
	if err1 != nil || err2 != nil {
		return *p == freq
	}
	return math.Abs(a-b) < 0.0025 // nearer than the 8.33 kHz channels
}

// centreSay sends t on station's frequency, counting n transmissions (a
// call and its readback) against the cap; false when over the cap (the
// call waits). Not tuned: not said, and taken as done.
func (s *scheduler) centreSay(now time.Time, icao string, st traffic.Station, t traffic.Transmission, n int) bool {
	if !s.tunedTo(st.Freq) {
		return true
	}
	// Checked at when it would go out: a busy frequency puts it later.
	from := now
	if c := s.cc.radio.ClearAt(icao, st.Freq); c.After(from) {
		from = c
	}
	if !s.centreRoom(from, icao, st.Freq, n) {
		return false
	}
	t.Frequency = st.Freq
	said := s.cc.radio.Transmit(icao, t)
	// Each counted when it goes out: the call when said, its readback
	// once the frequency is clear of what was booked with it (the end of
	// the readback, a little late rather than early).
	at := said.At
	if at.Before(now) {
		at = now
	}
	back := at
	if n > 1 {
		if c := s.cc.radio.ClearAt(icao, st.Freq); c.After(back) {
			back = c
		}
	}
	s.mu.Lock()
	for i := range n {
		x := at
		if i > 0 {
			x = back
		}
		s.over.cr.sent[st.Freq] = append(s.over.cr.sent[st.Freq], x)
	}
	s.mu.Unlock()
	return true
}

// centreRoom reports whether n more transmissions on freq stay within the
// cap for the minute to now: ours as counted, or all the radio has said or
// booked on it at icao (others on it too), whichever is more.
func (s *scheduler) centreRoom(now time.Time, icao, freq string, n int) bool {
	heard := 0
	for _, t := range s.cc.radio.Recent(icao, 0) {
		if t.Frequency == freq && now.Sub(t.At) < time.Minute {
			heard++
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	limit := s.over.callsPerMinute
	if limit <= 0 {
		limit = centreCallsPerMinute
	}
	sent := s.over.cr.sent[freq][:0]
	for _, x := range s.over.cr.sent[freq] {
		if now.Sub(x) < time.Minute {
			sent = append(sent, x)
		}
	}
	s.over.cr.sent[freq] = sent
	return max(len(sent), heard)+n <= limit
}

// EnrouteInstruction is a player's instruction to an en-route aircraft of
// ours, an overflight or transit (#1487):
// one of a level (AltFt, feet MSL), a true heading (HeadingDeg, with
// Heading set: north is 0), a speed (Kts, ground speed) or a direct-to
// (Direct, a position, or Fix, a named fix of its route). By is the
// player (network play, ?as=): they must staff the centre station working
// it; "" any player's staffing.
type EnrouteInstruction struct {
	Callsign   string          `json:"callsign"`
	AltFt      float64         `json:"altFt,omitempty"`
	HeadingDeg float64         `json:"headingDeg,omitempty"`
	Heading    bool            `json:"heading,omitempty"`
	Kts        float64         `json:"kts,omitempty"`
	Direct     *airport.LatLon `json:"direct,omitempty"`
	Fix        string          `json:"fix,omitempty"`
	By         string          `json:"by,omitempty"`
}

// ErrNotStaffed: the centre station working the aircraft is not the
// player's.
var ErrNotStaffed = errors.New("world: its station is not staffed by the player")

// ErrNotEnroute: no overflight or transit of ours by that call sign (an
// arrival before its STAR is approach's: #1419).
var ErrNotEnroute = errors.New("world: no en-route aircraft of ours by that call sign")

// InstructEnroute gives an en-route aircraft a player's instruction: said
// on its centre station's frequency (read back by its crew) and flown from
// where it is now; what was said.
func (w *World) InstructEnroute(in EnrouteInstruction) (traffic.Transmission, error) {
	w.st.mu.Lock()
	s := w.st.schedule
	w.st.mu.Unlock()
	if s == nil {
		return traffic.Transmission{}, errors.New("world: simulator not connected")
	}
	return s.instruct(in)
}

func (s *scheduler) instruct(in EnrouteInstruction) (traffic.Transmission, error) {
	cs := strings.ToUpper(strings.TrimSpace(in.Callsign))
	s.mu.Lock()
	e := s.enroute[cs]
	focus := append([]string(nil), s.focus...)
	var a traffic.TrackedAircraft
	virtual := e != nil && e.vt != nil && !e.materialising
	if virtual {
		p := e.vt.Pose()
		a = traffic.TrackedAircraft{Observation: traffic.Observation{Tail: cs, Position: p.Position, AltFt: p.AltFt, Heading: p.Heading, GroundKts: p.Kts}}
	}
	s.mu.Unlock()
	// An arrival before its STAR is approach's (enroute_told.go, #1419).
	if e == nil || !e.f.Overflight() || len(focus) == 0 || !virtual && e.objectID == 0 {
		return traffic.Transmission{}, ErrNotEnroute
	}
	if !virtual {
		found := false
		for _, x := range s.cc.world.Aircraft() {
			if x.ObjectID == e.objectID {
				a, found = x, true
			}
		}
		if !found {
			return traffic.Transmission{}, ErrNotEnroute
		}
	}
	icao := focus[0]
	pos := a.Position
	st, ok := traffic.PickStation(s.cc.stations(icao), traffic.PosCenter, traffic.Where{At: &pos})
	if !ok {
		return traffic.Transmission{}, ErrNotStaffed
	}
	staffed, ok := s.cc.core.staffedAt(icao, traffic.PosCenter, st.Name)
	if by := strings.ToLower(strings.TrimSpace(in.By)); !ok || by != "" && staffed.By != by {
		return traffic.Transmission{}, ErrNotStaffed
	}
	r := traffic.Resolution{Callsign: cs, ObjectID: e.objectID}
	look := instructRestOf
	switch {
	case in.AltFt > 0:
		r.Kind, r.AltFt = traffic.ResolveLevel, math.Round(in.AltFt/100)*100
	case in.Heading:
		r.Kind, r.HeadingDeg, look = traffic.ResolveHeading, math.Mod(in.HeadingDeg+360, 360), instructHeadingFor
	case in.Kts > 0:
		r.Kind, r.Kts = traffic.ResolveSpeed, in.Kts
	case in.Direct != nil || in.Fix != "":
		r.Kind, r.Fix, r.Direct = traffic.ResolveDirect, strings.ToUpper(in.Fix), in.Direct
		if r.Direct == nil {
			s.mu.Lock()
			for _, f := range e.fixes {
				if strings.EqualFold(f.Ident, in.Fix) {
					at := f.LatLon
					r.Direct = &at
				}
			}
			s.mu.Unlock()
			if r.Direct == nil {
				return traffic.Transmission{}, fmt.Errorf("world: %s is not a fix of %s's route", in.Fix, cs)
			}
		}
		if r.Fix == "" {
			r.Fix = "the point given"
		}
	default:
		return traffic.Transmission{}, errors.New("world: no level, heading, speed or direct-to given")
	}
	s.mu.Lock()
	route := e.route
	s.mu.Unlock()
	resolved := traffic.ResolvedRoute(route, a, r, look)
	start, wps, err := traffic.EnrouteStart(resolved)
	if err != nil {
		return traffic.Transmission{}, fmt.Errorf("world: %s: %w", cs, err)
	}
	if virtual {
		s.mu.Lock()
		if e.vt != nil && !e.materialising {
			// On from its pose now, as it flies (no jump).
			p := e.vt.Pose()
			start = p.InitPosition()
			e.vt = traffic.NewVirtualFlight(start, wps)
			e.route = resolved
		}
		s.mu.Unlock()
	} else {
		if err := s.cc.do(func() error { return s.cc.sim.SetRoute(e.objectID, wps) }); err != nil {
			return traffic.Transmission{}, fmt.Errorf("world: %s: %w", cs, err)
		}
		s.mu.Lock()
		e.route, e.waypoints = resolved, wps
		s.mu.Unlock()
	}
	tx := traffic.Resolved(traffic.PosCenter, r, a.AltFt, a.Heading, a.GroundKts, traffic.SaidWhere{TAFt: s.cc.taOf(icao), MagVar: s.cc.magVar(icao)})
	tx.Frequency = st.Freq
	s.cc.log.printf("%-6s en route: %s (the player on %s)", cs, r.Kind, st.Name)
	return s.cc.radio.Transmit(icao, tx), nil
}

// registerEnrouteInstruct serves POST /api/enroute/instruct (an
// EnrouteInstruction; ?as= the player): what was said.
func registerEnrouteInstruct(mux *http.ServeMux, w *World) {
	mux.HandleFunc("POST /api/enroute/instruct", func(rw http.ResponseWriter, r *http.Request) {
		var in EnrouteInstruction
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(rw, "an en-route instruction: "+err.Error(), http.StatusBadRequest)
			return
		}
		if as := positionOf(r); as != "" && as != "all" && in.By == "" {
			in.By = as
		}
		tx, err := w.InstructEnroute(in)
		switch {
		case errors.Is(err, ErrNotStaffed):
			http.Error(rw, err.Error(), http.StatusForbidden)
			return
		case errors.Is(err, ErrNotEnroute):
			http.Error(rw, err.Error(), http.StatusNotFound)
			return
		case err != nil:
			http.Error(rw, err.Error(), http.StatusConflict)
			return
		}
		writeJSON(rw, tx)
	})
}
