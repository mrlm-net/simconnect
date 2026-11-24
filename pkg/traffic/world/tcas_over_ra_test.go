package world

import (
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// No route, level, speed or suggestion over a TCAS RA until clear of
// conflict (#1669), by the issue's five points.

// sentLog notes what is given to a fake controller.
type sentLog struct {
	mu   sync.Mutex
	sent []string
}

func (l *sentLog) note(what string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sent = append(l.sent, what)
	return nil
}

func (l *sentLog) list() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.sent)
}

// raDep is a departure handed to MSFS AI that only notes what it is given.
type raDep struct {
	departureCtl
	sentLog
	plan []traffic.RoutePoint
}

func (d *raDep) State() traffic.TaxiState { return traffic.TaxiComplete }
func (d *raDep) ClimbPlan(airport.LatLon) []traffic.RoutePoint {
	return slices.Clone(d.plan)
}
func (d *raDep) ClimbRoute(airport.LatLon) []airport.LatLon {
	var out []airport.LatLon
	for _, p := range d.plan {
		out = append(out, p.Position)
	}
	return out
}
func (d *raDep) Reroute([]traffic.RoutePoint) error { return d.note("route") }
func (d *raDep) ClimbTo(airport.LatLon, float64, float64) error {
	return d.note("climbOn")
}
func (d *raDep) ClimbExpedited(airport.LatLon, float64, float64, float64) error {
	return d.note("climb")
}
func (d *raDep) FlyHeading(airport.LatLon, float64, float64, float64, float64) (traffic.Vector, error) {
	return traffic.Vector{}, d.note("heading")
}
func (d *raDep) AssignSpeed(airport.LatLon, float64, float64) error { return d.note("speed") }
func (d *raDep) DirectTo(airport.LatLon, float64, float64, airport.LatLon) error {
	return d.note("direct")
}

// raArr is an arrival on its STAR that only notes what it is given.
type raArr struct {
	arrivalCtl
	sentLog
	plan []traffic.RoutePoint
}

func (a *raArr) State() traffic.ArrivalState         { return traffic.ArrivalApproaching }
func (a *raArr) ProcedurePlan() []traffic.RoutePoint { return slices.Clone(a.plan) }
func (a *raArr) ProcedureRoute() []airport.LatLon    { return nil }
func (a *raArr) StopDescent(float64, float64) error  { return a.note("level") }
func (a *raArr) AssignSpeed(k float64) (float64, error) {
	return k, a.note("speed")
}

// depBed is the meeting bed's KLM221 as a departure of ours handed to MSFS
// AI, alone of ours, with departure radar.
func depBed(t *testing.T) (*meetingBed, *raDep, *controlled) {
	t.Helper()
	b := newMeetingBed(t)
	clear(b.s.enroute)
	b.air[1].Ours = false
	d := &raDep{plan: slices.Clone(b.west.route[1:])}
	it := &controlled{
		ID: 1, Tail: "KLM221", ICAO: "LKPR", cc: b.s.cc, dep: d, objectID: westID, removed: make(chan struct{}),
		graph:  &airport.Graph{Layout: &airport.Layout{ICAO: "LKPR"}},
		spoken: map[string]bool{},
	}
	it.atc, it.identified, it.radarFt, it.radarSaid = traffic.PosDeparture, true, 39000, "flight level 390"
	it.view.State = traffic.TaxiComplete.String()
	it.view.Position = b.air[0].Position
	b.s.cc.items[1] = it
	return b, d, it
}

// raBegins has the TCAS watch give KLM221 an RA; respond has the crew fly it.
func raBegins(b *meetingBed) (tw *tcasWatch, respond func()) {
	return raBeginsAt(b, b.now)
}

// raBeginsAt is raBegins with the RA begun at at.
func raBeginsAt(b *meetingBed, at time.Time) (tw *tcasWatch, respond func()) {
	tw = newTCASWatch(b.s)
	me, near := b.air[0], b.air[1]
	lat, lon := calc.DisplaceByHeading(me.Position.Lat, me.Position.Lon, me.Heading, 3*1852)
	near.Position.Lat, near.Position.Lon, near.Ours = lat, lon, false
	air := []traffic.TrackedAircraft{me, near}
	tw.tick(at, air)
	return tw, func() {
		tw.tick(at.Add(raResponseDelay), air)
	}
}

