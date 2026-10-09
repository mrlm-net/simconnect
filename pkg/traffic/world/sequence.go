package world

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// The landing sequence on the map (#390): an approach sequencer per airport
// and arrival runway, fed every second with our arrivals (controlled on
// their STAR and approach, or en route to it) and, respected, the other
// traffic arriving there.
//
//	GET /api/sequence?icao=LKPR — the runways and their sequences

type sequences struct {
	cc *controlCenter
	s  *scheduler

	mu   sync.Mutex
	seq  map[string]*traffic.ApproachSequencer // by "ICAO runway"
	cond map[string]traffic.ApproachConditions // the conditions last logged, by sequencer
	// absorbed: when each arrival last got a delay to absorb (#391).
	absorbed map[string]time.Time
	// refused: when approach could not resolve a conflict for an arrival
	// (not on a procedure: on the approach, or a real one off a STAR); not
	// asked again for conflictRefusedWait.
	refused map[string]time.Time
	// stacks: the holding stacks by airport and fix (#392).
	stacks map[string]*traffic.HoldStack
	// slowedFinal: when an arrival closing up on the final was told to fly
	// its final approach speed; brokeOff: sent around early for spacing.
	slowedFinal map[string]time.Time
	brokeOff    map[string]bool
	// seqSaid: the last number and speed each arrival was told, and when:
	// nothing is said again unless one changed, and the number only once
	// an approach (live, AUA529 heard "number 2" with every call).
	seqSaid map[string]seqSaid
	// shortcutAt: when each arrival was last looked at for a shortcut.
	shortcutAt map[string]time.Time
	// noShortcut: why each arrival's last shortcut did not fit, logged on change.
	noShortcut map[string]string
	// seenAt: when each arrival joined the sequence (settling).
	seenAt map[string]time.Time
	// fixesAhead: each arrival's named fixes ahead at the last tick, for
	// the merge points a shortcut keeps (#788).
	fixesAhead map[string][]traffic.FixAhead
	// conflictHeld: arrivals holding for a conflict with another (the
	// conflict watch, #455): the sequence does not release them, however
	// small their delay, before conflictHoldMin has passed and inConflict
	// no longer predicts the pair to lose separation.
	conflictHeld map[string]conflictHold
	inConflict   func(a, b string) bool
	// engaged: an aircraft in or just out of a conflict resolution gets no
	// shortcut (#785).
	engaged func(cs string, now time.Time) bool
}

type conflictHold struct {
	other string
	at    time.Time
	// manual: told by a controller to hold (#443): until released.
	manual bool
}

// conflictHoldMin is the least an arrival holds for a conflict.
const conflictHoldMin = 2 * time.Minute

// keepHolding reports that arrival cs holds for a conflict that is not
// over yet (conflictHeld); one that is over is forgotten.
func (q *sequences) keepHolding(now time.Time, cs string) bool {
	q.mu.Lock()
	h, ok := q.conflictHeld[cs]
	q.mu.Unlock()
	if !ok {
		return false
	}
	if h.manual {
		return true
	}
	if now.Sub(h.at) < conflictHoldMin || q.inConflict != nil && q.inConflict(cs, h.other) {
		return true
	}
	q.mu.Lock()
	delete(q.conflictHeld, cs)
	q.mu.Unlock()
	return false
}

func newSequences(cc *controlCenter, s *scheduler) *sequences {
	return &sequences{cc: cc, s: s, seq: map[string]*traffic.ApproachSequencer{}, cond: map[string]traffic.ApproachConditions{}, absorbed: map[string]time.Time{}, refused: map[string]time.Time{}, stacks: map[string]*traffic.HoldStack{},
		slowedFinal: map[string]time.Time{}, brokeOff: map[string]bool{}, seqSaid: map[string]seqSaid{}, shortcutAt: map[string]time.Time{}, noShortcut: map[string]string{}, seenAt: map[string]time.Time{}, conflictHeld: map[string]conflictHold{}}
}

// at is icao's landing sequences by runway.
func (q *sequences) at(icao string) map[string][]traffic.SequenceEntry {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := map[string][]traffic.SequenceEntry{}
	for k, s := range q.seq {
		if i, rwy, _ := strings.Cut(k, " "); i == icao {
			out[rwy] = s.Sequence()
		}
	}
	return out
}

// rejoin sequences an arrival at icao afresh after a go-around (#394).
func (q *sequences) rejoin(icao, tail string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	// A new approach: slowed and broken off afresh if need be.
	delete(q.slowedFinal, tail)
	delete(q.brokeOff, tail)
	delete(q.seqSaid, tail) // a new approach: its number is told again
	delete(q.absorbed, tail)
	for k, s := range q.seq {
		if i, _, _ := strings.Cut(k, " "); i == icao {
			s.Rejoin(tail)
		}
	}
}

// sequencer is the sequencer of an airport's runway, created on first use.
func (q *sequences) sequencer(icao, runway string) *traffic.ApproachSequencer {
	q.mu.Lock()
	defer q.mu.Unlock()
	k := icao + " " + runway
	if s := q.seq[k]; s != nil {
		return s
	}
	s := traffic.NewApproachSequencer(runway, traffic.SequencerOptions{MinSpacingNM: sepMinNM, OnChange: func(c traffic.SequenceChange) {
		e := c.Entry
		if e.Runway != "" {
			return // the adjacent final's: logged by its own sequence
		}
		switch {
		case c.Gone:
			q.cc.log.printf("%-6s sequence %s %s: out of the sequence", e.Callsign, icao, c.Runway)
		case c.Previous == 0:
			q.cc.log.printf("%-6s sequence %s %s: number %d%s, %.0f NM to go, delay %s", e.Callsign, icao, c.Runway, e.Number, behind(e), e.DistanceToGoNM, e.Delay.Round(time.Second))
		default:
			q.cc.log.printf("%-6s sequence %s %s: number %d (was %d)%s, delay %s", e.Callsign, icao, c.Runway, e.Number, c.Previous, behind(e), e.Delay.Round(time.Second))
		}
	}})
	q.seq[k] = s
	return s
}

