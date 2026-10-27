package world

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/airspace"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// airspaceStore opens the offline airspaces (a simconnect-data checkout)
// as the World is meant to (#1257): $SIMCONNECT_DATA, else beside this
// repository's checkout. The test skips without it.
func airspaceStore(t *testing.T) *airspace.Store {
	t.Helper()
	d := os.Getenv("SIMCONNECT_DATA")
	if d == "" {
		d = filepath.Join("..", "..", "..", "..", "simconnect-data")
	}
	dir := filepath.Join(d, "airspaces")
	if _, err := os.Stat(filepath.Join(dir, "index.json")); err != nil {
		t.Skipf("no offline airspaces at %s (SIMCONNECT_DATA)", dir)
	}
	s, err := airspace.Open(os.DirFS(dir), airspace.StorePreferNewerSet(), airspace.StoreSkipBroken())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// TestClassAtRealAirspaces (#1257): with Options.Airspaces the class
// changes at Ruzyne CTR's and Prague TMA II's real limits, not at 10 NM /
// 5,000 ft above the field; without it, as before.
func TestClassAtRealAirspaces(t *testing.T) {
	s := airspaceStore(t)
	g, err := airport.BuildGraph(lkprLayout(t))
	if err != nil {
		t.Fatal(err)
	}
	ccOf := func(o Options) *controlCenter {
		o.DataDir = t.TempDir()
		w := New(o)
		return &controlCenter{core: w.st.core, graph: func(string) (*airport.Graph, error) { return g, nil }}
	}
	real, zone := ccOf(Options{Airspaces: s}), ccOf(Options{})
	lkpr := airport.LatLon{Lat: 50.1008, Lon: 14.26}
	at := func(brg, nm, altFt, aglFt float64) traffic.TrackedAircraft {
		lat, lon := calc.DisplaceByHeading(lkpr.Lat, lkpr.Lon, brg, nm*1852)
		return traffic.TrackedAircraft{Observation: traffic.Observation{Tail: "OKTST", Position: airport.LatLon{Lat: lat, Lon: lon}, AltFt: altFt, AGLFt: aglFt}}
	}
	for _, c := range []struct {
		name      string
		a         traffic.TrackedAircraft
		real, old traffic.AirspaceClass
	}{
		// Overhead at 3,000 ft: Ruzyne CTR (D, GND–3,500 ft MSL).
		{"overhead 3000 ft", at(0, 0, 3000, 1750), traffic.ClassD, traffic.ClassD},
		// Overhead at 4,000 ft: Prague TMA II (C from 3,500 ft), still
		// within 5,000 ft of the field.
		{"overhead 4000 ft", at(0, 0, 4000, 2750), traffic.ClassC, traffic.ClassD},
		// 8 NM south, outside the CTR (it is 5–12 NM round the field): at
		// 1,900 ft, 700 ft above the ground, under the Prague CTA's E (from
		// 1,000 ft AGL): no class record, G; at 2,300 ft in the CTA (E); at
		// 3,000 ft in Prague TMA I (C from 2,500 ft).
		{"8 NM south 1900 ft", at(180, 8, 1900, 700), traffic.ClassG, traffic.ClassD},
		{"8 NM south 2300 ft", at(180, 8, 2300, 1100), traffic.ClassE, traffic.ClassD},
		{"8 NM south 3000 ft", at(180, 8, 3000, 1800), traffic.ClassC, traffic.ClassD},
		// 11.5 NM east-northeast, along the runway 06/24 extension: still in
		// the CTR, past the old 10 NM zone.
		{"11.5 NM ENE 2000 ft", at(calc.BearingDegrees(lkpr.Lat, lkpr.Lon, 50.1545, 14.562), 11.5, 2000, 800), traffic.ClassD, traffic.ClassE},
	} {
		if got := real.classAt(c.a, []string{"LKPR"}); got != c.real {
			t.Errorf("%s: class %s from the airspaces, want %s", c.name, got, c.real)
		}
		if got := zone.classAt(c.a, []string{"LKPR"}); got != c.old {
			t.Errorf("%s: class %s without them, want %s (as before)", c.name, got, c.old)
		}
	}
	// No height in the air: the field's elevation (1,195 ft in the
	// layout) for the CTA's AGL floor: 2,300 ft is in its E as above, not
	// G under a floor of 1,000 ft over sea level.
	a := at(180, 8, 2300, 0)
	if got := real.groundFt(a, []string{"LKPR"}); got < 1150 || got > 1300 {
		t.Errorf("ground without a height %.0f ft, want LKPR's elevation", got)
	}
	if got := real.classAt(a, []string{"LKPR"}); got != traffic.ClassE {
		t.Errorf("no height, 2300 ft: class %s, want E", got)
	}
}
