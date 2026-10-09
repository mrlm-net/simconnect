//go:build windows

package manager

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// eventMessage is a SIMCONNECT_RECV_EVENT message for eventID with data.
func eventMessage(eventID, data uint32) engine.Message {
	ev := &types.SIMCONNECT_RECV_EVENT{UEventID: types.DWORD(eventID), DwData: types.DWORD(data)}
	ev.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_EVENT)
	return engine.Message{SIMCONNECT_RECV: &ev.SIMCONNECT_RECV}
}

// TestEventSubscriptionsDeliver: a pause event reaches the channel
// subscription and the OnPause callback fires once (#404).
func TestEventSubscriptionsDeliver(t *testing.T) {
	m := New("test").(*Instance)
	m.pauseEventID = 42
	sub := m.SubscribeOnPause("p", 4)
	defer sub.Unsubscribe()
	calls := 0
	m.OnPause(func(paused bool) { calls++ })

	m.processMessage(eventMessage(42, 1))

	select {
	case msg := <-sub.Messages():
		if ev := msg.AsEvent(); ev == nil || ev.UEventID != 42 {
			t.Fatalf("got %+v, want the pause event", msg.SIMCONNECT_RECV)
		}
	case <-time.After(time.Second):
		t.Fatal("the pause subscription got nothing")
	}
	if calls != 1 {
		t.Errorf("OnPause fired %d times, want 1", calls)
	}
	if !m.simState.Paused {
		t.Error("the state is not paused")
	}
}

// TestOpenQuitForwarded: OPEN and QUIT reach a plain subscription after
// the manager handled them (#404).
func TestOpenQuitForwarded(t *testing.T) {
	m := New("test").(*Instance)
	sub := m.Subscribe("all", 4)
	defer sub.Unsubscribe()
	for _, id := range []types.SIMCONNECT_RECV_ID{types.SIMCONNECT_RECV_ID_OPEN, types.SIMCONNECT_RECV_ID_QUIT} {
		// A whole OPEN record, as the simulator sends: AsOpen reads it all
		// (checkptr under -race refused a bare header).
		open := &types.SIMCONNECT_RECV_OPEN{}
		open.DwID = types.DWORD(id)
		m.processMessage(engine.Message{SIMCONNECT_RECV: &open.SIMCONNECT_RECV})
		select {
		case msg := <-sub.Messages():
			if types.SIMCONNECT_RECV_ID(msg.DwID) != id {
				t.Fatalf("got %d, want %d", msg.DwID, id)
			}
		case <-time.After(time.Second):
			t.Fatalf("message %d was not forwarded", id)
		}
	}
}
