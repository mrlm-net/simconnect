//go:build windows
// +build windows

package manager

import (
	"sync"

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

type filenameSubscription struct {
	id      string
	sub     Subscription
	ch      chan FilenameEvent
	done    chan struct{}
	mgr     *Instance
	closeMu sync.Mutex
}

func (s *filenameSubscription) ID() string                   { return s.id }
func (s *filenameSubscription) Events() <-chan FilenameEvent { return s.ch }
func (s *filenameSubscription) Done() <-chan struct{}        { return s.done }
func (s *filenameSubscription) Unsubscribe() {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if s.sub != nil {
		s.sub.Unsubscribe()
		s.sub = nil
	}
	select {
	case <-s.done:
		// already closed
	default:
		close(s.done)
	}
	// ensure channel closed
	select {
	case <-s.ch:
	default:
		close(s.ch)
	}
}

// SubscribeOnFlightLoaded returns a subscription delivering FlightLoaded filenames
func (m *Instance) SubscribeOnFlightLoaded(id string, bufferSize int) FilenameSubscription {
	id = subscriptions.GenerateID(id)
	bufferSize = subscriptions.ValidateBufferSize(bufferSize)
	// subscribe to filename messages
	msgSub := m.SubscribeWithType(id+"-fname", bufferSize, []types.SIMCONNECT_RECV_ID{types.SIMCONNECT_RECV_ID_EVENT_FILENAME})
	fs := &filenameSubscription{id: id, sub: msgSub, ch: make(chan FilenameEvent, bufferSize), done: make(chan struct{}), mgr: m}

	go func() {
		defer fs.Unsubscribe()
		for {
			select {
			case <-m.ctx.Done():
				return
			case <-fs.sub.Done():
				return
			case msg, ok := <-fs.sub.Messages():
				if !ok {
					return
				}
				fname := msg.AsEventFilename()
				if fname == nil {
					continue
				}
				if fname.UEventID != types.DWORD(m.flightLoadedEventID) {
					continue
				}
				name := engine.BytesToString(fname.SzFileName[:])
				select {
				case fs.ch <- FilenameEvent{Filename: name}:
				default:
					m.logger.Debug("[manager] FlightLoaded subscription channel full, dropping event")
				}
			}
		}
	}()
	return fs
}

// SubscribeOnAircraftLoaded returns a subscription delivering AircraftLoaded filenames
func (m *Instance) SubscribeOnAircraftLoaded(id string, bufferSize int) FilenameSubscription {
	id = subscriptions.GenerateID(id)
	bufferSize = subscriptions.ValidateBufferSize(bufferSize)
	msgSub := m.SubscribeWithType(id+"-fname", bufferSize, []types.SIMCONNECT_RECV_ID{types.SIMCONNECT_RECV_ID_EVENT_FILENAME})
	fs := &filenameSubscription{id: id, sub: msgSub, ch: make(chan FilenameEvent, bufferSize), done: make(chan struct{}), mgr: m}

	go func() {
		defer fs.Unsubscribe()
		for {
			select {
			case <-m.ctx.Done():
				return
			case <-fs.sub.Done():
				return
			case msg, ok := <-fs.sub.Messages():
				if !ok {
					return
				}
				fname := msg.AsEventFilename()
				if fname == nil {
					continue
				}
				if fname.UEventID != types.DWORD(m.aircraftLoadedEventID) {
					continue
				}
				name := engine.BytesToString(fname.SzFileName[:])
				select {
				case fs.ch <- FilenameEvent{Filename: name}:
				default:
					m.logger.Debug("[manager] AircraftLoaded subscription channel full, dropping event")
				}
			}
		}
	}()
	return fs
}

// SubscribeOnFlightPlanActivated returns a subscription delivering FlightPlanActivated filenames
func (m *Instance) SubscribeOnFlightPlanActivated(id string, bufferSize int) FilenameSubscription {
	id = subscriptions.GenerateID(id)
	bufferSize = subscriptions.ValidateBufferSize(bufferSize)
	msgSub := m.SubscribeWithType(id+"-fname", bufferSize, []types.SIMCONNECT_RECV_ID{types.SIMCONNECT_RECV_ID_EVENT_FILENAME})
	fs := &filenameSubscription{id: id, sub: msgSub, ch: make(chan FilenameEvent, bufferSize), done: make(chan struct{}), mgr: m}

	go func() {
		defer fs.Unsubscribe()
		for {
			select {
			case <-m.ctx.Done():
				return
			case <-fs.sub.Done():
				return
			case msg, ok := <-fs.sub.Messages():
				if !ok {
					return
				}
				fname := msg.AsEventFilename()
				if fname == nil {
					continue
				}
				if fname.UEventID != types.DWORD(m.flightPlanActivatedEventID) {
					continue
				}
				name := engine.BytesToString(fname.SzFileName[:])
				select {
				case fs.ch <- FilenameEvent{Filename: name}:
				default:
					m.logger.Debug("[manager] FlightPlanActivated subscription channel full, dropping event")
				}
			}
		}
	}()
	return fs
}
