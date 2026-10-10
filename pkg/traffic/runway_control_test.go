package traffic

import (
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/calc"
)

func dep(cs, typ string, p RunwayPhase) RunwayUser {
	return RunwayUser{Callsign: cs, Wake: WakeFor(typ), Phase: p, Route: "VENO7D"}
}

func final(cs string, nm float64) RunwayUser {
	return RunwayUser{Callsign: cs, Wake: WakeFor("A320"), Phase: RunwayFinal, Arrival: true, DistanceNM: nm, GroundKts: 140, Established: true}
}

func TestRunwayControllerFreeRunway(t *testing.T) {
	r := NewRunwayController(RunwayControllerOptions{})
	c := r.Decide(time.Now(), []RunwayUser{dep("CSA1", "A320", RunwayHoldingShort)})
	if !slices.Contains(c.LineUp, "CSA1") || !slices.Contains(c.Takeoff, "CSA1") {
		t.Fatalf("a free runway: %+v", c)
	}
}

func TestRunwayControllerArrivals(t *testing.T) {
	r := NewRunwayController(RunwayControllerOptions{})
	now := time.Now()
	// An arrival on a 3 NM final: the departure waits, and so does a crossing at 2 NM.
	c := r.Decide(now, []RunwayUser{dep("CSA1", "A320", RunwayHoldingShort), final("DLH2", 3)})
	if len(c.LineUp)+len(c.Takeoff) != 0 || !strings.Contains(c.Waiting["CSA1"], "DLH2") {
		t.Fatalf("departure ahead of an arrival at 3 NM: %+v", c)
	}
	x := RunwayUser{Callsign: "TVS3", Phase: RunwayHoldingShort, Crossing: true}
	if c := r.Decide(now, []RunwayUser{x, final("DLH2", 1.5)}); len(c.Cross) != 0 {
		t.Fatal("crossing ahead of an arrival at 1.5 NM")
	}
	// At 8 NM: the gap is big enough: line up and go.
	c = r.Decide(now, []RunwayUser{dep("CSA1", "A320", RunwayHoldingShort), final("DLH2", 8)})
	if !slices.Contains(c.Takeoff, "CSA1") {
		t.Fatalf("departure in an 8 NM gap: %+v", c)
	}
	// Landing roll: the runway is occupied.
	roll := final("DLH2", 0)
	roll.Phase = RunwayRolling
	c = r.Decide(now, []RunwayUser{dep("CSA4", "A320", RunwayHoldingShort), roll})
	if len(c.LineUp) != 0 || !strings.Contains(c.Waiting["CSA4"], "on the runway") {
		t.Fatalf("with an arrival on its landing roll: %+v", c)
	}
}

func TestRunwayControllerInterval(t *testing.T) {
	r := NewRunwayController(RunwayControllerOptions{})
	now := time.Now()
	// A heavy rolls; an A320 behind it lines up and waits 2 min.
	r.Decide(now, []RunwayUser{dep("QTR1", "B77W", RunwayRolling), dep("CSA2", "A320", RunwayHoldingShort)})
	c := r.Decide(now.Add(40*time.Second), []RunwayUser{dep("QTR1", "B77W", RunwayAirborne), dep("CSA2", "A320", RunwayHoldingShort)})
	if !slices.Contains(c.LineUp, "CSA2") || slices.Contains(c.Takeoff, "CSA2") || !strings.Contains(c.Waiting["CSA2"], "QTR1") {
		t.Fatalf("40 s behind a heavy: %+v", c)
	}
	c = r.Decide(now.Add(90*time.Second), []RunwayUser{dep("QTR1", "B77W", RunwayAirborne), dep("CSA2", "A320", RunwayLinedUp)})
	if slices.Contains(c.Takeoff, "CSA2") {
		t.Fatal("take-off 90 s behind a heavy")
	}
	c = r.Decide(now.Add(2*time.Minute+time.Second), []RunwayUser{dep("QTR1", "B77W", RunwayAirborne), dep("CSA2", "A320", RunwayLinedUp)})
	if !slices.Contains(c.Takeoff, "CSA2") {
		t.Fatalf("2 min behind a heavy: %+v", c)
	}
}

