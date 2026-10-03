//go:build windows
// +build windows

package traffic

import (
	"sort"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// TrafficPicture is all traffic around a centre of the world (#366): our
// controlled aircraft, MSFS AI and the user, with where each one is and
// what it is doing, and the airports inside the radius. The centre is
// fixed (an airport or a position) or follows the user's aircraft; the
// library never assumes which.
//
// It is fed, not self-driving: Observe with each aircraft scan (e.g.
// SimConnect RequestDataOnSimObjectType, which reaches at most
// MaxScanRadiusMeters), SetAirports with the airports around (e.g.
// AirportLister), and SetOwn for the aircraft our controllers drive, whose
// phase they know best. It keeps one GroundPicture per airport for the
// controllers there, and feeds the StandAllocators given to Allocate.
type TrafficPicture struct {
	mu       sync.Mutex
	opts     PictureOptions
	centre   airport.LatLon
	hasCtr   bool
	aircraft map[uint32]*TrackedAircraft
	own      map[uint32]ownInfo
	all      []AirportRef // every airport known
	inside   map[string]AirportRef
	ground   map[string]*GroundPicture
	stands   map[string]*StandAllocator
	events   chan PictureEvent
	// tracks: what is remembered of each aircraft between scans, for its
	// phase (picture_phase.go).
	tracks map[uint32]*phaseTrack
}

// MaxScanRadiusMeters is the largest radius SimConnect's
// RequestDataOnSimObjectType takes: MSFS AI farther away is not seen; our
// own aircraft are, through SetOwn.
const MaxScanRadiusMeters = 200000

// Picture defaults.
var (
	DefaultPictureRadiusNM = 250.0
	// RecentreNM: a centre following the user moves only once the user is
	// this far from it, so the picture does not churn with every scan.
	RecentreNM = 25.0
	// PictureStaleAfter drops an aircraft not seen in a scan for this long.
	PictureStaleAfter = 10 * time.Second
	// AirportNearNM ties an aircraft on the ground to an airport;
	// AirportTerminalNM an aircraft departing or arriving.
	AirportNearNM     = 5.0
	AirportTerminalNM = 30.0
)

// Centre is the centre of the world: an airport, a position, or the user's
// aircraft (FollowUser; until it is seen, Position or ICAO if given).
type Centre struct {
	ICAO       string
	Position   airport.LatLon
	FollowUser bool
}

// PictureOptions configure a TrafficPicture; zero values take the defaults.
type PictureOptions struct {
	Centre   Centre
	RadiusNM float64
	// Layout gives an airport's layout when loaded (nil: none), for where
	// on the airfield an aircraft on the ground is (airport.Locate: on a
	// runway, on a stand, taxiing); without layouts the nearest airport by
	// distance, the phase by speed alone.
	Layout func(icao string) *airport.Layout
}

// AirportRef is an airport the picture knows: where it is and how far from
// the centre.
type AirportRef struct {
	ICAO       string         `json:"icao"`
	Position   airport.LatLon `json:"position"`
	DistanceNM float64        `json:"distanceNM"`
}

// Phase is what an aircraft in the picture is doing.
type Phase string

const (
	PhaseParked  Phase = "parked" // on a stand, or still with nothing to tell
	PhaseTaxiing Phase = "taxiing"
	// PhaseRunway: on a runway, slow (lining up, vacating, waiting on it);
	// fast, PhaseTakeoff or PhaseLanding.
	PhaseRunway    Phase = "runway"
	PhaseDeparting Phase = "departing" // climbing in the terminal area
	PhaseEnroute   Phase = "enroute"   // level in the air
	PhaseArriving  Phase = "arriving"  // descending in the terminal area
	// #623: pushing back; holding (stopped off a stand); the take-off and
	// landing rolls; climbing and descending away from the airports; the
	// approach (descending low near an airport, until a climb or back up).
	PhasePushback   Phase = "pushback"
	PhaseHolding    Phase = "holding"
	PhaseTakeoff    Phase = "takeoff"
	PhaseLanding    Phase = "landing"
	PhaseClimbing   Phase = "climbing"
	PhaseDescending Phase = "descending"
	PhaseApproach   Phase = "approach"
)

// Observation is one aircraft of a scan.
type Observation struct {
	ObjectID  uint32         `json:"objectId"`
	Title     string         `json:"title,omitempty"`
	Tail      string         `json:"tail,omitempty"`
	Position  airport.LatLon `json:"position"`
	AltFt     float64        `json:"altFt,omitempty"` // MSL
	AGLFt     float64        `json:"aglFt"`
	GroundKts float64        `json:"groundKts"`
	Heading   float64        `json:"heading"` // true
	VSFpm     float64        `json:"vsFpm"`
	OnGround  bool           `json:"onGround"`
	User      bool           `json:"user,omitempty"`
	SpanM     float64        `json:"spanM,omitempty"`
	// From and To are the flight's departure and destination when the
	// simulator knows them (AI TRAFFIC FROMAIRPORT/TOAIRPORT; "" for
	// aircraft that do not say, such as FSLTL's). They name the airport of
	// a departing or arriving aircraft.
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
}

// TrackedAircraft is an aircraft in the picture.
type TrackedAircraft struct {
	Observation
	Phase Phase `json:"phase"`
	// Airport is the airport it is at, departing from or arriving at; ""
	// enroute.
	Airport string `json:"airport,omitempty"`
	// Ours marks an aircraft one of our controllers drives.
	Ours    bool      `json:"ours,omitempty"`
	Updated time.Time `json:"updated"`
	// Where on the airfield an aircraft on the ground is (airport.Locate:
	// runway, parking, taxiway, near) and its name ("06/24", "C22", "A");
	// "" without the airport's layout or off it.
	Where     airport.LocateFeature `json:"where,omitempty"`
	WhereName string                `json:"whereName,omitempty"`
	// SpeedDerived: GroundKts is worked out from the movement between
	// scans, the simulator reporting 0 for AI on the ground (#622).
	SpeedDerived bool `json:"speedDerived,omitempty"`
	// VSDerived: VSFpm is the altitude trend over the last ProfileWindow,
	// not the reported vertical speed (wrong sign for FSLTL AI on short
	// final); every aircraft in the air but the user's, once ProfileMin of
	// history is in.
	VSDerived bool `json:"vsDerived,omitempty"`
}

type ownInfo struct {
	phase   Phase
	airport string
}

// PictureEventKind says what changed.
type PictureEventKind string

const (
	AircraftEntered PictureEventKind = "aircraft entered"
	AircraftLeft    PictureEventKind = "aircraft left"
	AirportEntered  PictureEventKind = "airport entered"
	AirportLeft     PictureEventKind = "airport left"
	Recentred       PictureEventKind = "recentred"
)

// PictureEvent is a change of the picture.
type PictureEvent struct {
	Kind     PictureEventKind
	ObjectID uint32
	Tail     string
	ICAO     string
	Centre   airport.LatLon
}

// NewTrafficPicture creates a picture.
func NewTrafficPicture(opts PictureOptions) *TrafficPicture {
	if opts.RadiusNM <= 0 {
		opts.RadiusNM = DefaultPictureRadiusNM
	}
	p := &TrafficPicture{opts: opts, aircraft: map[uint32]*TrackedAircraft{}, own: map[uint32]ownInfo{},
		inside: map[string]AirportRef{}, ground: map[string]*GroundPicture{}, stands: map[string]*StandAllocator{}, tracks: map[uint32]*phaseTrack{},
		events: make(chan PictureEvent, 256)}
	p.setCentreLocked(opts.Centre)
	return p
}

// Events are the picture's changes. Events are dropped when the channel is
// full: the picture itself stays current.
func (p *TrafficPicture) Events() <-chan PictureEvent { return p.events }

func (p *TrafficPicture) emit(e PictureEvent) {
	select {
	case p.events <- e:
	default:
	}
}

// SetCentre moves the centre of the world.
func (p *TrafficPicture) SetCentre(c Centre) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.opts.Centre = c
	p.setCentreLocked(c)
}