func behind(e traffic.SequenceEntry) string {
	if e.Leader == "" {
		return ""
	}
	why := ""
	if e.SpacingWhy != "" {
		why = " (" + e.SpacingWhy + ")"
	}
	return fmt.Sprintf(" behind %s, %s NM%s", e.Leader, strconv.FormatFloat(math.Round(e.SpacingNM*100)/100, 'f', -1, 64), why)
}

// Delays: absorbed from 30 s on, at most every absorbEvery per arrival so
// it has slowed before its delay is looked at again.
const (
	absorbFrom  = 30 * time.Second
	absorbEvery = 90 * time.Second
)

// absorb has the arrivals of a sequence on their STAR lose their delay:
// speed, then a dog-leg; what is left waits for the hold (#392).
func (q *sequences) absorb(now time.Time, icao string, seq []traffic.SequenceEntry, items []*controlled) {
	dtg := map[string]float64{} // distance to go by call sign: the gap now
	for _, e := range seq {
		dtg[e.Callsign] = e.DistanceToGoNM
	}
	for _, e := range seq {
		if e.Runway != "" {
			continue // on the adjacent final: its own sequence handles it
		}
		var it *controlled
		for _, x := range items {
			if x.arr != nil && x.Tail == e.Callsign && x.ICAO == icao {
				it = x
				break
			}
		}
		if it == nil || it.joinPending.Load() {
			continue // VFR: told to join first (#711)
		}
		if q.settling(now, icao, e.Callsign, seq) {
			continue // its prediction is not settled yet: nothing decided on it
		}
		// Looking ahead: an established arrival (fixed, it keeps its time)
		// acts only when predicted to land short of its spacing behind its
		// leader — closing up on a slower one ahead — before they meet.
		delay := e.Delay
		if e.Fixed {
			if e.ShortBy < spacingActFrom {
				continue
			}
			delay = e.ShortBy
		} else if e.ShortBy > delay {
			delay = e.ShortBy
		}
		// Going around with the tower, not handed back yet: approach has
		// nothing to say to it (live, FTHAB told "number 3, reduce speed to
		// 210 knots" 8 s into its go-around, on the tower's frequency).
		it.mu.Lock()
		withTower := it.atc == traffic.PosTower
		it.mu.Unlock()
		if withTower && it.circuit == nil && len(it.arr.ProcedureRoute()) > 0 { // not on its final: flying the go-around
			continue
		}
		// In a hold: released once its delay is down to holdRelease.
		if h, _, holding := it.arr.Holding(); holding {
			if e.Delay <= holdRelease && !q.keepHolding(now, e.Callsign) {
				q.leaveHold(icao, it, h, e)
			}
			continue
		}
		if delay < absorbFrom && e.ShortBy < spacingActFrom {
			q.shortcut(now, it, e, seq)
			continue
		}
		// A circuit arrival is looked at again soon: its downwind is short,
		// and once on base nothing delays it any more (live, OKJZE told to
		// follow the A220 turned base 21 s into a 90 s wait, in front of it:
		// TCAS RA, the A220 went around).
		every := absorbEvery
		if it.circuit != nil {
			every = circuitAbsorbEvery
		}
		q.mu.Lock()
		recent := now.Sub(q.absorbed[e.Callsign]) < every
		q.mu.Unlock()
		if recent {
			continue
		}
		var a traffic.Absorption
		err := q.cc.do(func() (err error) { a, err = it.arr.AbsorbDelay(delay); return err })
		if errors.Is(err, traffic.ErrNotOnProcedure) && e.ShortBy >= spacingActFrom {
			lead, ok := dtg[e.Leader]
			if !ok {
				lead = -1
			}
			q.closingUp(now, it, e, e.DistanceToGoNM-lead) // on the final: slower, else around early
			continue
		}
		if errors.Is(err, traffic.ErrNotOnProcedure) || errors.Is(err, traffic.ErrHolding) {
			continue // on the final, not flying a STAR, or holding
		}
		q.mu.Lock()
		q.absorbed[e.Callsign] = now
		q.mu.Unlock()
		if err != nil {
			q.cc.log.printf("%-6s sequence: absorbing %s failed: %v", e.Callsign, delay.Round(time.Second), err)
			continue
		}
		if r := it.arr.ProcedureRoute(); len(r) > 0 {
			it.mu.Lock()
			it.approach = r // the route with its dog-leg: the distance to go
			it.mu.Unlock()
		}
		// Nothing changed (at that speed already, stretched as far as it
		// goes): nothing to say — live, CSA1389 heard "reduce speed to 210
		// knots" three times.
		if a == (traffic.Absorption{}) {
			continue
		}
		if e.ShortBy >= spacingActFrom {
			q.cc.log.printf("%-6s sequence: closing on %s, %s short of its spacing: %s", e.Callsign, e.Leader, e.ShortBy.Round(time.Second), a)
		}
		if it.circuit != nil {
			// VFR in the circuit (#569): its downwind extended, said so with
			// whom it follows (12.3.4.14 b: the slower one fitted in behind,
			// #711); no speed for a light aircraft and no hold (12.3.4.15 c).
			// Much more than a longer downwind can take: another circuit
			// (12.3.4.17 c), said alone. No orbit for spacing: orbits are
			// for emergencies (the user, live OKKKQ).
			extend := a.ExtraNM > 0 && a.Left < anotherCircuitFrom
			if extend && !it.extendSaid.Swap(true) {
				q.sayInCircuit(it, e, traffic.CircuitInstruction(e.Callsign, traffic.InstrExtendCallBase))
			}
			if a.Left >= anotherCircuitFrom {
				q.anotherCircuit(it, e, now, false)
				q.cc.log.printf("%-6s sequence: %s to lose in the circuit: another circuit", e.Callsign, a.Left.Round(time.Second))
				continue
			}
			continue
		}
		if say, n := q.sequenceCall(now, e.Callsign, e.Number, a.SpeedKts, a.Orbit != "" || a.Downwind); say {
			tx := traffic.Sequenced(e.Callsign, n, delay, a)
			// A vector due now (a dog-leg from where it is) in the same call,
			// not a second one right after (live, KLM868, #707).
			if v, ok := it.arr.VectorDue(); ok {
				tx = traffic.WithVector(tx, v, q.cc.magVar(it.ICAO))
			}
			// In radio order on approach's frequency: after its arrival
			// clearance (KLM868 heard the speed and the vector before it).
			it.call(traffic.PosApproach, prioApproach, func() { it.say(tx) })
		}
		// Too much for speed and a dog-leg: the rest in the hold.
		if a.Left >= holdFrom {
			_ = q.enterHold(now, icao, it, e, a.Left) // logged
		}
	}
}

