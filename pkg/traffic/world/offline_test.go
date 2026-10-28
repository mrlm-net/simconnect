package world

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// What the simulator gives about an airport is dumped per airport and
// served back from there without one.
func TestAirportDumpRoundTrip(t *testing.T) {
	dir := t.TempDir()
	w := New(Options{DataDir: t.TempDir(), AirportDumpDir: dir})
	var raw airport.RawAirport
	b, _ := os.ReadFile("../../airport/testdata/LKPR.json")
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	var p airport.Procedures
	b, _ = os.ReadFile("../../airport/testdata/LKPR-procedures.json")
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	g, err := nav.LoadAirwayGraph("../../nav/testdata/LKPR-airways.json")
	if err != nil {
		t.Fatal(err)
	}
	f := localFeed{st: w.st}
	w.st.dump("LKPR", dumpLayout, raw)
	f.Procedures(p)
	f.Airways("LKPR", g)
	w.st.core.ils.Lock()
	w.st.core.ils.byICAO = map[string]map[string]ilsInfo{"LKPR": {"24": {Runway: "24", Ident: "OKL", MHz: 109.5}}}
	w.st.core.ils.Unlock()
	w.st.dumpILS()
	for _, name := range []string{dumpLayout, dumpProcedures, dumpAirways, dumpILS} {
		if !exists(filepath.Join(dir, "LKPR", name)) {
			t.Fatalf("%s not dumped", name)
		}
	}

	off := New(Options{DataDir: t.TempDir()})
	icao, err := off.LoadAirportDir(filepath.Join(dir, "LKPR"))
	if err != nil || icao != "LKPR" {
		t.Fatalf("%q %v", icao, err)
	}
	if l, ok := off.st.cache.Layout("LKPR"); !ok || len(l.Runways) != 2 {
		t.Error("layout not served")
	}
	off.st.mu.Lock()
	gotP, okP := off.st.procedures["LKPR"]
	nFix := 0
	if off.st.airways != nil {
		nFix = len(off.st.airways.Fixes)
	}
	off.st.mu.Unlock()
	if !okP || len(gotP.Arrivals) != len(p.Arrivals) {
		t.Error("procedures not served")
	}
	if nFix == 0 {
		t.Error("airways not served")
	}
	if ils := off.st.core.ilsOf("LKPR"); len(ils) != 1 || ils[0].Ident != "OKL" {
		t.Errorf("ILS %+v", ils)
	}
	if _, err := off.LoadAirportDir(t.TempDir()); err == nil {
		t.Error("an empty directory loaded")
	}
}

// The worldwide airport list is dumped once, sorted, and served back.
func TestAirportListRoundTrip(t *testing.T) {
	dir := t.TempDir()
	w := New(Options{DataDir: t.TempDir(), AirportDumpDir: dir})
	w.st.dumpAirports([]traffic.AirportRef{
		{ICAO: "lkpr", Region: "LK", Position: airport.LatLon{Lat: 50.1, Lon: 14.26}, DistanceNM: 3},
		{ICAO: "EGLL", Region: "EG", Position: airport.LatLon{Lat: 51.47, Lon: -0.45}},
	})
	off := New(Options{DataDir: t.TempDir()})
	n, err := off.LoadAirportsFile(filepath.Join(dir, dumpAirportList))
	if err != nil || n != 2 {
		t.Fatalf("%d %v, want 2 airports", n, err)
	}
	off.st.mu.Lock()
	a, ok := off.st.airportRefs["LKPR"]
	off.st.mu.Unlock()
	if !ok || a.Region != "LK" || a.DistanceNM != 0 {
		t.Errorf("LKPR %+v %v", a, ok)
	}
}

