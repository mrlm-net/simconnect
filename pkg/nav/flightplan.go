//go:build windows
// +build windows

package nav

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Phase is the part of a flight plan a waypoint belongs to.
type Phase string

// Flight plan phases.
const (
	PhaseSID      Phase = "SID"
	PhaseEnroute  Phase = "ENROUTE"
	PhaseSTAR     Phase = "STAR"
	PhaseApproach Phase = "APPROACH"
)

// Waypoint kinds besides the fix kinds of FixKind ("W", "V", "N"); a
// computed point (the end of a climb or of a heading leg) has none.
const (
	PointAirport = "A"
	PointRunway  = "R"
	PointProfile = "P" // top of climb (TOC) and top of descent (TOD)
)

// Planning parameters.
const (
	// EnrouteMaxStretch is how much longer than the direct leg an airway
	// route between the SID and the STAR may be before Plan flies direct
	// (AirwayGraph.RouteOrDirect).
	EnrouteMaxStretch = 1.5
	// ContingencyRatio is the contingency fuel, a share of the trip fuel.
	ContingencyRatio = 0.05
	// FinalReserve is the final reserve fuel, at the cruise fuel flow.
	FinalReserve = 30 * time.Minute
	// climbExtraRatio is the fuel a climb burns over the cruise flow.
	climbExtraRatio = 0.5
	// minCruiseShare is the least share of the distance flown level: the
	// cruise level is lowered on short flights until climb and descent fit.
	minCruiseShare = 0.15
	// joinRadiusNM is how far from an airport without procedures the
	// airway network may be joined (or left).
	joinRadiusNM = 50.0
	// fixMatchNM is how close a procedure fix must be to the airway
	// graph's fix of the same identifier to be taken for it.
	fixMatchNM   = 3.0
	feetPerMeter = 1 / 0.3048
)

// ErrNoAirport is returned by Plan when an airport has no position.
var ErrNoAirport = errors.New("nav: airport without a position")

// AirportInfo is one end of a flight plan. With a Layout, its reference
// point, elevation and runways are used; without one, Position and
// ElevationM (an airport known only by where it is). Procedures are
// optional: without them the aircraft departs or arrives direct from or to
// the runway threshold (or the airport).
type AirportInfo struct {
	ICAO       string              `json:"icao"`
	Name       string              `json:"name,omitempty"`
	Layout     *airport.Layout     `json:"-"`
	Procedures *airport.Procedures `json:"-"`
	Position   airport.LatLon      `json:"position"`
	ElevationM float64             `json:"elevationM,omitempty"`
}

// reference returns the airport's position and elevation in meters.
func (a AirportInfo) reference() (airport.LatLon, float64) {
	if a.Layout != nil {
		return airport.LatLon{Lat: a.Layout.Latitude, Lon: a.Layout.Longitude}, a.Layout.Altitude
	}
	return a.Position, a.ElevationM
}

// DisplayName returns the airport's name: Name, the layout's, or the ICAO.
func (a AirportInfo) DisplayName() string {
	switch {
	case a.Name != "":
		return a.Name
	case a.Layout != nil && a.Layout.Name != "":
		return a.Layout.Name
	}
	return strings.ToUpper(a.ICAO)
}

// FlightPlanRequest is what Plan plans.
type FlightPlanRequest struct {
	Departure AirportInfo `json:"departure"`
	Arrival   AirportInfo `json:"arrival"`
	// Type is the ICAO aircraft type designator (PerformanceFor).
	Type string `json:"type"`
	// CruiseFL is the cruise flight level; 0 lets Plan choose (CruiseLevel).
	CruiseFL int `json:"cruiseFL,omitempty"`
	// DepartureRunway and ArrivalRunway are runway ends ("24", "06L"); ""
	// chooses them with ActiveRunways from the airport's weather (calm
	// without it) and limits.
	DepartureRunway string       `json:"departureRunway,omitempty"`
	ArrivalRunway   string       `json:"arrivalRunway,omitempty"`
	DepWeather      *Weather     `json:"-"`
	ArrWeather      *Weather     `json:"-"`
	DepLimits       RunwayLimits `json:"-"`
	ArrLimits       RunwayLimits `json:"-"`
	// AlternateFuelKg is the fuel to an alternate; 0 when none is planned.
	AlternateFuelKg float64 `json:"alternateFuelKg,omitempty"`
}

