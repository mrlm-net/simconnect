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
	prioUrgent     callPrio = iota // go around
	prioSeparation                 // a conflict's resolution: stop descent, descend, turn
	prioTraffic                    // traffic information
	prioLanding                    // cleared to land
	prioRunway                   // take-off, line-up, crossing
	prioApproach                 // approach clearances
	prioClearing                 // taxi for an aircraft in the way: a vacated arrival
	prioTaxi                     // taxi for a departure
	prioStand                    // pushback, start-up
	prioDelivery                 // departure clearance
)

// safety: a go-around or a separation instruction, said at once: no
// answer time, the first gap on the frequency.
func (p callPrio) safety() bool { return p <= prioSeparation }

// pause is how long the frequency is quiet before a call of p: none for
// safety, atcAnswerDelay for the rest.
func (p callPrio) pause() time.Duration {
	if p.safety() {
		return 0
	}
	return atcAnswerDelay
}

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
	// urgent: until when a frequency is spoken fast after a safety call.
	urgent map[string]time.Time
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
		case a.radio != nil && now.Before(a.radio(c.icao, c.freq).Add(c.prio.pause())):
			busy[key] = true
			keep = append(keep, c)
		default:
			busy[key] = true // one call per frequency: the next once this one is said
			if c.prio.safety() {
				if a.urgent == nil {
					a.urgent = map[string]time.Time{}
				}
				a.urgent[key] = now.Add(tempoUrgentFor)
			}
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
	_, freq := it.station(pos)
	now := it.cc.clock.Now()
	p := it.cc.pending
	ready := now.Add(atcAnswerDelay + p.jitter(atcAnswerJitter))
	if prio.safety() {
		ready = now // a controller says these at once
	}
	it.cc.agenda.add(call{icao: it.ICAO, freq: freq, prio: prio, tail: it.Tail,
		ready: ready, since: now, still: still, dropped: dropped, f: f})
}

// callAt is call for an aircraft without its controller here (one of ours
// en route, a departure handed on): on icao's frequency for pos.
func (cc *controlCenter) callAt(icao string, pos traffic.Position, tail string, prio callPrio, f func()) {
	_, freq := cc.stationOf(icao, pos)
	now := cc.clock.Now()
	ready := now.Add(atcAnswerDelay + cc.pending.jitter(atcAnswerJitter))
	if prio.safety() {
		ready = now
	}
	cc.agenda.add(call{icao: icao, freq: freq, prio: prio, tail: tail, ready: ready, since: now, f: f})
}

// Talking speed: each call waiting on a frequency adds tempoPerCall to the
// pace, up to tempoMax; for tempoUrgentFor after a safety call it is at
// least tempoUrgent.
const (
	tempoPerCall   = 0.05
	tempoMax       = 1.3
	tempoUrgent    = 1.15
	tempoUrgentFor = 15 * time.Second
)

// tempo is how fast freq at icao is spoken at now (1 normal).
func (a *agenda) tempo(icao, freq string, now time.Time) float64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	n := 0
	for _, c := range a.calls {
		if c.icao == icao && c.freq == freq {
			n++
		}
	}
	t := 1 + tempoPerCall*float64(n)
	if now.Before(a.urgent[icao+" "+freq]) {
		t = max(t, tempoUrgent)
	}
	return min(t, tempoMax)
}
