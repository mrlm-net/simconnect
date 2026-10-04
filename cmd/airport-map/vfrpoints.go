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

type vfrPointStore struct {
	sync.Mutex
	loaded bool
	m      map[string][]traffic.ReportingPoint // ICAO -> points
}

func vfrPointsFile() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "mrlm-simconnect", "airport-map", "vfrpoints.json")
}

// vfrPoints are the reporting points set for icao.
func (k *core) vfrPoints(icao string) []traffic.ReportingPoint {
	k.vfrSets.Lock()
	defer k.vfrSets.Unlock()
	if !k.vfrSets.loaded {
		k.vfrSets.loaded = true
		if b, err := os.ReadFile(vfrPointsFile()); err == nil {
			_ = json.Unmarshal(b, &k.vfrSets.m)
		}
	}
	return append([]traffic.ReportingPoint(nil), k.vfrSets.m[strings.ToUpper(icao)]...)
}

func (k *core) setVFRPoints(icao string, pts []traffic.ReportingPoint) error {
	k.vfrPoints(icao) // loaded
	k.vfrSets.Lock()
	defer k.vfrSets.Unlock()
	icao = strings.ToUpper(icao)
	if len(pts) == 0 {
		delete(k.vfrSets.m, icao)
	} else {
		k.vfrSets.m[icao] = pts
	}
	file := vfrPointsFile()
	if file == "" {
		return nil
	}
	b, err := json.MarshalIndent(k.vfrSets.m, "", "  ")
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
func (k *core) vfrPointFor(icao, name, callsign string) *traffic.ReportingPoint {
	pts := k.vfrPoints(icao)
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

func registerVFRPoints(mux *http.ServeMux, k *core) {
	mux.HandleFunc("GET /api/vfrpoints", func(w http.ResponseWriter, r *http.Request) {
		icao := r.URL.Query().Get("icao")
		if icao == "" {
			http.Error(w, "icao is needed", http.StatusBadRequest)
			return
		}
		pts := k.vfrPoints(icao)
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
		if err := k.setVFRPoints(icao, pts); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