// Waypoint is one point of a flight plan, in the order flown. Altitudes
// are feet above sea level, 0 meaning no constraint.
type Waypoint struct {
	// Ident is the fix identifier ("DOBEN", "RW24", "LKPR", "TOC"); ""
	// for a computed point.
	Ident  string `json:"ident,omitempty"`
	Region string `json:"region,omitempty"` // ICAO region, "" when unknown
	// Kind is FixKind's letter ("W", "V", "N"), PointAirport, PointRunway,
	// PointProfile, or "" for a computed point.
	Kind     string         `json:"kind,omitempty"`
	Position airport.LatLon `json:"position"`
	// Airway is how the point is reached: an airway, Direct, the SID or
	// STAR name, or the approach name ("ILS 06"); "" for the first point.
	Airway      string  `json:"airway,omitempty"`
	Phase       Phase   `json:"phase"`
	AltMinFt    float64 `json:"altMinFt,omitempty"`
	AltMaxFt    float64 `json:"altMaxFt,omitempty"`
	SpeedMaxKts float64 `json:"speedMaxKts,omitempty"`
	// AltFt is the planned altitude (the vertical profile, within the
	// constraints); DistanceNM the distance flown from the first point.
	AltFt      float64 `json:"altFt"`
	DistanceNM float64 `json:"distanceNM"`
	FlyOver    bool    `json:"flyOver,omitempty"`
	IAF        bool    `json:"iaf,omitempty"`
	FAF        bool    `json:"faf,omitempty"`
	MAP        bool    `json:"map,omitempty"`
	Vectors    bool    `json:"vectors,omitempty"` // expect radar vectors after
}

// Computed reports whether the point is not a charted fix.
func (w Waypoint) Computed() bool { return w.Ident == "" }

// Fix reports whether the point is an enroute or procedure fix (not an
// airport, runway, computed or profile point).
func (w Waypoint) Fix() bool {
	switch w.Kind {
	case string(KindWaypoint), string(KindVOR), string(KindNDB):
		return w.Ident != ""
	}
	return false
}

// Constraint formats the altitude and speed constraint: "5000", "≥4000",
// "≤FL100", "FL070–FL100", "≤210KT"; "" for none.
func (w Waypoint) Constraint() string {
	ft := func(f float64) string {
		f = math.Round(f/100) * 100
		if f >= 18000 {
			return fmt.Sprintf("FL%03d", int(f/100))
		}
		return fmt.Sprintf("%d", int(f))
	}
	s := ""
	switch {
	case w.AltMinFt > 0 && w.AltMinFt == w.AltMaxFt:
		s = ft(w.AltMinFt)
	case w.AltMinFt > 0 && w.AltMaxFt > 0:
		s = ft(w.AltMinFt) + "–" + ft(w.AltMaxFt)
	case w.AltMinFt > 0:
		s = "≥" + ft(w.AltMinFt)
	case w.AltMaxFt > 0:
		s = "≤" + ft(w.AltMaxFt)
	}
	if w.SpeedMaxKts > 0 {
		if s != "" {
			s += " "
		}
		s += fmt.Sprintf("≤%.0fKT", w.SpeedMaxKts)
	}
	return s
}

// FuelPlan is the fuel of a flight, kilograms.
type FuelPlan struct {
	Taxi        float64 `json:"taxiKg"`
	Trip        float64 `json:"tripKg"`
	Contingency float64 `json:"contingencyKg"`
	Alternate   float64 `json:"alternateKg"`
	Reserve     float64 `json:"reserveKg"` // final reserve
	Total       float64 `json:"totalKg"`   // block fuel
}

// FlightPlan is a planned IFR flight: the procedures chosen, the route,
// every waypoint with its planned altitude, the cruise level, time and
// fuel. Build one with Plan.
type FlightPlan struct {
	Request         FlightPlanRequest `json:"request"`
	DepartureRunway string            `json:"departureRunway,omitempty"`
	ArrivalRunway   string            `json:"arrivalRunway,omitempty"`
	SID             string            `json:"sid,omitempty"`
	SIDTransition   string            `json:"sidTransition,omitempty"`
	STAR            string            `json:"star,omitempty"`
	STARTransition  string            `json:"starTransition,omitempty"`
	// Approach is the approach name ("ILS 06"), ApproachType its kind as
	// a .pln file spells it ("ILS", "RNAV", "VORDME"…).
	Approach           string `json:"approach,omitempty"`
	ApproachType       string `json:"approachType,omitempty"`
	ApproachSuffix     string `json:"approachSuffix,omitempty"`
	ApproachTransition string `json:"approachTransition,omitempty"`
	// Route is the ICAO flight plan route (item 15): "DOBE4A DOBEN DCT
	// GOLOP GOLO4T".
	Route     string     `json:"route"`
	Waypoints []Waypoint `json:"waypoints"`
	CruiseFL  int        `json:"cruiseFL"`
	// MagneticTrack is the magnetic track departure → arrival the cruise
	// level's direction is chosen from.
	MagneticTrack float64       `json:"magneticTrack"`
	DistanceNM    float64       `json:"distanceNM"`
	TOCNM         float64       `json:"tocNM"` // distance to the top of climb
	TODNM         float64       `json:"todNM"` // distance to the top of descent
	ETE           time.Duration `json:"ete"`
	Fuel          FuelPlan      `json:"fuel"`
	Performance   Performance   `json:"performance"`
}

