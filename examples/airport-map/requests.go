//go:build windows

package main

import (
	"math/rand/v2"
	"sort"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Requests and clearances in radio order (#462): the crew asks when it is
// ready (TaxiEvent.Request), the controller answers once the request has
// been said and a moment has passed, the crew reads the clearance back, and
// only then, after a moment of its own, does the aircraft act. Everything
// runs on traffic time.

// The pauses: the controller's before answering, the crew's before acting
// on a clearance read back; each with up to the jitter more.
const (
	atcAnswerDelay   = 1500 * time.Millisecond
	atcAnswerJitter  = 1500 * time.Millisecond
	crewActDelay     = 2 * time.Second
	crewActJitter    = 2 * time.Second
	pendingCheckTick = time.Second
)

// pending runs actions at their traffic time.
type pending struct {
	mu  sync.Mutex
	due []pendingAct
	rng *rand.Rand
}

type pendingAct struct {
	at time.Time
	f  func()
}

func newPending() *pending {
	return &pending{rng: rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0x7e9))}
}

// later runs f at (or just after) at.
func (p *pending) later(at time.Time, f func()) {
	p.mu.Lock()
	p.due = append(p.due, pendingAct{at, f})
	p.mu.Unlock()
}

// jitter is up to d, at random.
func (p *pending) jitter(d time.Duration) time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	return time.Duration(p.rng.Int64N(int64(d) + 1))
}

// run runs what is due at now, in time order.
func (p *pending) run(now time.Time) {
	p.mu.Lock()
	var run, keep []pendingAct
	for _, a := range p.due {
		if now.Before(a.at) {
			keep = append(keep, a)
		} else {
			run = append(run, a)
		}
	}
	p.due = keep
	p.mu.Unlock()
	sort.SliceStable(run, func(i, j int) bool { return run[i].at.Before(run[j].at) })
	for _, a := range run {
		a.f()
	}
}

// clearAt is when the frequency of position pos at the aircraft's airport
// is clear: what is said on it said, readbacks included.
func (it *controlled) clearAt(pos traffic.Position) time.Time {
	_, freq := it.cc.stationOf(it.ICAO, pos)
	at := it.cc.radio.ClearAt(it.ICAO, freq)
	if now := it.cc.clock.Now(); at.Before(now) {
		return now
	}
	return at
}

// onRequest is the crew asking (TaxiEvent.Request): the call on the ground
// frequency and, unless the user is the controller, the answer in turn.
// it.mu is held.
func (it *controlled) onRequest(req string) {
	switch req {
	case "pushback":
		info := ""
		if !it.atisSaid && it.cc.atisLetter != nil {
			info, it.atisSaid = it.cc.atisLetter(it.ICAO), true
		}
		// The first call to ground: the station, the stand, the ATIS.
		station, _ := it.cc.stationOf(it.ICAO, traffic.PosGround)
		it.say(traffic.RequestPushback(station, it.Tail, it.view.Stand, info))
	case "taxi":
		it.say(traffic.RequestTaxi(it.Tail))
	default:
		return
	}
	if it.gates {
		return // the user answers
	}
	p := it.cc.pending
	p.later(it.clearAt(traffic.PosGround).Add(atcAnswerDelay+p.jitter(atcAnswerJitter)), func() { it.answer(req) })
}

// answer is the ground controller's clearance for request req, if the
// crew still asks for it; the aircraft acts once it has read it back.
func (it *controlled) answer(req string) {
	it.mu.Lock()
	if it.request != req {
		it.mu.Unlock()
		return // answered, or no longer asking
	}
	it.spoken[req] = true // said here: the state change is not said again
	it.mu.Unlock()
	tx := it.phrase(req, -1) // takes it.mu itself
	it.say(tx)
	it.actAfterReadback(traffic.PosGround, req, func() error { return it.act(req, -1) })
}

// actAfterReadback runs f in the simulator's goroutine once the clearance
// just said on pos's frequency has been read back and the crew has taken a
// moment.
func (it *controlled) actAfterReadback(pos traffic.Position, what string, f func() error) {
	p := it.cc.pending
	p.later(it.clearAt(pos).Add(crewActDelay+p.jitter(crewActJitter)), func() {
		if err := it.cc.do(f); err != nil {
			tlog.printf("%-6s %s refused: %v", it.Tail, what, err)
		}
	})
}
