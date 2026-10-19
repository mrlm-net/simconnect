package world

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/nav"
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
// airways.json, ils.json; each optional) instead of the simulator. It
// returns the airport's ICAO.
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
	if !loaded {
		return "", errors.New(dir + ": no offline data")
	}
	return icao, nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