// sayInCircuit says tx, a delay in the circuit (extend downwind, orbit),
// to a VFR arrival with its place and whom it follows the first time
// (12.3.4.14 b): "number 2, follow the Citation on short final, orbit
// right" — not the orbit alone and the place after it (live, OKWEK).
func (q *sequences) sayInCircuit(it *controlled, e traffic.SequenceEntry, tx traffic.Transmission) {
	it.call(traffic.PosTower, prioLanding, func() { q.sayInCircuitNow(it, e, tx) })
}

// sayInCircuitNow is sayInCircuit said at once (its turn on the agenda).
func (q *sequences) sayInCircuitNow(it *controlled, e traffic.SequenceEntry, tx traffic.Transmission) {
	if n, tr, lead := it.circuitPlace(); n > 1 && tr != "" && int32(n) != it.placeSaid.Load() {
		tx = traffic.Joined(traffic.FollowTraffic(e.Callsign, n, tr), tx)
		it.placeSaid.Store(int32(n))
		if lead != "" && q.cc.followed != nil {
			q.cc.followed(it.ICAO, it.Tail, lead)
		}
	}
	it.say(tx)
}

// circuitAbsorbEvery: how often a VFR circuit arrival's delay is acted on
// (absorbEvery for the others).
const circuitAbsorbEvery = 10 * time.Second

// busyFor has cs, sent on an orbit or another circuit taking d, left alone
// until it has flown it, its route (the distance to go) the new one.
func (q *sequences) busyFor(it *controlled, cs string, now time.Time, d time.Duration) {
	q.mu.Lock()
	q.absorbed[cs] = now.Add(d)
	q.mu.Unlock()
	if r := it.arr.ProcedureRoute(); len(r) > 0 {
		it.mu.Lock()
		it.approach = r
		it.mu.Unlock()
	}
}

// anotherCircuitFrom: a VFR arrival in the circuit with this much more to
// lose than its extended downwind takes makes another circuit (#569; no
// orbit for spacing).
const anotherCircuitFrom = 45 * time.Second

// Spacing on the final: an arrival predicted spacingActFrom or more short
// of its spacing acts; still breakOffFrom short breakOffAfter it was
// slowed, and more than breakOffNM out, it is sent around then, not on the
// short final.
const (
	spacingActFrom = 10 * time.Second
	breakOffFrom   = 25 * time.Second
	breakOffAfter  = 20 * time.Second
	breakOffNM     = 3.0
)

// closingUp handles an arrival on its final closing up on its leader: first
// its final approach speed from now on ("reduce to final approach speed");
// still short of its spacing once that has had time to work and inside it
// already (gapNM, the track distance to its leader now: the prediction
// alone can overstate the closing), around early.
func (q *sequences) closingUp(now time.Time, it *controlled, e traffic.SequenceEntry, gapNM float64) {
	q.mu.Lock()
	slowedAt, slowed := q.slowedFinal[e.Callsign]
	broke := q.brokeOff[e.Callsign]
	q.mu.Unlock()
	it.mu.Lock()
	pos := it.atc
	it.mu.Unlock()
	// VFR in the circuit behind an arrival established on the final:
	// another circuit while it can, else around; no speed for a light
	// aircraft (live, OKKSF on a short base ahead of AUA529 on a 5 NM
	// final) and no orbit (for emergencies only).
	if it.circuit != nil {
		if broke {
			return
		}
		if it.arr.ProcedureRoute() != nil { // still flying its circuit: round again, when told
			q.anotherCircuit(it, e, now, true)
			q.cc.log.printf("%-6s sequence: %s short behind %s: another circuit", e.Callsign, e.ShortBy.Round(time.Second), e.Leader)
		} else if err := q.cc.do(func() error { return it.arr.GoAround() }); err == nil {
			q.cc.log.printf("%-6s sequence: sent around for spacing behind %s at %.1f NM to go", e.Callsign, e.Leader, e.DistanceToGoNM)
			it.call(traffic.PosTower, prioUrgent, func() { it.say(it.goAround("spacing")) })
			q.cc.rejoin(it.ICAO, it.Tail)
		} else {
			return
		}
		q.mu.Lock()
		q.brokeOff[e.Callsign] = true
		q.mu.Unlock()
		return
	}
	if !slowed {
		var gain time.Duration
		if err := q.cc.do(func() (err error) { gain, err = it.arr.ReduceToFinalSpeed(); return err }); err != nil {
			return
		}
		q.mu.Lock()
		q.slowedFinal[e.Callsign] = now
		q.mu.Unlock()
		q.cc.log.printf("%-6s sequence: closing on %s on the final, %s short of its spacing: final approach speed gains %s", e.Callsign, e.Leader, e.ShortBy.Round(time.Second), gain.Round(time.Second))
		if gain > 0 {
			tx := traffic.SequencedFinalSpeed(pos, e.Callsign, q.numberToSay(now, e.Callsign, e.Number))
			it.call(pos, prioLanding, func() { it.say(tx) })
		}
		return
	}
	if broke || now.Sub(slowedAt) < breakOffAfter || e.ShortBy < breakOffFrom || e.DistanceToGoNM <= breakOffNM || gapNM >= e.SpacingNM {
		return
	}
	// Short of the compression buffer only: it still lands beyond the
	// minimum, the buffer is speed's to work off — around only for the
	// minimum itself (live, RYR270 sent around 6.2 NM behind OKZWR, a
	// PC-12, for the 7 NM of 5 and a 2 NM buffer).
	if e.MinimumNM > 0 && e.SpacingNM > e.MinimumNM {
		it.mu.Lock()
		kts := it.view.GroundSpeed
		it.mu.Unlock()
		if kts < 60 {
			kts = 140
		}
		landsAt := e.SpacingNM - e.ShortBy.Hours()*kts
		if landsAt >= e.MinimumNM {
			return
		}
	}
	q.mu.Lock()
	q.brokeOff[e.Callsign] = true
	q.mu.Unlock()
	if err := q.cc.do(func() error { return it.arr.GoAround() }); err != nil {
		q.cc.log.printf("%-6s sequence: go-around for spacing refused: %v", e.Callsign, err)
		return
	}
	q.cc.log.printf("%-6s sequence: sent around for spacing, %.1f NM behind %s (%.0f NM needed) at %.1f NM to go", e.Callsign, gapNM, e.Leader, e.SpacingNM, e.DistanceToGoNM)
	it.call(traffic.PosTower, prioUrgent, func() { it.say(it.goAround("spacing")) })
	q.cc.rejoin(it.ICAO, it.Tail)
}