// raResponseDelay is how long the crew takes to fly an RA.
const raResponseDelay = time.Duration(traffic.TCASResponseDelaySec * float64(time.Second))

// depRA is depBed with the RA flown.
func depRA(t *testing.T) (*meetingBed, *raDep, *controlled, *tcasWatch) {
	t.Helper()
	return depRAFrom(t, 0)
}

// depRAFrom is depRA with the RA begun back before the bed's now.
func depRAFrom(t *testing.T, back time.Duration) (*meetingBed, *raDep, *controlled, *tcasWatch) {
	t.Helper()
	b, d, it := depBed(t)
	tw, respond := raBeginsAt(b, b.now.Add(-back))
	respond()
	if got := d.list(); !it.tcasRA.Load() || !slices.Equal(got, []string{"route"}) {
		t.Fatalf("given %v: the bed has no RA flown", got)
	}
	return b, d, it, tw
}

// 1. An arrival en route isn't paced over an RA, and is once clear.
func TestOverRAPacing(t *testing.T) {
	b, tw := raBed(t)
	b.west.f.Kind, b.west.arrive = "arrival", &planned{}
	me, leader := b.air[0].Observation, b.air[1].Observation
	leader.GroundKts = 300
	b.s.cc.world.Observe(b.now.Add(time.Second), []traffic.Observation{me, leader})
	b.s.cc.sequencesAt = func(string) map[string][]traffic.SequenceEntry {
		first, then := traffic.SequenceEntry{}, traffic.SequenceEntry{}
		first.Callsign, first.DistanceToGoNM = "DLH836", 50
		then.Callsign, then.DistanceToGoNM = "KLM221", 55
		return map[string][]traffic.SequenceEntry{"24": {first, then}}
	}
	b.s.paceEnroute(b.now)
	if got := b.sim.routes(); len(got) != 1 || b.west.pacedKts != 0 {
		t.Errorf("routes set %v, paced at %.0f kt, want the RA's only", got, b.west.pacedKts)
	}
	raOver(b, tw)
	b.s.paceEnroute(b.now.Add(2 * time.Minute))
	if got := b.sim.routes(); len(got) != 2 || b.west.pacedKts != 300 {
		t.Errorf("clear of conflict: routes set %v, paced at %.0f kt, want 300", got, b.west.pacedKts)
	}
}

// 2. A departure stopped for traffic is cleared on once clear of conflict.
func TestOverRADepartureStopWaits(t *testing.T) {
	b, d, _, tw := depRA(t)
	b.air = b.air[:1]
	b.cw.stopped["KLM221"] = stoppedLevel{icao: "LKPR", pos: traffic.PosDeparture, altFt: 39000, climb: true, other: "GONE1", planned: slices.Clone(d.plan)}
	b.cw.busy["KLM221"] = b.now.Add(conflictLookAhead)
	b.look(0)
	if got := d.list(); !slices.Equal(got, []string{"route"}) {
		t.Errorf("given %v, want the RA's route only", got)
	}
	if !b.cw.isStopped("KLM221") {
		t.Errorf("the stop forgotten during the RA")
	}
	raOver(b, tw)
	b.look(2 * conflictEvery)
	if got := d.list(); !slices.Contains(got, "climbOn") {
		t.Errorf("clear of conflict: given %v, want the climb on", got)
	}
}

