package world

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// Arrivals and the radio over a TCAS RA (#1672), by the issue's points.

// seqArr is raArr in a sequence: it notes absorbing and holding too.
type seqArr struct {
	raArr
	holding bool
}

func (a *seqArr) Holding() (traffic.Hold, float64, bool) { return traffic.Hold{}, 0, a.holding }
func (a *seqArr) AbsorbDelay(time.Duration) (traffic.Absorption, error) {
	return traffic.Absorption{}, a.note("absorb")
}
func (a *seqArr) AbsorbWithin(time.Duration, float64) (traffic.Absorption, error) {
	return traffic.Absorption{}, a.note("absorb")
}
func (a *seqArr) LeaveHold() error           { return a.note("leave") }
func (a *seqArr) HoldAltitude(float64) error { return a.note("step down") }

// arrIn makes tail (object obj) an arrival of ours on approach's frequency.
func arrIn(b *meetingBed, id int, tail string, obj uint32) (*controlled, *seqArr) {
	arr := &seqArr{}
	it := &controlled{
		ID: id, Tail: tail, ICAO: "LKPR", cc: b.s.cc, arr: arr, objectID: obj, removed: make(chan struct{}),
		graph:  &airport.Graph{Layout: &airport.Layout{ICAO: "LKPR"}},
		spoken: map[string]bool{},
	}
	it.atc = traffic.PosApproach
	b.s.cc.items[id] = it
	return it, arr
}

// arrBed is the meeting bed's KLM221 as an arrival of ours on its STAR,
// DLH836 not ours, its RA against DLH836 begun; with ra, flown.
func arrBed(t *testing.T, ra bool) (*meetingBed, *controlled, *seqArr, *tcasWatch, func()) {
	t.Helper()
	b := newMeetingBed(t)
	clear(b.s.enroute)
	it, arr := arrIn(b, 1, "KLM221", westID)
	tw, respond := raBegins(b)
	if ra {
		respond()
		if !it.tcasRA.Load() {
			t.Fatalf("the bed has no RA flown")
		}
	}
	return b, it, arr, tw, respond
}

// added is what was given after before.
func added(before, after []string) []string { return after[len(before):] }

// 1. An arrival flying an RA isn't levelled for a conflict.
func TestOverRAArrivalConflict(t *testing.T) {
	given := func(ra bool) []string {
		b, _, arr, _, _ := arrBed(t, ra)
		q := newSequences(b.s.cc, b.s)
		b.s.st.sequences = q
		s := traffic.NewApproachSequencer("24", traffic.SequencerOptions{})
		s.Update(b.now, []traffic.ApproachAircraft{{Callsign: "KLM221", DistanceToGoNM: 60, GroundKts: 450}})
		q.seq["LKPR 24"] = s
		q.refused["KLM221"] = b.now // the level only, not the speed after it
		me, them := b.air[0].Observation, b.air[1].Observation
		them.AltFt = 33000
		b.s.cc.world.Observe(b.now.Add(time.Second), []traffic.Observation{me, them})
		before := arr.list()
		b.cw.resolveArrivals(b.now, traffic.Conflict{A: "KLM221", B: "DLH836", In: 3 * time.Minute})
		return added(before, arr.list())
	}
	if got := given(false); !slices.Equal(got, []string{"level"}) {
		t.Fatalf("no RA: given %v, want the level: nothing checked", got)
	}
	if got := given(true); len(got) > 0 {
		t.Errorf("given %v over the RA, want nothing", got)
	}
}

// 2. Nor slowed, vectored or released from a hold by its sequence; it
// keeps its place. (Its diversion and shortcut come after the same check.)
func TestOverRASequence(t *testing.T) {
	for what, c := range map[string]struct {
		holding bool
		delay   time.Duration
		want    string
	}{
		"absorb":         {false, 3 * time.Minute, "absorb"},
		"leave the hold": {true, 0, "leave"},
	} {
		t.Run(what, func(t *testing.T) {
			given := func(ra bool) []string {
				b, it, arr, _, _ := arrBed(t, ra)
				arr.holding = c.holding
				q := newSequences(b.s.cc, b.s)
				q.seenAt["LKPR KLM221"] = b.now.Add(-time.Hour) // settled
				e := traffic.SequenceEntry{Callsign: "KLM221", Number: 1, Delay: c.delay, DistanceToGoNM: 60}
				before := arr.list()
				q.absorb(b.now, "LKPR", []traffic.SequenceEntry{e}, []*controlled{it})
				return added(before, arr.list())
			}
			if got := given(false); !slices.Equal(got, []string{c.want}) {
				t.Fatalf("no RA: given %v, want %s: nothing checked", got, c.want)
			}
			if got := given(true); len(got) > 0 {
				t.Errorf("given %v over the RA, want nothing", got)
			}
		})
	}
}

