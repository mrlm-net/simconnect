package world

// POSDIAG: the diagnostic's gap rule and write hook (not to ship).

import (
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

func TestPosDiagGap(t *testing.T) {
	var g posGap
	t0 := time.Unix(0, 0)
	for k, c := range []struct {
		at     time.Duration
		report bool
	}{{0, false}, {20 * time.Millisecond, false}, {40 * time.Millisecond, false}, {105 * time.Millisecond, true}, {125 * time.Millisecond, false}, {3200 * time.Millisecond, false}} {
		if _, _, r := g.step(t0.Add(c.at)); r != c.report {
			t.Errorf("step %d at %s: report %v", k, c.at, r)
		}
	}
}

type writeOnly struct {
	engine.Client
	defs []uint32
}

func (w *writeOnly) SetDataOnSimObject(def, obj uint32, _ types.SIMCONNECT_DATA_SET_FLAG, _, _ uint32, _ unsafe.Pointer) error {
	w.defs = append(w.defs, def)
	return nil
}

func TestPosDiagClientWrites(t *testing.T) {
	if posDiagWrap(nil, 1) != nil {
		t.Error("a nil client wrapped")
	}
	var mu sync.Mutex
	var lines []string
	posDiag.setLog(func(f string, a ...any) { mu.Lock(); lines = append(lines, f); mu.Unlock() })
	defer posDiag.setLog(nil)
	inner := &writeOnly{}
	c := posDiagWrap(inner, 7)
	for k := 0; k < 3; k++ {
		c.SetDataOnSimObject(7, 99, 0, 0, 8, nil)
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(60 * time.Millisecond)
	c.SetDataOnSimObject(7, 99, 0, 0, 8, nil)
	c.SetDataOnSimObject(8, 99, 0, 0, 8, nil) // not a position write
	if len(inner.defs) != 5 {
		t.Errorf("passed on %d writes, want 5", len(inner.defs))
	}
	mu.Lock()
	defer mu.Unlock()
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "posdiag write gap") {
		t.Errorf("logged %q", lines)
	}
}
