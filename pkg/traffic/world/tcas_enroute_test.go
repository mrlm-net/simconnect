package world

import (
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// An en route aircraft of ours flying a TCAS RA (#1666). Before, it had no
// flag: the conflict watch and the player instructed it against the RA.

// raBed is the meeting bed's KLM221 alone of ours and flying an RA, against
// one of nobody's a few miles ahead, by the real TCAS watch.
func raBed(t *testing.T) (*meetingBed, *tcasWatch) {
	t.Helper()
	b := newMeetingBed(t)
	delete(b.s.enroute, "DLH836")
	b.air[1].Ours = false
	tw := newTCASWatch(b.s)
	me, near := b.air[0], b.air[1]
	lat, lon := calc.DisplaceByHeading(me.Position.Lat, me.Position.Lon, me.Heading, 3*1852)
	near.Position.Lat, near.Position.Lon = lat, lon
	tw.tick(b.now, []traffic.TrackedAircraft{me, near})
	tw.tick(b.now.Add(time.Duration(traffic.TCASResponseDelaySec*float64(time.Second))), []traffic.TrackedAircraft{me, near})
	if got := b.sim.routes(); !slices.Equal(got, []uint32{westID}) {
		t.Fatalf("routes set %v: the bed has no RA flown", got)
	}
	return b, tw
}

// raOver has the RA clear of conflict.
func raOver(b *meetingBed, tw *tcasWatch) {
	at := b.now.Add(time.Minute)
	tw.tick(at, b.air[:1])
	tw.tick(at.Add(tcasClearAfter+time.Second), b.air[:1])
}

// The conflict watch doesn't steer it, nor does the safety net; without
// the RA it does both.
func TestEnrouteRAConflictWatch(t *testing.T) {
	for _, mode := range []string{"", ModeSafetyNet} {
		t.Run("mode="+mode, func(t *testing.T) {
			watch := func(b *meetingBed) {
				if mode != "" {
					b.staff(StaffedStation{Position: "center", Mode: mode})
				}
				b.look(0)
				b.look(netGrace + conflictEvery)
			}
			plain := newMeetingBed(t)
			delete(plain.s.enroute, "DLH836")
			plain.air[1].Ours = false
			watch(plain)
			if got := plain.sim.routes(); !slices.Equal(got, []uint32{westID}) {
				t.Fatalf("no RA: routes set %v, want the watch's change: nothing checked", got)
			}
			b, _ := raBed(t)
			watch(b)
			if got := b.sim.routes(); !slices.Equal(got, []uint32{westID}) {
				t.Errorf("routes set %v, want the RA's only", got)
			}
		})
	}
}

// The player on the centre is refused, as approach is for its arrivals,
// until clear of conflict.
func TestEnrouteRAPlayer(t *testing.T) {
	for _, c := range []struct {
		what string
		in   EnrouteInstruction
	}{
		{"level", EnrouteInstruction{AltFt: 36000}},
		{"heading", EnrouteInstruction{Heading: true, HeadingDeg: 120}},
		{"speed", EnrouteInstruction{Kts: 420}},
	} {
		t.Run(c.what, func(t *testing.T) {
			b, tw := raBed(t)
			b.staff(StaffedStation{Position: "center", Mode: ModeSilent})
			c.in.Callsign = "KLM221"
			route := slices.Clone(b.west.route)
			const refused = "flying a TCAS RA: no instruction until clear of conflict"
			if _, err := b.s.instruct(c.in); err == nil || err.Error() != refused {
				t.Errorf("instructed flying an RA: %v, want %q", err, refused)
			}
			if got := b.sim.routes(); len(got) != 1 || !slices.Equal(b.west.route, route) {
				t.Errorf("routes set %v, want the RA's only", got)
			}
			raOver(b, tw)
			if _, err := b.s.instruct(c.in); err != nil {
				t.Errorf("clear of conflict: %v, want it taken", err)
			}
		})
	}
}

// The player on approach is refused for an arrival before its STAR, and
// taken once the RA has gone with its aircraft's track (#77).
func TestEnrouteRAToldArrival(t *testing.T) {
	b, tw := raBed(t)
	b.west.f.Kind, b.west.arrive = "arrival", &planned{}
	b.staff(StaffedStation{Position: "approach", Mode: ModeSilent})
	q := &sequences{cc: b.s.cc, s: b.s}
	descend := func() error {
		done, err := q.enrouteAction("LKPR", "KLM221", "descend", 0, 30000, spacing{})
		if !done {
			t.Fatal("not taken for an en route arrival: nothing checked")
		}
		return err
	}
	const refused = "flying a TCAS RA: no instruction until clear of conflict"
	if err := descend(); err == nil || err.Error() != refused {
		t.Errorf("descended flying an RA: %v, want %q", err, refused)
	}
	if got := b.sim.routes(); len(got) != 1 {
		t.Errorf("routes set %v, want the RA's only", got)
	}
	tw.tick(b.now.Add(time.Minute), nil)
	if err := descend(); err != nil {
		t.Errorf("the RA gone: %v, want it taken", err)
	}
}

// removingSim is the bed's simulator that also takes a removal.
type removingSim struct{ *routeSim }

func (removingSim) RemoveObject(uint32, uint32) error { return nil }

// Flown on virtually, it has no object to be cleared by: the player's
// instruction is taken again.
func TestEnrouteRAGoneVirtual(t *testing.T) {
	b, tw := raBed(t)
	b.staff(StaffedStation{Position: "center", Mode: ModeSilent})
	b.s.cc.sim = removingSim{b.sim}
	b.s.mgr = traffic.NewTrafficManager(b.s, traffic.ManagerOptions{})
	if !b.s.dematerialise(b.west, b.air[0], b.now) || b.west.vt == nil {
		t.Fatal("not flown on virtually: nothing checked")
	}
	tw.tick(b.now.Add(time.Minute), nil)
	if _, err := b.s.instruct(EnrouteInstruction{Callsign: "KLM221", AltFt: 36000}); err != nil {
		t.Errorf("virtual, the RA gone: %v, want it taken", err)
	}
}

// A stop of the watch's that is over is cleared on only once clear of
// conflict: not its old route in the middle of the RA.
func TestEnrouteRAStopWaits(t *testing.T) {
	b, tw := raBed(t)
	b.air = b.air[:1]
	b.cw.stopped["KLM221"] = stoppedLevel{icao: "LKPR", pos: traffic.PosCenter, altFt: 39000, climb: true, other: "GONE1", planned: slices.Clone(b.west.route)}
	b.cw.busy["KLM221"] = b.now.Add(conflictLookAhead)
	b.look(0)
	if got := b.sim.routes(); len(got) != 1 {
		t.Fatalf("routes set %v, want the RA's only", got)
	}
	raOver(b, tw)
	b.look(2 * conflictEvery)
	if got := b.sim.routes(); len(got) != 2 {
		t.Errorf("clear of conflict: routes set %v, want its route again", got)
	}
}
