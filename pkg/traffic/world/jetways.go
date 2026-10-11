package world

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/jetway"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Jetways for our traffic (#572): switched on or off (off by default),
// docked by a jetway.Service. With them off, or the service not available
// (no helper package), nothing is spawned and traffic parks and pushes as
// without them. On: our airports' jetway gates are prepared, an arrival
// parked on a gate (not a type with its own airstairs) and a departure
// spawned on one get the jetway, and ground holds a push until it is home.

var jetwaysOn struct {
	mu sync.Mutex
	on bool
}

// SetJetways switches jetways for our traffic on or off. Off, every helper
// and placeholder is removed (the jetways go home).
func (w *World) SetJetways(on bool) {
	jetwaysOn.mu.Lock()
	was := jetwaysOn.on
	jetwaysOn.on = on
	jetwaysOn.mu.Unlock()
	if was == on {
		return
	}
	w.st.core.log.printf("jetways for our traffic %s", map[bool]string{true: "on", false: "off"}[on])
	w.st.mu.Lock()
	cc := w.st.control
	w.st.mu.Unlock()
	if cc != nil && !on {
		cc.releaseJetways()
	}
}

func jetwaysEnabled() bool {
	jetwaysOn.mu.Lock()
	defer jetwaysOn.mu.Unlock()
	return jetwaysOn.on
}

// JetwayView is the jetways' state: switched on, whether the backend can
// work and why not, and each gate's jetway by airport and stand.
type JetwayView struct {
	On        bool                              `json:"on"`
	Available bool                              `json:"available"`
	Reason    string                            `json:"reason,omitempty"`
	Gates     map[string]map[int]jetway.Status `json:"gates,omitempty"`
}

// Jetways is the jetways' state now.
func (w *World) Jetways() JetwayView {
	v := JetwayView{On: jetwaysEnabled(), Reason: "not connected"}
	w.st.mu.Lock()
	cc := w.st.control
	w.st.mu.Unlock()
	if cc == nil {
		return v
	}
	svc := cc.jetwayService()
	if svc == nil {
		v.Reason = "jetway helper package missing"
		return v
	}
	v.Available, v.Reason = svc.Available()
	cc.jw.mu.Lock()
	defer cc.jw.mu.Unlock()
	v.Gates = map[string]map[int]jetway.Status{}
	for k := range cc.jw.docked {
		icao, stand := splitGateKey(k)
		if v.Gates[icao] == nil {
			v.Gates[icao] = map[int]jetway.Status{}
		}
		v.Gates[icao][stand] = svc.Status(icao, stand)
	}
	return v
}

// jetwayState is a connection's jetway backend and what it has asked of
// it.
type jetwayState struct {
	mu       sync.Mutex
	svc      *jetway.Helpers
	prepared map[string]bool
	docked   map[string]bool // "ICAO stand" asked to dock
}

func gateKey(icao string, stand int) string { return fmt.Sprintf("%s %d", icao, stand) }

func splitGateKey(k string) (string, int) {
	var icao string
	var stand int
	fmt.Sscanf(k, "%s %d", &icao, &stand)
	return icao, stand
}

// helperTypes are the types of the helper models the simulator lists.
func (cc *controlCenter) helperTypes() []string {
	var out []string
	for _, t := range cc.modelList() {
		if !jetway.IsHelper(t) || strings.HasSuffix(t, " ALL") {
			continue
		}
		out = append(out, strings.TrimPrefix(t, jetway.HelperTitlePrefix))
	}
	return out
}

// jetwayBuilt is the connection's helper backend if made, nil else (no
// model list scan: every message asks).
func (cc *controlCenter) jetwayBuilt() *jetway.Helpers {
	cc.jw.mu.Lock()
	defer cc.jw.mu.Unlock()
	return cc.jw.svc
}

// jetwayService is the connection's helper backend, made once the helper
// models are listed; nil before, or without them.
func (cc *controlCenter) jetwayService() *jetway.Helpers {
	cc.jw.mu.Lock()
	defer cc.jw.mu.Unlock()
	if cc.jw.svc != nil {
		return cc.jw.svc
	}
	types := cc.helperTypes()
	if len(types) == 0 {
		return nil
	}
	cc.jw.svc = jetway.NewHelpers(cc.client, jetway.HelperOptions{Types: types, RetractPlan: cc.retractPlan,
		RequestBase: cc.core.libIDs().puppetReq + 1000, Log: cc.core.log.printf})
	cc.jw.prepared, cc.jw.docked = map[string]bool{}, map[string]bool{}
	return cc.jw.svc
}

