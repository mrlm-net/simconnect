package world

import (
	"sort"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Controllers choose whom to call next: the clearances a controller has
// decided wait on its frequency's agenda, and whenever the frequency has
// been quiet for atcAnswerDelay the most urgent ready call goes out first.
// Urgency is the class (callPrio): safety first, then the runway (a
// landing before a take-off before a line-up), approach, the traffic in
// someone's way, and last the aircraft still on their stands and their
// departure clearances. Within a class the call that has waited longest
// goes first. A call no longer wanted when its turn comes (still false) is
// dropped. Before, each answer went out in the order it was decided: a
// landing clearance waited behind line-ups and pushbacks.
type callPrio int

const (
	prioUrgent   callPrio = iota // go around
	prioLanding                  // cleared to land
	prioRunway                   // take-off, line-up, crossing
	prioApproach                 // approach clearances
	prioClearing                 // taxi for an aircraft in the way: a vacated arrival
	prioTaxi                     // taxi for a departure
	prioStand                    // pushback, start-up
	prioDelivery                 // departure clearance
)

// call is one transmission a controller has decided on.
type call struct {
	icao, freq string
	prio       callPrio
	tail       string
	ready      time.Time   // not before: the controller's answer time
	since      time.Time   // decided at
	still      func() bool // nil: always wanted
	dropped    func()      // run when still says no
	f          func()      // says it (and has the crew act)
}

// agenda holds the calls waiting for their frequency.
type agenda struct {
	mu    sync.Mutex
	calls []call
	radio func(icao, freq string) time.Time // when the frequency is clear
}

func (a *agenda) add(c call) {
	a.mu.Lock()
	a.calls = append(a.calls, c)
	a.mu.Unlock()
}

// next takes the calls whose turn it is at now: on each frequency quiet for
// atcAnswerDelay, the most urgent ready one (the longest waiting within its
// class); and every call no longer wanted.
func (a *agenda) next(now time.Time) (run, drop []call) {
	a.mu.Lock()
	defer a.mu.Unlock()
	sort.SliceStable(a.calls, func(i, j int) bool {
		if a.calls[i].prio != a.calls[j].prio {
			return a.calls[i].prio < a.calls[j].prio
		}
		return a.calls[i].since.Before(a.calls[j].since)
	})
	busy := map[string]bool{}
	var keep []call
	for _, c := range a.calls {
		key := c.icao + " " + c.freq
		switch {
		case now.Before(c.ready) || busy[key]:
			keep = append(keep, c)
		case c.still != nil && !c.still():
			drop = append(drop, c)
		case a.radio != nil && now.Before(a.radio(c.icao, c.freq).Add(atcAnswerDelay)):
			busy[key] = true
			keep = append(keep, c)
		default:
			busy[key] = true // one call per frequency: the next once this one is said
			run = append(run, c)
		}
	}
	a.calls = keep
	return run, drop
}

// run says the calls whose turn it is at now.
func (a *agenda) run(now time.Time) {
	run, drop := a.next(now)
	for _, c := range drop {
		if c.dropped != nil {
			c.dropped()
		}
	}
	for _, c := range run {
		c.f()
	}
}

// waiting is what is on the agenda, most urgent first, for the API.
func (a *agenda) waiting() []call {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]call(nil), a.calls...)
}

// call puts a controller's transmission at pos on its frequency's agenda:
// ready after the controller's answer time, said in turn (agenda).
func (it *controlled) call(pos traffic.Position, prio callPrio, f func()) {
	it.callIf(pos, prio, nil, nil, f)
}

// callIf is call with still (nil: always wanted) checked when its turn
// comes, dropped run when it is no longer wanted.
func (it *controlled) callIf(pos traffic.Position, prio callPrio, still func() bool, dropped func(), f func()) {
	_, freq := it.cc.stationOf(it.ICAO, pos)
	now := it.cc.clock.Now()
	p := it.cc.pending
	it.cc.agenda.add(call{icao: it.ICAO, freq: freq, prio: prio, tail: it.Tail,
		ready: now.Add(atcAnswerDelay + p.jitter(atcAnswerJitter)), since: now, still: still, dropped: dropped, f: f})
}
