package world

import "github.com/mrlm-net/simconnect/pkg/traffic"

// SetAirportCity gives the World an airport's city (the simulator's
// facility data names only the airport, "Schwechat"; the host reads the
// city elsewhere): departure clearances name the destination by it
// (traffic.ClearanceLimit). nil: the schedule's airport names.
func (w *World) SetAirportCity(f func(icao string) string) {
	if f == nil {
		w.st.core.cityOf.Store(nil)
		return
	}
	w.st.core.cityOf.Store(&f)
}

// clearanceLimit is icao as a departure clearance names it: the table's
// name, else its city (SetAirportCity), else the schedule's name.
func (cc *controlCenter) clearanceLimit(icao string) string {
	city := ""
	if f := cc.core.cityOf.Load(); f != nil {
		city = (*f)(icao)
	}
	if city == "" {
		city = cc.airportName(icao)
	}
	return traffic.ClearanceLimit(icao, city)
}
