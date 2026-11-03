//go:build windows
// +build windows

package addons

import (
	"os"
	"path/filepath"
	"strings"
)

// TitleSource is where an installed SimObject title comes from: its
// package folder and creator, and whether it is a flyable aircraft (its
// [FLTSIM.N] isUserSelectable = 1: an add-on the user flies, not AI
// traffic).
type TitleSource struct {
	Package string
	Creator string
	Flyable bool
}

// TitleSources maps every SimObject title of the packages given (lower
// case) to its package: their SimObjects/*/*/aircraft.cfg or sim.cfg
// [FLTSIM.N] titles, and the presets of MSFS 2024 modular aircraft
// (SimObjects/*/*/presets/*/*/config/aircraft.cfg: the Fenix A32X). Packed
// packages (the stock ones) have none on disk.
func TitleSources(pkgs []Package) map[string]TitleSource {
	out := map[string]TitleSource{}
	for _, p := range pkgs {
		kinds, err := os.ReadDir(filepath.Join(p.Path, "SimObjects"))
		if err != nil {
			continue
		}
		for _, k := range kinds {
			if !k.IsDir() {
				continue
			}
			objs, err := os.ReadDir(filepath.Join(p.Path, "SimObjects", k.Name()))
			if err != nil {
				continue
			}
			for _, o := range objs {
				if !o.IsDir() {
					continue
				}
				dir := filepath.Join(p.Path, "SimObjects", k.Name(), o.Name())
				cfgs := []string{filepath.Join(dir, "aircraft.cfg"), filepath.Join(dir, "sim.cfg")}
				presets, _ := filepath.Glob(filepath.Join(dir, "presets", "*", "*", "config", "aircraft.cfg"))
				for _, cfg := range append(cfgs, presets...) {
					for _, f := range fltsims(cfg) {
						out[strings.ToLower(f.title)] = TitleSource{Package: p.Folder, Creator: p.Creator, Flyable: f.selectable}
					}
				}
			}
		}
	}
	return out
}