// sidOption is a SID (with an enroute transition) resolved from the runway.
type sidOption struct {
	name, transition string
	pts              []airport.NavPoint
	lengthNM         float64
}

// arrivalOption is a STAR (with an enroute transition) joined to the
// approach, or an approach alone.
type arrivalOption struct {
	star, starTransition string
	approach             airport.Approach
	appTransition        string
	starPts, appPts      []airport.NavPoint
	lengthNM             float64
}

func (a arrivalOption) points() []airport.NavPoint {
	return append(append([]airport.NavPoint(nil), a.starPts...), a.appPts...)
}

// Plan plans an IFR flight. The runways in use come from the request or
// the weather (ActiveRunways). Of the departure runway's SIDs (and their
// enroute transitions) and the arrival runway's STARs, Plan takes the
// pair with the shortest path: SID, great circle between the SID's last
// fix and the STAR's first, STAR and approach. The STAR joins the best
// approach (airport.Procedures.BestApproach) through the transition that
// starts where it ends. Between the two, the route follows the airways of
// g (RouteOrDirect, direct when they stretch it by more than
// EnrouteMaxStretch); an airport without procedures joins the airways at
// the nearest fix toward the other end within 50 NM, directly from or to
// its runway threshold. g may be nil: then the enroute part is direct.
//
// The cruise level follows the semicircular rule on the magnetic track
// between the airports (CruiseLevel), the time and fuel the type's
// Performance; winds are not taken into account.
func Plan(req FlightPlanRequest, g *AirwayGraph) (*FlightPlan, error) {
	depPos, depElev := req.Departure.reference()
	arrPos, arrElev := req.Arrival.reference()
	if depPos == (airport.LatLon{}) {
		return nil, fmt.Errorf("%w: %s", ErrNoAirport, req.Departure.ICAO)
	}
	if arrPos == (airport.LatLon{}) {
		return nil, fmt.Errorf("%w: %s", ErrNoAirport, req.Arrival.ICAO)
	}
	perf := PerformanceFor(req.Type)
	fp := &FlightPlan{Request: req, Performance: perf}

	// Runways and the points the flight starts and ends at.
	depRwy, depEnd, depDER, depOK, err := pickRunway(req.Departure, req.DepartureRunway, req.DepWeather, req.DepLimits, true)
	if err != nil {
		return nil, err
	}
	arrRwy, arrEnd, _, arrOK, err := pickRunway(req.Arrival, req.ArrivalRunway, req.ArrWeather, req.ArrLimits, false)
	if err != nil {
		return nil, err
	}
	fp.DepartureRunway, fp.ArrivalRunway = depRwy, arrRwy
	start := Waypoint{Ident: strings.ToUpper(req.Departure.ICAO), Kind: PointAirport, Position: depPos, Phase: PhaseSID}
	if depOK {
		start = Waypoint{Ident: "RW" + depEnd.Name, Kind: PointRunway, Position: depEnd.Threshold, Phase: PhaseSID}
		depElev = runwayAltitude(req.Departure.Layout, depEnd.Name, depElev)
	}
	end := Waypoint{Ident: strings.ToUpper(req.Arrival.ICAO), Kind: PointAirport, Position: arrPos, Phase: PhaseApproach}
	if arrOK {
		end = Waypoint{Ident: "RW" + arrEnd.Name, Kind: PointRunway, Position: arrEnd.Threshold, Phase: PhaseApproach}
		arrElev = runwayAltitude(req.Arrival.Layout, arrEnd.Name, arrElev)
	}

	// Procedures: the SID and arrival pair with the shortest path.
	sids := []sidOption{{}}
	if p := req.Departure.Procedures; p != nil {
		if o := sidOptions(p, depRwy, depDER, depElev); len(o) > 0 {
			sids = o
		}
	}
	arrs := []arrivalOption{{}}
	if p := req.Arrival.Procedures; p != nil {
		if o := arrivalOptions(p, arrRwy); len(o) > 0 {
			arrs = o
		}
	}
	var sid sidOption
	var arr arrivalOption
	best := math.Inf(1)
	for _, s := range sids {
		exit := start.Position
		if n := len(s.pts); n > 0 {
			exit = s.pts[n-1].Position
		}
		for _, a := range arrs {
			entry := end.Position
			if pts := a.points(); len(pts) > 0 {
				entry = pts[0].Position
			}
			if c := s.lengthNM + dist(exit, entry) + a.lengthNM; c < best {
				best, sid, arr = c, s, a
			}
		}
	}
	fp.SID, fp.SIDTransition = sid.name, sid.transition
	fp.STAR, fp.STARTransition = arr.star, arr.starTransition
	if arr.approach.Name != "" {
		fp.Approach, fp.ApproachTransition = arr.approach.Name, arr.appTransition
		fp.ApproachType, fp.ApproachSuffix = plnApproachType(arr.approach.Type), arr.approach.Suffix
	}

	// Waypoints: runway, SID, enroute, STAR, approach, runway.
	depRegions := regionIndex(req.Departure.Procedures)
	arrRegions := regionIndex(req.Arrival.Procedures)
	wps := []Waypoint{start}
	for _, n := range sid.pts {
		wps = append(wps, navWaypoint(n, depRegions, sid.name, PhaseSID))
	}
	var tail []Waypoint
	for _, n := range arr.starPts {
		tail = append(tail, navWaypoint(n, arrRegions, arr.star, PhaseSTAR))
	}
	for _, n := range arr.appPts {
		tail = append(tail, navWaypoint(n, arrRegions, arr.approach.Name, PhaseApproach))
	}
	if n := len(tail); n == 0 || (tail[n-1].Kind != PointRunway && end.Kind == PointRunway) {
		end.Airway = arr.approach.Name
		tail = append(tail, end)
	}
	mid, via := enroute(g, wps[len(wps)-1], tail[0])
	tail[0].Airway = via
	wps = dedupeWaypoints(append(append(wps, mid...), tail...))
	fp.Route = routeString(sid.name, wps, arr.star)

	// Distances and the vertical profile.
	for i := 1; i < len(wps); i++ {
		wps[i].DistanceNM = wps[i-1].DistanceNM + dist(wps[i-1].Position, wps[i].Position)
	}
	total := wps[len(wps)-1].DistanceNM
	fp.DistanceNM = total
	magVar := 0.0
	if p := req.Departure.Procedures; p != nil {
		magVar = p.MagVar
	} else if p := req.Arrival.Procedures; p != nil {
		magVar = p.MagVar
	}
	fp.MagneticTrack = math.Mod(calc.BearingDegrees(depPos.Lat, depPos.Lon, arrPos.Lat, arrPos.Lon)+magVar+720, 360)
	depFt, arrFt := depElev*feetPerMeter, arrElev*feetPerMeter
	fp.CruiseFL = req.CruiseFL
	if fp.CruiseFL <= 0 {
		fp.CruiseFL = CruiseLevel(perf, fp.MagneticTrack, total, depFt, arrFt)
	}
	cruiseFt := float64(fp.CruiseFL) * 100
	climbNM, descNM := profileNM(perf, cruiseFt, depFt, arrFt)
	peakFt := cruiseFt
	if climbNM+descNM > total && total > 0 {
		// Too short to reach the level: climb and descend at the rates to
		// where the two meet.
		k := total / (climbNM + descNM)
		climbNM, descNM = climbNM*k, descNM*k
		peakFt = depFt + k*(cruiseFt-depFt)
	}
	fp.TOCNM, fp.TODNM = climbNM, total-descNM
	profile := func(d float64) float64 {
		switch {
		case d <= climbNM && climbNM > 0:
			return depFt + (peakFt-depFt)*d/climbNM
		case d >= total-descNM && descNM > 0:
			return arrFt + (peakFt-arrFt)*(total-d)/descNM
		}
		return peakFt
	}
	for i := range wps {
		a := profile(wps[i].DistanceNM)
		if wps[i].AltMaxFt > 0 {
			a = math.Min(a, wps[i].AltMaxFt)
		}
		a = math.Max(a, wps[i].AltMinFt)
		wps[i].AltFt = math.Round(a)
	}
	wps = insertProfilePoint(wps, fp.TODNM, "TOD", peakFt)
	wps = insertProfilePoint(wps, fp.TOCNM, "TOC", peakFt)
	fp.Waypoints = wps

	// Time and fuel.
	climbH := climbNM / perf.ClimbTASKts
	descH := descNM / perf.DescentTASKts
	cruiseH := math.Max(0, total-climbNM-descNM) / perf.CruiseTASKts
	hours := climbH + cruiseH + descH
	fp.ETE = time.Duration(hours * float64(time.Hour)).Round(time.Minute)
	trip := perf.BurnKgH*hours + climbExtraRatio*perf.BurnKgH*climbH
	f := FuelPlan{Taxi: perf.TaxiKg, Trip: math.Round(trip), Contingency: math.Round(ContingencyRatio * trip),
		Alternate: math.Round(req.AlternateFuelKg), Reserve: math.Round(perf.BurnKgH * FinalReserve.Hours())}
	f.Total = f.Taxi + f.Trip + f.Contingency + f.Alternate + f.Reserve
	fp.Fuel = f
	return fp, nil
}

