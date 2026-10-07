//go:build windows
// +build windows

package manager

import (
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/manager/internal/subscriptions"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// FilenameEvent represents events that carry a filename (FlightLoaded, AircraftLoaded, FlightPlanActivated)
type FilenameEvent struct {
	Filename string
}

// FilenameSubscription is a typed subscription for filename-based system events
type FilenameSubscription interface {
	ID() string
	Events() <-chan FilenameEvent
	Done() <-chan struct{}
	Unsubscribe()
}

// SubscribeOnFlightLoaded returns a subscription delivering FlightLoaded filenames
func (m *Instance) SubscribeOnFlightLoaded(id string, bufferSize int) FilenameSubscription {
	return m.subscribeFilenameEvent(id, bufferSize, "-flightloaded", "FlightLoaded", m.flightLoadedEventID)
}

// SubscribeOnAircraftLoaded returns a subscription delivering AircraftLoaded filenames
func (m *Instance) SubscribeOnAircraftLoaded(id string, bufferSize int) FilenameSubscription {
	return m.subscribeFilenameEvent(id, bufferSize, "-aircraftloaded", "AircraftLoaded", m.aircraftLoadedEventID)
}

// SubscribeOnFlightPlanActivated returns a subscription delivering FlightPlanActivated filenames
func (m *Instance) SubscribeOnFlightPlanActivated(id string, bufferSize int) FilenameSubscription {
	return m.subscribeFilenameEvent(id, bufferSize, "-flightplanactivated", "FlightPlanActivated", m.flightPlanActivatedEventID)
}

// subscribeFilenameEvent is a filename subscription for eventID. Its message
// subscription is id+suffix, a suffix of its own per kind: the three kinds
// with the same id do not replace each other (review #19).
func (m *Instance) subscribeFilenameEvent(id string, bufferSize int, suffix, kind string, eventID uint32) FilenameSubscription {
	id = subscriptions.GenerateID(id)
	bufferSize = subscriptions.ValidateBufferSize(bufferSize)
	msgSub := m.SubscribeWithType(id+suffix, bufferSize, []types.SIMCONNECT_RECV_ID{types.SIMCONNECT_RECV_ID_EVENT_FILENAME})
	fs := newTypedSubscription[FilenameEvent](id, msgSub, bufferSize)
	forwardTyped(m, fs, kind, func(msg engine.Message) (FilenameEvent, bool) {
		fname := msg.AsEventFilename()
		if fname == nil || fname.UEventID != types.DWORD(eventID) {
			return FilenameEvent{}, false
		}
		return FilenameEvent{Filename: engine.BytesToString(fname.SzFileName[:])}, true
	})
	return fs
}