// 2. No "cross (fix) at or below" over an RA.
func TestOverRADepartureRestriction(t *testing.T) {
	for _, ra := range []bool{false, true} {
		b, d, it := depBed(t)
		me, them := b.air[0].Observation, b.air[1].Observation
		at := func(nm float64) airport.LatLon {
			lat, lon := calc.DisplaceByHeading(me.Position.Lat, me.Position.Lon, 90, nm*1852)
			return airport.LatLon{Lat: lat, Lon: lon}
		}
		me.Heading, me.AltFt, me.AGLFt, me.GroundKts, me.VSFpm = 90, 10000, 9000, 300, 2000
		them.Position, them.Heading, them.AltFt, them.AGLFt, them.GroundKts = at(22.5), 270, 14500, 13500, 300
		world := b.s.cc.world
		world.Observe(b.now.Add(time.Second), []traffic.Observation{me, them})
		world.SetOwn(westID, traffic.PhaseEnroute, "")
		world.SetOwn(eastID, traffic.PhaseArriving, "LKPR")
		d.plan = []traffic.RoutePoint{{Position: at(45), AltFt: 20000, Kts: 300}, {Position: at(80), AltFt: 20000, Kts: 300}}
		it.radarFt, it.fixes = 20000, []airFix{{Ident: "BEKVI", LatLon: at(45)}}
		b.s.cc.items[2] = &controlled{
			ID: 2, Tail: "DLH836", ICAO: "LKPR", cc: b.s.cc, objectID: eastID,
			arr: &raArr{plan: []traffic.RoutePoint{{Position: at(-60), AltFt: 14500, Kts: 300}}},
		}
		it.tcasRA.Store(ra)
		var air []traffic.TrackedAircraft
		for _, a := range world.Aircraft() {
			if a.ObjectID == westID || a.ObjectID == eastID {
				air = append(air, a)
			}
		}
		given := b.cw.crossInstead(b.now, traffic.Conflict{A: "KLM221", B: "DLH836"}, air, func(a traffic.TrackedAircraft) *controlled {
			if a.ObjectID == westID {
				return it
			}
			return nil
		})
		if !ra && !given {
			t.Fatalf("no RA: no restriction given: nothing checked")
		}
		if ra && (given || len(d.list()) != 0) {
			t.Errorf("flying an RA: restriction given (%v)", d.list())
		}
	}
}

// 2. Departure radar identifies it, and clears the climb once clear. The
// RA is flown before the hand-over, as the World's clock reads: a call
// decided before the RA is flown waits until clear of conflict (#1672),
// and the bed's RA times run ahead of that clock.
func TestOverRADepartureIdentified(t *testing.T) {
	b, d, it, tw := depRAFrom(t, raResponseDelay)
	it.atc, it.identified = traffic.PosTower, false
	it.view.ATC = string(traffic.PosTower)
	it.mu.Lock() // as update calls it
	it.handoff(TaxiOrArrival{dep: &traffic.TaxiEvent{State: traffic.TaxiDeparting, HeightFt: 5000}})
	it.mu.Unlock()
	cc := b.s.cc
	later := cc.clock.Now()
	for range 20 {
		later = later.Add(time.Minute)
		cc.agenda.run(later)
	}
	it.mu.Lock()
	identified := it.identified
	it.mu.Unlock()
	if !identified {
		t.Fatalf("not identified: nothing checked")
	}
	if got := d.list(); !slices.Equal(got, []string{"route"}) {
		t.Errorf("given %v, want the RA's route only", got)
	}
	raOver(b, tw)
	cc.pending.run(later.Add(time.Hour))
	if got := d.list(); !slices.Contains(got, "climbOn") {
		t.Errorf("clear of conflict: given %v, want the climb on", got)
	}
}

// 2. A climb held for traffic waits for clear of conflict too.
func TestOverRADepartureClimbWhenClear(t *testing.T) {
	b, d, it, tw := depRA(t)
	it.climbWhenClear()
	if got := d.list(); !slices.Equal(got, []string{"route"}) {
		t.Errorf("given %v, want the RA's route only", got)
	}
	raOver(b, tw)
	b.s.cc.pending.run(b.s.cc.clock.Now().Add(time.Hour))
	if got := d.list(); !slices.Contains(got, "climbOn") {
		t.Errorf("clear of conflict: given %v, want the climb on", got)
	}
}

