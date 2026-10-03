//go:build windows
// +build windows

package addons

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

// AircraftPackage finds the package that holds an aircraft, from the path
// the sim reports for AircraftLoaded, e.g.
// SimObjects\Airplanes\FNX_32X\presets\fnx\FNX_319_CFM_WF_HD\config\aircraft.CFG.
//
// Packages with a manifest are matched on their layout.json file list.
// Streamed packages have none; they match when their content holds the
// aircraft's folder (content\SimObjects\Airplanes\<folder>). Community
// packages are tried first, as they override the others. ok is false when
// nothing matches.
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
			b, err := os.ReadFile(filepath.Join(p.Path, "layout.json"))
			if err == nil && bytes.Contains(bytes.ToLower(b), needle) {
				return p, true
			}
		}
	}
	return Package{}, false
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
