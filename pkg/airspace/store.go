package airspace

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"sort"
	"sync"
)

// Box is a latitude/longitude box in degrees. West > East crosses the
// antimeridian.
type Box struct {
	North, South, West, East float64
}

// wideLonDeg: a record box wider than this spans the antimeridian the long
// way round (Magadan CTA, Bay CTA C, Nadi OCA/CTA at -180 to 180,
// Sankt-Peterburg CTA to 180): it matches every longitude at its
// latitudes.
const wideLonDeg = 90.0

func boxOf(tl, br LatLong) Box {
	return Box{North: tl.Lat, South: br.Lat, West: tl.Long, East: br.Long}
}

// BoxAround is the box within radiusM metres of lat/lon.
func BoxAround(lat, lon, radiusM float64) Box {
	dLat := radiusM / metersPerDeg
	b := Box{North: math.Min(90, lat+dLat), South: math.Max(-90, lat-dLat), West: -180, East: 180}
	if c := math.Cos(lat * math.Pi / 180); c > 1e-6 && dLat/c < 180 {
		b.West, b.East = wrapLon(lon-dLat/c), wrapLon(lon+dLat/c)
	}
	return b
}

// Overlaps reports that b and o share a point. A box wider than 90° of
// longitude (a record reaching the antimeridian) matches every longitude.
func (b Box) Overlaps(o Box) bool {
	if b.South > o.North || o.South > b.North {
		return false
	}
	if b.wide() || o.wide() {
		return true
	}
	for _, x := range b.lonSpans() {
		for _, y := range o.lonSpans() {
			if x[0] <= y[1] && y[0] <= x[1] {
				return true
			}
		}
	}
	return false
}

func (b Box) wide() bool { return b.West <= b.East && b.East-b.West > wideLonDeg }

// lonSpans splits a box crossing the antimeridian in two.
func (b Box) lonSpans() [][2]float64 {
	if b.West <= b.East {
		return [][2]float64{{b.West, b.East}}
	}
	return [][2]float64{{b.West, 180}, {-180, b.East}}
}

// IndexEntry is one line of airspaces/index.json.
type IndexEntry struct {
	ID          uint64       `json:"id"`
	Name        string       `json:"name"`
	Type        BoundaryType `json:"type"`
	TopLeft     LatLong      `json:"topLeft"`
	BottomRight LatLong      `json:"bottomRight"`
	// File is the tile the record is in, beside index.json.
	File string `json:"file"`
}

// Box is the entry's bounding box.
func (e IndexEntry) Box() Box { return boxOf(e.TopLeft, e.BottomRight) }

// LoadIndex reads index.json from fsys, the airspaces/ directory.
func LoadIndex(fsys fs.FS) ([]IndexEntry, error) {
	b, err := fs.ReadFile(fsys, "index.json")
	if err != nil {
		return nil, fmt.Errorf("airspace: %w", err)
	}
	var idx []IndexEntry
	if err := json.Unmarshal(b, &idx); err != nil {
		return nil, fmt.Errorf("airspace: index.json: %w", err)
	}
	return idx, nil
}

// StoreOption configures a Store.
type StoreOption func(*Store)

// StoreSkipBroken leaves out the broken records (Airspace.Broken).
func StoreSkipBroken() StoreOption { return func(s *Store) { s.skipBroken = true } }

// StorePreferNewerSet leaves out a record when one of a newer data set
// (Airspace.DataSet) has the same type and the same box within
// sameBoxDeg: over Czechia every airspace comes twice, once per data set,
// and the newer follows the AIP. Elsewhere the two sets were not compared.
func StorePreferNewerSet() StoreOption { return func(s *Store) { s.preferNewer = true } }

// sameBoxDeg: two records whose box edges are all this close (about 2 km)
// are the same airspace in two data sets (Ruzyne CTR: 0.0004° apart).
const sameBoxDeg = 0.02

