package worldtest

import (
	"math"
	"testing"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// jumpBed places one vehicle on a loopback on a manual clock, frame by
// frame, as the World's injector does: an INITPOSITION, and VELOCITY BODY Z
// after it while it travels backwards.
type jumpBed struct {
	l   *Loopback
	now time.Time
	id  uint32
}

const (
	jbDefPos = 1
	jbDefVz  = 2
	jbLat    = 50.1
	jbLon    = 14.26
)

func newJumpBed(t *testing.T, hdg float64) *jumpBed {
	t.Helper()
	b := &jumpBed{l: NewLoopback(jbLat, jbLon, 1000, 64), now: time.Date(2026, 10, 20, 6, 0, 0, 0, time.UTC)}
	b.l.clock = func() time.Time { return b.now }
	b.l.AddToDataDefinition(jbDefPos, "Initial Position", "", types.SIMCONNECT_DATATYPE_INITPOSITION, 0, 0)
	b.l.AddToDataDefinition(jbDefVz, "VELOCITY BODY Z", "meters per second", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0)
	if err := b.l.AICreateSimulatedObject("tug", types.SIMCONNECT_DATA_INITPOSITION{Latitude: jbLat, Longitude: jbLon, Heading: hdg, OnGround: 1}, 1); err != nil {
		t.Fatal(err)
	}
	b.id = b.l.nextID
	return b
}

// put places it n metres north and e east of the bed's origin, after dt.
// back: travelling backwards (Airspeed 0, minus kts in VELOCITY BODY Z).
func (b *jumpBed) put(dt time.Duration, n, e, hdg, kts float64, back bool) {
	b.now = b.now.Add(dt)
	p := types.SIMCONNECT_DATA_INITPOSITION{
		Latitude:  jbLat + n/metresPerDeg,
		Longitude: jbLon + e/(metresPerDeg*math.Cos(jbLat*math.Pi/180)),
		Heading:   math.Mod(hdg+360, 360),
		OnGround:  1,
		Airspeed:  types.SIMCONNECT_DATA_INITPOSITION_AIRSPEED(math.Round(kts)),
	}
	if back {
		p.Airspeed = 0
	}
	b.l.SetDataOnSimObject(jbDefPos, b.id, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, uint32(unsafe.Sizeof(p)), unsafe.Pointer(&p))
	if back {
		vz := -kts * 0.514444
		b.l.SetDataOnSimObject(jbDefVz, b.id, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, 8, unsafe.Pointer(&vz))
	}
}

// straight drives it along heading hdg at kts for frames of dt from s
// metres along, offset off metres to the right; it returns where it ended.
func (b *jumpBed) straight(frames int, dt time.Duration, s, off, hdg, kts float64, back bool) float64 {
	r := hdg * math.Pi / 180
	dir := 1.0
	if back {
		dir = -1
	}
	for range frames {
		s += dir * kts * 0.514444 * dt.Seconds()
		b.put(dt, s*math.Cos(r)-off*math.Sin(r), s*math.Sin(r)+off*math.Cos(r), hdg, kts, back)
	}
	return s
}

// TestLoopbackJumpsCoarseFrames: on 5 Hz frames (the overflight bed's) the
// loopback counts the steps and snaps the jump trace reports live, not a
// steady tight turn sampled coarsely nor a push backwards at its speed (a
// tug's way off a nose: 81 such at LKPR, 206 at EGLL in 3 h, none at 60 Hz).
func TestLoopbackJumpsCoarseFrames(t *testing.T) {
	const frame = 200 * time.Millisecond
	t.Run("steady turn 90°/s at 9 kt", func(t *testing.T) {
		b := newJumpBed(t, 0)
		const kts, rate = 9.0, 90.0 // °/s
		r := kts * 0.514444 / (rate * math.Pi / 180)
		for k := range 20 { // 4 s: a full turn
			a := rate * frame.Seconds() * float64(k) * math.Pi / 180
			// Round a centre r east of the origin, heading along the circle.
			b.put(frame, r*math.Sin(a), r-r*math.Cos(a), float64(rate)*frame.Seconds()*float64(k), kts, false)
		}
		if js := b.l.Jumps(); len(js) != 0 {
			t.Errorf("a steady turn counted %d jumps, the first %.1f m %.1f° in %v", len(js), js[0].Metres, js[0].TurnDeg, js[0].Dt)
		}
	})
	t.Run("backwards at 12 kt", func(t *testing.T) {
		b := newJumpBed(t, 90)
		b.straight(15, frame, 0, 0, 90, 12, true)
		if js := b.l.Jumps(); len(js) != 0 {
			t.Errorf("a push at its speed counted %d jumps, the first %.1f m in %v at %.0f kt", len(js), js[0].Metres, js[0].Dt, js[0].Kts)
		}
	})
	t.Run("a 5 m step aside", func(t *testing.T) {
		b := newJumpBed(t, 90)
		s := b.straight(10, frame, 0, 0, 90, 9, false)
		b.straight(10, frame, s, 5, 90, 9, false)
		if js := b.l.Jumps(); len(js) != 1 {
			t.Errorf("a 5 m step aside counted %d jumps, want 1", len(js))
		}
	})
	t.Run("a 20° snap", func(t *testing.T) {
		b := newJumpBed(t, 90)
		s := b.straight(10, frame, 0, 0, 90, 9, false)
		b.straight(10, frame, s, 0, 110, 9, false) // heading snapped, the way on
		if js := b.l.Jumps(); len(js) != 1 {
			t.Errorf("a 20° snap counted %d jumps, want 1", len(js))
		}
	})
	t.Run("a 5 m step aside, last placement", func(t *testing.T) {
		b := newJumpBed(t, 90)
		s := b.straight(10, frame, 0, 0, 90, 9, false)
		b.straight(1, frame, s, 5, 90, 9, false)
		if js := b.l.Jumps(); len(js) != 1 {
			t.Errorf("a step on the last placement counted %d jumps, want 1", len(js))
		}
	})
	t.Run("live frames: a 4° snap", func(t *testing.T) {
		b := newJumpBed(t, 90)
		live := time.Second / 60
		s := b.straight(30, live, 0, 0, 90, 9, false)
		b.straight(30, live, s, 0, 94, 9, false)
		if js := b.l.Jumps(); len(js) != 1 {
			t.Errorf("a 4° snap at 60 Hz counted %d jumps, want 1", len(js))
		}
	})
}
