//go:build windows
// +build windows

package traffic

// Aircraft is a handle for a spawned AI aircraft whose ObjectID has been
// confirmed by SimConnect. Handles are created by Fleet.Acknowledge and stored
// inside the Fleet. All aircraft operations are performed through the Fleet.
type Aircraft struct {
	// ObjectID is the SimConnect object identifier assigned by the simulator.
	ObjectID uint32
	// Kind indicates how the aircraft was created (parked / enroute / non-ATC).
	Kind AircraftKind
	// Model is the container title string used during creation.
	Model string
	// Livery is the livery folder name used during creation ("" = default).
	Livery string
	// Tail is the tail number or identifier string used during creation.
	Tail string
}
