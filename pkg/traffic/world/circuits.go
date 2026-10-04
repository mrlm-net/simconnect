package world

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// VFR circuits as the user sets them (#567): per airport, per runway end,
// kept in the user cache folder (circuits.json). What is not set takes
// traffic's defaults (left-hand, 1000 ft, the downwind from the turns).
//
// GET /api/circuits?icao=LKPR gives the airport's settings and, per runway
// end, the circuit a C172 would fly; POST /api/circuits?icao=LKPR&runway=24
// with a traffic.CircuitConfig sets one ({} resets it).

type circuitStore struct {
	sync.Mutex
	loaded bool
	m      map[string]map[string]traffic.CircuitConfig // ICAO -> runway end -> config
}

func circuitsFile() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "mrlm-simconnect", "airport-map", "circuits.json")
}

func (k *core) circuitsLoadLocked() {
	if k.circuits.loaded {
		return
	}
	k.circuits.loaded = true
	if b, err := os.ReadFile(circuitsFile()); err == nil {
		_ = json.Unmarshal(b, &k.circuits.m)
	}
}

// circuitConfig is the circuit set for runway end rwy at icao (zero: the
// defaults).
func (k *core) circuitConfig(icao, rwy string) traffic.CircuitConfig {
	k.circuits.Lock()
	defer k.circuits.Unlock()
	k.circuitsLoadLocked()
	return k.circuits.m[strings.ToUpper(icao)][rwy]
}

func (k *core) setCircuitConfig(icao, rwy string, c traffic.CircuitConfig) error {
	k.circuits.Lock()
	defer k.circuits.Unlock()
	k.circuitsLoadLocked()
	icao = strings.ToUpper(icao)
	if c == (traffic.CircuitConfig{}) {
		delete(k.circuits.m[icao], rwy)
		if len(k.circuits.m[icao]) == 0 {
			delete(k.circuits.m, icao)
		}
	} else {
		if k.circuits.m[icao] == nil {
			k.circuits.m[icao] = map[string]traffic.CircuitConfig{}
		}
		k.circuits.m[icao][rwy] = c
	}
	file := circuitsFile()
	if file == "" {
		return nil
	}
	b, err := json.MarshalIndent(k.circuits.m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	return os.WriteFile(file, b, 0o644)
}

func registerCircuits(mux *http.ServeMux, k *core, cc func() *controlCenter) {
	mux.HandleFunc("GET /api/circuits", func(w http.ResponseWriter, r *http.Request) {
		c := cc()
		if c == nil {
			http.Error(w, "not connected to the simulator", http.StatusServiceUnavailable)
			return
		}
		icao := strings.ToUpper(r.URL.Query().Get("icao"))
		g, err := c.graph(icao)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		p := traffic.ProfileFor("C172")
		type end struct {
			Config  traffic.CircuitConfig `json:"config"`
			Circuit traffic.Circuit       `json:"circuit"`
		}
		out := map[string]end{}
		for _, rw := range g.Layout.Runways {
			for _, e := range []string{rw.Primary.Name, rw.Secondary.Name} {
				cfg := k.circuitConfig(icao, e)
				if circ, err := traffic.NewCircuit(g.Layout, e, cfg, p); err == nil {
					out[e] = end{Config: cfg, Circuit: circ}
				}
			}
		}
		writeJSON(w, out)
	})
	mux.HandleFunc("POST /api/circuits", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		icao, rwy := q.Get("icao"), q.Get("runway")
		if icao == "" || rwy == "" {
			http.Error(w, "icao and runway are needed", http.StatusBadRequest)
			return
		}
		var cfg traffic.CircuitConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if cfg.Side != "" && cfg.Side != traffic.CircuitLeft && cfg.Side != traffic.CircuitRight {
			http.Error(w, "side is left or right", http.StatusBadRequest)
			return
		}
		if cfg.HeightFt < 0 || cfg.HeightFt > 3000 || cfg.DownwindNM < 0 || cfg.DownwindNM > 5 || cfg.UpwindNM < 0 || cfg.UpwindNM > 5 || cfg.BaseNM < 0 || cfg.BaseNM > 5 {
			http.Error(w, "height 0–3000 ft, distances 0–5 NM", http.StatusBadRequest)
			return
		}
		if err := k.setCircuitConfig(icao, rwy, cfg); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
