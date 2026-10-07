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
// Use takes the envelope or the bare items array, or the MyCrew API's
// envelope (UseSet).
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
	// shipped is the shipped item of an id as JSON fields; set the API's
	// set the table is fed from, aliases its fields renamed (ForSet).
	shipped func(id string) (map[string]json.RawMessage, bool)
	set     string
	aliases map[string]string
}

// ForSet feeds t from the MyCrew API's set (UseSet): the API's payload
// fields named in aliases are taken, renamed to t's (none: every field as
// it is).
func (t Table) ForSet(set string, aliases map[string]string) Table {
	t.set, t.aliases = set, aliases
	return t
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
	if api, ok, err := apiItems(items); err != nil {
		return fmt.Errorf("dict: %s: %w", name, err)
	} else if ok {
		if items, err = t.fromAPI(api); err != nil {
			return fmt.Errorf("dict: %s: %w", name, err)
		}
	}
	if err := t.use(items); err != nil {
		return fmt.Errorf("dict: %s: %w", name, err)
	}
	return nil
}

// UseSet feeds a set of the MyCrew API (GET /v1/aviation/{set}) to every
// table fed from it (ForSet): {"items": [{"key": "A319", "closed": false,
// "deprecated": false, "payload": {…}}]}. Closed and deprecated items are
// left out; key is the item's id, and the payload's fields replace the
// shipped item's one by one, so a field the API leaves out keeps its
// shipped value. It returns the tables fed (none: no table takes the set).
func UseSet(set string, data []byte) ([]string, error) {
	mu.Lock()
	var fed []string
	for n, t := range tables {
		if t.set == set {
			fed = append(fed, n)
		}
	}
	mu.Unlock()
	sort.Strings(fed)
	var errs []error
	for _, n := range fed {
		errs = append(errs, Use(n, data))
	}
	return fed, errors.Join(errs...)
}

// Sets are the API's sets the tables are fed from, sorted.
func Sets() []string {
	mu.Lock()
	defer mu.Unlock()
	var out []string
	for _, t := range tables {
		if t.set != "" && !slices.Contains(out, t.set) {
			out = append(out, t.set)
		}
	}
	sort.Strings(out)
	return out
}

// apiItem is an item of the MyCrew API.
type apiItem struct {
	Key        string                     `json:"key"`
	Closed     bool                       `json:"closed"`
	Deprecated bool                       `json:"deprecated"`
	Payload    map[string]json.RawMessage `json:"payload"`
}

// apiItems are raw's items when they are the API's (each with a payload,
// null included): those with a payload; an item without a key is an
// error, not a reason to read the set as the table's own items (#28).
func apiItems(raw json.RawMessage) ([]apiItem, bool, error) {
	var fields []map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || len(fields) == 0 {
		return nil, false, nil
	}
	for _, f := range fields {
		if _, ok := f["payload"]; !ok {
			return nil, false, nil
		}
	}
	var items []apiItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, true, err
	}
	out := items[:0]
	for i, it := range items {
		if it.Key == "" {
			return nil, true, fmt.Errorf("item %d: no key", i)
		}
		if it.Payload == nil {
			continue // nothing given: the shipped item stays
		}
		out = append(out, it)
	}
	return out, true, nil
}

// fromAPI makes the API's items t's: each the shipped item of its key,
// with the payload's fields (renamed by aliases) over it.
func (t Table) fromAPI(items []apiItem) (json.RawMessage, error) {
	var out []map[string]json.RawMessage
	for _, it := range items {
		if it.Closed || it.Deprecated {
			continue
		}
		obj := map[string]json.RawMessage{}
		if t.shipped != nil {
			if s, ok := t.shipped(it.Key); ok {
				obj = s
			}
		}
		for k, v := range it.Payload {
			if t.aliases != nil {
				var ok bool
				if k, ok = t.aliases[k]; !ok {
					continue
				}
			}
			if s := string(v); s == "null" || s == `""` {
				continue // not given: the shipped value
			}
			obj[k] = v
		}
		key, err := json.Marshal(it.Key)
		if err != nil {
			return nil, err
		}
		obj[t.ID] = key
		out = append(out, obj)
	}
	return json.Marshal(out)
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
// Use keeps the shipped order and appends new items; an item of a shipped
// id is read onto a copy of the shipped item, so it replaces only the
// values it gives (local wins per value). An item without an id is
// refused.
func Keyed[T any](name, idField, source, licence string, shipped func() []T, id func(T) string, apply func([]T)) Table {
	return Table{Name: name, ID: idField, Source: source, Licence: licence,
		export: func() (any, error) { return shipped(), nil },
		use: func(raw json.RawMessage) error {
			var raws []json.RawMessage
			if err := json.Unmarshal(raw, &raws); err != nil {
				return err
			}
			base := shipped()
			at := map[string]int{}
			for i, b := range base {
				at[id(b)] = i
			}
			items := make([]T, 0, len(raws))
			for n, r := range raws {
				var item T
				if err := json.Unmarshal(r, &item); err != nil {
					return fmt.Errorf("item %d: %w", n, err)
				}
				key := id(item)
				if key == "" {
					return fmt.Errorf("item %d: no %s", n, idField)
				}
				if i, ok := at[key]; ok {
					over, err := overlay(base[i], r)
					if err != nil {
						return fmt.Errorf("item %s: %w", key, err)
					}
					item = over
				}
				items = append(items, item)
			}
			apply(Merge(base, items, id))
			return nil
		},
		reset: func() { apply(shipped()) },
		shipped: func(key string) (map[string]json.RawMessage, bool) {
			for _, it := range shipped() {
				if id(it) != key {
					continue
				}
				b, err := json.Marshal(it)
				var m map[string]json.RawMessage
				if err != nil || json.Unmarshal(b, &m) != nil {
					return nil, false
				}
				return m, true
			}
			return nil, false
		},
	}
}

// overlay is item with r (an item's JSON) read onto a deep copy of it: the
// fields r gives replace item's, the others stay. The copy goes through
// JSON, so the shipped item (and maps it shares) is never written to.
func overlay[T any](item T, r json.RawMessage) (T, error) {
	var out T
	b, err := json.Marshal(item)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return out, err
	}
	if err := json.Unmarshal(r, &out); err != nil {
		return out, err
	}
	return out, nil
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