// retractPlan is a flight plan file out of icao to the nearest other
// airport known, for the placeholders (written once per airport).
func (cc *controlCenter) retractPlan(icao string) (string, error) {
	if cc.airportRefs == nil {
		return "", fmt.Errorf("no airports known")
	}
	refs := cc.airportRefs()
	from, ok := refs[icao]
	best, bestD := traffic.AirportRef{}, math.Inf(1)
	for code, r := range refs {
		if code == icao || len(code) != 4 {
			continue
		}
		if d := calc.HaversineNM(from.Position.Lat, from.Position.Lon, r.Position.Lat, r.Position.Lon); d >= 20 && d < bestD {
			best, bestD = r, d
		}
	}
	dir := cc.core.hooks.DataDir
	if !ok || best.ICAO == "" {
		return "", fmt.Errorf("no airport known near %s", icao)
	}
	pt := func(r traffic.AirportRef) string { return nav.FormatLLA(r.Position, r.AltM*3.28084) }
	pln := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<SimBase.Document Type="AceXML" version="1,1">
    <Descr>AceXML Document</Descr>
    <FlightPlan.FlightPlan>
        <Title>%[1]s to %[2]s</Title>
        <FPType>IFR</FPType>
        <RouteType>HighAlt</RouteType>
        <CruisingAlt>15000</CruisingAlt>
        <DepartureID>%[1]s</DepartureID>
        <DepartureLLA>%[3]s</DepartureLLA>
        <DestinationID>%[2]s</DestinationID>
        <DestinationLLA>%[4]s</DestinationLLA>
        <ATCWaypoint id="%[1]s">
            <ATCWaypointType>Airport</ATCWaypointType>
            <WorldPosition>%[3]s</WorldPosition>
            <ICAO><ICAOIdent>%[1]s</ICAOIdent></ICAO>
        </ATCWaypoint>
        <ATCWaypoint id="%[2]s">
            <ATCWaypointType>Airport</ATCWaypointType>
            <WorldPosition>%[4]s</WorldPosition>
            <ICAO><ICAOIdent>%[2]s</ICAOIdent></ICAO>
        </ATCWaypoint>
    </FlightPlan.FlightPlan>
</SimBase.Document>
`, icao, best.ICAO, pt(from), pt(best))
	if dir == "" {
		dir = "."
	}
	path, err := filepath.Abs(filepath.Join(dir, "jetways", icao+"-retract.pln"))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	return path, os.WriteFile(path, []byte(pln), 0o644)
}

// jetwayTick runs every second: our airports prepared, gates docked for
// parked aircraft, the backend's timers.
func (cc *controlCenter) jetwayTick(now time.Time) {
	if !jetwaysEnabled() {
		return
	}
	svc := cc.jetwayService()
	if svc == nil {
		return
	}
	if ok, _ := svc.Available(); !ok {
		return
	}
	cc.mu.Lock()
	airports := make([]string, 0, len(cc.stands))
	for icao := range cc.stands {
		airports = append(airports, icao)
	}
	items := make([]*controlled, 0, len(cc.items))
	for _, it := range cc.items {
		items = append(items, it)
	}
	cc.mu.Unlock()
	cc.jw.mu.Lock()
	for _, icao := range airports {
		if !cc.jw.prepared[icao] {
			cc.jw.prepared[icao] = true
			if err := svc.Prepare(icao); err != nil {
				cc.core.log.printf("jetways %s: %v", icao, err)
			}
		}
	}
	cc.jw.mu.Unlock()
	for _, it := range items {
		it.mu.Lock()
		icao, stand, model := it.ICAO, it.stand, it.view.Model
		it.mu.Unlock()
		if stand < 0 || !cc.jetwayParked(it) || traffic.NoJetway(model) {
			continue
		}
		k := gateKey(icao, stand)
		cc.jw.mu.Lock()
		asked := cc.jw.docked[k]
		cc.jw.docked[k] = true
		cc.jw.mu.Unlock()
		if !asked {
			if err := svc.Dock(icao, stand, model); err != nil {
				cc.jw.mu.Lock()
				delete(cc.jw.docked, k)
				cc.jw.mu.Unlock()
			}
		}
	}
	svc.Update(now)
}

// jetwayParked reports that it stands still on its gate for a jetway: an
// arrival parked, a departure waiting for its push.
func (cc *controlCenter) jetwayParked(it *controlled) bool {
	switch {
	case it.arr != nil:
		return it.arr.State() == traffic.ArrivalParked
	case it.dep != nil:
		return it.dep.State() == traffic.TaxiAwaitingPushback
	}
	return false
}

// jetwayHolds undocks it's jetway before its push: true while ground must
// wait (the jetway still on its way home).
func (it *controlled) jetwayHolds() bool {
	cc := it.cc
	svc := cc.jetwayBuilt()
	if svc == nil {
		return false
	}
	k := gateKey(it.ICAO, it.stand)
	cc.jw.mu.Lock()
	asked := cc.jw.docked[k]
	cc.jw.mu.Unlock()
	if !asked {
		return false
	}
	switch svc.Status(it.ICAO, it.stand) {
	case jetway.StatusDocking, jetway.StatusDocked:
		svc.Undock(it.ICAO, it.stand)
		return true
	case jetway.StatusUndocking:
		return true
	}
	cc.jw.mu.Lock()
	delete(cc.jw.docked, k)
	cc.jw.mu.Unlock()
	return false
}

// releaseJetways removes every helper and placeholder of the connection.
func (cc *controlCenter) releaseJetways() {
	cc.jw.mu.Lock()
	svc := cc.jw.svc
	prepared := cc.jw.prepared
	cc.jw.prepared, cc.jw.docked = map[string]bool{}, map[string]bool{}
	cc.jw.mu.Unlock()
	if svc == nil {
		return
	}
	for icao := range prepared {
		svc.Release(icao)
	}
}

// registerJetways serves GET /api/jetways and POST /api/jetways {"on": true}.
func registerJetways(mux *http.ServeMux, w *World) {
	mux.HandleFunc("GET /api/jetways", func(rw http.ResponseWriter, r *http.Request) {
		writeJSON(rw, w.Jetways())
	})
	mux.HandleFunc("POST /api/jetways", func(rw http.ResponseWriter, r *http.Request) {
		var req struct {
			On bool `json:"on"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		w.SetJetways(req.On)
		writeJSON(rw, w.Jetways())
	})
}
