//go:build windows

package hotkeys

import (
	"fmt"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

type fakeClient struct {
	calls []string
}

func (f *fakeClient) MapInputEventToClientEvent(g uint32, def string, down, _, up, _ uint32, _ bool) error {
	f.calls = append(f.calls, fmt.Sprintf("map %x %s %x %x", g, def, down, up))
	return nil
}
func (f *fakeClient) SetInputGroupPriority(g, p uint32) error {
	f.calls = append(f.calls, fmt.Sprintf("priority %x %d", g, p))
	return nil
}
func (f *fakeClient) SetInputGroupState(g, s uint32) error {
	f.calls = append(f.calls, fmt.Sprintf("state %x %d", g, s))
	return nil
}
func (f *fakeClient) RemoveInputEvent(g uint32, def string) error {
	f.calls = append(f.calls, fmt.Sprintf("remove %x %s", g, def))
	return nil
}

func event(id uint32) engine.Message {
	ev := &types.SIMCONNECT_RECV_EVENT{SIMCONNECT_RECV: types.SIMCONNECT_RECV{DwID: types.DWORD(types.SIMCONNECT_RECV_ID_EVENT)}, UEventID: types.DWORD(id)}
	return engine.Message{SIMCONNECT_RECV: &ev.SIMCONNECT_RECV}
}

func has(calls []string, want string) bool {
	for _, c := range calls {
		if c == want {
			return true
		}
	}
	return false
}

// TestHotkeys: bound before the connection, mapped on Attach (the group at
// the highest priority, on); rebinding removes the old input; the event
// calls the action; a reconnect maps every binding again.
func TestHotkeys(t *testing.T) {
	h := New(0)
	pressed := 0
	if err := h.Bind("checklistItem", "Ctrl+Shift+C", func() { pressed++ }); err != nil {
		t.Fatal(err)
	}
	f := &fakeClient{}
	if err := h.Attach(f); err != nil {
		t.Fatal(err)
	}
	if !has(f.calls, "map b100 Ctrl+Shift+C b101 ffffffff") || !has(f.calls, "priority b100 1") || !has(f.calls, "state b100 1") {
		t.Fatalf("attach: %v", f.calls)
	}
	if err := h.Bind("checklistItem", "joystick:0:button:5", nil); err != nil {
		t.Fatal(err)
	}
	if !has(f.calls, "remove b100 Ctrl+Shift+C") || !has(f.calls, "map b100 joystick:0:button:5 b101 ffffffff") {
		t.Errorf("rebind: %v", f.calls)
	}
	if !h.Handle(event(0xB101)) || pressed != 1 {
		t.Errorf("pressed %d", pressed)
	}
	if h.Handle(event(0xA001)) {
		t.Error("another range handled")
	}
	g := &fakeClient{}
	h.Attach(g) // reconnected
	if !has(g.calls, "map b100 joystick:0:button:5 b101 ffffffff") {
		t.Errorf("reconnect: %v", g.calls)
	}
	if b := h.Bindings(); b["checklistItem"] != "joystick:0:button:5" {
		t.Errorf("bindings %v", b)
	}
	if err := h.SetBindings(map[string]string{"checklistItem": "VK_F12"}); err != nil || !has(g.calls, "map b100 VK_F12 b101 ffffffff") {
		t.Errorf("set bindings: %v %v", err, g.calls)
	}
}
