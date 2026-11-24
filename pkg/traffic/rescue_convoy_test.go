package traffic

import (
	"math"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// The rescue convoy's speed and stops (#1643). Nothing here is newer than
// v0.188.13 but Attend, asked for by its method, so it runs there.

// convoySteps drives r by step for sec seconds and is its first fault.
func convoySteps(t *testing.T, r *SimObjectRescue, sec float64, step func(sec float64) error) string {
	t.Helper()
	for i := 0; float64(i)*homeStep < sec; i++ {
		last := r.pose
		if err := step(float64(i) * homeStep); err != nil {
			t.Fatal(err)
		}
		if r.Done() {
			return ""
		}
		if bad := homeStepFault(last, r.pose); bad != "" {
			return bad
		}
	}
	return ""
}

// convoyDriving is a vehicle driving out to standby above 10 kt, nil when
// none ever does.
func convoyDriving(t *testing.T, l *airport.Layout, spot GroundPose) *SimObjectRescue {
	t.Helper()
	c := &tugClient{}
	r := NewSimObjectRescue(c, NewInjector(c), "Truck Fire Airport Medium", 9200)
	r.Layout = l
	if err := r.Standby(spot); err != nil {
		t.Fatal(err)
	}
	r.Handle(assignedMsg(9200, 501))
	for i := 0; i < 120*10 && r.mover != nil; i++ {
		if err := r.Update(homeStep); err != nil {
			t.Fatal(err)
		}
		if r.pose.GroundSpeedKts > 10 {
			return r
		}
	}
	return nil
}

// TestRescueEnterDriving: sent to the aircraft as it drives out, with a
// way there or none, it never changes speed in a step. Before, it set off
// again from 0 kt.
func TestRescueEnterDriving(t *testing.T) {
	for _, icao := range homeAirports {
		l := smoothLayout(t, icao)
		g, err := airport.BuildGraph(l)
		if err != nil {
			t.Fatal(err)
		}
		sent := 0
		homeEnds(l, func(rwy airport.Runway, end airport.RunwayEnd) {
			spots := RescueStandby(l, end.Name, 1)
			if len(spots) == 0 {
				return
			}
			at := offsetHeading(end.Threshold, end.Heading, rwy.Length/2)
			way, ok := RescueAccess(g, end.Name, at, false)
			if !ok {
				return
			}
			for _, none := range []bool{false, true} {
				r := convoyDriving(t, l, spots[0])
				if r == nil {
					continue
				}
				to := way
				if none {
					to = RescueWay{Node: offsetHeading(end.Threshold, end.Heading+90, 20000)}
				}
				sent++
				if err := r.Enter(to); err != nil {
					t.Fatal(err)
				}
				place := GroundPose{Position: at, Heading: end.Heading}
				if bad := convoySteps(t, r, 300, func(float64) error { return r.Follow(place, 0, homeStep) }); bad != "" {
					t.Errorf("%s %s, no way %v: %s", icao, end.Name, none, bad)
				}
			}
		})
		if sent == 0 {
			t.Errorf("%s: none sent in driving: nothing checked", icao)
		}
	}
}

// TestRescueFollowRate: following at speed, its place moved onto it or
// behind it, or one turning up ahead, it brakes at a rate and does not
// turn back. Before, it stood in one step.
func TestRescueFollowRate(t *testing.T) {
	l := smoothLayout(t, "LKPR")
	_, end, ok := l.RunwayEnd("24")
	if !ok {
		t.Fatal("no runway 24")
	}
	for _, c := range []struct {
		what  string
		place float64 // the aircraft this far past the vehicle
		block bool
	}{
		{what: "its place on it", place: RescueGapMeters + 2},
		{what: "its place behind it", place: RescueGapMeters - 30},
		{what: "one ahead", place: 600, block: true},
	} {
		tc := &tugClient{}
		r := NewSimObjectRescue(tc, NewInjector(tc), "Truck Fire Airport Medium", 9200)
		if err := r.Standby(GroundPose{Position: offsetHeading(end.Threshold, end.Heading, 200), Heading: end.Heading}); err != nil {
			t.Fatal(err)
		}
		r.Handle(assignedMsg(9200, 501))
		ac := offsetHeading(r.pose.Position, end.Heading, 1500)
		if bad := convoySteps(t, r, 15, func(float64) error {
			return r.Follow(GroundPose{Position: ac, Heading: end.Heading}, 20, homeStep)
		}); bad != "" {
			t.Fatalf("%s, running up: %s", c.what, bad)
		}
		if r.pose.GroundSpeedKts < 20 {
			t.Fatalf("%s: only %.0f kt, nothing checked", c.what, r.pose.GroundSpeedKts)
		}
		ac = offsetHeading(r.pose.Position, end.Heading, c.place)
		if bad := convoySteps(t, r, 30, func(float64) error {
			if c.block {
				r.KeepBehind([]airport.LatLon{offsetHeading(r.pose.Position, end.Heading, RescueSpacingMeters-1)})
			}
			return r.Follow(GroundPose{Position: ac, Heading: end.Heading}, 0, homeStep)
		}); bad != "" {
			t.Errorf("%s: %s", c.what, bad)
		}
		if d := math.Abs(headingDiff(r.pose.Heading, end.Heading)); d > 90 {
			t.Errorf("%s: turned back, %.0f° round", c.what, d)
		}
		if r.pose.GroundSpeedKts > 0.1 {
			t.Errorf("%s: still at %.1f kt", c.what, r.pose.GroundSpeedKts)
		}
	}
}

// TestRescueEnterHolds: on its way in it keeps off the runway it is held
// off, held from the start or as it rolls, at a rate, and goes on once let.
func TestRescueEnterHolds(t *testing.T) {
	for _, icao := range homeAirports {
		l := smoothLayout(t, icao)
		g, err := airport.BuildGraph(l)
		if err != nil {
			t.Fatal(err)
		}
		checked := 0
		homeEnds(l, func(rwy airport.Runway, end airport.RunwayEnd) {
			spots := RescueStandby(l, end.Name, 1)
			if len(spots) == 0 {
				return
			}
			at := offsetHeading(end.Threshold, end.Heading, rwy.Length/2)
			way, ok := RescueAccess(g, end.Name, at, false)
			if !ok {
				return
			}
			place := GroundPose{Position: at, Heading: end.Heading}
			on := func(r *SimObjectRescue, margin float64) bool {
				rw, on := l.RunwayAt(r.pose.Position, margin)
				return on && rw.Index == rwy.Index
			}
			for _, after := range []float64{0, 2, 5, 10, 15} {
				crew := homeCrew(t, l, spots[:1], nil)
				if crew == nil {
					return
				}
				r := crew[0]
				if err := r.Enter(way); err != nil {
					t.Fatal(err)
				}
				if bad := convoySteps(t, r, after, func(float64) error { return r.Follow(place, 0, homeStep) }); bad != "" {
					t.Errorf("%s %s, driving in: %s", icao, end.Name, bad)
					continue
				}
				// Room to stop short of the runway's edge at rescueAccel.
				v := r.pose.GroundSpeedKts * ktsToMS
				ahead := v*v/(2*rescueAccel) + RescueRunwayMarginMeters
				if on(r, ahead) {
					continue
				}
				checked++
				r.HoldOffRunway(&rwy)
				entered := false
				if bad := convoySteps(t, r, 180, func(float64) error {
					err := r.Follow(place, 0, homeStep)
					entered = entered || on(r, 0)
					return err
				}); bad != "" {
					t.Errorf("%s %s, held %.0f s in: %s", icao, end.Name, after, bad)
				}
				if entered {
					t.Errorf("%s %s, held %.0f s in: drove onto the runway", icao, end.Name, after)
				}
				r.HoldOffRunway(nil)
				if bad := convoySteps(t, r, 300, func(float64) error { return r.Follow(place, 0, homeStep) }); bad != "" {
					t.Errorf("%s %s, let go: %s", icao, end.Name, bad)
				}
				if d := localDist(r.pose.Position, at); d > RescueGapMeters+RescueJoinMeters+20 {
					t.Errorf("%s %s, let go: %.0f m from the aircraft", icao, end.Name, d)
				}
			}
		})
		if checked == 0 {
			t.Errorf("%s: none held: nothing checked", icao)
		}
	}
}

// TestRescueHomeOffBody: sent home from behind or in front of the
// aircraft it attends, facing it, a small or a very large one, it drives
// to its depot without running into the body. Before, only the gap kept
// it off.
func TestRescueHomeOffBody(t *testing.T) {
	small := DefaultMotionProfile()
	large := small
	large.WheelbaseMeters, large.TailMeters, large.SpanMeters = 32, 35, 80
	for _, icao := range homeAirports {
		l := smoothLayout(t, icao)
		checked := 0
		homeEnds(l, func(rwy airport.Runway, end airport.RunwayEnd) {
			spots := RescueStandby(l, end.Name, 1)
			if len(spots) == 0 {
				return
			}
			for _, prof := range []MotionProfile{small, large} {
				for _, s := range []float64{0.25, 0.5, 0.75} {
					for _, side := range []float64{-1, 1} {
						crew := homeCrew(t, l, spots[:1], nil)
						if crew == nil {
							return
						}
						r := crew[0]
						ac := GroundPose{Position: offsetHeading(end.Threshold, end.Heading, s*rwy.Length), Heading: end.Heading}
						h := end.Heading
						if side > 0 {
							h = math.Mod(h+180, 360)
						}
						r.pose = GroundPose{Position: offsetHeading(ac.Position, end.Heading, side*RescueGapMeters), Heading: h}
						if a, ok := any(r).(interface {
							Attend(GroundPose, MotionProfile)
						}); ok {
							a.Attend(ac, prof)
						}
						// The body itself: the frame without its margin at the
						// wingtips.
						body := prof
						body.SpanMeters -= 2 * keepMarginM
						f := aircraftFrame{nose: NoseGear(ac.Position, ac.Heading, prof), hdg: ac.Heading, prof: body}
						if err := r.Home(); err != nil {
							t.Fatal(err)
						}
						checked++
						hit := false
						bad := convoySteps(t, r, 30*60, func(float64) error {
							err := r.Update(homeStep)
							front := NoseGear(r.pose.Position, r.pose.Heading, rescueRoadProfile())
							hit = hit || f.inside(r.pose.Position) || f.inside(front)
							return err
						})
						switch {
						case bad != "":
							t.Errorf("%s %s, %.2f along, side %.0f, span %.0f: %s", icao, end.Name, s, side, prof.SpanMeters, bad)
						case hit:
							t.Errorf("%s %s, %.2f along, side %.0f, span %.0f: into the aircraft", icao, end.Name, s, side, prof.SpanMeters)
						case !r.Done() || !homeArrived(r):
							t.Errorf("%s %s, %.2f along, side %.0f, span %.0f: never at its depot", icao, end.Name, s, side, prof.SpanMeters)
						}
					}
				}
			}
		})
		if checked == 0 {
			t.Errorf("%s: nothing checked", icao)
		}
	}
}
