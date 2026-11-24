package traffic

import (
	"fmt"
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// Rescue vehicles going home (#1630): one alone, two together from
// neighbouring standby points, two sent home as they drive out and two
// from following the aircraft. Every step of the ways is checked: no reversing, no jump, no swing, no stop
// from speed in a step and, two, no contact. Nothing here is newer than
// v0.188.6, so it runs there, and fails.
const (
	homeStep      = 0.1
	homeMaxMove   = 0.3  // m a step beyond what the speed carries it
	homeMaxSwing  = 15.0 // degrees a step
	homeMaxSpeed  = 0.5  // m/s of change a step
	homeBodyWidth = 3.0  // m between two vehicles' centre lines: touching
)

var homeAirports = []string{"LKPR", "LROP"}

// homeCrew sends a vehicle to each of spots, one after another from its
// depot as a crew's are, and drives them until until says so (nil: until
// all stand at them); nil when one has no depot or never gets there.
func homeCrew(t *testing.T, l *airport.Layout, spots []GroundPose, until func(sec float64) bool) []*SimObjectRescue {
	t.Helper()
	c := &tugClient{}
	inj := NewInjector(c)
	var crew []*SimObjectRescue
	for k, spot := range spots {
		r := NewSimObjectRescue(c, inj, "Truck Fire Airport Medium", 9200+uint32(k))
		r.Layout = l
		if k > 0 {
			r.After = crew[k-1]
		}
		if err := r.Standby(spot); err != nil {
			t.Fatal(err)
		}
		if !r.hasDepot {
			return nil
		}
		crew = append(crew, r)
	}
	for i := 0; i < 30*60*10; i++ {
		all, stand := true, true
		for k, r := range crew {
			if r.objectID == 0 && !r.Waiting() {
				r.Handle(assignedMsg(9200+uint32(k), 501+uint32(k)))
			}
			if err := r.Update(homeStep); err != nil {
				t.Fatal(err)
			}
			all = all && r.objectID != 0 && !r.Waiting()
			stand = stand && r.mover == nil
		}
		if all && until != nil && until(float64(i)*homeStep) {
			return crew
		}
		if all && stand {
			if until != nil {
				return nil
			}
			return crew
		}
	}
	return nil
}

// homeGap is the least distance between the centre lines of two vehicles.
func homeGap(a, b GroundPose) float64 {
	if localDist(a.Position, b.Position) > 30 {
		return math.Inf(1)
	}
	p := rescueRoadProfile()
	front := p.WheelbaseMeters * pushNoseFactor
	b0, b1 := offsetHeading(b.Position, b.Heading+180, p.TailMeters), offsetHeading(b.Position, b.Heading, front)
	near := math.Inf(1)
	for d := -p.TailMeters; d < front+0.5; d += 0.5 {
		near = math.Min(near, pointSegDist(offsetHeading(a.Position, a.Heading, math.Min(d, front)), b0, b1))
	}
	return near
}

// homeRun sends crew home at once and drives them there as the World
// does (KeepBehind those before, then Update), checking every step; the
// first fault, "" for none.
func homeRun(t *testing.T, crew []*SimObjectRescue) string {
	t.Helper()
	prev := make([]GroundPose, len(crew))
	for k, r := range crew {
		prev[k] = r.pose
	}
	for _, r := range crew {
		if err := r.Home(); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 30*60*10; i++ {
		at := float64(i) * homeStep
		var first []airport.LatLon
		done := true
		for k, r := range crew {
			if r.Done() {
				continue
			}
			r.KeepBehind(first)
			if err := r.Update(homeStep); err != nil {
				t.Fatal(err)
			}
			p := r.pose
			first = append(first, p.Position)
			move := localDist(prev[k].Position, p.Position)
			switch {
			case move > math.Max(prev[k].GroundSpeedKts, p.GroundSpeedKts)*ktsToMS*homeStep+homeMaxMove:
				return fmt.Sprintf("vehicle %d jumped %.1f m, %.1f s from being sent home", k, move, at)
			case math.Abs(p.GroundSpeedKts-prev[k].GroundSpeedKts)*ktsToMS > homeMaxSpeed && !r.Done():
				return fmt.Sprintf("vehicle %d went from %.1f to %.1f kt in a step, %.1f s from being sent home", k, prev[k].GroundSpeedKts, p.GroundSpeedKts, at)
			case move > 0.005 && math.Abs(headingDiff(p.Heading, localBearing(prev[k].Position, p.Position))) > 90:
				return fmt.Sprintf("vehicle %d drives backwards %.1f s from being sent home", k, at)
			case math.Abs(headingDiff(prev[k].Heading, p.Heading)) > homeMaxSwing:
				return fmt.Sprintf("vehicle %d swung round %.0f° in a step, %.1f s from being sent home", k, math.Abs(headingDiff(prev[k].Heading, p.Heading)), at)
			}
			prev[k] = p
			done = done && r.Done()
			if !r.Done() {
				continue
			}
			if !homeArrived(r) {
				// Taken away where it stood: only without a way from there.
				nose := NoseGear(p.Position, p.Heading, rescueRoadProfile())
				if route, err := r.Layout.VehicleRouteJoin(nose, r.depot, true, false); err == nil && len(route) >= 2 {
					return fmt.Sprintf("vehicle %d was taken away %.0f m from the depot, with a way there, %.1f s from being sent home", k, localDist(p.Position, r.depot), at)
				}
				continue
			}
			for j, o := range crew {
				if o.Done() || o.mover == nil || !r.hasDepot || localDist(o.depot, r.depot) >= RescueDepotGapMeters {
					continue
				}
				if left := o.mover.Path().Length() - o.mover.s; left < RescueSpacingMeters-TrafficOverrunMeters-1 {
					return fmt.Sprintf("vehicle %d is %.1f m behind vehicle %d into the depot: out of the crew's order", j, left, k)
				}
			}
		}
		for a := range crew {
			for b := a + 1; b < len(crew); b++ {
				if crew[a].Done() || crew[b].Done() {
					continue
				}
				if d := homeGap(crew[a].pose, crew[b].pose); d < homeBodyWidth {
					return fmt.Sprintf("vehicles %d and %d touch (%.1f m) %.1f s from being sent home", a, b, d, at)
				}
			}
		}
		if done {
			return ""
		}
	}
	return "never home"
}

// homeArrived reports whether r, gone, went at its depot.
func homeArrived(r *SimObjectRescue) bool {
	return localDist(r.pose.Position, r.depot) < RescueDepotGapMeters
}

// homeStranded is how many of crew, sent home, went short of their depot.
func homeStranded(crew []*SimObjectRescue) int {
	n := 0
	for _, r := range crew {
		if !homeArrived(r) {
			n++
		}
	}
	return n
}

// homeBehind reports whether the road home lies behind r standing.
func homeBehind(r *SimObjectRescue) bool {
	nose := NoseGear(r.pose.Position, r.pose.Heading, rescueRoadProfile())
	route, err := r.Layout.VehicleRouteJoin(nose, r.depot, true, false)
	if err != nil {
		return false
	}
	for _, q := range route {
		if localDist(nose, q) >= 2 {
			return math.Abs(headingDiff(r.pose.Heading, localBearing(nose, q))) > 90
		}
	}
	return false
}

// homeEnds calls f for both ends of every runway of l.
func homeEnds(l *airport.Layout, f func(rwy airport.Runway, end airport.RunwayEnd)) {
	for _, r := range l.Runways {
		for _, name := range []string{r.Primary.Name, r.Secondary.Name} {
			if rwy, end, ok := l.RunwayEnd(name); ok {
				f(rwy, end)
			}
		}
	}
}

// homePairs are neighbouring standby points of a landing on end: the
// candidates at and behind one holding position or the next one, each
// with its nearest, and of the points given to a crew of 3 each two next
// to each other; in the order a crew takes them.
func homePairs(l *airport.Layout, rwy airport.Runway, end airport.RunwayEnd) [][]GroundPose {
	var out [][]GroundPose
	add := func(a, b GroundPose) {
		out = append(out, RescueAssign(l, make([]*airport.LatLon, 2), []GroundPose{a, b}))
	}
	holds := rescueHolds(l, rwy, end, nil)
	nearest := func(i int) int {
		best, near := -1, math.Inf(1)
		for j, o := range holds {
			if d := localDist(holds[i].at, o.at); j != i && d < near {
				best, near = j, d
			}
		}
		return best
	}
	for i, s := range holds {
		j := nearest(i)
		if j < 0 || j < i && nearest(j) == i {
			continue
		}
		add(GroundPose{Position: s.at, Heading: s.facing}, GroundPose{Position: holds[j].at, Heading: holds[j].facing})
	}
	spots := RescueStandby(l, end.Name, 3)
	for i := 1; i < len(spots); i++ {
		add(spots[i-1], spots[i])
	}
	return out
}

// TestRescueHomeTurnsOut: each rescue vehicle at every standby point, both
// ends of every runway, drives off forwards, turning out where the road
// to the depot lies behind it. Before, it reversed a few metres, then
// swung round.
func TestRescueHomeTurnsOut(t *testing.T) {
	for _, icao := range homeAirports {
		l := smoothLayout(t, icao)
		sent, behind := 0, 0
		homeEnds(l, func(rwy airport.Runway, end airport.RunwayEnd) {
			for n, spot := range RescueStandby(l, end.Name, 3) {
				crew := homeCrew(t, l, []GroundPose{spot}, nil)
				if crew == nil {
					continue
				}
				if homeBehind(crew[0]) {
					behind++
				}
				sent++
				if bad := homeRun(t, crew); bad != "" {
					t.Errorf("%s %s standby %d: %s", icao, end.Name, n, bad)
				} else if homeStranded(crew) > 0 {
					t.Errorf("%s %s standby %d: never got to its depot", icao, end.Name, n)
				}
			}
		})
		t.Logf("%s: %d sent home, the road behind %d", icao, sent, behind)
		if sent == 0 || behind == 0 {
			t.Errorf("%s: %d sent home, the road behind %d: nothing checked", icao, sent, behind)
		}
	}
}

// TestRescueHomeTogether: two sent home at once from neighbouring standby
// points never touch, reverse or jump, and both get there.
func TestRescueHomeTogether(t *testing.T) {
	for _, icao := range homeAirports {
		l := smoothLayout(t, icao)
		sent, near := 0, 0
		homeEnds(l, func(rwy airport.Runway, end airport.RunwayEnd) {
			for n, pair := range homePairs(l, rwy, end) {
				crew := homeCrew(t, l, pair, nil)
				if crew == nil {
					continue
				}
				sent++
				if localDist(crew[0].pose.Position, crew[1].pose.Position) < 2*RescueSpacingMeters {
					near++
				}
				if bad := homeRun(t, crew); bad != "" {
					t.Errorf("%s %s pair %d (%.5f,%.5f and %.5f,%.5f): %s", icao, end.Name, n,
						pair[0].Position.Lat, pair[0].Position.Lon, pair[1].Position.Lat, pair[1].Position.Lon, bad)
				} else if n2 := homeStranded(crew); n2 > 0 {
					t.Errorf("%s %s pair %d: %d never got to the depot", icao, end.Name, n, n2)
				}
			}
		})
		t.Logf("%s: %d pairs sent home, %d of them within two spacings", icao, sent, near)
		if sent == 0 || near == 0 {
			t.Errorf("%s: %d pairs sent home, %d close: nothing checked", icao, sent, near)
		}
	}
}

// TestRescueHomeWhileDriving: two sent home as they drive out to standby
// carry their speed on. Before, each set off again from 0 kt in one step.
func TestRescueHomeWhileDriving(t *testing.T) {
	for _, icao := range homeAirports {
		l := smoothLayout(t, icao)
		sent, moving := 0, 0
		homeEnds(l, func(rwy airport.Runway, end airport.RunwayEnd) {
			spots := RescueStandby(l, end.Name, 2)
			if len(spots) < 2 {
				return
			}
			spots = RescueAssign(l, make([]*airport.LatLon, 2), spots)
			for _, after := range []float64{10, 30, 60, 120} {
				crew := homeCrew(t, l, spots, func(sec float64) bool { return sec >= after })
				if crew == nil {
					continue
				}
				sent++
				for _, r := range crew {
					if r.pose.GroundSpeedKts > 5 {
						moving++
					}
				}
				if bad := homeRun(t, crew); bad != "" {
					t.Errorf("%s %s, sent home %.0f s out: %s", icao, end.Name, after, bad)
				} else if n := homeStranded(crew); n > 0 {
					t.Errorf("%s %s, sent home %.0f s out: %d never got to the depot", icao, end.Name, after, n)
				}
			}
		})
		t.Logf("%s: %d pairs sent home driving, %d vehicles above 5 kt", icao, sent, moving)
		if sent == 0 || moving == 0 {
			t.Errorf("%s: %d pairs sent home, %d moving: nothing checked", icao, sent, moving)
		}
	}
}

// homeFollow has crew follow, as the World does, an aircraft rolling out
// on end from 400 m in at 120 kt, for sec seconds.
func homeFollow(t *testing.T, crew []*SimObjectRescue, end airport.RunwayEnd, sec float64) {
	t.Helper()
	s, kts := 400.0, 120.0
	for i := 0; float64(i)*homeStep < sec; i++ {
		s += kts * knot * homeStep
		kts = math.Max(0, kts-1.5/knot*homeStep)
		at := offsetHeading(end.Threshold, end.Heading, s)
		order := append([]*SimObjectRescue(nil), crew...)
		for a := 1; a < len(order); a++ {
			for b := a; b > 0 && localDist(order[b].pose.Position, at) < localDist(order[b-1].pose.Position, at); b-- {
				order[b], order[b-1] = order[b-1], order[b]
			}
		}
		var first []airport.LatLon
		for k, r := range order {
			r.KeepBehind(first)
			first = append(first, r.pose.Position)
			place := GroundPose{Position: offsetHeading(at, end.Heading+180, RescueSpacingMeters*float64(k)), Heading: end.Heading}
			if err := r.Follow(place, kts, homeStep); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// homeFollowing: the vehicles TestRescueHomeFromFollowing sends home at
// each airport.
const homeFollowing = 16

// TestRescueHomeFromFollowing: two sent home from following the aircraft,
// stopped by it or still rolling, turn out forwards and go home in their
// order. Before, they set off backwards, from 0 kt and each on its own.
func TestRescueHomeFromFollowing(t *testing.T) {
	for _, icao := range homeAirports {
		l := smoothLayout(t, icao)
		sent, behind, rolling, arrived := 0, 0, 0, 0
		homeEnds(l, func(rwy airport.Runway, end airport.RunwayEnd) {
			spots := RescueStandby(l, end.Name, 2)
			if len(spots) < 2 {
				return
			}
			spots = RescueAssign(l, make([]*airport.LatLon, 2), spots)
			for _, sec := range []float64{20, 240} {
				crew := homeCrew(t, l, spots, nil)
				if crew == nil {
					continue
				}
				homeFollow(t, crew, end, sec)
				sent++
				for _, r := range crew {
					if r.kts > 5 {
						rolling++
					} else if homeBehind(r) {
						behind++
					}
				}
				if bad := homeRun(t, crew); bad != "" {
					t.Errorf("%s %s, sent home %.0f s after the touchdown: %s", icao, end.Name, sec, bad)
				}
				arrived += len(crew) - homeStranded(crew)
			}
		})
		t.Logf("%s: %d pairs sent home following, %d vehicles rolling, the road behind %d; %d of %d vehicles got to the depot", icao, sent, rolling, behind, arrived, 2*sent)
		if sent == 0 || rolling == 0 || behind == 0 {
			t.Errorf("%s: %d pairs, %d rolling, the road behind %d: nothing checked", icao, sent, rolling, behind)
		}
		// #1656: every one drives home from the runway.
		if arrived != homeFollowing {
			t.Errorf("%s: %d of %d vehicles got to the depot, want %d", icao, arrived, 2*sent, homeFollowing)
		}
	}
}

// TestRescueWayOut (#1656): the ways out to every standby point, both
// ends of every runway, never turn back, reverse, jump or change speed in
// a step (#1630: a spike was driven backwards, a corner stood it still).
func TestRescueWayOut(t *testing.T) {
	for _, icao := range homeAirports {
		l := smoothLayout(t, icao)
		n := 0
		homeEnds(l, func(_ airport.Runway, end airport.RunwayEnd) {
			for k, spot := range RescueStandby(l, end.Name, 3) {
				c := &tugClient{}
				r := NewSimObjectRescue(c, NewInjector(c), "Truck Fire Airport Medium", 9200)
				r.Layout = l
				if err := r.Standby(spot); err != nil {
					t.Fatal(err)
				}
				if !r.hasDepot {
					t.Errorf("%s %s standby %d: no way out", icao, end.Name, k)
					continue
				}
				n++
				if at := hairpin(r.mover.Path()); at >= 0 {
					t.Errorf("%s %s standby %d: turns back %.0f m out", icao, end.Name, k, at)
				}
				r.Handle(assignedMsg(9200, 501))
				for i := 0; i < 30*60*10 && r.mover != nil; i++ {
					last := r.pose
					if err := r.Update(homeStep); err != nil {
						t.Fatal(err)
					}
					if bad := homeStepFault(last, r.pose); bad != "" {
						t.Errorf("%s %s standby %d: %s %.1f s out", icao, end.Name, k, bad, float64(i)*homeStep)
						break
					}
				}
				if r.mover != nil {
					t.Errorf("%s %s standby %d: never there", icao, end.Name, k)
				}
			}
		})
		if n == 0 {
			t.Errorf("%s: no way out checked", icao)
		}
	}
}

// homeStepFault is what is wrong with a vehicle's step from a to b.
func homeStepFault(a, b GroundPose) string {
	d := localDist(a.Position, b.Position)
	switch {
	case d > math.Max(a.GroundSpeedKts, b.GroundSpeedKts)*ktsToMS*homeStep+homeMaxMove:
		return fmt.Sprintf("jumped %.1f m", d)
	case math.Abs(b.GroundSpeedKts-a.GroundSpeedKts)*ktsToMS > homeMaxSpeed:
		return fmt.Sprintf("went from %.1f to %.1f kt in a step", a.GroundSpeedKts, b.GroundSpeedKts)
	case d > 0.005 && math.Abs(headingDiff(b.Heading, localBearing(a.Position, b.Position))) > 90:
		return "drove backwards"
	case math.Abs(headingDiff(a.Heading, b.Heading)) > homeMaxSwing:
		return fmt.Sprintf("swung %.0f°", math.Abs(headingDiff(a.Heading, b.Heading)))
	}
	return ""
}

// TestRescueHomeFromRunway (#1656): one standing anywhere on a runway,
// facing either way along it, drives to its depot and goes there.
func TestRescueHomeFromRunway(t *testing.T) {
	for _, icao := range homeAirports {
		l := smoothLayout(t, icao)
		sent, far := 0, 0
		homeEnds(l, func(rwy airport.Runway, end airport.RunwayEnd) {
			spots := RescueStandby(l, end.Name, 1)
			if len(spots) == 0 {
				return
			}
			for s := 150.0; s < rwy.Length-100; s += 400 {
				crew := homeCrew(t, l, spots[:1], nil)
				if crew == nil {
					t.Fatalf("%s %s: none to standby", icao, end.Name)
				}
				r := crew[0]
				r.pose = GroundPose{Position: offsetHeading(end.Threshold, end.Heading, s), Heading: end.Heading}
				if _, err := l.VehicleRouteJoin(NoseGear(r.pose.Position, r.pose.Heading, rescueRoadProfile()), r.depot, true, false); err != nil {
					far++
				}
				sent++
				if bad := homeRun(t, crew); bad != "" {
					t.Errorf("%s %s, %.0f m along: %s", icao, end.Name, s, bad)
				} else if homeStranded(crew) > 0 {
					t.Errorf("%s %s, %.0f m along: taken away on the runway", icao, end.Name, s)
				}
			}
		})
		t.Logf("%s: %d sent home from the runway, %d with no node in reach", icao, sent, far)
		if sent == 0 || far == 0 {
			t.Errorf("%s: %d sent, %d out of reach: nothing checked", icao, sent, far)
		}
	}
}

// TestRescueHomeStandsBeforeGone: sent home as it drives out or follows,
// with no layout, no depot or no way, it brakes to a stand before it is
// taken away. Before, it was gone from speed.
func TestRescueHomeStandsBeforeGone(t *testing.T) {
	lacks := []struct {
		what string
		set  func(r *SimObjectRescue)
	}{
		{"no layout", func(r *SimObjectRescue) { r.Layout = nil }},
		{"no depot", func(r *SimObjectRescue) { r.hasDepot = false }},
		{"no way", func(r *SimObjectRescue) { r.Layout = &airport.Layout{} }},
	}
	for _, icao := range homeAirports {
		l := smoothLayout(t, icao)
		for _, following := range []bool{false, true} {
			for _, lack := range lacks {
				sent := 0
				homeEnds(l, func(rwy airport.Runway, end airport.RunwayEnd) {
					spots := RescueStandby(l, end.Name, 1)
					if len(spots) == 0 {
						return
					}
					until := func(sec float64) bool { return sec >= 30 }
					if following {
						until = nil
					}
					crew := homeCrew(t, l, spots[:1], until)
					if crew == nil {
						return
					}
					if following {
						homeFollow(t, crew, end, 20)
					}
					r := crew[0]
					if r.pose.GroundSpeedKts <= 5 {
						return
					}
					lack.set(r)
					sent++
					where := fmt.Sprintf("%s %s, following %v, %s", icao, end.Name, following, lack.what)
					last := r.pose
					if err := r.Home(); err != nil {
						t.Fatal(err)
					}
					for i := 0; i < 5*60*10 && !r.Done(); i++ {
						if err := r.Update(homeStep); err != nil {
							t.Fatal(err)
						}
						p := r.pose
						if math.Abs(p.GroundSpeedKts-last.GroundSpeedKts)*ktsToMS > homeMaxSpeed || localDist(last.Position, p.Position) > last.GroundSpeedKts*ktsToMS*homeStep+0.1 {
							t.Errorf("%s: a jump from %.1f kt to %.1f kt", where, last.GroundSpeedKts, p.GroundSpeedKts)
							return
						}
						last = p
					}
					if !r.Done() || last.GroundSpeedKts > 0.1 {
						t.Errorf("%s: done %v at %.1f kt, want taken away standing", where, r.Done(), last.GroundSpeedKts)
					}
				})
				if sent == 0 {
					t.Errorf("%s, following %v, %s: none sent home driving: nothing checked", icao, following, lack.what)
				}
			}
		}
	}
}
