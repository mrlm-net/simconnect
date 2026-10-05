// Package dict makes the library's embedded tables replaceable at runtime
// (#768): airline telephony, aircraft types, wake categories, performance,
// airport names and limits, AIP unit names, airlines, GA types, systems
// profiles. Each package registers its tables; a host (the MyCrew app,
// fed from its API and a local cache) lists them (Names), exports the
// shipped copy as JSON to seed its own store (Export), and replaces a
// table with its data (Use): items merged by id over the shipped copy,
// so an item the data leaves out keeps its shipped value. Reset goes back
// to the shipped copy. Replacing is safe while other goroutines look up.
//
// The JSON of a table is an envelope:
//
//	{"name": "traffic.telephony", "id": "icao", "source": "…", "licence": "…",
//	 "items": [{"icao": "CSA", "telephony": "CSA-LINES", …}, …]}
//
// Use takes the envelope or the bare items array.
package dict

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
)

// ErrNoTable: no table has that name.
var ErrNoTable = errors.New("dict: no such table")

// Envelope is a table as JSON.
type Envelope struct {
	Name    string          `json:"name"`
	ID      string          `json:"id"` // the items' id field
	Source  string          `json:"source,omitempty"`
	Licence string          `json:"licence,omitempty"`
	Items   json.RawMessage `json:"items"`
}

// Table is one replaceable table.
type Table struct {
	Name, ID, Source, Licence string
	// export is the shipped items; use merges items (JSON) and applies
	// them; reset applies the shipped copy.
	export func() (any, error)
	use    func(items json.RawMessage) error
	reset  func()
}

var (
	mu     sync.Mutex
	tables = map[string]Table{}
)

// Register adds a table (each package's init).
func Register(t Table) {
	mu.Lock()
	defer mu.Unlock()
	tables[t.Name] = t
}

// Names are the tables, sorted.
func Names() []string {
	mu.Lock()
	defer mu.Unlock()
	out := make([]string, 0, len(tables))
	for n := range tables {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func table(name string) (Table, error) {
	mu.Lock()
	defer mu.Unlock()
	t, ok := tables[name]
	if !ok {
		return Table{}, fmt.Errorf("%w: %q", ErrNoTable, name)
	}
	return t, nil
}

// Export is the shipped copy of table name as an Envelope's JSON.
func Export(name string) ([]byte, error) {
	t, err := table(name)
	if err != nil {
		return nil, err
	}
	items, err := t.export()
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(Envelope{Name: t.Name, ID: t.ID, Source: t.Source, Licence: t.Licence, Items: raw}, "", "  ")
}

// Use replaces table name with data (an Envelope, or its items array):
// its items merged by id over the shipped copy.
func Use(name string, data []byte) error {
	t, err := table(name)
	if err != nil {
		return err
	}
	items := json.RawMessage(data)
	var env Envelope
	if json.Unmarshal(data, &env) == nil && len(env.Items) > 0 {
		if env.Name != "" && env.Name != name {
			return fmt.Errorf("dict: data of %q given for %q", env.Name, name)
		}
		items = env.Items
	}
	if err := t.use(items); err != nil {
		return fmt.Errorf("dict: %s: %w", name, err)
	}
	return nil
}

// Reset puts table name back to its shipped copy.
func Reset(name string) error {
	t, err := table(name)
	if err != nil {
		return err
	}
	t.reset()
	return nil
}

// Keyed makes a table of items T with an id: shipped is the embedded copy
// (in its order), id an item's id, apply makes items the table in use.
// Use keeps the shipped order, replaces items by id and appends new ones.
func Keyed[T any](name, idField, source, licence string, shipped func() []T, id func(T) string, apply func([]T)) Table {
	return Table{Name: name, ID: idField, Source: source, Licence: licence,
		export: func() (any, error) { return shipped(), nil },
		use: func(raw json.RawMessage) error {
			var items []T
			if err := json.Unmarshal(raw, &items); err != nil {
				return err
			}
			apply(Merge(shipped(), items, id))
			return nil
		},
		reset: func() { apply(shipped()) },
	}
}

// Merge is base with over's items replacing those of the same id and the
// others appended, in order.
func Merge[T any](base, over []T, id func(T) string) []T {
	out := slices.Clone(base)
	at := map[string]int{}
	for i, b := range out {
		at[id(b)] = i
	}
	for _, o := range over {
		if i, ok := at[id(o)]; ok {
			out[i] = o
			continue
		}
		at[id(o)] = len(out)
		out = append(out, o)
	}
	return out
}

// Value holds a table in use: Load is safe while Store replaces it.
type Value[T any] struct{ p atomic.Pointer[T] }

// Load is the table in use.
func (v *Value[T]) Load() T {
	if p := v.p.Load(); p != nil {
		return *p
	}
	var zero T
	return zero
}

// Store puts t in use.
func (v *Value[T]) Store(t T) { v.p.Store(&t) }
