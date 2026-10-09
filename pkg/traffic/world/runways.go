package world

import (
	"errors"
	"math"
	"net/http"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// The towers on the map (#393): a runway controller per airport and
// runway clears our departures to line up and take off, and our traffic
// to cross, by what the runway is doing — our arrivals, departures and
// crossings, and respected other traffic — and sends an arrival around
// when the runway is not free on short final (#394), to be sequenced again. Aircraft spawned with "hold at
// every clearance" are the user's: counted, never cleared.
//
//	GET /api/runways?icao=LKPR — each runway's users and who waits for what

type towers struct {
	cc *controlCenter
	s  *scheduler

	mu      sync.Mutex
	ctl     map[string]*traffic.RunwayController // by "ICAO runway"
	given   map[string]bool                      // clearances given: "tail action"
	waiting map[string]string                    // why each waits, as last logged
	last    map[string][]runwayUserView          // the users, for the API
	// behind: departures cleared to line up behind a landing aircraft
	// (#509), by call sign: that aircraft, and the departure's runway.
	behind map[string]behindClearance
	next   map[string]string // the next arrival to land, by "ICAO runway"
	// users: each runway's users as last decided on, by "ICAO runway", for
	// the player's ATC asking (PlayerRunway).
	users map[string][]traffic.RunwayUser
	// userPrev: where the user aircraft was at the last look (its way).
	userPrev airport.LatLon
	// playerStand: the stand held for the user aircraft, "ICAO LABEL".
	playerStand string
	// grantAt: when the runway controller last granted each clearance
	// ("tail action"): one waiting on the agenda is dropped once it is
	// no longer granted.
	grantAt map[string]time.Time
	// rtoJudged: departures whose crew has judged its take-off roll (#621).
	rtoJudged map[string]bool
}

// behindClearance is a conditional line-up or crossing (cross) waiting
// for its arrival.
type behindClearance struct {
	arrival, icao, rwy string
	cross              bool
	// departing: behind ours on its take-off roll (LineUpBehindDeparting),
	// not a landing aircraft.
	departing bool
}

type runwayUserView struct {
	Callsign string `json:"callsign"`
	Phase    string `json:"phase"`
	Waiting  string `json:"waiting,omitempty"`
	// departure: a departure (not crossing, not landing), for the number
	// to depart before a new one (departuresAhead).
	departure bool
}

func newTowers(cc *controlCenter, s *scheduler) *towers {
	return &towers{cc: cc, s: s, ctl: map[string]*traffic.RunwayController{}, given: map[string]bool{}, waiting: map[string]string{}, last: map[string][]runwayUserView{}, behind: map[string]behindClearance{}, next: map[string]string{}, rtoJudged: map[string]bool{}}
}

var phaseNames = map[traffic.RunwayPhase]string{traffic.RunwayHoldingShort: "holding short", traffic.RunwayLinedUp: "lined up",
	traffic.RunwayRolling: "on the runway", traffic.RunwayAirborne: "airborne", traffic.RunwayFinal: "final"}

// runwayOf is the runway of a runway end or runway name ("06", "06/24").
func runwayOf(l *airport.Layout, name string) (airport.Runway, bool) {
	for _, r := range l.Runways {
		if r.Name() == name || r.Primary.Name == name || r.Secondary.Name == name {
			return r, true
		}
	}
	return airport.Runway{}, false
}

// onRunway reports whether p is on the runway's surface (and a margin).
func onRunway(r airport.Runway, p airport.LatLon) bool {
	return onRunwayWithin(r, p, 15)
}

// vacatedClearM: a vacating aircraft's reference point this far beyond
// the runway edge has its tail clear of the runway too.
const vacatedClearM = 40.0

// onRunwayWithin reports whether p is on the runway or within margin
// meters of its edges.
func onRunwayWithin(r airport.Runway, p airport.LatLon, margin float64) bool {
	a, b := r.Primary.Threshold, r.Secondary.Threshold
	along := calc.AlongTrackMeters(a.Lat, a.Lon, b.Lat, b.Lon, p.Lat, p.Lon)
	cross := math.Abs(calc.CrossTrackMeters(a.Lat, a.Lon, b.Lat, b.Lon, p.Lat, p.Lon))
	return along > -100 && along < r.Length+100 && cross < r.Width/2+margin
}

func (t *towers) tick(now time.Time) {
	t.cc.mu.Lock()
	items := make([]*controlled, 0, len(t.cc.items))
	for _, it := range t.cc.items {
		items = append(items, it)
	}
	t.cc.mu.Unlock()
	type key struct{ icao, rwy string }
	users := map[key][]traffic.RunwayUser{}
	ours := map[string]*controlled{}
	graphs := map[string]*airport.Layout{}
	layout := func(icao string) *airport.Layout {
		if l, ok := graphs[icao]; ok {
			return l
		}
		g, err := t.cc.graph(icao)
		if err != nil {
			graphs[icao] = nil
			return nil
		}
		graphs[icao] = g.Layout
		return g.Layout
	}
	for _, it := range items {
		l := layout(it.ICAO)
		if l == nil {
			continue
		}
		it.mu.Lock()
		v, id := it.view, it.objectID
		it.mu.Unlock()
		if v.Done {
			continue
		}
		if v.State != "holding short" {
			t.forget(v.Tail) // the next crossing is cleared afresh
		}
		model := v.Model
		u := traffic.RunwayUser{Callsign: v.Tail, Wake: traffic.WakeFor(model), Route: v.Procedure, Other: it.gates.Load()}
		if it.dep != nil {
			u.ClimbKts = climbKts(model) // the interval and order behind a slower one
		}
		own, ok := runwayOf(l, v.Runway)
		if !ok {
			continue
		}
		// Taxiing across a runway (cleared, or not waiting to be): on it.
		if v.State == "taxiing" && v.OnGround {
			for _, r := range l.Runways {
				if onRunway(r, v.Position) {
					u.Phase, u.Crossing = traffic.RunwayRolling, true
					users[key{it.ICAO, r.Name()}] = append(users[key{it.ICAO, r.Name()}], u)
				}
			}
			continue
		}
		rk := key{it.ICAO, own.Name()}
		switch {
		case v.State == "holding short" && v.HoldingShortOf != "" && v.HoldingShortOf != own.Name():
			// Crossing another runway.
			x, ok := runwayOf(l, v.HoldingShortOf)
			if !ok {
				continue
			}
			u.Phase, u.Crossing = traffic.RunwayHoldingShort, true
			rk = key{it.ICAO, x.Name()}
		case it.dep != nil && v.State == "holding short":
			u.Phase = traffic.RunwayHoldingShort
		case it.dep != nil && (v.State == "lining up" || v.State == "lined up"):
			u.Phase = traffic.RunwayLinedUp
		case it.dep != nil && v.State == "departing" && v.OnGround:
			u.Phase, u.GroundKts = traffic.RunwayRolling, v.GroundSpeed
		case it.dep != nil && v.State == "departing":
			u.Phase = traffic.RunwayAirborne
		case it.arr != nil && (v.State == "approaching" || v.State == "landing") && !v.OnGround:
			rw, end, _ := l.RunwayEnd(v.Runway)
			// Along the way it still flies, as the sequence counts it: in a
			// straight line, an arrival passing near the field on its STAR or
			// downwind "landed in 1m38s" eleven minutes early and held every
			// departure (live, RYR1485, 2026-10-02).
			d := calc.HaversineNM(v.Position.Lat, v.Position.Lon, end.Threshold.Lat, end.Threshold.Lon)
			it.mu.Lock()
			route := it.approach
			it.mu.Unlock()
			// Lined up close in, it is on the final whatever its route says:
			// a route round another circuit passes the runway twice, and
			// measured along it OKUFC on a 1 NM final "landed" minutes later
			// (live, OKQOL cleared for take-off in front of it).
			switch {
			case overRunway(v.Position, v.Heading, end, rw.Length):
				// In its flare past the threshold: landing now, not minutes
				// out along its route (live, TVS837 cleared for take-off 2 s
				// before OKZLK touched down, then cancelled).
				d = 0
			case len(route) > 0 && !onFinalNear(v.Position, v.Heading, d, end):
				d = math.Max(d, traffic.DistanceToGo(v.Position, route, end.Threshold))
			}
			if d > 3 {
				t.forgetGoAround(v.Tail) // out again: another go-around may follow
			}
			if d > 20 {
				continue
			}
			u.Phase, u.Arrival, u.DistanceNM, u.GroundKts = traffic.RunwayFinal, true, d, v.GroundSpeed
			// Established: its STAR and approach flown, on the final (#486).
			u.Established = id != 0 && len(it.arr.ProcedureRoute()) == 0
		case it.arr != nil && (v.State == "landing" || v.State == "rollout"):
			u.Phase, u.Arrival = traffic.RunwayRolling, true
		case it.arr != nil && v.State == "vacating":
			// Off the runway once it is clear of it (its tail too), not when
			// it stops past the holding point: live, RYR1485 held a lined-up
			// departure 35 s after it had turned off (2026-10-02).
			if !onRunwayWithin(own, v.Position, vacatedClearM) {
				continue
			}
			u.Phase, u.Arrival = traffic.RunwayRolling, true
		default:
			continue
		}
		ours[v.Tail] = it
		users[rk] = append(users[rk], u)
	}
	// Respected other traffic: on a runway, or arriving on the one in use.
	if t.s.mgr.Options().Others == traffic.OtherRespect {
		// Ours are counted above, on their own runway: the manager does not
		// know the aircraft spawned on the map and would count them again as
		// other traffic (a departure rolling through the crossing runway
		// showed on 12/30 as well as on 06/24).
		ownIDs := t.cc.ownIDs()
		for _, icao := range t.s.mgr.Airports() {
			l := layout(icao)
			if l == nil {
				continue
			}
			g, _ := t.cc.graph(icao)
			arrRwy := t.cc.activeRunway(g, true)
			_, end, _ := l.RunwayEnd(arrRwy)
			for _, a := range t.s.mgr.Others(icao) {
				if a.User || ownIDs[a.ObjectID] || (a.Tail != "" && t.cc.byTail(a.Tail) != nil) {
					continue
				}
				u := traffic.RunwayUser{Callsign: nameOf(a), Wake: traffic.WakeFor(a.Title), Other: true}
				switch a.Phase {
				case traffic.PhaseRunway:
					for _, r := range l.Runways {
						if onRunway(r, a.Position) {
							u.Phase = traffic.RunwayRolling
							users[key{icao, r.Name()}] = append(users[key{icao, r.Name()}], u)
						}
					}
				case traffic.PhaseArriving:
					if r, ok := runwayOf(l, arrRwy); ok {
						u.Phase, u.Arrival = traffic.RunwayFinal, true
						u.DistanceNM, u.GroundKts = calc.HaversineNM(a.Position.Lat, a.Position.Lon, end.Threshold.Lat, end.Threshold.Lon), a.GroundKts
						users[key{icao, r.Name()}] = append(users[key{icao, r.Name()}], u)
					}
				}
			}
		}
	}
	// The user aircraft (#739): as the host's ATC cleared it (holding short
	// in the departure queue; lined up, rolling or crossing on the runway)
	// and as it is seen, on a runway or close in on a final, cleared or not,
	// on every runway crossing that one too; a stale clearance expires.
	ua := userAircraft(t.cc.world.Aircraft())
	if p, ok := t.cc.core.playerClearance(); ok {
		t.cc.core.expirePlayer(ua, layout(p.ICAO))
	}
	var pc *PlayerClearance
	if p, ok := t.cc.core.playerClearance(); ok {
		pc = &p
	}
	for k, list := range playerUsers(pc, ua, t.s.mgr.Airports(), layout) {
		users[key{k[0], k[1]}] = append(users[key{k[0], k[1]}], list...)
	}
	// On the ground: where it is going, for ours to give way (#739).
	t.mu.Lock()
	prev := t.userPrev
	if ua != nil {
		t.userPrev = ua.Position
	}
	t.mu.Unlock()
	t.reportUserGround(ua, prev, t.s.mgr.Airports(), layout)
	t.syncPlayerStand(pc, ua)
	// Service vehicles holding short to cross (#752): in the queue as any
	// crossing.
	for _, k := range t.cc.vehicles().runwaysWaiting() {
		users[key{k[0], k[1]}] = append(users[key{k[0], k[1]}], t.cc.vehicles().crossingUsers(k[0], k[1])...)
	}
	// A gap for a change of the runway in use (the selector's Ready moment):
	// no more arrivals within runwayChangeGapNM than finish on the old
	// runway (runwayChangeKeepArrivals); take-offs under way finish there.
	busy, nearby := map[string]bool{}, map[string]int{}
	for k, list := range users {
		for _, u := range list {
			if u.Arrival && u.Phase == traffic.RunwayFinal && u.DistanceNM <= runwayChangeGapNM {
				nearby[k.icao]++
				busy[k.icao] = nearby[k.icao] > runwayChangeKeepArrivals
			}
		}
	}
	t.cc.core.setRunwaysBusy(busy)
	// A runway nobody uses now shows nobody: its last state stayed on
	// (live, "12/30 · WZZ100 on the runway" long after it had crossed).
	t.mu.Lock()
	for name := range t.last {
		icao, rwy, _ := strings.Cut(name, " ")
		if _, used := users[key{icao, rwy}]; !used {
			delete(t.last, name)
			delete(t.users, name)
		}
	}
	t.mu.Unlock()
	for k, list := range users {
		t.mu.Lock()
		rc := t.ctl[k.icao+" "+k.rwy]
		if rc == nil {
			rc = traffic.NewRunwayController(traffic.RunwayControllerOptions{})
			t.ctl[k.icao+" "+k.rwy] = rc
		}
		t.mu.Unlock()
		c := rc.Decide(now, list)
		// The host's ATC cleared the user aircraft onto it (#710): none of
		// ours lines up, takes off, lands or crosses meanwhile.
		if t.cc.core.playerBlocks(k.icao, k.rwy, layout(k.icao)) {
			c.LineUp, c.Takeoff, c.Land, c.Cross, c.LineUpBehind, c.CrossBehind, c.LineUpBehindDeparting = nil, nil, nil, nil, nil, nil, nil
		}
		t.lineUpBehind(rc, k.icao, k.rwy, list, ours)
		t.crewDecides(k.icao, list, ours)
		t.crewRejects(ours)
		t.apply(k.icao, k.rwy, c, ours)
		t.cc.vehicles().crossCleared(k.icao, c.Cross)
		t.mu.Lock()
		t.next[k.icao+" "+k.rwy] = c.NextArrival
		if t.users == nil {
			t.users = map[string][]traffic.RunwayUser{}
		}
		t.users[k.icao+" "+k.rwy] = append([]traffic.RunwayUser(nil), list...)
		t.mu.Unlock()
		var view []runwayUserView
		for _, u := range list {
			view = append(view, runwayUserView{Callsign: u.Callsign, Phase: phaseNames[u.Phase], Waiting: c.Waiting[u.Callsign], departure: !u.Arrival && !u.Crossing})
		}
		t.mu.Lock()
		t.last[k.icao+" "+k.rwy] = view
		t.mu.Unlock()
	}
}

// behind gives departure it a conditional line-up behind arrival (#509):
// said now, lined up by lineUpBehind once the arrival has passed.
func (t *towers) clearBehind(it *controlled, arrival, icao, rwy string) {
	t.mu.Lock()
	t.behind[it.Tail] = behindClearance{arrival: arrival, icao: icao, rwy: rwy}
	t.given[it.Tail+" lineup"] = true
	t.mu.Unlock()
}

// nextArrival is the next arrival to land on its runway ("" none).
func (t *towers) nextArrival(it *controlled) string {
	l := it.graph.Layout
	it.mu.Lock()
	rwy := it.view.Runway
	it.mu.Unlock()
	r, ok := runwayOf(l, rwy)
	if !ok {
		return ""
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.next[it.ICAO+" "+r.Name()]
}

// behindNext is the user's conditional line-up of departure it behind the
// next arrival (#509); an error when none is to land.
func (t *towers) behindNext(it *controlled) error {
	arr := t.nextArrival(it)
	if arr == "" {
		return errors.New("no arrival to line up behind")
	}
	it.mu.Lock()
	rwy := it.view.Runway
	it.mu.Unlock()
	r, _ := runwayOf(it.graph.Layout, rwy)
	t.clearBehind(it, arr, it.ICAO, r.Name())
	return nil
}

// arrivalSaid is an arrival as a tower names it in a condition: its type
// ("Airbus A320"), "aircraft" when not one of ours.
func (t *towers) arrivalSaid(cs string) string {
	if a := t.cc.byTail(cs); a != nil {
		a.mu.Lock()
		model := a.view.Model
		a.mu.Unlock()
		return typeSaid(traffic.ProfileFor(strings.SplitN(model, liverySep, 2)[0]).Type)
	}
	return "aircraft"
}

// lineUpBehind lines up the departures cleared behind a landing aircraft
// once it has passed: no longer on the final — for an intersection
// departure, off the runway too.
func (t *towers) lineUpBehind(rc *traffic.RunwayController, icao, rwy string, list []traffic.RunwayUser, ours map[string]*controlled) {
	t.mu.Lock()
	var due, noRoom []string
	crossing := map[string]bool{}
	for dep, b := range t.behind {
		if b.icao != icao || b.rwy != rwy {
			continue
		}
		it := ours[dep]
		if it == nil || it.dep == nil && !b.cross {
			delete(t.behind, dep) // gone, or no longer holding short
			continue
		}
		phase, there := traffic.RunwayPhase(0), false
		for _, u := range list {
			if u.Callsign == b.arrival {
				phase, there = u.Phase, true
			}
		}
		passed := !there || phase != traffic.RunwayFinal
		if b.departing {
			// Behind a departure: once it is airborne, or from the full
			// length once it is well into its roll and away (from an
			// intersection it has to pass it: airborne).
			var lead traffic.RunwayUser
			for _, u := range list {
				if u.Callsign == b.arrival {
					lead = u
				}
			}
			full := it.dep == nil || it.dep.Route() == nil || it.dep.Route().Entry == ""
			passed = !there || phase != traffic.RunwayRolling || full && lead.GroundKts >= behindDepartingKts
		} else if b.cross && there {
			passed = false // across once it is off the runway: where it rolls to is not known
		} else if !b.cross {
			if r := it.dep.Route(); r != nil && r.Entry != "" && there {
				passed = false // an intersection: once it is off the runway
			}
		}
		if passed && !b.cross {
			// Still time before the arrival after it? Else not now: it holds
			// and goes behind the next one (live, TVS1124 lined up and LOT775
			// went around).
			var first traffic.RunwayUser
			then := math.Inf(1)
			for _, u := range list {
				switch {
				case u.Callsign == b.arrival:
					first = u
				case u.Phase == traffic.RunwayFinal:
					then = math.Min(then, u.DistanceNM/math.Max(u.GroundKts, 100)*3600)
				}
			}
			if first.Callsign == "" {
				first.Wake = traffic.WakeFor("A320")
			}
			it.mu.Lock()
			model := it.view.Model
			it.mu.Unlock()
			if !math.IsInf(then, 1) && !rc.BehindRoom(first, time.Duration(then*float64(time.Second)), traffic.RunwayUser{Wake: traffic.WakeFor(model)}) {
				delete(t.behind, dep)
				delete(t.given, dep+" lineup")
				noRoom = append(noRoom, dep)
				continue
			}
		}
		if passed {
			delete(t.behind, dep)
			due = append(due, dep)
			crossing[dep] = b.cross
		}
	}
	t.mu.Unlock()
	for _, dep := range noRoom {
		ours[dep].say(traffic.HoldPosition(dep))
		t.cc.log.printf("%-6s holds: no time to line up and go before the next arrival", dep)
	}
	for _, dep := range due {
		it := ours[dep]
		if crossing[dep] {
			t.cc.log.printf("%-6s crossing behind the landing traffic", dep)
			if err := t.cc.do(func() error {
				if it.dep != nil {
					it.dep.ClearToCross()
				} else if it.arr != nil {
					it.arr.ClearToCross()
				}
				return nil
			}); err != nil {
				t.cc.log.printf("%-6s crossing refused: %v", dep, err)
			}
			continue
		}
		t.cc.log.printf("%-6s lining up behind the traffic", dep)
		if err := t.cc.do(func() error { it.dep.ClearToLineUp(); return nil }); err != nil {
			t.cc.log.printf("%-6s line-up refused: %v", dep, err)
		}
	}
}

// apply gives the clearances (once each) and logs who waits for what.
func (t *towers) apply(icao, rwy string, c traffic.RunwayClearances, ours map[string]*controlled) {
	give := func(tail, action string, said traffic.Transmission, f func(it *controlled) error) {
		it := ours[tail]
		if it == nil || it.gates.Load() {
			return
		}
		said = it.rushed(said)
		t.mu.Lock()
		if t.grantAt == nil {
			t.grantAt = map[string]time.Time{}
		}
		t.grantAt[tail+" "+action] = t.cc.clock.Now()
		done := t.given[tail+" "+action]
		_, behind := t.behind[tail]
		t.mu.Unlock()
		if done {
			return
		}
		// Told to line up behind a landing aircraft: no other line-up or
		// take-off until it has passed (lineUpBehind) — live, EZY866 was
		// cleared for take-off 14 s after its conditional line-up.
		if behind && (action == "takeoff" || action == "lineup") {
			return
		}
		t.mu.Lock()
		t.given[tail+" "+action] = true
		if action == "takeoff" || action == "lineupbehind" {
			t.given[tail+" lineup"] = true // no "line up and wait" after it
		}
		t.mu.Unlock()
		// On the tower's agenda, most urgent first: a go-around, then a
		// landing, then the rest. Still granted when its turn comes, or
		// dropped (to be given again when granted again).
		prio := prioRunway
		switch action {
		case "goaround":
			prio = prioUrgent
		case "land":
			prio = prioLanding
		}
		key := tail + " " + action
		still := func() bool {
			if action == "goaround" {
				return true
			}
			t.mu.Lock()
			at := t.grantAt[key]
			t.mu.Unlock()
			return !it.gates.Load() && t.cc.clock.Now().Sub(at) <= grantFresh
		}
		dropped := func() {
			t.mu.Lock()
			delete(t.given, key)
			if action == "takeoff" || action == "lineupbehind" {
				delete(t.given, tail+" lineup")
			}
			t.mu.Unlock()
			t.cc.log.printf("%-6s tower: %s no longer granted when its turn came", tail, action)
		}
		it.callIf(traffic.PosTower, prio, still, dropped, func() {
			// Said here: the state change it causes is not logged again.
			spoken := []string{strings.Fields(action)[0]}
			if action == "takeoff" || action == "lineupbehind" {
				spoken = append(spoken, "lineup") // line up and take off in one; or said with its condition
			}
			it.mu.Lock()
			for _, k := range spoken {
				it.spoken[k] = true
			}
			it.mu.Unlock()
			// Said first; the crew acts once it has read it back (#462).
			it.say(said)
			it.actAfterReadback(traffic.PosTower, "tower: "+action, func() error { return f(it) })
		})
	}
	takeoff := map[string]bool{}
	for _, cs := range c.Takeoff {
		takeoff[cs] = true
	}
	end := func(tail string) string {
		if it := ours[tail]; it != nil {
			it.mu.Lock()
			defer it.mu.Unlock()
			return it.view.Runway
		}
		return rwy
	}
	// From an intersection: said with the runway ("runway 24 at B").
	entry := func(tail string) string {
		if it := ours[tail]; it != nil {
			it.mu.Lock()
			defer it.mu.Unlock()
			return it.view.Entry
		}
		return ""
	}
	// Traffic close behind on final: "no delay, traffic on 5 mile final".
	takeoffSaid := func(cs string) traffic.Transmission {
		if nm, ok := c.NoDelay[cs]; ok {
			return traffic.AtEntry(traffic.ClearedTakeoffNoDelay(cs, end(cs), t.cc.windSaid(icao), nm), entry(cs))
		}
		return traffic.AtEntry(traffic.ClearedTakeoff(cs, end(cs), t.cc.windSaid(icao)), entry(cs))
	}
	for _, cs := range c.LineUp {
		if takeoff[cs] {
			give(cs, "takeoff", takeoffSaid(cs), func(it *controlled) error {
				it.dep.ClearToLineUp()
				return it.dep.ClearForTakeoff()
			})
			continue
		}
		give(cs, "lineup", traffic.AtEntry(traffic.ClearedLineUp(cs, end(cs)), entry(cs)), func(it *controlled) error { it.dep.ClearToLineUp(); return nil })
	}
	// Waiting only for the next arrival: line up behind it once it has
	// passed (#509).
	for cs, arr := range c.LineUpBehind {
		if ours[cs] == nil || ours[cs].gates.Load() {
			continue
		}
		t.mu.Lock()
		cleared := t.given[cs+" takeoff"]
		t.mu.Unlock()
		if cleared {
			continue // cleared for take-off already: no condition after it
		}
		arr := arr
		give(cs, "lineupbehind", traffic.ClearedLineUpBehind(cs, t.arrivalSaid(arr), end(cs)), func(it *controlled) error {
			t.clearBehind(it, arr, icao, rwy)
			return nil
		})
	}
	// Busy: behind ours on its take-off roll, lined up as it rolls.
	for cs, dep := range c.LineUpBehindDeparting {
		if ours[cs] == nil || ours[cs].gates.Load() {
			continue
		}
		dep := dep
		give(cs, "lineupbehind", traffic.ClearedLineUpBehindDeparting(cs, t.arrivalSaid(dep), end(cs)), func(it *controlled) error {
			t.mu.Lock()
			t.behind[it.Tail] = behindClearance{arrival: dep, icao: icao, rwy: rwy, departing: true}
			t.given[it.Tail+" lineup"] = true
			t.mu.Unlock()
			return nil
		})
	}
	for _, cs := range c.Takeoff {
		give(cs, "takeoff", takeoffSaid(cs), func(it *controlled) error { return it.dep.ClearForTakeoff() })
	}
	// Cleared for take-off but not rolling yet, and someone else on the
	// runway: the clearance is cancelled; it is cleared again once free (Doc
	// 4444 12.3.4.11 c). Not for an arrival closing in: lining up, the
	// departure is on the runway either way, and stopped there it sends the
	// arrival around (live, WZZ1387 behind DLH1233 at 2.9 NM).
	for cs, why := range c.Waiting {
		it := ours[cs]
		if it == nil || it.dep == nil || it.gates.Load() || !cancelTakeoffFor(why) {
			continue
		}
		it.mu.Lock()
		state := it.view.State
		it.mu.Unlock()
		t.mu.Lock()
		cleared := t.given[cs+" takeoff"]
		if cleared && (state == "lining up" || state == "lined up") {
			delete(t.given, cs+" takeoff")
		}
		t.mu.Unlock()
		if !cleared || state != "lining up" && state != "lined up" {
			continue
		}
		it.say(traffic.CancelTakeoff(cs))
		t.cc.log.printf("%-6s take-off clearance cancelled — %s", cs, why)
		if err := t.cc.do(func() error { return it.dep.AbortTakeoff() }); err != nil {
			t.cc.log.printf("%-6s cancel take-off refused: %v", cs, err)
		}
	}
	// The next arrival, the runway free: cleared to land (#462); on the
	// landing roll it is told to call ground when vacated.
	for _, cs := range c.Land {
		said := traffic.ClearedToLand(cs, end(cs), t.cc.windSaid(icao))
		// A VFR arrival with touch-and-goes left: cleared touch and go (#569).
		if it := ours[cs]; it != nil && it.arr != nil && it.arr.TouchAndGosLeft() > 0 {
			said = traffic.ClearedTouchAndGo(cs, end(cs))
			if it.stopAndGo {
				said = traffic.ClearedStopAndGo(cs, end(cs))
			}
		}
		give(cs, "land", said, func(it *controlled) error { return nil })
	}
	// Waiting only for the next arrival at a crossing: across behind it,
	// once it is off the runway. Given once; the plain crossing is then
	// taken as given.
	for cs, arr := range c.CrossBehind {
		if ours[cs] == nil || ours[cs].gates.Load() {
			continue
		}
		arr := arr
		give(cs, "cross "+rwy, traffic.ClearedCrossBehind(cs, t.arrivalSaid(arr), oneDesignator(rwy)), func(it *controlled) error {
			t.mu.Lock()
			t.behind[it.Tail] = behindClearance{arrival: arr, icao: icao, rwy: rwy, cross: true}
			t.mu.Unlock()
			return nil
		})
	}
	for _, cs := range c.Cross {
		// A crossing is cleared once per holding point: forget it once done.
		give(cs, "cross "+rwy, traffic.ClearedCross(cs, oneDesignator(rwy)), func(it *controlled) error {
			if it.dep != nil {
				it.dep.ClearToCross()
			} else {
				it.arr.ClearToCross()
			}
			return nil
		})
	}
	for _, cs := range c.GoAround {
		why := c.Waiting[cs]
		if ours[cs] == nil {
			continue
		}
		give(cs, "goaround", ours[cs].goAround(why), func(it *controlled) error {
			if it.arr == nil {
				return nil
			}
			if err := it.arr.GoAround(); err != nil {
				return err
			}
			t.forgetLanding(cs) // the next approach is cleared again (#486)
			if t.cc.rejoin != nil {
				t.cc.rejoin(icao, cs)
			}
			return nil
		})
	}
	// Who waits, and why: logged when it changes.
	names := make([]string, 0, len(c.Waiting))
	for cs := range c.Waiting {
		names = append(names, cs)
	}
	sort.Strings(names)
	for _, cs := range names {
		why := c.Waiting[cs]
		kind := waitKind(why) // "1m20s behind QTR1" counting down is the same wait
		t.mu.Lock()
		changed := t.waiting[cs] != kind
		t.waiting[cs] = kind
		t.mu.Unlock()
		if changed && ours[cs] != nil && !ours[cs].gates.Load() && !slices.Contains(c.GoAround, cs) {
			t.cc.log.printf("%-6s tower %s: waits — %s", cs, rwy, why)
		}
	}
}

// grantFresh: a clearance the runway controller granted no longer ago than
// this (it decides every second) is still granted.
const grantFresh = 2500 * time.Millisecond

// waitKind is a wait reason without its numbers: the same wait while its
// time or distance counts down.
func waitKind(why string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' || r == '.' {
			return -1
		}
		return r
	}, why)
}

