package failures

import (
	"errors"
	"slices"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/avionics"
	"github.com/mrlm-net/simconnect/pkg/systems"
	"github.com/mrlm-net/simconnect/pkg/types"
)

type fakeClient struct {
	mapped map[uint32]string
	sent   []string
}

func (c *fakeClient) MapClientEventToSimEvent(id uint32, name string) error {
	if c.mapped == nil {
		c.mapped = map[uint32]string{}
	}
	c.mapped[id] = name
	return nil
}

func (c *fakeClient) TransmitClientEvent(obj, id, data, group uint32, flags types.SIMCONNECT_EVENT_FLAG) error {
	c.sent = append(c.sent, c.mapped[id])
	return nil
}

// TestFailures: a failure is toggled on once and off once by its event;
// an engine by its own; a kind the profile cannot fail is refused, and a
// model's override takes a kind away.
func TestFailures(t *testing.T) {
	c := &fakeClient{}
	f := New(c, Default(), 0)
	if err := f.Apply(Failure{Kind: EngineFailure, Engine: 2}); err != nil {
		t.Fatal(err)
	}
	if err := f.Apply(Failure{Kind: EngineFailure, Engine: 2}); err != nil { // active already
		t.Fatal(err)
	}
	if err := f.Apply(Failure{Kind: Hydraulic}); err != nil {
		t.Fatal(err)
	}
	if len(f.Active()) != 2 {
		t.Fatalf("active %v, want two", f.Active())
	}
	if err := f.ClearAll(); err != nil {
		t.Fatal(err)
	}
	want := []string{"TOGGLE_ENGINE2_FAILURE", "TOGGLE_HYDRAULIC_FAILURE"}
	got := slices.Clone(c.sent)
	slices.Sort(got)
	if len(c.sent) != 4 || got[0] != want[0] || got[2] != want[1] {
		t.Errorf("sent %v, want each of %v on and off", c.sent, want)
	}
	if err := f.Apply(Failure{Kind: EngineFire, Engine: 1}); !errors.Is(err, ErrUnsupported) {
		t.Errorf("engine fire: %v, want unsupported", err)
	}
	study := New(c, Default().With(Profile{Events: map[Kind]string{Hydraulic: ""}}), 0)
	if slices.Contains(study.Supported(), Hydraulic) {
		t.Error("taken away by the override, still supported")
	}
}

// TestEventBaseOwnBlock: the default client event IDs are a block no other
// package of the library maps by default (systems.Controls had 0x7B00 too).
func TestEventBaseOwnBlock(t *testing.T) {
	for name, b := range map[string]uint32{"systems.Controls": systems.DefaultControlBase, "avionics": avionics.DefaultEventBase} {
		if DefaultEventBase>>8 == b>>8 {
			t.Errorf("DefaultEventBase %#x in the block of %s (%#x)", DefaultEventBase, name, b)
		}
	}
}
