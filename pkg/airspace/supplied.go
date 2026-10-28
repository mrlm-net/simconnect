package airspace

import "fmt"

// Stores over records the caller has (#1260): an app without the
// simconnect-data tiles gets the same records live from the simulator's
// nearest search (the same fields as a tile, so its JSON decodes into
// []Airspace as a tile does).

// suppliedTile is the one in-memory tile of a store over supplied records.
const suppliedTile = ""

// Fetch is a store's source of the airspaces whose box overlaps box: what
// it has not seen yet, e.g. a nearest search in the simulator. A record
// listed twice, or one far from box, is kept once and filtered by the
// store.
type Fetch func(box Box) ([]*Airspace, error)

// NewStore is a Store over records, e.g. the simulator's answer decoded
// into []Airspace: Area, At, Containing, ClassAt and the options as over
// the tiles. A record listed more than once (by ID) is kept once, the
// first; nil ones are skipped. The records are shared with the store: do
// not change them.
func NewStore(records []*Airspace, opts ...StoreOption) *Store {
	s := &Store{tiles: map[string]map[uint64]*Airspace{suppliedTile: {}}}
	for _, o := range opts {
		o(s)
	}
	s.add(records)
	return s
}

// NewFetchStore is a Store that asks fetch for the airspaces of each box
// it is queried for, unless the box lies inside one asked for before, and
// keeps what came back. An error from fetch is the query's; the box is
// asked for again next time.
func NewFetchStore(fetch Fetch, opts ...StoreOption) *Store {
	s := NewStore(nil, opts...)
	s.fetch = fetch
	return s
}

// add puts records in the store's index and its in-memory tile.
func (s *Store) add(records []*Airspace) {
	s.mu.Lock()
	tile := s.tiles[suppliedTile]
	var fresh []*Airspace
	for _, a := range records {
		if a == nil || tile[a.ID] != nil {
			continue
		}
		tile[a.ID] = a
		fresh = append(fresh, a)
	}
	s.mu.Unlock()
	s.idxMu.Lock()
	defer s.idxMu.Unlock()
	for _, a := range fresh {
		s.index = append(s.index, IndexEntry{ID: a.ID, Name: a.Name, Type: a.Type, TopLeft: a.TopLeft, BottomRight: a.BottomRight, File: suppliedTile})
		if s.built {
			s.cell(int32(len(s.index) - 1))
		}
	}
}

// fetchFor asks the store's fetch for box unless a box asked for before
// holds it.
func (s *Store) fetchFor(box Box) error {
	if s.fetch == nil {
		return nil
	}
	s.fetchMu.Lock()
	defer s.fetchMu.Unlock()
	for _, b := range s.fetched {
		if b.holds(box) {
			return nil
		}
	}
	list, err := s.fetch(box)
	if err != nil {
		return fmt.Errorf("airspace: fetch: %w", err)
	}
	s.add(list)
	s.fetched = append(s.fetched, box)
	return nil
}

// holds reports that o lies inside b. A box crossing the antimeridian
// holds only the same box.
func (b Box) holds(o Box) bool {
	if b.West > b.East || o.West > o.East {
		return b == o
	}
	return o.North <= b.North && o.South >= b.South && o.West >= b.West && o.East <= b.East
}