// departureCleared: tail is cleared to line up or take off.
func (t *towers) departureCleared(tail string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.given[tail+" lineup"] || t.given[tail+" takeoff"] || t.given[tail+" lineupbehind"]
}

// departuresAhead is how many departures hold short of, or are lined up on,
// runway end at icao, tail aside: those to depart before it.
func (t *towers) departuresAhead(icao, end, tail string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := 0
	for k, list := range t.last {
		i, rwy, _ := strings.Cut(k, " ")
		if i != icao || !slices.Contains(strings.Split(rwy, "/"), end) {
			continue
		}
		for _, u := range list {
			if u.departure && u.Callsign != tail && (u.Phase == "holding short" || u.Phase == "lined up") {
				n++
			}
		}
	}
	return n
}

// landCleared: tail is cleared to land.
func (t *towers) landCleared(tail string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.given[tail+" land"]
}

// forgetLanding lets an arrival that went around be cleared to land on its
// next approach (#486).
func (t *towers) forgetLanding(tail string) {
	t.mu.Lock()
	delete(t.given, tail+" land")
	delete(t.given, tail+" crew") // the next approach judged afresh (#621)
	t.mu.Unlock()
}

// cancelTakeoffFor reports whether why (Decide's wait) cancels a take-off
// clearance given: someone on the runway only, never an arrival closing in.
func cancelTakeoffFor(why string) bool {
	return strings.Contains(why, "on the runway")
}

