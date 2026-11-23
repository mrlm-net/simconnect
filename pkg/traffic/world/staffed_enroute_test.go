package world

import (
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// routeSim is a simulator that only notes whose route is set.
type routeSim struct {
	simPort
	mu  sync.Mutex
	set []uint32
}

func (r *routeSim) SetRoute(id uint32, _ []types.SIMCONNECT_DATA_WAYPOINT) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.set = append(r.set, id)
	return nil
}

func (r *routeSim) routes() []uint32 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.set)
}

const westID, eastID = uint32(101), uint32(102)

// meetingBed is two overflights of ours head on at FL370, 40 NM apart
// across the line between two centre sectors, and the conflict watch on
// them: what TestOverflightRadio met by chance (#1662).
type meetingBed struct {
	w    *World
	s    *scheduler
	cw   *conflictWatch
	sim  *routeSim
	now  time.Time
	air  []traffic.TrackedAircraft
	west *enrouteAC
}

const meetWest, meetEast = "Test Centre West", "Test Centre East"

func newMeetingBed(t *testing.T) *meetingBed {
	t.Helper()
	w := New(Options{DataDir: t.TempDir()})
	cc := newControlCenter(nil, w.st.core)
	b := &meetingBed{w: w, sim: &routeSim{}}
	cc.sim = b.sim
	cc.graph = w.st.cache.Graph // no airports: class E throughout
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	go func() {
		for {
			select {
			case f := <-cc.cmds:
				f()
			case <-stop:
				return
			}
		}
	}()
	mid := airport.LatLon{Lat: 50.1, Lon: 14.26}
	box := func(from, to float64) []airport.LatLon {
		return []airport.LatLon{{Lat: 46, Lon: from}, {Lat: 54, Lon: from}, {Lat: 54, Lon: to}, {Lat: 46, Lon: to}}
	}
	cc.localStations = func(string) []traffic.Station {
		return []traffic.Station{
			{Position: traffic.PosCenter, Name: meetWest, Freq: "127.000", Area: box(mid.Lon-6, mid.Lon)},
			{Position: traffic.PosCenter, Name: meetEast, Freq: "128.000", Area: box(mid.Lon, mid.Lon+6)},
		}
	}
	b.s = &scheduler{st: w.st, cc: cc, focus: []string{"LKPR"}, enroute: map[string]*enrouteAC{}, pending: map[uint32]*enrouteAC{}}
	b.cw = newConflictWatch(b.s)
	at := func(brg, nm float64) airport.LatLon {
		lat, lon := calc.DisplaceByHeading(mid.Lat, mid.Lon, brg, nm*1852)
		return airport.LatLon{Lat: lat, Lon: lon}
	}
	b.now = cc.clock.Now()
	seen := []traffic.Observation{{ObjectID: 1, Tail: "USER", Position: mid, OnGround: true, User: true}}
	for _, f := range []struct {
		cs       string
		id       uint32
		from, to float64
	}{{"KLM221", westID, 270, 90}, {"DLH836", eastID, 90, 270}} {
		e := &enrouteAC{f: traffic.ManagedFlight{Flight: traffic.Flight{Callsign: f.cs}, Kind: "overflight", Airport: "LKPR"}, objectID: f.id, cruiseKts: 450,
			route: []traffic.RoutePoint{{Position: at(f.from, 20), AltFt: 37000, Kts: 450}, {Position: at(f.to, 150), AltFt: 37000, Kts: 450}}}
		b.s.enroute[f.cs] = e
		if f.id == westID {
			b.west = e
		}
		o := traffic.Observation{ObjectID: f.id, Tail: f.cs, Position: at(f.from, 20), AltFt: 37000, AGLFt: 36000, GroundKts: 450, Heading: f.to}
		seen = append(seen, o)
		b.air = append(b.air, traffic.TrackedAircraft{Observation: o, Phase: traffic.PhaseEnroute, Ours: true, Updated: b.now})
	}
	cc.world.Observe(b.now, seen)
	return b
}

// look is the watch's look, after from now.
func (b *meetingBed) look(after time.Duration) {
	b.cw.tick(b.now.Add(after), b.air)
}

func (b *meetingBed) staff(st ...StaffedStation) {
	for i := range st {
		st[i].ICAO = "LKPR"
	}
	b.w.SetStaffed(st)
}