// 2. The crew asks for a direct once clear, and one asked before the RA is
// "unable".
func TestOverRADepartureDirect(t *testing.T) {
	for _, c := range []struct {
		what           string
		before, during bool // the RA begins before it asks, or before the answer
	}{{"no RA", false, false}, {"RA, then the request", true, false}, {"request, then the RA", false, true}} {
		t.Run(c.what, func(t *testing.T) {
			b, d, it := depBed(t)
			me := b.air[0]
			at := func(off, nm float64) airport.LatLon {
				lat, lon := calc.DisplaceByHeading(me.Position.Lat, me.Position.Lon, me.Heading+off, nm*1852)
				return airport.LatLon{Lat: lat, Lon: lon}
			}
			p1, p2 := at(60, 20), at(0, 30)
			d.plan = []traffic.RoutePoint{{Position: p1, AltFt: 37000, Kts: 450}, {Position: p2, AltFt: 37000, Kts: 450}}
			it.identified, it.fixes = false, []airFix{{Ident: "ARTUP", LatLon: p1}, {Ident: "BEKVI", LatLon: p2}}
			ask := func() func() (traffic.Transmission, bool) {
				// The share that asks is drawn: 200 draws miss it once in 10^31.
				for range 200 {
					b.cw.crewRequests(b.now, b.air[:1], conflictOpts)
					if it.directAnswer != nil {
						return it.directAnswer
					}
					if c.before {
						if b.cw.asked["KLM221"] {
							return nil // its one request spent
						}
						continue
					}
					delete(b.cw.asked, "KLM221")
				}
				return nil
			}
			fly := func() {
				_, respond := raBegins(b)
				respond()
			}
			if c.before {
				fly()
				if ask() != nil || b.cw.asked["KLM221"] {
					t.Errorf("asked, or its request spent, flying an RA")
				}
				return
			}
			answer := ask()
			if answer == nil {
				t.Fatalf("no request: nothing checked")
			}
			if c.during {
				fly()
			}
			_, ok := answer()
			got := slices.Contains(d.list(), "direct")
			if !c.during && !got {
				t.Fatalf("no RA: no direct: nothing checked")
			}
			if c.during && (got || !ok) {
				t.Errorf("direct given %v, answered %v, want unable", got, ok)
			}
		})
	}
}

// 2. The player's climb, heading and speed are refused until clear.
func TestOverRADeparturePlayer(t *testing.T) {
	for what, give := range map[string]func(*controlled) error{
		"climb":   func(it *controlled) error { return it.climbTo(40000, 0, false) },
		"heading": func(it *controlled) error { return it.headingTo(120) },
		"speed":   func(it *controlled) error { return it.speedTo(280) },
	} {
		t.Run(what, func(t *testing.T) {
			b, d, it, tw := depRA(t)
			if err := give(it); !errors.Is(err, errFlyingRA) {
				t.Errorf("flying an RA: %v, want refused", err)
			}
			if got := d.list(); !slices.Equal(got, []string{"route"}) {
				t.Errorf("given %v, want the RA's route only", got)
			}
			raOver(b, tw)
			if err := give(it); err != nil || !slices.Contains(d.list(), what) {
				t.Errorf("clear of conflict: %v, given %v, want it taken", err, d.list())
			}
		})
	}
}

// 2. On a staffed departure it levels at its cleared level once clear.
func TestOverRADepartureLevelsAfter(t *testing.T) {
	b, _, it, tw := depRA(t)
	it.gates.Store(true)
	it.clearedFt = 39000
	levelled := func() bool {
		b.s.cc.crewAsksHigher([]*controlled{it}, b.now)
		it.mu.Lock()
		defer it.mu.Unlock()
		return it.levelSet
	}
	if levelled() {
		t.Errorf("levelled at its cleared level over the RA")
	}
	raOver(b, tw)
	if !levelled() {
		t.Errorf("clear of conflict: not levelled")
	}
}

// handoverBed is raBed's KLM221 as an arrival at its STAR entry.
func handoverBed(t *testing.T) (*meetingBed, *tcasWatch) {
	t.Helper()
	b, tw := raBed(t)
	b.west.f.Kind, b.west.f.STA = "arrival", b.now.Add(3*time.Hour)
	b.west.arrive = &planned{route: []airport.NavPoint{{Ident: "GOLOP", Position: b.air[0].Position}}}
	b.s.cc.sim = removingSim{b.sim}
	b.s.mgr = traffic.NewTrafficManager(b.s, traffic.ManagerOptions{})
	return b, tw
}

