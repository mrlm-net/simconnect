//go:build windows
// +build windows

package simconnect

import (
	"fmt"
	"strings"
	"sync"
)

// callTrace keeps the last calls made with their send IDs (Config.TraceCalls):
// an exception names the call it was raised for. Recorded after the call
// with GetLastSentPacketID, so a call made on another goroutine in between
// can take the label.
type callTrace struct {
	mu   sync.Mutex
	ring [traceKeep]tracedCall
	next int
}

type tracedCall struct {
	id   uint32
	call string
}

// traceKeep: calls remembered; an exception arrives within a few sends.
const traceKeep = 1024

func (t *callTrace) add(id uint32, call string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ring[t.next] = tracedCall{id, call}
	t.next = (t.next + 1) % traceKeep
}

// CallFor is the call that had send ID id, with its arguments (the handle
// left out), when calls are traced and it is still remembered.
func (sc *SimConnect) CallFor(id uint32) (string, bool) {
	t := sc.trace
	if t == nil || id == 0 {
		return "", false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, c := range t.ring {
		if c.id == id && c.call != "" {
			return c.call, true
		}
	}
	return "", false
}

// traced records a call just made (the connection held).
func (sc *SimConnect) traced(name string, args []uintptr) {
	if sc.trace == nil || name == "SimConnect_GetLastSentPacketID" || name == "SimConnect_GetNextDispatch" {
		return
	}
	var id uint32
	p := sc.library.LoadProcedure("SimConnect_GetLastSentPacketID")
	if r, _, _ := p.Call(sc.connection, toUnsafePointer(&id)); !isHRESULTSuccess(r) || id == 0 {
		return
	}
	var b strings.Builder
	b.WriteString(strings.TrimPrefix(name, "SimConnect_"))
	b.WriteByte('(')
	for i, a := range args {
		if i == 0 {
			continue // the handle
		}
		if i > 1 {
			b.WriteString(", ")
		}
		if i > 4 {
			b.WriteString("…")
			break
		}
		fmt.Fprintf(&b, "%d", a)
	}
	b.WriteByte(')')
	sc.trace.add(id, b.String())
}
