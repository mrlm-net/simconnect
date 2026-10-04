package airport

import (
	"math"
	"sort"
)

// LocateFeature is what of an airport a located position is on or near.
type LocateFeature string

const (
	OnRunway    LocateFeature = "runway"    // on a runway surface
	OnTaxiway   LocateFeature = "taxiway"   // on a taxiway (or apron path)
	AtParking   LocateFeature = "parking"   // on a parking spot
	OnApproach  LocateFeature = "approach"  // airborne, in a runway end's approach corridor
	OnDeparture LocateFeature = "departure" // airborne, in a runway end's departure corridor
	NearAirport LocateFeature = "near"      // off its surfaces, within LocateNearMeters
)

// LocateQuery is a position to locate: on the ground or in the air, with
// the height above the ground and the track over it where known.
type LocateQuery struct {
	Position LatLon
	OnGround bool
	// HeightFt is the height above the ground in the air (0: unknown).
	HeightFt float64
	// Track is the direction of travel (degrees true) when HasTrack: it
	// tells an approach from a departure on the same line.
	Track    float64
	HasTrack bool
	// VerticalFpm is the vertical speed in the air: descending (below
	// −locateLevelFpm) it is no departure, climbing no approach; level or 0,
	// either.
	VerticalFpm float64
	// RunwayMeters is the runway the aircraft needs (0: any): the corridors
	// of shorter runways are not its own (a jet near a gliding field).
	RunwayMeters float64
}

// Location is the airport a position is at.
type Location struct {
	ICAO    string
	Feature LocateFeature
	// Name is the runway ("06/24"), runway end ("24": approach, departure),
	// parking spot label or taxiway name; "" where there is none.
	Name string
	// Meters is the distance to that feature's surface (0 on it), or for an
	// approach or departure the distance from the runway end.
	Meters float64
	// Alternatives are other airports in the same place (the same surface:
	// aliases, a heliport on the apron), by the same rules worse.
	Alternatives []string
}

// Locate limits: beyond LocateNearMeters from every airport's surfaces a
// position is at no airport. An approach corridor runs LocateApproachNM out
// from the threshold, a departure corridor LocateDepartureNM beyond the
// runway's far end, each LocateCorridorMeters either side of the centreline
// at the runway and widening at locateApproachDeg / locateDepartureDeg.
const (
	LocateNearMeters     = 5000.0
	LocateApproachNM     = 10.0
	LocateDepartureNM    = 5.0
	LocateCorridorMeters = 150.0
	locateApproachDeg    = 10.0
	locateDepartureDeg   = 15.0
	// locateLowFt: below this, in the air off every corridor, a position
	// is placed by the surface under it (a circuit, a low pass).
	locateLowFt = 300.0
	// locateTrackDeg: with a track, the runway end's direction within this.
	locateTrackDeg = 45.0
	// locateTieMeters: surfaces this close count as the same place.
	locateTieMeters = 1.0
	// locateLevelFpm: a vertical speed within this either way tells
	// neither an approach nor a departure.
	locateLevelFpm = 300.0
)

const metersPerNM = 1852.0

// Locate finds the airport a position is at among layouts (the airports
// around it, e.g. those within 15 km): on the ground, the airport whose
// surface (runways, taxiways, parking; the reference point of one without
// any) is nearest; in the air, the runway end whose approach or departure
// corridor it is in, else as on the ground. Where two airports share the
// surface (aliases, a strip inside a larger field) the larger one wins,
// then a four-letter ICAO code; the others are Alternatives. It reports
// false beyond LocateNearMeters of every airport.
func Locate(q LocateQuery, layouts []*Layout) (Location, bool) {
	if !q.OnGround {
		if loc, ok := locateAir(q, layouts, ""); ok {
			return loc, true
		}
	}
	type cand struct {
		loc  Location
		size float64
	}
	var cs []cand
	for _, l := range layouts {
		if l == nil {
			continue
		}
		loc := surface(l, q.Position)
		if loc.Meters <= LocateNearMeters {
			cs = append(cs, cand{loc, airportSize(l)})
		}
	}
	if len(cs) == 0 {
		return Location{}, false
	}
	sort.SliceStable(cs, func(i, j int) bool {
		a, b := cs[i], cs[j]
		if math.Abs(a.loc.Meters-b.loc.Meters) > locateTieMeters {
			return a.loc.Meters < b.loc.Meters
		}
		if a.size != b.size {
			return a.size > b.size
		}
		if ia, ib := icaoCode(a.loc.ICAO), icaoCode(b.loc.ICAO); ia != ib {
			return ia
		}
		return a.loc.ICAO < b.loc.ICAO
	})
	best := cs[0].loc
	for _, c := range cs[1:] {
		if c.loc.Meters-best.Meters <= locateTieMeters {
			best.Alternatives = append(best.Alternatives, c.loc.ICAO)
		}
	}
	// Airborne and well above the ground: near, not on, whatever is below.
	if !q.OnGround && q.HeightFt >= locateLowFt && best.Feature != NearAirport {
		best.Feature, best.Name = NearAirport, ""
	}
	return best, true
}

