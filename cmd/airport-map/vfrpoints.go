//go:build windows
// +build windows

package main

import (
	"encoding/json"
	"hash/fnv"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// VFR reporting points (#566): per airport, as the user sets them on the
// map, kept in the user cache folder (vfrpoints.json). The simulator's
// navigation data has none near LKPR (only unnamed VP* idents).
//
// GET /api/vfrpoints?icao=LKPR gives the airport's points; POST with a
// list of traffic.ReportingPoint replaces them ([] removes them all).

var vfrPointSets = struct {
	sync.Mutex
	loaded bool
	m      map[string][]traffic.ReportingPoint // ICAO -> points
}{m: map[string][]traffic.ReportingPoint{}}

func vfrPointsFile() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "mrlm-simconnect", "airport-map", "vfrpoints.json")
}

// vfrPoints are the reporting points set for icao.
func vfrPoints(icao string) []traffic.ReportingPoint {
	vfrPointSets.Lock()
	defer vfrPointSets.Unlock()
	if !vfrPointSets.loaded {
		vfrPointSets.loaded = true
		if b, err := os.ReadFile(vfrPointsFile()); err == nil {
			_ = json.Unmarshal(b, &vfrPointSets.m)
		}
	}
	return append([]traffic.ReportingPoint(nil), vfrPointSets.m[strings.ToUpper(icao)]...)
}

func setVFRPoints(icao string, pts []traffic.ReportingPoint) error {
	vfrPoints(icao) // loaded
	vfrPointSets.Lock()
	defer vfrPointSets.Unlock()
	icao = strings.ToUpper(icao)
	if len(pts) == 0 {
		delete(vfrPointSets.m, icao)
	} else {
		vfrPointSets.m[icao] = pts
	}
	file := vfrPointsFile()
	if file == "" {
		return nil
	}
	b, err := json.MarshalIndent(vfrPointSets.m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	return os.WriteFile(file, b, 0o644)
}

// vfrPointFor is the reporting point a VFR flight uses at icao: the one
// named (any case), else one by its call sign (the same each time); nil
// when the airport has none.
func vfrPointFor(icao, name, callsign string) *traffic.ReportingPoint {
	pts := vfrPoints(icao)
	if len(pts) == 0 {
		return nil
	}
	for i := range pts {
		if name != "" && strings.EqualFold(pts[i].Name, name) {
			return &pts[i]
		}
	}
	h := fnv.New32a()
	h.Write([]byte(callsign))
	return &pts[int(h.Sum32()%uint32(len(pts)))]
}

func registerVFRPoints(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/vfrpoints", func(w http.ResponseWriter, r *http.Request) {
		icao := r.URL.Query().Get("icao")
		if icao == "" {
			http.Error(w, "icao is needed", http.StatusBadRequest)
			return
		}
		pts := vfrPoints(icao)
		if pts == nil {
			pts = []traffic.ReportingPoint{}
		}
		writeJSON(w, pts)
	})
	mux.HandleFunc("POST /api/vfrpoints", func(w http.ResponseWriter, r *http.Request) {
		icao := r.URL.Query().Get("icao")
		if icao == "" {
			http.Error(w, "icao is needed", http.StatusBadRequest)
			return
		}
		var pts []traffic.ReportingPoint
		if err := json.NewDecoder(r.Body).Decode(&pts); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for i := range pts {
			pts[i].Name = strings.ToUpper(strings.TrimSpace(pts[i].Name))
			if pts[i].Name == "" || pts[i].Position.Lat == 0 && pts[i].Position.Lon == 0 {
				http.Error(w, "each point needs a name and a position", http.StatusBadRequest)
				return
			}
		}
		if err := setVFRPoints(icao, pts); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
