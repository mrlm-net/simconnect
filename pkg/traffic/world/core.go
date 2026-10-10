package world

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// core is the traffic engine's state that outlives a connection: what the
// library's World will hold (#710). One per process today; the control
// center of each connection reaches it as cc.core.
type core struct {
	// log is the traffic log (console, file, /api/control/log).
	log *trafficLog

	// cityOf: an airport's city, set by the host (SetAirportCity).
	cityOf atomic.Pointer[func(icao string) string]

	// runwaysClosed: runways closed by the host (closures.go), made once.
	runwaysClosed *closures
	closedOnce    sync.Once

	// runwayMu guards selectors and inUse: each airport's runway in use,
	// one selector for the traffic and its ATIS (#454), and the last one
	// logged (#465).
	runwayMu  sync.Mutex
	selectors map[string]*nav.RunwaySelector
	inUse     map[string]string
	// busyMu guards busy: the airports with more arrivals within
	// runwayChangeGapNM on the final than finish on the old runway (the
	// towers' last look): no moment to change the runway in use.
	busyMu sync.Mutex
	busy   map[string]bool

	// ils: the airports' ILS and the lookups under way; circuits and
	// vfrSets: the circuits and reporting points set on the map, saved
	// beside it.
	ils      ilsStore
	circuits circuitStore
	vfrSets  vfrPointStore

	// player is the host's clearance of the user aircraft (World.ClearPlayer).
	player playerState

	// hooks are the host's (Options): the radio, COM1, scenes.
	hooks Options

	// zone is the managed airports' control zone class (-airspace, #570).
	zone traffic.AirspaceClass
	// pushes plans every stand's standard push of the airports we push
	// back at, in the background (planStandardPushes).
	pushes pushPlanQueue
}

func newCore(log *trafficLog) *core {
	return &core{log: log, zone: traffic.ClassD, pushes: pushPlanQueue{log: log}, selectors: map[string]*nav.RunwaySelector{}, inUse: map[string]string{}, busy: map[string]bool{},
		ils:      ilsStore{byICAO: map[string]map[string]ilsInfo{}, pending: map[nav.FixKey][]ilsRef{}},
		circuits: circuitStore{m: map[string]map[string]traffic.CircuitConfig{}},
		vfrSets:  vfrPointStore{m: map[string][]traffic.ReportingPoint{}}}
}

// runwaySelector is icao's runway selector, made on first use.
func (k *core) runwaySelector(icao string) *nav.RunwaySelector {
	k.runwayMu.Lock()
	defer k.runwayMu.Unlock()
	s := k.selectors[icao]
	if s == nil {
		// A change that is due waits for a gap in the traffic, as a tower
		// supervisor times it (the selector waits RunwayChangeMaxWait at
		// most; out of limits it changes at once).
		s = &nav.RunwaySelector{Ready: func(from, to nav.RunwayUse) bool {
			k.busyMu.Lock()
			defer k.busyMu.Unlock()
			return !k.busy[icao]
		}}
		k.selectors[icao] = s
	}
	return s
}

// setRunwaysBusy replaces the airports with no moment to change runway.
func (k *core) setRunwaysBusy(busy map[string]bool) {
	k.busyMu.Lock()
	k.busy = busy
	k.busyMu.Unlock()
}

// runwayChangeGapNM: an arrival this close on the final lands on the
// runway in use before a change.
const runwayChangeGapNM = 10.0

// logRunwayChange logs a change of icao's runway in use, with the wind
// that made it (#465: to see every change, and why, in the traffic log).
func (k *core) logRunwayChange(icao string, use nav.RunwayUse, w nav.Weather) {
	now := strings.Join(nav.Names(use.Departures), "+") + "/" + strings.Join(nav.Names(use.Arrivals), "+")
	k.runwayMu.Lock()
	before := k.inUse[icao]
	k.inUse[icao] = now
	k.runwayMu.Unlock()
	if before == now {
		return
	}
	head, cross := w.Components(use.Arrival.Heading)
	mode := ""
	if use.Parallel != nav.ParallelNone {
		mode = fmt.Sprintf(", %s, %.0f m apart", use.Parallel, use.SpacingM)
	}
	k.log.printf("runway in use %s: %s → %s (departures/arrivals)%s, wind %03.0f°/%.0f kt gust %.0f: headwind %.1f kt, crosswind %.1f kt on %s", icao, orNone(before), now, mode, w.WindDirTrue, w.WindKts, w.GustKts, head, cross, use.Arrival.Name)
}

// libraryIDs are the SimConnect ID bases of the library helpers the World
// creates (Options.IDBase).
type libraryIDs struct {
	loaderDef, loaderReq, procDef, procReq, navDef, navReq, airportList, injDef, injReq, injEvt uint32
	crawlDef, crawlReq, enrouteReq                                                              uint32
	// puppetRec, puppetReq: a primary's recorder, a follower's puppets (#964).
	puppetRec, puppetReq uint32
}

// libIDs are the helpers' IDs: the library defaults, or moved to IDBase.
func (k *core) libIDs() libraryIDs {
	if b := k.hooks.IDBase; b != 0 {
		return libraryIDs{b, b + 100, b + 200, b + 300, b + 400, b + 500, b + 600, b + 700, b + 800, b + 900, b + 1000, b + 1010, b + 1100, b + 2100, b + 2200}
	}
	return libraryIDs{airport.DefaultLoaderDefinitionBase, airport.DefaultLoaderRequestBase, airport.DefaultProcedureDefinitionBase, airport.DefaultProcedureRequestBase,
		nav.DefaultNavDefinitionBase, nav.DefaultNavRequestBase, traffic.DefaultAirportListRequestID,
		traffic.DefaultInjectDefinitionBase, traffic.DefaultInjectRequestBase, traffic.DefaultInjectEventBase, airwayCrawlDefBase, airwayCrawlReqBase, enrouteReqBase,
		puppetRecBase, puppetReqBase}
}
