package world

import (
	"fmt"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// simFeed is what the simulator side tells the World (#710, option 3):
// the connection loop reads the simulator and hands each finding here, so
// a remote actuator can send the same over the network. Called in the
// connection's goroutine.
type simFeed interface {
	// Airports are the airports around (the traffic picture's).
	Airports(list []traffic.AirportRef)
	// Weather is the weather at the user aircraft.
	Weather(w nav.Weather)
	// ILS is a runway's ILS looked up (or the lookup failed).
	ILS(r nav.NavResult)
	// Procedures are an airport's SIDs, STARs and approaches.
	Procedures(p airport.Procedures)
	// Airways are an airport's airways read from the sim (#799).
	Airways(icao string, g *nav.AirwayGraph)
	// Layout is an airport loaded (l), or why it was not (err).
	Layout(icao string, l *airport.Layout, err error)
	// UserAircraft is the user's aircraft each second, with the simulation
	// rate; com1 its COM1 frequency ("" none).
	UserAircraft(a Aircraft, rate float64, com1 string)
	// Paused is the simulator paused or resumed.
	Paused(paused bool)
	// Traffic is a whole scan of the aircraft around.
	Traffic(scan []Traffic)
}

// localFeed is the simFeed of a World on its own connection: straight into
// its state.
type localFeed struct {
	st *state
	cc *controlCenter
}

func (f localFeed) Airports(list []traffic.AirportRef) { f.cc.world.SetAirports(list) }

func (f localFeed) Weather(w nav.Weather) {
	f.st.mu.Lock()
	f.st.weather = &w
	f.st.mu.Unlock()
}

func (f localFeed) ILS(r nav.NavResult) { f.st.core.gotILS(r) }

func (f localFeed) Airways(icao string, g *nav.AirwayGraph) { f.st.addAirways(icao, g) }

func (f localFeed) Procedures(p airport.Procedures) {
	fmt.Printf("🧭 %s procedures: %d SIDs, %d STARs, %d approaches\n", p.ICAO, len(p.Departures), len(p.Arrivals), len(p.Approaches))
	f.st.mu.Lock()
	if f.st.procedures == nil {
		f.st.procedures = map[string]airport.Procedures{}
	}
	f.st.procedures[p.ICAO] = p
	f.st.mu.Unlock()
}

func (f localFeed) Layout(icao string, _ *airport.Layout, err error) { f.st.finish(icao, err) }

func (f localFeed) UserAircraft(a Aircraft, rate float64, com1 string) {
	cc := f.cc
	f.st.mu.Lock()
	if rate > 0 && rate != cc.clock.Rate() {
		cc.clock.SetRate(rate)
		cc.log.printf("simulation rate %g×: traffic follows it", rate)
	}
	a.SimRate, a.Paused, a.Updated = cc.clock.Rate(), cc.clock.Paused(), time.Now()
	f.st.aircraft = &a
	f.st.mu.Unlock()
	f.st.core.setUserAt(airport.LatLon{Lat: a.Latitude, Lon: a.Longitude})
	f.st.core.setLocalSec(a.LocalSec)
	if h := f.st.core.hooks.OnCom1; h != nil && com1 != "" {
		h(com1) // a voice follows it when synced
	}
}

func (f localFeed) Paused(paused bool) {
	cc := f.cc
	if paused != cc.clock.Paused() {
		cc.clock.SetPaused(paused)
		cc.log.printf("simulation %s: traffic %s", map[bool]string{true: "paused", false: "resumed"}[paused], map[bool]string{true: "stops", false: "goes on"}[paused])
	}
}

func (f localFeed) Traffic(scan []Traffic) {
	f.st.mu.Lock()
	f.st.traffic, f.st.trafficAt = scan, time.Now()
	f.st.mu.Unlock()
	// Not under st.mu: reportTraffic takes cc.mu, and an aircraft handing
	// off holds its own lock while it reads st (the weather), with
	// /api/control taking cc.mu then the aircraft's: three locks in a ring
	// froze the map.
	f.cc.reportTraffic(scan)
}