// Holding (#392), the exception: an arrival with holdFrom or more left
// after speed and path stretching holds at the first STAR point holdFixNM
// or more from the threshold, in that fix's stack from holdBaseFt; it leaves once its delay
// is down to holdRelease, and the ones above step down.
// settleFor: an arrival new to the sequence is left alone this long. Its
// first predictions swing by minutes while it reports its first speed and
// route (live, LKPR: LOT1477 at 15 min 31 s, then 6 min 45 s seven
// seconds later, was sent to hold on the first; MRG1 told to expect 10
// minutes with 10 s left). Speed, a dog-leg or a hold is decided on the
// settled delay.
const settleFor = 30 * time.Second

// settling reports whether cs joined icao's sequence seq less than
// settleFor ago; those no longer in it are forgotten (a call sign flown
// again later settles again).
func (q *sequences) settling(now time.Time, icao, cs string, seq []traffic.SequenceEntry) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for k := range q.seenAt {
		if rest, ok := strings.CutPrefix(k, icao+" "); ok && !slices.ContainsFunc(seq, func(e traffic.SequenceEntry) bool { return e.Callsign == rest }) {
			delete(q.seenAt, k)
		}
	}
	k := icao + " " + cs
	first, ok := q.seenAt[k]
	if !ok {
		q.seenAt[k] = now
		return true
	}
	return now.Sub(first) < settleFor
}

const (
	holdFrom    = 4 * time.Minute // one racetrack: less is left to speed and vectors, asked again
	holdRelease = time.Minute
	holdFixNM   = 15.0
	holdBaseFt  = 6000.0
)

// stack is the stack at a fix, created on first use.
func (q *sequences) stack(icao string, h traffic.Hold) *traffic.HoldStack {
	q.mu.Lock()
	defer q.mu.Unlock()
	k := fmt.Sprintf("%s %.3f %.3f", icao, h.Fix.Lat, h.Fix.Lon)
	if st := q.stacks[k]; st != nil {
		return st
	}
	st := &traffic.HoldStack{Hold: h, BaseFt: holdBaseFt}
	q.stacks[k] = st
	return st
}

// enterHold sends it into the hold, its error when it did not go (#78:
// only logged, the map and the conflict watch took it as held).
func (q *sequences) enterHold(now time.Time, icao string, it *controlled, e traffic.SequenceEntry, left time.Duration) error {
	h, ok := it.arr.HoldFix(holdFixNM)
	if !ok {
		q.cc.log.printf("%-6s sequence: %s to lose, no fix to hold at", e.Callsign, left.Round(time.Second))
		return errors.New("no fix to hold at")
	}
	st := q.stack(icao, h)
	h = st.Hold // the stack's: the same racetrack for all
	alt := st.Assign(e.Callsign)
	var entry traffic.HoldEntry
	if err := q.cc.do(func() (err error) { entry, err = it.arr.EnterHold(h, alt); return err }); err != nil {
		st.Release(e.Callsign)
		q.cc.log.printf("%-6s sequence: hold failed: %v", e.Callsign, err)
		return err
	}
	if r := it.arr.ProcedureRoute(); len(r) > 0 {
		it.mu.Lock()
		it.approach = r
		it.mu.Unlock()
	}
	tx := traffic.HoldAt(e.Callsign, fixName(h), entry, alt, now.Add(left), q.cc.taOf(icao))
	it.call(traffic.PosApproach, prioApproach, func() { it.say(tx) })
	return nil
}