// CruiseLevel chooses the cruise flight level for a flight of distanceNM
// on a magnetic track, from and to airports at depFt and arrFt: the
// highest level of the semicircular rule (RVSM: odd levels — FL350, FL370
// — on tracks 000–179°, even — FL340, FL360 — on 180–359°) up to the
// type's MaxFL at which climb and descent leave at least 15% of the
// distance level; at least 2000 ft above the higher airport.
func CruiseLevel(p Performance, magTrack, distanceNM, depFt, arrFt float64) int {
	east := math.Mod(magTrack+360, 360) < 180
	valid := func(fl int) bool { return fl%10 == 0 && (fl/10)%2 == 1 == east }
	floor := int(math.Ceil((math.Max(depFt, arrFt) + 2000) / 100))
	top := p.MaxFL
	for top > 0 && !valid(top) {
		top--
	}
	for fl := top; fl >= floor; fl -= 20 {
		c, d := profileNM(p, float64(fl)*100, depFt, arrFt)
		if c+d <= (1-minCruiseShare)*distanceNM {
			return fl
		}
	}
	fl := floor
	for !valid(fl) {
		fl++
	}
	return fl
}

// profileNM returns the climb and descent distances to and from cruiseFt.
func profileNM(p Performance, cruiseFt, depFt, arrFt float64) (climb, descent float64) {
	climb = math.Max(0, cruiseFt-depFt) / p.ClimbFPM / 60 * p.ClimbTASKts
	descent = math.Max(0, cruiseFt-arrFt) / p.DescentFPM / 60 * p.DescentTASKts
	return climb, descent
}

