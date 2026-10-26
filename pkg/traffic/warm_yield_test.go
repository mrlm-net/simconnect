package traffic

import (
	"testing"
	"time"
)

// TestWarmTaxiInLetsTheTickRun: LFPG's taxi-ins warmed in the background
// while a tick runs every 10 ms: the tick still runs. On a host with one
// thread and no preemption (WASM: GOOS=js GOARCH=wasm go test) the warm-up
// held the thread to its end before, and the first spawn waited 2 min for
// it (#1233).
func TestWarmTaxiInLetsTheTickRun(t *testing.T) {
	if testing.Short() {
		t.Skip("warms LFPG")
	}
	g := airportGraph(t, "LFPG")
	alloc := NewStandAllocator(nil, g)
	done := make(chan struct{})
	go func() {
		alloc.WarmTaxiIn([]string{"27L", "26L"})
		close(done)
	}()
	var worst time.Duration
	last := time.Now()
	for range 100 {
		select {
		case <-done:
		default:
			time.Sleep(10 * time.Millisecond)
			worst = max(worst, time.Since(last))
			last = time.Now()
			continue
		}
		break
	}
	t.Logf("longest tick gap %v", worst)
	if worst > time.Second {
		t.Errorf("the tick waited %v for the warm-up", worst)
	}
}