// dropBehind lets the departures told to line up behind arrival (gone
// around) be cleared afresh: no longer waiting for it.
func (t *towers) dropBehind(arrival string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for dep, b := range t.behind {
		if b.arrival == arrival {
			delete(t.behind, dep)
			delete(t.given, dep+" lineupbehind")
			delete(t.given, dep+" lineup")
		}
	}
}

// forgetTail drops every clearance given to tail and its conditional
// line-up: a call sign spawned again starts afresh (a replayed scene).
func (t *towers) forgetTail(tail string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for k := range t.given {
		if strings.HasPrefix(k, tail+" ") {
			delete(t.given, k)
		}
	}
	for k := range t.grantAt {
		if strings.HasPrefix(k, tail+" ") {
			delete(t.grantAt, k) // #85: kept for good before
		}
	}
	delete(t.rtoJudged, tail)
	delete(t.behind, tail)
	delete(t.waiting, tail)
}

// forgetGoAround lets an arrival be sent around again on its next approach.
func (t *towers) forgetGoAround(tail string) {
	t.mu.Lock()
	delete(t.given, tail+" goaround")
	t.mu.Unlock()
}

// forget lets a crossing be cleared again at the next holding point once
// the aircraft moves on.
func (t *towers) forget(tail string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for k := range t.given {
		if strings.HasPrefix(k, tail+" cross ") {
			delete(t.given, k)
		}
	}
}

