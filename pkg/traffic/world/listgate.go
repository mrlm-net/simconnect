package world

import (
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// The airports around the user (the simulator's facility list, its reality
// bubble) were asked for every minute, whether or not the user had moved:
// each a list built on the simulator's main thread. The list only changes
// as the user moves (the picture measures its distances itself), so it is
// asked again once the user is airportListMoveNM from where it was last
// asked, or airportListMaxAge after.
const (
	airportListMoveNM = 10.0
	airportListMaxAge = 10 * time.Minute
)

// listGate tells when the airport list is due again.
type listGate struct {
	at      airport.LatLon
	askedAt time.Time
	asked   bool
}

// due reports whether the list should be asked for now, the user at user
// (zero: not known).
func (g *listGate) due(now time.Time, user airport.LatLon) bool {
	switch {
	case !g.asked:
		return true
	case now.Sub(g.askedAt) >= airportListMaxAge:
		return true
	case user == (airport.LatLon{}) || g.at == (airport.LatLon{}):
		return false
	}
	return calc.HaversineMeters(g.at.Lat, g.at.Lon, user.Lat, user.Lon)/1852 >= airportListMoveNM
}

// done records the list asked for now, the user at user.
func (g *listGate) done(now time.Time, user airport.LatLon) {
	g.at, g.askedAt, g.asked = user, now, true
}
