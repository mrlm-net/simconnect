package airport

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Cache holds loaded layouts by ICAO code and builds each airport's taxi
// graph at most once, on first use. It is safe for concurrent use.
//
// Entries never expire on their own; call Invalidate (or Put a newer Layout)
// after scenery or navdata changes.
type Cache struct {
	mu      sync.Mutex
	entries map[string]*cacheEntry
}

type cacheEntry struct {
	layout *Layout
	once   sync.Once
	graph  *Graph
	err    error
}

// NewCache creates an empty Cache.
func NewCache() *Cache {
	return &Cache{entries: map[string]*cacheEntry{}}
}

func key(icao string) string { return strings.ToUpper(strings.TrimSpace(icao)) }

// Put stores l, replacing any earlier layout (and graph) for its ICAO code.
func (c *Cache) Put(l *Layout) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key(l.ICAO)] = &cacheEntry{layout: l}
}

// Layout returns the cached layout for icao.
func (c *Cache) Layout(icao string) (*Layout, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.entries[key(icao)]
	if e == nil {
		return nil, false
	}
	return e.layout, true
}

// Graph returns the taxi graph for a cached layout, building it on first use.
// Concurrent callers for the same airport share one build.
func (c *Cache) Graph(icao string) (*Graph, error) {
	c.mu.Lock()
	e := c.entries[key(icao)]
	c.mu.Unlock()
	if e == nil {
		return nil, fmt.Errorf("airport: %s is not loaded", key(icao))
	}
	e.once.Do(func() { e.graph, e.err = BuildGraph(e.layout) })
	return e.graph, e.err
}

// Invalidate removes an airport so it is loaded again next time.
func (c *Cache) Invalidate(icao string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key(icao))
}

// ICAOs returns the cached ICAO codes in sorted order.
func (c *Cache) ICAOs() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, 0, len(c.entries))
	for k := range c.entries {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
