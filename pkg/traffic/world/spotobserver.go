package world

import (
	"math"
	"slices"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/camera"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// The observer: with SpottingRequest.TravelMaxM set, the spotting director
// is one drone that moves between its spots instead of cutting across the
// airport. A cut far away makes the simulator load the scenery and models
// there, and they pop in after it; flown there, they load along the way.
//
//   - Every shot starts from where the camera is. The first of a spotting is
//     a cut from wherever the camera was.
//   - A cut goes at most CutM: the area is loaded, it reads as a new angle.
//   - A spot within TravelMaxM is flown to (camera.Spot.TravelTo), the spot
//     shot joined to the travel; farther is not gone to.
//   - Subjects near the drone come first, a landing or a take-off before a
//     taxi still; a landing is flown to early enough to see it touch down.
//     An aircraft inbound to a runway whose spots are out of reach pulls
//     the drone there, a spot within reach at a time (a hop), filming it
//     on the way; with no spot nearer in reach, a point in the air towards
//     it (a waypoint).
//   - With nothing to film it looks round where it is, at a parked aircraft
//     (now and then the player's), not somewhere else.

const (
	// DefaultSpotCutM is how far a cut goes with the observer, CutM not set.
	DefaultSpotCutM = 300.0
	// spotTravelSpeedMin and spotTravelSpeedMax keep the drone slow.
	spotTravelSpeedMin = 10.0
	spotTravelSpeedMax = 20.0
	// spotEarlySec: a landing is gone to this long before the travel to its
	// spot would be too late, and spotEarlyMin before its touchdown at the
	// least, when the drone is on another aircraft.
	spotEarlySec = 30.0
	spotEarlyMin = 75.0
	// spotTakeoffTravelSec: a take-off is flown to only this far: it is
	// over soon.
	spotTakeoffTravelSec = 45.0
	// spotKindAfter: a landing's spots by preference, metres added to the
	// way to them: the touchdown, then the threshold, then the approach.
	spotKindAfter = 300.0
	// spotLookEvery: with nothing to film, a look round this often;
	// spotPlayerEvery: the player's aircraft is looked at this often at
	// most.
	spotLookEvery   = 40 * time.Second
	spotPlayerEvery = 5 * time.Minute
	// spotLookAt: a look round at least this long; spotLookSweep the turn
	// when there is nothing to look at.
	spotLookAt    = 12 * time.Second
	spotLookSweep = 30.0
	// spotPullM: an aircraft lined up this far out pulls the drone to its
	// runway; spotPullFt below this height above the field. spotHopGain:
	// a hop brings the drone at least this much nearer.
	spotPullM   = 12 * 1852.0
	spotPullFt  = 5000.0
	spotHopGain = 200.0
	// spotPullLead: the drone leaves for an inbound aircraft's spot in
	// reach this long before it would be too late; spotTaxiQuiet: no travel
	// for a taxi with an aircraft inbound this soon.
	spotPullLead  = 60.0
	spotTaxiQuiet = 300.0
	// spotBusyAfter: after a travel or a look round, the decisions wait
	// this much longer (the shot starts at the next frame, holds a moment).
	spotBusyAfter = time.Second + camera.CutSettle
)

// observer is the observer's state, in the spotter.
type observer struct {
	cutM, travelMaxM, speed float64
	// pose: where the camera is, once a shot of this spotting played
	// (havePose).
	pose     camera.Pose
	havePose bool
	played   bool
	// busy: a travel or a look round plays until then.
	busy time.Time
	// aim: the subject's aim, for its shots one after another.
	aim *camera.Aim
	// lookAt: the last look round; looked what it turned to; playerAt
	// when the player's aircraft was last looked at.
	lookAt   time.Time
	looked   uint32
	playerAt time.Time
	// aimAt: when the aim was last given a shot.
	aimAt time.Time
	// hop: the spot in use is on the way to the subject's runway.
	hop bool
	// nextIn: seconds to the threshold of the aircraft inbound soonest (+Inf
	// none), at this look.
	nextIn float64
}

// spotLook is a look round where the camera is: at an aircraft (obj, at
// pos) or, none, a slow sweep.
type spotLook struct {
	obj   uint32
	user  bool
	name  string
	lat   float64
	lon   float64
	altM  float64
	size  float64
	sweep bool
}

// observerFor is the observer of req (TravelMaxM > 0), its defaults set.
func observerFor(req SpottingRequest) (observer, bool) {
	if req.TravelMaxM <= 0 {
		return observer{}, false
	}
	o := observer{cutM: req.CutM, travelMaxM: req.TravelMaxM, speed: req.TravelSpeed}
	if o.cutM <= 0 {
		o.cutM = DefaultSpotCutM
	}
	o.cutM = min(o.cutM, o.travelMaxM)
	if o.speed <= 0 {
		o.speed = camera.DefaultTravelSpeed
	}
	o.speed = max(spotTravelSpeedMin, min(spotTravelSpeedMax, o.speed))
	return o, true
}

// eyeTo is how far the camera is from p, metres on the ground.
func (o *observer) eyeTo(lat, lon float64) float64 {
	return calc.HaversineMeters(o.pose.Eye.Lat, o.pose.Eye.Lon, lat, lon)
}

// travelSec is how long the drone takes over d metres (0: a cut).
func (o *observer) travelSec(d float64) float64 {
	if !o.havePose || d <= o.cutM {
		return 0
	}
	return d/o.speed + camera.TravelRamp.Seconds()
}

// spotStart is where a spot's camera starts on t: on a rail, abeam it.
func spotStart(c CameraSpot, t *Traffic) (lat, lon float64) {
	if c.RailLat == 0 && c.RailLon == 0 {
		return c.Lat, c.Lon
	}
	cos := math.Cos(c.Lat * math.Pi / 180)
	re, rn := (c.RailLon-c.Lon)*cos, c.RailLat-c.Lat
	te, tn := (t.Longitude-c.Lon)*cos, t.Latitude-c.Lat
	l2 := re*re + rn*rn
	if l2 == 0 {
		return c.Lat, c.Lon
	}
	f := max(0, min(1, (te*re+tn*rn)/l2))
	return c.Lat + rn*f, c.Lon + re*f/cos
}

// untilTouchdown is how long t takes to the threshold of end, seconds: 0
// over the runway or on it.
func (s *spotter) untilTouchdown(t *Traffic, end string, kts float64) float64 {
	_, e, ok := s.g.Layout.RunwayEnd(end)
	if !ok || spotOnGround(t) {
		return 0
	}
	along, _ := spotBearing(e.Threshold, airport.LatLon{Lat: t.Latitude, Lon: t.Longitude}, e.Heading)
	if along >= 0 || kts <= 0 {
		return 0
	}
	return -along / (kts * 0.514444)
}

// spotNear is the spot to see t's event from, as the observer goes: of the
// event's kinds, within reach of the aircraft and of the drone, and near
// both; for a landing one it reaches before the touchdown, the touchdown
// spot first. Its score is the metres to it from the aircraft and the
// drone; ok false: none.
func (s *spotter) spotNear(t *Traffic, kts float64, event, end string) (CameraSpot, float64, bool) {
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
	tta := s.untilTouchdown(t, end, kts)
	var best CameraSpot
	bestScore := math.Inf(1)
	for _, c := range s.spots {
		use, after := c.Kind == "custom", 0.0
		switch event {
		case "landing":
			use = use || c.Runway == end && (c.Kind == "approach" || c.Kind == "threshold" || c.Kind == "touchdown")
			switch c.Kind {
			case "threshold":
				after = spotKindAfter
			case "approach":
				after = 2 * spotKindAfter
			}
		case "takeoff":
			use = use || (c.Runway == end || c.Runway == other) && (c.Kind == "threshold" || c.Kind == "touchdown" || c.Kind == "holding")
		case "taxi":
			use = use || c.Kind == "apron" || c.Kind == "holding"
		}
		if !use {
			continue
		}
		d := calc.HaversineMeters(lat, lon, c.Lat, c.Lon)
		if d > reach {
			continue
		}
		way := 0.0
		if s.havePose {
			slat, slon := spotStart(c, t)
			if cut := s.eyeTo(slat, slon); cut <= s.cutM {
				way = cut
			} else if way = s.eyeTo(c.Lat, c.Lon); way > s.travelMaxM {
				continue // too far to go
			}
		}
		sec := s.travelSec(way)
		if event == "landing" && sec > 0 && sec > tta {
			continue // there too late
		}
		if event == "takeoff" && sec > spotTakeoffTravelSec {
			continue
		}
		if score := d + way + after; score < bestScore {
			best, bestScore = c, score
		}
	}
	return best, bestScore, !math.IsInf(bestScore, 1)
}

// pickNear is the aircraft to watch next as the observer goes: a landing
// or a take-off before a taxi, the nearest to the drone, not the last one
// while there is another. busy: the drone is on another aircraft (cur
// doing what it does), so only a landing that cannot wait, or a take-off
// over a taxi, takes it away.
func (s *spotter) pickNear(list []Traffic, now time.Time, cur *Traffic) (*Traffic, string, string, CameraSpot) {
	var best *Traffic
	var bestEv, bestEnd string
	var bestSpot CameraSpot
	bestScore := math.Inf(1)
	for i := range list {
		t := &list[i]
		ev, end := s.eventOf(t, s.seen[t.ObjectID], "", now)
		if ev == "" || !s.passes(t, ev) {
			continue
		}
		if cur != nil && (ev == "taxi" || t.ObjectID == cur.ObjectID || ev == "takeoff" && s.event != "taxi") {
			continue
		}
		kts := s.seen[t.ObjectID].kts
		spot, score, ok := s.spotNear(t, kts, ev, end)
		if !ok {
			continue
		}
		if ev == "taxi" && s.nextIn < spotTaxiQuiet {
			if slat, slon := spotStart(spot, t); s.travelSec(s.eyeTo(slat, slon)) > 0 {
				continue // no flying off with a landing soon
			}
		}
		if cur != nil && ev == "landing" {
			way := s.eyeTo(spot.Lat, spot.Lon)
			if s.untilTouchdown(t, end, kts) > max(spotEarlyMin, s.travelSec(way)+spotEarlySec) {
				continue // not yet: the drone stays on what it films
			}
		}
		if ev == "taxi" {
			score += 2 * spotReachM
		}
		if t.ObjectID == s.last || t.ObjectID == s.subject {
			score += 4 * spotReachM
		}
		if score < bestScore {
			best, bestEv, bestEnd, bestSpot, bestScore = t, ev, end, spot, score
		}
	}
	return best, bestEv, bestEnd, bestSpot
}

// observe is look for the observer: the spot of a subject is kept to the
// end of what it does (taken again, joined, as a shot runs out), a travel
// or a look round plays out before anything else is decided.
func (s *spotter) observe(now time.Time, list []Traffic) *spotCut {
	sub := s.subjectIn(now, list)
	if now.Before(s.busy) {
		return nil
	}
	s.nextIn = math.Inf(1)
	if s.havePose {
		for i := range list {
			if _, tta, ok := s.inbound(&list[i]); ok && s.passes(&list[i], "landing") {
				s.nextIn = min(s.nextIn, tta)
			}
		}
	}
	// A taxi, or a take-off once flying, gives way to a landing that
	// cannot wait; a taxi to a take-off. An aircraft inbound out of reach
	// comes before a taxi: the drone is on its way to the runway.
	if sub == nil || s.event == "taxi" || s.event == "takeoff" && !spotOnGround(sub) {
		p, ev, end, spot := s.pickNear(list, now, sub)
		if p == nil || ev == "taxi" {
			if c := s.pull(now, list); c != nil {
				return c
			}
		}
		if p != nil {
			s.last, s.subject, s.event, s.end, s.lastIn = s.subject, p.ObjectID, ev, end, now
			s.name = p.Tail
			if p.User && s.name == "" {
				s.name = "player"
			}
			s.spot, s.cutAt, s.hop = spot, now, false
			slat, slon := spotStart(spot, p)
			travel := s.havePose && s.eyeTo(slat, slon) > s.cutM
			return &spotCut{spot: spot.Name, t: *p, event: ev, travel: travel}
		}
		if sub == nil {
			if s.subject != 0 {
				s.last = s.subject
			}
			s.subject, s.name, s.event, s.end = 0, "", "", ""
			s.spot, s.hop = CameraSpot{}, false
			return s.lookRound(now, list)
		}
	}
	if s.hop && sub != nil && s.event == "landing" {
		// On the way: to its spot once in reach, else a hop on.
		if spot, _, ok := s.spotNear(sub, s.seen[sub.ObjectID].kts, "landing", s.end); ok && spot.Name != s.spot.Name {
			s.hop, s.spot, s.cutAt = false, spot, now
			slat, slon := spotStart(spot, sub)
			return &spotCut{spot: spot.Name, t: *sub, event: s.event, travel: s.eyeTo(slat, slon) > s.cutM}
		}
		if c := s.pull(now, list); c != nil {
			return c
		}
	}
	if s.spot.Name == "" || now.Sub(s.cutAt).Seconds() < spotShotSec-2 {
		return nil // the shot runs on
	}
	s.cutAt = now
	return &spotCut{spot: s.spot.Name, t: *sub, event: s.event, again: true}
}

// inbound says t is lined up with a runway end, in the air within
// spotPullM of its threshold and low enough to be landing there: the end
// and how long to the threshold, seconds.
func (s *spotter) inbound(t *Traffic) (string, float64, bool) {
	kts := s.seen[t.ObjectID].kts
	if spotOnGround(t) || t.AGL > spotPullFt || kts <= 0 {
		return "", 0, false
	}
	pos := airport.LatLon{Lat: t.Latitude, Lon: t.Longitude}
	for _, r := range s.g.Layout.Runways {
		for _, e := range []airport.RunwayEnd{r.Primary, r.Secondary} {
			if headingOff(t.Heading, e.Heading) > spotAlignDeg {
				continue
			}
			along, cross := spotBearing(e.Threshold, pos, e.Heading)
			if along < 0 && along >= -spotPullM && math.Abs(cross) <= 2*spotAlignM {
				return e.Name, -along / (kts * 0.514444), true
			}
		}
	}
	return "", 0, false
}

// pull is a hop towards the runway of the aircraft inbound soonest, when
// its spots are out of the drone's reach: to the spot within reach
// nearest its touchdown spot, aimed at it on the way. nil: none, or no hop
// gets nearer.
func (s *spotter) pull(now time.Time, list []Traffic) *spotCut {
	if !s.havePose {
		return nil
	}
	var in *Traffic
	var end string
	soonest := math.Inf(1)
	for i := range list {
		t := &list[i]
		if e, tta, ok := s.inbound(t); ok && tta < soonest && s.passes(t, "landing") {
			in, end, soonest = t, e, tta
		}
	}
	if in == nil {
		return nil
	}
	// Where it is to be seen from: its touchdown spot, else the threshold.
	var goal CameraSpot
	for _, c := range s.spots {
		if c.Runway == end && (c.Kind == "touchdown" || c.Kind == "threshold" && goal.Kind != "touchdown") {
			goal = c
		}
	}
	if goal.Name == "" || s.eyeTo(goal.Lat, goal.Lon) <= s.cutM || goal.Name == s.spot.Name {
		return nil // a cut away, or there
	}
	var hop CameraSpot
	left := s.eyeTo(goal.Lat, goal.Lon) - spotHopGain
	if left+spotHopGain <= s.travelMaxM {
		// In reach: flown to in time, early enough to see it come in.
		if soonest > s.travelSec(left+spotHopGain)+spotPullLead {
			return nil
		}
		hop, left = goal, 0
	}
	for _, c := range s.spots {
		if hop.Name != "" {
			break
		}
		if c.Name == s.spot.Name || s.eyeTo(c.Lat, c.Lon) > s.travelMaxM {
			continue
		}
		if d := calc.HaversineMeters(c.Lat, c.Lon, goal.Lat, goal.Lon); d < left {
			hop, left = c, d
		}
	}
	if hop.Name == "" {
		hop = s.waypoint(goal)
	}
	s.last, s.subject, s.event, s.end, s.lastIn = s.subject, in.ObjectID, "landing", end, now
	s.name = in.Tail
	if in.User && s.name == "" {
		s.name = "player"
	}
	s.spot, s.cutAt, s.hop = hop, now, true
	slat, slon := spotStart(hop, in)
	return &spotCut{spot: hop.Name, at: &hop, t: *in, event: "landing", travel: s.eyeTo(slat, slon) > s.cutM}
}

// waypoint is a point in the air travelMaxM from the drone towards goal
// (less a little, so it stays in reach), as high as the higher of the two
// and a margin: no spot in reach is nearer.
func (s *spotter) waypoint(goal CameraSpot) CameraSpot {
	e := s.pose.Eye
	h := calc.BearingDegrees(e.Lat, e.Lon, goal.Lat, goal.Lon)
	lat, lon := calc.DisplaceByHeading(e.Lat, e.Lon, h, 0.95*s.travelMaxM)
	return CameraSpot{Name: "waypoint:" + goal.Name, Kind: "waypoint", Lat: lat, Lon: lon, AltM: max(e.AltM, goal.AltM) + camera.DefaultTravelMargin}
}

// lookRound is a look round where the camera is, when one is due: at the
// player's aircraft now and then, else the nearest aircraft parked near by
// not looked at last, else a sweep.
func (s *spotter) lookRound(now time.Time, list []Traffic) *spotCut {
	if !s.havePose || now.Sub(s.lookAt) < spotLookEvery {
		return nil
	}
	s.lookAt = now
	var best *Traffic
	bestD := math.Inf(1)
	for i := range list {
		t := &list[i]
		d := s.eyeTo(t.Latitude, t.Longitude)
		if d > s.travelMaxM || !spotOnGround(t) || len(s.req.Kinds) > 0 && !slices.Contains(s.req.Kinds, spotKind(t)) {
			continue
		}
		if t.User {
			if now.Sub(s.playerAt) >= spotPlayerEvery {
				best, bestD = t, -1 // the player's first, now and then
			}
			continue
		}
		if s.seen[t.ObjectID].kts < spotTaxiKts && t.ObjectID != s.looked && d < bestD {
			best, bestD = t, d
		}
	}
	if best == nil {
		return &spotCut{look: &spotLook{sweep: true}}
	}
	if best.User {
		s.playerAt = now
	}
	s.looked = best.ObjectID
	size := best.Span * 1.05
	if size <= 0 {
		size = nominalSize.Length
	}
	name := best.Tail
	if best.User && name == "" {
		name = "player"
	}
	return &spotCut{look: &spotLook{obj: best.ObjectID, user: best.User, name: name, lat: best.Latitude, lon: best.Longitude, altM: best.Alt*0.3048 + spotLookUpM, size: size}}
}

// spotLookUpM: a look round aims this far above an aircraft's reference
// point.
const spotLookUpM = 2.0

// placedTarget is the point p looks at in the world: an aircraft's from the
// scan (false: not found).
func placedTarget(p camera.Pose, list []Traffic) (camera.Point, bool) {
	if p.Target.Frame == camera.World {
		return p.Target, true
	}
	if p.Target.Frame != camera.Object {
		return camera.Point{}, false
	}
	lat, lon, alt, ok := camera.Placed(p.Target, func(o uint32) (camera.ObjectPose, bool) {
		for i := range list {
			t := &list[i]
			if o == 0 && t.User || o != 0 && t.ObjectID == o {
				return camera.ObjectPose{Lat: t.Latitude, Lon: t.Longitude, AltM: t.Alt * 0.3048, Heading: t.Heading}, true
			}
		}
		return camera.ObjectPose{}, false
	})
	return camera.At(lat, lon, alt), ok
}

// observerShots are the shots of a cut the observer decided: a cut, the
// spot again, a travel and the spot joined to it, or a look round; the
// move ("cut", "travel", "join", "look"); and how long they keep the drone
// busy (0: not).
func (m *cameraMan) observerShots(s *spotter, cut *spotCut, list []Traffic) ([]camera.Shot, string, time.Duration) {
	if cut.look != nil {
		return m.lookShots(s, cut.look, list)
	}
	t := &cut.t
	var spot CameraSpot
	for _, c := range s.spots {
		if c.Name == cut.spot {
			spot = c
		}
	}
	if cut.at != nil {
		spot = *cut.at
	}
	if spot.Name == "" {
		return nil, "", 0
	}
	cs := spot.camera()
	if s.req.Share > 0 {
		cs.Share = s.req.Share
	}
	cs.LockAim = s.req.Lock
	size := t.Span * 1.05
	if size <= 0 {
		size = nominalSize.Length
	}
	id := t.ObjectID
	if t.User {
		id = 0
	}
	// The same aircraft as the shot before: its aim runs on.
	now := traffic.Now()
	if s.aim == nil || s.aim.Object() != id || !cut.again && now.Sub(s.aimAt) > spotGrace {
		s.aim = camera.NewAim(id, m.cc.spotFeed(t.ObjectID, t.User), traffic.Now)
	}
	s.aimAt = now
	sec := time.Duration(spotShotSec * float64(time.Second))
	switch {
	case cut.again:
		return []camera.Shot{cs.Follow(s.aim, size, sec, true)}, "join", 0
	case cut.travel:
		from := s.pose
		if p, ok := placedTarget(from, list); ok {
			from.Target = p
		}
		move := camera.TravelAuto
		if spot.Kind == "apron" {
			move = camera.TravelRise // over the apron
		}
		tr := cs.TravelTo(s.aim, size, camera.Travel{From: from, Move: move, SpeedMps: s.speed})
		return []camera.Shot{tr, cs.Follow(s.aim, size, sec, true)}, "travel", tr.Length()
	}
	return []camera.Shot{cs.Follow(s.aim, size, sec, false)}, "cut", 0
}

// lookShots are a look round from where the camera is.
func (m *cameraMan) lookShots(s *spotter, l *spotLook, list []Traffic) ([]camera.Shot, string, time.Duration) {
	from, ok := placedTarget(s.pose, list)
	if !ok {
		return nil, "", 0
	}
	eye := s.pose.Eye
	to := camera.At(l.lat, l.lon, l.altM)
	fov := s.pose.FovDeg
	name := "look"
	if l.sweep {
		// Turned a little from where it looks, the other way each time.
		h := calc.BearingDegrees(eye.Lat, eye.Lon, from.Lat, from.Lon)
		d := max(100, calc.HaversineMeters(eye.Lat, eye.Lon, from.Lat, from.Lon))
		turn := spotLookSweep
		if s.lookAt.Unix()/int64(spotLookEvery.Seconds())%2 == 1 {
			turn = -turn
		}
		lat, lon := calc.DisplaceByHeading(eye.Lat, eye.Lon, math.Mod(h+turn+360, 360), d)
		to = camera.At(lat, lon, from.AltM)
	} else {
		d := calc.HaversineMeters(eye.Lat, eye.Lon, l.lat, l.lon)
		fov = camera.Spot{}.Framing(l.size, d)
		name = "look " + l.name
	}
	d := camera.PanLength(eye, from, to, spotLookAt)
	// Not busy: a look round gives way to anything to film.
	return []camera.Shot{camera.Pan(name, eye, from, to, s.pose.FovDeg, fov, d)}, "look", 0
}
