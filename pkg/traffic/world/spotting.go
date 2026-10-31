package world

import (
	"errors"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// The spotting director: left to itself the camera watches an airport from its
// spots. It picks an aircraft doing something and the spot to see it from, stays
// with that aircraft to the end of what it is doing, and moves on. It reads the
// scan, so it sees all the traffic at the airport: ours, other traffic and the
// player's aircraft alike.

// SpottingRequest starts the spotting director at ICAO. The filters are what it
// may pick, empty for anything: Kinds "ours", "other", "player"; Types ICAO type
// designators; Events "landing", "takeoff", "taxi". Share and Lock as on a spot.
type SpottingRequest struct {
	ICAO   string   `json:"icao"`
	Kinds  []string `json:"kinds,omitempty"`
	Types  []string `json:"types,omitempty"`
	Events []string `json:"events,omitempty"`
	Share  float64  `json:"share,omitempty"`
	Lock   bool     `json:"lock,omitempty"`
}

const (
	spottingEvery = time.Second
	// spotFinalM and spotClimbM: how far out before the threshold a landing
	// counts, and past the runway's end a take-off.
	spotFinalM = 6 * 1852.0
	spotClimbM = 3 * 1852.0
	// spotAlignDeg and spotAlignM: lined up with a runway, in the air.
	spotAlignDeg = 25.0
	spotAlignM   = 800.0
	// spotRollKts: on a runway at this speed it is rolling, not taxiing.
	spotRollKts = 30.0
	// spotTaxiKts: moving, on the ground.
	spotTaxiKts = 3.0
	// spotGrace: what it was doing has to have stopped this long, so a pause
	// in a taxi or a report that reads wrong for a moment does not cut away.
	spotGrace = 10 * time.Second
	// spotMinShot: no cut to another spot sooner; slow films, no jumping about.
	spotMinShot = 10 * time.Second
	// spotCloser: another spot is taken only this much closer than the one in use.
	spotCloser = 0.7
	// spotShotSec: a shot's length, taken again if the event runs past it.
	spotShotSec = 120.0
	// spotAirborneFt: lower than this it is on the ground, whatever it reports:
	// one placed by us reads as flying from the moment it lines up.
	spotAirborneFt = 40.0
	// spotFlew: on a runway this soon after flying it has landed.
	spotFlew = 2 * time.Minute
	// spotLeadSec: the spot is picked for where it will be, not where it is.
	spotLeadSec = 8.0
	// spotReachM and spotTaxiReachM: the furthest spot worth using.
	spotReachM     = 4000.0
	spotTaxiReachM = 1000.0
)

// spotted is what the director remembers of an aircraft between looks: AI on the
// ground report no speed, so it comes from their positions.
type spotted struct {
	at   time.Time
	pos  airport.LatLon
	kts  float64
	agl  float64
	flew time.Time // when last seen in the air
	// climbing: in the air, higher than at the last look.
	climbing bool
}

// spotter is the director's state, on the goroutine of its look (one at a time).
type spotter struct {
	req   SpottingRequest
	g     *airport.Graph
	spots []CameraSpot
	seen  map[uint32]spotted

	subject uint32 // object id; 0 none
	name    string // the subject as shown
	last    uint32 // the one before, to move on from
	event   string
	end     string // the runway end of a landing or a take-off
	lastIn  time.Time
	spot    CameraSpot // the one in use (Name "": none)
	cutAt   time.Time
}

// spotCut is a cut the director wants.
type spotCut struct {
	spot  string
	t     Traffic
	event string
}

// spotOnGround says t is on the ground.
func spotOnGround(t *Traffic) bool { return t.OnGround || t.AGL < spotAirborneFt }

// spotKind is what kind of traffic t is, as the filter names it.
func spotKind(t *Traffic) string {
	switch {
	case t.User:
		return "player"
	case t.Ours:
		return "ours"
	}
	return "other"
}

func (s *spotter) passes(t *Traffic, event string) bool {
	if len(s.req.Kinds) > 0 && !slices.Contains(s.req.Kinds, spotKind(t)) {
		return false
	}
	if len(s.req.Types) > 0 && !slices.ContainsFunc(s.req.Types, func(k string) bool { return strings.EqualFold(k, t.Type) }) {
		return false
	}
	return len(s.req.Events) == 0 || slices.Contains(s.req.Events, event)
}

// spotBearing is p's distance along and across a line from a at heading, meters.
func spotBearing(a, p airport.LatLon, heading float64) (along, cross float64) {
	d := calc.HaversineMeters(a.Lat, a.Lon, p.Lat, p.Lon)
	off := (calc.BearingDegrees(a.Lat, a.Lon, p.Lat, p.Lon) - heading) * math.Pi / 180
	return d * math.Cos(off), d * math.Sin(off)
}

func headingOff(a, b float64) float64 {
	return math.Abs(math.Mod(a-b+540, 360) - 180)
}

// eventOf says what t is doing at kts: "landing" or "takeoff" with the runway end,
// "taxi", or "" nothing to watch. was is what it was doing (a roll is either until
// it is known); h what the looks so far say about it.
func (s *spotter) eventOf(t *Traffic, h spotted, was string, now time.Time) (string, string) {
	pos := airport.LatLon{Lat: t.Latitude, Lon: t.Longitude}
	kts, ground := h.kts, spotOnGround(t)
	for _, r := range s.g.Layout.Runways {
		for _, e := range []airport.RunwayEnd{r.Primary, r.Secondary} {
			if headingOff(t.Heading, e.Heading) > spotAlignDeg {
				continue
			}
			along, cross := spotBearing(e.Threshold, pos, e.Heading)
			if ground {
				if math.Abs(cross) <= r.Width/2+10 && along >= -50 && along <= r.Length+50 && kts >= spotRollKts {
					// Just down it has landed; never seen in the air, it is taking
					// off (a speed from positions says too little to tell braking).
					if was == "landing" || was != "takeoff" && !h.flew.IsZero() && now.Sub(h.flew) < spotFlew {
						return "landing", e.Name
					}
					return "takeoff", e.Name
				}
				continue
			}
			if math.Abs(cross) > spotAlignM {
				continue
			}
			switch {
			case along < 0 && along >= -spotFinalM:
				return "landing", e.Name
			case along >= 0 && along <= r.Length && was != "takeoff" && (was == "landing" || !h.climbing):
				return "landing", e.Name // over the runway, in the flare
			case along >= 0 && along <= r.Length+spotClimbM && was != "landing":
				return "takeoff", e.Name
			}
		}
	}
	if ground && kts >= spotTaxiKts {
		return "taxi", ""
	}
	return "", ""
}

// spotFor is the spot to see an event from: the nearest of its kinds to where the
// aircraft will shortly be, within reach. ok false: none.
func (s *spotter) spotFor(t *Traffic, kts float64, event, end string) (CameraSpot, float64, bool) {
	lat, lon := calc.DisplaceByHeading(t.Latitude, t.Longitude, t.Heading, kts*0.514444*spotLeadSec)
	reach := spotReachM
	if event == "taxi" {
		reach = spotTaxiReachM
	}
	other := ""
	if r, _, ok := s.g.Layout.RunwayEnd(end); ok && end != "" {
		other = r.Primary.Name
		if other == end {
			other = r.Secondary.Name
		}
	}
	var best CameraSpot
	bestD := math.Inf(1)
	for _, c := range s.spots {
		use := c.Kind == "custom"
		switch event {
		case "landing":
			use = use || c.Runway == end && (c.Kind == "approach" || c.Kind == "threshold" || c.Kind == "touchdown")
		case "takeoff":
			use = use || (c.Runway == end || c.Runway == other) && (c.Kind == "threshold" || c.Kind == "touchdown" || c.Kind == "holding")
		case "taxi":
			use = use || c.Kind == "apron" || c.Kind == "holding"
		}
		if !use {
			continue
		}
		if d := calc.HaversineMeters(lat, lon, c.Lat, c.Lon); d < bestD && d <= reach {
			best, bestD = c, d
		}
	}
	return best, bestD, bestD <= reach
}

// look reads the traffic and decides: the subject, and a cut when one is due.
func (s *spotter) look(now time.Time, list []Traffic) *spotCut {
	seen := map[uint32]spotted{}
	for i := range list {
		t := &list[i]
		pos := airport.LatLon{Lat: t.Latitude, Lon: t.Longitude}
		kts := t.GroundKts
		was, ok := s.seen[t.ObjectID]
		if kts <= 0 && ok {
			kts = was.kts // the same scan again
			if dt := now.Sub(was.at).Seconds(); dt > 0 {
				kts = calc.HaversineMeters(was.pos.Lat, was.pos.Lon, pos.Lat, pos.Lon) / dt / 0.514444
			}
		}
		h := spotted{at: now, pos: pos, kts: kts, agl: t.AGL, flew: was.flew}
		if !spotOnGround(t) {
			h.flew = now
		}
		h.climbing = ok && !spotOnGround(t) && t.AGL > was.agl
		seen[t.ObjectID] = h
	}
	s.seen = seen

	var sub *Traffic
	for i := range list {
		if s.subject != 0 && list[i].ObjectID == s.subject {
			sub = &list[i]
		}
	}
	if sub != nil {
		if ev, end := s.eventOf(sub, s.seen[sub.ObjectID], s.event, now); ev == s.event {
			s.end, s.lastIn = end, now
		} else if now.Sub(s.lastIn) > spotGrace {
			sub = nil
		}
	}
	// A taxi gives way to a landing or a take-off; those are seen through.
	if sub == nil || s.event == "taxi" {
		if p, ev, end := s.pick(list, now, sub != nil); p != nil {
			sub = p
			s.last, s.subject, s.event, s.end, s.lastIn = s.subject, p.ObjectID, ev, end, now
			s.name = p.Tail
			if p.User && s.name == "" {
				s.name = "player"
			}
			s.spot = CameraSpot{}
		} else if sub == nil {
			if s.subject != 0 {
				s.last = s.subject
			}
			s.subject, s.name, s.event, s.end = 0, "", "", ""
			s.spot = CameraSpot{}
			return nil
		}
	}

	spot, d, ok := s.spotFor(sub, s.seen[sub.ObjectID].kts, s.event, s.end)
	if !ok {
		return nil
	}
	in := now.Sub(s.cutAt)
	switch {
	case s.spot.Name == "":
	case s.spot.Name == spot.Name:
		if in.Seconds() < spotShotSec-2 {
			return nil // the shot runs on
		}
	default:
		if in < spotMinShot || d > spotCloser*calc.HaversineMeters(sub.Latitude, sub.Longitude, s.spot.Lat, s.spot.Lon) {
			return nil
		}
	}
	s.spot, s.cutAt = spot, now
	return &spotCut{spot: spot.Name, t: *sub, event: s.event}
}

// pick is the aircraft to watch next: one landing or taking off before one
// taxiing, the nearest to its spot, not the last one while there is another.
// onlyRunway: looking past a taxi, for a landing or a take-off only.
func (s *spotter) pick(list []Traffic, now time.Time, onlyRunway bool) (*Traffic, string, string) {
	var best *Traffic
	var bestEv, bestEnd string
	bestScore := math.Inf(1)
	for i := range list {
		t := &list[i]
		ev, end := s.eventOf(t, s.seen[t.ObjectID], "", now)
		if ev == "" || !s.passes(t, ev) || onlyRunway && ev == "taxi" {
			continue
		}
		_, d, ok := s.spotFor(t, s.seen[t.ObjectID].kts, ev, end)
		if !ok {
			continue
		}
		score := d
		if ev == "taxi" {
			score += 2 * spotReachM
		}
		if t.ObjectID == s.last || t.ObjectID == s.subject {
			score += 4 * spotReachM
		}
		if score < bestScore {
			best, bestEv, bestEnd, bestScore = t, ev, end, score
		}
	}
	return best, bestEv, bestEnd
}

// startSpotting starts the director at req.ICAO. A scene playing keeps the camera.
func (m *cameraMan) startSpotting(req SpottingRequest) error {
	if m.dir == nil {
		return errors.New("no camera on this connection")
	}
	m.mu.Lock()
	scene := m.mode == "scene"
	m.mu.Unlock()
	if scene {
		return errors.New("a scene is playing")
	}
	g, err := m.cc.graph(strings.ToUpper(req.ICAO))
	if err != nil {
		return err
	}
	spots := m.cc.spots(g)
	if len(spots) == 0 {
		return errors.New("no spots at " + g.Layout.ICAO)
	}
	if err := m.setMode("spotting", 0); err != nil {
		return err
	}
	m.mu.Lock()
	m.spotter = &spotter{req: req, g: g, spots: spots}
	m.spotEvent, m.spotAt = "", time.Time{}
	m.mu.Unlock()
	return nil
}

// spottingLook is the director's look, off the connection's goroutine.
func (m *cameraMan) spottingLook(now time.Time) {
	m.mu.Lock()
	s := m.spotter
	m.mu.Unlock()
	sc := m.cc.scanNow.Load()
	if s == nil || sc == nil {
		return
	}
	cut := s.look(sc.at, sc.list) // by the scan's clock: speeds come from its positions
	m.mu.Lock()
	if m.spotter != s {
		m.mu.Unlock()
		return // stopped meanwhile
	}
	m.spotEvent, m.subject = s.event, s.name
	if s.spot.Name == "" {
		m.shot = "" // no spot yet for this one
	}
	m.mu.Unlock()
	if cut == nil {
		return
	}
	t := &cut.t
	size := t.Span * 1.05
	if size <= 0 {
		size = nominalSize.Length
	}
	shot, err := m.cc.spotShot(s.g, cut.spot, t.ObjectID, t.User, size, spotShotSec, spotOptions{s.req.Share, s.req.Lock})
	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		m.err = err.Error()
		return
	}
	if m.spotter != s {
		return
	}
	m.shot = shot.Name()
	m.dir.Play(shot)
}

// StartSpotting starts the spotting director; POST /api/camera with mode off or any other
// camera mode ends it. Refused with no camera on this connection, with a scene
// playing, or with no spots at the airport.
func (w *World) StartSpotting(req SpottingRequest) error {
	_, err := w.Do("POST", "/api/camera/spotting", req)
	return err
}
