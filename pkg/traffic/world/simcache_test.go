package world

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// The worldwide airport list is read from disk while fresh, a week; then
// asked of the simulator again.
func TestSimCacheAirportsFresh(t *testing.T) {
	dir := t.TempDir()
	c := newSimCache(dir, "")
	if _, ok := c.airports(); ok {
		t.Fatal("empty cache answered")
	}
	c.writeAirports([]traffic.AirportRef{{ICAO: "lkpr", DistanceNM: 3}, {ICAO: "LOWW"}})
	refs, ok := newSimCache(dir, "").airports()
	if !ok || len(refs) != 2 || refs["LKPR"].DistanceNM != 0 {
		t.Fatalf("kept list %v %+v", ok, refs)
	}
	late := newSimCache(dir, "")
	late.now = func() time.Time { return time.Now().Add(simCacheMaxAge + time.Hour) }
	if _, ok := late.airports(); ok {
		t.Error("a list over a week old answered")
	}
}

// enumMsg is the last (or not) message of a title enumeration.
func enumMsg(req, entry, outOf uint32) engine.Message {
	buf := make([]byte, unsafe.Sizeof(types.SIMCONNECT_RECV_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST{}))
	e := (*types.SIMCONNECT_RECV_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST)(unsafe.Pointer(&buf[0]))
	e.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST)
	e.DwSize = types.DWORD(len(buf))
	e.DwRequestID, e.DwEntryNumber, e.DwOutOf = types.DWORD(req), types.DWORD(entry), types.DWORD(outOf)
	return engine.Message{SIMCONNECT_RECV: (*types.SIMCONNECT_RECV)(unsafe.Pointer(&buf[0]))}
}

// The aircraft and ground vehicle titles, kept once both enumerations are
// in, are handed to the next connection instead of enumerating; another
// set of packages (the stamp) enumerates again, and what was kept for the
// old set is not read after the new one is written.
func TestSimCacheModelsByStamp(t *testing.T) {
	dir := t.TempDir()
	cc := &controlCenter{models: map[string]bool{"A320 :: Air France": true, "B738": true}}
	cc.addGroundTitles([]string{"FSDT_FuelTruck_BP", "ASO_Boarding_Stairs"})
	c := newSimCache(dir, "stamp-1")
	if _, ok := c.models(); ok {
		t.Fatal("empty cache answered")
	}
	c.noteEnumeration(enumMsg(reqModels, 0, 2))
	c.noteEnumeration(enumMsg(reqModels, 1, 2))
	c.flushModels(cc)
	if _, err := os.Stat(filepath.Join(dir, simCacheDir, simCacheModels)); err == nil {
		t.Fatal("kept before the ground vehicles were in")
	}
	c.noteEnumeration(enumMsg(reqGroundVehicles, 0, 1))
	c.flushModels(cc)

	m, ok := newSimCache(dir, "stamp-1").models()
	if !ok || len(m.Aircraft) != 2 || len(m.Ground) != 2 {
		t.Fatalf("kept titles %v %+v", ok, m)
	}
	next := &controlCenter{models: map[string]bool{}}
	next.feedModels(m)
	if !next.models["A320 :: Air France"] || len(next.fuelTitles.trucks) != 1 || len(next.fuelTitles.stairsStock) != 1 {
		t.Errorf("fed: models %v, fuel %+v", next.models, next.fuelTitles)
	}

	other := newSimCache(dir, "stamp-2")
	if _, ok := other.models(); ok {
		t.Fatal("titles kept for other packages answered")
	}
	if _, ok := newSimCache(dir, "").models(); ok {
		t.Fatal("titles answered with the packages unknown")
	}
	// stamp-2 writes a layout: the old stamp's titles still not read.
	time.Sleep(20 * time.Millisecond)
	other.writeLayout(airport.RawAirport{ICAO: "LKPR"})
	if _, ok := newSimCache(dir, "stamp-2").models(); ok {
		t.Error("titles written for stamp-1 read under stamp-2")
	}
}

// An airport's layout kept as its raw records, built again as loaded.
func TestSimCacheLayout(t *testing.T) {
	b, err := os.ReadFile("../../airport/testdata/LKPR.json")
	if err != nil {
		t.Fatal(err)
	}
	var raw airport.RawAirport
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	want, err := airport.BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	newSimCache(dir, "s").writeLayout(raw)
	l, _, ok := newSimCache(dir, "s").layout("lkpr")
	if !ok || l.ICAO != "LKPR" || len(l.Parking) != len(want.Parking) || len(l.TaxiPaths) != len(want.TaxiPaths) {
		t.Fatalf("kept layout %v", ok)
	}
	if _, _, ok := newSimCache(dir, "other").layout("LKPR"); ok {
		t.Error("layout kept for other packages answered")
	}
}

// Procedures kept by airport; none at all (a failed load) not kept.
func TestSimCacheProcedures(t *testing.T) {
	dir := t.TempDir()
	c := newSimCache(dir, "s")
	c.writeProcedures(airport.Procedures{ICAO: "LKPR"})
	if _, ok := c.procedures("LKPR"); ok {
		t.Fatal("empty procedures kept")
	}
	c.writeProcedures(airport.Procedures{ICAO: "LKPR", Approaches: make([]airport.Approach, 1)})
	if p, ok := newSimCache(dir, "s").procedures("lkpr"); !ok || len(p.Approaches) != 1 {
		t.Fatalf("procedures %v %+v", ok, p)
	}
	if _, err := os.Stat(filepath.Join(dir, simCacheDir, "LKPR", dumpProcedures)); err != nil {
		t.Errorf("not where LoadAirportDir reads it: %v", err)
	}
}

// ?refresh: the airport's next load from the simulator, not the disk.
func TestSimCacheRefreshBypass(t *testing.T) {
	s := &state{cache: airport.NewCache(), fetched: map[string]time.Time{}, waiters: map[string][]chan error{}}
	if _, err := s.load(context.Background(), "LKPR", true, nil); err == nil {
		t.Fatal("loaded without a simulator")
	}
	if !s.takeRefresh("lkpr") {
		t.Fatal("refresh not noted for the loop")
	}
	if s.takeRefresh("LKPR") {
		t.Error("refresh noted twice")
	}
	s.load(context.Background(), "LKPR", false, nil)
	if s.takeRefresh("LKPR") {
		t.Error("a plain load noted as refresh")
	}
}
