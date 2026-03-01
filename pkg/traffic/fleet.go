//go:build windows
// +build windows

package traffic

import (
	"sync"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Fleet manages a thread-safe collection of AI aircraft bound to a single
// engine client. It tracks both pending creations (awaiting ObjectID assignment
// from SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID) and active aircraft.
//
// Lifecycle:
//
//   - Create with NewFleet (typically inside a Manager or your own controller).
//   - Call Request* to spawn aircraft — each issues the SimConnect creation call
//     and records a Pending entry keyed by reqID.
//   - On receiving SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID, call Acknowledge to
//     promote the Pending entry to a full Aircraft tracked by ObjectID.
//   - On disconnect / reconnect, call SetClient(newClient) which also clears all
//     stale pending and member state (ObjectIDs are invalid after a disconnect).
type Fleet struct {
	mu      sync.RWMutex
	pending map[uint32]*Pending  // reqID → pending creation
	members map[uint32]*Aircraft // objectID → active aircraft
	client  engine.Client
}

// NewFleet constructs a Fleet bound to the given engine client.
// Pass nil to create an unconnected fleet and call SetClient later.
func NewFleet(client engine.Client) *Fleet {
	return &Fleet{
		pending: make(map[uint32]*Pending),
		members: make(map[uint32]*Aircraft),
		client:  client,
	}
}

// SetClient updates the engine client reference and clears all pending and
// member state. Call this on connect (passing the new client) and on disconnect
// (passing nil). ObjectIDs are invalidated across reconnects so the fleet must
// be reset each time.
func (f *Fleet) SetClient(client engine.Client) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.client = client
	f.pending = make(map[uint32]*Pending)
	f.members = make(map[uint32]*Aircraft)
}

// ── Creation requests (async) ─────────────────────────────────────────────

// RequestParked queues a parked ATC aircraft creation.
// The ObjectID is assigned asynchronously; call Acknowledge from your
// SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID handler to complete the handle.
// Returns ErrNotConnected if the fleet has no active client.
func (f *Fleet) RequestParked(opts ParkedOpts, reqID uint32) error {
	f.mu.RLock()
	c := f.client
	f.mu.RUnlock()
	if c == nil {
		return ErrNotConnected
	}
	var err error
	if opts.Livery != "" {
		err = c.AICreateParkedATCAircraftEX1(opts.Model, opts.Livery, opts.Tail, opts.Airport, reqID)
	} else {
		err = c.AICreateParkedATCAircraft(opts.Model, opts.Tail, opts.Airport, reqID)
	}
	if err != nil {
		return err
	}
	f.mu.Lock()
	f.pending[reqID] = &Pending{ReqID: reqID, Kind: KindParked, Model: opts.Model, Livery: opts.Livery, Tail: opts.Tail}
	f.mu.Unlock()
	return nil
}

// RequestEnroute queues an enroute ATC aircraft creation along a flight plan.
// Returns ErrNotConnected if the fleet has no active client.
func (f *Fleet) RequestEnroute(opts EnrouteOpts, reqID uint32) error {
	f.mu.RLock()
	c := f.client
	f.mu.RUnlock()
	if c == nil {
		return ErrNotConnected
	}
	var err error
	if opts.Livery != "" {
		err = c.AICreateEnrouteATCAircraftEX1(opts.Model, opts.Livery, opts.Tail, opts.FlightNumber, opts.FlightPlan, opts.Phase, opts.TouchAndGo, reqID)
	} else {
		err = c.AICreateEnrouteATCAircraft(opts.Model, opts.Tail, opts.FlightNumber, opts.FlightPlan, opts.Phase, opts.TouchAndGo, reqID)
	}
	if err != nil {
		return err
	}
	f.mu.Lock()
	f.pending[reqID] = &Pending{ReqID: reqID, Kind: KindEnroute, Model: opts.Model, Livery: opts.Livery, Tail: opts.Tail}
	f.mu.Unlock()
	return nil
}

// RequestNonATC queues a non-ATC aircraft creation at an explicit position.
// These aircraft follow a waypoint chain rather than ATC instructions.
// After Acknowledge, call ReleaseControl then SetWaypoints to begin movement.
// Returns ErrNotConnected if the fleet has no active client.
func (f *Fleet) RequestNonATC(opts NonATCOpts, reqID uint32) error {
	f.mu.RLock()
	c := f.client
	f.mu.RUnlock()
	if c == nil {
		return ErrNotConnected
	}
	var err error
	if opts.Livery != "" {
		err = c.AICreateNonATCAircraftEX1(opts.Model, opts.Livery, opts.Tail, opts.Position, reqID)
	} else {
		err = c.AICreateNonATCAircraft(opts.Model, opts.Tail, opts.Position, reqID)
	}
	if err != nil {
		return err
	}
	f.mu.Lock()
	f.pending[reqID] = &Pending{ReqID: reqID, Kind: KindNonATC, Model: opts.Model, Livery: opts.Livery, Tail: opts.Tail}
	f.mu.Unlock()
	return nil
}

// ── Acknowledge ────────────────────────────────────────────────────────────