// pickRunway chooses a runway end: the requested one, or the one in use
// for the weather (calm without it). ok is false when the airport has no
// layout or runways; der is the departure end of the runway (the other
// end's threshold).
func pickRunway(a AirportInfo, want string, w *Weather, lim RunwayLimits, departure bool) (name string, end airport.RunwayEnd, der airport.LatLon, ok bool, err error) {
	l := a.Layout
	if l == nil || len(l.Runways) == 0 {
		return normalizeEnd(want), airport.RunwayEnd{}, airport.LatLon{}, false, nil
	}
	if want == "" {
		var wx Weather
		if w != nil {
			wx = *w
		}
		use := ActiveRunways(l, wx, lim)
		want = use.Arrival.Name
		if departure {
			want = use.Departure.Name
		}
	}
	rwy, e, found := l.RunwayEnd(want)
	if !found {
		return "", airport.RunwayEnd{}, airport.LatLon{}, false, fmt.Errorf("nav: %s has no runway %s", a.ICAO, want)
	}
	der = rwy.Primary.Threshold
	if e.Name == rwy.Primary.Name {
		der = rwy.Secondary.Threshold
	}
	return e.Name, e, der, true, nil
}

// runwayAltitude is the elevation of a runway, else def.
func runwayAltitude(l *airport.Layout, name string, def float64) float64 {
	if l == nil {
		return def
	}
	if r, _, ok := l.RunwayEnd(name); ok && r.Altitude != 0 {
		return r.Altitude
	}
	return def
}

// sidOptions resolves every SID from the runway, with each enroute
// transition, cut after its last charted fix.
func sidOptions(p *airport.Procedures, rwy string, der airport.LatLon, elev float64) []sidOption {
	var out []sidOption
	for _, s := range p.SIDsFor(rwy) {
		for _, t := range transitionNames(s.EnrouteTransitions) {
			pts, err := p.ResolveSID(s.Name, rwy, t, der, elev)
			if err != nil {
				continue
			}
			if pts = throughLastFix(pts); len(pts) == 0 {
				continue
			}
			out = append(out, sidOption{name: s.Name, transition: t, pts: pts, lengthNM: pathNM(der, pts)})
		}
	}
	return out
}

// arrivalOptions resolves every STAR to the runway (with each enroute
// transition) joined to the best approach; the approach alone through
// each of its transitions when there is no STAR; the STARs alone when
// there is no approach.
func arrivalOptions(p *airport.Procedures, rwy string) []arrivalOption {
	app, hasApp := p.BestApproach(rwy)
	var out []arrivalOption
	for _, s := range p.STARsFor(rwy) {
		for _, t := range transitionNames(s.EnrouteTransitions) {
			pts, err := p.ResolveSTAR(s.Name, t, rwy)
			if err != nil || len(throughLastFix(pts)) == 0 {
				continue
			}
			o := arrivalOption{star: s.Name, starTransition: t, starPts: pts}
			if hasApp {
				o.approach = app
				join := throughLastFix(pts)
				last := join[len(join)-1].Ident
				tr := ""
				for _, at := range app.Transitions {
					if strings.EqualFold(at.Name, last) {
						tr, o.starPts = at.Name, join // the approach takes over from the STAR's last fix
						break
					}
				}
				a, err := p.ResolveApproach(app.Name, tr)
				if err != nil {
					continue
				}
				o.appTransition, o.appPts = tr, a
				o.starPts, o.appPts = mergeJoin(o.starPts, o.appPts)
			}
			o.lengthNM = pathNM(airport.LatLon{}, o.points())
			out = append(out, o)
		}
	}
	if len(out) > 0 || !hasApp {
		return out
	}
	for _, t := range transitionNames(app.Transitions) {
		a, err := p.ResolveApproach(app.Name, t)
		if err != nil || len(a) == 0 {
			continue
		}
		out = append(out, arrivalOption{approach: app, appTransition: t, appPts: a, lengthNM: pathNM(airport.LatLon{}, a)})
	}
	return out
}

