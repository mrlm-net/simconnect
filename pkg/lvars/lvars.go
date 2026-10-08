//go:build windows
// +build windows

// Package lvars writes local variables (L:vars) on the user aircraft through
// SimConnect, so an application can signal other add-ons — the aircraft's
// gauges, GSX, FSUIPC, an in-sim package — and write the ones they document
// as settable. An L:var exists once written: writing a new name creates it,
// and every other client reads it (measured live, MSFS 2024: L:MYCREW_TEST
// written to 42 by one connection read 42 by another).
package lvars

import (
	"fmt"
	"strings"
	"sync"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// Client is what a Writer needs of a connection (engine.Engine, the
// manager).
type Client interface {
	AddToDataDefinition(definitionID uint32, datumName string, unitsName string, datumType types.SIMCONNECT_DATATYPE, epsilon float32, datumID uint32) error
	SetDataOnSimObject(definitionID uint32, objectID uint32, flags types.SIMCONNECT_DATA_SET_FLAG, arrayCount uint32, cbUnitSize uint32, data unsafe.Pointer) error
}

// Writer writes L:vars, each through a data definition of its own taken
// from defBase on (at most max of them), defined once per connection.
type Writer struct {
	mu      sync.Mutex
	client  Client
	defBase uint32
	max     int
	defs    map[string]uint32 // name → definition, defined on this connection
	next    uint32
}

// DefaultMax: the L:vars a Writer gives a definition of its own.
const DefaultMax = 64

// NewWriter is a Writer on client with definition IDs from defBase
// (defBase … defBase+max-1; max 0: DefaultMax).
func NewWriter(client Client, defBase uint32, max int) *Writer {
	if max <= 0 {
		max = DefaultMax
	}
	return &Writer{client: client, defBase: defBase, max: max, defs: map[string]uint32{}}
}

// Set writes value to L:name ("MYCREW_BOARDING" or "L:MYCREW_BOARDING")
// on the user aircraft, as a number.
func (w *Writer) Set(name string, value float64) error {
	name = "L:" + strings.TrimPrefix(name, "L:")
	w.mu.Lock()
	defer w.mu.Unlock()
	def, ok := w.defs[name]
	if !ok {
		if int(w.next) >= w.max {
			return fmt.Errorf("lvars: %d L:vars already defined (NewWriter max)", w.max)
		}
		def = w.defBase + w.next
		if err := w.client.AddToDataDefinition(def, name, "number", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0); err != nil {
			return fmt.Errorf("lvars: define %s: %w", name, err)
		}
		w.defs[name] = def
		w.next++
	}
	if err := w.client.SetDataOnSimObject(def, types.SIMCONNECT_OBJECT_ID_USER, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, 8, unsafe.Pointer(&value)); err != nil {
		return fmt.Errorf("lvars: set %s: %w", name, err)
	}
	return nil
}

// Reset forgets the definitions (a new connection has none), and takes
// client when not nil.
func (w *Writer) Reset(client Client) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if client != nil {
		w.client = client
	}
	w.defs, w.next = map[string]uint32{}, 0
}