// 2. One below it in a hold leaves only once it can step down.
func TestOverRAHoldAbove(t *testing.T) {
	h := traffic.Hold{Ident: "GOLOP", Fix: airport.LatLon{Lat: 50.1, Lon: 14.26}}
	type bed struct {
		b        *meetingBed
		tw       *tcasWatch
		q        *sequences
		low      *controlled
		lowA, up *seqArr
	}
	newBed := func(ra bool) bed {
		b, _, up, tw, _ := arrBed(t, ra)
		low, lowA := arrIn(b, 2, "OKLOW", eastID+1)
		q := newSequences(b.s.cc, b.s)
		q.stack("LKPR", h).Assign("OKLOW")
		q.stack("LKPR", h).Assign("KLM221")
		return bed{b, tw, q, low, lowA, up}
	}
	leave := func(c bed) (error, []string, []string) {
		lowWas, upWas := c.lowA.list(), c.up.list()
		e := traffic.SequenceEntry{Callsign: "OKLOW", Number: 1}
		err := c.q.leaveHold("LKPR", c.low, h, e, c.b.s.cc.do)
		return err, added(lowWas, c.lowA.list()), added(upWas, c.up.list())
	}
	if err, low, up := leave(newBed(false)); err != nil || !slices.Equal(low, []string{"leave"}) || !slices.Equal(up, []string{"step down"}) {
		t.Fatalf("no RA: %v, given %v and %v: nothing checked", err, low, up)
	}
	c := newBed(true)
	err, low, up := leave(c)
	if !errors.Is(err, errFlyingRA) || len(low) > 0 || len(up) > 0 {
		t.Errorf("over the RA: %v, given %v below and %v above, want it held, nothing given", err, low, up)
	}
	raOver(c.b, c.tw)
	if err, low, up := leave(c); err != nil || !slices.Equal(low, []string{"leave"}) || !slices.Equal(up, []string{"step down"}) {
		t.Errorf("clear of conflict: %v, given %v and %v, want it left and the one above down", err, low, up)
	}
}

// 3. A call decided before the crew reported the RA is said once clear.
func TestOverRACallWaits(t *testing.T) {
	b, it, _, tw, respond := arrBed(t, false)
	ag := b.s.cc.agenda
	far := b.now.Add(time.Hour)
	it.call(traffic.PosApproach, prioSpacing, func() {})
	if run, _ := ag.next(far); len(run) != 1 {
		t.Fatalf("no RA: %d said, want the call: nothing checked", len(run))
	}
	it.call(traffic.PosApproach, prioSpacing, func() {})
	respond()
	if !it.tcasRA.Load() {
		t.Fatalf("no RA flown")
	}
	if run, _ := ag.next(far); len(run) > 0 {
		t.Errorf("said over the RA")
	}
	if len(ag.waiting()) != 1 {
		t.Fatalf("waiting %d, want the call kept", len(ag.waiting()))
	}
	raOver(b, tw)
	if run, _ := ag.next(far); len(run) != 1 {
		t.Errorf("clear of conflict: %d said, want the call", len(run))
	}
}

// 5. Between the en-route list and its arrival controller TCAS still has
// it; its RA is flown once the controller does.
func TestOverRAHandoverGap(t *testing.T) {
	b := newMeetingBed(t)
	delete(b.s.enroute, "DLH836")
	b.west.f.Kind, b.west.f.Airport = "arrival", "LKPR"
	b.s.cc.world.SetOwn(westID, traffic.PhaseEnroute, "")
	if !b.s.handEnroute(b.west) {
		t.Fatalf("not handed over: nothing checked")
	}
	me, near := b.air[0].Observation, b.air[1].Observation
	lat, lon := calc.DisplaceByHeading(me.Position.Lat, me.Position.Lon, me.Heading, 3*1852)
	near.Position.Lat, near.Position.Lon = lat, lon
	b.s.cc.world.Observe(b.now.Add(time.Second), []traffic.Observation{me, near})
	air := b.s.cc.world.Aircraft()
	if !slices.ContainsFunc(air, func(a traffic.TrackedAircraft) bool { return a.ObjectID == westID && a.Ours }) {
		t.Fatalf("not ours in the picture after leaving the en-route list: TCAS doesn't see it")
	}
	tw := newTCASWatch(b.s)
	tw.tick(b.now, air)
	tw.tick(b.now.Add(time.Duration(traffic.TCASResponseDelaySec*float64(time.Second))), air)
	if v := tw.view(westID); v == nil || v.Advisory != "RA" {
		t.Fatalf("TCAS %+v, want an RA: nothing checked", v)
	}
	it, _ := arrIn(b, 1, "KLM221", westID)
	tw.tick(b.now.Add(time.Duration(traffic.TCASResponseDelaySec*float64(time.Second))+time.Second), air)
	if !it.tcasRA.Load() {
		t.Errorf("adopted by its arrival controller: the RA not flown")
	}
}