// 3. Not handed to approach during an RA; due once clear, wherever it is.
func TestOverRAHandoverWaits(t *testing.T) {
	b, tw := handoverBed(t)
	state := func() (listed, handing bool) {
		b.s.mu.Lock()
		defer b.s.mu.Unlock()
		return b.s.enroute["KLM221"] == b.west, b.west.handing
	}
	b.s.handovers(b.now)
	if listed, handing := state(); !listed || handing {
		t.Fatalf("handed over flying an RA")
	}
	// Past its entry by the time it is clear.
	lat, lon := calc.DisplaceByHeading(b.air[0].Position.Lat, b.air[0].Position.Lon, 180, 200*1852)
	b.west.arrive.route[0].Position = airport.LatLon{Lat: lat, Lon: lon}
	raOver(b, tw)
	b.s.handovers(b.now)
	for until := time.Now().Add(5 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		if listed, _ := state(); !listed {
			return
		}
		if time.Now().After(until) {
			t.Fatalf("clear of conflict: not handed over")
		}
	}
}

// 3. An RA begun during its hand-over keeps it en route, flag and all.
func TestOverRAHandoverBegun(t *testing.T) {
	b, _ := handoverBed(t)
	b.west.handing = true
	b.s.handEnroute(b.west)
	if b.s.enroute["KLM221"] != b.west {
		t.Fatalf("taken off the list flying an RA: the flag is lost")
	}
	if b.west.handing {
		t.Errorf("still being handed over: never due again")
	}
}

// 3. Nor is it cleared on while being handed over.
func TestOverRAHandingStopWaits(t *testing.T) {
	b, _ := raBed(t)
	b.air = b.air[:1]
	b.west.handing = true
	b.cw.stopped["KLM221"] = stoppedLevel{icao: "LKPR", pos: traffic.PosCenter, altFt: 39000, climb: true, other: "GONE1", planned: slices.Clone(b.west.route)}
	b.cw.busy["KLM221"] = b.now.Add(conflictLookAhead)
	b.look(0)
	if got := b.sim.routes(); len(got) != 1 {
		t.Errorf("routes set %v, want the RA's only", got)
	}
}

