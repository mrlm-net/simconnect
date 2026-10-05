package world

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// An airport's ATC stations (#722): the scenery's frequencies by default
// (traffic.DefaultStations), an airport's own in the local settings
// (DataDir/stations.json, the GSX way: what is there wins, per position).
// Several stations of a position work sectors (Ground North and South,
// Apron); one controller may work several frequencies, one voice.

// stationStore keeps the stations set per airport.
type stationStore struct {
	mu       sync.Mutex
	file     string
	airports map[string][]traffic.Station
}

// loadStationStore reads file (missing: none set).
func loadStationStore(file string) *stationStore {
	s := &stationStore{file: file, airports: map[string][]traffic.Station{}}
	if b, err := os.ReadFile(file); err == nil {
		if err := json.Unmarshal(b, &s.airports); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  %s: %v\n", file, err)
		}
	}
	return s
}

// forAirport is icao's stations as set ("" none).
func (s *stationStore) forAirport(icao string) []traffic.Station {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.airports[strings.ToUpper(icao)]
}

// set sets icao's stations (none: the defaults again) and saves.
func (s *stationStore) set(icao string, st []traffic.Station) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	icao = strings.ToUpper(icao)
	if len(st) == 0 {
		delete(s.airports, icao)
	} else {
		s.airports[icao] = st
	}
	b, err := json.MarshalIndent(s.airports, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.file, b, 0o644)
}

// stations are icao's stations: the defaults with the local ones.
func (cc *controlCenter) stations(icao string) []traffic.Station {
	var l *airport.Layout
	if cc.graph != nil {
		if g, err := cc.graph(icao); err == nil {
			l = g.Layout
		}
	}
	var mine []traffic.Station
	if cc.localStations != nil {
		mine = cc.localStations(icao)
	}
	return traffic.StationsWith(icao, traffic.DefaultStations(l), mine)
}

// station is the station of position pos working it now (#722): by the
// taxiways it is on, its runway, where it is; name as said and frequency.
func (it *controlled) station(pos traffic.Position) (string, string) {
	w := traffic.Where{Runway: it.view.Runway}
	if p := it.view.Position; p.Lat != 0 || p.Lon != 0 {
		w.At = &p
		if it.graph != nil {
			if n, ok := it.graph.NearestNode(p, 60); ok {
				for _, e := range it.graph.Adj[n] {
					if e.Name != "" {
						w.Taxiways = append(w.Taxiways, e.Name)
					}
				}
			}
		}
	}
	if s, ok := traffic.PickStation(it.cc.stations(it.ICAO), pos, w); ok {
		return s.Name, s.Freq
	}
	return it.cc.stationOf(it.ICAO, pos)
}

// registerStations serves the stations (#722).
func registerStations(mux *http.ServeMux, st *state) {
	// GET /api/stations?icao=LKPR — the airport's stations as worked:
	// position, name, frequency, controller (one person on several
	// frequencies shares it), sector; "local": set in the local settings.
	mux.HandleFunc("GET /api/stations", func(w http.ResponseWriter, r *http.Request) {
		icao := strings.ToUpper(r.URL.Query().Get("icao"))
		st.mu.Lock()
		cc := st.control
		st.mu.Unlock()
		out := struct {
			ICAO     string            `json:"icao"`
			Stations []traffic.Station `json:"stations"`
			Local    bool              `json:"local"`
		}{ICAO: icao, Stations: []traffic.Station{}, Local: len(st.stations.forAirport(icao)) > 0}
		if cc != nil {
			out.Stations = append(out.Stations, cc.stations(icao)...)
		}
		writeJSON(w, out)
	})
	// PUT /api/stations?icao=LKPR [stations] — the airport's own stations
	// (each position given replaces its default); DELETE: the defaults.
	mux.HandleFunc("PUT /api/stations", func(w http.ResponseWriter, r *http.Request) {
		var s []traffic.Station
		if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for _, x := range s {
			if x.Position == "" || x.Freq == "" {
				http.Error(w, "each station needs a position and a freq", http.StatusBadRequest)
				return
			}
		}
		if err := st.stations.set(r.URL.Query().Get("icao"), s); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("DELETE /api/stations", func(w http.ResponseWriter, r *http.Request) {
		if err := st.stations.set(r.URL.Query().Get("icao"), nil); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
