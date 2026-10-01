//go:build windows
// +build windows

package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
)

// The tower the user placed (#tower view): the facility data puts some
// towers elsewhere (LKPR) and gives no cab height, so the user can set both
// on the map, per airport, kept between runs in the user cache folder.

// towerSite is a tower as the user set it: a position (0, 0: not set) and
// the cab's height above the airfield (0: not set).
type towerSite struct {
	Lat  float64 `json:"lat,omitempty"`
	Lon  float64 `json:"lon,omitempty"`
	CabM float64 `json:"cabM,omitempty"`
}

var towerSites = struct {
	sync.Mutex
	loaded bool
	m      map[string]towerSite
}{m: map[string]towerSite{}}

func towersFile() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "mrlm-simconnect", "airport-map", "towers.json")
}

// userTowerLocked is the tower the user set at icao, if any. towerSites held.
func userTowerLocked(icao string) (towerSite, bool) {
	if !towerSites.loaded {
		towerSites.loaded = true
		if b, err := os.ReadFile(towersFile()); err == nil {
			_ = json.Unmarshal(b, &towerSites.m)
		}
	}
	t, ok := towerSites.m[icao]
	return t, ok
}

func userTower(icao string) (towerSite, bool) {
	towerSites.Lock()
	defer towerSites.Unlock()
	return userTowerLocked(icao)
}

func setUserTower(icao string, t towerSite, reset bool) error {
	towerSites.Lock()
	defer towerSites.Unlock()
	userTowerLocked(icao)
	if reset {
		delete(towerSites.m, icao)
	} else {
		towerSites.m[icao] = t
	}
	file := towersFile()
	if file == "" {
		return nil
	}
	b, err := json.MarshalIndent(towerSites.m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	// Written whole or not at all: a temporary file, then renamed.
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, file)
}

// towerReachM: a tower is placed at most this far from the airport.
const towerReachM = 5000

// towerHeightM: the cab's height above the airfield when nobody set one.
const towerHeightM = 100

// towerView is where the tower stands at an airport and where that comes
// from: "yours" (set on the map), "known" (airport.Limits), "simulator"
// (the facility's TOWER_*), "airport" (none: over the reference point).
type towerView struct {
	ICAO   string  `json:"icao"`
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
	CabM   float64 `json:"cabM"`
	Source string  `json:"source"`
}

// towerAt is the tower of g's airport: the user's, else the limits', else
// the facility's, else over the reference point; the cab towerHeightM high
// unless set.
func (cc *controlCenter) towerAt(g *airport.Graph) towerView {
	l := g.Layout
	v := towerView{ICAO: l.ICAO, Lat: l.Latitude, Lon: l.Longitude, CabM: towerHeightM, Source: "airport"}
	if l.HasTower {
		v.Lat, v.Lon, v.Source = l.Tower.Lat, l.Tower.Lon, "simulator"
	}
	if t := cc.limitsOf(g).Tower; t != nil {
		v.Lat, v.Lon, v.Source = t.Position.Lat, t.Position.Lon, "known"
		if t.CabM > 0 {
			v.CabM = t.CabM
		}
	}
	if t, ok := userTower(l.ICAO); ok {
		if t.Lat != 0 || t.Lon != 0 {
			v.Lat, v.Lon, v.Source = t.Lat, t.Lon, "yours"
		}
		if t.CabM > 0 {
			v.CabM = t.CabM
		}
	}
	return v
}

// registerTowers serves the tower:
//
//	GET  /api/tower?icao=XXXX — where it stands and why (towerView)
//	POST /api/tower — {icao, lat, lon, cabM}: set it (a field left 0 keeps
//	     what is there); {icao, reset: true}: forget the user's
func registerTowers(mux *http.ServeMux, st *state) {
	graph := func(w http.ResponseWriter, icao string) *airport.Graph {
		g, err := st.cache.Graph(strings.ToUpper(icao))
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return nil
		}
		return g
	}
	control := func() *controlCenter {
		st.mu.Lock()
		defer st.mu.Unlock()
		return st.control
	}
	mux.HandleFunc("GET /api/tower", func(w http.ResponseWriter, r *http.Request) {
		g := graph(w, r.URL.Query().Get("icao"))
		cc := control()
		if g == nil {
			return
		}
		if cc == nil {
			cc = &controlCenter{}
		}
		writeJSON(w, cc.towerAt(g))
	})
	mux.HandleFunc("POST /api/tower", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ICAO  string  `json:"icao"`
			Lat   float64 `json:"lat"`
			Lon   float64 `json:"lon"`
			CabM  float64 `json:"cabM"`
			Reset bool    `json:"reset"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		g := graph(w, req.ICAO)
		if g == nil {
			return
		}
		if req.CabM < 0 || req.CabM > 400 {
			http.Error(w, "cabM: 0 to 400 m", http.StatusUnprocessableEntity)
			return
		}
		// On the airport: a tower placed within towerReachM of it.
		if req.Lat != 0 || req.Lon != 0 {
			l := g.Layout
			if req.Lat < -90 || req.Lat > 90 || req.Lon < -180 || req.Lon > 180 || calc.HaversineMeters(req.Lat, req.Lon, l.Latitude, l.Longitude) > towerReachM {
				http.Error(w, "the tower must stand within 5 km of the airport", http.StatusUnprocessableEntity)
				return
			}
		}
		icao := g.Layout.ICAO
		t, _ := userTower(icao)
		if req.Lat != 0 || req.Lon != 0 {
			t.Lat, t.Lon = req.Lat, req.Lon
		}
		if req.CabM > 0 {
			t.CabM = req.CabM
		}
		if err := setUserTower(icao, t, req.Reset); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		cc := control()
		if cc == nil {
			cc = &controlCenter{}
		}
		writeJSON(w, cc.towerAt(g))
	})
}
