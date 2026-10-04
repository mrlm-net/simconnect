//go:build windows
// +build windows

package engine

import (
	"strings"
	"sync"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// SubscribeToCommBusEvent subscribes to CommBus event eventName: each call
// of it arrives on Stream as a SIMCONNECT_RECV_COMM_BUS message with eventID
// (AsCommBus, CommBusData; CommBusAssembler joins one sent in several
// messages). MSFS 2024 only.
func (e *Engine) SubscribeToCommBusEvent(eventID uint32, eventName string) error {
	return e.api.SubscribeToCommBusEvent(eventID, eventName)
}

// UnsubscribeToCommBusEvent ends the subscription with eventID. MSFS 2024 only.
func (e *Engine) UnsubscribeToCommBusEvent(eventID uint32) error {
	return e.api.UnsubscribeToCommBusEvent(eventID)
}

// CallCommBusEvent calls CommBus event eventName with data (a string,
// usually JSON) for those in broadcastTo
// (types.SIMCONNECT_COMM_BUS_BROADCAST_TO_DEFAULT: JavaScript, WebAssembly
// and other clients). MSFS 2024 only.
func (e *Engine) CallCommBusEvent(eventName string, broadcastTo types.SIMCONNECT_COMM_BUS_BROADCAST_TO, data string) error {
	return e.api.CallCommBusEvent(eventName, broadcastTo, data)
}

// AsCommBus casts the message to SIMCONNECT_RECV_COMM_BUS; nil if it is not
// one. MSFS 2024 only.
func (m *Message) AsCommBus() *types.SIMCONNECT_RECV_COMM_BUS {
	if m.SIMCONNECT_RECV == nil || types.SIMCONNECT_RECV_ID(m.DwID) != types.SIMCONNECT_RECV_ID_COMM_BUS {
		return nil
	}
	return (*types.SIMCONNECT_RECV_COMM_BUS)(unsafe.Pointer(m.SIMCONNECT_RECV))
}

// CommBusData is the data a CommBus message carries (this message's part
// of it when sent in several): the string after its header, up to the
// message's end or a NUL.
func (m *Message) CommBusData() string {
	cb := m.AsCommBus()
	if cb == nil {
		return ""
	}
	head := uint32(unsafe.Sizeof(*cb))
	size := uint32(cb.DwSize)
	if m.Size != 0 && m.Size < size {
		size = m.Size
	}
	if size <= head {
		return ""
	}
	b := unsafe.Slice((*byte)(unsafe.Add(unsafe.Pointer(cb), head)), size-head)
	if i := strings.IndexByte(string(b), 0); i >= 0 {
		b = b[:i]
	}
	return string(b)
}

// CommBusAssembler joins CommBus data sent in several messages
// (DwEntryNumber of DwOutOf) back into one, per event ID.
type CommBusAssembler struct {
	mu    sync.Mutex
	parts map[uint32][]string
}

// Add takes a message: when it completes a CommBus call (the last part, or
// the only one) it returns the event ID and the whole data, true.
func (a *CommBusAssembler) Add(m *Message) (eventID uint32, data string, ok bool) {
	cb := m.AsCommBus()
	if cb == nil {
		return 0, "", false
	}
	id, part := uint32(cb.UEventID), m.CommBusData()
	n, of := int(cb.DwEntryNumber), int(cb.DwOutOf)
	if of <= 1 {
		return id, part, true
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.parts == nil {
		a.parts = map[uint32][]string{}
	}
	p := a.parts[id]
	if len(p) != of || n == 0 {
		p = make([]string, of) // a new call: its first part, or another length
	}
	if n >= 0 && n < of {
		p[n] = part
	}
	a.parts[id] = p
	if n != of-1 {
		return 0, "", false
	}
	delete(a.parts, id)
	return id, strings.Join(p, ""), true
}
