package world

import (
	"github.com/mrlm-net/simconnect/pkg/flight"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TitleFor is the installed aircraft title the World would fly a flight of
// icaoType as (#1027): the type in the livery of the airline of callsign
// (its first three letters, when they are an ICAO code), else another of
// its size in that livery, else the type in any livery (traffic.ModelsFor).
// ok is false before the simulator has listed its models, or when nothing
// installed fits.
func (w *World) TitleFor(icaoType, callsign string) (title string, ok bool) {
	w.st.mu.Lock()
	cc := w.st.control
	w.st.mu.Unlock()
	if cc == nil || icaoType == "" {
		return "", false
	}
	models := traffic.ModelsForFlight(cc.modelList(), airlineOf(callsign), "", icaoType, callsign, 1)
	if len(models) == 0 {
		return "", false
	}
	return models[0], true
}

// FleetFallback is a flight.FleetOptions.Fallback from TitleFor: a recorded
// aircraft whose own title is not installed here is created as the
// installed model of its type, in its airline's livery where there is one.
func (w *World) FleetFallback() func(a *flight.SceneAircraft) string {
	return func(a *flight.SceneAircraft) string {
		t, _ := w.TitleFor(a.Type, a.Callsign)
		return t
	}
}
