//go:build windows
// +build windows

package manager

// FlightLoad requests the simulator to load the flight file at the given path.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) FlightLoad(flightFile string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.FlightLoad(flightFile)
}

// FlightPlanLoad requests the simulator to load the flight plan at the given path.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) FlightPlanLoad(flightPlanFile string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.FlightPlanLoad(flightPlanFile)
}

// FlightSave requests the simulator to save the current flight to the given path.
// Returns ErrNotConnected if not connected to the simulator.
func (m *Instance) FlightSave(flightFile string, title string, description string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engine == nil {
		return ErrNotConnected
	}
	return m.engine.FlightSave(flightFile, title, description)
}
