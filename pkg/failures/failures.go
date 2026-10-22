// Package failures fails and restores systems of the user aircraft (the
// emergency add-on, part 3), by generic names, through the simulator's
// failure key events (MSFS 2024 "Aircraft Miscellaneous Events":
// TOGGLE_ENGINE1_FAILURE … TOGGLE_ENGINE4_FAILURE, TOGGLE_ELECTRICAL_FAILURE,
// TOGGLE_HYDRAULIC_FAILURE, TOGGLE_VACUUM_FAILURE, TOGGLE_PITOT_BLOCKAGE,
// TOGGLE_STATIC_PORT_BLOCKAGE, TOGGLE_LEFT_BRAKE_FAILURE,
// TOGGLE_RIGHT_BRAKE_FAILURE, TOGGLE_TOTAL_BRAKE_FAILURE; each a toggle
// with no parameter).
//
// A Profile maps the generic kinds to events: Default for the simulator's
// own, a model's override for aircraft with their own systems (a
// study-level aircraft ignoring the simulator's failures maps none, and
// Supported says so). An engine fire, a pressurisation failure and a fuel
// leak have no failure event in the simulator: unsupported unless a
// model's override maps them.
package failures

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// Kind is a failure by its generic name.
type Kind string

const (
	EngineFailure  Kind = "engine-failure"
	EngineFire     Kind = "engine-fire"
	Electrical     Kind = "electrical"
	Hydraulic      Kind = "hydraulic"
	Pitot          Kind = "pitot"
	Static         Kind = "static"
	Vacuum         Kind = "vacuum"
	Brakes         Kind = "brakes"
	BrakeLeft      Kind = "brake-left"
	BrakeRight     Kind = "brake-right"
	FuelLeak       Kind = "fuel-leak"
	Pressurisation Kind = "pressurisation"
)

// Failure is one failure: a kind, and for engine kinds the engine (1–4).
type Failure struct {
	Kind   Kind `json:"kind"`
	Engine int  `json:"engine,omitempty"`
}

// Status is a failure applied, and since when.
type Status struct {
	Failure
	Active bool      `json:"active"`
	Since  time.Time `json:"since"`
}

// Profile maps kinds to the key events that toggle them. Engine kinds name
// one event per engine, in order (index 0: engine 1).
type Profile struct {
	Events       map[Kind]string   `json:"events,omitempty"`
	EngineEvents map[Kind][]string `json:"engineEvents,omitempty"`
}

// Default is the simulator's own failures (the documented key events).
func Default() Profile {
	return Profile{
		Events: map[Kind]string{
			Electrical: "TOGGLE_ELECTRICAL_FAILURE",
			Hydraulic:  "TOGGLE_HYDRAULIC_FAILURE",
			Vacuum:     "TOGGLE_VACUUM_FAILURE",
			Pitot:      "TOGGLE_PITOT_BLOCKAGE",
			Static:     "TOGGLE_STATIC_PORT_BLOCKAGE",
			Brakes:     "TOGGLE_TOTAL_BRAKE_FAILURE",
			BrakeLeft:  "TOGGLE_LEFT_BRAKE_FAILURE",
			BrakeRight: "TOGGLE_RIGHT_BRAKE_FAILURE",
		},
		EngineEvents: map[Kind][]string{
			EngineFailure: {"TOGGLE_ENGINE1_FAILURE", "TOGGLE_ENGINE2_FAILURE", "TOGGLE_ENGINE3_FAILURE", "TOGGLE_ENGINE4_FAILURE"},
		},
	}
}

// With is p with over's mappings on top: a model's override, or a local
// override file's; an empty event name ("", or no engine events) takes a
// kind away.
func (p Profile) With(over Profile) Profile {
	out := Profile{Events: map[Kind]string{}, EngineEvents: map[Kind][]string{}}
	for k, v := range p.Events {
		out.Events[k] = v
	}
	for k, v := range p.EngineEvents {
		out.EngineEvents[k] = slices.Clone(v)
	}
	for k, v := range over.Events {
		if v == "" {
			delete(out.Events, k)
		} else {
			out.Events[k] = v
		}
	}
	for k, v := range over.EngineEvents {
		if len(v) == 0 {
			delete(out.EngineEvents, k)
		} else {
			out.EngineEvents[k] = slices.Clone(v)
		}
	}
	return out
}

