package world

import (
	"encoding/binary"
	"math"
	"testing"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/addons"
	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/jetway"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// placeArrival is an arrival on approach whose stop on the stand can move.
type placeArrival struct {
	arrivalCtl
	nose, side float64
	calls      int
}

func (a *placeArrival) State() traffic.ArrivalState { return traffic.ArrivalApproaching }
func (a *placeArrival) SetStandPlace(nose, side float64) (bool, error) {
	if math.Abs(a.nose-nose) < 0.05 && math.Abs(a.side-side) < 0.05 {
		return false, nil
	}
	a.nose, a.side = nose, side
	a.calls++
	return true, nil
}

// An arrival spawned before the jetways were ready (live, LKPR 2026-11-07,
// CSA1894 at C22: spawned 19:47:18, gates filled 19:48:22) was placed as
// with no jetway, nose gear 5.4 m into the circle; at 20:10 the "B748
// D-A320" helper docked its jetway to a door 9 m ahead of the aircraft's.
// Once the helper is known, the stop moves to where its door meets ours,
// before the arrival lands.
func TestArrivalStandPlaceFollowsJetway(t *testing.T) {
	g, err := airport.BuildGraph(lkprLayout(t))
	if err != nil {
		t.Fatal(err)
	}
	c22, err := g.Layout.ParkingIndex("C22")
	if err != nil {
		t.Fatal(err)
	}
	// The installed figures (feet), as the map reads them.
	setDoors(map[string]addons.ModelDoor{
		"FSLTL_A320_LYX":                   {Type: "A320", First: addons.InteractivePoint{Long: 37.65}, NoseGearFt: 37.319, HasNoseGear: true},
		"MyCrew Jetway Helper A320":        {Type: "A320", First: addons.InteractivePoint{Long: 37.65}, NoseGearFt: 37.319, HasNoseGear: true},
		"MyCrew Jetway Helper B748":        {Type: "B748", First: addons.InteractivePoint{Long: -40.2}, NoseGearFt: 9.15, HasNoseGear: true},
		"MyCrew Jetway Helper B748 D-A320": {Type: "B748", First: addons.InteractivePoint{Long: 37.65}, NoseGearFt: 9.15, HasNoseGear: true},
	}, map[string]addons.ModelDoor{"A320": {WingSpanFt: 117.4}, "B748": {WingSpanFt: 203.1}})
	defer setDoors(nil, nil)
	jetwaysOn.mu.Lock()
	wasOn := jetwaysOn.on
	jetwaysOn.on = true
	jetwaysOn.mu.Unlock()
	defer func() {
		jetwaysOn.mu.Lock()
		jetwaysOn.on = wasOn
		jetwaysOn.mu.Unlock()
	}()

	w := New(Options{DataDir: t.TempDir()})
	cc := newControlCenter(nil, w.st.core)
	const model = "FSLTL_A320_LYX"
	// As it spawned: the helper backend made, the door variant listed, but
	// the simulator's jetway list not in yet, so C22's stand rule (9 m
	// back) still applied: 35 m − 17.14 m, its reference 17.86 m ahead of
	// the parking point, as logged.
	cc.mu.Lock()
	cc.models = map[string]bool{jetway.HelperDoorTitle("B748", "A320"): true}
	cc.mu.Unlock()
	svc := jetway.NewHelpers(nil, jetway.HelperOptions{Types: []string{"A320", "B748"}})
	cc.jw.mu.Lock()
	cc.jw.svc = svc
	cc.jw.mu.Unlock()
	if h := gateHelper(svc, g, c22, model, cc.hasModel); h != jetway.HelperDoorTitle("B748", "A320") {
		t.Fatalf("C22 helper %q", h)
	}
	spawned, _ := cc.standPlace(g, c22, model)
	if math.Abs(g.Layout.Parking[c22].Radius-spawned-17.86) > 0.1 {
		t.Errorf("spawned %.2f m ahead of the parking point, live 17.86", g.Layout.Parking[c22].Radius-spawned)
	}
	arr := &placeArrival{nose: spawned}
	cc.items[1] = &controlled{ID: 1, Tail: "CSA1894", ICAO: "LKPR", cc: cc, arr: arr, graph: g, stand: c22, removed: make(chan struct{})}
	cc.items[1].view.Model = model

	// The jetway list in: C22 has a jetway, the jetway wins over the rule.
	svc.Handle(jetwayList("LKPR", c22))
	if !svc.HasJetway("LKPR", c22) {
		t.Fatal("C22 has no jetway after the list")
	}
	want, wantSide := cc.standPlace(g, c22, model)
	if math.Abs(want-spawned) < 1 {
		t.Fatalf("the helper moves nothing (%.2f, spawned %.2f): no case", want, spawned)
	}

	cc.recheckArrivalStands()
	if arr.calls != 1 || math.Abs(arr.nose-want) > 0.01 || math.Abs(arr.side-wantSide) > 0.01 {
		t.Errorf("stop at %.2f m, %.2f m right (%d moves), want %.2f, %.2f (spawned at %.2f)", arr.nose, arr.side, arr.calls, want, wantSide, spawned)
	}
	cc.recheckArrivalStands()
	if arr.calls != 1 {
		t.Errorf("moved again with nothing changed (%d)", arr.calls)
	}
}

// jetwayList is the simulator's jetway list: one idle jetway on each of
// icao's stands given (packed entries, types.JetwayDataSize bytes).
func jetwayList(icao string, stands ...int) engine.Message {
	const header = 28 // SIMCONNECT_RECV_LIST_TEMPLATE
	buf := make([]byte, header+len(stands)*types.JetwayDataSize)
	le := binary.LittleEndian
	le.PutUint32(buf[0:], uint32(len(buf)))
	le.PutUint32(buf[8:], uint32(types.SIMCONNECT_RECV_ID_JETWAY_DATA))
	le.PutUint32(buf[16:], uint32(len(stands)))
	le.PutUint32(buf[24:], 1)
	for i, s := range stands {
		e := buf[header+i*types.JetwayDataSize:]
		copy(e[0:8], icao)
		le.PutUint32(e[8:], uint32(s))
	}
	return engine.Message{SIMCONNECT_RECV: (*types.SIMCONNECT_RECV)(unsafe.Pointer(&buf[0]))}
}
