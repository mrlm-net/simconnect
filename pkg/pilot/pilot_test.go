//go:build windows

package pilot

import (
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/flight"
	"github.com/mrlm-net/simconnect/pkg/systems"
)

// sim is a crude aircraft on its autopilot: it climbs or descends to the
// selected altitude at 2000 ft/min, changes speed at 1 kt/s toward the
// selected one, and covers ground at its speed.
type sim struct {
	in      Input
	flaps   float64
	actions []string
	player  bool // the player does what is asked
	asked   []string
}

func newSim(player bool) *sim {
	s := &sim{player: player, flaps: 2}
	s.in = Input{Now: time.Unix(0, 0),
		State: systems.State{Values: map[string]float64{systems.FlapsIndex: 2, systems.DesignVS0: 110}, DHFt: 200},
		Air:   flight.Sample{AltFt: 600, GroundFt: 300, IAS: 160, VS: 2000, GearHandle: true},
		Plan:  Plan{CruiseFt: 35000, DestElevFt: 300, DistanceToGoNM: 250},
		ATC:   Clearance{AltitudeFt: 10000}}
	return s
}

func (s *sim) apply(a Action) {
	s.actions = append(s.actions, a.String())
	ap := &s.in.State.AP
	switch a.Name {
	case systems.APMaster:
		ap.Master = *a.On
	case systems.APAltitudeSel:
		ap.AltitudeSel = *a.Value
	case systems.APSpeedSel:
		ap.SpeedSel = *a.Value
	case systems.APHeadingSel:
		ap.HeadingSel = *a.Value
	case systems.APNav:
		ap.Nav = *a.On
	case systems.APFLC:
		ap.FLC = *a.On
	case systems.APApproach:
		ap.ApproachArmed = *a.On
	case systems.ATHR:
		ap.ATHR = *a.On
	case systems.GearDown:
		s.in.Air.GearHandle = *a.On
	case systems.FlapsUp:
		s.flaps = math.Max(0, s.flaps-1)
	case systems.FlapsDown:
		s.flaps = math.Min(4, s.flaps+1)
	}
}

func (s *sim) step(e *Engine) Output {
	out := e.Update(s.in)
	for _, a := range out.Actions {
		s.apply(a)
	}
	for _, r := range out.Requests {
		s.asked = append(s.asked, r.Say)
		if s.player {
			for _, a := range r.Do {
				s.apply(a)
			}
		}
	}
	// The aircraft.
	in := &s.in
	in.Now = in.Now.Add(time.Second)
	ap := in.State.AP
	if ap.Master {
		d := ap.AltitudeSel - in.Air.AltFt
		in.Air.VS = math.Max(-2000, math.Min(2000, d*10))
		if (ap.ApproachArmed || ap.Glideslope) && in.Plan.DistanceToGoNM < 9 {
			in.State.AP.Glideslope = true
			in.Air.VS = -700
		}
		dv := ap.SpeedSel - in.Air.IAS
		in.Air.IAS += math.Max(-1, math.Min(1, dv))
	}
	in.Air.AltFt += in.Air.VS / 60
	in.Plan.DistanceToGoNM = math.Max(0.1, in.Plan.DistanceToGoNM-in.Air.IAS/3600*1.3)
	in.State.Values[systems.FlapsIndex] = s.flaps
	return out
}

// TestFlight: the player takes off; at 1000 ft the copilot takes the
// controls on the autopilot, asks for the gear and the flaps up, climbs to
// the cleared altitude and then cruise, asks for descent at the top of
// descent, slows down asking for each flap step and the gear, arms the
// approach once cleared, and gives the controls back at minimums.
func TestFlight(t *testing.T) {
	s := newSim(true)
	e := New(Config{}, []string{"zero", "one", "two", "three", "full"})
	var says []string
	handback := false
	for i := 0; i < 3*3600 && !handback; i++ {
		switch {
		case e.Phase() == PhaseClimb && s.in.Air.AltFt > 9800:
			s.in.ATC.AltitudeFt = 35000
		case e.Phase() == PhaseDescent && s.in.ATC.AltitudeFt == 35000:
			s.in.ATC.AltitudeFt = 3000
		case e.Phase() == PhaseApproach && s.in.Plan.DistanceToGoNM < 15:
			s.in.ATC.Approach = true
		}
		out := s.step(e)
		says = append(says, out.Say...)
		handback = out.Handback
	}
	all := strings.Join(says, " | ")
	if !handback {
		t.Fatalf("no handback; phase %v at %.0f ft, %.1f NM: %s", e.Phase(), s.in.Air.AltFt, s.in.Plan.DistanceToGoNM, all)
	}
	order := []string{"I have control", "Gear up", "Flaps one", "Flaps zero", "Top of descent", "Flaps one", "Flaps two", "Gear down", "Flaps three", "Flaps full", "Your controls"}
	i := 0
	for _, s := range says {
		if i < len(order) && s == order[i] {
			i++
		}
	}
	if i < len(order) {
		t.Errorf("said in order up to %q, missing %q:\n%s", order[:i], order[i], all)
	}
	if !slices.Contains(says, "Approach armed") {
		t.Errorf("approach not armed: %s", all)
	}
	if len(s.actions) > 200 {
		t.Errorf("%d actions: flooding", len(s.actions))
	}
	t.Logf("%d actions; said: %s", len(s.actions), all)
}