func (q *sequences) leaveHold(icao string, it *controlled, h traffic.Hold, e traffic.SequenceEntry) {
	if err := q.cc.do(it.arr.LeaveHold); err != nil {
		q.cc.log.printf("%-6s sequence: leaving the hold failed: %v", e.Callsign, err)
		return
	}
	tx := traffic.LeaveHoldAt(e.Callsign, fixName(h), q.numberToSay(q.cc.clock.Now(), e.Callsign, e.Number))
	it.call(traffic.PosApproach, prioApproach, func() { it.say(tx) })
	if r := it.arr.ProcedureRoute(); len(r) > 0 {
		it.mu.Lock()
		it.approach = r
		it.mu.Unlock()
	}
	// The ones above step down.
	for cs, alt := range q.stack(icao, h).Release(e.Callsign) {
		if above := q.cc.byTail(cs); above != nil && above.arr != nil {
			if err := q.cc.do(func() error { return above.arr.HoldAltitude(alt) }); err == nil {
				tx := traffic.HoldDescend(cs, alt, q.cc.taOf(above.ICAO))
				above.call(traffic.PosApproach, prioApproach, func() { above.say(tx) })
			}
		}
	}
}

// fixName is a hold's name: the STAR fix ident when it has one.
func fixName(h traffic.Hold) string {
	if h.Ident != "" {
		return h.Ident
	}
	return fmt.Sprintf("%.3f %.3f", h.Fix.Lat, h.Fix.Lon)
}