// 4. An instruction past its check as the RA begins is not sent after it.
func TestOverRAInstructionInFlight(t *testing.T) {
	type bed struct {
		b    *meetingBed
		give func() error
		sent func() []string
		want []string // after the RA is flown
	}
	enroute := func(t *testing.T, station string) (*meetingBed, func() []string) {
		b := newMeetingBed(t)
		delete(b.s.enroute, "DLH836")
		b.staff(StaffedStation{Position: station, Mode: ModeSilent})
		return b, func() []string {
			var out []string
			for range b.sim.routes() {
				out = append(out, "route")
			}
			return out
		}
	}
	departure := func(give func(*controlled) error) func(*testing.T) bed {
		return func(t *testing.T) bed {
			b, d, it := depBed(t)
			return bed{b, func() error { return give(it) }, d.list, []string{"route"}}
		}
	}
	for what, newBed := range map[string]func(*testing.T) bed{
		"centre": func(t *testing.T) bed {
			b, sent := enroute(t, "center")
			return bed{b, func() error {
				_, err := b.s.instruct(EnrouteInstruction{Callsign: "KLM221", AltFt: 36000})
				return err
			}, sent, []string{"route"}}
		},
		"approach, before the STAR": func(t *testing.T) bed {
			b, sent := enroute(t, "approach")
			b.west.f.Kind, b.west.arrive = "arrival", &planned{}
			q := &sequences{cc: b.s.cc, s: b.s}
			return bed{b, func() error {
				_, err := q.enrouteAction("LKPR", "KLM221", "descend", 0, 30000, spacing{})
				return err
			}, sent, []string{"route"}}
		},
		"approach, on the STAR": func(t *testing.T) bed {
			b := newMeetingBed(t)
			clear(b.s.enroute)
			arr := &raArr{}
			b.s.cc.items[1] = &controlled{ID: 1, Tail: "KLM221", ICAO: "LKPR", cc: b.s.cc, arr: arr, objectID: westID, removed: make(chan struct{})}
			q := &sequences{cc: b.s.cc, s: b.s, seq: map[string]*traffic.ApproachSequencer{}}
			return bed{b, func() error {
				return q.approachDo("LKPR", "KLM221", "speed", nil, 240, 0, spacing{})
			}, func() []string {
				// Its RA may be a level or none: only the speed matters.
				return slices.DeleteFunc(arr.list(), func(s string) bool { return s != "speed" })
			}, nil}
		},
		"departure climb":   departure(func(it *controlled) error { return it.climbTo(40000, 0, false) }),
		"departure heading": departure(func(it *controlled) error { return it.headingTo(120) }),
		"departure speed":   departure(func(it *controlled) error { return it.speedTo(280) }),
		"departure climb on": departure(func(it *controlled) error {
			it.climbOn()
			return errFlyingRA // it only logs
		}),
	} {
		t.Run(what, func(t *testing.T) {
			c := newBed(t)
			_, respond := raBegins(c.b)
			cc := c.b.s.cc
			busy, release := make(chan struct{}), make(chan struct{})
			go cc.do(func() error {
				close(busy)
				<-release
				return nil
			})
			<-busy
			told := make(chan struct{})
			var err error
			go func() {
				err = c.give()
				close(told)
			}()
			waitFor := func(what string, ok func() bool) {
				for until := time.Now().Add(5 * time.Second); !ok(); time.Sleep(time.Millisecond) {
					if time.Now().After(until) {
						close(release)
						t.Fatalf("%s: nothing checked", what)
					}
				}
			}
			waitFor("the instruction not on its way", func() bool { return len(cc.cmds) == 1 })
			flown := make(chan struct{})
			go func() {
				respond()
				close(flown)
			}()
			waitFor("no RA flown", func() bool {
				select {
				case <-flown:
					return true
				default:
					return len(cc.cmds) == 2
				}
			})
			close(release)
			<-told
			<-flown
			if !errors.Is(err, errFlyingRA) {
				t.Errorf("%v, want refused", err)
			}
			if got := c.sent(); !slices.Equal(got, c.want) {
				t.Errorf("given %v, want %v", got, c.want)
			}
		})
	}
}

// 5. Assist suggests nothing to one flying an RA.
func TestOverRANoSuggestion(t *testing.T) {
	suggested := func(b *meetingBed) string {
		b.staff(StaffedStation{Position: "center", Mode: ModeAssist})
		b.look(0)
		alerts := b.cw.alertList()
		if len(alerts) != 1 {
			t.Fatalf("%d alerts, want one: nothing checked", len(alerts))
		}
		return alerts[0].Suggested
	}
	plain := newMeetingBed(t)
	delete(plain.s.enroute, "DLH836")
	plain.air[1].Ours = false
	if suggested(plain) == "" {
		t.Fatalf("no RA: nothing suggested: nothing checked")
	}
	b, _ := raBed(t)
	if got := suggested(b); got != "" {
		t.Errorf("suggested %q to one flying an RA", got)
	}
}

// Nor is it told to resume own navigation.
func TestOverRAResumeWaits(t *testing.T) {
	b, tw := raBed(t)
	b.west.f.Kind, b.west.arrive = "arrival", &planned{}
	b.west.told = &enrouteTold{icao: "LKPR", resumeAt: b.west.route[0].Position, resumeFix: "GOLOP"}
	b.s.enrouteResumes()
	if b.west.told.resumeFix == "" {
		t.Errorf("told to resume own navigation over the RA")
	}
	raOver(b, tw)
	b.s.enrouteResumes()
	if b.west.told.resumeFix != "" {
		t.Errorf("clear of conflict: not told to resume")
	}
}
