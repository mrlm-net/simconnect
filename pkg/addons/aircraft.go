//go:build windows
// +build windows

package addons

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// AircraftPackage finds the package that holds an aircraft, from the path
// the sim reports for AircraftLoaded, e.g.
// SimObjects\Airplanes\FNX_32X\presets\fnx\FNX_319_CFM_WF_HD\config\aircraft.CFG.
//
// Packages with a manifest are matched on their layout.json file list
// (read once and cached while the file is unchanged). Streamed packages
// have none; they match when their content holds the aircraft's folder
// (content\SimObjects\Airplanes\<folder>). Community packages are tried
// first, as they override the others. ok is false when nothing matches.
func AircraftPackage(pkgs []Package, aircraftPath string) (Package, bool) {
	rel := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(aircraftPath), `\`, "/"))
	if rel == "" {
		return Package{}, false
	}
	needle := []byte(`"` + rel + `"`)
	for _, src := range []Source{Community, Community2024, Official, Streamed} {
		for _, p := range pkgs {
			if p.Source != src {
				continue
			}
			if src == Streamed {
				if streamedHolds(p, rel) {
					return p, true
				}
				continue
			}
			layout := filepath.Join(p.Path, "layout.json")
			if strings.HasPrefix(rel, simObjectsPrefix) {
				if layoutSimObjects(layout)[rel] {
					return p, true
				}
				continue
			}
			b, err := os.ReadFile(layout)
			if err == nil && bytes.Contains(bytes.ToLower(b), needle) {
				return p, true
			}
		}
	}
	return Package{}, false
}

const simObjectsPrefix = "simobjects/"

// layoutCache holds each layout.json's SimObjects paths (lower case),
// with the file's size and time they were read at (E13): an aircraft
// loaded again does not read every package's file list again.
var layoutCache = struct {
	sync.Mutex
	m map[string]layoutEntry
}{m: map[string]layoutEntry{}}

type layoutEntry struct {
	size  int64
	mod   time.Time
	paths map[string]bool
}

// layoutSimObjects are the paths under SimObjects/ that the layout.json at
// file lists, lower case; nil when it cannot be read.
func layoutSimObjects(file string) map[string]bool {
	st, err := os.Stat(file)
	if err != nil {
		return nil
	}
	layoutCache.Lock()
	e, ok := layoutCache.m[file]
	layoutCache.Unlock()
	if ok && e.size == st.Size() && e.mod.Equal(st.ModTime()) {
		return e.paths
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var layout struct {
		Content []struct {
			Path string `json:"path"`
		} `json:"content"`
	}
	if json.Unmarshal(b, &layout) != nil {
		return nil
	}
	paths := map[string]bool{}
	for _, c := range layout.Content {
		p := strings.ToLower(strings.ReplaceAll(c.Path, `\`, "/"))
		if strings.HasPrefix(p, simObjectsPrefix) {
			paths[p] = true
		}
	}
	layoutCache.Lock()
	layoutCache.m[file] = layoutEntry{size: st.Size(), mod: st.ModTime(), paths: paths}
	layoutCache.Unlock()
	return paths
}

// streamedHolds: the package content has the aircraft's SimObjects folder
// (SimObjects/<category>/<folder>).
func streamedHolds(p Package, rel string) bool {
	parts := strings.Split(rel, "/")
	if len(parts) < 3 || parts[0] != "simobjects" {
		return false
	}
	dir := filepath.Join(p.Path, "content", "SimObjects")
	for _, want := range parts[1:3] {
		ents, err := os.ReadDir(dir)
		if err != nil {
			return false
		}
		found := ""
		for _, e := range ents {
			if e.IsDir() && strings.EqualFold(e.Name(), want) {
				found = e.Name()
				break
			}
		}
		if found == "" {
			return false
		}
		dir = filepath.Join(dir, found)
	}
	return true
}
