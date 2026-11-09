package traffic

import (
	"fmt"
	"math"
	"time"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// The ground height request of an injected object follows how it is
// placed: every frame while it moves, a few times a second once it has
// stood still a while, once a second parked. Each request costs the
// simulator's main thread its SimVar reads on every frame it is due,
// whether or not FLAG_CHANGED sends anything; standing departures, parked
// arrivals and parked vehicles took most of them for nothing.
//
// Only objects taken over and placed slow down: one only watched (MSFS AI
// still flies it, a takeover to come) is read every frame. A placement
// that moves the object brings it back to every frame at once, in the
// same call, before the placement is sent.
const (
	// groundStillAfter: standing still this long, groundStillInterval.
	groundStillAfter = DefaultDetailSlowerAfter
	// groundStillInterval: frames skipped standing still. Not the detail's
	// StillInterval+1 (30) nor a divisor of it: in step with the
	// placements, every answer came right after one and its rest was never
	// learnt (settled needs restSettleAfter since the placement).
	groundStillInterval = 22
	// groundParkedAfter: standing still this long, once a second.
	groundParkedAfter = 30 * time.Second
	// groundRateEvery: how often the rates are looked over.
	groundRateEvery = 250 * time.Millisecond
	// groundRateGap: no injector call for this long is a pause (no frames):
	// the stillness clocks are moved on by it, so nothing moving is taken
	// for still as the frames come back.
	groundRateGap = time.Second
)

// Ground request tiers.
const (
	groundEveryFrame int8 = iota
	groundStill
	groundParked
)

// groundRate is an object's ground request rate and what it was last
// placed at.
type groundRate struct {
	tier int8
	// fast: the ground alone asked for (injDefGroundFast); asked: a
	// request since Inject's, which was every frame and full.
	fast, asked bool
	movedAt     time.Time
	last        types.SIMCONNECT_DATA_INITPOSITION
	haveLast    bool
}

// Moved within these, a placement is standing still (as Detail's).
const (
	groundMovedMeters = detailStillMeters
	groundMovedDeg    = detailStillDeg
	groundMovedFt     = 0.01
)

// notePlaced records placement p of o and reports whether it is the same
// as the last one sent, to the bit. A placement that moves o brings its
// ground request back to every frame. full: what the full ground answer
// learns from is wanted on the next frames (slow or stopped, its rest; a
// landing near the ground, its touchdown); otherwise, moving, only the
// ground under it is read (injDefGroundFast), once a full answer was had.
// i.mu held.
func (i *Injector) notePlaced(o *injected, p types.SIMCONNECT_DATA_INITPOSITION, full bool) (same bool) {
	now := i.now()
	i.noteActive(now)
	r := &o.rate
	tier := r.tier
	if !r.haveLast {
		r.movedAt = now
	} else if same = p == r.last; !same && placementMoved(r.last, p) {
		r.movedAt = now
		tier = groundEveryFrame
	}
	r.last, r.haveLast = p, true
	if tier == groundEveryFrame {
		i.setGroundRequest(o, groundEveryFrame, !full && o.haveGround)
	}
	i.groundRates(now)
	return same
}

// placementMoved reports whether b is moved from a beyond standing still.
func placementMoved(a, b types.SIMCONNECT_DATA_INITPOSITION) bool {
	const mPerDeg = 111320.0
	dn := (b.Latitude - a.Latitude) * mPerDeg
	de := (b.Longitude - a.Longitude) * mPerDeg * math.Cos(a.Latitude*math.Pi/180)
	return math.Hypot(dn, de) > groundMovedMeters ||
		math.Abs(math.Remainder(b.Heading-a.Heading, 360)) > groundMovedDeg ||
		math.Abs(b.Pitch-a.Pitch) > groundMovedDeg || math.Abs(b.Bank-a.Bank) > groundMovedDeg ||
		math.Abs(b.Altitude-a.Altitude) > groundMovedFt ||
		b.OnGround != a.OnGround || b.Airspeed != a.Airspeed
}

// noteActive moves the stillness clocks on by a pause: no injector call
// for over groundRateGap. i.mu held.
func (i *Injector) noteActive(now time.Time) {
	if gap := now.Sub(i.rateActiveAt); !i.rateActiveAt.IsZero() && gap > groundRateGap {
		for _, o := range i.objects {
			if !o.rate.movedAt.IsZero() {
				o.rate.movedAt = o.rate.movedAt.Add(gap)
			}
		}
	}
	i.rateActiveAt = now
}

// groundRates slows the ground requests of objects standing still, at most
// every groundRateEvery. i.mu held.
func (i *Injector) groundRates(now time.Time) {
	i.noteActive(now)
	if now.Sub(i.rateAt) < groundRateEvery {
		return
	}
	i.rateAt = now
	for _, o := range i.objects {
		if !o.taken || !o.rate.haveLast {
			continue
		}
		still := now.Sub(o.rate.movedAt)
		switch {
		case still >= groundParkedAfter:
			i.setGroundRequest(o, groundParked, false)
		case still >= groundStillAfter && o.rate.tier < groundStill:
			i.setGroundRequest(o, groundStill, false)
		}
	}
}

// setGroundRequest asks for o's ground height at tier's rate, the ground
// alone when fast, when either changes. i.mu held.
func (i *Injector) setGroundRequest(o *injected, tier int8, fast bool) {
	r := &o.rate
	if r.asked && r.tier == tier && r.fast == fast {
		return
	}
	if !r.asked && tier == groundEveryFrame && !fast {
		r.asked = true // Inject's own request: every frame, all of it
		return
	}
	r.asked, r.tier, r.fast = true, tier, fast
	def := i.groundDef(o)
	period, interval := types.SIMCONNECT_PERIOD_SIM_FRAME, uint32(0)
	switch tier {
	case groundStill:
		interval = groundStillInterval
	case groundParked:
		period = types.SIMCONNECT_PERIOD_SECOND
	}
	req := i.reqBase + 2*uint32(o.slot) + 1
	_ = i.track(fmt.Sprintf("ground height object %d at tier %d fast %v", o.id, tier, fast),
		i.client.RequestDataOnSimObject(req, def, o.id, period, types.SIMCONNECT_DATA_REQUEST_FLAG_CHANGED, 0, interval, 0))
}

// groundDef is the definition o's ground height is asked for with now.
func (i *Injector) groundDef(o *injected) uint32 {
	if o.rate.fast {
		return i.defBase + injDefGroundFast
	}
	return i.defBase + injDefGround
}

// strutsWanted: o rolls on the runway fast enough to learn its struts or
// their reach (struts.go: the extension off its static pitch, the
// snap-back while raised on the take-off roll or let down on the rollout):
// the full ground sample wanted. A take-off or a rollout at a time, a few
// seconds each. i.mu held.
func (i *Injector) strutsWanted(o *injected, pose ApproachPose) bool {
	return strutsKey(o.cgFt) != 0 && pose.OnGround && pose.GroundSpeedKts >= strutsLearnMinKts
}