// transitionNames is "" (none) and every transition's name.
func transitionNames(ts []airport.Transition) []string {
	out := []string{""}
	for _, t := range ts {
		if t.Name != "" {
			out = append(out, t.Name)
		}
	}
	return out
}

// throughLastFix cuts the points after the last charted fix.
func throughLastFix(pts []airport.NavPoint) []airport.NavPoint {
	for i := len(pts) - 1; i >= 0; i-- {
		if !pts[i].Computed() {
			return pts[:i+1]
		}
	}
	return nil
}

// mergeJoin merges the STAR's last point into the approach's first when
// they are the same fix (the IAF): one point, flags combined, the tighter
// constraints kept.
func mergeJoin(star, app []airport.NavPoint) ([]airport.NavPoint, []airport.NavPoint) {
	if len(star) == 0 || len(app) == 0 {
		return star, app
	}
	s, a := &star[len(star)-1], app[0]
	if s.Ident == "" || s.Ident != a.Ident || dist(s.Position, a.Position) > 0.1 {
		return star, app
	}
	s.IAF, s.FAF, s.MAP, s.FlyOver = s.IAF || a.IAF, s.FAF || a.FAF, s.MAP || a.MAP, s.FlyOver || a.FlyOver
	s.AltMin = math.Max(s.AltMin, a.AltMin)
	s.AltMax, s.SpeedMax = minLimit(s.AltMax, a.AltMax), minLimit(s.SpeedMax, a.SpeedMax)
	return star, app[1:]
}

// minLimit is the smaller of two limits, 0 meaning none.
func minLimit(a, b float64) float64 {
	if a == 0 || (b != 0 && b < a) {
		return b
	}
	return a
}

// pathNM is the length of a path through pts, from start when set.
func pathNM(start airport.LatLon, pts []airport.NavPoint) float64 {
	d, cur := 0.0, start
	for _, n := range pts {
		if cur != (airport.LatLon{}) {
			d += dist(cur, n.Position)
		}
		cur = n.Position
	}
	return d
}

// regionIndex maps the procedures' fix identifiers to their ICAO regions.
func regionIndex(p *airport.Procedures) map[string]string {
	m := map[string]string{}
	if p == nil {
		return m
	}
	add := func(legs []airport.Leg) {
		for _, l := range legs {
			if l.Fix != "" && l.Region != "" {
				m[strings.ToUpper(l.Fix)] = l.Region
			}
		}
	}
	addTs := func(ts []airport.Transition) {
		for _, t := range ts {
			add(t.Legs)
		}
	}
	for _, list := range [][]airport.Procedure{p.Departures, p.Arrivals} {
		for _, s := range list {
			add(s.Legs)
			addTs(s.RunwayTransitions)
			addTs(s.EnrouteTransitions)
		}
	}
	for _, a := range p.Approaches {
		addTs(a.Transitions)
		add(a.Final)
		add(a.Missed)
	}
	return m
}

// navWaypoint turns a procedure point into a waypoint.
func navWaypoint(n airport.NavPoint, regions map[string]string, airway string, phase Phase) Waypoint {
	ft := func(m float64) float64 { return math.Round(m * feetPerMeter) }
	return Waypoint{Ident: n.Ident, Region: regions[strings.ToUpper(n.Ident)], Kind: n.Kind, Position: n.Position,
		Airway: airway, Phase: phase, AltMinFt: ft(n.AltMin), AltMaxFt: ft(n.AltMax), SpeedMaxKts: n.SpeedMax,
		FlyOver: n.FlyOver, IAF: n.IAF, FAF: n.FAF, MAP: n.MAP, Vectors: n.Vectors}
}

// fixWaypoint turns an airway graph fix into an enroute waypoint.
func fixWaypoint(f Fix, airway string) Waypoint {
	return Waypoint{Ident: f.Ident, Region: f.Region, Kind: f.Kind.String(), Position: f.Position, Airway: airway, Phase: PhaseEnroute}
}

// keyNear finds the graph fix a flight plan point stands for: the same
// identifier within fixMatchNM.
func (g *AirwayGraph) keyNear(w Waypoint) (FixKey, bool) {
	if !w.Fix() {
		return FixKey{}, false
	}
	best, bestD := FixKey{}, -1.0
	for _, f := range g.Find(w.Ident) {
		if d := dist(f.Position, w.Position); d <= fixMatchNM && (bestD < 0 || d < bestD) {
			best, bestD = f.Key(), d
		}
	}
	return best, bestD >= 0
}

// joinFix is the airway fix nearest to p within joinRadiusNM of the
// ones closer to toward than p is.
func (g *AirwayGraph) joinFix(p, toward airport.LatLon) (Fix, bool) {
	best, bestD := -1, joinRadiusNM
	ahead := dist(p, toward)
	for i, f := range g.Fixes {
		if len(g.adj[f.Key()]) == 0 {
			continue
		}
		if d := dist(p, f.Position); d <= bestD && dist(f.Position, toward) < ahead {
			best, bestD = i, d
		}
	}
	if best < 0 {
		return Fix{}, false
	}
	return g.Fixes[best], true
}

