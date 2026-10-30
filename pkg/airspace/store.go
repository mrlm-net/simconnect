package airspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"slices"
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
// sameBoxDeg, or the same name and the same box within sameNameBoxDeg:
// over Czechia every airspace comes twice, once per data set, and the
// newer follows the AIP. Elsewhere the two sets were not compared.
func StorePreferNewerSet() StoreOption { return func(s *Store) { s.preferNewer = true } }

// sameBoxDeg: two records whose box edges are all this close (about 2 km)
// are the same airspace in two data sets (Ruzyne CTR: 0.0004° apart).
// sameNameBoxDeg: with the same name, this close (PRAGUE CTA 2, E: the
// older set's from 1,000 ft MSL, 0.04° off the newer's from 1,000 ft AGL,
// left Prague without class G).
const (
	sameBoxDeg     = 0.02
	sameNameBoxDeg = 0.1
)

// Store gives the airspaces of an area from a simconnect-data airspaces/
// directory: it picks the tiles by the index's boxes and decodes each
// tile once, when first needed. NewStore and NewFetchStore give the same
// from records the caller has, or fetches (#1260). Safe for concurrent use.
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

	// cells: the index by cellDeg cell of each box, built on first use
	// (Area scanned all 56,419 entries per call); spread: the entries too
	// wide for cells, checked every time. idxMu guards them and index,
	// which supplied records grow (add).
	idxMu  sync.Mutex
	built  bool
	cells  map[int32][]int32
	spread []int32

	// fetch, for a store over a fetch function: what it is asked for a box
	// not inside one of fetched.
	fetchMu sync.Mutex
	fetch   Fetch
	fetched []Box

	shapeMu sync.Mutex
	shapes  map[uint64]*Shape
}

// cellDeg is the cell of the Store's index; a box over maxCells cells
// (FIRs, oceanic areas) is checked on every query instead.
const (
	cellDeg  = 1.0
	maxCells = 400
)

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
func (s *Store) Index() []IndexEntry {
	s.idxMu.Lock()
	defer s.idxMu.Unlock()
	return s.index
}

