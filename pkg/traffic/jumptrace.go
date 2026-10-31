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
	// jumpTraceDegPerSec: a steady turn over a long frame is no jump (a tug on
	// its 6 m arc at 9 kt turns 44°/s: 3.3° in a frame of 106 ms, live).
	jumpTraceDegPerSec = 45.0
	jumpTraceGap       = 1500 * time.Millisecond // longer between two placements: not the same run (a still aircraft is placed every 30th frame)
)

type lastSent struct {
	at       time.Time
	gap      time.Duration // since the placement before this one
	pos      airport.LatLon
	hdg, kts float64
}

// traceJump compares a placement with the last one sent (i.mu held).
func (i *Injector) traceJump(objectID uint32, o *injected, kind string, pos airport.LatLon, hdg, kts float64) {
	now := i.now()
	last := o.sent
	dt := now.Sub(last.at)
	o.sent = lastSent{at: now, gap: dt, pos: pos, hdg: hdg, kts: kts}
	if last.at.IsZero() {
		return
	}
	if dt <= 0 || dt > jumpTraceGap {
		return
	}
	d := localDist(last.pos, pos)
	allowed := jumpAllowedM(math.Max(last.kts, kts), dt, last.gap)
	turn := math.Abs(math.Remainder(hdg-last.hdg, 360))
	if d <= allowed && turn <= jumpTurnAllowed(dt) {
		return
	}
	// The simulator's own step beside ours: the mover moved by it, so a distance
	// it explains came from the simulator's clock, not from a wrong placement.
	simStep := ""
	if o.simStep > 0 {
		simStep = fmt.Sprintf(", sim step %s", time.Duration(o.simStep*float64(time.Second)).Round(time.Millisecond))
	}
	fmt.Fprintf(os.Stderr, "%s  jump trace: object %d %s %.1f m (%.1f allowed), %.1f° in %s%s at %.0f kt; %s\n",
		now.Format("15:04:05.000"), objectID, kind, d, allowed, turn, dt.Round(time.Millisecond), simStep, kts, callers())
}

// jumpTurnAllowed is how far a placement dt after the last may turn.
func jumpTurnAllowed(dt time.Duration) float64 {
	return math.Max(jumpTraceDeg, jumpTraceDegPerSec*dt.Seconds())
}

// noteSimStep tells the trace how far the simulator's clock moved objectID's
// coming placement, seconds; 0 when it stepped by ours.
func (i *Injector) noteSimStep(objectID uint32, seconds float64) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if o, ok := i.objects[objectID]; ok {
		o.simStep = seconds
	}
}

// jumpAllowedM is how far a placement dt after the last may lie at kts. Motion
// steps on simulator time and dt is the clock's: a frame delivered late and the
// next one early carry steps longer than the gap between them, so the gap before
// counts too (8.5 m in 59 ms at 135 kt is 122 ms of flight, not a jump). At least
// two frames: frames come in pairs.
func jumpAllowedM(kts float64, dt, gap time.Duration) float64 {
	return kts*ktsToMS*math.Max(math.Max(dt.Seconds(), gap.Seconds()), 2.0/60)*1.5 + jumpTraceMarginM
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
