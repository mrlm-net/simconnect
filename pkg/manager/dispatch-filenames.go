//go:build windows
// +build windows

package manager

import (
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// processFilenameEvent handles SIMCONNECT_RECV_ID_EVENT_FILENAME messages.
func (m *Instance) processFilenameEvent(msg engine.Message) {
	fnameMsg := msg.AsEventFilename()
	if fnameMsg == nil {
		return
	}

	name := engine.BytesToString(fnameMsg.SzFileName[:])

	if fnameMsg.UEventID == types.DWORD(m.flightLoadedEventID) {
		m.logger.Debug("[manager] FlightLoaded event", "filename", name)
		// Invoke registered FlightLoaded handlers with panic recovery
		m.mu.RLock()
		if cap(m.flightLoadedHandlersBuf) < len(m.flightLoadedHandlers) {
			m.flightLoadedHandlersBuf = make([]FlightLoadedHandler, len(m.flightLoadedHandlers))
		} else {
			m.flightLoadedHandlersBuf = m.flightLoadedHandlersBuf[:len(m.flightLoadedHandlers)]
		}
		for i, e := range m.flightLoadedHandlers {
			m.flightLoadedHandlersBuf[i] = e.Fn.(FlightLoadedHandler)
		}
		hs := m.flightLoadedHandlersBuf
		m.mu.RUnlock()
		for _, h := range hs {
			handler := h // capture for closure
			n := name    // capture for closure
			safeCallHandler(m.logger, "FlightLoadedHandler", func() {
				handler(n)
			})
		}
	}

	if fnameMsg.UEventID == types.DWORD(m.aircraftLoadedEventID) {
		m.logger.Debug("[manager] AircraftLoaded event", "filename", name)
		m.mu.RLock()
		if cap(m.flightLoadedHandlersBuf) < len(m.aircraftLoadedHandlers) {
			m.flightLoadedHandlersBuf = make([]FlightLoadedHandler, len(m.aircraftLoadedHandlers))
		} else {
			m.flightLoadedHandlersBuf = m.flightLoadedHandlersBuf[:len(m.aircraftLoadedHandlers)]
		}
		for i, e := range m.aircraftLoadedHandlers {
			m.flightLoadedHandlersBuf[i] = e.Fn.(FlightLoadedHandler)
		}
		hs := m.flightLoadedHandlersBuf
		m.mu.RUnlock()
		for _, h := range hs {
			handler := h // capture for closure
			n := name    // capture for closure
			safeCallHandler(m.logger, "AircraftLoadedHandler", func() {
				handler(n)
			})
		}
	}

	if fnameMsg.UEventID == types.DWORD(m.flightPlanActivatedEventID) {
		m.logger.Debug("[manager] FlightPlanActivated event", "filename", name)
		m.mu.RLock()
		if cap(m.flightLoadedHandlersBuf) < len(m.flightPlanActivatedHandlers) {
			m.flightLoadedHandlersBuf = make([]FlightLoadedHandler, len(m.flightPlanActivatedHandlers))
		} else {
			m.flightLoadedHandlersBuf = m.flightLoadedHandlersBuf[:len(m.flightPlanActivatedHandlers)]
		}
		for i, e := range m.flightPlanActivatedHandlers {
			m.flightLoadedHandlersBuf[i] = e.Fn.(FlightLoadedHandler)
		}
		hs := m.flightLoadedHandlersBuf
		m.mu.RUnlock()
		for _, h := range hs {
			handler := h // capture for closure
			n := name    // capture for closure
			safeCallHandler(m.logger, "FlightPlanActivatedHandler", func() {
				handler(n)
			})
		}
	}
}
