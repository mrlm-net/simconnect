package engine

import (
	"testing"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// TestDetachOutlivesRelease: a detached message keeps its content after the
// pooled buffer is released and written over (#404).
func TestDetachOutlivesRelease(t *testing.T) {
	ev := types.SIMCONNECT_RECV_EVENT{UEventID: 42, DwData: 7}
	ev.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_EVENT)
	size := int(unsafe.Sizeof(ev))
	buf := make([]byte, size)
	copy(buf, unsafe.Slice((*byte)(unsafe.Pointer(&ev)), size))
	released := false
	msg := newMessage((*types.SIMCONNECT_RECV)(unsafe.Pointer(&buf[0])), uint32(size), nil, buf, func() { released = true })

	d := msg.Detach()
	msg.Release()
	for i := range buf {
		buf[i] = 0xff // the pool hands the buffer to the next message
	}
	if !released {
		t.Fatal("the original was not released")
	}
	if got := d.AsEvent(); got == nil || got.UEventID != 42 || got.DwData != 7 {
		t.Fatalf("detached event %+v, want id 42 data 7", got)
	}
	d.Release() // no-op: owns its buffer
}

// An error Message has no SIMCONNECT_RECV: the As helpers say nil, not panic.
func TestErrorMessageAsHelpers(t *testing.T) {
	m := Message{Err: ErrConnectionLost}
	if m.AsEvent() != nil || m.AsOpen() != nil || m.AsException() != nil || m.AsFacilityList() != nil ||
		m.AsEnumerateInputEvents() != nil || m.AsClientData() != nil || m.AsCommBus() != nil {
		t.Error("an As helper returned a value for an error message")
	}
	if _, ok := m.AsCameraData(); ok {
		t.Error("AsCameraData read an error message")
	}
	if CastAs[*types.SIMCONNECT_RECV_EVENT](&m) != nil {
		t.Error("CastAs returned a value for an error message")
	}
}

// Copies of a Message return the buffer once, whichever calls Release.
func TestReleaseOnceAcrossCopies(t *testing.T) {
	n := 0
	m := newMessage(nil, 0, nil, nil, func() { n++ })
	c := m
	m.Release()
	c.Release()
	m.Release()
	if n != 1 {
		t.Errorf("released %d times", n)
	}
}

func TestPooledSliceReuse(t *testing.T) {
	b, release := getPooledSlice(100)
	if len(b) != 100 || cap(b) != 4*1024 {
		t.Fatalf("len %d cap %d", len(b), cap(b))
	}
	release()
	if b, _ := getPooledSlice(70 * 1024); len(b) != 70*1024 {
		t.Errorf("large slice len %d", len(b))
	}
}
