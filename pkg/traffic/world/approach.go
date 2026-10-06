package world

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Approach control on the map (#396): the controller's actions on an
// arrival in the landing sequence.
//
//	POST /api/approach/{icao}/{callsign}/{action}
//
// up, down: a place earlier or later in the landing order (kept);
// hold: hold now at the STAR's hold fix, on the stack; release: leave the
// hold; slow: lose another minute by speed and stretching; direct: direct
// to the join point on the final; goaround: go around.

// approachSlowBy is what one "slow" asks the arrival to lose.
const approachSlowBy = time.Minute

// errNothingToSlow: an arrival told to lose time flies as slow and as long
// a way as it can already (AbsorbDelay changed nothing).
var errNothingToSlow = errors.New("nothing more to slow")

// entryOf is callsign's sequencer at icao and its entry in the sequence.
func (q *sequences) entryOf(icao, callsign string) (*traffic.ApproachSequencer, traffic.SequenceEntry, bool) {
	q.mu.Lock()
	var seqs []*traffic.ApproachSequencer
	for k, s := range q.seq {
		if i, _, _ := strings.Cut(k, " "); i == icao {
			seqs = append(seqs, s)
		}
	}
	q.mu.Unlock()
	for _, s := range seqs {
		for _, e := range s.Sequence() {
			if e.Callsign == callsign {
				return s, e, true
			}
		}
	}
	return nil, traffic.SequenceEntry{}, false
}

// approachAction does action for callsign at icao, and says it as ATC.
func (q *sequences) approachAction(icao, callsign, action string) error {
	s, e, ok := q.entryOf(icao, callsign)
	if !ok {
		return traffic.ErrNotSequenced
	}
	switch action {
	case "up", "down":
		places := -1
		if action == "down" {
			places = 1
		}
		if err := s.Move(callsign, places); err != nil {
			return err
		}
		q.cc.log.printf("%-6s approach: moved %s in the sequence to %s", callsign, action, s.Runway())
		return nil
	}
	it := q.cc.byTail(callsign)
	if it == nil || it.arr == nil {
		return errors.New("not one of our arrivals")
	}
	switch action {
	case "hold":
		if _, _, holding := it.arr.Holding(); holding {
			return traffic.ErrHolding
		}
		q.enterHold(q.cc.clock.Now(), icao, it, e, max(e.Delay, 2*time.Minute))
	case "release":
		h, _, holding := it.arr.Holding()
		if !holding {
			return traffic.ErrNotHolding
		}
		q.leaveHold(icao, it, h, e)
	case "slow":
		var a traffic.Absorption
		if err := q.cc.do(func() (err error) { a, err = it.arr.AbsorbDelay(approachSlowBy); return err }); err != nil {
			return err
		}
		if a == (traffic.Absorption{}) {
			return errNothingToSlow // as slow and as long as it goes already
		}
		// The number once, as the sequence says it (live, AFR850 heard
		// "number 4" from a conflict that only stretched its route).
		if say, n := q.sequenceCall(q.cc.clock.Now(), callsign, e.Number, a.SpeedKts, a.Orbit != ""); say && (n > 0 || a.SpeedKts > 0 || a.Orbit != "") {
			tx := traffic.Sequenced(callsign, n, 0, a)
			// In radio order: after its arrival clearance (live, TVS554 told to
			// slow 21 s before it was cleared its STAR).
			it.call(traffic.PosApproach, prioApproach, func() { it.say(tx) })
		}
	case "direct":
		if err := q.cc.do(it.arr.DirectToJoin); err != nil {
			return err
		}
		it.say(traffic.DirectToFinal(callsign, q.numberToSay(q.cc.clock.Now(), callsign, e.Number)))
	case "goaround":
		// On the connection's goroutine, as every SimConnect call.
		if err := q.cc.do(func() error { return it.act("goaround", 0) }); err != nil {
			return err
		}
		it.say(traffic.GoAround(callsign, ""))
		q.s.st.mu.Lock()
		tw := q.s.st.towers
		q.s.st.mu.Unlock()
		if tw != nil {
			tw.forgetLanding(callsign) // a new approach, a new landing clearance (#486)
		}
	default:
		return errors.New("unknown action " + action)
	}
	if r := it.arr.ProcedureRoute(); len(r) > 0 {
		it.mu.Lock()
		it.approach = r
		it.mu.Unlock()
	}
	return nil
}

func registerApproach(mux *http.ServeMux, st *state) {
	mux.HandleFunc("POST /api/approach/{icao}/{callsign}/{action}", func(w http.ResponseWriter, r *http.Request) {
		st.mu.Lock()
		q := st.sequences
		st.mu.Unlock()
		if q == nil {
			http.Error(w, "not connected", http.StatusServiceUnavailable)
			return
		}
		// Network play (#511): only the position working it.
		if as := positionOf(r); q.cc != nil {
			if it := q.cc.byTail(r.PathValue("callsign")); it != nil && !mayClear(as, it) {
				http.Error(w, it.Tail+" is not on your frequency ("+as+")", http.StatusForbidden)
				return
			}
		}
		err := q.approachAction(strings.ToUpper(r.PathValue("icao")), r.PathValue("callsign"), r.PathValue("action"))
		switch {
		case errors.Is(err, traffic.ErrNotSequenced):
			http.Error(w, err.Error(), http.StatusNotFound)
		case err != nil:
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	})
}