// TestAirportListKinds: LoadAirportsFile with kinds holds only those
// kinds of airport (an entry without one counts as every kind), and
// AirportDataDir finds each kind's data folder as simconnect-data lays it
// out.
func TestAirportListKinds(t *testing.T) {
	path := filepath.Join(t.TempDir(), dumpAirportList)
	list := `[{"icao":"LKPR","kind":"towered","selfAnnounce":true,"groundLayout":true},{"icao":"00R","kind":"towered"},{"icao":"LKTB"},{"icao":"KVUO","kind":"ifr"},{"icao":"00NK","kind":"water"},{"icao":"00AK","kind":"airfield"}]`
	if err := os.WriteFile(path, []byte(list), 0o644); err != nil {
		t.Fatal(err)
	}
	w := New(Options{DataDir: t.TempDir()})
	if n, err := w.LoadAirportsFile(path, traffic.AirportKindTowered, traffic.AirportKindIFR); err != nil || n != 4 {
		t.Fatalf("towered and ifr: %d %v, want 4 (LKPR, 00R, LKTB, KVUO)", n, err)
	}
	w.st.mu.Lock()
	_, water := w.st.airportRefs["00NK"]
	lkpr := w.st.airportRefs["LKPR"]
	w.st.mu.Unlock()
	if water {
		t.Error("a water airport held loading towered and ifr only")
	}
	if !lkpr.SelfAnnounce || lkpr.GroundLayout == nil || !*lkpr.GroundLayout {
		t.Errorf("LKPR's flags not read: %+v", lkpr)
	}
	if n, err := w.LoadAirportsFile(path); err != nil || n != 6 {
		t.Fatalf("all: %d %v, want 6", n, err)
	}
	for _, c := range []struct {
		a    traffic.AirportRef
		want string
	}{
		{traffic.AirportRef{ICAO: "lkpr", Kind: "towered"}, filepath.Join("d", "LKPR")},
		{traffic.AirportRef{ICAO: "LKTB"}, filepath.Join("d", "LKTB")},
		{traffic.AirportRef{ICAO: "00R", Kind: "towered"}, filepath.Join("d", "fields", "towered", "00R")},
		{traffic.AirportRef{ICAO: "KVUO", Kind: "ifr"}, filepath.Join("d", "fields", "ifr", "KVUO")},
		{traffic.AirportRef{ICAO: "CEN5", Kind: "towered"}, filepath.Join("d", "fields", "towered", "CEN5")},
		{traffic.AirportRef{ICAO: "00NK", Kind: "water"}, filepath.Join("d", "fields", "water", "00NK")},
	} {
		if got := AirportDataDir("d", c.a); got != c.want {
			t.Errorf("%+v: %s, want %s", c.a, got, c.want)
		}
	}
}

// TestOfflineWeather: an airport's weather.json is the World's weather
// with no simulator (Vector), kept over the simulator's; SetWeather(nil)
// gives it back.
func TestOfflineWeather(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "LKPR")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, dumpWeather), []byte(`{"windDirTrue":240,"windKts":12,"gustKts":20,"visibilityM":8000,"tempC":14,"qnhHPa":1009}`), 0o644); err != nil {
		t.Fatal(err)
	}
	w := New(Options{DataDir: t.TempDir()})
	if _, err := w.LoadAirportDir(dir); err != nil {
		t.Fatal(err)
	}
	got := func() *nav.Weather {
		w.st.mu.Lock()
		defer w.st.mu.Unlock()
		return w.st.weather
	}
	wx := got()
	if wx == nil || wx.WindDirTrue != 240 || wx.WindKts != 12 || wx.GustKts != 20 || wx.QNHhPa != 1009 {
		t.Fatalf("weather %+v, want 240/12G20, QNH 1009", wx)
	}
	f := localFeed{st: w.st}
	f.Weather(nav.StaticWeather(60, 5, 9999, 10, 5, 1020)) // the simulator's
	if got().WindDirTrue != 240 {
		t.Error("the simulator's weather over the one set")
	}
	w.SetWeather(nil)
	f.Weather(nav.StaticWeather(60, 5, 9999, 10, 5, 1020))
	if got().WindDirTrue != 60 {
		t.Error("given back: the simulator's weather not taken")
	}
}
