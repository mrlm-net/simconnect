package traffic

import (
	"math"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// Real-world traffic (#841): aircraft seen by a feed (ADS-B), flown by the
// engine in place of the generated schedule. A feed gives no origin or
// destination: unknown stays "" (shown as unknown), never guessed.

// Observed is one aircraft of a feed's snapshot.
type Observed struct {
	// ID is the aircraft's own (the ICAO 24-bit address): one aircraft is
	// one flight at a time, never spawned twice.
	ID           string    `json:"id"`
	Callsign     string    `json:"callsign,omitempty"`
	Registration string    `json:"registration,omitempty"`
	Type         string    `json:"type,omitempty"` // ICAO type designator
	Lat          float64   `json:"lat"`
	Lon          float64   `json:"lon"`
	AltFt        float64   `json:"altFt"`
	GroundKts    float64   `json:"groundKts"`
	TrackDeg     float64   `json:"trackDeg"`
	VSFpm        float64   `json:"vsFpm"`
	OnGround     bool      `json:"onGround"`
	SeenAt       time.Time `json:"seenAt"`
	// Kind is "arrival", "departure", "parked" or "overflight"; ""
	// inferred from where it is and how it moves (ClassifyObserved).
	Kind        string `json:"kind,omitempty"`
	Origin      string `json:"origin,omitempty"`
	Destination string `json:"destination,omitempty"`
	// DepartAt: a departure's push; zero now.
	DepartAt time.Time `json:"departAt,omitempty"`
}

// Sighting is what a flight keeps of the aircraft it flies (Flight's
// Observed): the feed's sighting.
type Sighting struct {
	ID           string         `json:"id"`
	Registration string         `json:"registration,omitempty"`
	Position     airport.LatLon `json:"position"`
	AltFt        float64        `json:"altFt"`
	GroundKts    float64        `json:"groundKts"`
	TrackDeg     float64        `json:"trackDeg"`
	VSFpm        float64        `json:"vsFpm"`
	OnGround     bool           `json:"onGround"`
	SeenAt       time.Time      `json:"seenAt"`
}

// Sighting is o's sighting.
func (o Observed) Sighting() Sighting {
	return Sighting{ID: o.ID, Registration: o.Registration, Position: airport.LatLon{Lat: o.Lat, Lon: o.Lon},
		AltFt: o.AltFt, GroundKts: o.GroundKts, TrackDeg: o.TrackDeg, VSFpm: o.VSFpm, OnGround: o.OnGround, SeenAt: o.SeenAt}
}

// ObservedProjectMax: a sighting is projected at most this far ahead.
const ObservedProjectMax = 10 * time.Minute

// At is where the aircraft is at t: on along its track at its ground
// speed and vertical rate since it was seen (at most ObservedProjectMax),
// not below the ground.
func (o Sighting) At(t time.Time) (airport.LatLon, float64) {
	dt := t.Sub(o.SeenAt)
	if o.SeenAt.IsZero() || dt < 0 {
		dt = 0
	}
	dt = min(dt, ObservedProjectMax)
	h := dt.Hours()
	pos := o.Position
	if o.GroundKts > 0 && !o.OnGround {
		lat, lon := calc.DisplaceByHeading(pos.Lat, pos.Lon, o.TrackDeg, o.GroundKts*h*1852)
		pos = airport.LatLon{Lat: lat, Lon: lon}
	}
	return pos, math.Max(0, o.AltFt+o.VSFpm*h*60)
}

// Observed kinds.
const (
	ObservedArrival    = "arrival"
	ObservedDeparture  = "departure"
	ObservedParked     = "parked"
	ObservedOverflight = "overflight"
)

// Classification limits (ClassifyObserved).
const (
	// ObservedFieldNM: on the ground within this of the field, at it.
	ObservedFieldNM = 3.0
	// ObservedTaxiKts: on the ground faster than this, moving (taxiing).
	ObservedTaxiKts = 3.0
	// ObservedArrivalNM: airborne within this, heading for the field
	// (ObservedClosingDeg) and not climbing away, an arrival.
	ObservedArrivalNM  = 150.0
	ObservedClosingDeg = 60.0
	// ObservedClimbFpm: climbing faster than this near the field, a
	// departure already off (not ours to fly).
	ObservedClimbFpm = 500.0
)

// ClassifyObserved is what o is at the field at fieldPos: its Kind when
// given, else parked (on the ground at the field, still), departure (on
// the ground at the field, moving), arrival (airborne, within
// ObservedArrivalNM, heading for the field, not climbing away) or
// overflight. "" with a reason when it is none of them (on the ground
// elsewhere, climbing out).
func ClassifyObserved(o Observed, field airport.LatLon) (string, string) {
	if k := strings.ToLower(strings.TrimSpace(o.Kind)); k != "" {
		switch k {
		case ObservedArrival, ObservedDeparture, ObservedParked, ObservedOverflight:
			return k, ""
		}
		return "", "unknown kind " + o.Kind
	}
	d := calc.HaversineNM(o.Lat, o.Lon, field.Lat, field.Lon)
	if o.OnGround {
		switch {
		case d > ObservedFieldNM:
			return "", "on the ground elsewhere"
		case o.GroundKts > ObservedTaxiKts:
			return ObservedDeparture, ""
		}
		return ObservedParked, ""
	}
	toField := calc.BearingDegrees(o.Lat, o.Lon, field.Lat, field.Lon)
	off := math.Abs(math.Mod(o.TrackDeg-toField+540, 360) - 180)
	switch {
	case d <= ObservedArrivalNM && off <= ObservedClosingDeg && o.VSFpm <= ObservedClimbFpm:
		return ObservedArrival, ""
	case d <= 30 && o.VSFpm > ObservedClimbFpm:
		return "", "climbing out"
	}
	return ObservedOverflight, ""
}