func registerRunways(mux *http.ServeMux, st *state) {
	mux.HandleFunc("GET /api/runways", func(w http.ResponseWriter, r *http.Request) {
		st.mu.Lock()
		t := st.towers
		st.mu.Unlock()
		out := map[string][]runwayUserView{}
		if t != nil {
			icao := strings.ToUpper(r.URL.Query().Get("icao"))
			t.mu.Lock()
			for k, v := range t.last {
				if i, rwy, _ := strings.Cut(k, " "); i == icao {
					out[rwy] = v
				}
			}
			t.mu.Unlock()
		}
		writeJSON(w, out)
	})
}

// climbKts is a type's climb speed (TAS) for the departure interval and
// order: nav's performance data, else, for a business type there is none
// for, its published cruise speed at the climb-to-cruise ratio of the
// generic performance; 0 unknown.
func climbKts(model string) float64 {
	typ := traffic.ProfileFor(model).Type
	if p := nav.PerformanceFor(typ); p.Type != "" {
		return p.ClimbTASKts
	}
	g := nav.PerformanceFor("")
	for _, b := range traffic.BusinessTypes {
		if b.Type == typ && g.CruiseTASKts > 0 {
			return b.CruiseKts * g.ClimbTASKts / g.CruiseTASKts
		}
	}
	return 0
}