// SetRadius sets the radius in nautical miles.
func (p *TrafficPicture) SetRadius(nm float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if nm > 0 {
		p.opts.RadiusNM = nm
		p.refreshAirportsLocked()
	}
}

func (p *TrafficPicture) setCentreLocked(c Centre) {
	switch {
	case c.Position != (airport.LatLon{}):
		p.moveCentreLocked(c.Position)
	case c.ICAO != "":
		for _, a := range p.all {
			if a.ICAO == c.ICAO {
				p.moveCentreLocked(a.Position)
				return
			}
		}
		// Known once SetAirports lists it.
	}
}

func (p *TrafficPicture) moveCentreLocked(at airport.LatLon) {
	p.centre, p.hasCtr = at, true
	p.emit(PictureEvent{Kind: Recentred, Centre: at})
	p.refreshAirportsLocked()
}

// Centre returns the centre, and false before it is known.
func (p *TrafficPicture) Centre() (airport.LatLon, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.centre, p.hasCtr
}

// Options returns the picture's centre setting and radius.
func (p *TrafficPicture) Options() PictureOptions {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.opts
}

// SetAirports gives the airports around (e.g. from AirportLister); those
// inside the radius are the picture's airports.
func (p *TrafficPicture) SetAirports(list []AirportRef) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.all = append(p.all[:0], list...)
	if c := p.opts.Centre; !p.hasCtr && c.ICAO != "" {
		p.setCentreLocked(c)
	}
	p.refreshAirportsLocked()
}