func TestRunwayControllerQueue(t *testing.T) {
	r := NewRunwayController(RunwayControllerOptions{})
	now := time.Now()
	r.Decide(now, []RunwayUser{dep("A1", "A320", RunwayHoldingShort)})
	c := r.Decide(now.Add(time.Second), []RunwayUser{dep("B2", "A320", RunwayHoldingShort), dep("A1", "A320", RunwayHoldingShort)})
	if !slices.Contains(c.LineUp, "A1") || slices.Contains(c.LineUp, "B2") || c.Waiting["B2"] != "number 2 for departure" {
		t.Fatalf("first come first: %+v", c)
	}
	// Other traffic lined up: ours wait; it is never cleared.
	other := dep("AI9", "A320", RunwayLinedUp)
	other.Other = true
	c = r.Decide(now.Add(2*time.Second), []RunwayUser{other, dep("A1", "A320", RunwayHoldingShort)})
	if len(c.LineUp) != 0 || slices.Contains(c.Takeoff, "AI9") {
		t.Fatalf("with other traffic lined up: %+v", c)
	}
}

// TestHoldForRunway: pushback and taxi go by themselves; the departure
// waits at the holding point and lined up for its runway clearances.
func TestHoldForRunway(t *testing.T) {
	ctl, _, run, _ := injectedDeparture(t, TaxiRequest{HoldForRunway: true})
	if !run(TaxiHoldingShort, 60*60*20) {
		t.Fatalf("state %v: pushback and taxi did not go by themselves", ctl.State())
	}
	run(TaxiLiningUp, 60*120)
	if ctl.State() != TaxiHoldingShort {
		t.Fatalf("lined up without a clearance: %v", ctl.State())
	}
	ctl.ClearToLineUp()
	if !run(TaxiLinedUp, 60*120) {
		t.Fatalf("state %v after the line-up clearance", ctl.State())
	}
	run(TaxiDeparting, 60*120)
	if ctl.State() != TaxiLinedUp {
		t.Fatalf("took off without a clearance: %v", ctl.State())
	}
	if err := ctl.ClearForTakeoff(); err != nil {
		t.Fatal(err)
	}
	if !run(TaxiDeparting, 60*30) {
		t.Fatalf("state %v after the take-off clearance", ctl.State())
	}
}

// TestLineupFollowsLeadIn: at LKPR F onto runway 06 (a turn past
// MaxExitAngle, so no listed entry) the line-up still follows the taxi
// lead-in from the hold-short onto the runway instead of a turn of its own.
func TestLineupFollowsLeadIn(t *testing.T) {
	g := lkprGraph(t)
	ec := &eventClient{}
	inj := NewInjector(ec)
	ctl := NewTaxiController(NewFleet(ec), TaxiWithInjector(inj))
	a1, err := g.Layout.ParkingIndex("A1")
	if err != nil {
		t.Fatal(err)
	}
	if err := ctl.Start(TaxiRequest{Graph: g, Parking: a1, Runway: "06", Model: "FSLTL A320 Air France SL", Tail: "TVS1", HoldForRunway: true}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	ctl.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultTaxiRequestBase+reqOffSpawn, 77))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, 1200, 12))
	mon := DefaultTaxiRequestBase + reqOffMonitor
	stand := g.Layout.Parking[a1]
	frame := func() { now = now.Add(time.Second / 60); ctl.Handle(positionMsg(mon, 77, stand.Position, 0, 0, true)) }
	for i := 0; i < 60*60*30; i++ {
		frame()
		if ctl.State() == TaxiHoldingShort {
			if ctl.last.HoldingShortOf == "06/24" {
				break
			}
			ctl.ClearToCross()
		}
	}
	lead := ctl.entryPath()
	if len(lead) < 3 {
		t.Fatalf("no lead-in found from the hold-short (%d points)", len(lead))
	}
	start := len(placements(ec))
	ctl.ClearToLineUp()
	for i := 0; i < 60*120 && ctl.State() != TaxiLinedUp; i++ {
		frame()
	}
	ps := placements(ec)[start:]
	// Every lead-in point is passed within a few metres.
	for _, p := range lead[:len(lead)-1] {
		best := math.Inf(1)
		for _, q := range ps {
			best = math.Min(best, calc.HaversineMeters(p.Lat, p.Lon, q.Latitude, q.Longitude))
		}
		if best > 4 {
			t.Errorf("the line-up passes %.1f m from the lead-in point %v", best, p)
		}
	}
}

