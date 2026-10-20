package nav

import (
	"sync"
	"time"
)

var (
	clockMu  sync.RWMutex
	clockNow = time.Now
)

// SetClock makes now the clock of the package's loaders (their timeouts and
// stamps); nil: the wall clock. traffic.SetTimeSource sets it with its own,
// so a test on a manual clock times out loads on that clock.
func SetClock(now func() time.Time) {
	if now == nil {
		now = time.Now
	}
	clockMu.Lock()
	clockNow = now
	clockMu.Unlock()
}

func nowTime() time.Time {
	clockMu.RLock()
	defer clockMu.RUnlock()
	return clockNow()
}