// AddAirport adds one airport, e.g. a flight's destination the lister
// does not reach.
func (p *TrafficPicture) AddAirport(a AirportRef) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.all {
		if p.all[i].ICAO == a.ICAO {
			p.all[i] = a
			p.refreshAirportsLocked()
			return
		}
	}
	p.all = append(p.all, a)
	p.refreshAirportsLocked()
}

func (p *TrafficPicture) refreshAirportsLocked() {
	if !p.hasCtr {
		return
	}
	now := map[string]AirportRef{}
	for _, a := range p.all {
		a.DistanceNM = calc.HaversineMeters(p.centre.Lat, p.centre.Lon, a.Position.Lat, a.Position.Lon) / 1852
		if a.DistanceNM <= p.opts.RadiusNM {
			now[a.ICAO] = a
		}
	}
	for icao := range p.inside {
		if _, ok := now[icao]; !ok {
			p.emit(PictureEvent{Kind: AirportLeft, ICAO: icao})
		}
	}
	for icao := range now {
		if _, ok := p.inside[icao]; !ok {
			p.emit(PictureEvent{Kind: AirportEntered, ICAO: icao})
		}
	}
	p.inside = now
}

// Airports are the airports inside the radius, nearest to the centre first.
func (p *TrafficPicture) Airports() []AirportRef {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]AirportRef, 0, len(p.inside))
	for _, a := range p.inside {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DistanceNM < out[j].DistanceNM })
	return out
}

// SetOwn marks objectID as one of ours, driven by a controller that knows
// its phase and airport best; ForgetOwn when it is gone.
func (p *TrafficPicture) SetOwn(objectID uint32, phase Phase, icao string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.own[objectID] = ownInfo{phase: phase, airport: icao}
	if a := p.aircraft[objectID]; a != nil {
		a.Ours, a.Phase, a.Airport = true, phase, icao
	}
}

// ForgetOwn removes objectID from our aircraft.
func (p *TrafficPicture) ForgetOwn(objectID uint32) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.own, objectID)
}

// Ground is the ground picture of an airport, shared by the controllers
// there (TaxiWithGroundPicture, ArrivalWithGroundPicture); Observe reports
// the aircraft on the ground near it that are not ours (ours report
// themselves).
func (p *TrafficPicture) Ground(icao string) *GroundPicture {
	p.mu.Lock()
	defer p.mu.Unlock()
	g := p.ground[icao]
	if g == nil {
		g = NewGroundPicture()
		p.ground[icao] = g
	}
	return g
}

// Allocate feeds a StandAllocator from the picture's scans (its own Scan
// is then not needed).
func (p *TrafficPicture) Allocate(icao string, a *StandAllocator) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stands[icao] = a
}

