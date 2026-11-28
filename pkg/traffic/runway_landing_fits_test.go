package traffic

import (
	"math"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestLandingFitsRequiredRun (#1718): LKPR's 30 declared exactly
// RequiredLandingRun to land on, for several types and seeds. The injected
// landing (its short-field one there) is down to its exit speed by the end
// of the landing distance in every one: the rule is never shorter than
// the roll the World flies.
func TestLandingFitsRequiredRun(t *testing.T) {
	for _, model := range []string{landA320, "FSLTL B738", "FSLTL B744", "FSLTL B77W", "ATR 72", landDHC6} {
		ldaM := math.Ceil(RequiredLandingRun(model))
		g := declaredGraph(t, map[string]func(*airport.RunwayEnd){"30": func(e *airport.RunwayEnd) { e.LDAM = ldaM }})
		for seed := uint64(1); seed <= 5; seed++ {
			td, slowAt, pastKts, exitKts := landingRollOn(t, g, "30", model, seed)
			t.Logf("%-46s LDA %4.0f m seed %d: touchdown %4.0f m, at %3.0f kt by %4.0f m, %4.1f kt past the LDA end (exit %2.0f kt)", model, ldaM, seed, td, exitKts, slowAt, pastKts, exitKts)
			if pastKts > exitKts+1 {
				t.Errorf("%s seed %d: %.1f kt past the end of 30's %.0f m to land on, want %.0f kt at most", model, seed, pastKts, ldaM, exitKts+1)
			}
		}
	}
}

// landingRollOn flies an injected landing of model onto end, seeded: where
// it touched down past the landing threshold, where it was down to its
// exit speed, its fastest past the end of the landing distance, and that
// exit speed (its planned exit's).
func landingRollOn(t *testing.T, g *airport.Graph, end, model string, seed uint64) (td, slowAt, pastKts, exitKts float64) {
	t.Helper()
	ec := &eventClient{}
	inj := NewInjector(NewSimlessAI(ec))
	r, e, _ := g.Layout.RunwayEnd(end)
	var exit *airport.RunwayExit
	xs, _ := g.RunwayExits(end)
	for k := range xs {
		if exit == nil && !xs[k].HighSpeed && xs[k].Along-e.DisplacedM > e.LandingDistance(r.Length) {
			exit = &xs[k]
		}
	}
	if exit == nil {
		t.Fatalf("no right-angle exit past %s's landing distance", end)
	}
	var ctl *ArrivalController
	var err error
	for st := range g.Layout.Parking {
		if ctl != nil || !g.Layout.Parking[st].IsGate() {
			continue
		}
		c := NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj), ArrivalWithSeed(seed))
		if err = c.Start(ArrivalRequest{Graph: g, Runway: end, Parking: st, Model: model, Tail: "OKABC", Exit: exit,
			InjectApproach: true, RollThroughChance: -1, AfterLandingDwell: time.Second}); err == nil {
			ctl = c
		}
	}
	if ctl == nil {
		t.Fatalf("%s: no stand to land for: %v", model, err)
	}
	go func() {
		for range ctl.Events() {
		}
	}()
	p := ctl.Plan()
	ro := ctl.rolloutProfile()
	exitKts = ro.ExitKts
	if p.Exit.HighSpeed {
		exitKts = ro.HighSpeedExitKts
	}
	lda := p.End.LandingDistance(p.Runway.Length)
	thr := p.End.LandingThreshold()
	mon := DefaultArrivalRequestBase + arrReqMonitor
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	ctl.now = func() time.Time { return now }
	inj.now = func() time.Time { return now }
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, p.Runway.Altitude/0.3048, 3))
	type fix struct {
		along float64
		at    time.Time
	}
	var down []fix
	td, slowAt = -1, -1
	for i := 0; i < 60*1800 && ctl.State() < ArrivalVacating; i++ {
		now = now.Add(time.Second / 60)
		n := len(placements(ec))
		ctl.Handle(arrivalPositionMsg(mon, 77, p.End.Threshold, 0, 0, 0, false))
		ps := placements(ec)
		if len(ps) == n || ps[len(ps)-1].OnGround == 0 {
			continue
		}
		w := ps[len(ps)-1]
		pos := airport.LatLon{Lat: w.Latitude, Lon: w.Longitude}
		along := localDist(thr, pos) * math.Cos((localBearing(thr, pos)-p.End.Heading)*math.Pi/180)
		if td < 0 {
			td = along
		}
		down = append(down, fix{along, now})
		if len(down) <= 30 {
			continue
		}
		back := down[len(down)-31]
		kts := (along - back.along) / now.Sub(back.at).Seconds() / ktsToMS
		if slowAt < 0 && kts <= exitKts {
			slowAt = along
		}
		if along > lda {
			pastKts = math.Max(pastKts, kts)
			if along > lda+300 {
				break
			}
		}
	}
	if td < 0 {
		t.Fatalf("%s seed %d: never on the ground", model, seed)
	}
	return td, slowAt, pastKts, exitKts
}
