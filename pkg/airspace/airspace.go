// Package airspace reads the simulator's airspace outlines as dumped in
// simconnect-data (airspaces/): the raw records exactly as the simulator
// gives them (Airspace), a Store that loads the 10° tiles an area needs by
// the index's boxes, and the geometry (Shape) that flattens an outline's
// arcs and circles and tells whether a point is inside, floor to ceiling.
//
// SimConnect has no airspace records; the dump comes from the simulator's
// JavaScript side (a nearest search for boundaries). The records stay as
// the simulator gave them: the cleanup (StoreSkipBroken,
// StorePreferNewerSet) is a Store option, off by default.
package airspace

import "encoding/json"

// BoundaryType is an airspace's kind, numbered as the simulator's
// BoundaryType enum. Values not listed are kept as they come.
type BoundaryType int

const (
	ClassB     BoundaryType = 3
	ClassC     BoundaryType = 4
	ClassD     BoundaryType = 5
	ClassE     BoundaryType = 6
	MOA        BoundaryType = 14
	Restricted BoundaryType = 15
	Prohibited BoundaryType = 16
	Warning    BoundaryType = 17
	// Alert follows the enum's order; only Canadian records carry it, not
	// checked against a source.
	Alert BoundaryType = 18
)

func (t BoundaryType) String() string {
	switch t {
	case ClassB:
		return "B"
	case ClassC:
		return "C"
	case ClassD:
		return "D"
	case ClassE:
		return "E"
	case MOA:
		return "MOA"
	case Restricted:
		return "restricted"
	case Prohibited:
		return "prohibited"
	case Warning:
		return "warning"
	case Alert:
		return "alert"
	}
	return "unknown"
}

// AltType is what a limit is measured from.
type AltType int

const (
	AltUnknown AltType = 0
	AltMSL     AltType = 1
	AltAGL     AltType = 2
)

// VectorType is the kind of an outline vector.
type VectorType int

const (
	// VectorStart begins an outline at its point.
	VectorStart VectorType = 1
	// VectorLine runs straight to its point.
	VectorLine VectorType = 2
	// VectorOrigin is the centre of the arc or circle after it.
	VectorOrigin VectorType = 3
	// VectorArcCW and VectorArcCCW run round the last origin to their
	// point; the radius is the origin's distance to the point before.
	VectorArcCW  VectorType = 4
	VectorArcCCW VectorType = 5
	// VectorCircle is a circle of Radius metres round the origin before it
	// (its own lat/lon are 0).
	VectorCircle VectorType = 6
)

// LatLong is a position as the simulator writes it (the longitude key is
// "long").
type LatLong struct {
	Lat  float64 `json:"lat"`
	Long float64 `json:"long"`
}

// Vector is one step of an outline.
type Vector struct {
	Type     VectorType `json:"type"`
	OriginID int        `json:"originId"`
	Lat      float64    `json:"lat"`
	Lon      float64    `json:"lon"`
	Radius   float64    `json:"radius"` // metres, VectorCircle only
}

// Airspace is one record of simconnect-data airspaces/<tile>.json, as the
// simulator gave it.
type Airspace struct {
	ID   uint64       `json:"id"`
	Name string       `json:"name"`
	Type BoundaryType `json:"type"`
	// MinAlt and MaxAlt are metres from MinAltType and MaxAltType. Flight
	// levels come as feet MSL (FL95 as 9,500 ft).
	MinAlt     float64 `json:"minAlt"`
	MaxAlt     float64 `json:"maxAlt"`
	MinAltType AltType `json:"minAltType"`
	MaxAltType AltType `json:"maxAltType"`
	// TopLeft and BottomRight bound it. A few reaching the antimeridian
	// span -180 to 180 (Box).
	TopLeft     LatLong `json:"topLeft"`
	BottomRight LatLong `json:"bottomRight"`
	// Frequency is null in every record dumped; kept raw.
	Frequency json.RawMessage `json:"frequency"`
	// Designation is the area number of restricted, prohibited and
	// military areas.
	Designation string   `json:"designation"`
	Vectors     []Vector `json:"vectors"`
}

// DataSet is the simulator data set the record comes from (ID / 2^32).
func (a *Airspace) DataSet() uint32 { return uint32(a.ID >> 32) }

// Box is the record's bounding box.
func (a *Airspace) Box() Box {
	return boxOf(a.TopLeft, a.BottomRight)
}

// Broken reports a record without an outline start (no start or circle
// vector) or with an unknown altitude type: 1,890 records of the newer
// data sets, mostly Russia and the CIS (StoreSkipBroken).
func (a *Airspace) Broken() bool {
	if a.MinAltType == AltUnknown || a.MaxAltType == AltUnknown {
		return true
	}
	for _, v := range a.Vectors {
		if v.Type == VectorStart || v.Type == VectorCircle {
			return false
		}
	}
	return true
}

// Floor is the lower limit in metres MSL over ground at groundM metres
// MSL; an unknown type counts as MSL.
func (a *Airspace) Floor(groundM float64) float64 {
	return limit(a.MinAlt, a.MinAltType, groundM)
}

// Ceiling is the upper limit in metres MSL over ground at groundM.
func (a *Airspace) Ceiling(groundM float64) float64 {
	return limit(a.MaxAlt, a.MaxAltType, groundM)
}

func limit(alt float64, t AltType, groundM float64) float64 {
	if t == AltAGL {
		return alt + groundM
	}
	return alt
}
