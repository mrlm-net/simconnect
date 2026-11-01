package world

import (
	"errors"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/camera"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// wireSentinels are the errors a caller may check with errors.Is. An error
// crosses the link as its text, so the director finds them again by it.
// TestWireSentinelsComplete keeps this list whole.
var wireSentinels = []error{
	traffic.ErrAlreadyCleared, traffic.ErrAlreadyStarted, traffic.ErrBadTaxiRequest,
	traffic.ErrCreationFailed, traffic.ErrEmptyWaypoints, traffic.ErrEntryTooShort,
	traffic.ErrEstablished, traffic.ErrExitFixed, traffic.ErrExitTooClose,
	traffic.ErrExitTooTight, traffic.ErrGroundUnknown, traffic.ErrHolding,
	traffic.ErrInjectorFull, traffic.ErrNoIDs, traffic.ErrNoPushPose,
	traffic.ErrNoRunway, traffic.ErrNoShortcut, traffic.ErrNoStand,
	traffic.ErrNoTouchdown, traffic.ErrNotApplicable, traffic.ErrNotConnected,
	traffic.ErrNotHolding, traffic.ErrNotHoldingShort, traffic.ErrNotInjected,
	traffic.ErrNotOnProcedure, traffic.ErrNotOnRoute, traffic.ErrNotSequenced,
	traffic.ErrNotStandRoute, traffic.ErrNotTaxiing, traffic.ErrObjectNotFound,
	traffic.ErrPathTooShort, traffic.ErrRunwayTooShort, traffic.ErrShortRoute,
	traffic.ErrSpawnBlocked, traffic.ErrSpawnImpossible, traffic.ErrStandFixed,
	traffic.ErrStandTaken, traffic.ErrStandardStale, traffic.ErrTaxiStuck,
	traffic.ErrTooLate,

	airport.ErrAmbiguousParking, airport.ErrNoData, airport.ErrNoExit,
	airport.ErrNoHoldShort, airport.ErrNoProcedure, airport.ErrNoRoute,
	airport.ErrNoRunways, airport.ErrNoTaxiNetwork, airport.ErrNoTransition,
	airport.ErrNoVehicleRoute, airport.ErrTaxiwaysNotFollowed, airport.ErrTimeout,
	airport.ErrTooNarrow, airport.ErrUnknownEntry, airport.ErrUnknownParking,
	airport.ErrUnknownRunway, airport.ErrUnknownTaxiway, airport.ErrViaUnreachable,

	nav.ErrNoAirport, nav.ErrNoRoute,

	camera.ErrNotAcquired,

	ErrActuator, ErrHeld, ErrNotConnectedUs, ErrStandOccupied,
	ErrUserAirborne, ErrUserEngines, ErrUserMoving, ErrUserUnknown,
}

// remoteError is an error from the other end of the link: its text as it
// was, and the known errors that text names, for errors.Is.
type remoteError struct {
	msg string
	is  []error
}

func (e *remoteError) Error() string   { return e.msg }
func (e *remoteError) Unwrap() []error { return e.is }

// wireError is the error with this text from the other end of the link.
// Found anywhere in the text: the other end may have wrapped it.
func wireError(msg string) error {
	var is []error
	for _, s := range wireSentinels {
		if strings.Contains(msg, s.Error()) {
			is = append(is, s)
		}
	}
	if len(is) == 0 {
		return errors.New(msg)
	}
	return &remoteError{msg: msg, is: is}
}