// enroute returns the waypoints between from (the SID's last point) and to
// (the arrival's first), both excluded, and the airway to is reached by.
func enroute(g *AirwayGraph, from, to Waypoint) ([]Waypoint, string) {
	if g == nil {
		return nil, Direct
	}
	var mid []Waypoint
	fk, fok := g.keyNear(from)
	if !fok {
		if f, ok := g.joinFix(from.Position, to.Position); ok {
			fk, fok = f.Key(), true
			mid = append(mid, fixWaypoint(f, Direct))
		}
	}
	tk, tok := g.keyNear(to)
	var exit *Waypoint // where the airways are left for an airport without procedures
	if !tok {
		if f, ok := g.joinFix(to.Position, from.Position); ok {
			w := fixWaypoint(f, "")
			exit, tk = &w, f.Key()
		}
	}
	if !fok || (!tok && exit == nil) || fk == tk {
		if exit != nil && (!fok || fk != tk) {
			exit.Airway = Direct
			mid = append(mid, *exit)
		}
		return mid, Direct
	}
	steps, err := g.RouteOrDirect(fk, tk, EnrouteMaxStretch)
	if err != nil || len(steps) < 2 {
		return mid, Direct
	}
	for _, s := range steps[1:] {
		f, _ := g.Fix(s.Fix)
		mid = append(mid, fixWaypoint(f, s.Airway))
	}
	if exit != nil {
		return mid, Direct
	}
	via := mid[len(mid)-1].Airway
	return mid[:len(mid)-1], via
}

// dedupeWaypoints merges consecutive points at the same fix (a SID ending
// where the STAR starts, a join fix that is the STAR's first): the
// procedure point is kept, with the tighter constraints.
func dedupeWaypoints(wps []Waypoint) []Waypoint {
	var out []Waypoint
	for _, w := range wps {
		if k := len(out) - 1; k >= 0 && w.Ident != "" && out[k].Ident == w.Ident && dist(out[k].Position, w.Position) < 1 {
			m := &out[k]
			if m.Phase == PhaseEnroute {
				m.Phase, m.Airway, m.Region = w.Phase, w.Airway, firstSet(m.Region, w.Region)
			}
			m.AltMinFt = math.Max(m.AltMinFt, w.AltMinFt)
			m.AltMaxFt, m.SpeedMaxKts = minLimit(m.AltMaxFt, w.AltMaxFt), minLimit(m.SpeedMaxKts, w.SpeedMaxKts)
			m.IAF, m.FAF, m.MAP = m.IAF || w.IAF, m.FAF || w.FAF, m.MAP || w.MAP
			continue
		}
		out = append(out, w)
	}
	return out
}