// An arrival on short final goes around for anyone lined up, crossing or
// still on the runway after landing — not for a departure rolling, and not
// while still farther out (#394).
func TestRunwayControllerGoAround(t *testing.T) {
	now := time.Now()
	landed := RunwayUser{Callsign: "KLM4", Phase: RunwayRolling, Arrival: true}
	crossing := RunwayUser{Callsign: "TVS3", Phase: RunwayRolling, Crossing: true}
	other := RunwayUser{Callsign: "N123", Phase: RunwayRolling, Other: true}
	for _, c := range []struct {
		name  string
		users []RunwayUser
		want  bool
	}{
		{"lined up", []RunwayUser{dep("CSA1", "A320", RunwayLinedUp), final("DLH2", 1)}, true},
		{"still on it after landing", []RunwayUser{landed, final("DLH2", 1)}, true},
		{"crossing", []RunwayUser{crossing, final("DLH2", 1)}, true},
		{"other traffic on it", []RunwayUser{other, final("DLH2", 1)}, true},
		{"a departure rolling", []RunwayUser{dep("CSA1", "A320", RunwayRolling), final("DLH2", 1)}, false},
		{"farther out", []RunwayUser{dep("CSA1", "A320", RunwayLinedUp), final("DLH2", 2)}, false},
		{"runway free", []RunwayUser{dep("CSA1", "A320", RunwayHoldingShort), final("DLH2", 1)}, false},
	} {
		r := NewRunwayController(RunwayControllerOptions{})
		got := r.Decide(now, c.users)
		if slices.Contains(got.GoAround, "DLH2") != c.want {
			t.Errorf("%s: go around %v, want %v (%+v)", c.name, got.GoAround, c.want, got)
		}
		if c.want && got.Waiting["DLH2"] == "" {
			t.Errorf("%s: no reason given", c.name)
		}
	}
	// Only the first arrival: the one behind is still far out.
	r := NewRunwayController(RunwayControllerOptions{})
	got := r.Decide(now, []RunwayUser{landed, final("DLH2", 1), final("AFR5", 1.1)})
	if !slices.Equal(got.GoAround, []string{"DLH2"}) {
		t.Errorf("two arrivals: %v, want only the first", got.GoAround)
	}
}

// After a go-around the arrival is sequenced again by its new prediction,
// behind those now ahead of it, instead of keeping its first place.
func TestSequencerRejoin(t *testing.T) {
	s := NewApproachSequencer("06", SequencerOptions{MinSpacingNM: 5})
	now := time.Now()
	a := func(cs string, nm float64) ApproachAircraft {
		return ApproachAircraft{Callsign: cs, Wake: WakeFor("A320"), DistanceToGoNM: nm, GroundKts: 250}
	}
	s.Update(now, []ApproachAircraft{a("DLH2", 20), a("AFR5", 30)})
	// DLH2 went around: now 40 NM to go, AFR5 at 15.
	now = now.Add(2 * time.Minute)
	s.Rejoin("DLH2")
	seq := s.Update(now, []ApproachAircraft{a("DLH2", 40), a("AFR5", 15)})
	if len(seq) != 2 || seq[0].Callsign != "AFR5" || seq[1].Callsign != "DLH2" {
		t.Fatalf("after the go-around: %+v", seq)
	}
}

// The next arrival within 6 NM is cleared to land once nothing is in the
// way; one behind it, or one with a departure lined up or rolling, is not
// (Doc 4444 12.3.4.16; no reduced runway separation).
func TestRunwayControllerLandingClearance(t *testing.T) {
	r := NewRunwayController(RunwayControllerOptions{})
	now := time.Now()
	c := r.Decide(now, []RunwayUser{final("DLH2", 5), final("QTR3", 9)})
	if len(c.Land) != 1 || c.Land[0] != "DLH2" {
		t.Errorf("free runway: land %v, want DLH2 only", c.Land)
	}
	if c := r.Decide(now, []RunwayUser{final("DLH2", 8)}); len(c.Land) != 0 {
		t.Errorf("8 NM out: land %v", c.Land)
	}
	downwind := final("RYR1590", 4)
	downwind.Established = false // on the downwind, 4 NM from the threshold (#486)
	if c := r.Decide(now, []RunwayUser{downwind}); len(c.Land) != 0 {
		t.Errorf("cleared to land on the downwind: %v", c.Land)
	}
	if c := r.Decide(now, []RunwayUser{final("DLH2", 5), dep("CSA1", "A320", RunwayLinedUp)}); slices.Contains(c.Land, "DLH2") {
		t.Error("cleared to land with a departure lined up")
	}
	rolling := dep("CSA1", "A320", RunwayRolling)
	if c := r.Decide(now, []RunwayUser{final("DLH2", 5), rolling}); slices.Contains(c.Land, "DLH2") {
		t.Error("cleared to land with a departure still on its roll")
	}
}

