package traffic

import (
	"time"
)

// Moving up in a queue (a hold-short queue, any line behind traffic):
// each aircraft standing behind the one ahead starts only once the gap
// ahead has opened past its own start distance and then after its own
// reaction time, and closes up to its own spacing. Started the moment the
// leader moved, a whole queue rolled off in the same frame, in step (live
// LKPR 2026-11-07: "like on a road in a jam each should get some small
// delay").
const (
	// QueueReactMinSeconds to QueueReactMaxSeconds: how long after the gap
	// opened a crew releases the brakes.
	QueueReactMinSeconds = 1.0
	QueueReactMaxSeconds = 3.0
	// QueueStartMinMeters to QueueStartMaxMeters: how far the gap ahead
	// must have opened before it moves up (it does not creep a metre).
	QueueStartMinMeters = 0.5
	QueueStartMaxMeters = 2.0
	// QueueGapExtraMaxMeters: added to TrafficGapMeters, up to this, its own
	// spacing behind the one ahead (never closer than TrafficGapMeters).
	QueueGapExtraMaxMeters = 4.0
	// queueStandingMeters: standing within this of its traffic stop, it is
	// held behind traffic.
	queueStandingMeters = 1.0
)

// queueRipple is an aircraft's queue behaviour, drawn once per aircraft.
type queueRipple struct {
	drawn  bool
	obj    uint32
	p      queueDraw
	openAt time.Time // when the gap ahead opened past its start distance
}

type queueDraw struct{ react, start, gap float64 }

// draw is the aircraft's own reaction (s), start distance and extra gap
// (m): from its object ID and the traffic's random seed (SetRandomSeed),
// so a seeded run draws the same, without taking numbers from the shared
// stream (which would shift every draw after it).
func (q *queueRipple) draw(obj uint32) queueDraw {
	if q.drawn && q.obj == obj {
		return q.p
	}
	randomSource.mu.Lock()
	seed := randomSource.seed
	randomSource.mu.Unlock()
	x := splitmix(seed ^ uint64(obj)*0x9e3779b97f4a7c15)
	u := func() float64 { x = splitmix(x); return float64(x>>11) / (1 << 53) }
	q.p = queueDraw{
		react: QueueReactMinSeconds + u()*(QueueReactMaxSeconds-QueueReactMinSeconds),
		start: QueueStartMinMeters + u()*(QueueStartMaxMeters-QueueStartMinMeters),
		gap:   u() * QueueGapExtraMaxMeters,
	}
	q.drawn, q.obj = true, obj
	return q.p
}

func splitmix(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ x>>30) * 0xbf58476d1ce4e5b9
	x = (x ^ x>>27) * 0x94d049bb133111eb
	return x ^ x>>31
}

// hold is the traffic stop to set for an aircraft at s0 whose traffic stop
// would be stop: standing behind traffic, it stays where it is until the
// gap has opened its start distance and its reaction time has passed.
// Rolling, or not held behind traffic, stop as it is.
func (q *queueRipple) hold(obj uint32, m *GroundMover, s0, stop float64, now time.Time) float64 {
	standing := m.v == 0 && m.hasTraffic && !m.giveWay && m.trafficAt-s0 < queueStandingMeters
	if !standing || stop <= s0 {
		q.openAt = time.Time{}
		return stop
	}
	p := q.draw(obj)
	if stop-s0 < p.start {
		q.openAt = time.Time{}
		return s0 // not yet open enough to move up
	}
	if q.openAt.IsZero() {
		q.openAt = now
	}
	if now.Sub(q.openAt).Seconds() < p.react {
		return s0
	}
	q.openAt = time.Time{}
	return stop
}

// reset forgets a gap opening (giving way, or nothing ahead).
func (q *queueRipple) reset() { q.openAt = time.Time{} }
