package airport

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"testing"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

func loadLKPRProcedures(t *testing.T) Procedures {
	t.Helper()
	b, err := os.ReadFile("testdata/LKPR-procedures.json")
	if err != nil {
		t.Fatal(err)
	}
	var p Procedures
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestLKPRProcedures: LKPR as captured in MSFS 2024 — SIDs and STARs with
// runway transitions, an ILS to 24 whose final ends at its missed approach
// point near the threshold of 24.
func TestLKPRProcedures(t *testing.T) {
	p := loadLKPRProcedures(t)
	if len(p.Departures) < 20 || len(p.Arrivals) < 10 || len(p.Approaches) < 8 {
		t.Fatalf("%d SIDs, %d STARs, %d approaches", len(p.Departures), len(p.Arrivals), len(p.Approaches))
	}
	l := loadLKPR(t)
	_, end, _ := l.RunwayEnd("24")
	for _, a := range p.Approaches {
		if a.Name != "ILS 24" {
			continue
		}
		last := a.Final[len(a.Final)-1]
		if !last.MAP || calc.HaversineMeters(last.Position.Lat, last.Position.Lon, end.Threshold.Lat, end.Threshold.Lon) > 3000 {
			t.Errorf("ILS 24 ends at %s %+v, MAP %v", last.Fix, last.Position, last.MAP)
		}
		return
	}
	t.Error("no ILS 24")
}

// TestProcedurePath: a SID from 24 starts at the departure end and passes
// through its fixes; open legs extend along their course.
func TestProcedurePath(t *testing.T) {
	p := loadLKPRProcedures(t)
	l := loadLKPR(t)
	rwy, end, _ := l.RunwayEnd("24")
	der := rwy.Primary.Threshold // the far end of 24
	if end.Name == rwy.Primary.Name {
		der = rwy.Secondary.Threshold
	}
	for _, d := range p.Departures {
		for _, tr := range d.RunwayTransitions {
			if tr.Runway != "24" {
				continue
			}
			pts := ProcedurePath(append(tr.Legs, d.Legs...), der, l.Altitude, p.MagVar, 0)
			if len(pts) < 2 || pts[0] != der {
				t.Fatalf("%s: %d points", d.Name, len(pts))
			}
			for _, leg := range tr.Legs {
				if !leg.HasFix() {
					continue
				}
				found := false
				for _, q := range pts {
					found = found || q == leg.Position
				}
				if !found {
					t.Errorf("%s misses %s", d.Name, leg.Fix)
				}
			}
			return
		}
	}
	t.Fatal("no SID from 24")
}

// leg112 encodes an APPROACH_LEG record as the simulator packs it.
func leg112(t *testing.T, typ int32, fix string, lat, lon float64) []byte {
	var b bytes.Buffer
	w := func(v any) { binary.Write(&b, binary.LittleEndian, v) }
	str := func(s string) { var a [8]byte; copy(a[:], s); w(a) }
	w(typ)
	str(fix)
	str("LK")
	w(int32('W'))
	w(lat)
	w(lon)
	w(0.0)               // FIX_ALTITUDE
	w(int32(0))          // FLY_OVER
	w(int32(2))          // TURN_DIRECTION right
	w(float32(242))      // COURSE
	w(float32(1852))     // ROUTE_DISTANCE
	w(int32(2))          // AT_OR_ABOVE
	w(float32(1219))     // ALTITUDE1
	w(float32(0))        // ALTITUDE2
	w(float32(210))      // SPEED_LIMIT
	w(0.0)               // ARC_CENTER lat
	w(0.0)               // ARC_CENTER lon
	w(float32(0))        // RHO
	w([3]int32{1, 0, 0}) // IAF, FAF, MAP
	if b.Len() != 112 {
		t.Fatalf("leg record %d bytes", b.Len())
	}
	return b.Bytes()
}

func procRecord(req uint32, typ types.SIMCONNECT_FACILITY_DATA_TYPE, id, parent uint32, payload []byte) engine.Message {
	m := facilityMsg(req, typ, 0, payload)
	h := m.AsFacilityData()
	h.UniqueRequestId, h.ParentUniqueRequestId = types.DWORD(id), types.DWORD(parent)
	off := int(uintptrOffsetData())
	h.DwSize = types.DWORD(off + len(payload))
	return m
}

// TestProcedureLoader: records nested by parent ID come out as a SID with its
// runway transition and legs decoded.
func TestProcedureLoader(t *testing.T) {
	c := &fakeClient{}
	l := NewProcedureLoader(c)
	if err := l.Request("lkpr"); err != nil {
		t.Fatal(err)
	}
	base := DefaultProcedureRequestBase
	var name [8]byte
	copy(name[:], "ARTU5A")
	var sid bytes.Buffer
	binary.Write(&sid, binary.LittleEndian, name)
	binary.Write(&sid, binary.LittleEndian, [3]int32{1, 0, 0})
	var rt bytes.Buffer
	binary.Write(&rt, binary.LittleEndian, [3]int32{24, 0, 1})
	msgs := []engine.Message{
		procRecord(base, types.SIMCONNECT_FACILITY_DATA_DEPARTURE, 10, 0, sid.Bytes()),
		procRecord(base, types.SIMCONNECT_FACILITY_DATA_RUNWAY_TRANSITION, 11, 10, rt.Bytes()),
		procRecord(base, types.SIMCONNECT_FACILITY_DATA_APPROACH_LEG, 12, 11, leg112(t, 18, "ARTUP", 50.68, 14.90)),
		endMsg(base), endMsg(base + 1), endMsg(base + 2),
	}
	var got Procedures
	done := false
	for _, m := range msgs {
		if p, ok := l.Handle(m); ok {
			got, done = p, true
		}
	}
	if !done || got.ICAO != "LKPR" || len(got.Departures) != 1 {
		t.Fatalf("done %v: %+v", done, got)
	}
	d := got.Departures[0]
	if d.Name != "ARTU5A" || len(d.RunwayTransitions) != 1 || d.RunwayTransitions[0].Runway != "24" {
		t.Fatalf("%+v", d)
	}
	leg := d.RunwayTransitions[0].Legs[0]
	if leg.Type != types.SIMCONNECT_FACILITY_LEG_TYPE_TF || leg.Fix != "ARTUP" || leg.Region != "LK" || leg.FixKind != "W" ||
		math.Abs(leg.Position.Lat-50.68) > 1e-9 || !leg.TurnRight || leg.Course != 242 || leg.Alt1 != 1219 ||
		leg.AltDesc != types.SIMCONNECT_FACILITY_ALTITUDE_DESCRIPTOR_AT_OR_ABOVE || leg.Speed != 210 || !leg.IAF {
		t.Errorf("leg %+v", leg)
	}
}

func uintptrOffsetData() uintptr {
	var h types.SIMCONNECT_RECV_FACILITY_DATA
	return unsafe.Offsetof(h.Data)
}

// TestProcedureLoaderWithIDs: a loader on its own IDs takes only its own
// replies; the default one's are not its (#710: two on one connection).
func TestProcedureLoaderWithIDs(t *testing.T) {
	c := &fakeClient{}
	l := NewProcedureLoaderWithIDs(c, 9400, 9500)
	if err := l.Request("lkpr"); err != nil {
		t.Fatal(err)
	}
	if _, ok := l.Handle(endMsg(DefaultProcedureRequestBase)); ok {
		t.Error("took a reply on the default IDs")
	}
	done := false
	for i := uint32(0); i < 3; i++ {
		if p, ok := l.Handle(endMsg(9500 + i)); ok {
			done = p.ICAO == "LKPR"
		}
	}
	if !done {
		t.Error("its own replies did not finish the airport")
	}
}

// The constrained fixes of a leg list, in feet and knots (#754).
func TestLegConstraints(t *testing.T) {
	legs := []Leg{
		{Fix: "AAA", Position: LatLon{Lat: 50, Lon: 14}, AltDesc: types.SIMCONNECT_FACILITY_ALTITUDE_DESCRIPTOR_AT_OR_ABOVE, Alt1: 3657.6},
		{Fix: "BBB", Position: LatLon{Lat: 50, Lon: 15}},
		{Fix: "CCC", Position: LatLon{Lat: 50, Lon: 16}, AltDesc: types.SIMCONNECT_FACILITY_ALTITUDE_DESCRIPTOR_BETWEEN, Alt1: 3048, Alt2: 2133.6, Speed: 220},
	}
	got := LegConstraints(legs)
	if len(got) != 2 || got[0].Fix != "AAA" || got[0].AtOrAboveFt != 12000 || got[0].AtOrBelowFt != 0 ||
		got[1].AtOrAboveFt != 7000 || got[1].AtOrBelowFt != 10000 || got[1].SpeedKts != 220 {
		t.Errorf("%+v", got)
	}
}