// Waiting only for the next arrival, the first departure is given a
// conditional line-up behind it; the second, and one waiting for the
// interval, are not (#509).
func TestRunwayControllerLineUpBehind(t *testing.T) {
	r := NewRunwayController(RunwayControllerOptions{})
	now := time.Now()
	c := r.Decide(now, []RunwayUser{dep("CSA1", "A320", RunwayHoldingShort), dep("CSA2", "A320", RunwayHoldingShort), final("DLH2", 3)})
	if c.LineUpBehind["CSA1"] != "DLH2" || c.LineUpBehind["CSA2"] != "" || c.NextArrival != "DLH2" {
		t.Fatalf("behind DLH2: %+v", c)
	}
	// Nobody to land: no condition.
	c = r.Decide(now, []RunwayUser{dep("CSA1", "A320", RunwayHoldingShort)})
	if len(c.LineUpBehind) != 0 {
		t.Fatalf("no arrival: %+v", c)
	}
	tx := ClearedLineUpBehind("CSA1", "A320", "24")
	if tx.Text != "CSA1, behind the landing A320, line up and wait runway 24, behind" {
		t.Errorf("%q", tx.Text)
	}
	if rb, _ := Readback(tx); !strings.HasPrefix(rb.Text, "Behind the landing A320, line up and wait runway 24, behind") {
		t.Errorf("readback %q", rb.Text)
	}
}

// The incident of a departure cleared off the holding point with an arrival
// on a 5 NM final at 154 kt: lining up takes its time, so it lines up behind.
func TestRunwayControllerLineUpTime(t *testing.T) {
	r := NewRunwayController(RunwayControllerOptions{})
	arr := final("DLH1402", 5)
	arr.GroundKts = 154
	c := r.Decide(time.Now(), []RunwayUser{dep("WZZ222", "A20N", RunwayHoldingShort), arr})
	if slices.Contains(c.Takeoff, "WZZ222") || slices.Contains(c.LineUp, "WZZ222") {
		t.Fatalf("take-off ahead of an arrival 2 minutes out: %+v", c)
	}
	if c.LineUpBehind["WZZ222"] != "DLH1402" {
		t.Fatalf("not behind DLH1402: %+v", c)
	}
	// Lined up already: no line-up time, it goes.
	c = r.Decide(time.Now(), []RunwayUser{dep("WZZ222", "A20N", RunwayLinedUp), arr})
	if !slices.Contains(c.Takeoff, "WZZ222") {
		t.Fatalf("lined up with the arrival 2 minutes out: %+v", c)
	}
}

// A departure lining up behind a landing aircraft still rolling out is not
// cleared for take-off, whichever of them is listed first (live: BAW1272
// was cleared as AFR558 rolled out, the arrival listed first).
func TestRunwayControllerRollingArrivalBlocksTakeoff(t *testing.T) {
	roll := final("AFR558", 0)
	roll.Phase = RunwayRolling
	up := dep("BAW1272", "A320", RunwayLinedUp)
	for _, users := range [][]RunwayUser{{roll, up}, {up, roll}} {
		r := NewRunwayController(RunwayControllerOptions{})
		c := r.Decide(time.Now(), users)
		if slices.Contains(c.Takeoff, "BAW1272") || !strings.Contains(c.Waiting["BAW1272"], "AFR558") {
			t.Errorf("%s first: %+v", users[0].Callsign, c)
		}
	}
}

// A crossing waiting only for the next arrival is given conditionally,
// behind it; one in the way of someone on the runway is not.
func TestRunwayControllerCrossBehind(t *testing.T) {
	r := NewRunwayController(RunwayControllerOptions{})
	now := time.Now()
	x := RunwayUser{Callsign: "TVS3", Phase: RunwayHoldingShort, Crossing: true}
	c := r.Decide(now, []RunwayUser{x, final("DLH2", 1.5)})
	if len(c.Cross) != 0 || c.CrossBehind["TVS3"] != "DLH2" {
		t.Fatalf("crossing behind an arrival at 1.5 NM: %+v", c)
	}
	roll := final("DLH2", 0)
	roll.Phase = RunwayRolling
	if c := r.Decide(now, []RunwayUser{x, roll}); len(c.CrossBehind) != 0 || len(c.Cross) != 0 {
		t.Fatalf("with an arrival rolling out: %+v", c)
	}
	tx := ClearedCrossBehind("TVS3", "A320", "12")
	if tx.Text != "TVS3, behind the landing A320, cross runway 12, behind" {
		t.Errorf("said %q", tx.Text)
	}
}