// tick feeds every sequencer with the arrivals now.
func (q *sequences) tick(now time.Time) {
	type key struct{ icao, rwy string }
	feed := map[key][]traffic.ApproachAircraft{}
	thresholds := map[key]airport.LatLon{}
	pos := map[uint32]traffic.TrackedAircraft{}
	for _, a := range q.cc.world.Aircraft() {
		pos[a.ObjectID] = a
	}
	threshold := func(icao, rwy string) (airport.LatLon, bool) {
		k := key{icao, rwy}
		if t, ok := thresholds[k]; ok {
			return t, true
		}
		g, err := q.cc.graph(icao)
		if err != nil {
			return airport.LatLon{}, false
		}
		_, end, ok := g.Layout.RunwayEnd(rwy)
		if ok {
			thresholds[k] = end.Threshold
		}
		return end.Threshold, ok
	}
	// flying: route starts at the point it flies to (DistanceVia); else it
	// is the planned route, found from where the aircraft is (DistanceToGo).
	fixesOf := map[string][]traffic.FixAhead{} // named fixes ahead: merge points (#788)
	add := func(icao, rwy, cs, model string, p airport.LatLon, kts float64, route []airport.LatLon, flying, fixed bool) {
		t, ok := threshold(icao, rwy)
		if !ok {
			return
		}
		dtg := traffic.DistanceToGo(p, route, t)
		if flying {
			dtg = traffic.DistanceVia(p, route, t)
		}
		prof := traffic.ProfileFor(model)
		feed[key{icao, rwy}] = append(feed[key{icao, rwy}], traffic.ApproachAircraft{Callsign: cs, Wake: traffic.WakeFor(model),
			DistanceToGoNM: dtg, GroundKts: kts, FinalKts: prof.Approach.ApproachKts, Fixed: fixed, Fixes: fixesOf[cs]})
	}
	// Controlled arrivals, airborne.
	q.cc.mu.Lock()
	items := make([]*controlled, 0, len(q.cc.items))
	for _, it := range q.cc.items {
		items = append(items, it)
	}
	q.cc.mu.Unlock()
	for _, it := range items {
		if it.arr == nil {
			continue
		}
		it.mu.Lock()
		v, route, id, fixes := it.view, it.approach, it.objectID, it.fixes
		it.mu.Unlock()
		if v.OnGround || v.Done || (v.State != "approaching" && v.State != "landing" && v.State != "spawning") {
			continue
		}
		p, kts := v.Position, v.GroundSpeed
		if a, ok := pos[id]; ok {
			p, kts = a.Position, a.GroundKts
		}
		if p == (airport.LatLon{}) {
			continue
		}
		// On its procedure: what it still flies (dog-legs, a go-around's
		// circuit); on the final (flown by injection, or MSFS AI without a
		// procedure): straight to the threshold — the planned approach from
		// its nearest point put TST2 2 NM further out than it was.
		if r := it.arr.ProcedureRoute(); len(r) > 0 {
			fixesOf[v.Tail] = namedAhead(p, r, fixesAhead(fixes, r))
			add(it.ICAO, v.Runway, v.Tail, v.Model, p, kts, r, true, false)
			continue
		}
		if v.State == "spawning" {
			add(it.ICAO, v.Runway, v.Tail, v.Model, p, kts, remaining(p, route), false, false)
			continue
		}
		add(it.ICAO, v.Runway, v.Tail, v.Model, p, kts, nil, true, false)
	}
	// Enroute arrivals, on their way to the STAR entry.
	q.s.mu.Lock()
	var enroute []*enrouteAC
	for _, e := range q.s.enroute {
		if e.arrive != nil && !e.handing {
			enroute = append(enroute, e)
		}
	}
	q.s.mu.Unlock()
	// The user aircraft landing, as the host's ATC cleared it (#710): in the
	// sequence, never told anything; ours fit around it.
	if p, ok := q.cc.core.playerLanding(); ok {
		cs := p.Callsign
		if cs == "" {
			cs = "Player"
		}
		for _, a := range pos {
			if a.User && !a.OnGround {
				add(p.ICAO, p.Runway, cs, p.Model, a.Position, a.GroundKts, nil, true, false)
			}
		}
	}
	for _, e := range enroute {
		a, ok := pos[e.objectID]
		if !ok {
			continue
		}
		var route []airport.LatLon
		for _, n := range e.arrive.route {
			route = append(route, n.Position)
		}
		rwy := ""
		if e.arrive.plan != nil {
			rwy = e.arrive.plan.ArrivalRunway
		}
		// The runway in use now: its plan, made at the spawn, is planned
		// again for a new runway only at the hand-over (enroute.go), and
		// was sequenced on the old one meanwhile (E29).
		if g, err := q.cc.graph(e.f.Airport); err == nil && !slices.ContainsFunc(q.cc.runwaysInUse(g, true), func(r airport.RunwayEnd) bool { return r.Name == rwy }) {
			if now := q.cc.activeRunway(g, true); now != "" {
				rwy = now
			}
		}
		var named []airFix
		for _, n := range e.arrive.route {
			if n.Ident != "" {
				named = append(named, airFix{Ident: n.Ident, LatLon: n.Position})
			}
		}
		fixesOf[e.f.Callsign] = namedAhead(a.Position, remaining(a.Position, route), named)
		add(e.f.Airport, rwy, e.f.Callsign, e.model, a.Position, a.GroundKts, route, false, false)
	}
	// Other traffic arriving, respected: it keeps its slot.
	if q.s.mgr.Options().Others == traffic.OtherRespect {
		for _, icao := range q.s.mgr.Airports() {
			g, err := q.cc.graph(icao)
			if err != nil {
				continue
			}
			rwy := q.cc.activeRunway(g, true)
			for _, a := range q.s.mgr.Others(icao) {
				if a.Phase == traffic.PhaseArriving {
					add(icao, rwy, nameOf(a), a.Title, a.Position, a.GroundKts, nil, false, true)
				}
			}
		}
	}
	// Dependent parallel approaches: each final's sequence also keeps the
	// adjacent final's arrivals, ParallelDiagonalNM away (fixed: that
	// final places them).
	q.mu.Lock()
	q.fixesAhead = fixesOf
	q.mu.Unlock()
	adjacent := map[key][]traffic.ApproachAircraft{}
	done := map[string]bool{}
	for k := range feed {
		if done[k.icao] {
			continue
		}
		done[k.icao] = true
		g, err := q.cc.graph(k.icao)
		if err != nil {
			continue
		}
		use, ok := q.cc.runwayUse(g)
		if !ok || use.Parallel != nav.ParallelDependent {
			continue
		}
		names := nav.Names(use.Arrivals)
		for _, r := range names {
			for _, o := range names {
				if o == r {
					continue
				}
				for _, a := range feed[key{k.icao, o}] {
					a.Runway, a.Fixed = o, true
					adjacent[key{k.icao, r}] = append(adjacent[key{k.icao, r}], a)
				}
			}
		}
	}
	for k, l := range adjacent {
		feed[k] = append(feed[k], l...)
	}
	// Every sequencer gets its arrivals, also none (they leave).
	q.mu.Lock()
	for k, s := range q.seq {
		i, r, _ := strings.Cut(k, " ")
		if _, ok := feed[key{i, r}]; !ok && len(s.Sequence()) > 0 {
			feed[key{i, r}] = nil
		}
	}
	q.mu.Unlock()
	var wx *nav.Weather
	if q.cc.weather != nil {
		wx = q.cc.weather()
	}
	// Departure slots: our departures waiting at a runway end (holding short
	// of it, lining up, lined up) each get a gap in its arrivals. A runway
	// for departures only has no arrivals to open one in.
	waiting := map[key]int{}
	for _, it := range items {
		it.mu.Lock()
		v := it.view
		it.mu.Unlock()
		if it.dep == nil || v.Done {
			continue
		}
		atRunway := v.State == "lining up" || v.State == "lined up" ||
			v.State == "holding short" && (v.HoldingShortOf == "" || strings.Contains(v.HoldingShortOf, v.Runway))
		// Taxiing there within departureGapLead: its gap opens now, while the
		// arrivals it goes between can still be slowed for it — not once
		// it waits at the runway and the ones close in are fixed.
		soon := v.State == "taxiing" && v.TaxiRemainingM > 0 && v.TaxiRemainingM <= departureGapLeadM
		if atRunway || soon {
			waiting[key{it.ICAO, v.Runway}]++
		}
	}
	for k, list := range feed {
		if k.rwy == "" {
			continue
		}
		s := q.sequencer(k.icao, k.rwy)
		s.SetDepartureSlots(waiting[k])
		if wx != nil {
			if g, err := q.cc.graph(k.icao); err == nil {
				if _, end, ok := g.Layout.RunwayEnd(k.rwy); ok {
					c := traffic.ConditionsFrom(*wx, end.Heading)
					s.SetConditions(c)
					// Logged when what matters for spacing changes.
					q.mu.Lock()
					last, seen := q.cond[k.icao+" "+k.rwy]
					changed := !seen || last.LowVisibility() != c.LowVisibility() || last.ReducedAllowed() != c.ReducedAllowed() ||
						last.Surface != c.Surface || math.Abs(last.HeadwindKts-c.HeadwindKts) >= 5
					if changed {
						q.cond[k.icao+" "+k.rwy] = c
					}
					q.mu.Unlock()
					if changed {
						lvp := ""
						if c.LowVisibility() {
							lvp = ", low visibility procedures"
						}
						q.cc.log.printf("sequence %s %s: %s%s", k.icao, k.rwy, c, lvp)
					}
				}
			}
		}
		q.absorb(now, k.icao, s.Update(now, list), items)
	}
}

func nameOf(a traffic.TrackedAircraft) string {
	if a.Tail != "" {
		return a.Tail
	}
	return a.Title
}

// namedAhead are the fixes on route ahead of p, nearest first, with the
// track distance to each.
func namedAhead(p airport.LatLon, route []airport.LatLon, fixes []airFix) []traffic.FixAhead {
	var out []traffic.FixAhead
	for _, f := range fixesAhead(fixes, route) {
		nm, _ := traffic.AlongTo(p, route, f.LatLon)
		out = append(out, traffic.FixAhead{Name: f.Ident, NM: nm})
	}
	slices.SortStableFunc(out, func(a, b traffic.FixAhead) int { return cmp.Compare(a.NM, b.NM) })
	return out
}

