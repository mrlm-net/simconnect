//go:build windows
// +build windows

package nav

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/manager"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Both clients satisfy the loader, including exception matching.
var (
	_ FacilityClient = engine.Client(nil)
	_ sendIDClient   = engine.Client(nil)
	_ FacilityClient = manager.Manager(nil)
	_ sendIDClient   = manager.Manager(nil)
)

type recordWriter struct{ bytes.Buffer }

func (w *recordWriter) i32(v int32)  { binary.Write(&w.Buffer, binary.LittleEndian, v) }
func (w *recordWriter) u32(v uint32) { binary.Write(&w.Buffer, binary.LittleEndian, v) }
func (w *recordWriter) f32(v float32) {
	binary.Write(&w.Buffer, binary.LittleEndian, math.Float32bits(v))
}
func (w *recordWriter) f64(v float64) {
	binary.Write(&w.Buffer, binary.LittleEndian, math.Float64bits(v))
}
func (w *recordWriter) str(s string, n int) {
	b := make([]byte, n)
	copy(b, s)
	w.Write(b)
}

// TestDecodeRecords decodes records laid out as MSFS 2024 sent them for
// TABEM (checked with examples/spike-airways -raw 2) and the VOZ VOR.
func TestDecodeRecords(t *testing.T) {
	st := &navState{fix: Fix{Ident: "TABEM", Region: "LK", Kind: KindWaypoint}}

	var w recordWriter
	w.f64(49.24331)
	w.f64(15.22147)
	w.i32(int32(WaypointRNAV))
	w.str("TABEM", 8)
	w.str("LK", 8)
	w.i32(2) // N_ROUTES
	w.i32(0) // IS_TERMINAL_WPT
	if w.Len() != 44 {
		t.Fatalf("waypoint record %d bytes", w.Len())
	}
	st.add(types.SIMCONNECT_FACILITY_DATA_WAYPOINT, w.Bytes())

	w.Reset()
	w.str("M725", 32)
	w.i32(int32(AirwayVictor))
	w.str("OKF", 8)
	w.str("LK", 8)
	w.i32('V')
	w.f64(48.9692)
	w.f64(15.5456)
	w.f32(1219.2)
	w.str("VOZ", 8)
	w.str("LK", 8)
	w.i32('V')
	w.f64(49.5323)
	w.f64(14.8747)
	w.f32(0)
	if w.Len() != 116 {
		t.Fatalf("route record %d bytes, MSFS sends 116", w.Len())
	}
	st.add(types.SIMCONNECT_FACILITY_DATA_ROUTE, w.Bytes())

	w.Reset()
	w.str("Z401", 32)
	w.i32(int32(AirwayVictor))
	w.str("", 8) // no next: end of the airway
	w.str("", 8)
	w.i32(0)
	w.f64(0)
	w.f64(0)
	w.f32(0)
	w.str("USUPA", 8)
	w.str("LK", 8)
	w.i32('W')
	w.f64(49.5205)
	w.f64(15.1288)
	w.f32(0)
	st.add(types.SIMCONNECT_FACILITY_DATA_ROUTE, w.Bytes())

	if st.fix.Position.Lat != 49.24331 || st.fix.Type != WaypointRNAV || !st.wpt {
		t.Errorf("fix = %+v", st.fix)
	}
	if len(st.routes) != 2 {
		t.Fatalf("routes = %+v", st.routes)
	}
	m := st.routes[0]
	if m.Airway != "M725" || m.Next == nil || m.Next.Key != Key("OKF", "LK", KindVOR) || m.Next.MinAltM != 1219.2 ||
		m.Prev == nil || m.Prev.Key.Ident != "VOZ" {
		t.Errorf("M725 = %+v next %+v prev %+v", m, m.Next, m.Prev)
	}
	if z := st.routes[1]; z.Next != nil || z.Prev == nil || z.Prev.Key.Kind != KindWaypoint {
		t.Errorf("Z401 = %+v", z)
	}

	vor := &navState{fix: Fix{Ident: "VOZ", Region: "LK", Kind: KindVOR}}
	w.Reset()
	w.f64(49.53233)
	w.f64(14.87466)
	w.u32(116950000)
	w.str("VOZICE", 64)
	vor.add(types.SIMCONNECT_FACILITY_DATA_VOR, w.Bytes())
	if vor.fix.Freq != 116.95 || vor.fix.Name != "VOZICE" || vor.fix.Position.Lat != 49.53233 {
		t.Errorf("VOR = %+v", vor.fix)
	}
	ndb := &navState{fix: Fix{Ident: "MIQ", Region: "ED", Kind: KindNDB}}
	w.Reset()
	w.f64(48.57)
	w.f64(11.5975)
	w.u32(426000)
	w.str("MIKE", 64)
	ndb.add(types.SIMCONNECT_FACILITY_DATA_NDB, w.Bytes())
	if ndb.fix.Freq != 426 || ndb.fix.Name != "MIKE" {
		t.Errorf("NDB = %+v", ndb.fix)
	}
}
