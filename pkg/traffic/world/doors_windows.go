//go:build windows

package world

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/addons"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

var loadDoorsOnce sync.Once

// loadInstalledDoors reads the installed models' doors from the
// simulator's packages, once, in the background (a few seconds over a
// large traffic package).
func loadInstalledDoors() {
	loadDoorsOnce.Do(func() {
		go func() {
			ins, err := addons.Find()
			if err != nil {
				return
			}
			pkgs, err := addons.Scan(ins.Packages)
			if err != nil {
				return
			}
			var dirs []string
			for _, p := range pkgs {
				d := filepath.Join(p.Path, "SimObjects", "Airplanes")
				if fi, err := os.Stat(d); err == nil && fi.IsDir() {
					dirs = append(dirs, d)
				}
			}
			// The titles' packages: who provides each model.
			src := map[string]traffic.ModelSource{}
			for t, s := range addons.TitleSources(pkgs) {
				src[t] = traffic.ModelSource{Package: s.Package, Creator: s.Creator, Flyable: s.Flyable}
			}
			traffic.SetModelSources(src)
			tlog.printf("model sources: %d titles on disk", len(src))
			byTitle, byType := addons.DoorsByTitle(dirs...), addons.DoorsByType(dirs...)
			if len(byTitle) > 0 || len(byType) > 0 {
				setDoors(byTitle, byType)
				tlog.printf("model doors: %d titles, %d types from the installed models", len(byTitle), len(byType))
			}
		}()
	})
}
