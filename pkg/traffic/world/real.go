package world

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Real-world traffic (#841): with it on, the generator is off and the
// engine flies the aircraft a feed (ADS-B) observes, with all it does for
// its own (push, taxi, ATC, radio). Each is one flight by its ID, never
// spawned twice; once ours flies it, later sightings do not move it.
//
//   - arrival: appears where it is now (its sighting projected), flies to
//     the cheapest point of a STAR of the runway in use and is handed to
//     approach there;
//   - departure: on a stand (the one it is seen on, else by its type),
//     pushing at DepartAt or now; with no destination it flies the SID and
//     leaves the area;
//   - parked: on the stand it is seen on, waiting with no push until a
//     departure comes for its ID (Retime);
//   - overflight: not flown yet (TCAS, later).

const (
	// realStandNearM: a real aircraft on the ground takes the free stand
	// within this of where it is seen.
	realStandNearM = 80.0
	// realParkedFor: a parked aircraft's STD this far ahead (no push) until
	// its departure is seen.
	realParkedFor = 24 * time.Hour
	// realMinKts: the speed a slow or unknown sighting is flown at.
	realMinKts = 180.0
	// realMinAGLFt: an airborne sighting appears no lower than this over
	// the field's elevation.
	realMinAGLFt = 1500.0
)

// errNoOther: a flight with no other end known (a real aircraft's): the
// runway's procedure.
var errNoOther = errors.New("no other end known")

// realID is what one observed aircraft is in the manager: its arrival
// and departure call signs ("" none).
type realID struct {
	arrival, departure string
}

// ObserveResult is what Observe made of one sighting: Status "added",
// "updated", "retimed", "turnaround" or "ignored" (Reason says why).
type ObserveResult struct {
	ID       string `json:"id"`
	Kind     string `json:"kind,omitempty"`
	Callsign string `json:"callsign,omitempty"`
	Airport  string `json:"airport,omitempty"`
	Status   string `json:"status"`
	Reason   string `json:"reason,omitempty"`
}

// SetRealTraffic switches real-world traffic on or off (#841): on, the
// generator stops and its flights not yet in the simulator go (those
// flying finish); icao, when set, is the airport managed. Off, the
// generator comes back.
func (w *World) SetRealTraffic(on bool, icao string) error {
	_, err := w.Do(http.MethodPost, "/api/realtraffic", map[string]any{"on": on, "icao": icao})
	return err
}

// Observe takes a feed's snapshot (#841): one result per sighting.
func (w *World) Observe(obs []traffic.Observed) ([]ObserveResult, error) {
	b, err := w.Do(http.MethodPost, "/api/realflights", obs)
	if err != nil {
		return nil, err
	}
	var out []ObserveResult
	return out, json.Unmarshal(b, &out)
}

// Drop ends the flight of an aircraft the feed no longer sees (#841): not
// spawned yet, waiting on its stand or parked, it goes at once; in
// progress it plays out (lands and parks, or leaves), then goes.
func (w *World) Drop(id string) error {
	_, err := w.Do(http.MethodDelete, "/api/realflights/"+id, nil)
	return err
}

func registerReal(mux *http.ServeMux, st *state) {
	sched := func(w http.ResponseWriter) *scheduler {
		st.mu.Lock()
		s := st.schedule
		st.mu.Unlock()
		if s == nil {
			http.Error(w, "simulator not connected", http.StatusServiceUnavailable)
		}
		return s
	}
	mux.HandleFunc("GET /api/realtraffic", func(w http.ResponseWriter, r *http.Request) {
		s := sched(w)
		if s == nil {
			return
		}
		s.mu.Lock()
		on := s.realOn
		s.mu.Unlock()
		writeJSON(w, map[string]any{"on": on, "airports": s.mgr.Airports()})
	})
	mux.HandleFunc("POST /api/realtraffic", func(w http.ResponseWriter, r *http.Request) {
		s := sched(w)
		if s == nil {
			return
		}
		var req struct {
			On   bool   `json:"on"`
			ICAO string `json:"icao"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if icao := strings.ToUpper(strings.TrimSpace(req.ICAO)); icao != "" {
			if _, err := st.cache.Graph(icao); err != nil {
				http.Error(w, fmt.Sprintf("%s not loaded: %v", icao, err), http.StatusBadRequest)
				return
			}
			s.mgr.SetAirports(icao)
			s.mu.Lock()
			s.focus = []string{icao}
			s.mu.Unlock()
		}
		s.setReal(req.On)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/realflights", func(w http.ResponseWriter, r *http.Request) {
		s := sched(w)
		if s == nil {
			return
		}
		var obs []traffic.Observed
		if err := json.NewDecoder(r.Body).Decode(&obs); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		out := make([]ObserveResult, 0, len(obs))
		for _, o := range obs {
			out = append(out, s.observe(o))
		}
		writeJSON(w, out)
	})
	mux.HandleFunc("DELETE /api/realflights/{id}", func(w http.ResponseWriter, r *http.Request) {
		s := sched(w)
		if s == nil {
			return
		}
		if !s.drop(r.PathValue("id")) {
			http.Error(w, "no such aircraft", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// setReal switches real-world traffic: the generator off (and its flights
// not in the simulator yet gone) or back on.
func (s *scheduler) setReal(on bool) {
	s.mu.Lock()
	changed := s.realOn != on
	s.realOn, s.noGen = on, on
	s.mu.Unlock()
	if on {
		now := s.cc.clock.Now()
		for _, f := range s.mgr.Flights() {
			if f.Status == traffic.FlightScheduled && f.Observed == nil {
				s.mgr.Remove(f.Callsign, now)
			}
		}
		s.mgr.SetEnabled(true)
	} else if changed {
		s.mgr.Replan() // the generator back: the hours ahead asked again
	}
	if changed {
		s.cc.log.printf("schedule: real-world traffic %s", map[bool]string{true: "on", false: "off"}[on])
	}
}

// observe takes one sighting.
func (s *scheduler) observe(o traffic.Observed) ObserveResult {
	o.ID = strings.ToLower(strings.TrimSpace(o.ID))
	res := ObserveResult{ID: o.ID}
	ignore := func(why string) ObserveResult { res.Status, res.Reason = "ignored", why; return res }
	if o.ID == "" {
		return ignore("no id")
	}
	s.mu.Lock()
	on := s.realOn
	s.mu.Unlock()
	if !on {
		return ignore("real-world traffic is off")
	}
	icao, field, ok := s.realAirport(o)
	if !ok {
		return ignore("no managed airport")
	}
	res.Airport = icao
	if l, ok := s.st.cache.Layout(icao); !ok || len(l.Runways) == 0 {
		return ignore("no runways known at " + icao) // nothing could land or leave (live: runway "")
	}
	kind, why := traffic.ClassifyObserved(o, field)
	if kind == "" {
		return ignore(why)
	}
	res.Kind = kind
	cs := realCallsign(o)
	res.Callsign = cs
	now := s.cc.clock.Now()
	sight := o.Sighting()
	s.mu.Lock()
	if s.real == nil {
		s.real = map[string]*realID{}
	}
	id := s.real[o.ID]
	if id == nil {
		id = &realID{}
		s.real[o.ID] = id
	}
	arr, dep := id.arrival, id.departure
	s.mu.Unlock()
	live := func(kind, callsign string) (traffic.ManagedFlight, bool) {
		if callsign == "" {
			return traffic.ManagedFlight{}, false
		}
		f, ok := s.mgr.Flight(kind, callsign)
		return f, ok && f.Status != traffic.FlightDone && f.Status != traffic.FlightCancelled
	}
	af, hasArr := live("arrival", arr)
	df, hasDep := live("departure", dep)
	add := func(f traffic.Flight, kind string) {
		f.Observed = &sight
		s.mgr.Add([]traffic.Flight{f})
		s.mu.Lock()
		if kind == "arrival" {
			id.arrival = f.Callsign
		} else {
			id.departure = f.Callsign
		}
		s.mu.Unlock()
		res.Status = "added"
	}
	if o.Type == "" && !hasArr && !hasDep {
		return ignore("no type")
	}
	base := traffic.Flight{Callsign: cs, Airline: realAirline(cs), Type: strings.ToUpper(o.Type)}
	switch kind {
	case traffic.ObservedOverflight:
		return ignore("overflights are not flown yet")
	case traffic.ObservedArrival:
		switch {
		case hasArr:
			s.mgr.Observe("arrival", af.Callsign, sight, o.Origin, "")
			res.Callsign, res.Status = af.Callsign, "updated"
		case hasDep:
			return ignore("flying as departure " + df.Callsign)
		default:
			f := base
			f.Origin, f.Destination = strings.ToUpper(o.Origin), icao
			pos, _ := sight.At(time.Now()) // the feed's clock, not the simulator's
			kts := math.Max(o.GroundKts, realMinKts)
			f.STA = now.Add(time.Duration(calc.HaversineNM(pos.Lat, pos.Lon, field.Lat, field.Lon) / kts * float64(time.Hour)))
			f.DistanceNM = calc.HaversineNM(pos.Lat, pos.Lon, field.Lat, field.Lon)
			add(f, "arrival")
		}
	case traffic.ObservedParked:
		switch {
		case hasDep:
			s.mgr.Observe("departure", df.Callsign, sight, "", "")
			res.Callsign, res.Status = df.Callsign, "updated"
		case hasArr:
			s.mgr.Observe("arrival", af.Callsign, sight, "", "")
			res.Callsign, res.Status = af.Callsign, "updated" // ours flies it in
		default:
			f := base
			f.Origin, f.STD = icao, now.Add(realParkedFor)
			add(f, "departure")
		}
	case traffic.ObservedDeparture:
		at := o.DepartAt
		if at.IsZero() || at.Before(now) {
			at = now
		}
		switch {
		case hasDep:
			s.mgr.Observe("departure", df.Callsign, sight, "", strings.ToUpper(o.Destination))
			res.Callsign = df.Callsign
			if df.STD.Sub(at).Abs() < time.Minute {
				res.Status = "updated"
				break
			}
			res.Status = "retimed"
			if !s.mgr.Retime(df.Callsign, at) {
				res.Status, res.Reason = "updated", "already pushed"
				break
			}
			if it := s.cc.byTail(df.Callsign); it != nil && it.dep != nil {
				it.dep.SetPushbackAt(at)
				it.mu.Lock()
				deliver := it.deliver
				it.deliver = nil
				it.mu.Unlock()
				if deliver != nil {
					deliver() // parked until now: its first call
				}
			}
		case hasArr && af.Status <= traffic.FlightParked && af.TurnTo == "":
			// The aircraft ours flew in goes out again: its next flight.
			f := base // the same call sign both ways is fine: the keys differ by kind
			f.Origin, f.Destination, f.STD = icao, strings.ToUpper(o.Destination), at
			add(f, "departure")
			if s.mgr.Turn(af.Callsign, f.Callsign) {
				res.Status = "turnaround"
			}
		case hasArr:
			return ignore("flying as arrival " + af.Callsign)
		default:
			f := base
			f.Origin, f.Destination, f.STD = icao, strings.ToUpper(o.Destination), at
			add(f, "departure")
		}
	}
	return res
}

// drop ends an aircraft's flights (Drop): false when it is not known.
func (s *scheduler) drop(id string) bool {
	id = strings.ToLower(strings.TrimSpace(id))
	s.mu.Lock()
	r := s.real[id]
	delete(s.real, id)
	s.mu.Unlock()
	if r == nil {
		return false
	}
	now := s.cc.clock.Now()
	if r.arrival != "" {
		s.mgr.Drop("arrival", r.arrival, now)
	}
	if r.departure != "" {
		s.mgr.Drop("departure", r.departure, now)
	}
	return true
}

// realAirport is the managed airport a sighting is at or bound for: its
// destination or origin when managed, else the nearest managed one.
func (s *scheduler) realAirport(o traffic.Observed) (string, airport.LatLon, bool) {
	best, bestNM := "", math.Inf(1)
	var at airport.LatLon
	for _, icao := range s.mgr.Airports() {
		l, ok := s.st.cache.Layout(icao)
		if !ok {
			continue
		}
		p := airport.LatLon{Lat: l.Latitude, Lon: l.Longitude}
		d := calc.HaversineNM(o.Lat, o.Lon, p.Lat, p.Lon)
		if strings.EqualFold(icao, o.Destination) || strings.EqualFold(icao, o.Origin) {
			d = -1
		}
		if d < bestNM {
			best, bestNM, at = icao, d, p
		}
	}
	return best, at, best != ""
}

// realCallsign is the call sign a real aircraft flies under: its own, else
// its registration, else its ID.
func realCallsign(o traffic.Observed) string {
	for _, c := range []string{o.Callsign, o.Registration, o.ID} {
		if c = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(c), "-", "")); c != "" {
			return c
		}
	}
	return ""
}

// realAirline is the ICAO airline of a call sign of three letters and a
// flight number ("CSA123"); "" for a registration.
func realAirline(cs string) string {
	if len(cs) < 4 {
		return ""
	}
	for i, r := range cs[:3] {
		if !unicode.IsLetter(r) || i == 2 && !unicode.IsDigit(rune(cs[3])) {
			return ""
		}
	}
	return cs[:3]
}

// spawnObserved puts a real arrival in the air where it is now (#841): MSFS
// AI flies it to the cheapest point of a STAR of the runway in use, where
// it is handed to approach (handovers) as an enroute arrival is.
func (s *scheduler) spawnObserved(f traffic.ManagedFlight) error {
	cc := s.cc
	g, err := s.st.cache.Graph(f.Airport)
	if err != nil {
		return err
	}
	rwy := cc.pickRunway(g, true, -1)
	pos, alt := f.Observed.At(time.Now()) // the feed's clock, not the simulator's
	// Airborne: not below realMinAGLFt over the field.
	alt = math.Max(alt, g.Layout.Altitude/0.3048+realMinAGLFt)
	pts, name, expect, err := s.arrivalJoin(g, rwy, pos)
	if err != nil {
		// No STAR to join: it appears on the runway's approach instead.
		cc.log.printf("%-6s schedule: real arrival: %v: on the approach", f.Callsign, err)
		f.Stage = ""
		return s.spawnWith(f, nil, "")
	}
	a := s.airlines[f.Airline]
	models := traffic.ModelsForFlight(cc.modelList(), f.Airline, a.Name, f.Type, f.Callsign, 6)
	if len(models) == 0 {
		return fmt.Errorf("no model of a %s", f.Type)
	}
	model := models[(f.Attempts-1)%len(models)]
	e := &enrouteAC{f: f, model: model, arrive: &planned{route: pts, name: name, expect: expect, runway: rwy}}
	kts := math.Max(f.Observed.GroundKts, realMinKts)
	join := pts[0]
	joinAlt := math.Min(alt, 10000)
	if join.AltMax > 0 {
		joinAlt = math.Min(alt, join.AltMax/0.3048)
	}
	if join.AltMin > 0 {
		joinAlt = math.Max(joinAlt, join.AltMin/0.3048)
	}
	route := []traffic.RoutePoint{{Position: pos, AltFt: alt, Kts: traffic.EnrouteSpeedKts(alt, kts)}}
	route = append(route, slowDownBefore(route[0], join.Position, joinAlt, kts)...)
	route = append(route, traffic.RoutePoint{Position: join.Position, AltFt: joinAlt, Kts: traffic.EnrouteSpeedKts(joinAlt, kts)})
	return s.spawnEnrouteOn(f, e, model, route, "its track to "+join.Ident)
}

// arrivalJoin is where an aircraft at pos joins the arrival to runway:
// of every STAR of the runway, its route from the point that makes the
// shortest way in (to the point, then along the rest), up to the initial
// approach fix.
func (s *scheduler) arrivalJoin(g *airport.Graph, runway string, pos airport.LatLon) ([]airport.NavPoint, string, string, error) {
	cc := s.cc
	if cc.procedures == nil {
		return nil, "", "", errors.New("procedures not available")
	}
	p, ok := cc.procedures(g.Layout.ICAO)
	if !ok {
		return nil, "", "", fmt.Errorf("procedures of %s not loaded (yet)", g.Layout.ICAO)
	}
	var best []airport.NavPoint
	bestName, bestExpect, bestNM := "", "", math.Inf(1)
	for _, star := range p.STARsFor(runway) {
		pts, name, expect, err := cc.procedureFor(g, SpawnRequest{Kind: "arrival", Runway: runway, Procedure: true, ProcName: star.Name})
		if err != nil || len(pts) < 2 {
			continue
		}
		// The way in from each point: there, then along the rest.
		rest := make([]float64, len(pts))
		for i := len(pts) - 2; i >= 0; i-- {
			a, b := pts[i].Position, pts[i+1].Position
			rest[i] = rest[i+1] + calc.HaversineNM(a.Lat, a.Lon, b.Lat, b.Lon)
		}
		for k := range pts {
			if k > 0 && (pts[k-1].IAF || pts[k-1].FAF) {
				break // the STAR, up to the initial approach fix: no straight-in from afar
			}
			if pts[k].Ident == "" {
				continue
			}
			if nm := calc.HaversineNM(pos.Lat, pos.Lon, pts[k].Position.Lat, pts[k].Position.Lon) + rest[k]; nm < bestNM {
				best, bestName, bestExpect, bestNM = pts[k:], name, expect, nm
			}
		}
	}
	if best == nil {
		return nil, "", "", fmt.Errorf("no STAR for runway %s", runway)
	}
	return best, bestName, bestExpect, nil
}

// orUnknown is an airport for the log: "unknown" when not known (a real
// aircraft's other end, #841).
func orUnknown(icao string) string {
	if icao == "" {
		return "unknown"
	}
	return icao
}
