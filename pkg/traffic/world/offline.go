package world

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Offline data: what the simulator gives the World about an airport —
// its layout (the raw facility records), SIDs, STARs and approaches,
// airways and ILS — written per airport as it arrives
// (Options.AirportDumpDir, <dir>/<ICAO>/<file>) and served back without a
// simulator (LoadAirportDir): Vector, the ATC game, runs the World in
// WebAssembly on a loopback client.
const (
	dumpLayout     = "layout.json"     // airport.RawAirport, as LoadFile reads
	dumpProcedures = "procedures.json" // airport.Procedures, as LoadProceduresFile reads
	dumpAirways    = "airways.json"    // nav.AirwayGraph (SaveJSON)
	dumpILS        = "ils.json"        // the ILS by runway end
)

// dumpPath is where icao's file name goes, its directory made; "" when
// not dumping.
func (st *state) dumpPath(icao, name string) string {
	if st.airportDump == "" || icao == "" {
		return ""
	}
	dir := filepath.Join(st.airportDump, strings.ToUpper(icao))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(stdout, "❌ Dump %s: %v\n", dir, err)
		return ""
	}
	return filepath.Join(dir, name)
}

// dump writes v as icao's file name (Options.AirportDumpDir).
func (st *state) dump(icao, name string, v any) {
	path := st.dumpPath(icao, name)
	if path == "" {
		return
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err == nil {
		err = os.WriteFile(path, b, 0o644)
	}
	if err != nil {
		fmt.Fprintf(stdout, "❌ Dump %s: %v\n", path, err)
		return
	}
	fmt.Fprintf(stdout, "💾 Wrote %s\n", path)
}

// dumpAirways writes icao's airways (Options.AirportDumpDir).
func (st *state) dumpAirways(icao string, g *nav.AirwayGraph) {
	path := st.dumpPath(icao, dumpAirways)
	if path == "" || g == nil {
		return
	}
	if err := g.SaveJSON(path); err != nil {
		fmt.Fprintf(stdout, "❌ Dump %s: %v\n", path, err)
		return
	}
	fmt.Fprintf(stdout, "💾 Wrote %s\n", path)
}

// dumpILS writes the ILS of every airport known so far (Options.
// AirportDumpDir): a lookup answers for one runway end at a time.
func (st *state) dumpILS() {
	if st.airportDump == "" {
		return
	}
	st.core.ils.Lock()
	all := map[string]map[string]ilsInfo{}
	for icao, m := range st.core.ils.byICAO {
		c := map[string]ilsInfo{}
		for k, v := range m {
			c[k] = v
		}
		all[icao] = c
	}
	st.core.ils.Unlock()
	for icao, m := range all {
		st.dump(icao, dumpILS, m)
	}
}

// LoadAirportDir serves an airport from its offline data directory (as
// Options.AirportDumpDir writes it: layout.json, procedures.json,
// airways.json, ils.json; each optional) instead of the simulator, and
// its weather.json (a WeatherFile, written by hand: a representative day)
// as the World's weather. It returns the airport's ICAO.
func (w *World) LoadAirportDir(dir string) (string, error) {
	icao := strings.ToUpper(filepath.Base(filepath.Clean(dir)))
	loaded := false
	if p := filepath.Join(dir, dumpLayout); exists(p) {
		got, err := w.LoadFile(p)
		if err != nil {
			return "", err
		}
		icao, loaded = got, true
	}
	if p := filepath.Join(dir, dumpProcedures); exists(p) {
		got, err := w.LoadProceduresFile(p)
		if err != nil {
			return "", err
		}
		icao, loaded = got, true
	}
	if p := filepath.Join(dir, dumpAirways); exists(p) {
		g, err := nav.LoadAirwayGraph(p)
		if err != nil {
			return "", fmt.Errorf("%s: %w", p, err)
		}
		w.st.addAirways(icao, g)
		loaded = true
	}
	if p := filepath.Join(dir, dumpILS); exists(p) {
		b, err := os.ReadFile(p)
		if err != nil {
			return "", err
		}
		var m map[string]ilsInfo
		if err := json.Unmarshal(b, &m); err != nil {
			return "", fmt.Errorf("%s: %w", p, err)
		}
		w.st.core.ils.Lock()
		if w.st.core.ils.byICAO == nil {
			w.st.core.ils.byICAO = map[string]map[string]ilsInfo{}
		}
		w.st.core.ils.byICAO[icao] = m
		w.st.core.ils.Unlock()
		loaded = true
	}
	if p := filepath.Join(dir, dumpWeather); exists(p) {
		if err := w.loadWeatherFile(p); err != nil {
			return "", err
		}
		loaded = true
	}
	if !loaded {
		return "", errors.New(dir + ": no offline data")
	}
	return icao, nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// dumpAirportList is the worldwide airport list's file, at the top of
// Options.AirportDumpDir: every airport the simulator knows (where
// diversions go and en-route traffic comes from), for LoadAirportsFile.
const dumpAirportList = "airports.json"

// dumpAirports writes the worldwide airport list (Options.AirportDumpDir),
// in ICAO order, without the distances from the user aircraft.
func (st *state) dumpAirports(list []traffic.AirportRef) {
	if st.airportDump == "" || len(list) == 0 {
		return
	}
	out := make([]traffic.AirportRef, len(list))
	copy(out, list)
	for i := range out {
		out[i].ICAO, out[i].DistanceNM = strings.ToUpper(out[i].ICAO), 0
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ICAO < out[j].ICAO })
	if err := os.MkdirAll(st.airportDump, 0o755); err != nil {
		fmt.Fprintf(stdout, "❌ Dump %s: %v\n", st.airportDump, err)
		return
	}
	path := filepath.Join(st.airportDump, dumpAirportList)
	b, err := json.Marshal(out) // compact: some 85,000 airports
	if err == nil {
		err = os.WriteFile(path, b, 0o644)
	}
	if err != nil {
		fmt.Fprintf(stdout, "❌ Dump %s: %v\n", path, err)
		return
	}
	fmt.Fprintf(stdout, "💾 Wrote %s (%d airports)\n", path, len(out))
}

// LoadAirportsFile serves the worldwide airport list from a dump (as
// Options.AirportDumpDir writes airports.json) instead of the simulator:
// where diversions go and en-route traffic comes from. It returns how many
// airports it holds.
func (w *World) LoadAirportsFile(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var list []traffic.AirportRef
	if err := json.Unmarshal(b, &list); err != nil {
		return 0, fmt.Errorf("%s: %w", path, err)
	}
	refs := make(map[string]traffic.AirportRef, len(list))
	for _, a := range list {
		if a.ICAO != "" {
			refs[strings.ToUpper(a.ICAO)] = a
		}
	}
	if len(refs) == 0 {
		return 0, errors.New(path + ": no airports")
	}
	w.st.mu.Lock()
	w.st.airportRefs = refs
	w.st.mu.Unlock()
	return len(refs), nil
}

// offlineBubbleNM: with no simulator (SimlessAI), the airports this close
// to the user aircraft, from the offline list (LoadAirportsFile), stand in
// for the simulator's facility list (its reality bubble).
const offlineBubbleNM = 100.0

// offlineAirportsNear are the offline airports within offlineBubbleNM of
// the user aircraft among aircraft, nearest first; nil with no list or no
// user aircraft.
func (st *state) offlineAirportsNear(aircraft []traffic.TrackedAircraft) []traffic.AirportRef {
	var user *traffic.TrackedAircraft
	for i := range aircraft {
		if aircraft[i].User {
			user = &aircraft[i]
		}
	}
	if user == nil {
		return nil
	}
	st.mu.Lock()
	refs := st.airportRefs
	st.mu.Unlock()
	var out []traffic.AirportRef
	for _, a := range refs {
		d := calc.HaversineNM(user.Position.Lat, user.Position.Lon, a.Position.Lat, a.Position.Lon)
		if d <= offlineBubbleNM {
			a.DistanceNM = d
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DistanceNM < out[j].DistanceNM })
	return out
}
