//go:build windows

package manager

import (
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// processMessage handles a single message from the simulator.
// This method ensures defer msg.Release() fires at the end of each message processing,
// not at the end of the entire connection loop.
func (m *Instance) processMessage(msg engine.Message) {
	// Release the message buffer back to pool when done processing
	// This is critical for memory efficiency under high message load
	defer msg.Release()

	if msg.Err != nil {
		m.logger.Error("[manager] Stream error", "error", msg.Err)
		return
	}

	// Check for connection ready (OPEN) message
	if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_OPEN {
		m.logger.Debug("[manager] Received OPEN message, connection is now available")

		// Initialize simulator state and subscribe again what was kept over a
		// lost connection first, then announce the connection: an Available or
		// OnOpen handler subscribing a custom event is not subscribed a second
		// time by resubscribeCustomEvents (review #32).
		m.mu.Lock()
		client := m.engine
		m.mu.Unlock()

		if client != nil {
			m.registerSimStateSubscriptions(client)
			m.resubscribeCustomEvents(client) // kept over a lost connection (#405)
			m.replayUserSubscriptions(client) // opt-in, WithResubscribeOnReconnect (E10)
		}

		m.setState(StateAvailable)

		// Extract version information from OPEN message
		openMsg := msg.AsOpen()
		if openMsg != nil {
			appName := engine.BytesToString(openMsg.SzApplicationName[:])
			openData := types.ConnectionOpenData{
				ApplicationName:         appName,
				ApplicationVersionMajor: uint32(openMsg.DwApplicationVersionMajor),
				ApplicationVersionMinor: uint32(openMsg.DwApplicationVersionMinor),
				ApplicationBuildMajor:   uint32(openMsg.DwApplicationBuildMajor),
				ApplicationBuildMinor:   uint32(openMsg.DwApplicationBuildMinor),
				SimConnectVersionMajor:  uint32(openMsg.DwSimConnectVersionMajor),
				SimConnectVersionMinor:  uint32(openMsg.DwSimConnectVersionMinor),
				SimConnectBuildMajor:    uint32(openMsg.DwSimConnectBuildMajor),
				SimConnectBuildMinor:    uint32(openMsg.DwSimConnectBuildMinor),
			}
			m.setOpen(openData)
		}
	}

	// Check for quit message
	if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_QUIT {
		m.logger.Debug("[manager] Received QUIT message from simulator")
		quitData := types.ConnectionQuitData{}
		m.setQuit(quitData)
		m.setSimState(defaultSimState())
		m.setState(StateDisconnected)
		m.mu.Lock()
		// Closed once its stream ends (connectionLost): not here, where its
		// dispatcher may wait on this loop (#405).
		m.quitEngine, m.engine = m.engine, nil
		m.mu.Unlock()
	}

	// Handle pause and sim events. The OPEN, QUIT and event messages are handled
	// here and then forwarded like any other (#404): OnMessage handlers and
	// channel subscriptions (SubscribeOnPause, SubscribeOnSimRunning, the
	// filename and object events, custom system events) receive them too.
	if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_EVENT {
		m.processEventMessage(msg)
	}

	// Handle filename events (FlightLoaded, AircraftLoaded, FlightPlanActivated)
	if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_EVENT_FILENAME {
		m.processFilenameEvent(msg)
	}

	// Handle object add/remove events (ObjectAdded, ObjectRemoved)
	if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_EVENT_OBJECT_ADDREMOVE {
		m.processObjectEvent(msg)
	}

	// Handle camera state data
	if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA {
		m.processSimStateData(msg)
	}

	// Forward message to registered handlers
	m.mu.RLock()
	// Reuse pre-allocated slices, grow if necessary
	if cap(m.handlersBuf) < len(m.messageHandlers) {
		m.handlersBuf = make([]MessageHandler, len(m.messageHandlers))
	} else {
		m.handlersBuf = m.handlersBuf[:len(m.messageHandlers)]
	}
	for i, e := range m.messageHandlers {
		m.handlersBuf[i] = e.Fn.(MessageHandler)
	}
	if cap(m.subsBuf) < len(m.subscriptions) {
		m.subsBuf = make([]*subscription, 0, len(m.subscriptions))
	} else {
		m.subsBuf = m.subsBuf[:0]
	}
	for _, sub := range m.subscriptions {
		m.subsBuf = append(m.subsBuf, sub)
	}
	m.mu.RUnlock()

	for _, handler := range m.handlersBuf {
		h := handler   // capture for closure
		message := msg // capture for closure
		safeCallHandler(m.logger, "MessageHandler", func() {
			h(message)
		})
	}

	// Forward message to subscriptions (non-blocking): each gets a copy that
	// outlives this call, as the buffer goes back to the pool at its end (#404).
	var detached *engine.Message
	for _, sub := range m.subsBuf {
		// fast-path: skip closed subscriptions
		sub.closeMu.Lock()
		closed := sub.closed.Load()
		sub.closeMu.Unlock()
		if closed {
			continue
		}

		// Determine whether this subscription should receive the message
		allowed := true
		if sub.filter != nil {
			// Protect against panics in user-provided filters
			func() {
				defer func() {
					if r := recover(); r != nil {
						m.logger.Error("[manager] Subscription filter panic", "panic", r)
						allowed = false
					}
				}()
				allowed = sub.filter(msg)
			}()
		} else if len(sub.allowedTypes) > 0 {
			_, ok := sub.allowedTypes[types.SIMCONNECT_RECV_ID(msg.DwID)]
			allowed = ok
		}

		if !allowed {
			continue
		}

		sub.closeMu.Lock()
		if !sub.closed.Load() {
			select {
			case sub.ch <- detachOnce(msg, &detached):
			default:
				// Channel full, skip message to avoid blocking
				if sub.onDrop != nil {
					// Protect dispatch loop from user callback panics
					func() {
						defer func() {
							if r := recover(); r != nil {
								m.logger.Error("[manager] OnDrop callback panicked", "panic", r)
							}
						}()
						sub.onDrop(1)
					}()
				}
				m.logger.Debug("[manager] Subscription channel full, dropping message")
			}
		}
		sub.closeMu.Unlock()
	}
}

// detachOnce is msg detached from its pooled buffer, copied on the first
// call and shared after (subscribers only read it).
func detachOnce(msg engine.Message, d **engine.Message) engine.Message {
	if *d == nil {
		c := msg.Detach()
		*d = &c
	}
	return **d
}