// Observe takes a scan: every aircraft seen, at time now. A centre that
// follows the user moves once the user is RecentreNM from it; aircraft
// outside the radius or not seen for PictureStaleAfter leave the picture.
func (p *TrafficPicture) Observe(now time.Time, scan []Observation) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.opts.Centre.FollowUser {
		for _, o := range scan {
			if o.User && (!p.hasCtr || calc.HaversineMeters(p.centre.Lat, p.centre.Lon, o.Position.Lat, o.Position.Lon)/1852 >= RecentreNM) {
				p.moveCentreLocked(o.Position)
			}
		}
	}
	for _, o := range scan {
		if p.hasCtr && calc.HaversineMeters(p.centre.Lat, p.centre.Lon, o.Position.Lat, o.Position.Lon)/1852 > p.opts.RadiusNM {
			continue
		}
		a := p.aircraft[o.ObjectID]
		if a == nil {
			a = &TrackedAircraft{}
			p.aircraft[o.ObjectID] = a
			p.emit(PictureEvent{Kind: AircraftEntered, ObjectID: o.ObjectID, Tail: o.Tail})
		}
		prev, prevAt := a.Observation, a.Updated
		a.Observation, a.Updated = o, now
		if own, ok := p.own[o.ObjectID]; ok {
			a.Ours, a.Phase, a.Airport = true, own.phase, own.airport
			// Ours may be injected, whose vertical speed the sim does not
			// know (live: +560 fpm descending on the glide path): measured
			// from the altitude between scans, smoothed.
			if dt := now.Sub(prevAt).Seconds(); !prevAt.IsZero() && dt >= 0.5 && dt < 10 && prev.ObjectID == o.ObjectID {
				a.VSFpm = (prev.VSFpm + (o.AltFt-prev.AltFt)/dt*60) / 2
			}
		} else {
			a.Ours = false
			p.classifyLocked(a, now)
		}
	}
	for id, a := range p.aircraft {
		out := p.hasCtr && calc.HaversineMeters(p.centre.Lat, p.centre.Lon, a.Position.Lat, a.Position.Lon)/1852 > p.opts.RadiusNM
		if out || now.Sub(a.Updated) > PictureStaleAfter {
			delete(p.aircraft, id)
			delete(p.tracks, id)
			for _, g := range p.ground {
				g.Forget(id)
			}
			p.emit(PictureEvent{Kind: AircraftLeft, ObjectID: id, Tail: a.Tail})
		}
	}
	p.feedLocked(now)
}

// feedLocked reports the aircraft on the ground that are not ours to the
// ground picture and the stand allocator of the airport they are at.
func (p *TrafficPicture) feedLocked(now time.Time) {
	perAirport := map[string][]scanned{}
	for id, a := range p.aircraft {
		if !a.OnGround || a.Airport == "" {
			continue
		}
		if g := p.ground[a.Airport]; g != nil && !a.Ours {
			prof := DefaultMotionProfile()
			if a.SpanM > 0 {
				prof.SpanMeters = a.SpanM
			}
			g.Report(id, a.Position, a.Heading, prof, now)
		}
		perAirport[a.Airport] = append(perAirport[a.Airport], scanned{object: id, data: standScanData{
			Lat: a.Position.Lat, Lon: a.Position.Lon, OnGround: 1, GroundSpeedKt: a.GroundKts, WingSpanFt: a.SpanM / 0.3048}})
	}
	for icao, s := range p.stands {
		s.observe(perAirport[icao])
	}
}

// Aircraft are the aircraft in the picture, nearest to the centre first.
func (p *TrafficPicture) Aircraft() []TrackedAircraft {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]TrackedAircraft, 0, len(p.aircraft))
	for _, a := range p.aircraft {
		out = append(out, *a)
	}
	c := p.centre
	sort.Slice(out, func(i, j int) bool {
		return calc.HaversineMeters(c.Lat, c.Lon, out[i].Position.Lat, out[i].Position.Lon) <
			calc.HaversineMeters(c.Lat, c.Lon, out[j].Position.Lat, out[j].Position.Lon)
	})
	return out
}