// onFinalNearNM, onFinalSectorDeg: an arrival within onFinalNearNM of the
// threshold, on the approach side within onFinalSectorDeg of the extended
// centreline and heading within onFinalSectorDeg of the runway, is on its
// final.
const (
	onFinalNearNM    = 4.0
	onFinalSectorDeg = 30.0
)

// onFinalNear: at p, heading hdg, dNM from end's threshold, the arrival is
// lined up on end's final close in.
func onFinalNear(p airport.LatLon, hdg, dNM float64, end airport.RunwayEnd) bool {
	if dNM > onFinalNearNM {
		return false
	}
	from := calc.BearingDegrees(end.Threshold.Lat, end.Threshold.Lon, p.Lat, p.Lon)
	return math.Abs(headingDiff(end.Heading+180, from)) <= onFinalSectorDeg && math.Abs(headingDiff(end.Heading, hdg)) <= onFinalSectorDeg
}

// behindDepartingKts: a departure lined up behind ours on its take-off
// roll from the full length lines up once that one rolls this fast (well
// away down the runway).
const behindDepartingKts = 40.0

// overRunway: at p, heading hdg, an arrival is over end's runway (length
// m) past its threshold, heading along it: in its flare.
func overRunway(p airport.LatLon, hdg float64, end airport.RunwayEnd, length float64) bool {
	along := calc.HaversineMeters(end.Threshold.Lat, end.Threshold.Lon, p.Lat, p.Lon)
	if along > length {
		return false
	}
	from := calc.BearingDegrees(end.Threshold.Lat, end.Threshold.Lon, p.Lat, p.Lon)
	return along < 50 || math.Abs(headingDiff(end.Heading, from)) <= 10 && math.Abs(headingDiff(end.Heading, hdg)) <= onFinalSectorDeg
}