// surface is how far p is from l's surfaces, and the nearest of them.
func surface(l *Layout, p LatLon) Location {
	f := newLocalFrame(l.Latitude, l.Longitude)
	px, pz := f.xz(p)
	best := Location{ICAO: l.ICAO, Feature: NearAirport, Meters: math.Hypot(px, pz)}
	if len(l.Runways) == 0 && len(l.TaxiPaths) == 0 && len(l.Parking) == 0 {
		return best // no geometry: its reference point
	}
	best.Meters = math.Inf(1)
	take := func(d float64, feat LocateFeature, name string) {
		d = math.Max(0, d)
		if d < best.Meters {
			best.Meters, best.Feature, best.Name = d, feat, name
		}
	}
	for _, r := range l.Runways {
		x0, z0 := f.xz(r.Primary.Threshold)
		x1, z1 := f.xz(r.Secondary.Threshold)
		length := math.Hypot(x1-x0, z1-z0)
		if length == 0 {
			take(math.Hypot(px-x0, pz-z0)-r.Width/2, OnRunway, r.Name())
			continue
		}
		ux, uz := (x1-x0)/length, (z1-z0)/length
		along := (px-x0)*ux + (pz-z0)*uz
		off := math.Abs((px-x0)*uz - (pz-z0)*ux)
		take(math.Hypot(math.Max(0, math.Max(-along, along-length)), math.Max(0, off-r.Width/2)), OnRunway, r.Name())
	}
	for _, pk := range l.Parking {
		x, z := f.xz(pk.Position)
		take(math.Hypot(px-x, pz-z)-math.Max(pk.Radius, 5), AtParking, pk.Label())
	}
	for _, tp := range l.TaxiPaths {
		a, b, ok := l.PathEndpoints(tp)
		if !ok {
			continue
		}
		ax, az := f.xz(a)
		bx, bz := f.xz(b)
		take(segmentDistance(ax, az, bx, bz, px, pz)-math.Max(tp.Width, 10)/2, OnTaxiway, l.PathName(tp))
	}
	if math.IsInf(best.Meters, 1) {
		best.Meters, best.Feature = math.Hypot(px, pz), NearAirport
	}
	return best
}

