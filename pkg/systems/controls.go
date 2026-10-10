//go:build windows
// +build windows

package systems

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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
	efb     *EFB              // the aircraft's tablet (Profile.EFB)
	efbHost string            // where the sim runs: "127.0.0.1" unless SetEFBHost
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

// Reset forgets the mapped events and defined variables (a new
// connection): they are mapped and defined again on first use, on client
// (nil: the same client).
func (c *Controls) Reset(client ControlClient) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if client != nil {
		c.client = client
	}
	c.events, c.defs, c.next = map[string]uint32{}, map[string]uint32{}, 0
}

// ControlIDs is how many client event and data definition IDs Controls
// takes from its base.
const ControlIDs = 64

// ErrNoIDs: Controls has used all its IDs (ControlIDs).
var ErrNoIDs = errors.New("systems: no control IDs left")

// nextID takes the next ID; c.mu held.
func (c *Controls) nextID() (uint32, error) {
	if c.next >= ControlIDs {
		return 0, ErrNoIDs
	}
	id := c.base + c.next
	c.next++
	return id, nil
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
		if a.On != nil {
			want = *a.On
		}
	} else if a.Off != nil {
		want = *a.Off
	}
	switch {
	case a.EFB != "":
		return c.efbWrite(a.EFB, on)
	case a.Set != "":
		return c.setVars(a, want)
	case a.Counter != "":
		if now.Values[name] != 0 == on {
			return nil // as wanted already
		}
		return c.count(a.Counter, now.Values[name+"Counter"])
	case a.Press != "":
		if now.Values[name] != 0 == on {
			return nil // as wanted already
		}
		// Pressed with 1, or with On / Off where given: a knob pushed (+1)
		// for on and pulled (−1) for off on one variable (the Fenix's FCU).
		press := 1.0
		if on && a.On != nil {
			press = *a.On
		} else if !on && a.Off != nil {
			press = *a.Off
		}
		if err := c.setVar(a.Press, press); err != nil {
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
		if !on && a.OffEvent != "" {
			return c.event(a.OffEvent, data) // its own event for off
		}
		return c.event(a.Event, data)
	}
	return fmt.Errorf("%w: %s (an empty action)", ErrNoControl, name)
}

// Request asks for ground service name (Jetway, Stairs, Baggage, Catering,
// PowerSupply, FuelTruck, Pushback): its event or variable sent once, as a
// crew's request; the toggling ones (jetway, stairs, pushback) send it away
// again when asked again (#666).
func (c *Controls) Request(name string) error {
	return c.Set(name, true, State{})
}

// pressHold is how long a pressed button is held.
const pressHold = 300 * time.Millisecond

