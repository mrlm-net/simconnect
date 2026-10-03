//go:build windows
// +build windows

package avionics

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/types"
)

type fakeClient struct {
	mapped map[uint32]string
	sent   [][2]uint32 // event, data
}

func (f *fakeClient) MapClientEventToSimEvent(id uint32, name string) error {
	if f.mapped == nil {
		f.mapped = map[uint32]string{}
	}
	f.mapped[id] = name
	return nil
}

func (f *fakeClient) TransmitClientEvent(obj, id, data, group uint32, flags types.SIMCONNECT_EVENT_FLAG) error {
	f.sent = append(f.sent, [2]uint32{id, data})
	return nil
}

func TestRadios(t *testing.T) {
	f := &fakeClient{}
	r := New(f, 0)
	if err := r.SetCOMStandby(1, 134.56); err != nil {
		t.Fatal(err)
	}
	if err := r.SetCOMActive(2, 118.105); err != nil { // 8.33 kHz channel
		t.Fatal(err)
	}
	if err := r.SwapCOM(1); err != nil {
		t.Fatal(err)
	}
	if err := r.SetSquawk("4521"); err != nil {
		t.Fatal(err)
	}
	want := []struct {
		name string
		data uint32
	}{{"COM_STBY_RADIO_SET_HZ", 134560000}, {"COM2_RADIO_SET_HZ", 118105000}, {"COM1_RADIO_SWAP", 0}, {"XPNDR_SET", 0x4521}}
	if len(f.sent) != len(want) || len(f.mapped) != eventCount {
		t.Fatalf("sent %v, mapped %d", f.sent, len(f.mapped))
	}
	for i, w := range want {
		if got := f.mapped[f.sent[i][0]]; got != w.name || f.sent[i][1] != w.data {
			t.Errorf("event %d: %s %d, want %s %d", i, got, f.sent[i][1], w.name, w.data)
		}
	}
	for _, bad := range []error{r.SetCOMStandby(4, 120), r.SetCOMStandby(1, 108), r.SwapCOM(0), r.SetSquawk("7800"), r.SetSquawk("123")} {
		if bad == nil {
			t.Error("bad input accepted")
		}
	}
}
