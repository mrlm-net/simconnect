//go:build windows
// +build windows

package world

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// De-icing pads picked on the map (#323): MSFS facility data has none, so
// the user picks them from the taxi points. They are kept per airport in a
// JSON file and used by de-icing at a pad and by automatic de-icing.

type padStore struct {
	mu   sync.Mutex
	file string
	pads map[string][]airport.DeicingPad
}

// loadPadStore reads file (missing: no pads yet).
func loadPadStore(file string) *padStore {
	s := &padStore{file: file, pads: map[string][]airport.DeicingPad{}}
	if b, err := os.ReadFile(file); err == nil {
		if err := json.Unmarshal(b, &s.pads); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  %s: %v\n", file, err)
		}
	}
	return s
}

// forAirport are the pads of icao: picked ones, else the known ones.
func (s *padStore) forAirport(l *airport.Layout) []airport.DeicingPad {
	s.mu.Lock()
	picked, ok := s.pads[l.ICAO]
	s.mu.Unlock()
	if ok {
		return picked
	}
	return airport.LimitsFor(l, nil).DeicingPads
}

func (s *padStore) set(icao string, pads []airport.DeicingPad) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pads[icao] = pads
	b, err := json.MarshalIndent(s.pads, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.file), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.file, b, 0o644)
}

// registerDeicing serves GET /api/deicing?icao=X (the pads) and PUT
// /api/deicing?icao=X with the pads to keep; each is snapped onto its
// nearest taxi point and must be within traffic.DeicingPadReachMeters of
// one.
func registerDeicing(mux *http.ServeMux, st *state) {
	mux.HandleFunc("GET /api/deicing", func(w http.ResponseWriter, r *http.Request) {
		l, ok := st.cache.Layout(strings.ToUpper(r.URL.Query().Get("icao")))
		if !ok {
			http.Error(w, "airport not loaded", http.StatusNotFound)
			return
		}
		writeJSON(w, st.pads.forAirport(l))
	})
	mux.HandleFunc("PUT /api/deicing", func(w http.ResponseWriter, r *http.Request) {
		g, err := st.cache.Graph(strings.ToUpper(r.URL.Query().Get("icao")))
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		var pads []airport.DeicingPad
		if err := json.NewDecoder(r.Body).Decode(&pads); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for i := range pads {
			n, ok := g.NearestNode(pads[i].Position, traffic.DeicingPadReachMeters)
			if !ok {
				http.Error(w, fmt.Sprintf("%s is not on a taxiway", pads[i].Name), http.StatusUnprocessableEntity)
				return
			}
			pads[i].Position = g.Nodes[n].Position
		}
		if pads == nil {
			pads = []airport.DeicingPad{}
		}
		if err := st.pads.set(g.Layout.ICAO, pads); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tlog.printf("%s de-icing pads: %d", g.Layout.ICAO, len(pads))
		writeJSON(w, pads)
	})
}