func (c *Controls) event(name string, data uint32) error {
	c.mu.Lock()
	id, ok := c.events[name]
	client := c.client
	if !ok {
		var err error
		if id, err = c.nextID(); err != nil {
			c.mu.Unlock()
			return fmt.Errorf("systems: mapping %s: %w", name, err)
		}
		if err := client.MapClientEventToSimEvent(id, name); err != nil {
			c.mu.Unlock()
			return fmt.Errorf("systems: mapping %s: %w", name, err)
		}
		c.events[name] = id
	}
	c.mu.Unlock()
	return client.TransmitClientEvent(types.SIMCONNECT_OBJECT_ID_USER, id, data,
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
	client := c.client
	if !ok {
		var err error
		if def, err = c.nextID(); err != nil {
			c.mu.Unlock()
			return fmt.Errorf("systems: define %s: %w", name, err)
		}
		if err := client.AddToDataDefinition(def, name, "number", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0); err != nil {
			c.mu.Unlock()
			return fmt.Errorf("systems: define %s: %w", name, err)
		}
		c.defs[name] = def
	}
	c.mu.Unlock()
	return client.SetDataOnSimObject(def, types.SIMCONNECT_OBJECT_ID_USER, 0, 0, 8, unsafe.Pointer(&v))
}

// count presses a counted button standing at v: to the next odd count,
// then the even one after (press and release).
func (c *Controls) count(name string, v float64) error {
	n := math.Floor(v)
	if int(n)%2 != 0 {
		n++ // released half way: from the next even count
	}
	if err := c.setVar(name, n+1); err != nil {
		return err
	}
	time.Sleep(pressHold)
	return c.setVar(name, n+2)
}

// Press presses control name once, whatever its state (CabinCall): a
// counted button, a button variable or an event (#759).
func (c *Controls) Press(name string, now State) error {
	c.mu.Lock()
	a, ok := c.actions[name]
	c.mu.Unlock()
	if !ok {
		return fmt.Errorf("%w: %s", ErrNoControl, name)
	}
	switch {
	case a.Set != "":
		want := 1.0 // a variable set: pressed, it takes its On value
		if a.On != nil {
			want = *a.On
		}
		return c.setVars(a, want)
	case a.Counter != "":
		return c.count(a.Counter, now.Values[name+"Counter"])
	case a.Press != "":
		if err := c.setVar(a.Press, 1); err != nil {
			return err
		}
		time.Sleep(pressHold)
		return c.setVar(a.Press, 0)
	case a.Event != "":
		data := uint32(1)
		if a.Data != nil {
			data = *a.Data
		}
		return c.event(a.Event, data)
	case a.EFB != "":
		return c.efbWrite(a.EFB, true)
	}
	return fmt.Errorf("%w: %s (an empty action)", ErrNoControl, name)
}

// SetValue puts control name to value v (the no smoking sign: 0 off, 1
// auto, 2 on): a variable set to v; any other action on (v != 0) or off
// (#759).
func (c *Controls) SetValue(name string, v float64, now State) error {
	c.mu.Lock()
	a, ok := c.actions[name]
	c.mu.Unlock()
	if !ok {
		return fmt.Errorf("%w: %s", ErrNoControl, name)
	}
	if a.Set != "" {
		if a.Scale != nil {
			v *= *a.Scale
		}
		if a.Offset != nil {
			v += *a.Offset
		}
		return c.setVars(a, v)
	}
	if a.Encoder != "" {
		return c.turn(name, a, v, now)
	}
	if a.Event != "" && a.Value {
		return c.event(a.Event, eventData(v, a.Scale, a.Offset))
	}
	return c.Set(name, v != 0, now)
}

// eventData is v times scale (nil: 1), rounded, as a key event's data: a
// negative value as its two's complement.
func eventData(v float64, scale, offset *float64) uint32 {
	if scale != nil {
		v *= *scale
	}
	if offset != nil {
		v += *offset
	}
	return uint32(int32(math.Round(v)))
}

// ErrEncoderWoken: the knob's display was dashed (0); one click was turned
// to show it. Call SetValue again with a state read after it.
var ErrEncoderWoken = errors.New("systems: encoder display woken, set again")

// turn sets value name to v on a relative knob (Action.Encoder): its
// counter moved by the clicks from the value shown to v.
func (c *Controls) turn(name string, a Action, v float64, now State) error {
	step := a.Step
	if step <= 0 {
		step = 1
	}
	count := now.Values[name+"Encoder"]
	shown := now.Values[a.Display]
	if shown == 0 && a.Wake {
		if err := c.setVar(a.Encoder, count+1); err != nil {
			return err
		}
		return ErrEncoderWoken
	}
	clicks := math.Round((v - shown) / step)
	if clicks == 0 {
		return nil
	}
	return c.setVar(a.Encoder, count+clicks)
}

// setVars writes v to a's Set variable and each of Also.
func (c *Controls) setVars(a Action, v float64) error {
	for _, name := range append([]string{a.Set}, a.Also...) {
		if err := c.setVar(name, v); err != nil {
			return err
		}
	}
	return nil
}
