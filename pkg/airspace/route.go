package airspace

import (
	"encoding/json"
	"math"
	"time"
)

// Need is what an airspace on a route asks of a VFR flight (Route).
type Need int

const (
	// NeedNone: nothing (class E: VFR flights need no clearance,
	// SERA.6001(a)(5)).
	NeedNone Need = iota
	// NeedClearance: an ATC clearance before entering (classes B, C and D,
	// SERA.6001(a)(2)–(4)).
	NeedClearance
	// NeedAvoid: keep out (restricted, prohibited). The data has no
	// activation times, so an area counts as active always: over-cautious.
	NeedAvoid
	// NeedInform: know it is there (MOA, warning, alert areas).
	NeedInform
	// NeedUnknown: a type without a meaning.
	NeedUnknown
)

var needNames = [...]string{"none", "clearance", "avoid", "inform", "unknown"}

func (n Need) String() string {
	if n >= 0 && int(n) < len(needNames) {
		return needNames[n]
	}
	return "unknown"
}

// MarshalText makes the need its name in JSON.
func (n Need) MarshalText() ([]byte, error) { return []byte(n.String()), nil }

// VFRNeeds is what each type asks of a VFR flight by default, by the
// SERA classes; RouteNeeds replaces it per type where a country differs
// (US class C asks only for two-way radio contact).
func VFRNeeds() map[BoundaryType]Need {
	return map[BoundaryType]Need{
		ClassB: NeedClearance, ClassC: NeedClearance, ClassD: NeedClearance, ClassE: NeedNone,
		Restricted: NeedAvoid, Prohibited: NeedAvoid,
		MOA: NeedInform, Warning: NeedInform, Alert: NeedInform,
	}
}

// PathPoint is a point of a planned path.
type PathPoint struct {
	Lat, Lon float64
	// AltM is the altitude, metres MSL.
	AltM float64
	// GroundM is the ground elevation under it, metres MSL, for AGL
	// limits; 0 when not known: AGL limits are then over sea level.
	GroundM float64
}

// Entry is an airspace a path enters (Route).
type Entry struct {
	Airspace    *Airspace    `json:"-"`
	ID          uint64       `json:"id"`
	Name        string       `json:"name"`
	Type        BoundaryType `json:"type"`
	Designation string       `json:"designation,omitempty"`
	// Frequency is the record's, "" without one (null in every record
	// dumped so far).
	Frequency string `json:"frequency,omitempty"`
	// At is the first point sampled inside, AlongM its distance along the
	// path; Start: the path starts inside (AlongM 0).
	At     PathPoint `json:"at"`
	AlongM float64   `json:"alongM"`
	Start  bool      `json:"start,omitempty"`
	// FloorM and CeilingM are the limits over At's ground, metres MSL.
	FloorM   float64 `json:"floorM"`
	CeilingM float64 `json:"ceilingM"`
	Need     Need    `json:"need"`
}

// RouteOption configures Route.
type RouteOption func(*routeOpts)

type routeOpts struct {
	stepM float64
	needs map[BoundaryType]Need
}

// RouteStep samples the path every m metres (default routeStepM).
func RouteStep(m float64) RouteOption {
	return func(o *routeOpts) {
		if m > 0 {
			o.stepM = m
		}
	}
}

// RouteNeeds sets what types ask, over VFRNeeds.
func RouteNeeds(m map[BoundaryType]Need) RouteOption {
	return func(o *routeOpts) {
		for t, n := range m {
			o.needs[t] = n
		}
	}
}

// routeStepM: a 250 m sample misses at most a corner of an outline that
// short; a CTR boundary is found within 0.15 NM.
const routeStepM = 250.0

