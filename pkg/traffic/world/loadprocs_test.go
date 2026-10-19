package world

import (
	"os"
	"path/filepath"
	"testing"
)

// A procedures dump is served offline as the simulator's would be.
func TestLoadProceduresFile(t *testing.T) {
	w := New(Options{DataDir: t.TempDir()})
	icao, err := w.LoadProceduresFile("../../airport/testdata/LKPR-procedures.json")
	if err != nil || icao != "LKPR" {
		t.Fatalf("%q %v, want LKPR", icao, err)
	}
	w.st.mu.Lock()
	p, ok := w.st.procedures["LKPR"]
	w.st.mu.Unlock()
	if !ok || len(p.Arrivals) == 0 || len(p.Departures) == 0 || len(p.Approaches) == 0 {
		t.Fatalf("LKPR procedures not served: %d STARs, %d SIDs, %d approaches", len(p.Arrivals), len(p.Departures), len(p.Approaches))
	}
	bad := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(bad, []byte(`{"departures":[]}`), 0o644)
	if _, err := w.LoadProceduresFile(bad); err == nil {
		t.Error("a dump without its ICAO loaded")
	}
}
