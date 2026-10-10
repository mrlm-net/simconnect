//go:build windows

// Package hotkeys binds keys and joystick buttons the player presses in
// the simulator to actions of an add-on (SimConnect input groups): "answer
// the checklist item" on Ctrl+Shift+C or a yoke button. Bindings are by
// action name, can be changed (the host keeps them in a local file), and
// are mapped again after a reconnect.
//
// A definition is SimConnect's input string: keys joined with "+"
// ("Ctrl+Shift+C", "VK_F12"), or a joystick input ("joystick:0:button:5").
// Which keys MSFS 2024 passes to SimConnect is to verify live.
package hotkeys

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Client is what Hotkeys needs of a connection (an engine or a manager).
type Client interface {
	MapInputEventToClientEvent(groupID uint32, definition string, downEventID, downValue, upEventID, upValue uint32, maskable bool) error
	SetInputGroupPriority(groupID uint32, priority uint32) error
	SetInputGroupState(groupID uint32, state uint32) error
	RemoveInputEvent(groupID uint32, definition string) error
}

// DefaultIDBase: the input group is IDBase and the actions' client events
// IDBase+1 on (clear of the World's 0xA000 range and pkg/systems').
const DefaultIDBase = 0xB100

// MaxActions is how many actions one Hotkeys binds.
const MaxActions = 64

const unused = ^uint32(0) // SIMCONNECT_UNUSED

type binding struct {
	definition string
	onDown     func()
	event      uint32
	mapped     bool // mapped on the current connection
}

// Hotkeys is one input group of bound actions.
type Hotkeys struct {
	idBase uint32

	mu      sync.Mutex
	client  Client
	order   []string // action names, in binding order
	byName  map[string]*binding
	enabled bool
}

// New returns Hotkeys using IDs from idBase (0: DefaultIDBase): the group
// idBase, the actions' events idBase+1…idBase+MaxActions.
func New(idBase uint32) *Hotkeys {
	if idBase == 0 {
		idBase = DefaultIDBase
	}
	return &Hotkeys{idBase: idBase, byName: map[string]*binding{}, enabled: true}
}

// Bind binds action name to definition, calling onDown when it is pressed;
// binding a name again moves it to the new definition ("" unbinds it).
func (h *Hotkeys) Bind(name, definition string, onDown func()) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	b := h.byName[name]
	if b == nil {
		if len(h.order) >= MaxActions {
			return fmt.Errorf("hotkeys: more than %d actions", MaxActions)
		}
		b = &binding{event: h.idBase + 1 + uint32(len(h.order))}
		h.byName[name] = b
		h.order = append(h.order, name)
	}
	if onDown != nil {
		b.onDown = onDown
	}
	definition = strings.TrimSpace(definition)
	if definition == b.definition {
		return nil
	}
	if b.mapped && h.client != nil {
		_ = h.client.RemoveInputEvent(h.idBase, b.definition)
	}
	b.definition, b.mapped = definition, false
	return h.mapLocked(b)
}

// Bindings are the actions' definitions by name (for the host's file).
func (h *Hotkeys) Bindings() map[string]string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := map[string]string{}
	for name, b := range h.byName {
		out[name] = b.definition
	}
	return out
}

// SetBindings rebinds the actions named in defs (a host's file read back);
// actions not bound yet are bound without a handler until Bind gives one.
func (h *Hotkeys) SetBindings(defs map[string]string) error {
	names := make([]string, 0, len(defs))
	for n := range defs {
		names = append(names, n)
	}
	slices.Sort(names)
	var errs []string
	for _, n := range names {
		if err := h.Bind(n, defs[n], nil); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("hotkeys: %s", strings.Join(errs, "; "))
	}
	return nil
}

// Attach maps every binding on client (a new connection, or the same one
// after a reconnect: call it again then), with the group at the highest
// priority and on.
func (h *Hotkeys) Attach(client Client) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.client = client
	for _, b := range h.byName {
		b.mapped = false
	}
	if client == nil {
		return nil
	}
	var first error
	for _, n := range h.order {
		if err := h.mapLocked(h.byName[n]); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// mapLocked maps b on the current client, and sets the group up after its
// first mapping (SimConnect creates the group with it).
func (h *Hotkeys) mapLocked(b *binding) error {
	if h.client == nil || b.definition == "" || b.mapped {
		return nil
	}
	if err := h.client.MapInputEventToClientEvent(h.idBase, b.definition, b.event, 0, unused, 0, false); err != nil {
		return fmt.Errorf("hotkeys: %q: %w", b.definition, err)
	}
	b.mapped = true
	_ = h.client.SetInputGroupPriority(h.idBase, types.SIMCONNECT_GROUP_PRIORITY_HIGHEST)
	state := uint32(types.SIMCONNECT_STATE_OFF)
	if h.enabled {
		state = uint32(types.SIMCONNECT_STATE_ON)
	}
	return h.client.SetInputGroupState(h.idBase, state)
}

// Enable turns the group on or off (off: the keys reach the sim as usual).
func (h *Hotkeys) Enable(on bool) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.enabled = on
	if h.client == nil {
		return nil
	}
	state := uint32(types.SIMCONNECT_STATE_OFF)
	if on {
		state = uint32(types.SIMCONNECT_STATE_ON)
	}
	return h.client.SetInputGroupState(h.idBase, state)
}

// Handle takes a message: an event of one of the actions calls its
// handler (outside the lock) and reports true.
func (h *Hotkeys) Handle(msg engine.Message) bool {
	ev := msg.AsEvent()
	if ev == nil {
		return false
	}
	id := uint32(ev.UEventID)
	if id <= h.idBase || id > h.idBase+MaxActions {
		return false
	}
	h.mu.Lock()
	var fn func()
	for _, b := range h.byName {
		if b.event == id {
			fn = b.onDown
		}
	}
	h.mu.Unlock()
	if fn != nil {
		fn()
	}
	return true
}