// TestPMTimeout: a player who does not act is asked once; with
// CopilotActs the copilot does it after the timeout, without it the
// request waits.
func TestPMTimeout(t *testing.T) {
	for _, acts := range []bool{false, true} {
		s := newSim(false)
		s.in.Air.AltFt, s.in.Air.IAS = 2000, 200
		e := New(Config{CopilotActs: acts, PMTimeout: 5 * time.Second}, []string{"zero", "one", "two", "three", "full"})
		var timedOut int
		for range 20 {
			out := s.step(e)
			timedOut += len(out.TimedOut)
		}
		gearAsks := 0
		for _, a := range s.asked {
			if a == "Gear up" {
				gearAsks++
			}
		}
		if gearAsks != 1 {
			t.Errorf("acts %v: gear up asked %d times", acts, gearAsks)
		}
		if acts == (timedOut == 0) || acts == s.in.Air.GearHandle {
			t.Errorf("acts %v: timed out %d, gear down %v", acts, timedOut, s.in.Air.GearHandle)
		}
	}
}

// TestWithLearned: learned values fill the defaults, never what is set.
func TestWithLearned(t *testing.T) {
	l := flight.Learned{AccelAGLFt: 2000, GearDownAGLFt: 1800, ApproachKts: 137,
		FlapsDownKts: map[int]float64{1: 205, 2: 185, 3: 170, 4: 150}, FlapsDownAGLFt: map[int]float64{1: 6000, 2: 4000, 3: 2000, 4: 1400}}
	c := Config{ApproachKts: 140}.WithLearned(l)
	if c.AccelAGLFt != 2000 || c.GearDownAGLFt != 1800 || c.ApproachKts != 140 || c.FlapStepKts != 18 || c.FinalFlapsAGLFt != 1400 {
		t.Errorf("config %+v", c)
	}
}

// TestTakeOverInCruise: switched on in cruise the engine takes the
// controls at once, in the cruise phase; HandBack gives them back ("Your
// controls"), TakeControl takes them again ("I have control"), the
// autopilot already on.
func TestTakeOverInCruise(t *testing.T) {
	s := newSim(true)
	s.in.Air.AltFt, s.in.Air.VS, s.in.Air.IAS, s.in.Air.GearHandle = 35000, 0, 280, false
	s.in.ATC.AltitudeFt = 35000
	s.flaps = 0
	s.in.State.Values[systems.FlapsIndex] = 0
	e := New(Config{}, nil)
	out := s.step(e)
	if e.Phase() != PhaseCruise || !slices.Contains(out.Say, "I have control") || !s.in.State.AP.Master {
		t.Fatalf("phase %v, said %v, AP %v", e.Phase(), out.Say, s.in.State.AP.Master)
	}
	if got := e.HandBack(); len(got) != 1 || got[0] != "Your controls" || e.Phase() != PhaseHandback {
		t.Fatalf("handback said %v, phase %v", got, e.Phase())
	}
	if out := s.step(e); len(out.Actions) != 0 {
		t.Errorf("handed back, still acting: %v", out.Actions)
	}
	e.TakeControl()
	out = s.step(e)
	if e.Phase() != PhaseCruise || len(out.Say) != 1 || out.Say[0] != "I have control" {
		t.Errorf("taken again: phase %v, said %v", e.Phase(), out.Say)
	}
	if New(Config{}, nil).HandBack() != nil {
		t.Error("a handback before ever flying said something")
	}
}

// TestAutopilotGoAround: told to go around on the autopilot (not HandFly),
// the copilot calls it, sets TOGA, asks for one flap step, drops the
// approach, holds the runway heading and climbs to ATC's altitude by level
// change; above the acceleration height the climb takes over, and the next
// approach is armed again.
func TestAutopilotGoAround(t *testing.T) {
	s := newSim(true)
	e := New(Config{}, nil)
	s.in.State.AP = systems.AutopilotState{Master: true, ATHR: true, ApproachArmed: true}
	s.in.Air = flight.Sample{AltFt: 1300, GroundFt: 300, IAS: 140, VS: -700, GearHandle: true, Heading: 160}
	s.in.Plan.DistanceToGoNM = 4
	s.in.ATC = Clearance{Approach: true, AltitudeFt: 5000}
	e.phase, e.taking = PhaseApproach, false
	e.said["appr"] = true // armed on this approach
	s.in.ATC.GoAround = true
	out := s.step(e)
	if e.Phase() != PhaseGoAround || !slices.Contains(out.Say, "Go around, flaps") {
		t.Fatalf("phase %v, said %q", e.Phase(), out.Say)
	}
	for _, want := range []string{systems.TOGA, systems.APFLC, systems.APHeadingHold, systems.APApproach} {
		if !slices.ContainsFunc(out.Actions, func(a Action) bool { return a.Name == want }) {
			t.Errorf("no %s in %v", want, s.actions)
		}
	}
	if s.in.State.AP.AltitudeSel != 5000 || s.in.State.AP.HeadingSel != 160 {
		t.Errorf("selected %v ft, heading %v", s.in.State.AP.AltitudeSel, s.in.State.AP.HeadingSel)
	}
	if !slices.Contains(s.asked, "Flaps 1") {
		t.Errorf("asked %q, want a flap step", s.asked)
	}
	s.in.ATC.GoAround = false
	for range 120 {
		if s.step(e); e.Phase() != PhaseGoAround {
			break
		}
	}
	if e.Phase() != PhaseClimb {
		t.Fatalf("phase %v after the go-around, want climb", e.Phase())
	}
	if s.in.Air.GearHandle {
		t.Error("gear still down")
	}
	e.phase = PhaseApproach
	s.in.State.AP.ApproachArmed = false
	if out := s.step(e); !slices.Contains(out.Say, "Approach armed") {
		t.Errorf("next approach not armed: %q", out.Say)
	}
}
