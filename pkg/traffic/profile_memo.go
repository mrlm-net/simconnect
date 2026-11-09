package traffic

import (
	"sync"
	"unsafe"
)

// profileMemo keeps ProfileFor's answers by title: the same few titles are
// resolved again and again (doors, camera, the picture, keep-clear, every
// scan or tick), each an upper-casing and a scan of the known types. A
// profile is a plain value (no slices, maps or pointers), so a copy is
// handed out. The memo is dropped when the type table is replaced (a
// dictionary reload), told by the table's backing array.
var profileMemo = &profileCache{}

// profileMemoMax: a memo this big is started afresh (titles are few; a
// caller passing ever new strings must not grow it without end).
const profileMemoMax = 4096

type profileCache struct {
	mu    sync.Mutex
	table *typeSpec // the type table the entries were resolved against
	n     int
	m     map[string]AircraftProfile
}

func tableID(types []typeSpec) (*typeSpec, int) {
	return unsafe.SliceData(types), len(types)
}

func (c *profileCache) get(types []typeSpec, model string) (AircraftProfile, bool) {
	t, n := tableID(types)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil || c.table != t || c.n != n {
		return AircraftProfile{}, false
	}
	p, ok := c.m[model]
	return p, ok
}

func (c *profileCache) put(types []typeSpec, model string, p AircraftProfile) {
	t, n := tableID(types)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil || c.table != t || c.n != n || len(c.m) >= profileMemoMax {
		c.m, c.table, c.n = map[string]AircraftProfile{}, t, n
	}
	c.m[model] = p
}