// mergePoints are e's fixes ahead an arrival landing before it on the same
// runway still has to pass: where it follows in trail, never cut past by a
// shortcut (#788).
func (q *sequences) mergePoints(e traffic.SequenceEntry, seq []traffic.SequenceEntry) []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	var keep []string
	for _, f := range q.fixesAhead[e.Callsign] {
		for _, o := range seq {
			if o.Number >= e.Number || o.Runway != e.Runway {
				continue
			}
			if slices.ContainsFunc(q.fixesAhead[o.Callsign], func(g traffic.FixAhead) bool { return g.Name == f.Name }) {
				keep = append(keep, f.Name)
				break
			}
		}
	}
	return keep
}

// remaining is the part of route still ahead of p: from the point after
// the nearest one.
func remaining(p airport.LatLon, route []airport.LatLon) []airport.LatLon {
	if len(route) == 0 {
		return nil
	}
	best, at := -1.0, 0
	for i, r := range route {
		if d := dist(p, r); best < 0 || d < best {
			best, at = d, i
		}
	}
	return route[at:]
}

func dist(a, b airport.LatLon) float64 {
	dx, dy := a.Lat-b.Lat, a.Lon-b.Lon
	return dx*dx + dy*dy
}

type sequenceView struct {
	Runway     string                     `json:"runway"`
	Conditions traffic.ApproachConditions `json:"conditions"`
	LVP        bool                       `json:"lvp"`
	Arrive     []traffic.SequenceEntry    `json:"sequence"`
}

func registerSequence(mux *http.ServeMux, st *state) {
	mux.HandleFunc("GET /api/sequence", func(w http.ResponseWriter, r *http.Request) {
		st.mu.Lock()
		q := st.sequences
		st.mu.Unlock()
		out := []sequenceView{}
		if q == nil {
			writeJSON(w, out)
			return
		}
		icao := strings.ToUpper(r.URL.Query().Get("icao"))
		q.mu.Lock()
		for k, s := range q.seq {
			if i, rwy, _ := strings.Cut(k, " "); i == icao {
				// Its own arrivals: not the adjacent final's it keeps spaced from.
				var seq []traffic.SequenceEntry
				for _, e := range s.Sequence() {
					if e.Runway == "" {
						seq = append(seq, e)
					}
				}
				if len(seq) > 0 {
					c := s.Conditions()
					out = append(out, sequenceView{Runway: rwy, Conditions: c, LVP: c.LowVisibility(), Arrive: seq})
				}
			}
		}
		q.mu.Unlock()
		sort.Slice(out, func(i, j int) bool { return out[i].Runway < out[j].Runway })
		writeJSON(w, out)
	})
}

// behind keeps tail landing after lead in icao's sequences (a VFR arrival
// told "number 2, follow …"; ApproachSequencer.Behind).
func (q *sequences) behind(icao, tail, lead string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for k, s := range q.seq {
		if i, _, _ := strings.Cut(k, " "); i == icao {
			s.Behind(tail, lead)
		}
	}
}

type seqSaid struct {
	number int
	kts    float64
	at     time.Time
}

// sequenceCall is what cs is told now of its number and speed kts (and
// notes it): nothing when neither is new (say false); the number only the
// first time and when it changes, 0 otherwise — a repeated "number N" with
// every speed annoys. orbit: a 360 is always said. A speed of 0 after one
// told keeps that one.
func (q *sequences) sequenceCall(now time.Time, cs string, number int, kts float64, orbit bool) (say bool, sayNumber int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	last, ok := q.seqSaid[cs]
	told := ok && last.number == number
	if told && !orbit && (kts == 0 || kts == last.kts) {
		return false, 0
	}
	if kts == 0 {
		kts = last.kts
	}
	q.seqSaid[cs] = seqSaid{number: number, kts: kts, at: now}
	if told {
		return true, 0
	}
	return true, number
}

// numberToSay is number when cs has not been told it yet (noting it), else 0.
func (q *sequences) numberToSay(now time.Time, cs string, number int) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	last, ok := q.seqSaid[cs]
	if ok && last.number == number {
		return 0
	}
	last.number, last.at = number, now
	q.seqSaid[cs] = last
	return number
}

// Shortcuts: an arrival with room ahead of it in the sequence — it could
// land shortcutFrom or more before it needs to behind its leader — is sent
// direct to a fix further on its STAR, using at most shortcutShare of that
// room and only where it can still descend (traffic.Shortcut); number 1
// saves up to shortcutMaxNM. Once each shortcutEvery.
const (
	shortcutFrom  = time.Minute
	shortcutShare = 0.7
	shortcutMaxNM = 15.0
	shortcutEvery = 3 * time.Minute
	shortcutFirst = 30 * time.Second
)