// LoadProfile reads a profile (an override) from a JSON file.
func LoadProfile(path string) (Profile, error) {
	var p Profile
	b, err := os.ReadFile(path)
	if err != nil {
		return p, err
	}
	if err := json.Unmarshal(b, &p); err != nil {
		return p, fmt.Errorf("%s: %w", path, err)
	}
	return p, nil
}

// Client is what Failures needs of a connection.
type Client interface {
	MapClientEventToSimEvent(eventID uint32, eventName string) error
	TransmitClientEvent(objectID uint32, eventID uint32, data uint32, groupID uint32, flags types.SIMCONNECT_EVENT_FLAG) error
}

// DefaultEventBase is the first client event ID Failures maps: 0x7F00, its
// own block of 256 (0x7B00 was systems.Controls', the IDs collided).
const DefaultEventBase uint32 = 0x7F00

// ErrUnsupported: the aircraft's profile has no way to fail it.
var ErrUnsupported = errors.New("failures: not supported on this aircraft")

// Failures fails and restores the user aircraft's systems. Its events are
// mapped on first use; after a reconnect, call Reset, then Reapply.
type Failures struct {
	client Client
	base   uint32
	prof   Profile

	mu     sync.Mutex
	ids    map[string]uint32 // event name → client event ID, mapped
	active map[Failure]time.Time
	now    func() time.Time
}

// New returns Failures on client for an aircraft of profile prof, its
// events from base (0: DefaultEventBase).
func New(client Client, prof Profile, base uint32) *Failures {
	if base == 0 {
		base = DefaultEventBase
	}
	return &Failures{client: client, base: base, prof: prof, ids: map[string]uint32{}, active: map[Failure]time.Time{}, now: time.Now}
}

// Supported are the kinds this aircraft's profile can fail, sorted.
func (f *Failures) Supported() []Kind {
	var out []Kind
	for k := range f.prof.Events {
		out = append(out, k)
	}
	for k, v := range f.prof.EngineEvents {
		if len(v) > 0 {
			out = append(out, k)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// event is the key event that toggles x, "" none.
func (f *Failures) event(x Failure) string {
	if evs, ok := f.prof.EngineEvents[x.Kind]; ok {
		if x.Engine < 1 || x.Engine > len(evs) {
			return ""
		}
		return evs[x.Engine-1]
	}
	return f.prof.Events[x.Kind]
}

// toggle sends x's event. f.mu held.
func (f *Failures) toggle(x Failure) error {
	name := f.event(x)
	if name == "" {
		return fmt.Errorf("%w: %s", ErrUnsupported, x.Kind)
	}
	id, ok := f.ids[name]
	if !ok {
		id = f.base + uint32(len(f.ids))
		if err := f.client.MapClientEventToSimEvent(id, name); err != nil {
			return fmt.Errorf("failures: mapping %s: %w", name, err)
		}
		f.ids[name] = id
	}
	return f.client.TransmitClientEvent(types.SIMCONNECT_OBJECT_ID_USER, id, 0,
		types.SIMCONNECT_GROUP_PRIORITY_HIGHEST, types.SIMCONNECT_EVENT_FLAG_GROUPID_IS_PRIORITY)
}

// Apply fails x (nothing when it is active already).
func (f *Failures) Apply(x Failure) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, on := f.active[x]; on {
		return nil
	}
	if err := f.toggle(x); err != nil {
		return err
	}
	f.active[x] = f.now()
	return nil
}

// Clear restores x (nothing when it is not active).
func (f *Failures) Clear(x Failure) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, on := f.active[x]; !on {
		return nil
	}
	if err := f.toggle(x); err != nil {
		return err
	}
	delete(f.active, x)
	return nil
}

// ClearAll restores every failure applied.
func (f *Failures) ClearAll() error {
	f.mu.Lock()
	list := make([]Failure, 0, len(f.active))
	for x := range f.active {
		list = append(list, x)
	}
	f.mu.Unlock()
	var first error
	for _, x := range list {
		if err := f.Clear(x); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// Active are the failures applied, oldest first.
func (f *Failures) Active() []Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Status, 0, len(f.active))
	for x, at := range f.active {
		out = append(out, Status{Failure: x, Active: true, Since: at})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Since.Before(out[j].Since) })
	return out
}

// Reset forgets the event mapping (a new connection).
func (f *Failures) Reset() {
	f.mu.Lock()
	f.ids = map[string]uint32{}
	f.mu.Unlock()
}

// Reapply toggles every active failure on again, after Reset on a new
// connection where the simulator does not have them.
func (f *Failures) Reapply() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for x := range f.active {
		if err := f.toggle(x); err != nil {
			return err
		}
	}
	return nil
}