// Route lists the airspaces path enters, in order along it: sampled every
// RouteStep, an entry each time a sample is inside an airspace (floor to
// ceiling, Shape.Contains) after one that was not, or at the first sample.
// The airspaces are picked by the path's box and their shapes cached
// (Store.Shape); records without limits (NoLimits) are left out.
func (s *Store) Route(path []PathPoint, opts ...RouteOption) ([]Entry, error) {
	o := routeOpts{stepM: routeStepM, needs: VFRNeeds()}
	for _, f := range opts {
		f(&o)
	}
	if len(path) == 0 {
		return nil, nil
	}
	list, err := s.Area(pathBox(path))
	if err != nil {
		return nil, err
	}
	type cand struct {
		a  *Airspace
		sh *Shape
	}
	var cs []cand
	for _, a := range list {
		if a.NoLimits() {
			continue
		}
		if sh, err := s.Shape(a); err == nil {
			cs = append(cs, cand{a, sh})
		}
	}
	var out []Entry
	in := make([]bool, len(cs))
	visit := func(p PathPoint, along float64, first bool) {
		for i, c := range cs {
			now := c.sh.Contains(p.Lat, p.Lon, p.AltM, p.GroundM)
			if now && !in[i] {
				out = append(out, entryOf(c.a, p, along, first, o.needs))
			}
			in[i] = now
		}
	}
	visit(path[0], 0, true)
	along := 0.0
	for i := 1; i < len(path); i++ {
		a, b := path[i-1], path[i]
		b.Lon = near(b.Lon, a.Lon)
		d := distM(a, b)
		n := max(1, int(math.Ceil(d/o.stepM)))
		for k := 1; k <= n; k++ {
			f := float64(k) / float64(n)
			p := PathPoint{
				Lat: a.Lat + (b.Lat-a.Lat)*f, Lon: wrapLon(a.Lon + (b.Lon-a.Lon)*f),
				AltM: a.AltM + (b.AltM-a.AltM)*f, GroundM: a.GroundM + (b.GroundM-a.GroundM)*f,
			}
			visit(p, along+d*f, false)
		}
		along += d
	}
	return out, nil
}

func entryOf(a *Airspace, p PathPoint, along float64, start bool, needs map[BoundaryType]Need) Entry {
	n, ok := needs[a.Type]
	if !ok {
		n = NeedUnknown
	}
	return Entry{Airspace: a, ID: a.ID, Name: a.Name, Type: a.Type, Designation: a.Designation,
		Frequency: frequencyOf(a.Frequency), At: p, AlongM: along, Start: start,
		FloorM: a.Floor(p.GroundM), CeilingM: a.Ceiling(p.GroundM), Need: n}
}

// frequencyOf is a record's frequency as text: a JSON string or number,
// "" for null.
func frequencyOf(raw json.RawMessage) string {
	var v any
	if len(raw) == 0 || json.Unmarshal(raw, &v) != nil {
		return ""
	}
	switch f := v.(type) {
	case string:
		return f
	case float64:
		return string(raw)
	}
	return ""
}

// Ahead is the straight path from p on trackDeg (true) at gsKts for d,
// altitude and ground kept: a look-ahead for a flight without a plan
// (a VFR player).
func Ahead(p PathPoint, trackDeg, gsKts float64, d time.Duration) []PathPoint {
	distM := gsKts * 1852 * d.Hours()
	const earthR = 6371000.0
	lat1, lon1, brg, dr := p.Lat*math.Pi/180, p.Lon*math.Pi/180, trackDeg*math.Pi/180, distM/earthR
	lat2 := math.Asin(math.Sin(lat1)*math.Cos(dr) + math.Cos(lat1)*math.Sin(dr)*math.Cos(brg))
	lon2 := lon1 + math.Atan2(math.Sin(brg)*math.Sin(dr)*math.Cos(lat1), math.Cos(dr)-math.Sin(lat1)*math.Sin(lat2))
	q := p
	q.Lat, q.Lon = lat2*180/math.Pi, wrapLon(lon2*180/math.Pi)
	return []PathPoint{p, q}
}

// pathBox bounds path; a path crossing ±180 takes every longitude.
func pathBox(path []PathPoint) Box {
	b := Box{North: -90, South: 90, West: path[0].Lon, East: path[0].Lon}
	lon := path[0].Lon
	for i, p := range path {
		if i > 0 {
			lon = near(p.Lon, lon)
		}
		b.North, b.South = math.Max(b.North, p.Lat), math.Min(b.South, p.Lat)
		b.West, b.East = math.Min(b.West, lon), math.Max(b.East, lon)
	}
	if b.West < -180 || b.East > 180 {
		b.West, b.East = -180, 180
	}
	return b
}

// distM is the great-circle distance between two points in metres.
func distM(a, b PathPoint) float64 {
	la1, la2 := a.Lat*math.Pi/180, b.Lat*math.Pi/180
	dLat, dLon := la2-la1, (b.Lon-a.Lon)*math.Pi/180
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(la1)*math.Cos(la2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * 6371000 * math.Asin(math.Sqrt(math.Min(1, h)))
}