func TestRunwayControllerNoDelay(t *testing.T) {
	r := NewRunwayController(RunwayControllerOptions{})
	now := time.Now()
	lined := dep("CSA716", "A320", RunwayLinedUp)
	c := r.Decide(now, []RunwayUser{lined, final("TVS1034", 7)})
	if !slices.Contains(c.Takeoff, "CSA716") || c.NoDelay["CSA716"] != 7 {
		t.Fatalf("traffic on a 7 NM final: %+v", c)
	}
	c = r.Decide(now, []RunwayUser{lined, final("TVS1034", 12)})
	if _, ok := c.NoDelay["CSA716"]; !slices.Contains(c.Takeoff, "CSA716") || ok {
		t.Fatalf("traffic on a 12 NM final: %+v", c)
	}
	// Not yet on the final approach (a STAR passing near): no traffic said.
	near := final("TVS1034", 6)
	near.Established = false
	if c = r.Decide(now, []RunwayUser{lined, near}); len(c.NoDelay) != 0 {
		t.Fatalf("arrival not established: %+v", c)
	}
}

// Departures on the same route: a faster follower waits the catch-up as
// well, and at the holding points about as long the faster goes first
// (live: a B738 four minutes behind a C25C on VENO7D flew through it).
func TestDepartureSpeeds(t *testing.T) {
	m, l := WakeFor("A320"), WakeFor("C25C")
	if d := DepartureIntervalSpeeds(l, m, true, 300, 380); d != SameRouteDepartureInterval+2*time.Minute {
		t.Errorf("80 kt faster on the same route: %s", d)
	}
	if d := DepartureIntervalSpeeds(l, m, false, 300, 380); d != DepartureInterval(l, m, false) {
		t.Errorf("another route: %s", d)
	}
	if d := DepartureIntervalSpeeds(m, l, true, 380, 300); d != SameRouteDepartureInterval {
		t.Errorf("slower behind: %s", d)
	}
	r := NewRunwayController(RunwayControllerOptions{})
	now := time.Now()
	slow, fast := dep("OKCVY", "C25C", RunwayHoldingShort), dep("RYR1527", "B738", RunwayHoldingShort)
	slow.ClimbKts, fast.ClimbKts = 300, 380
	r.Decide(now, []RunwayUser{slow})
	c := r.Decide(now.Add(30*time.Second), []RunwayUser{slow, fast})
	if !slices.Contains(c.LineUp, "RYR1527") || c.Waiting["OKCVY"] != "number 2 for departure" {
		t.Errorf("the faster at the holding point 30 s later: %+v", c)
	}
}

// The host's aircraft at the holding point takes its place, first come:
// ours behind it wait as the next numbers; it is never cleared (#739).
func TestHostInTheDepartureQueue(t *testing.T) {
	rc := NewRunwayController(RunwayControllerOptions{})
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	host := RunwayUser{Callsign: "PLAYER", Wake: WakeFor("A320"), Phase: RunwayHoldingShort, Host: true}
	ours := RunwayUser{Callsign: "CSA1", Wake: WakeFor("A320"), Phase: RunwayHoldingShort}
	rc.Decide(now, []RunwayUser{host})
	c := rc.Decide(now.Add(10*time.Second), []RunwayUser{host, ours})
	if len(c.LineUp) != 0 || len(c.Takeoff) != 0 {
		t.Fatalf("cleared with the host ahead: %+v", c)
	}
	if c.Waiting["CSA1"] != "number 2 for departure" {
		t.Errorf("CSA1 waits %q", c.Waiting["CSA1"])
	}
	// Ours first: it goes, the host is number 2.
	rc = NewRunwayController(RunwayControllerOptions{})
	rc.Decide(now, []RunwayUser{ours})
	c = rc.Decide(now.Add(10*time.Second), []RunwayUser{ours, host})
	if !slices.Contains(c.Takeoff, "CSA1") || slices.Contains(c.Takeoff, "PLAYER") || slices.Contains(c.LineUp, "PLAYER") {
		t.Errorf("ours first: %+v", c)
	}
}

