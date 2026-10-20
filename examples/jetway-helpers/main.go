//go:build windows
// +build windows

// Command jetway-helpers writes the invisible jetway helper package (#572,
// pkg/jetway) from the installed traffic models: one helper per ICAO type
// with a base model on disk (its door, interactive_point.0), into the
// simulator's Community folder (or -out). MSFS reads it at its next start.
// Usage: jetway-helpers [-types A320,B738,...] [-out dir] [-codes DLH,AFR].
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/addons"
	"github.com/mrlm-net/simconnect/pkg/jetway"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

func main() {
	only := flag.String("types", "", "comma-separated ICAO types (default: every type with a base model)")
	out := flag.String("out", "", "where the package goes (default: the Community folder)")
	codes := flag.String("codes", "", "airline codes for each type's \"ALL\" helper (none: only the one without codes)")
	span := flag.Float64("span", 0, "every helper's wing span, feet (0: the model's)")
	sideSteps := flag.Int("side-steps", 6, "each type's door variants each side (0: none)")
	sideStep := flag.Float64("side-step", 0.1, "door variant step, meters")
	ahead := flag.String("ahead", "rules", "door variants along the fuselage, meters ahead (negative back), comma-separated; \"rules\": the stand rules' offsets (traffic.StandRules)")
	flag.Parse()
	var aheadM []float64
	if *ahead == "rules" {
		seen := map[float64]bool{}
		for _, r := range traffic.StandRules() {
			if r.ForwardM != 0 && !seen[r.ForwardM] {
				seen[r.ForwardM] = true
				aheadM = append(aheadM, r.ForwardM)
			}
		}
	} else {
		for _, s := range strings.Split(*ahead, ",") {
			var m float64
			if _, err := fmt.Sscan(strings.TrimSpace(s), &m); err == nil && m != 0 {
				aheadM = append(aheadM, m)
			}
		}
	}
	ins, err := addons.Find()
	if err != nil {
		fail(err)
	}
	pkgs, err := addons.Scan(ins.Packages)
	if err != nil {
		fail(err)
	}
	var dirs []string
	for _, p := range pkgs {
		if strings.HasPrefix(p.Folder, "mycrew-") {
			continue // never from our own helpers
		}
		d := filepath.Join(p.Path, "SimObjects", "Airplanes")
		if fi, err := os.Stat(d); err == nil && fi.IsDir() {
			dirs = append(dirs, d)
		}
	}
	doors := addons.DoorsByType(dirs...)
	want := map[string]bool{}
	for _, t := range strings.Split(*only, ",") {
		if t = strings.ToUpper(strings.TrimSpace(t)); t != "" {
			want[t] = true
		}
	}
	var models []jetway.HelperModel
	for t, d := range doors {
		if len(want) > 0 && !want[t] {
			continue
		}
		for _, dir := range dirs {
			if p := filepath.Join(dir, d.Folder); exists(filepath.Join(p, "flight_model.cfg")) {
				models = append(models, jetway.HelperModel{Type: t, Dir: p})
				break
			}
		}
	}
	sort.Slice(models, func(i, j int) bool { return models[i].Type < models[j].Type })
	if len(models) == 0 {
		fail(fmt.Errorf("no installed model with a door for %q", *only))
	}
	dest := *out
	if dest == "" {
		dest = filepath.Join(ins.Packages, "Community")
	}
	var cs []string
	for _, c := range strings.Split(*codes, ",") {
		if c = strings.ToUpper(strings.TrimSpace(c)); c != "" {
			cs = append(cs, c)
		}
	}
	root, err := jetway.WritePackage(dest, models, jetway.PackageOptions{Creator: "MyCrew", Codes: cs, WingSpanFt: *span, SideSteps: *sideSteps, SideStepM: *sideStep, AheadM: aheadM})
	if err != nil {
		fail(err)
	}
	fmt.Printf("%d jetway helpers written to %s; restart MSFS to load them\n", len(models), root)
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
