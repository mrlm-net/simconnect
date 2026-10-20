package traffic

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Jump trace: a placement farther from the last one sent for the same
// aircraft than its speed explains, or turned more than jumpTraceDeg, is
// reported on stderr with who placed it. The glitch watch sees what the
// simulator shows; this tells whether we sent the jump or the simulator
// made it (docs/traffic-smoothness.md).
const (
	jumpTraceMarginM = 1.0
	jumpTraceDeg     = 3.0
	jumpTraceGap     = 1500 * time.Millisecond // longer between two placements: not the same run (a still aircraft is placed every 30th frame)
)

type lastSent struct {
	at       time.Time
	pos      airport.LatLon
	hdg, kts float64
}

// traceJump compares a placement with the last one sent (i.mu held).
func (i *Injector) traceJump(objectID uint32, o *injected, kind string, pos airport.LatLon, hdg, kts float64) {
	now := i.now()
	last := o.sent
	o.sent = lastSent{at: now, pos: pos, hdg: hdg, kts: kts}
	if last.at.IsZero() {
		return
	}
	dt := now.Sub(last.at)
	if dt <= 0 || dt > jumpTraceGap {
		return
	}
	d := localDist(last.pos, pos)
	allowed := math.Max(last.kts, kts)*ktsToMS*math.Max(dt.Seconds(), 2.0/60)*1.5 + jumpTraceMarginM // two frames: frames come in pairs
	turn := math.Abs(math.Remainder(hdg-last.hdg, 360))
	if d <= allowed && turn <= jumpTraceDeg {
		return
	}
	fmt.Fprintf(os.Stderr, "%s  jump trace: object %d %s %.1f m (%.1f allowed), %.1f° in %s at %.0f kt; %s\n",
		now.Format("15:04:05.000"), objectID, kind, d, allowed, turn, dt.Round(time.Millisecond), kts, callers())
}

// callers names the functions that led to a placement, outside the injector.
func callers() string {
	pc := make([]uintptr, 12)
	n := runtime.Callers(4, pc)
	frames := runtime.CallersFrames(pc[:n])
	var names []string
	for {
		f, more := frames.Next()
		name := f.Function[strings.LastIndex(f.Function, "/")+1:]
		names = append(names, fmt.Sprintf("%s:%d", name, f.Line))
		if !more || len(names) == 6 {
			break
		}
	}
	return strings.Join(names, " < ")
}

// Frame stats (the jump trace's other half): of the frames an injected
// aircraft was stepped by the simulator's clock, how many came a frame or
// more late (the simulator's time since the last one over 1.6 of the one
// before), reported on stderr each minute.
var frameStats struct {
	sync.Mutex
	n, late, bunched int
	lastDt  map[*groundDrive]float64
	lastAt  map[*groundDrive]time.Time
	since   time.Time
}

func countFrame(d *groundDrive, dt float64) {
	frameStats.Lock()
	defer frameStats.Unlock()
	if frameStats.lastDt == nil {
		frameStats.lastDt, frameStats.lastAt, frameStats.since = map[*groundDrive]float64{}, map[*groundDrive]time.Time{}, Now()
	}
	// Handled within 5 ms of the one before: frames that came, or were
	// handled, together.
	if at, ok := frameStats.lastAt[d]; ok && Since(at) < 5*time.Millisecond {
		frameStats.bunched++
	}
	frameStats.lastAt[d] = Now()
	if prev, ok := frameStats.lastDt[d]; ok && prev > 0 {
		frameStats.n++
		if dt > 1.6*prev {
			frameStats.late++
		}
	}
	frameStats.lastDt[d] = dt
	if Since(frameStats.since) >= time.Minute {
		pct := func(k int) float64 { return 100 * float64(k) / math.Max(1, float64(frameStats.n)) }
		fmt.Fprintf(os.Stderr, "%s  frame stats: %d frames stepped, %d late (%.1f%%), %d bunched (%.1f%%), %d aircraft\n",
			Now().Format("15:04:05"), frameStats.n, frameStats.late, pct(frameStats.late), frameStats.bunched, pct(frameStats.bunched), len(frameStats.lastDt))
		frameStats.n, frameStats.late, frameStats.bunched, frameStats.lastDt, frameStats.lastAt, frameStats.since = 0, 0, 0, map[*groundDrive]float64{}, map[*groundDrive]time.Time{}, Now()
	}
}
