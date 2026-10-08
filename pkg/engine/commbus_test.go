package engine

import (
	"testing"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// commBusMsg is a SIMCONNECT_RECV_COMM_BUS message for eventID with data,
// part n of of.
func commBusMsg(eventID uint32, data string, n, of int) Message {
	head := int(unsafe.Sizeof(types.SIMCONNECT_RECV_COMM_BUS{}))
	buf := make([]byte, head+len(data)+1)
	cb := (*types.SIMCONNECT_RECV_COMM_BUS)(unsafe.Pointer(&buf[0]))
	cb.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_COMM_BUS)
	cb.DwSize = types.DWORD(len(buf))
	cb.UEventID = types.DWORD(eventID)
	cb.DwEntryNumber, cb.DwOutOf = types.DWORD(n), types.DWORD(of)
	copy(buf[head:], data)
	return newMessage(&cb.SIMCONNECT_RECV, uint32(len(buf)), nil, buf, nil)
}

// TestCommBus: the data after the header, and a call sent in three parts
// joined back (#678).
func TestCommBus(t *testing.T) {
	m := commBusMsg(7, `{"icao":"LKPR"}`, 0, 1)
	if got := m.CommBusData(); got != `{"icao":"LKPR"}` {
		t.Fatalf("data %q", got)
	}
	var a CommBusAssembler
	if id, data, ok := a.Add(&m); !ok || id != 7 || data != `{"icao":"LKPR"}` {
		t.Fatalf("single: %d %q %v", id, data, ok)
	}
	parts := []string{`{"metar":"LKPR 041`, `200Z 35006KT CAVOK`, ` 15/03 Q1029"}`}
	for i, p := range parts {
		m := commBusMsg(9, p, i, len(parts))
		id, data, ok := a.Add(&m)
		if i < len(parts)-1 {
			if ok {
				t.Fatalf("part %d: complete too early", i)
			}
			continue
		}
		if !ok || id != 9 || data != parts[0]+parts[1]+parts[2] {
			t.Fatalf("joined: %d %q %v", id, data, ok)
		}
	}
	other := Message{SIMCONNECT_RECV: &types.SIMCONNECT_RECV{DwID: types.DWORD(types.SIMCONNECT_RECV_ID_EVENT)}}
	if other.AsCommBus() != nil || other.CommBusData() != "" {
		t.Error("an event read as CommBus")
	}
}

// A call whose middle part was lost is dropped, not joined with a gap (E8).
func TestCommBusPartLost(t *testing.T) {
	var a CommBusAssembler
	for _, i := range []int{0, 2} {
		m := commBusMsg(9, "x", i, 3)
		if _, _, ok := a.Add(&m); ok {
			t.Fatalf("part %d completed a call missing part 1", i)
		}
	}
	m := commBusMsg(9, "whole", 0, 1)
	if _, data, ok := a.Add(&m); !ok || data != "whole" {
		t.Errorf("next call: %q %v", data, ok)
	}
}
