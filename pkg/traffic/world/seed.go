package world

import (
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// "Real start": the traffic of one real-world snapshot placed when a
// flight loads, each aircraft a flight of ours from then on (our ATC,
// sequencing, give-way), with its real callsign and airline; no feed
// follows it, and the schedule fills in afterwards. Marked Seeded (not
// Real) on the map.
//
//   - Parked at the airport: on the stand it is at (else one for its
//     type), departing after a turnaround to its real destination, else
//     on a SID out of the area.
//   - Airborne inbound: joins the arrival sequence from where it is.
//   - Airborne outbound or overflying: across the area along its route,
//     else straight on.
//   - Moving on the ground (taxiing): left out for now, and said why.
//
// Nearest the airport first, up to the room the schedule has left.

// seedTurnaround: a parked aircraft departs this long after the snapshot,
// spread by its place in the list so they do not all push at once.
const (
	seedTurnaround = 40 * time.Minute
	seedSpread     = 2 * time.Minute
)

// SeedResult is what Seed made of a snapshot: one ObserveResult per
// aircraft ("added" or "ignored" with why).
type SeedResult struct {
	Placed  int             `json:"placed"`
	Skipped int             `json:"skipped"`
	Results []ObserveResult `json:"results"`
}

// Seed places the aircraft of one real-world snapshot taken at at (zero:
// now) as flights of ours ("real start").
func (w *World) Seed(obs []traffic.Observed, at time.Time) (SeedResult, error) {
	var res SeedResult
	body, err := w.Do(http.MethodPost, "/api/seed", map[string]any{"observed": obs, "at": at})
	if err != nil {
		return res, err
	}
	err = json.Unmarshal(body, &res)
	return res, err
}

func registerSeed(mux *http.ServeMux, st *state) {
	mux.HandleFunc("POST /api/seed", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Observed []traffic.Observed `json:"observed"`
			At       time.Time          `json:"at"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		st.mu.Lock()
		s := st.schedule
		st.mu.Unlock()
		if s == nil {
			http.Error(w, "simulator not connected", http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, s.seedSnapshot(req.Observed, req.At))
	})
}

// seedSnapshot is World.Seed on the scheduler.
func (s *scheduler) seedSnapshot(obs []traffic.Observed, at time.Time) SeedResult {
	if at.IsZero() {
		at = time.Now()
	}
	type cand struct {
		o     traffic.Observed
		icao  string
		field airport.LatLon
		nm    float64
	}
	var cands []cand
	var res SeedResult
	skip := func(o traffic.Observed, why string) {
		res.Skipped++
		res.Results = append(res.Results, ObserveResult{ID: o.ID, Callsign: realCallsign(o), Status: "ignored", Reason: why})
		s.cc.log.printf("%-6s real start: not placed: %s", realCallsign(o), why)
	}
	for _, o := range obs {
		icao, field, ok := s.realAirport(o)
		if !ok {
			skip(o, "no managed airport")
			continue
		}
		cands = append(cands, cand{o, icao, field, calc.HaversineNM(o.Lat, o.Lon, field.Lat, field.Lon)})
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].nm < cands[j].nm })
	room := math.MaxInt
	if max := s.mgr.Options().MaxAircraft; max > 0 {
		room = max - s.mgr.Active()
	}
	now := s.cc.clock.Now()
	for i, c := range cands {
		o := c.o
		if res.Placed >= room {
			skip(o, "no room left (the aircraft budget)")
			continue
		}
		if o.SeenAt.IsZero() {
			o.SeenAt = at
		}
		if len(o.Route) == 0 && strings.TrimSpace(o.RouteText) != "" {
			o.Route = s.routeFromText(realCallsign(o), o)
		}
		if l, ok := s.st.cache.Layout(c.icao); !ok || len(l.Runways) == 0 {
			skip(o, "no runways known at "+c.icao)
			continue
		}
		kind, why := traffic.ClassifyObserved(o, c.field)
		if kind == "" {
			skip(o, why)
			continue
		}
		if o.Type == "" {
			skip(o, "no type")
			continue
		}
		sight := o.Sighting()
		sight.Seeded = true
		cs := realCallsign(o)
		f := traffic.Flight{Callsign: cs, Airline: realAirline(cs), Type: strings.ToUpper(o.Type), Observed: &sight}
		switch kind {
		case traffic.ObservedParked:
			f.Origin, f.Destination = c.icao, strings.ToUpper(o.Destination)
			f.STD = now.Add(seedTurnaround + time.Duration(i)*seedSpread)
			s.mgr.Add([]traffic.Flight{f})
		case traffic.ObservedArrival:
			pos, _ := sight.At(time.Now())
			kts := math.Max(o.GroundKts, realMinKts)
			d := calc.HaversineNM(pos.Lat, pos.Lon, c.field.Lat, c.field.Lon)
			f.Origin, f.Destination = strings.ToUpper(o.Origin), c.icao
			f.STA, f.DistanceNM = now.Add(time.Duration(d/kts*float64(time.Hour))), d
			s.mgr.Add([]traffic.Flight{f})
		case traffic.ObservedOverflight:
			var centre airport.LatLon
			ok := false
			if s.cc.world != nil {
				centre, ok = s.cc.world.Centre()
			}
			pos, alt := sight.At(time.Now())
			if !ok || calc.HaversineNM(pos.Lat, pos.Lon, centre.Lat, centre.Lon) > overflightRadiusNM {
				skip(o, "overflight outside the area")
				continue
			}
			path := realOverflightPath(centre, pos, alt, o.TrackDeg, o.Route)
			if len(path) == 0 {
				skip(o, "its way does not cross the area")
				continue
			}
			kts := math.Max(o.GroundKts, realMinKts)
			f.Origin, f.Destination = strings.ToUpper(o.Origin), strings.ToUpper(o.Destination)
			f.Enter, f.Exit = now, now.Add(time.Duration(pathNM(pos, path)/kts*float64(time.Hour)))
			s.mgr.AddOverflight(f)
		default: // a departure moving on the ground: taxiing
			skip(o, "taxiing aircraft are not placed (yet)")
			continue
		}
		res.Placed++
		res.Results = append(res.Results, ObserveResult{ID: o.ID, Airport: c.icao, Kind: kind, Callsign: cs, Status: "added"})
		s.cc.log.printf("%-6s real start: %s at %s (%s)", cs, kind, c.icao, strings.ToUpper(o.Type))
	}
	s.cc.log.printf("real start: %d placed, %d not", res.Placed, res.Skipped)
	return res
}
