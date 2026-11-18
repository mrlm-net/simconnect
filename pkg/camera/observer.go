package camera

import (
	"math"
	"time"

	"github.com/mrlm-net/simconnect/pkg/calc"
)

// An observer is the camera as one drone: it goes from one spot to the next
// instead of cutting across the airport. A cut far away makes the simulator
// load the scenery and models there, and they pop in after it; flown there,
// they load along the way. The World's spotting director moves this way
// (SpottingRequest.TravelMaxM); a host app with its own spotter plans each
// move with an Observer and plays the shots it gives.
//
//   - A cut goes at most CutM: the area is loaded, it reads as a new angle.
//   - A spot within TravelMaxM is flown to (Spot.TravelTo), the spot shot
//     joined to the travel.
//   - Farther is not gone to.
//
// There is no ground or building data: the travel's only clearance is
// MarginM above the higher of its two ends.

// Observer plans the camera's way to its next spot. The zero Observer
// (TravelMaxM 0) always cuts.
type Observer struct {
	// CutM is how far a cut goes (0: DefaultObserverCutM), at most
	// TravelMaxM.
	CutM float64
	// TravelMaxM is how far the drone flies to a spot (0: no travel, every
	// move a cut).
	TravelMaxM float64
	// SpeedMps is the travel's cruise (0: DefaultTravelSpeed), kept to
	// ObserverSpeedMin..ObserverSpeedMax.
	SpeedMps float64
	// MarginM is how far above the higher end the travel cruises (0:
	// DefaultTravelMargin): the only clearance there is.
	MarginM float64
}

// Defaults and limits of an Observer.
const (
	DefaultObserverCutM = 300.0
	// ObserverSpeedMin and ObserverSpeedMax keep the drone slow.
	ObserverSpeedMin = 10.0
	ObserverSpeedMax = 20.0
)

// MoveKind is how the camera gets to a spot.
type MoveKind string

// The moves.
const (
	// MoveCut: a cut, the spot shot alone.
	MoveCut MoveKind = "cut"
	// MoveTravel: a travel, the spot shot joined to it.
	MoveTravel MoveKind = "travel"
	// MoveTooFar: out of reach, not gone to.
	MoveTooFar MoveKind = "tooFar"
)

// Leg is a move to plan: from the camera now to a spot, filming a subject.
type Leg struct {
	// From is the camera now (its eye a World point); Have false: not known
	// (the first shot), a cut.
	From Pose
	Have bool
	To   Spot
	// Aim is the subject; Size its largest dimension, metres.
	Aim  *Aim
	Size float64
	// D is the spot shot's length (0: MaxStretch).
	D time.Duration
	// Move is the travel's shape (TravelAuto).
	Move TravelMove
}

// Way is a planned move: Kind, the way (metres on the ground from the eye,
// 0 with no From) and the shots to play one after another: a cut, the spot
// shot; a travel, the travel and the spot shot joined to it (Travel its
// length); too far, none.
type Way struct {
	Kind   MoveKind
	M      float64
	Travel time.Duration
	Shots  []Shot
}

// WithDefaults is o as it plans: its zero values filled, CutM at most
// TravelMaxM, SpeedMps kept to its limits. With TravelMaxM 0 it is o.
func (o Observer) WithDefaults() Observer {
	if o.TravelMaxM <= 0 {
		return o
	}
	if o.CutM <= 0 {
		o.CutM = DefaultObserverCutM
	}
	o.CutM = min(o.CutM, o.TravelMaxM)
	if o.SpeedMps <= 0 {
		o.SpeedMps = DefaultTravelSpeed
	}
	o.SpeedMps = max(ObserverSpeedMin, min(ObserverSpeedMax, o.SpeedMps))
	if o.MarginM <= 0 {
		o.MarginM = DefaultTravelMargin
	}
	return o
}

// Decide is the move to a spot whose shot starts startM from the eye (on a
// rail, abeam the subject; else its eye) and whose eye is eyeM from it: a
// cut within CutM of the start, a travel within TravelMaxM of the eye, else
// too far; and the way, metres. have false (no eye known) or TravelMaxM 0:
// a cut, way 0.
func (o Observer) Decide(have bool, startM, eyeM float64) (MoveKind, float64) {
	o = o.WithDefaults()
	switch {
	case !have || o.TravelMaxM <= 0:
		return MoveCut, 0
	case startM <= o.CutM:
		return MoveCut, startM
	case eyeM > o.TravelMaxM:
		return MoveTooFar, eyeM
	}
	return MoveTravel, eyeM
}

// Seconds is about how long the drone takes over d metres, a ramp
// included, to decide before a travel is made (0: a cut). A travel's own
// Length is the exact one.
func (o Observer) Seconds(have bool, d float64) float64 {
	o = o.WithDefaults()
	if !have || o.TravelMaxM <= 0 || d <= o.CutM {
		return 0
	}
	return d/o.SpeedMps + TravelRamp.Seconds()
}

// Plan plans l: the way from l.From's eye to l.To (its rail's start abeam
// the subject now) and the shots of the move.
func (o Observer) Plan(l Leg) Way {
	if !l.Have || l.From.Eye.Frame != World {
		return Way{Kind: MoveCut, Shots: o.Shots(MoveCut, l)}
	}
	e := l.From.Eye
	start := l.To.start(l.Aim)
	kind, m := o.Decide(true, calc.HaversineMeters(e.Lat, e.Lon, start.Lat, start.Lon), calc.HaversineMeters(e.Lat, e.Lon, l.To.Eye.Lat, l.To.Eye.Lon))
	mv := Way{Kind: kind, M: m, Shots: o.Shots(kind, l)}
	if kind == MoveTravel {
		mv.Travel = mv.Shots[0].Length()
	}
	return mv
}

// Shots are the shots of a move of kind on l: a cut, the spot shot
// (Spot.Follow, not joined); a travel, l.From to the spot's eye at o's
// cruise and margin, the spot shot joined to it; too far, none.
func (o Observer) Shots(kind MoveKind, l Leg) []Shot {
	switch kind {
	case MoveCut:
		return []Shot{l.To.Follow(l.Aim, l.Size, l.D, false)}
	case MoveTravel:
		o = o.WithDefaults()
		tr := l.To.TravelTo(l.Aim, l.Size, Travel{From: l.From, Move: l.Move, SpeedMps: o.SpeedMps, MarginM: o.MarginM})
		return []Shot{tr, l.To.Follow(l.Aim, l.Size, l.D, true)}
	}
	return nil
}

// start is where s's shot starts on aim's subject now: on a rail, abeam
// it; else (or no subject yet) the eye.
func (s Spot) start(aim *Aim) Point {
	if s.RailEnd == nil || aim == nil {
		return s.Eye
	}
	aim.mu.Lock()
	now, have := aim.update()
	var lat, lon float64
	if have {
		lat, lon, _ = aim.at(now)
	}
	aim.mu.Unlock()
	if !have {
		return s.Eye
	}
	cos := math.Cos(s.Eye.Lat * math.Pi / 180)
	re, rn := (s.RailEnd.Lon-s.Eye.Lon)*cos, s.RailEnd.Lat-s.Eye.Lat
	te, tn := (lon-s.Eye.Lon)*cos, lat-s.Eye.Lat
	l2 := re*re + rn*rn
	if l2 == 0 {
		return s.Eye
	}
	f := max(0, min(1, (te*re+tn*rn)/l2))
	return At(s.Eye.Lat+rn*f, s.Eye.Lon+re*f/cos, s.Eye.AltM+(s.RailEnd.AltM-s.Eye.AltM)*f)
}
