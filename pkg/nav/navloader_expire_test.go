package nav

import (
	"errors"
	"testing"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// fakeNavClient records the requests, numbers the packets, and fails the
// request numbered failAt (1-based; 0 never).
type fakeNavClient struct {
	reqs   []uint32
	sent   uint32
	failAt int
}

func (f *fakeNavClient) AddToFacilityDefinition(uint32, string) error { return nil }
func (f *fakeNavClient) RequestFacilityDataEX1(def, req uint32, icao, region string, kind byte) error {
	if f.failAt > 0 && len(f.reqs)+1 == f.failAt {
		f.failAt = 0
		return errors.New("send failed")
	}
	f.reqs = append(f.reqs, req)
	f.sent++
	return nil
}
func (f *fakeNavClient) GetLastSentPacketID() (uint32, error) { return f.sent, nil }

func navEndMsg(req uint32) engine.Message {
	h := &types.SIMCONNECT_RECV_FACILITY_DATA_END{RequestId: types.DWORD(req)}
	h.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_FACILITY_DATA_END)
	return engine.Message{SIMCONNECT_RECV: &h.SIMCONNECT_RECV}
}

func navExceptionMsg(sendID uint32) engine.Message {
	h := &types.SIMCONNECT_RECV_EXCEPTION{DwSendID: types.DWORD(sendID)}
	h.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_EXCEPTION)
	return engine.Message{SIMCONNECT_RECV: &h.SIMCONNECT_RECV}
}

// navWaypointMsg is a WAYPOINT record at lat, lon for request req.
func navWaypointMsg(req uint32, lat, lon float64) engine.Message {
	var w recordWriter
	w.f64(lat)
	w.f64(lon)
	w.i32(int32(WaypointRNAV))
	w.str("", 8)
	w.str("", 8)
	w.i32(0)
	w.i32(0)
	var hdr types.SIMCONNECT_RECV_FACILITY_DATA
	off := int(unsafe.Offsetof(hdr.Data))
	buf := make([]byte, off+w.Len())
	h := (*types.SIMCONNECT_RECV_FACILITY_DATA)(unsafe.Pointer(&buf[0]))
	h.DwSize = types.DWORD(len(buf))
	h.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_FACILITY_DATA)
	h.UserRequestId = types.DWORD(req)
	h.Type = types.SIMCONNECT_FACILITY_DATA_WAYPOINT
	copy(buf[off:], w.Bytes())
	return engine.Message{SIMCONNECT_RECV: (*types.SIMCONNECT_RECV)(unsafe.Pointer(&buf[0]))}
}

// TestNavLoaderLateReply: a request ended by Expire keeps its slot until
// its late reply is in; that reply never lands in the next fix (#45).
func TestNavLoaderLateReply(t *testing.T) {
	c := &fakeNavClient{}
	l := NewNavLoaderWithIDs(c, 100, 200, 1)
	l.SetTimeout(time.Second)
	if err := l.Request(Key("AAAAA", "LK", KindWaypoint)); err != nil {
		t.Fatal(err)
	}
	res := l.Expire(time.Now().Add(2 * time.Second))
	if len(res) != 1 || res[0].Found || res[0].Key.Ident != "AAAAA" {
		t.Fatalf("Expire = %+v", res)
	}
	if l.Free() != 0 || l.Pending() != 0 {
		t.Fatalf("after Expire: free %d pending %d, want the slot held, nothing loading", l.Free(), l.Pending())
	}
	if err := l.Request(Key("BBBBB", "LK", KindWaypoint)); err == nil {
		t.Fatal("a held slot was reused")
	}
	// The late reply: taken, nothing reported, slot free again.
	if r, ok := l.Handle(navWaypointMsg(200, 10, 20)); ok {
		t.Fatalf("late record reported %+v", r)
	}
	if r, ok := l.Handle(navEndMsg(200)); ok {
		t.Fatalf("late END reported %+v", r)
	}
	if l.Free() != 1 {
		t.Fatal("slot not freed by the late END")
	}
	if err := l.Request(Key("BBBBB", "LK", KindWaypoint)); err != nil {
		t.Fatal(err)
	}
	l.Handle(navWaypointMsg(200, 50, 14))
	r, ok := l.Handle(navEndMsg(200))
	if !ok || r.Key.Ident != "BBBBB" || !r.Found || r.Fix.Position.Lat != 50 {
		t.Fatalf("BBBBB = %+v %v", r, ok)
	}

	// No late reply at all: the slot comes back after another timeout.
	if err := l.Request(Key("CCCCC", "LK", KindWaypoint)); err != nil {
		t.Fatal(err)
	}
	t0 := time.Now()
	l.Expire(t0.Add(2 * time.Second))
	l.Expire(t0.Add(2500 * time.Millisecond))
	if l.Free() != 0 {
		t.Fatal("held slot freed early")
	}
	l.Expire(t0.Add(4 * time.Second))
	if l.Free() != 1 {
		t.Fatal("held slot never freed")
	}

	// A late exception frees it as well.
	if err := l.Request(Key("DDDDD", "LK", KindWaypoint)); err != nil {
		t.Fatal(err)
	}
	l.Expire(time.Now().Add(2 * time.Second))
	l.Handle(navExceptionMsg(c.sent))
	if l.Free() != 1 {
		t.Fatal("slot not freed by the late exception")
	}
}

// TestNavLoaderPartialSend: the navaid request failing after the waypoint
// one went out is an error, and the slot waits for the waypoint's reply
// (#47).
func TestNavLoaderPartialSend(t *testing.T) {
	c := &fakeNavClient{failAt: 2}
	l := NewNavLoaderWithIDs(c, 100, 200, 1)
	if err := l.Request(Key("VOZ", "LK", KindVOR)); err == nil {
		t.Fatal("no error for the failed navaid request")
	}
	if l.Pending() != 0 || l.Free() != 0 {
		t.Fatalf("pending %d free %d", l.Pending(), l.Free())
	}
	if r, ok := l.Handle(navEndMsg(200)); ok {
		t.Fatalf("reported %+v", r)
	}
	if l.Free() != 1 {
		t.Fatal("slot not freed")
	}
}

// failingFixLoader fails every request.
type failingFixLoader struct{ fakeFixLoader }

func (f *failingFixLoader) Request(FixKey) error { return errors.New("not connected") }

// TestPLNResolverRequestError: fixes that cannot be requested are missing
// and the resolver finishes (#47).
func TestPLNResolverRequestError(t *testing.T) {
	plan := &PLNPlan{Waypoints: []PLNWaypoint{
		{Type: "Intersection", Ident: "GOLOP", Region: "LK"},
		{Type: "VOR", Ident: "VOZ", Region: "LK"},
	}}
	r := newPLNResolver(&failingFixLoader{}, plan, nil)
	if err := r.Start(); err == nil {
		t.Error("Start: no error")
	}
	if !r.Done() || len(r.Missing()) != 2 || r.Err() == nil {
		t.Errorf("done %v missing %v err %v", r.Done(), r.Missing(), r.Err())
	}
}