func firstSet(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// routeString writes the ICAO route: the SID and its last fix, then
// airway and fix at every change of airway up to the arrival's first fix,
// and the STAR: "DOBE4A DOBEN DCT LOMKI LOMK8T". Without a SID it starts
// with "DCT".
func routeString(sid string, wps []Waypoint, star string) string {
	start, stop := 0, len(wps)
	for i, w := range wps {
		if w.Phase == PhaseSID {
			start = i
		}
		if (w.Phase == PhaseSTAR || w.Phase == PhaseApproach) && stop == len(wps) {
			stop = i + 1
		}
	}
	var parts []string
	if sid != "" {
		parts = append(parts, sid, wps[start].Ident)
	}
	var legs []Waypoint
	for _, w := range wps[start+1 : stop] {
		if w.Fix() {
			legs = append(legs, w)
		}
	}
	for i, w := range legs {
		if i+1 < len(legs) && legs[i+1].Airway == w.Airway && w.Airway != Direct {
			continue
		}
		parts = append(parts, w.Airway, w.Ident)
	}
	if star != "" {
		parts = append(parts, star)
	}
	return strings.Join(parts, " ")
}

// insertProfilePoint inserts a TOC or TOD at distance d along the route.
func insertProfilePoint(wps []Waypoint, d float64, ident string, altFt float64) []Waypoint {
	for i := 1; i < len(wps); i++ {
		if wps[i].DistanceNM < d {
			continue
		}
		a, b := wps[i-1], wps[i]
		leg := b.DistanceNM - a.DistanceNM
		pos := a.Position
		if leg > 0 {
			brg := calc.BearingDegrees(a.Position.Lat, a.Position.Lon, b.Position.Lat, b.Position.Lon)
			lat, lon := calc.DisplaceByHeading(a.Position.Lat, a.Position.Lon, brg, (d-a.DistanceNM)*1852)
			pos = airport.LatLon{Lat: lat, Lon: lon}
		}
		phase := PhaseEnroute // between two phases, e.g. from the SID's last fix onto an airway
		if a.Phase == b.Phase {
			phase = a.Phase
		}
		p := Waypoint{Ident: ident, Kind: PointProfile, Position: pos, Phase: phase, AltFt: math.Round(altFt), DistanceNM: d}
		return append(wps[:i], append([]Waypoint{p}, wps[i:]...)...)
	}
	return wps
}

// plnApproachType spells an approach type as a .pln file does.
func plnApproachType(t types.SIMCONNECT_FACILITY_APPROACH_TYPE) string {
	switch t {
	case types.SIMCONNECT_FACILITY_APPROACH_TYPE_GPS:
		return "GPS"
	case types.SIMCONNECT_FACILITY_APPROACH_TYPE_VOR:
		return "VOR"
	case types.SIMCONNECT_FACILITY_APPROACH_TYPE_NDB:
		return "NDB"
	case types.SIMCONNECT_FACILITY_APPROACH_TYPE_ILS:
		return "ILS"
	case types.SIMCONNECT_FACILITY_APPROACH_TYPE_LOCALIZER:
		return "LOCALIZER"
	case types.SIMCONNECT_FACILITY_APPROACH_TYPE_SDF:
		return "SDF"
	case types.SIMCONNECT_FACILITY_APPROACH_TYPE_LDA:
		return "LDA"
	case types.SIMCONNECT_FACILITY_APPROACH_TYPE_VORDME:
		return "VORDME"
	case types.SIMCONNECT_FACILITY_APPROACH_TYPE_NDBDME:
		return "NDBDME"
	case types.SIMCONNECT_FACILITY_APPROACH_TYPE_RNAV:
		return "RNAV"
	case types.SIMCONNECT_FACILITY_APPROACH_TYPE_LOCALIZER_BACK_COURSE:
		return "LOCALIZER_BACK_COURSE"
	}
	return ""
}

// String summarizes the plan: header, route, fuel, then one line per
// waypoint.
func (fp *FlightPlan) String() string {
	var b strings.Builder
	typ := fp.Performance.Type
	if typ == "" {
		typ = strings.ToUpper(fp.Request.Type)
	}
	end := func(icao, rwy string) string {
		if rwy == "" {
			return strings.ToUpper(icao)
		}
		return strings.ToUpper(icao) + "/" + rwy
	}
	fmt.Fprintf(&b, "%s → %s  %s  FL%03d  %.0f NM  ETE %d:%02d  track %03.0f°M\n",
		end(fp.Request.Departure.ICAO, fp.DepartureRunway), end(fp.Request.Arrival.ICAO, fp.ArrivalRunway),
		typ, fp.CruiseFL, fp.DistanceNM, int(fp.ETE.Hours()), int(fp.ETE.Minutes())%60, fp.MagneticTrack)
	fmt.Fprintf(&b, "route: %s\n", fp.Route)
	if fp.Approach != "" {
		app := fp.Approach
		if fp.ApproachTransition != "" {
			app += " via " + fp.ApproachTransition
		}
		fmt.Fprintf(&b, "approach: %s\n", app)
	}
	f := fp.Fuel
	fmt.Fprintf(&b, "fuel kg: taxi %.0f  trip %.0f  contingency %.0f  alternate %.0f  reserve %.0f  block %.0f\n",
		f.Taxi, f.Trip, f.Contingency, f.Alternate, f.Reserve, f.Total)
	for _, w := range fp.Waypoints {
		id := w.Ident
		if id == "" {
			id = "*"
		}
		fmt.Fprintf(&b, "  %-8s %-9s %-8s %6.1f NM %6.0f ft  %s\n", id, w.Phase, w.Airway, w.DistanceNM, w.AltFt, w.Constraint())
	}
	return b.String()
}

// PositionAt is where the plan is distNM along it (clamped to the plan):
// the position, the planned altitude in feet and the track in degrees
// (#369: aircraft that appear en route).
func (fp *FlightPlan) PositionAt(distNM float64) (airport.LatLon, float64, float64) {
	w := fp.Waypoints
	if len(w) == 0 {
		return airport.LatLon{}, 0, 0
	}
	for i := 1; i < len(w); i++ {
		a, b := w[i-1], w[i]
		if distNM > b.DistanceNM && i < len(w)-1 {
			continue
		}
		t := 1.0
		if leg := b.DistanceNM - a.DistanceNM; leg > 0 {
			t = math.Max(0, math.Min(1, (distNM-a.DistanceNM)/leg))
		}
		p := airport.LatLon{Lat: a.Position.Lat + t*(b.Position.Lat-a.Position.Lat), Lon: a.Position.Lon + t*(b.Position.Lon-a.Position.Lon)}
		return p, a.AltFt + t*(b.AltFt-a.AltFt), calc.BearingDegrees(a.Position.Lat, a.Position.Lon, b.Position.Lat, b.Position.Lon)
	}
	return w[0].Position, w[0].AltFt, 0
}