// Acknowledge resolves a pending creation with the ObjectID returned by SimConnect.
// Call this from your SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID handler.
//
// Returns the created Aircraft and true if reqID was a known pending request.
// Returns nil and false when the reqID is unrecognised (belongs to another subsystem).
func (f *Fleet) Acknowledge(reqID uint32, objectID uint32) (*Aircraft, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.pending[reqID]
	if !ok {
		return nil, false
	}
	delete(f.pending, reqID)
	a := &Aircraft{
		ObjectID: objectID,
		Kind:     p.Kind,
		Model:    p.Model,
		Livery:   p.Livery,
		Tail:     p.Tail,
	}
	f.members[objectID] = a
	return a, true
}

// ── Aircraft operations ────────────────────────────────────────────────────

// Remove removes an AI aircraft from the simulation and from the fleet.
// Returns ErrNotConnected if the fleet has no active client.
func (f *Fleet) Remove(objectID uint32, reqID uint32) error {
	f.mu.RLock()
	c := f.client
	f.mu.RUnlock()
	if c == nil {
		return ErrNotConnected
	}
	if err := c.AIRemoveObject(objectID, reqID); err != nil {
		return err
	}
	f.mu.Lock()
	delete(f.members, objectID)
	f.mu.Unlock()
	return nil
}

// ReleaseControl releases SimConnect control over an aircraft, handing it back
// to the simulator's AI system. This must be called before sending a waypoint
// chain to a NonATC aircraft — the simulator will not honour waypoints for an
// aircraft it still holds under its own control.
// Returns ErrNotConnected if the fleet has no active client.
func (f *Fleet) ReleaseControl(objectID uint32, reqID uint32) error {
	f.mu.RLock()
	c := f.client
	f.mu.RUnlock()
	if c == nil {
		return ErrNotConnected
	}
	return c.AIReleaseControl(objectID, reqID)
}

// SetWaypoints assigns a waypoint chain to a non-ATC aircraft.
// defID must be a SimConnect data definition registered for "AI Waypoint List"
// (type SIMCONNECT_DATATYPE_WAYPOINT). Build the waypoint slice using the helpers
// in this package: PushbackWaypoint, TaxiWaypoint, LineupWaypoint, ClimbWaypoint,
// and TakeoffClimb.
//
// Call ReleaseControl before SetWaypoints — the simulator will otherwise ignore
// the waypoints.
// Returns ErrNotConnected if the fleet has no active client.
// Returns ErrEmptyWaypoints if wps is nil or empty.
func (f *Fleet) SetWaypoints(objectID uint32, defID uint32, wps []types.SIMCONNECT_DATA_WAYPOINT) error {
	if len(wps) == 0 {
		return ErrEmptyWaypoints
	}
	f.mu.RLock()
	c := f.client
	f.mu.RUnlock()
	if c == nil {
		return ErrNotConnected
	}
	packed := engine.PackWaypoints(wps)
	return c.SetDataOnSimObject(
		defID,
		objectID,
		types.SIMCONNECT_DATA_SET_FLAG_DEFAULT,
		uint32(len(wps)),
		engine.WaypointWireSize,
		unsafe.Pointer(&packed[0]),
	)
}

// SetFlightPlan assigns a flight plan to an ATC aircraft (parked or enroute).
// Returns ErrNotConnected if the fleet has no active client.
func (f *Fleet) SetFlightPlan(objectID uint32, planPath string, reqID uint32) error {
	f.mu.RLock()
	c := f.client
	f.mu.RUnlock()
	if c == nil {
		return ErrNotConnected
	}
	return c.AISetAircraftFlightPlan(objectID, planPath, reqID)
}

// ── Collection ─────────────────────────────────────────────────────────────

// Get returns the Aircraft for the given ObjectID.
// Returns (nil, false) if the ObjectID is not tracked in the fleet.
func (f *Fleet) Get(objectID uint32) (*Aircraft, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	a, ok := f.members[objectID]
	return a, ok
}

// List returns a snapshot of all currently tracked Aircraft.
func (f *Fleet) List() []*Aircraft {
	f.mu.RLock()
	defer f.mu.RUnlock()
	result := make([]*Aircraft, 0, len(f.members))
	for _, a := range f.members {
		result = append(result, a)
	}
	return result
}

// Len returns the number of active (acknowledged) aircraft in the fleet.
func (f *Fleet) Len() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.members)
}

// RemoveAll removes all tracked aircraft from the simulation.
// reqIDBase is used as the base request ID; each aircraft is assigned
// reqIDBase + i for i in [0, len). Returns the last non-nil error encountered.
func (f *Fleet) RemoveAll(reqIDBase uint32) error {
	f.mu.RLock()
	c := f.client
	ids := make([]uint32, 0, len(f.members))
	for id := range f.members {
		ids = append(ids, id)
	}
	f.mu.RUnlock()
	if c == nil {
		return ErrNotConnected
	}
	var last error
	for i, id := range ids {
		if err := c.AIRemoveObject(id, reqIDBase+uint32(i)); err != nil {
			last = err
		}
	}
	f.mu.Lock()
	f.members = make(map[uint32]*Aircraft)
	f.mu.Unlock()
	return last
}

// Clear resets the fleet without issuing removal requests to the simulator.
// Use this after a disconnect when all ObjectIDs are already stale.
func (f *Fleet) Clear() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pending = make(map[uint32]*Pending)
	f.members = make(map[uint32]*Aircraft)
}
