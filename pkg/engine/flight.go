//go:build windows
// +build windows

package engine

func (e *Engine) FlightLoad(flightFile string) error {
	return e.api.FlightLoad(flightFile)
}

func (e *Engine) FlightPlanLoad(flightPlanFile string) error {
	return e.api.FlightPlanLoad(flightPlanFile)
}

func (e *Engine) FlightSave(flightFile string, title string, description string) error {
	return e.api.FlightSave(flightFile, title, description)
}
