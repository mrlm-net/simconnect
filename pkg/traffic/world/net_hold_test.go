package world

import (
	"slices"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// netBed is the meeting bed on a centre staffed as a safety net, the net
// due on KLM221.
func netBed(t *testing.T) (*meetingBed, traffic.TrackedAircraft) {
	t.Helper()
	b := newMeetingBed(t)
	b.staff(StaffedStation{Position: "center", Mode: ModeSafetyNet})
	b.cw.net = map[string]bool{"KLM221": true}
	b.w.st.conflicts = b.cw
	return b, b.air[0]
}

// Where the player cleared a level or a speed, the safety net's change of
// it ends back on exactly that, as it did: nothing held.
func TestNetChangeEndsOnPlayersClearance(t *testing.T) {
	for _, c := range []struct {
		in   EnrouteInstruction
		r    traffic.Resolution
		want func(traffic.RoutePoint) bool
	}{
		{EnrouteInstruction{AltFt: 36500}, traffic.Resolution{Kind: traffic.ResolveLevel, AltFt: 35000},
			func(p traffic.RoutePoint) bool { return p.AltFt == 36500 }},
		{EnrouteInstruction{Kts: 420}, traffic.Resolution{Kind: traffic.ResolveSpeed, Kts: 380},
			func(p traffic.RoutePoint) bool { return p.Kts == 420 }},
	} {
		b, a := netBed(t)
		c.in.Callsign = "KLM221"
		if _, err := b.s.instruct(c.in); err != nil {
			t.Fatal(err)
		}
		if _, h := b.cw.netChange(b.west, a, c.r); h != nil {
			t.Errorf("%s: held, want it to end on the player's clearance", c.r.Kind)
		}
		route := traffic.ResolvedRoute(b.west.route, a, c.r, conflictLookAhead)
		if end := route[len(route)-1]; !c.want(end) {
			t.Errorf("%s: ends on %+v, want the player's clearance", c.r.Kind, end)
		}
	}
}

// With none cleared, each change that would end is held.
func TestNetChangeHeldByKind(t *testing.T) {
	for _, r := range []traffic.Resolution{
		{Kind: traffic.ResolveLevel, AltFt: 35000},
		{Kind: traffic.ResolveSpeed, Kts: 380},
		{Kind: traffic.ResolveHeading, HeadingDeg: 120},
	} {
		b, a := netBed(t)
		route, h := b.cw.netChange(b.west, a, r)
		if h == nil {
			t.Fatalf("%s: not held", r.Kind)
		}
		end := route[len(route)-1]
		switch r.Kind {
		case traffic.ResolveLevel:
			if end.AltFt != r.AltFt {
				t.Errorf("level: ends at %.0f ft, want %.0f", end.AltFt, r.AltFt)
			}
		case traffic.ResolveSpeed:
			if end.Kts != r.Kts {
				t.Errorf("speed: ends at %.0f kt, want %.0f", end.Kts, r.Kts)
			}
		case traffic.ResolveHeading:
			if legIndex(route, h.leg) < 0 || end.Position != b.west.route[1].Position {
				t.Errorf("heading: %+v, want its leg, and the route's end kept", route)
			}
		}
	}
}

// A held heading is put on before its leg runs out, and the player's own
// heading ends it.
func TestNetHeadingHeldOn(t *testing.T) {
	b, a := netBed(t)
	route, h := b.cw.netChange(b.west, a, traffic.Resolution{Kind: traffic.ResolveHeading, HeadingDeg: 120})
	if h == nil {
		t.Fatal("not held")
	}
	b.west.route = route
	b.cw.keepNetHold("KLM221", *h)
	flight := func(x traffic.TrackedAircraft) (heldFlight, bool) { return b.cw.heldEnroute(b.west, x), true }
	near := a
	near.Position.Lat, near.Position.Lon = calc.DisplaceByHeading(h.leg.Lat, h.leg.Lon, 300, 5*1852)
	b.cw.tendNetHolds(b.now, []traffic.TrackedAircraft{near}, nil, flight)
	if got := b.sim.routes(); !slices.Equal(got, []uint32{westID}) {
		t.Fatalf("routes set %v, want the heading put on", got)
	}
	on := b.cw.netHolds["KLM221"].leg
	if on == h.leg || legIndex(b.west.route, on) < 0 {
		t.Fatalf("leg ends at %+v, want further on", on)
	}
	if _, err := b.s.instruct(EnrouteInstruction{Callsign: "KLM221", Heading: true, HeadingDeg: 90}); err != nil {
		t.Fatal(err)
	}
	b.cw.tendNetHolds(b.now, []traffic.TrackedAircraft{near}, nil, flight)
	if _, held := b.cw.netHolds["KLM221"]; held {
		t.Error("still held over the player's heading")
	}
}
