//go:build windows
// +build windows

package traffic

import "errors"

var (
	// ErrNotConnected is returned when the fleet's engine client is nil.
	ErrNotConnected = errors.New("traffic: not connected to simulator")

	// ErrObjectNotFound is returned when an operation targets an ObjectID that
	// is not tracked in the fleet.
	ErrObjectNotFound = errors.New("traffic: object ID not found in fleet")

	// ErrCreationFailed is returned when an aircraft creation call fails.
	ErrCreationFailed = errors.New("traffic: aircraft creation failed")

	// ErrEmptyWaypoints is returned when SetWaypoints is called with a nil or
	// zero-length waypoint slice.
	ErrEmptyWaypoints = errors.New("traffic: waypoints slice must not be empty")
)

var (
	// ErrPathTooShort is returned for a ground path with fewer than two
	// distinct points.
	ErrPathTooShort = errors.New("traffic: ground path needs at least two distinct points")

	// ErrNotInjected is returned by Injector methods for an object that was
	// not taken over with Takeover.
	ErrNotInjected = errors.New("traffic: object is not driven by the injector")

	// ErrGroundUnknown is returned by Injector.Place until the ground height
	// under the aircraft has been received; the frozen aircraft stays where
	// it is meanwhile.
	ErrGroundUnknown = errors.New("traffic: ground height under the aircraft not received yet")

	// ErrInjectorFull is returned by Takeover when injectMaxAircraft aircraft
	// are already driven.
	ErrInjectorFull = errors.New("traffic: injector drives the maximum number of aircraft")
)

// ErrNotOnRoute is returned by ClearUpTo for a node that is not ahead on the
// aircraft's current taxi path.
var ErrNotOnRoute = errors.New("traffic: clearance limit is not ahead on the taxi route")