// In a rush the first departure behind ours on its take-off roll is lined
// up behind it ("behind the departing ..."); without one it waits for the
// runway, and with an arrival too close it waits too.
func TestRunwayControllerLineUpBehindDeparting(t *testing.T) {
	now := time.Now()
	roll := dep("CSA1", "A320", RunwayRolling)
	// Two waiting: a rush.
	r := NewRunwayController(RunwayControllerOptions{})
	c := r.Decide(now, []RunwayUser{roll, dep("EZY2", "A320", RunwayHoldingShort), dep("WZZ3", "A321", RunwayHoldingShort)})
	if c.LineUpBehindDeparting["EZY2"] != "CSA1" || len(c.LineUpBehindDeparting) != 1 {
		t.Fatalf("rush: %+v", c)
	}
	if !strings.Contains(c.Waiting["WZZ3"], "number 2") {
		t.Errorf("the one after: %q, want number 2", c.Waiting["WZZ3"])
	}
	// One waiting, no arrival: no rush, it waits for the runway.
	r = NewRunwayController(RunwayControllerOptions{})
	c = r.Decide(now, []RunwayUser{roll, dep("EZY2", "A320", RunwayHoldingShort)})
	if len(c.LineUpBehindDeparting) != 0 {
		t.Fatalf("no rush: %+v", c.LineUpBehindDeparting)
	}
	// One waiting on another SID (a shorter interval), an arrival 3 min out:
	// a rush; on a 2 NM final: too close.
	other := dep("EZY2", "A320", RunwayHoldingShort)
	other.Route = "DOBE4A"
	r = NewRunwayController(RunwayControllerOptions{})
	c = r.Decide(now, []RunwayUser{roll, other, final("AUA4", 7)})
	if c.LineUpBehindDeparting["EZY2"] != "CSA1" {
		t.Fatalf("arrival 3 min out: %+v", c)
	}
	r = NewRunwayController(RunwayControllerOptions{})
	c = r.Decide(now, []RunwayUser{roll, other, final("AUA4", 2)})
	if len(c.LineUpBehindDeparting) != 0 {
		t.Fatalf("arrival on a 2 NM final: %+v", c.LineUpBehindDeparting)
	}
}

// TestRunwayControllerDoubleGap: two departures go in one double gap (the
// next arrival 10.5 NM out, DoubleGapExtraNM): the first at once, the
// second when the interval behind it allows, both before the arrival is
// too close — on different routes (1 min) and on one route (2 min).
func TestRunwayControllerDoubleGap(t *testing.T) {
	for _, same := range []bool{false, true} {
		r := NewRunwayController(RunwayControllerOptions{})
		now := time.Now()
		a1, b2 := dep("A1", "A320", RunwayHoldingShort), dep("B2", "A320", RunwayHoldingShort)
		if !same {
			b2.Route = "LOMK1A"
		}
		arrNM := DefaultDepartureGapNM + DoubleGapExtraNM
		var a1Off, b2Off time.Duration
		a1Gone := false
		for s := 0; s <= 300 && b2Off == 0; s += 5 {
			at := now.Add(time.Duration(s) * time.Second)
			c := r.Decide(at, []RunwayUser{a1, b2, final("DLH9", arrNM)})
			el := time.Duration(s) * time.Second
			if slices.Contains(c.Takeoff, "A1") && !a1Gone {
				a1Off, a1.Phase, a1Gone = el, RunwayRolling, true
			}
			if a1Gone && el-a1Off >= 35*time.Second {
				a1.Phase = RunwayAirborne
			}
			if slices.Contains(c.LineUp, "B2") && b2.Phase == RunwayHoldingShort {
				b2.Phase = RunwayLinedUp
			}
			if slices.Contains(c.Takeoff, "B2") {
				b2Off = el
			}
			arrNM -= 140.0 / 3600 * 5
		}
		if a1Off != 0 || b2Off == 0 {
			t.Errorf("same route %v: A1 off at %v, B2 off at %v (want A1 at once, B2 in the gap); arrival at %.1f NM", same, a1Off, b2Off, arrNM)
			continue
		}
		t.Logf("same route %v: B2 off after %v, the arrival %.1f NM out", same, b2Off, arrNM)
	}
}