// Area returns the airspaces whose box overlaps box, by ID. The records
// are shared with the store: do not change them.
func (s *Store) Area(box Box) ([]*Airspace, error) {
	if err := s.fetchFor(box); err != nil {
		return nil, err
	}
	cand, index := s.candidatesOf(box)
	var out []*Airspace
	for _, i := range cand {
		e := index[i]
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

// candidates are the index entries that may overlap box, in index order:
// those in its cells and the spread ones; all for a box over maxCells.
func (s *Store) candidates(box Box) []int32 {
	c, _ := s.candidatesOf(box)
	return c
}

// candidatesOf is candidates and the index they are of.
func (s *Store) candidatesOf(box Box) ([]int32, []IndexEntry) {
	s.idxMu.Lock()
	defer s.idxMu.Unlock()
	if !s.built {
		s.buildCells()
	}
	keys, ok := cellsOf(box)
	if !ok {
		all := make([]int32, len(s.index))
		for i := range all {
			all[i] = int32(i)
		}
		return all, s.index
	}
	seen := map[int32]bool{}
	out := append([]int32(nil), s.spread...)
	for _, k := range keys {
		for _, i := range s.cells[k] {
			if !seen[i] {
				seen[i] = true
				out = append(out, i)
			}
		}
	}
	slices.Sort(out)
	return out, s.index
}

// buildCells puts the index in its cells; s.idxMu is held.
func (s *Store) buildCells() {
	s.cells, s.spread, s.built = map[int32][]int32{}, nil, true
	for i := range s.index {
		s.cell(int32(i))
	}
}

// cell puts index entry i in its cells, or the spread ones; s.idxMu is
// held.
func (s *Store) cell(i int32) {
	keys, ok := cellsOf(s.index[i].Box())
	if !ok {
		s.spread = append(s.spread, i)
		return
	}
	for _, k := range keys {
		s.cells[k] = append(s.cells[k], i)
	}
}

// cellsOf are the cells box covers; false for a wide box or one over
// maxCells.
func cellsOf(b Box) ([]int32, bool) {
	if b.wide() {
		return nil, false
	}
	lat0, lat1 := cellIndex(b.South, 90), cellIndex(b.North, 90)
	var keys []int32
	for _, sp := range b.lonSpans() {
		lon0, lon1 := cellIndex(sp[0], 180), cellIndex(sp[1], 180)
		if int((lat1-lat0+1)*(lon1-lon0+1))+len(keys) > maxCells {
			return nil, false
		}
		for la := lat0; la <= lat1; la++ {
			for lo := lon0; lo <= lon1; lo++ {
				keys = append(keys, la*1000+lo)
			}
		}
	}
	return keys, true
}

// cellIndex is the cell of v in -lim..lim (lim itself in the last one).
func cellIndex(v, lim float64) int32 {
	v = math.Max(-lim, math.Min(v, lim-cellDeg/2))
	return int32(math.Floor(v / cellDeg))
}

// At returns the airspaces whose outline holds lat/lon (any altitude).
func (s *Store) At(lat, lon float64) ([]*Airspace, error) {
	list, err := s.Area(Box{North: lat, South: lat, West: lon, East: lon})
	if err != nil {
		return nil, err
	}
	var out []*Airspace
	for _, a := range list {
		sh, err := s.Shape(a)
		if err == nil && sh.Inside(lat, lon) {
			out = append(out, a)
		}
	}
	return out, nil
}

// Shape is a.Shape(), built once per record and kept: outlines run to
// 10,000 vectors (Riga CTA, Vilnius CTA/UTA).
func (s *Store) Shape(a *Airspace) (*Shape, error) {
	s.shapeMu.Lock()
	sh, ok := s.shapes[a.ID]
	s.shapeMu.Unlock()
	if ok {
		if sh == nil {
			return nil, ErrNoOutline
		}
		return sh, nil
	}
	sh, err := a.Shape()
	if err != nil && !errors.Is(err, ErrNoOutline) {
		return nil, err
	}
	s.shapeMu.Lock()
	if s.shapes == nil {
		s.shapes = map[uint64]*Shape{}
	}
	s.shapes[a.ID] = sh
	s.shapeMu.Unlock()
	return sh, err
}

// Containing returns the airspaces holding a point at altM metres MSL over
// ground at groundM metres MSL, floor to ceiling, by ID. Records without
// limits (NoLimits) are left out.
func (s *Store) Containing(lat, lon, altM, groundM float64) ([]*Airspace, error) {
	at, err := s.At(lat, lon)
	if err != nil {
		return nil, err
	}
	out := at[:0]
	for _, a := range at {
		if !a.NoLimits() && altM >= a.Floor(groundM) && altM <= a.Ceiling(groundM) {
			out = append(out, a)
		}
	}
	return out, nil
}

// ClassAt is the class record (B–E) holding the point (Containing): the
// strictest where several do (LKPR at 2,000 ft: Ruzyne CTR, D, over the
// Prague CTA's E from 1,000 ft AGL), the lowest ID among equals. Nil
// where none holds it: the data has no class A or G records, so what
// that is (G in most of Europe, A above FL180 in the US, C above FL195
// under SERA.6001(b)) is the caller's.
func (s *Store) ClassAt(lat, lon, altM, groundM float64) (*Airspace, error) {
	in, err := s.Containing(lat, lon, altM, groundM)
	if err != nil {
		return nil, err
	}
	var best *Airspace
	for _, a := range in {
		if a.Type.IsClass() && (best == nil || a.Type < best.Type) {
			best = a
		}
	}
	return best, nil
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
	// By north edge: only neighbours within sameNameBoxDeg can match.
	byN := append([]*Airspace(nil), list...)
	sort.Slice(byN, func(i, j int) bool { return byN[i].TopLeft.Lat < byN[j].TopLeft.Lat })
	replaced := map[uint64]bool{}
	for i, a := range byN {
		for j := i + 1; j < len(byN) && byN[j].TopLeft.Lat-a.TopLeft.Lat <= sameNameBoxDeg; j++ {
			b := byN[j]
			tol := sameBoxDeg
			if a.Name == b.Name {
				tol = sameNameBoxDeg
			}
			if a.Type != b.Type || a.DataSet() == b.DataSet() || !sameBox(a.Box(), b.Box(), tol) {
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

func sameBox(a, b Box, tol float64) bool {
	return math.Abs(a.North-b.North) <= tol && math.Abs(a.South-b.South) <= tol &&
		math.Abs(a.West-b.West) <= tol && math.Abs(a.East-b.East) <= tol
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
