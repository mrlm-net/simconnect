package world

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Traffic along the user's route (#740): while the host's flight cruises,
// the World keeps a few airliners around it — ahead the same way, coming
// the other way, crossing — created airborne and flown by MSFS AI
// (traffic.CorridorRoute, the enroute machinery of #369), and takes them
// out once they are DespawnNM from the user aircraft.

// CorridorSettings are the user's flight and how much traffic around it.
type CorridorSettings struct {
	Enabled bool `json:"enabled"`
	// Route is the user's route ahead, in its direction (two or more
	// points; the whole flight plan will do).
	Route []airport.LatLon `json:"route"`
	// LevelFt and Kts: the user's cruise level and speed (0 kt: 450).
	LevelFt float64 `json:"levelFt"`
	Kts     float64 `json:"kts,omitempty"`
	// How many of each kind at once (nil: 1).
	Same     *int `json:"same,omitempty"`
	Opposite *int `json:"opposite,omitempty"`
	Crossing *int `json:"crossing,omitempty"`
	// DespawnNM: one this far from the user aircraft is taken out (0: 80).
	DespawnNM float64 `json:"despawnNM,omitempty"`
}

// SetCorridor sets the traffic along the user's route (#740).
func (w *World) SetCorridor(c CorridorSettings) error {
	_, err := w.Do(http.MethodPost, "/api/corridor", c)
	return err
}

// corridorEvery: the corridor is looked after this often (traffic time).
const corridorEvery = 15 * time.Second

func (c CorridorSettings) want(k traffic.CorridorKind) int {
	n := map[traffic.CorridorKind]*int{traffic.CorridorSame: c.Same, traffic.CorridorOpposite: c.Opposite, traffic.CorridorCrossing: c.Crossing}[k]
	if n == nil {
		return 1
	}
	return *n
}

func (c CorridorSettings) despawnNM() float64 {
	if c.DespawnNM > 0 {
		return c.DespawnNM
	}
	return 80
}

// corridorTick keeps the corridor: those too far from the user out, one
// missing in (scheduler goroutine).
func (s *scheduler) corridorTick(now time.Time) {
	s.mu.Lock()
	c := s.corridor
	due := c != nil && now.Sub(s.corridorAt) >= corridorEvery
	if due {
		s.corridorAt = now
	}
	s.mu.Unlock()
	if !due || !c.Enabled {
		return
	}
	user := s.st.core.userAt()
	if user.Lat == 0 && user.Lon == 0 {
		return
	}
	pos := map[uint32]airport.LatLon{}
	for _, a := range s.cc.world.Aircraft() {
		pos[a.ObjectID] = a.Position
	}
	have := map[traffic.CorridorKind]int{}
	var far []*enrouteAC
	s.mu.Lock()
	for _, e := range s.enroute {
		if e.corridor == "" {
			continue
		}
		// Far and going farther (one coming at the user may appear
		// beyond DespawnNM).
		if p, ok := pos[e.objectID]; ok {
			d := calc.HaversineNM(p.Lat, p.Lon, user.Lat, user.Lon)
			away := e.corridorNM > 0 && d > e.corridorNM
			e.corridorNM = d
			if d > c.despawnNM() && away {
				far = append(far, e)
				continue
			}
		}
		have[e.corridor]++
	}
	for _, e := range s.pending {
		have[e.corridor]++
	}
	s.mu.Unlock()
	for _, e := range far {
		s.cc.log.printf("%-6s corridor: %s, %.0f NM from the user: out", e.f.Callsign, e.corridor, c.despawnNM())
		s.dropEnroute(e)
	}
	for _, k := range []traffic.CorridorKind{traffic.CorridorSame, traffic.CorridorOpposite, traffic.CorridorCrossing} {
		if have[k] < c.want(k) {
			if err := s.spawnCorridor(k, *c, user); err != nil {
				s.cc.log.printf("corridor: %s: %v", k, err)
			}
			return // one a time
		}
	}
}