// shortcut squeezes it in when there is room (see the constants).
func (q *sequences) shortcut(now time.Time, it *controlled, e traffic.SequenceEntry, seq []traffic.SequenceEntry) {
	if e.Fixed || e.Delay > 0 || it.circuit != nil || it.gates.Load() {
		return
	}
	if q.engaged != nil && q.engaged(e.Callsign, now) {
		return // shortening its way would undo a resolution (#785)
	}
	if q.trafficNear(e.Callsign) {
		return // cutting across with traffic close by (live, OKGOZ direct PR517 2.6 NM from EZY131 at its level)
	}
	q.mu.Lock()
	first, seen := q.shortcutAt[e.Callsign]
	if !seen {
		// Just appeared (and settled): first looked at shortcutFirst on,
		// heading set and cleared for its STAR (live, TVS1442 sent direct
		// 2 s after it appeared, its heading still 0) — not shortcutEvery
		// on: by then the big saving is flown (live, TVS1796 and AUA1045
		// on VLM6T: direct AKEVA saves 12 NM at PR721, nothing at PR723).
		q.shortcutAt[e.Callsign] = now.Add(shortcutFirst - shortcutEvery)
	}
	recent := !seen || now.Sub(first) < shortcutEvery
	q.mu.Unlock()
	if recent {
		return
	}
	maxNM := shortcutMaxNM
	if e.Leader != "" {
		var lead *traffic.SequenceEntry
		for i := range seq {
			if seq[i].Callsign == e.Leader {
				lead = &seq[i]
			}
		}
		if lead == nil {
			return
		}
		earliest := lead.Landing.Add(traffic.SeparationTime(e.SpacingNM, 140))
		room := e.ETA.Sub(earliest)
		if room < shortcutFrom {
			return
		}
		it.mu.Lock()
		gs := it.view.GroundSpeed
		it.mu.Unlock()
		maxNM = min(maxNM, room.Hours()*max(gs, 180)*shortcutShare)
	}
	keep := q.mergePoints(e, seq)
	var fix string
	var saved float64
	err := q.cc.do(func() (err error) { fix, saved, err = it.arr.Shortcut(maxNM, keep); return err })
	q.mu.Lock()
	q.shortcutAt[e.Callsign] = now
	said := q.noShortcut[e.Callsign]
	if err != nil && err.Error() != said {
		q.noShortcut[e.Callsign] = err.Error()
	}
	q.mu.Unlock()
	if err != nil {
		// Why not, when that changes (live: TVS1796, number 1 with nothing
		// ahead, got no direct and the log said nothing).
		if errors.Is(err, traffic.ErrNoShortcut) && err.Error() != said {
			q.cc.log.printf("%-6s sequence: %v", e.Callsign, err)
		}
		return
	}
	if fix == "" {
		return
	}
	if r := it.arr.ProcedureRoute(); len(r) > 0 {
		it.mu.Lock()
		it.approach = r
		it.mu.Unlock()
	}
	q.cc.log.printf("%-6s sequence: room ahead, direct %s (%.1f NM shorter)", e.Callsign, fix, saved)
	tx := traffic.ClearedDirectTo(traffic.PosApproach, e.Callsign, fix)
	it.call(traffic.PosApproach, prioApproach, func() { it.say(tx) }) // after its STAR clearance
}

// departureGapLeadM: a departure taxiing this close to its runway (about 5
// min at taxi speed) already counts for a departure gap in the arrivals.
const departureGapLeadM = 2500.0

// forget drops what the sequence remembers of a call sign: spawned again,
// it is a new flight (#75: a reused call sign kept "broke off", a manual
// hold, the numbers said; and the maps only grew).
func (q *sequences) forget(tail string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.absorbed, tail)
	delete(q.refused, tail)
	delete(q.slowedFinal, tail)
	delete(q.brokeOff, tail)
	delete(q.seqSaid, tail)
	delete(q.shortcutAt, tail)
	delete(q.noShortcut, tail)
	delete(q.seenAt, tail)
	delete(q.fixesAhead, tail)
	delete(q.conflictHeld, tail)
}

// shortcutClearNM, shortcutClearFt: no shortcut with another aircraft
// within this of it.
const (
	shortcutClearNM = 10.0
	shortcutClearFt = 2000.0
)

// trafficNear: another aircraft within shortcutClearNM and shortcutClearFt
// of cs.
func (q *sequences) trafficNear(cs string) bool {
	air := q.cc.world.Aircraft()
	var me *traffic.TrackedAircraft
	for i := range air {
		if air[i].Tail == cs {
			me = &air[i]
		}
	}
	if me == nil {
		return false
	}
	for _, a := range air {
		if a.ObjectID == me.ObjectID || a.OnGround {
			continue
		}
		if math.Abs(a.AltFt-me.AltFt) < shortcutClearFt && calc.HaversineNM(me.Position.Lat, me.Position.Lon, a.Position.Lat, a.Position.Lon) < shortcutClearNM {
			return true
		}
	}
	return false
}

// anotherCircuit sends VFR circuit arrival it round another circuit when
// the tower says so, not before: decided now, the turn flown only once it
// is told (live, OKVML re-routed 55 s before "make another circuit" was
// said on a busy frequency). Dropped when, by its turn, it no longer flies
// its circuit or has been cleared to land (OKVML told "make another
// circuit" after "cleared to land", then landed). place: said with its
// number and whom it follows (sayInCircuit).
func (q *sequences) anotherCircuit(it *controlled, e traffic.SequenceEntry, now time.Time, place bool) {
	q.mu.Lock()
	q.absorbed[e.Callsign] = now.Add(time.Minute) // decided: not again before it is said
	q.mu.Unlock()
	still := func() bool {
		if it.arr.State() != traffic.ArrivalApproaching {
			return false
		}
		q.s.st.mu.Lock()
		tw := q.s.st.towers
		q.s.st.mu.Unlock()
		return tw == nil || !tw.landCleared(e.Callsign)
	}
	it.callIf(traffic.PosTower, prioLanding, still, nil, func() {
		var d time.Duration
		if err := q.cc.do(func() (err error) { d, err = it.arr.AnotherCircuit(); return err }); err != nil {
			q.cc.log.printf("%-6s sequence: another circuit refused: %v", e.Callsign, err)
			return
		}
		q.busyFor(it, e.Callsign, q.cc.clock.Now(), d)
		it.extendSaid.Store(false) // a new downwind
		tx := traffic.CircuitDelay(e.Callsign, traffic.DelayAnotherCircuit)
		if place {
			q.sayInCircuitNow(it, e, tx)
			return
		}
		it.say(tx)
	})
}
