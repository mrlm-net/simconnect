//go:build windows

package manager

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/manager/internal/instance"
)

type fakeSubscriber struct{ got map[string]uint32 }

func (f *fakeSubscriber) SubscribeToSystemEvent(id uint32, name string) error {
	f.got[name] = id
	return nil
}

// TestConnectionLostCleansUp: the simulator going away clears the request
// registry and the camera request as Stop does, keeps the custom system
// events, and the next connection subscribes them again with their IDs (#405).
func TestConnectionLostCleansUp(t *testing.T) {
	m := New("test").(*Instance)
	m.requestRegistry.Register(7000, RequestTypeDataDefinition, "a request of the old connection")
	m.cameraDataRequestPending = true
	m.customSystemEvents["Crashed2"] = &instance.CustomSystemEvent{Name: "Crashed2", ID: CustomEventIDMin + 3}

	m.connectionLost()

	if n := m.requestRegistry.Count(); n != 0 {
		t.Errorf("%d requests left, want 0", n)
	}
	if m.cameraDataRequestPending {
		t.Error("the camera request is still pending")
	}
	if m.state != StateDisconnected {
		t.Errorf("state %v, want disconnected", m.state)
	}
	if _, kept := m.customSystemEvents["Crashed2"]; !kept {
		t.Fatal("the custom event was dropped")
	}
	f := &fakeSubscriber{got: map[string]uint32{}}
	m.resubscribeCustomEvents(f)
	if id, ok := f.got["Crashed2"]; !ok || id != CustomEventIDMin+3 {
		t.Errorf("subscribed again %v, want Crashed2 with id %d", f.got, CustomEventIDMin+3)
	}
}