// A centre staffed at a second focus airport is the player's too: the
// first one's alone was looked at (#1684).
func TestEnrouteStaffedSecondAirport(t *testing.T) {
	b := newMeetingBed(t)
	b.s.focus = []string{"LKPR", "LKKV"}
	lkpr := b.s.cc.localStations
	b.s.cc.localStations = func(icao string) []traffic.Station {
		if icao == "LKKV" {
			return []traffic.Station{{Position: traffic.PosCenter, Name: "Test Centre Karlovy", Freq: "129.000"}}
		}
		return lkpr(icao)
	}
	b.w.SetStaffed([]StaffedStation{{ICAO: "LKKV", Position: "center", Mode: ModeSilent}})
	b.look(0)
	if got := b.sim.routes(); len(got) != 0 {
		t.Fatalf("staffed at LKKV: routes set %v, want none", got)
	}
	if _, err := b.s.instruct(EnrouteInstruction{Callsign: "KLM221", AltFt: 36000}); err != nil {
		t.Fatalf("the player's instruction: %v", err)
	}
	// And the player hears it: its check-in on the Karlovy centre.
	b.s.over.radio = true
	b.s.over.crossing = map[string][2]string{"KLM221": {}}
	b.s.radioTick(b.now.Add(radioEvery))
	heard := false
	for _, x := range b.s.cc.radio.Recent("LKKV", 0) {
		heard = heard || x.Callsign == "KLM221" && x.Frequency == "129.000"
	}
	if !heard {
		t.Errorf("no check-in from KLM221 on the staffed Karlovy centre (LKPR's: %d transmissions)", len(b.s.cc.radio.Recent("LKPR", 0)))
	}
}

// The AI resolves its own two overflights' conflict: the bed has one.
func TestEnrouteConflictUnstaffed(t *testing.T) {
	b := newMeetingBed(t)
	b.look(0)
	if got := b.sim.routes(); len(got) != 1 {
		t.Fatalf("not staffed: routes set %v, want one change", got)
	}
}

// A player's clearance to an overflight on a staffed centre stands: the
// conflict watch set its own route a second after "descend FL360" (#1662).
func TestEnrouteClearanceStands(t *testing.T) {
	for _, mode := range []string{ModeSilent, ModeAssist} {
		t.Run(mode, func(t *testing.T) {
			b := newMeetingBed(t)
			b.staff(StaffedStation{Position: "center", Mode: mode})
			if _, err := b.s.instruct(EnrouteInstruction{Callsign: "KLM221", AltFt: 36000}); err != nil {
				t.Fatal(err)
			}
			cleared := slices.Clone(b.west.route)
			for after := time.Duration(0); after <= 2*time.Minute; after += conflictEvery {
				b.look(after)
			}
			if got := b.sim.routes(); !slices.Equal(got, []uint32{westID}) {
				t.Errorf("routes set %v, want the player's only, [%d]", got, westID)
			}
			if !slices.Equal(b.west.route, cleared) {
				t.Errorf("flies %+v, want the player's %+v", b.west.route, cleared)
			}
		})
	}
}

// The modes are an arrival's: silent nothing, assist an alert with a
// suggestion, the safety net only once the player is too late.
func TestEnrouteStaffedModes(t *testing.T) {
	for _, c := range []struct {
		mode          string
		alert, stepIn bool
	}{{ModeSilent, false, false}, {ModeAssist, true, false}, {ModeSafetyNet, true, true}} {
		t.Run(c.mode, func(t *testing.T) {
			b := newMeetingBed(t)
			b.staff(StaffedStation{Position: "center", Mode: c.mode})
			b.look(0)
			if got := b.sim.routes(); len(got) != 0 {
				t.Fatalf("at once: routes set %v, want none", got)
			}
			alerts := b.cw.alertList()
			if (len(alerts) == 1) != c.alert {
				t.Fatalf("%d alerts, want one: %v", len(alerts), c.alert)
			}
			if c.alert {
				if a := alerts[0]; len(a.Staffed) != 2 || a.Suggested == "" {
					t.Errorf("staffed %v, suggested %q; want both and a suggestion", a.Staffed, a.Suggested)
				}
			}
			b.look(netGrace + conflictEvery)
			if got := b.sim.routes(); (len(got) == 1) != c.stepIn {
				t.Fatalf("after the deadline: routes set %v, want a change: %v", got, c.stepIn)
			}
			if _, done := b.cw.view(); c.stepIn && (len(done) != 1 || !done[0].SafetyNet) {
				t.Errorf("resolutions %+v, want one marked the safety net's", done)
			}
		})
	}
}

