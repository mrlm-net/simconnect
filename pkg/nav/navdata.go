//go:build windows
// +build windows

// Package nav holds enroute navigation data: fixes (waypoints, VORs,
// NDBs), the airway network crawled from the facility API, and routing
// over it.
package nav

import (
	"fmt"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// FixKind is the kind of an enroute fix, as the facility API spells it:
// 'W' waypoint (intersection), 'V' VOR, 'N' NDB.
type FixKind byte

// Fix kinds.
const (
	KindWaypoint FixKind = 'W'
	KindVOR      FixKind = 'V'
	KindNDB      FixKind = 'N'
)

func (k FixKind) String() string {
	switch k {
	case KindWaypoint, KindVOR, KindNDB:
		return string(rune(k))
	}
	return "?"
}

// MarshalText writes the kind as its letter ("W", "V", "N").
func (k FixKind) MarshalText() ([]byte, error) { return []byte(k.String()), nil }

// UnmarshalText reads a kind letter.
func (k *FixKind) UnmarshalText(b []byte) error {
	if len(b) != 1 {
		return fmt.Errorf("nav: bad fix kind %q", b)
	}
	*k = FixKind(b[0])
	return nil
}

// FixKey identifies a fix worldwide: identifiers repeat between regions,
// and a VOR and a waypoint may share one.
type FixKey struct {
	Ident  string  `json:"ident"`
	Region string  `json:"region"`
	Kind   FixKind `json:"kind"`
}

// Key builds a FixKey, upper-casing the identifiers.
func Key(ident, region string, kind FixKind) FixKey {
	return FixKey{Ident: strings.ToUpper(strings.TrimSpace(ident)), Region: strings.ToUpper(strings.TrimSpace(region)), Kind: kind}
}

// String formats the key as IDENT.REGION.KIND ("VOZ.LK.V").
func (k FixKey) String() string { return k.Ident + "." + k.Region + "." + k.Kind.String() }

// ParseFixKey reads IDENT, IDENT.REGION or IDENT.REGION.KIND (also with ':'
// separators); the kind defaults to a waypoint.
func ParseFixKey(s string) (FixKey, error) {
	p := strings.FieldsFunc(s, func(r rune) bool { return r == '.' || r == ':' })
	if len(p) == 0 || len(p) > 3 {
		return FixKey{}, fmt.Errorf("nav: bad fix %q", s)
	}
	k := Key(p[0], "", KindWaypoint)
	if len(p) > 1 {
		k.Region = strings.ToUpper(p[1])
	}
	if len(p) > 2 {
		switch kind := FixKind(strings.ToUpper(p[2])[0]); kind {
		case KindWaypoint, KindVOR, KindNDB:
			k.Kind = kind
		default:
			return FixKey{}, fmt.Errorf("nav: bad fix kind in %q", s)
		}
	}
	return k, nil
}

// Fix is one enroute fix: a named waypoint, a VOR or an NDB.
type Fix struct {
	Ident    string         `json:"ident"`
	Region   string         `json:"region"`
	Kind     FixKind        `json:"kind"`
	Position airport.LatLon `json:"position"`
	// Type is the WAYPOINT record's TYPE (see WaypointType); 0 when the fix
	// has no waypoint record (a VOR or NDB off the airway network).
	Type WaypointType `json:"type,omitempty"`
	// Freq is the navaid frequency: MHz for a VOR, kHz for an NDB.
	Freq float64 `json:"freq,omitempty"`
	// Name is the navaid's name ("VOZICE"); waypoints have none.
	Name string `json:"name,omitempty"`
	// Terminal marks a terminal-area waypoint (IS_TERMINAL_WPT).
	Terminal bool `json:"terminal,omitempty"`
}

// Key returns the fix's key.
func (f Fix) Key() FixKey { return FixKey{Ident: f.Ident, Region: f.Region, Kind: f.Kind} }

// WaypointType is the WAYPOINT record's TYPE.
type WaypointType int32

// Waypoint types.
const (
	WaypointNone WaypointType = iota
	WaypointNamed
	WaypointUnnamed
	WaypointVOR
	WaypointNDB
	WaypointOffRoute
	WaypointIAF
	WaypointFAF
	WaypointRNAV
	WaypointVFR
)

// AirwayType is the ROUTE record's TYPE: low (victor), high (jet) or both.
type AirwayType int32

// Airway types.
const (
	AirwayNone AirwayType = iota
	AirwayVictor
	AirwayJet
	AirwayBoth
)

func (t AirwayType) String() string {
	switch t {
	case AirwayVictor:
		return "victor"
	case AirwayJet:
		return "jet"
	case AirwayBoth:
		return "both"
	}
	return "none"
}

// FixRef is a neighbouring fix named by a ROUTE record, with the position
// and minimum altitude the record gives for it.
type FixRef struct {
	Key      FixKey         `json:"key"`
	Position airport.LatLon `json:"position"`
	// MinAltM is the record's minimum altitude (NEXT_/PREV_ALTITUDE) in
	// meters; 0 when not given, as on most Czech (LK) segments.
	MinAltM float64 `json:"minAltM,omitempty"`
}

// RouteLink is one ROUTE record of a waypoint: the airway through it and
// its previous and next fixes (either is nil at the airway's ends).
type RouteLink struct {
	Airway string     `json:"airway"`
	Type   AirwayType `json:"type"`
	Prev   *FixRef    `json:"prev,omitempty"`
	Next   *FixRef    `json:"next,omitempty"`
}

// NavResult is a loaded fix with its airway links. Found is false when the
// simulator knows no such fix (the request raised an exception or timed
// out); a VOR or NDB with no waypoint record still loads, without links.
type NavResult struct {
	Key    FixKey
	Fix    Fix
	Routes []RouteLink
	Found  bool
}
