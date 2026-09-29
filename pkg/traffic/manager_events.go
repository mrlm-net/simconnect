//go:build windows
// +build windows

package traffic

import "time"

// ManagerEventKind is what happened in a TrafficManager.
type ManagerEventKind uint8

const (
	EventAdded      ManagerEventKind = iota // a flight entered the schedule
	EventTurnaround                         // an arrival and a departure became one aircraft (Reason "CSA101 → CSA102")
	EventStatus                             // a flight's status changed (Previous → Flight.Status); spawn, cancel and done are status changes
	EventRetry                              // a spawn failed and will be tried again (Reason: the error)
	EventBlocked                            // a spawn found its place taken and waits (ErrSpawnBlocked)
	EventDelayed                            // a situation check delayed a spawn (Reason)
	EventEstimated                          // a flight's estimated time changed (Flight.Estimated; zero: on time)
	EventHeld                               // a boarding departure is held on its stand (Reason)
	EventReleased                           // the hold is over
	EventRemoved                            // the flight's aircraft was taken out of the simulator
	EventEnabled                            // spawning switched on
	EventDisabled                           // spawning switched off
)

var managerEventNames = [...]string{"added", "turnaround", "status", "retry", "blocked", "delayed", "estimated", "held", "released", "removed", "enabled", "disabled"}

func (k ManagerEventKind) String() string {
	if int(k) < len(managerEventNames) {
		return managerEventNames[k]
	}
	return "unknown"
}

// MarshalText makes the kind its name in JSON.
func (k ManagerEventKind) MarshalText() ([]byte, error) { return []byte(k.String()), nil }

// ManagerEvent is one step of the manager's lifecycle, for an application
// to react to (logs, boards, sounds, its own rules): Flight is the flight
// as it is after the step (zero for EventEnabled/EventDisabled).
type ManagerEvent struct {
	Kind     ManagerEventKind `json:"kind"`
	Time     time.Time        `json:"time"`
	Flight   ManagedFlight    `json:"flight"`
	Previous FlightStatus     `json:"previous"` // EventStatus
	Reason   string           `json:"reason,omitempty"`
}

// Events delivers the lifecycle events; they are dropped when the channel
// (256) is full — use ManagerOptions.OnEvent to see every one.
func (m *TrafficManager) Events() <-chan ManagerEvent { return m.events }

func (m *TrafficManager) emit(k ManagerEventKind, f *ManagedFlight, now time.Time, reason string) {
	m.emitPrev(k, f, f.Status, now, reason)
}

func (m *TrafficManager) emitPrev(k ManagerEventKind, f *ManagedFlight, prev FlightStatus, now time.Time, reason string) {
	if now.IsZero() {
		now = time.Now()
	}
	m.pending = append(m.pending, ManagerEvent{Kind: k, Time: now, Flight: *f, Previous: prev, Reason: reason})
}

// unlock releases mu and delivers the events collected under it.
func (m *TrafficManager) unlock() {
	evs := m.pending
	m.pending = nil
	m.mu.Unlock()
	for _, e := range evs {
		m.deliver(e)
	}
}

func (m *TrafficManager) deliver(e ManagerEvent) {
	if m.onEvent != nil {
		m.onEvent(e)
	}
	select {
	case m.events <- e:
	default:
	}
}

// removeNow has the Spawner take f's aircraft out; outside mu.
func (m *TrafficManager) removeNow(f ManagedFlight) {
	m.spawner.Remove(f)
	m.deliver(ManagerEvent{Kind: EventRemoved, Time: time.Now(), Flight: f, Previous: f.Status, Reason: f.Err})
}
