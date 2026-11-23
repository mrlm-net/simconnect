package traffic

import (
	"math"
	"testing"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// altClient records the altitudes the injector writes, at the test's time.
type altClient struct {
	eventClient
	now    *time.Time
	writes []altWrite
}

type altWrite struct {
	at time.Time
	ft float64
}

func (c *altClient) SetDataOnSimObject(def, obj uint32, flag types.SIMCONNECT_DATA_SET_FLAG, n, size uint32, p unsafe.Pointer) error {
	if def == DefaultInjectDefinitionBase+injDefPosition && size == uint32(unsafe.Sizeof(types.SIMCONNECT_DATA_INITPOSITION{})) {
		c.writes = append(c.writes, altWrite{*c.now, (*types.SIMCONNECT_DATA_INITPOSITION)(p).Altitude})
	}
	return c.eventClient.SetDataOnSimObject(def, obj, flag, n, size, p)
}

// flownMsg is MSFS AI's position with its MSL altitude.
func flownMsg(req, obj uint32, p airport.LatLon, altFt, aglFt, hdg, kts float64) engine.Message {
	var hdr types.SIMCONNECT_RECV_SIMOBJECT_DATA
	off := int(unsafe.Offsetof(hdr.DwData))
	buf := make([]byte, off+int(unsafe.Sizeof(arrivalMonitor{})))
	h := (*types.SIMCONNECT_RECV_SIMOBJECT_DATA)(unsafe.Pointer(&buf[0]))
	h.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA)
	h.DwRequestID, h.DwObjectID = types.DWORD(req), types.DWORD(obj)
	*(*arrivalMonitor)(unsafe.Pointer(&buf[off])) = arrivalMonitor{Latitude: p.Lat, Longitude: p.Lon, AGL: aglFt, Heading: hdg, GroundKts: kts, VS: -300, AltFt: altFt}
	return engine.Message{SIMCONNECT_RECV: (*types.SIMCONNECT_RECV)(unsafe.Pointer(&buf[0]))}
}

// goAroundMaxFpm: the most a go-around climbs or descends, flown or
// written, from its circuit to the runway (#1681).
const goAroundMaxFpm = 4000.0

// A go-around round its circuit at 14,000 ft (TNCM RNAV 10's hold before
// #1659) comes down within an airliner's rate, and the hand-over to the
// injected approach writes no altitude step: AAL432 was 7,000 ft above
// the final at the join and put onto it in 30 s (#1681). MSFS AI flies the
// circuit at 3,000 fpm at most.
func TestGoAroundDescentFromHigh(t *testing.T) {
	g := lkprGraph(t)
	now := time.Now()
	ec := &altClient{now: &now}
	inj := NewInjector(ec)
	ctl := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj))
	c22, _ := g.Layout.ParkingIndex("C22")
	if err := ctl.Start(ArrivalRequest{Graph: g, Runway: "24", Parking: c22, Model: "B738", Tail: "AAL432",
		InjectApproach: true, RollThroughChance: -1}); err != nil {
		t.Fatal(err)
	}
	go func() {
		for range ctl.Events() {
		}
	}()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	mon := DefaultArrivalRequestBase + arrReqMonitor
	p := ctl.Plan()
	for i := 0; i < 60*600 && ctl.approach != nil && ctl.approach.Pose().HeightFt > 300; i++ {
		now = now.Add(time.Second / 60)
		ctl.Handle(arrivalPositionMsg(mon, 77, p.End.Threshold, 0, 0, 0, false))
	}
	ctl.req.MissedApproach = []airport.NavPoint{{Ident: "CLIMB", Position: offsetHeading(p.End.LandingThreshold(), p.End.Heading, 8*1852), AltMin: 14000 / ftPerMeter}}
	if err := ctl.GoAround(); err != nil {
		t.Fatal(err)
	}
	fieldFt := g.Layout.Altitude / 0.3048
	wps := append([]types.SIMCONNECT_DATA_WAYPOINT(nil), ctl.proc.Waypoints...)
	if top := wps[0].Altitude; top < 13900 {
		t.Fatalf("the circuit at %.0f ft, want the missed approach's 14000", top)
	}
	// MSFS AI round the circuit at 180 kt, a second at a time, each
	// waypoint's altitude by its arrival there at 3,000 fpm at most.
	const kts, aiFpm = 180.0, 3000.0
	pos, alt := airport.LatLon{Lat: wps[0].Latitude, Lon: wps[0].Longitude}, wps[0].Altitude
	worst, k := 0.0, 1
	for i := 0; i < 3*3600 && ctl.flyingProc; i++ {
		for k < len(wps) && calc.HaversineNM(pos.Lat, pos.Lon, wps[k].Latitude, wps[k].Longitude) < kts/3600 {
			k++
		}
		if k >= len(wps) {
			t.Fatal("flew its circuit to the end, not taken over")
		}
		w := airport.LatLon{Lat: wps[k].Latitude, Lon: wps[k].Longitude}
		d := calc.HaversineNM(pos.Lat, pos.Lon, w.Lat, w.Lon)
		hdg := calc.BearingDegrees(pos.Lat, pos.Lon, w.Lat, w.Lon)
		need := (wps[k].Altitude - alt) / (d / kts * 60) // fpm to be there at it
		vs := math.Max(-aiFpm, math.Min(aiFpm, need))
		lat, lon := calc.DisplaceByHeading(pos.Lat, pos.Lon, hdg, kts/3600*1852)
		pos, alt = airport.LatLon{Lat: lat, Lon: lon}, alt+vs/60
		worst = math.Max(worst, math.Abs(need))
		now = now.Add(time.Second)
		ctl.Handle(flownMsg(mon, 77, pos, alt, alt-fieldFt, hdg, kts))
	}
	if ctl.flyingProc || ctl.approach == nil {
		t.Fatal("not taken over for the second approach")
	}
	if worst > aiFpm*1.05 {
		t.Errorf("its circuit asks %.0f fpm of MSFS AI, it flies %.0f at most", worst, aiFpm)
	}
	// The injected approach from the hand-over: the altitudes written, each
	// second's change no more than goAroundMaxFpm, the first against MSFS
	// AI's last.
	from := len(ec.writes)
	for i := 0; i < 90*60 && ctl.approach != nil; i++ {
		now = now.Add(time.Second / 60)
		ctl.Handle(arrivalPositionMsg(mon, 77, p.End.Threshold, 0, 0, 0, false))
		inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	}
	writes := ec.writes[from:]
	if len(writes) < 60 {
		t.Fatalf("%d placements after the hand-over", len(writes))
	}
	prev := altWrite{writes[0].at.Add(-time.Second), alt}
	for _, x := range writes {
		if dt := x.at.Sub(prev.at); dt >= time.Second {
			if fpm := (x.ft - prev.ft) / dt.Minutes(); math.Abs(fpm) > goAroundMaxFpm {
				t.Fatalf("%.0f ft to %.0f ft in %v after the hand-over (%.0f fpm)", prev.ft, x.ft, dt, fpm)
			}
			prev = x
		}
	}
}
