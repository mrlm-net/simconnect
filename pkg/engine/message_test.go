//go:build windows

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
