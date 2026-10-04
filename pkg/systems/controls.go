//go:build windows
// +build windows

package systems

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// ControlClient is what Controls needs of a connection: key events, and
// variables set on the user aircraft.
type ControlClient interface {
	MapClientEventToSimEvent(eventID uint32, eventName string) error
	TransmitClientEvent(objectID uint32, eventID uint32, data uint32, groupID uint32, flags types.SIMCONNECT_EVENT_FLAG) error
	AddToDataDefinition(definitionID uint32, datumName string, unitsName string, datumType types.SIMCONNECT_DATATYPE, epsilon float32, datumID uint32) error
	SetDataOnSimObject(definitionID uint32, objectID uint32, flags types.SIMCONNECT_DATA_SET_FLAG, arrayCount uint32, cbUnitSize uint32, data unsafe.Pointer) error
}

// ErrNoControl: the aircraft's profile gives no way to operate it (a model
// without chocks, say): the app leaves the button out (Controls.Can).
var ErrNoControl = errors.New("systems: no such control for this aircraft")

// DefaultControlBase is the first client event ID and data definition ID
// Controls uses (a block of 64 from there, apart from pkg/avionics' 0x7A00).
const DefaultControlBase uint32 = 0x7B00

// Controls operates the user aircraft's ground controls by name — Door(n),
// Chocks, GPU, ParkingBrake — the same way for every aircraft: the
// profile's actions say how (the standard key events by default, a model's
// own variables where it has them; #667). The app never asks which
// aircraft it is.
type Controls struct {
	client ControlClient
	base   uint32

	mu      sync.Mutex
	actions map[string]Action
	efb     *EFB   // the aircraft's tablet (Profile.EFB)
	efbHost string // where the sim runs: "127.0.0.1" unless SetEFBHost
	events  map[string]uint32 // event name → mapped client event ID
	defs    map[string]uint32 // variable → data definition ID
	next    uint32
}

// NewControls returns Controls on client, its IDs from base
// (DefaultControlBase when 0).
func NewControls(client ControlClient, base uint32) *Controls {
	if base == 0 {
		base = DefaultControlBase
	}
	return &Controls{client: client, base: base, events: map[string]uint32{}, defs: map[string]uint32{}, efbHost: "127.0.0.1"}
}

// SetEFBHost sets the host the sim (and the aircraft's tablet) runs on,
// for an app on another machine; "127.0.0.1" by default.
func (c *Controls) SetEFBHost(host string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.efbHost = host
}

// Use takes the aircraft's profile (For): its actions from now on.
func (c *Controls) Use(p Profile) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.actions, c.efb = p.Actions, p.EFB
}

// Can reports whether the aircraft's profile gives a way to operate name.
func (c *Controls) Can(name string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.actions[name]
	return ok
}

// Set puts control name on (open, set, connected) or off; now is its state
// now (a Reader's State), for a toggle sent only when it differs.
func (c *Controls) Set(name string, on bool, now State) error {
	c.mu.Lock()
	a, ok := c.actions[name]
	c.mu.Unlock()
	if !ok {
		return fmt.Errorf("%w: %s", ErrNoControl, name)
	}
	want := 0.0
	if on {
		want = 1
	}
	switch {
	case a.EFB != "":
		return c.efbWrite(a.EFB, on)
	case a.Set != "":
		return c.setVar(a.Set, want)
	case a.Press != "":
		if now.Values[name] != 0 == on {
			return nil // as wanted already
		}
		if err := c.setVar(a.Press, 1); err != nil {
			return err
		}
		time.Sleep(pressHold)
		return c.setVar(a.Press, 0)
	case a.Event != "":
		data := uint32(want)
		if a.Data != nil {
			data = *a.Data
		}
		if a.Toggle && now.Values[name] != 0 == on {
			return nil // as wanted already
		}
		return c.event(a.Event, data)
	}
	return fmt.Errorf("%w: %s (an empty action)", ErrNoControl, name)
}

// pressHold is how long a pressed button is held.
const pressHold = 300 * time.Millisecond

func (c *Controls) event(name string, data uint32) error {
	c.mu.Lock()
	id, ok := c.events[name]
	if !ok {
		id = c.base + c.next
		c.next++
		if err := c.client.MapClientEventToSimEvent(id, name); err != nil {
			c.mu.Unlock()
			return fmt.Errorf("systems: mapping %s: %w", name, err)
		}
		c.events[name] = id
	}
	c.mu.Unlock()
	return c.client.TransmitClientEvent(types.SIMCONNECT_OBJECT_ID_USER, id, data,
		types.SIMCONNECT_GROUP_PRIORITY_HIGHEST, types.SIMCONNECT_EVENT_FLAG_GROUPID_IS_PRIORITY)
}

// efbWrite writes boolean data ref name through the aircraft's tablet API
// (GraphQL writeBool, as its own EFB does).
func (c *Controls) efbWrite(name string, v bool) error {
	c.mu.Lock()
	efb, host := c.efb, c.efbHost
	c.mu.Unlock()
	if efb == nil {
		return fmt.Errorf("%w: %s (no EFB in the profile)", ErrNoControl, name)
	}
	body, _ := json.Marshal(map[string]any{
		"query":     "mutation($v: Boolean!) { dataRef { writeBool(name: " + strconv.Quote(name) + ", value: $v) } }",
		"variables": map[string]any{"v": v},
	})
	cl := http.Client{Timeout: 5 * time.Second}
	resp, err := cl.Post("http://"+host+":"+strconv.Itoa(efb.Port)+"/graphql", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("systems: EFB write %s: %w", name, err)
	}
	defer resp.Body.Close()
	var out struct {
		Errors []struct{ Message string } `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || resp.StatusCode != http.StatusOK {
		return fmt.Errorf("systems: EFB write %s: status %d %v", name, resp.StatusCode, err)
	}
	if len(out.Errors) > 0 {
		return fmt.Errorf("systems: EFB write %s: %s", name, out.Errors[0].Message)
	}
	return nil
}

func (c *Controls) setVar(name string, v float64) error {
	c.mu.Lock()
	def, ok := c.defs[name]
	if !ok {
		def = c.base + c.next
		c.next++
		if err := c.client.AddToDataDefinition(def, name, "number", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0); err != nil {
			c.mu.Unlock()
			return fmt.Errorf("systems: define %s: %w", name, err)
		}
		c.defs[name] = def
	}
	c.mu.Unlock()
	return c.client.SetDataOnSimObject(def, types.SIMCONNECT_OBJECT_ID_USER, 0, 0, 8, unsafe.Pointer(&v))
}
