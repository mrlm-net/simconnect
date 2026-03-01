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