// locateAir finds the approach or departure corridor (want: only that
// kind, "" either) p is in, the best
// placed of all (nearest the centreline for its width, then the nearer,
// at the height expected there). Airports scoring the same (aliases with
// the same runway) are ranked as on the ground.
func locateAir(q LocateQuery, layouts []*Layout, want LocateFeature) (Location, bool) {
	type cand struct {
		loc         Location
		score, size float64
	}
	var cs []cand
	for _, l := range layouts {
		if l == nil {
			continue
		}
		var lb Location // this airport's best corridor
		ls := math.Inf(1)
		f := newLocalFrame(l.Latitude, l.Longitude)
		px, pz := f.xz(q.Position)
		for _, r := range l.Runways {
			if r.Length < q.RunwayMeters {
				continue // too short for this aircraft
			}
			for _, e := range [2]struct{ from, to RunwayEnd }{{r.Primary, r.Secondary}, {r.Secondary, r.Primary}} {
				// e.from: the end landed on or departed from, flying towards e.to.
				x0, z0 := f.xz(e.from.Threshold)
				x1, z1 := f.xz(e.to.Threshold)
				length := math.Hypot(x1-x0, z1-z0)
				if length == 0 {
					continue
				}
				ux, uz := (x1-x0)/length, (z1-z0)/length
				along := (px-x0)*ux + (pz-z0)*uz
				off := math.Abs((px-x0)*uz - (pz-z0)*ux)
				if q.HasTrack && math.Abs(angleDiff(q.Track, math.Atan2(ux, uz)*180/math.Pi)) > locateTrackDeg {
					continue
				}
				// The height against what the corridor's profile expects there
				// (a 3° glide path, a 7° climb): another airport's corridor
				// crossing at the same place expects another height.
				fit := func(want, slack float64) float64 {
					if q.HeightFt <= 0 {
						return 0
					}
					return math.Abs(q.HeightFt-want) / (want + slack)
				}
				if before := -along; want != OnDeparture && before >= 0 && before <= LocateApproachNM*metersPerNM && q.VerticalFpm <= locateLevelFpm {
					w := LocateCorridorMeters + before*math.Tan(locateApproachDeg*math.Pi/180)
					want := before / 0.3048 * math.Tan(3*math.Pi/180)
					if off <= w && (q.HeightFt <= 0 || q.HeightFt <= 500+2*want) {
						if s := off/w + 0.3*before/(LocateApproachNM*metersPerNM) + fit(want, 300); s < ls {
							ls, lb = s, Location{ICAO: l.ICAO, Feature: OnApproach, Name: e.from.Name, Meters: before}
						}
					}
				}
				if beyond := along - length; want != OnApproach && beyond >= 0 && beyond <= LocateDepartureNM*metersPerNM && q.VerticalFpm >= -locateLevelFpm {
					w := LocateCorridorMeters + beyond*math.Tan(locateDepartureDeg*math.Pi/180)
					want := 50 + beyond/0.3048*math.Tan(7*math.Pi/180)
					if off <= w && (q.HeightFt <= 0 || q.HeightFt <= 1000+2*want) {
						if s := off/w + 0.3*beyond/(LocateDepartureNM*metersPerNM) + fit(want, 500); s < ls {
							ls, lb = s, Location{ICAO: l.ICAO, Feature: OnDeparture, Name: e.from.Name, Meters: beyond}
						}
					}
				}
			}
		}
		if !math.IsInf(ls, 1) {
			cs = append(cs, cand{lb, ls, airportSize(l)})
		}
	}
	if len(cs) == 0 {
		return Location{}, false
	}
	sort.SliceStable(cs, func(i, j int) bool {
		a, b := cs[i], cs[j]
		if math.Abs(a.score-b.score) > locateTieScore {
			return a.score < b.score
		}
		if a.size != b.size {
			return a.size > b.size
		}
		if ia, ib := icaoCode(a.loc.ICAO), icaoCode(b.loc.ICAO); ia != ib {
			return ia
		}
		return a.loc.ICAO < b.loc.ICAO
	})
	best := cs[0].loc
	for _, c := range cs[1:] {
		if c.score-cs[0].score <= locateTieScore {
			best.Alternatives = append(best.Alternatives, c.loc.ICAO)
		}
	}
	return best, true
}

// locateTieScore: corridor scores this close count as the same place.
const locateTieScore = 0.01

// airportSize ranks airports sharing a surface: runway length plus a
// stand's worth (30 m) per parking spot.
func airportSize(l *Layout) float64 {
	s := 30 * float64(len(l.Parking))
	for _, r := range l.Runways {
		s += r.Length
	}
	return s
}

// icaoCode reports a four-letter code, not a local or generated ident.
func icaoCode(s string) bool {
	if len(s) != 4 {
		return false
	}
	for i := 0; i < 4; i++ {
		if s[i] < 'A' || s[i] > 'Z' {
			return false
		}
	}
	return true
}

// angleDiff is b−a in (−180, 180].
func angleDiff(a, b float64) float64 {
	d := math.Mod(b-a+540, 360) - 180
	if d == -180 {
		return 180
	}
	return d
}
