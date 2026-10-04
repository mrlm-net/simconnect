//go:build windows

package world

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
	atcAnswerDelay   = time.Second // 1 to 5 s
	atcAnswerJitter  = 4 * time.Second
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
// frequency and, unless the user is the controller, the answer in turn. A
// departure still with delivery asks once its clearance is done. it.mu is
// held.
func (it *controlled) onRequest(req string) {
	if it.dep != nil && !it.delivered {
		it.waiting = req // after the clearance and the transfer to ground
		return
	}
	switch req {
	case "pushback":
		// The first call to ground: mostly pushback and start-up in one;
		// some crews ask for the pushback first and the start-up with the
		// push under way (docs/traffic-phraseology.md, Doc 4444 12.3.4.3-4).
		info := ""
		if !it.atisSaid && it.cc.atisLetter != nil {
			info, it.atisSaid = it.cc.atisLetter(it.ICAO), true
		}
		station, _ := it.cc.stationOf(it.ICAO, traffic.PosGround)
		// Asked separately, the start-up comes once the tug has gone
		// (TaxiEvent.Request "start_up").
		it.pushAndStart = float64(it.cc.pending.jitter(time.Second)) < pushAndStartShare*float64(time.Second)
		if it.pushAndStart {
			it.say(traffic.RequestPushbackAndStartUp(station, it.Tail, it.view.Stand, info))
		} else {
			it.say(traffic.RequestPushback(station, it.Tail, it.view.Stand, info))
		}
	case "start_up":
		if it.view.State == traffic.TaxiAwaitingPushback.String() {
			// A stand it taxis out of: start-up is its first call to ground.
			info := ""
			if !it.atisSaid && it.cc.atisLetter != nil {
				info, it.atisSaid = it.cc.atisLetter(it.ICAO), true
			}
			station, _ := it.cc.stationOf(it.ICAO, traffic.PosGround)
			it.say(traffic.RequestStartUp(station, it.Tail, it.view.Stand, info))
			break
		}
		it.say(traffic.RequestStartUp("", it.Tail, "", ""))
	case "taxi":
		// Some crews ask to take the runway from an intersection (#621).
		if e := it.crewEntry(); e != "" {
			it.askedEntry = e
			it.say(traffic.RequestTaxiIntersection(it.Tail, e))
			break
		}
		it.say(traffic.RequestTaxi(it.Tail))
	default:
		return
	}
	if it.gates.Load() {
		return // the user answers
	}
	it.askGround(req)
}

// askGround puts the ground controller's answer to request req on the
// agenda: a vacated arrival's taxi before a departure's, a pushback or
// start-up last; dropped if the crew no longer asks when its turn comes.
func (it *controlled) askGround(req string) {
	prio := prioStand
	if req == "taxi" {
		prio = prioTaxi
		if it.arr != nil {
			prio = prioClearing // in the way at its runway exit
		}
	}
	it.callIf(traffic.PosGround, prio, func() bool {
		it.mu.Lock()
		defer it.mu.Unlock()
		return it.request == req
	}, nil, func() { it.answer(req) })
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
	for _, k := range impliedBy(req) {
		it.spoken[k] = true
	}
	it.mu.Unlock()
	switch req {
	case "pushback":
		it.mu.Lock()
		both := it.pushAndStart
		it.mu.Unlock()
		if both {
			it.say(traffic.WithFacing(traffic.ClearedPushbackAndStartUp(it.Tail), it.dep.PushFacingSaid()))
			it.actAfterReadback(traffic.PosGround, req, func() error {
				if err := it.act("startup", -1); err != nil {
					return err
				}
				return it.act(req, -1)
			})
			return
		}
		it.say(traffic.WithFacing(traffic.ClearedPushback(it.Tail), it.dep.PushFacingSaid()))
		it.actAfterReadback(traffic.PosGround, req, func() error { return it.act(req, -1) })
		return
	case "start_up":
		it.say(traffic.ClearedStartUp(it.Tail))
		it.actAfterReadback(traffic.PosGround, req, func() error { return it.act("startup", -1) })
		return
	}
	if req == "taxi" {
		it.grantEntry()
	}
	tx := it.phrase(req, -1) // takes it.mu itself
	it.say(tx)
	it.actAfterReadback(traffic.PosGround, req, func() error { return it.act(req, -1) })
}

// pushAndStartShare is the share of crews that ask for the pushback and
// the start-up in one call; the others ask for each on its own.
const pushAndStartShare = 0.85

// clearance is the delivery exchange of a departure, in radio order: the
// crew's request (said already), the clearance once it has been heard, the
// readback, "readback correct" and the transfer to ground; then the
// crew's waiting request, if any, to ground (#462).
func (it *controlled) clearance(clr traffic.Transmission) {
	p := it.cc.pending
	it.call(traffic.PosDelivery, prioDelivery, func() {
		it.say(clr) // read back by the crew
		p.later(it.clearAt(traffic.PosDelivery).Add(atcAnswerDelay+p.jitter(atcAnswerJitter)), func() {
			it.say(traffic.ReadbackCorrect(traffic.PosDelivery, it.Tail))
			station, freq := it.cc.stationOf(it.ICAO, traffic.PosGround)
			it.say(traffic.Handoff(it.Tail, traffic.PosDelivery, traffic.PosGround, station, freq))
			p.later(it.clearAt(traffic.PosDelivery).Add(crewActDelay+p.jitter(crewActJitter)), func() {
				it.mu.Lock()
				it.delivered, it.atc = true, traffic.PosGround
				it.view.ATC, it.view.Frequency = string(traffic.PosGround), freq
				req := it.waiting
				it.waiting = ""
				if req != "" && req == it.request {
					it.onRequest(req)
				}
				it.mu.Unlock()
			})
		})
	})
}

// firstContact is an arrival's first call to approach (said already), then
// the approach controller's clearance once it has been heard.
func (it *controlled) firstContact(clr traffic.Transmission) {
	it.call(traffic.PosApproach, prioApproach, func() { it.say(clr) })
}

// actAfterReadback runs f in the simulator's goroutine once the clearance
// just said on pos's frequency has been read back and the crew has taken a
// moment.
func (it *controlled) actAfterReadback(pos traffic.Position, what string, f func() error) {
	p := it.cc.pending
	p.later(it.clearAt(pos).Add(crewActDelay+p.jitter(crewActJitter)), func() {
		if err := it.cc.do(f); err != nil {
			it.cc.log.printf("%-6s %s refused: %v", it.Tail, what, err)
		}
	})
}