// Store gives the airspaces of an area from a simconnect-data airspaces/
// directory: it picks the tiles by the index's boxes and decodes each
// tile once, when first needed. Safe for concurrent use.
//
//	s, err := airspace.Open(os.DirFS(filepath.Join(data, "airspaces")))
//	list, err := s.Area(airspace.BoxAround(50.1, 14.26, 30000))
type Store struct {
	fsys        fs.FS
	index       []IndexEntry
	skipBroken  bool
	preferNewer bool

	mu    sync.Mutex
	tiles map[string]map[uint64]*Airspace
}

// Open reads the index of fsys, the airspaces/ directory.
func Open(fsys fs.FS, opts ...StoreOption) (*Store, error) {
	idx, err := LoadIndex(fsys)
	if err != nil {
		return nil, err
	}
	s := &Store{fsys: fsys, index: idx, tiles: map[string]map[uint64]*Airspace{}}
	for _, o := range opts {
		o(s)
	}
	return s, nil
}

// Index is the store's index.
func (s *Store) Index() []IndexEntry { return s.index }

// Area returns the airspaces whose box overlaps box, by ID. The records
// are shared with the store: do not change them.
func (s *Store) Area(box Box) ([]*Airspace, error) {
	var out []*Airspace
	for _, e := range s.index {
		if !e.Box().Overlaps(box) {
			continue
		}
		tile, err := s.tile(e.File)
		if err != nil {
			return nil, err
		}
		a := tile[e.ID]
		if a == nil || s.skipBroken && a.Broken() {
			continue
		}
		out = append(out, a)
	}
	if s.preferNewer {
		out = newest(out)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// At returns the airspaces whose outline holds lat/lon (any altitude).
func (s *Store) At(lat, lon float64) ([]*Airspace, error) {
	list, err := s.Area(Box{North: lat, South: lat, West: lon, East: lon})
	if err != nil {
		return nil, err
	}
	var out []*Airspace
	for _, a := range list {
		sh, err := a.Shape()
		if err == nil && sh.Inside(lat, lon) {
			out = append(out, a)
		}
	}
	return out, nil
}

func (s *Store) tile(file string) (map[uint64]*Airspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.tiles[file]; ok {
		return t, nil
	}
	b, err := fs.ReadFile(s.fsys, file)
	if err != nil {
		return nil, fmt.Errorf("airspace: %w", err)
	}
	var list []Airspace
	if err := json.Unmarshal(b, &list); err != nil {
		return nil, fmt.Errorf("airspace: %s: %w", file, err)
	}
	t := make(map[uint64]*Airspace, len(list))
	for i := range list {
		t[list[i].ID] = &list[i]
	}
	s.tiles[file] = t
	return t, nil
}

// newest drops each record that a newer data set's record of the same
// type and box replaces (StorePreferNewerSet).
func newest(list []*Airspace) []*Airspace {
	// By north edge: only neighbours within sameBoxDeg can match.
	byN := append([]*Airspace(nil), list...)
	sort.Slice(byN, func(i, j int) bool { return byN[i].TopLeft.Lat < byN[j].TopLeft.Lat })
	replaced := map[uint64]bool{}
	for i, a := range byN {
		for j := i + 1; j < len(byN) && byN[j].TopLeft.Lat-a.TopLeft.Lat <= sameBoxDeg; j++ {
			b := byN[j]
			if a.Type != b.Type || a.DataSet() == b.DataSet() || !sameBox(a.Box(), b.Box()) {
				continue
			}
			if a.DataSet() < b.DataSet() {
				replaced[a.ID] = true
			} else {
				replaced[b.ID] = true
			}
		}
	}
	out := list[:0:0]
	for _, a := range list {
		if !replaced[a.ID] {
			out = append(out, a)
		}
	}
	return out
}

func sameBox(a, b Box) bool {
	return math.Abs(a.North-b.North) <= sameBoxDeg && math.Abs(a.South-b.South) <= sameBoxDeg &&
		math.Abs(a.West-b.West) <= sameBoxDeg && math.Abs(a.East-b.East) <= sameBoxDeg
}

// wrapLon brings lon into -180..180.
func wrapLon(lon float64) float64 {
	for lon > 180 {
		lon -= 360
	}
	for lon < -180 {
		lon += 360
	}
	return lon
}