// spawnCorridor creates one aircraft of kind around the user's flight: an
// airline of the schedule's, one of its types, a model of it.
func (s *scheduler) spawnCorridor(kind traffic.CorridorKind, c CorridorSettings, user airport.LatLon) error {
	rng := rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0x740))
	// An airline with a jet that cruises up there (2000 ft above the user
	// at most): no turboprop at FL340.
	type pick struct {
		a   traffic.Airline
		typ string
	}
	var picks []pick
	for _, a := range s.cfg.Airlines {
		for t := range a.Fleet {
			if p := nav.PerformanceFor(t); p.CruiseTASKts >= 400 && float64(p.MaxFL)*100 >= c.LevelFt+2000 {
				picks = append(picks, pick{a, t})
			}
		}
	}
	if len(picks) == 0 {
		return fmt.Errorf("no airline type cruises at FL%03d", int(c.LevelFt/100))
	}
	slices.SortFunc(picks, func(x, y pick) int { return strings.Compare(x.a.ICAO+x.typ, y.a.ICAO+y.typ) })
	pk := picks[rng.IntN(len(picks))]
	a, typ := pk.a, pk.typ
	route, ok := traffic.CorridorRoute(kind, traffic.CorridorOptions{Route: c.Route, At: user, LevelFt: c.LevelFt, Kts: nav.PerformanceFor(typ).CruiseTASKts}, rng)
	if !ok {
		return fmt.Errorf("no room on the route ahead")
	}
	if who := s.nearPoint(route[0].Position, route[0].AltFt); who != "" {
		return fmt.Errorf("%s near where it would appear", who)
	}
	cs := fmt.Sprintf("%s%d", a.ICAO, 100+rng.IntN(8900))
	models := traffic.ModelsForFlight(s.cc.modelList(), a.ICAO, a.Name, typ, cs, 6)
	if len(models) == 0 {
		return fmt.Errorf("no model of a %s %s", a.ICAO, typ)
	}
	model := models[rng.IntN(len(models))]
	spawn, wps, err := traffic.EnrouteStart(route)
	if err != nil {
		return err
	}
	e := &enrouteAC{f: traffic.ManagedFlight{Flight: traffic.Flight{Callsign: cs, Airline: a.ICAO, Type: typ}, Kind: "overflight"},
		model: model, waypoints: wps, route: route, corridor: kind}
	title, livery, _ := strings.Cut(model, liverySep)
	s.mu.Lock()
	e.reqID, e.pendingAt = s.freeEnrouteReq(), time.Now()
	s.pending[e.reqID] = e
	s.mu.Unlock()
	err = s.cc.do(func() error {
		return s.cc.sim.SpawnEnroute(traffic.NonATCOpts{Model: title, Livery: livery, Tail: cs, Position: spawn}, e.reqID)
	})
	if err != nil {
		s.mu.Lock()
		delete(s.pending, e.reqID)
		s.mu.Unlock()
		return err
	}
	s.cc.log.printf("%-6s corridor: %s, %s, FL%03d, %.0f NM from the user", cs, kind, typ, int(route[0].AltFt/100),
		calc.HaversineNM(user.Lat, user.Lon, route[0].Position.Lat, route[0].Position.Lon))
	return nil
}

// corridorView is the corridor and its aircraft (GET /api/corridor).
type corridorView struct {
	CorridorSettings
	Aircraft []corridorAircraft `json:"aircraft"`
}

type corridorAircraft struct {
	Callsign string               `json:"callsign"`
	Kind     traffic.CorridorKind `json:"kind"`
	Model    string               `json:"model"`
}

func registerCorridor(mux *http.ServeMux, st *state) {
	sched := func(w http.ResponseWriter) *scheduler {
		st.mu.Lock()
		s := st.schedule
		st.mu.Unlock()
		if s == nil {
			http.Error(w, "simulator not connected", http.StatusServiceUnavailable)
		}
		return s
	}
	// GET /api/corridor — the traffic along the user's route and its
	// aircraft; POST CorridorSettings sets it ("enabled": false takes them
	// out).
	mux.HandleFunc("GET /api/corridor", func(w http.ResponseWriter, r *http.Request) {
		s := sched(w)
		if s == nil {
			return
		}
		v := corridorView{Aircraft: []corridorAircraft{}}
		s.mu.Lock()
		if s.corridor != nil {
			v.CorridorSettings = *s.corridor
		}
		for _, e := range s.enroute {
			if e.corridor != "" {
				v.Aircraft = append(v.Aircraft, corridorAircraft{Callsign: e.f.Callsign, Kind: e.corridor, Model: e.model})
			}
		}
		s.mu.Unlock()
		writeJSON(w, v)
	})
	mux.HandleFunc("POST /api/corridor", func(w http.ResponseWriter, r *http.Request) {
		s := sched(w)
		if s == nil {
			return
		}
		var c CorridorSettings
		if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if c.Enabled && (len(c.Route) < 2 || c.LevelFt <= 0) {
			http.Error(w, "a route of two or more points and a level are needed", http.StatusUnprocessableEntity)
			return
		}
		s.mu.Lock()
		s.corridor, s.corridorAt = &c, time.Time{}
		var out []*enrouteAC
		if !c.Enabled {
			for _, e := range s.enroute {
				if e.corridor != "" {
					out = append(out, e)
				}
			}
		}
		s.mu.Unlock()
		for _, e := range out {
			s.dropEnroute(e)
		}
		s.cc.log.printf("corridor: %v, FL%03d, %d points", c.Enabled, int(c.LevelFt/100), len(c.Route))
		w.WriteHeader(http.StatusNoContent)
	})
}
