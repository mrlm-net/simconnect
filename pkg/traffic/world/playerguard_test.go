package world

import (
	"errors"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

func lkprLayout(t *testing.T) *airport.Layout {
	t.Helper()
	b, err := os.ReadFile("../../airport/testdata/LKPR.json")
	if err != nil {
		t.Fatal(err)
	}
	var raw airport.RawAirport
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	l, err := airport.BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func userAt(p airport.LatLon, hdg, aglFt, kts float64, ground bool) *traffic.TrackedAircraft {
	a := &traffic.TrackedAircraft{}
	a.User, a.Position, a.Heading, a.AGLFt, a.GroundKts, a.OnGround = true, p, hdg, aglFt, kts, ground
	return a
}

// TestPlayerUsers: the user aircraft on a 3 NM final of 24, with no
// clearance from its ATC, is landing traffic on 06/24; rolling on 24 it is
// on 06/24 and on 12/30, which crosses it; holding short as cleared, in the
// queue of 24 only.
func TestPlayerUsers(t *testing.T) {
	l := lkprLayout(t)
	layout := func(string) *airport.Layout { return l }
	_, end, _ := l.RunwayEnd("24")
	lat, lon := calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, end.Heading+180, 3*1852)
	final := userAt(airport.LatLon{Lat: lat, Lon: lon}, end.Heading, 900, 140, false)
	got := playerUsers(nil, final, []string{"LKPR"}, layout)
	u := got[[2]string{"LKPR", "06/24"}]
	if len(u) != 1 || u[0].Phase != traffic.RunwayFinal || u[0].DistanceNM < 2.5 || u[0].DistanceNM > 3.5 {
		t.Fatalf("on a 3 NM final, not cleared: %+v", got)
	}
	lat, lon = calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, end.Heading, 1500)
	rolling := userAt(airport.LatLon{Lat: lat, Lon: lon}, end.Heading, 0, 90, true)
	got = playerUsers(nil, rolling, []string{"LKPR"}, layout)
	if len(got[[2]string{"LKPR", "06/24"}]) != 1 || len(got[[2]string{"LKPR", "12/30"}]) != 1 {
		t.Fatalf("rolling on 24: %+v, want on 06/24 and 12/30", got)
	}
	hs := &PlayerClearance{ICAO: "LKPR", Runway: "24", Phase: PlayerHoldingShort, Callsign: "CEF007"}
	got = playerUsers(hs, nil, []string{"LKPR"}, layout)
	if q := got[[2]string{"LKPR", "06/24"}]; len(q) != 1 || !q[0].Host || q[0].Phase != traffic.RunwayHoldingShort || len(got) != 1 {
		t.Fatalf("holding short: %+v", got)
	}
	// Cleared to hold short, but stopped mid-field on 24 (a rejected
	// take-off): where it is wins, the runway blocked.
	got = playerUsers(hs, rolling, []string{"LKPR"}, layout)
	if q := got[[2]string{"LKPR", "06/24"}]; len(q) != 1 || q[0].Phase != traffic.RunwayRolling || !q[0].Other {
		t.Fatalf("on the runway, cleared to hold short: %+v, want on it", got)
	}
}

// TestExpirePlayer: a take-off clearance ends once the user aircraft is
// airborne off the runway; a crossing once it has been on the runway and
// is off it; a landing once it is on the ground off the runway.
func TestExpirePlayer(t *testing.T) {
	l := lkprLayout(t)
	_, end, _ := l.RunwayEnd("24")
	on := end.Threshold
	lat, lon := calc.DisplaceByHeading(end.Threshold.Lat, end.Threshold.Lon, end.Heading+90, 600)
	off := airport.LatLon{Lat: lat, Lon: lon}
	k := &core{}
	k.setPlayer(PlayerClearance{ICAO: "LKPR", Runway: "24", Phase: PlayerTakeoff})
	k.expirePlayer(userAt(on, end.Heading, 0, 100, true), l)
	if _, ok := k.playerClearance(); !ok {
		t.Fatal("rolling: take-off clearance gone")
	}
	k.expirePlayer(userAt(off, end.Heading, 1500, 160, false), l)
	if _, ok := k.playerClearance(); ok {
		t.Fatal("airborne off the runway: take-off clearance kept")
	}
	k.setPlayer(PlayerClearance{ICAO: "LKPR", Runway: "24", Phase: PlayerCrossing})
	k.expirePlayer(userAt(off, 0, 0, 10, true), l)
	if _, ok := k.playerClearance(); !ok {
		t.Fatal("not yet across: crossing gone")
	}
	k.expirePlayer(userAt(on, 0, 0, 10, true), l)
	k.expirePlayer(userAt(off, 0, 0, 10, true), l)
	if _, ok := k.playerClearance(); ok {
		t.Fatal("across: crossing kept")
	}
	k.setPlayer(PlayerClearance{ICAO: "LKPR", Runway: "24", Phase: PlayerLanding})
	k.expirePlayer(userAt(off, 0, 0, 15, true), l)
	if _, ok := k.playerClearance(); ok {
		t.Fatal("landed and off the runway: landing clearance kept")
	}
	// Too old: gone whatever the user aircraft does.
	k.setPlayer(PlayerClearance{ICAO: "LKPR", Runway: "24", Phase: PlayerLineUp})
	k.player.since = time.Now().Add(-playerRunwayStale - time.Minute)
	k.expirePlayer(nil, l)
	if _, ok := k.playerClearance(); ok {
		t.Fatal("stale line-up kept")
	}
}

// TestPlayerBlocks: cleared to take off on 24, ours are kept off 06/24 and
// off 12/30, which crosses it.
func TestPlayerBlocks(t *testing.T) {
	l := lkprLayout(t)
	k := &core{}
	k.setPlayer(PlayerClearance{ICAO: "LKPR", Runway: "24", Phase: PlayerTakeoff})
	if !k.playerBlocks("LKPR", "06/24", l) || !k.playerBlocks("LKPR", "12/30", l) {
		t.Errorf("take-off on 24: blocks 06/24 %v, 12/30 %v", k.playerBlocks("LKPR", "06/24", l), k.playerBlocks("LKPR", "12/30", l))
	}
	k.setPlayer(PlayerClearance{ICAO: "LKPR", Runway: "24", Phase: PlayerHoldingShort})
	if k.playerBlocks("LKPR", "06/24", l) {
		t.Error("holding short blocks the runway")
	}
}

// TestStandIndexAmbiguous: of two spots with one label, the one nearest the
// user aircraft; without the aircraft, the ambiguity.
func TestStandIndexAmbiguous(t *testing.T) {
	l := &airport.Layout{Parking: []airport.Parking{
		{Index: 0, Number: 5, Position: airport.LatLon{Lat: 50.10, Lon: 14.26}},
		{Index: 1, Number: 5, Position: airport.LatLon{Lat: 50.11, Lon: 14.27}},
	}}
	label := l.Parking[0].Label()
	ua := userAt(airport.LatLon{Lat: 50.1101, Lon: 14.2699}, 0, 0, 0, true)
	if i, err := standIndex(l, label, ua); err != nil || i != 1 {
		t.Errorf("near the second: %d %v", i, err)
	}
	if _, err := standIndex(l, label, nil); !errors.Is(err, airport.ErrAmbiguousParking) {
		t.Errorf("without the aircraft: %v, want ambiguous", err)
	}
}
