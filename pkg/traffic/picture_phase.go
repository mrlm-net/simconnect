package traffic

import (
	"math"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// What an aircraft not ours is doing, scan after scan (#622, #623): on the
// ground by where it is on the airfield (airport.Locate) and how it moves,
// in the air by its altitude over the last half minute. After the MyCrew
// app's observer (mycrew-online/app internal/agent/traffic_phase.go), which
// measured the simulator live at LKPR.

// Phase thresholds.
const (
	// RollKts: faster than this on a runway is a take-off or landing roll.
	RollKts = 30.0
	// MovingKts: faster than this on the ground is moving, by the reported
	// speed; DerivedMovingKts by one worked out from positions (a metre of
	// jitter between scans is about 1 kt).
	MovingKts        = 1.0
	DerivedMovingKts = 2.0
	// StandMovingKts: slower than this on a stand is parked (rolling the
	// last metres into the parking position is not taxiing).
	StandMovingKts = 3.0
	// PushbackMinKts and PushbackOffNoseDeg: moving at least this fast with
	// the track this far off the nose is a pushback.
	PushbackMinKts     = 1.0
	PushbackOffNoseDeg = 120.0
	// LandingRollFor: a roll this soon after touching down is the landing's.
	LandingRollFor = 90 * time.Second
	// DepartingBelowFt: climbing this low in the terminal area
	// (AirportTerminalNM) is departing; elsewhere, or higher, climbing.
	DepartingBelowFt = 10000.0
	// ApproachBelowFt: descending this low near an airport is an approach;
	// it lasts until a climb (a go-around) or back above ApproachLeaveFt.
	ApproachBelowFt = 3000.0
	ApproachLeaveFt = 4000.0
	// ProfileWindow: the vertical trend looks this far back; with less than
	// ProfileMin of history the phase stands (first seen: by the reported rate, or
	// departing just after lift-off). A climb or a descent starts past
	// ProfileEnterFpm and ends inside ProfileLeaveFpm.
	ProfileWindow   = 30 * time.Second
	ProfileMin      = 4 * time.Second
	ProfileEnterFpm = 400.0
	ProfileLeaveFpm = 150.0
)

// phaseTrack is what the picture remembers of an aircraft between scans.
type phaseTrack struct {
	pos       airport.LatLon
	at        time.Time
	onGround  bool
	seen      bool
	touchdown time.Time // when it last touched down
	liftoff   time.Time // when it last left the ground
	last      Phase
	alts      []altAt // its altitude over the last ProfileWindow
}

type altAt struct {
	at    time.Time
	altFt float64
}

// classifyLocked sets a's phase, airport and place on the airfield from
// this scan and what the picture remembers of it; on the ground with a
// reported speed under MovingKts, a.GroundKts becomes the one its movement
// since the last scan gives (the simulator reports 0 for AI on the ground
// however it moves, #622).
func (p *TrafficPicture) classifyLocked(a *TrackedAircraft, now time.Time) {
	t := p.tracks[a.ObjectID]
	if t == nil {
		t = &phaseTrack{}
		p.tracks[a.ObjectID] = t
	}
	if t.seen && now.Equal(t.at) {
		a.Phase = t.last // the same scan again
		return
	}
	derivedKts := 0.0
	if t.seen {
		if dt := now.Sub(t.at).Seconds(); dt > 0 {
			derivedKts = calc.HaversineMeters(t.pos.Lat, t.pos.Lon, a.Position.Lat, a.Position.Lon) / dt / 0.514444
		}
	}
	a.SpeedDerived = a.OnGround && a.GroundKts < MovingKts
	if a.SpeedDerived {
		a.GroundKts = derivedKts
	}
	if t.seen && t.onGround != a.OnGround {
		if a.OnGround {
			t.touchdown = now
		} else {
			t.liftoff = now
		}
	}
	a.Phase, a.Airport, a.Where, a.WhereName = p.phaseLocked(t, a, now)
	t.pos, t.at, t.onGround, t.last, t.seen = a.Position, now, a.OnGround, a.Phase, true
}

// phaseLocked is a's phase, its airport and where on the airfield it is.
func (p *TrafficPicture) phaseLocked(t *phaseTrack, a *TrackedAircraft, now time.Time) (Phase, string, airport.LocateFeature, string) {
	near, nearNM := p.nearestAirportLocked(a.Position)
	if a.OnGround {
		at := ""
		if nearNM <= AirportNearNM {
			at = near
		}
		where, name := airport.LocateFeature(""), ""
		if loc, ok := p.locateLocked(a.Position, nearNM); ok {
			at, where, name = loc.ICAO, loc.Feature, loc.Name
		}
		return groundPhase(t, a, now, where), at, where, name
	}
	ph := p.airPhase(t, a, now, nearNM)
	if ph == PhaseEnroute || ph == PhaseClimbing || ph == PhaseDescending {
		return ph, "", "", ""
	}
	return ph, p.flightAirportLocked(a, ph, near), "", ""
}

// AirportAheadDeg: an arriving aircraft heads for an airport within this
// angle of its heading; a departing one has it as far behind.
const AirportAheadDeg = 60.0

// flightAirportLocked is the airport a departing or arriving aircraft
// belongs to: its origin or destination when it says (and it is in range or
// not known to the picture), else among the airports within
// AirportTerminalNM the ones ahead of it (behind it departing), those with
// a layout loaded first (not on approach), then the nearest; near when
// none is ahead.
func (p *TrafficPicture) flightAirportLocked(a *TrackedAircraft, ph Phase, near string) string {
	departing := ph == PhaseDeparting
	known := a.To
	if departing {
		known = a.From
	}
	if known != "" {
		in, nm := false, 0.0
		for _, r := range p.all {
			if r.ICAO == known {
				in, nm = true, calc.HaversineMeters(a.Position.Lat, a.Position.Lon, r.Position.Lat, r.Position.Lon)/1852
				break
			}
		}
		if !in || nm <= AirportTerminalNM {
			return known
		}
	}
	type cand struct {
		icao    string
		nm      float64
		aligned bool    // a runway end lined up with its track
		layout  bool    // a layout loaded (not on approach)
		rwyM    float64 // its longest runway
	}
	better := func(x, y cand) bool {
		switch {
		case x.aligned != y.aligned:
			return x.aligned
		case x.layout != y.layout:
			return x.layout
		case x.rwyM != y.rwyM && ph != PhaseApproach:
			return x.rwyM > y.rwyM
		}
		return x.nm < y.nm
	}
	var best *cand
	for _, r := range p.all {
		nm := calc.HaversineMeters(a.Position.Lat, a.Position.Lon, r.Position.Lat, r.Position.Lon) / 1852
		if nm > AirportTerminalNM {
			continue
		}
		if nm > 1 { // overhead every direction is ahead
			brg := calc.BearingDegrees(a.Position.Lat, a.Position.Lon, r.Position.Lat, r.Position.Lon)
			if departing {
				brg = math.Mod(brg+180, 360)
			}
			if math.Abs(headingDiff(brg, a.Heading)) > AirportAheadDeg {
				continue
			}
		}
		c := cand{icao: r.ICAO, nm: nm}
		var l *airport.Layout
		if p.opts.Layout != nil {
			l = p.opts.Layout(r.ICAO)
		}
		if l != nil {
			// Low on approach the nearest lined up is the one; higher up a
			// layout loaded marks an airport the caller cares about.
			c.layout = ph != PhaseApproach
			c.aligned = alignedRunway(l, a.Position, a.Heading, departing)
			for _, rw := range l.Runways {
				c.rwyM = math.Max(c.rwyM, rw.Length)
			}
		}
		if best == nil || better(c, *best) {
			best = &c
		}
	}
	if best == nil {
		return near
	}
	return best.icao
}

// AlignedRunwayDeg: a runway end within this angle of an aircraft's track,
// with the aircraft within AlignedCentrelineNM (or a tenth of the distance)
// of its extended centreline, is the one it lands on or took off from.
const (
	AlignedRunwayDeg    = 20.0
	AlignedCentrelineNM = 1.5
)

// alignedRunway reports a runway of l lined up with an aircraft at pos on
// heading hdg: before the threshold arriving, past it departing.
func alignedRunway(l *airport.Layout, pos airport.LatLon, hdg float64, departing bool) bool {
	for _, rw := range l.Runways {
		for _, end := range []airport.RunwayEnd{rw.Primary, rw.Secondary} {
			if math.Abs(headingDiff(end.Heading, hdg)) > AlignedRunwayDeg {
				continue
			}
			d := calc.HaversineMeters(end.Threshold.Lat, end.Threshold.Lon, pos.Lat, pos.Lon) / 1852
			off := headingDiff(end.Heading, calc.BearingDegrees(end.Threshold.Lat, end.Threshold.Lon, pos.Lat, pos.Lon)) * math.Pi / 180
			along, cross := d*math.Cos(off), math.Abs(d*math.Sin(off))
			if !departing {
				along = -along // arriving: out on the approach side
			}
			if along > 0 && cross <= math.Max(AlignedCentrelineNM, along/10) {
				return true
			}
		}
	}
	return false
}

// groundPhase is a's phase on the ground, where it is on the airfield
// (where; "" unknown).
func groundPhase(t *phaseTrack, a *TrackedAircraft, now time.Time, where airport.LocateFeature) Phase {
	if where == airport.OnRunway {
		switch {
		case a.GroundKts > RollKts && !t.touchdown.IsZero() && now.Sub(t.touchdown) < LandingRollFor:
			return PhaseLanding
		case a.GroundKts > RollKts:
			return PhaseTakeoff
		}
		return PhaseRunway
	}
	if where == "" && a.GroundKts > 40 {
		return PhaseRunway // no layout: fast is a roll, as before
	}
	moving := MovingKts
	if a.SpeedDerived {
		moving = DerivedMovingKts
	}
	stand := where == airport.AtParking
	if t.seen && a.GroundKts >= PushbackMinKts {
		if d := calc.HaversineMeters(t.pos.Lat, t.pos.Lon, a.Position.Lat, a.Position.Lon); d > 0.5 {
			track := calc.BearingDegrees(t.pos.Lat, t.pos.Lon, a.Position.Lat, a.Position.Lon)
			if math.Abs(headingDiff(track, a.Heading)) > PushbackOffNoseDeg {
				return PhasePushback
			}
		}
	}
	if t.last == PhasePushback && stand && a.GroundKts >= moving {
		return PhasePushback // too slow to measure a track: still pushing
	}
	if stand && a.GroundKts < StandMovingKts {
		return PhaseParked // still, or creeping into its parking position
	}
	if a.GroundKts < moving {
		if stand || where == "" {
			return PhaseParked
		}
		return PhaseHolding
	}
	return PhaseTaxiing
}

// airPhase is a's phase in the air: by its altitude trend, not one scan's
// vertical speed (a blip of a few dozen feet must not turn a cruise into a
// climb); nearNM is the nearest airport's distance.
func (p *TrafficPicture) airPhase(t *phaseTrack, a *TrackedAircraft, now time.Time, nearNM float64) Phase {
	trend, ok := t.trendFpm(now, a.AltFt)
	was := t.last
	if ok && !a.User {
		a.VSFpm, a.VSDerived = trend, true
	}
	if !ok {
		switch {
		case t.seen && !t.onGround:
			return was // too little history to change it
		case t.seen && nearNM <= AirportTerminalNM:
			return PhaseDeparting // just lifted off
		}
		// First seen: the reported rate for this scan only (right but on
		// short final, where the trend takes over after ProfileMin).
		trend = a.VSFpm
	}
	nearAirport := nearNM <= AirportTerminalNM
	climbing := was == PhaseClimbing || was == PhaseDeparting
	descending := was == PhaseDescending || was == PhaseApproach || was == PhaseArriving
	switch {
	case trend > ProfileEnterFpm || climbing && trend > ProfileLeaveFpm:
		if nearAirport && a.AGLFt < DepartingBelowFt {
			return PhaseDeparting
		}
		return PhaseClimbing
	case was == PhaseApproach && a.AGLFt < ApproachLeaveFt && nearAirport:
		return PhaseApproach // until a climb (above) or back up high
	case trend < -ProfileEnterFpm || descending && trend < -ProfileLeaveFpm:
		switch {
		case a.AGLFt < ApproachBelowFt && nearAirport:
			return PhaseApproach
		case nearAirport && a.AGLFt < 10000:
			return PhaseArriving
		}
		return PhaseDescending
	}
	return PhaseEnroute
}

// trendFpm adds the altitude to the history and is the vertical rate over
// the last ProfileWindow; false while the history is shorter than
// ProfileMin. The reported vertical speed is not used: MSFS gives FSLTL AI
// on short final the wrong sign (live: +500…+940 fpm descending at about
// 1,100 fpm, BAW1989 at LKPR).
func (t *phaseTrack) trendFpm(now time.Time, altFt float64) (float64, bool) {
	t.alts = append(t.alts, altAt{now, altFt})
	for len(t.alts) > 1 && now.Sub(t.alts[0].at) > ProfileWindow {
		t.alts = t.alts[1:]
	}
	span := now.Sub(t.alts[0].at)
	if span < ProfileMin {
		return 0, false
	}
	return (altFt - t.alts[0].altFt) / span.Minutes(), true
}

// nearestAirportLocked is the airport nearest pos and its distance (NM).
func (p *TrafficPicture) nearestAirportLocked(pos airport.LatLon) (string, float64) {
	near, nearNM := "", math.Inf(1)
	for _, a := range p.all {
		if d := calc.HaversineMeters(pos.Lat, pos.Lon, a.Position.Lat, a.Position.Lon) / 1852; d < nearNM {
			near, nearNM = a.ICAO, d
		}
	}
	return near, nearNM
}

// locateLocked is where on the ground pos is (airport.Locate), among the
// airports within AirportNearNM whose layout is loaded (PictureOptions.Layout).
func (p *TrafficPicture) locateLocked(pos airport.LatLon, nearNM float64) (airport.Location, bool) {
	if p.opts.Layout == nil || nearNM > AirportNearNM {
		return airport.Location{}, false
	}
	var ls []*airport.Layout
	for _, a := range p.all {
		if calc.HaversineMeters(pos.Lat, pos.Lon, a.Position.Lat, a.Position.Lon)/1852 > AirportNearNM {
			continue
		}
		if l := p.opts.Layout(a.ICAO); l != nil {
			ls = append(ls, l)
		}
	}
	if len(ls) == 0 {
		return airport.Location{}, false
	}
	loc, ok := airport.Locate(airport.LocateQuery{Position: pos, OnGround: true}, ls)
	if !ok || loc.Feature == airport.NearAirport && loc.Meters > 200 {
		return loc, ok && loc.Feature != airport.NearAirport
	}
	return loc, true
}
