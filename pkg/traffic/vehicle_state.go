package traffic

// VehicleState is where a ground vehicle is in its service to an aircraft
// (#710: a host draws it as stated, not inferred).
type VehicleState string

const (
	VehicleWaiting  VehicleState = "waiting"  // asked for, not in the simulator yet
	VehicleInbound  VehicleState = "inbound"  // driving to the aircraft
	VehicleAttached VehicleState = "attached" // a tug on the nose gear: pushing or about to
	VehicleFuelling VehicleState = "fuelling" // a fuel truck parked at the wing
	VehicleOutbound VehicleState = "outbound" // backing off or driving home
	VehicleRemoved  VehicleState = "removed"  // gone from the simulator
)

// State is where the tug is in its push.
func (t *SimObjectTug) State() VehicleState {
	t.mu.Lock()
	defer t.mu.Unlock()
	switch {
	case t.done:
		return VehicleRemoved
	case t.objectID == 0:
		return VehicleWaiting
	case t.arrive != nil:
		return VehicleInbound
	case t.away != nil || t.homing:
		return VehicleOutbound
	}
	return VehicleAttached
}

// Title is the tug's model.
func (t *SimObjectTug) Title() string { return t.title }

// State is where the fuel truck is in its service.
func (f *SimObjectFuelTruck) State() VehicleState {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case f.done:
		return VehicleRemoved
	case f.objectID == 0:
		return VehicleWaiting
	case f.arrive != nil:
		return VehicleInbound
	case f.away != nil || f.homing:
		return VehicleOutbound
	}
	return VehicleFuelling
}

// Title is the fuel truck's model.
func (f *SimObjectFuelTruck) Title() string { return f.title }

// Title is the stairs' ground vehicle title.
func (s *SimObjectStairs) Title() string { return s.title }