// The safety net's change to an aircraft the player has cleared nothing
// doesn't end by itself on the flight plan's level, speed or route: it is
// held until the player instructs it, or it is no longer staffed (#1662).
func TestEnrouteNetChangeHeld(t *testing.T) {
	b := newMeetingBed(t)
	b.staff(StaffedStation{Position: "center", Mode: ModeSafetyNet})
	b.look(0)
	b.look(netGrace + conflictEvery)
	_, done := b.cw.view()
	if len(done) != 1 || !done[0].SafetyNet {
		t.Fatalf("resolutions %+v, want the safety net's one", done)
	}
	r := done[0].Resolution
	e := b.s.enroute[r.Callsign]
	var me traffic.TrackedAircraft
	for _, a := range b.air {
		if a.Tail == r.Callsign {
			me = a
		}
	}
	last := e.route[len(e.route)-1]
	switch r.Kind {
	case traffic.ResolveLevel:
		if last.AltFt != r.AltFt {
			t.Errorf("ends at %.0f ft by itself, want %.0f held", last.AltFt, r.AltFt)
		}
	case traffic.ResolveSpeed:
		if last.Kts != r.Kts {
			t.Errorf("ends at %.0f kt by itself, want %.0f held", last.Kts, r.Kts)
		}
	case traffic.ResolveHeading:
		far := 0.0
		for _, p := range e.route[:len(e.route)-1] {
			far = max(far, calc.HaversineNM(me.Position.Lat, me.Position.Lon, p.Position.Lat, p.Position.Lon))
		}
		if want := me.GroundKts * conflictLookAhead.Hours(); far <= want {
			t.Errorf("heading flown %.0f NM then its route again, want it held past %.0f", far, want)
		}
	default:
		t.Fatalf("resolved by %s: the bed no longer has a change that ends", r.Kind)
	}
	// Still staffed when the change would have ended: nothing given.
	b.air = []traffic.TrackedAircraft{me}
	b.look(netGrace + conflictLookAhead + time.Minute)
	if got := b.sim.routes(); len(got) != 1 {
		t.Fatalf("staffed, the change over: routes set %v, want the one", got)
	}
	// No longer staffed: the AI gives it its route again.
	b.staff()
	b.look(netGrace + conflictLookAhead + 2*time.Minute)
	if got := b.sim.routes(); len(got) != 2 {
		t.Fatalf("unstaffed: routes set %v, want its route again", got)
	}
	if end := e.route[len(e.route)-1]; end.AltFt != 37000 || end.Kts != 450 {
		t.Errorf("unstaffed: on to %+v, want its route's FL370 at 450 kt", end)
	}
}

// With one sector staffed the AI steers round the player's aircraft with
// its own, as it does for arrivals and departures.
func TestEnrouteOneSectorStaffed(t *testing.T) {
	for _, c := range []struct {
		station string
		steered uint32
	}{{meetWest, eastID}, {meetEast, westID}} {
		b := newMeetingBed(t)
		b.staff(StaffedStation{Station: c.station, Mode: ModeSilent})
		b.look(0)
		if got := b.sim.routes(); !slices.Equal(got, []uint32{c.steered}) {
			t.Errorf("%s staffed: routes set %v, want [%d]", c.station, got, c.steered)
		}
	}
}

// An arrival before its STAR that the staffed approach has told something
// is the player's as well (#1419).
func TestEnrouteToldArrivalStands(t *testing.T) {
	b := newMeetingBed(t)
	for _, e := range b.s.enroute {
		e.f.Kind, e.arrive, e.told = "arrival", &planned{}, &enrouteTold{icao: "LKPR", levelFt: 36000}
	}
	b.staff(StaffedStation{Position: "approach", Mode: ModeSilent})
	b.look(0)
	if got := b.sim.routes(); len(got) != 0 {
		t.Errorf("routes set %v, want none", got)
	}
}

// A stop the watch gave before the centre was staffed is not cleared on
// over the player: no old route flown again.
func TestEnrouteStopNotRestoredOverPlayer(t *testing.T) {
	b := newMeetingBed(t)
	b.air = b.air[:1]
	b.cw.stopped["KLM221"] = stoppedLevel{icao: "LKPR", pos: traffic.PosCenter, altFt: 39000, climb: true, other: "GONE1", planned: slices.Clone(b.west.route)}
	b.cw.busy["KLM221"] = b.now.Add(conflictLookAhead)
	b.staff(StaffedStation{Position: "center", Mode: ModeSilent})
	b.look(0)
	if got := b.sim.routes(); len(got) != 0 {
		t.Errorf("routes set %v, want the old route not flown again", got)
	}
}
