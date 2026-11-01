package traffic

import (
	"testing"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// gearClient records the definitions written on top of eventClient.
type gearClient struct {
	eventClient
	setDefs []uint32
}

func (c *gearClient) SetDataOnSimObject(def, obj uint32, f types.SIMCONNECT_DATA_SET_FLAG, n, size uint32, p unsafe.Pointer) error {
	c.setDefs = append(c.setDefs, def)
	return nil
}

func exceptionMsg(sendID, index uint32) engine.Message {
	m := &types.SIMCONNECT_RECV_EXCEPTION{DwException: 20, DwSendID: types.DWORD(sendID), DwIndex: types.DWORD(index)}
	m.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_EXCEPTION)
	return engine.Message{SIMCONNECT_RECV: &m.SIMCONNECT_RECV}
}

// TestHoldGearDownPartial: a model refusing the main gears' positions (an
// exception on the tracked HoldGearDown write) is held down by the handle
// and nose gear from then on, and the next aircraft of that model too
// (live, the FSLTL Phenom: an exception every frame of its take-off roll).
func TestHoldGearDownPartial(t *testing.T) {
	c := &gearClient{}
	inj := NewInjector(c)
	if err := inj.Takeover(42); err != nil {
		t.Fatal(err)
	}
	inj.SetModel(42, "FSLTL E55P")
	if err := inj.HoldGearDown(42); err != nil {
		t.Fatal(err)
	}
	full, partial := inj.defBase+injDefGearDown, inj.defBase+injDefGearDownPartial
	if c.setDefs[len(c.setDefs)-1] != full {
		t.Fatalf("first write to def %d, want the full gear %d", c.setDefs[len(c.setDefs)-1], full)
	}
	if handled, err := inj.Handle(exceptionMsg(c.sendID, 2)); !handled || err == nil {
		t.Fatalf("exception not taken: %v %v", handled, err)
	}
	inj.HoldGearDown(42)
	if c.setDefs[len(c.setDefs)-1] != partial {
		t.Errorf("after the exception def %d, want the handle and nose gear %d", c.setDefs[len(c.setDefs)-1], partial)
	}
	if err := inj.Takeover(43); err != nil {
		t.Fatal(err)
	}
	inj.SetModel(43, "FSLTL E55P")
	inj.HoldGearDown(43)
	if c.setDefs[len(c.setDefs)-1] != partial {
		t.Errorf("the next of the model: def %d, want %d", c.setDefs[len(c.setDefs)-1], partial)
	}
}

// TestHoldGearDownRefusedAll: a model refusing the handle and nose gear too
// goes on to the handle alone, then to nothing, and the frames already sent
// when the first refusal comes don't take it further (live, the passive
// Phenom: an exception every frame of its take-off roll, 1,850 of them).
func TestHoldGearDownRefusedAll(t *testing.T) {
	c := &gearClient{}
	inj := NewInjector(c)
	if err := inj.Takeover(42); err != nil {
		t.Fatal(err)
	}
	inj.SetModel(42, "Passive E55P")
	full, partial, handle := inj.defBase+injDefGearDown, inj.defBase+injDefGearDownPartial, inj.defBase+injDefGear
	for _, want := range []uint32{full, partial, handle} {
		inj.HoldGearDown(42)
		first := c.sendID
		inj.HoldGearDown(42) // the next frame, before the answer
		if got := c.setDefs[len(c.setDefs)-1]; got != want {
			t.Fatalf("def %d, want %d", got, want)
		}
		if _, err := inj.Handle(exceptionMsg(first, 2)); err == nil {
			t.Fatal("the first refusal not told")
		}
		if handled, err := inj.Handle(exceptionMsg(c.sendID, 2)); !handled || err != nil {
			t.Fatalf("the second refusal: %v %v", handled, err)
		}
	}
	n := len(c.setDefs)
	inj.HoldGearDown(42)
	if len(c.setDefs) != n {
		t.Errorf("refused everything and still written: def %d", c.setDefs[len(c.setDefs)-1])
	}
	// The next of the model starts where this one ended.
	if err := inj.Takeover(43); err != nil {
		t.Fatal(err)
	}
	inj.SetModel(43, "Passive E55P")
	inj.HoldGearDown(43)
	if len(c.setDefs) != n {
		t.Errorf("the next of the model written: def %d", c.setDefs[len(c.setDefs)-1])
	}
}
