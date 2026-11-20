package traffic

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Landings as the pose stream writes them (#1606, TNCM 10, KLM685 B744):
// the pitch snapped from the flare attitude to the ground attitude in one
// write and the altitude stepped 0.4 ft down with it, still airborne, on a
// model that lands flat (its struts as learnt beyond strutsFlatFt). From
// the flare to the rollout handover the written pitch moves at most
// landingPitchRateMax a second, and the altitude never steps beyond its
// trend from one write to the next.
func TestLandingDerotatesSmoothly(t *testing.T) {
	type typeCase struct {
		model         string
		cg, struts    float64
		flat          bool
		minDerotation float64
	}
	kinds := []typeCase{
		{"A320", 12.25, 0.62, false, 1.5},
		{"B744", 18.50, 0.80, false, 1.5},
		{"B744", 18.50, 1.55, true, 1.5},
		{"B77W", 17.20, 1.40, true, 1.5},
	}
	for _, ap := range []struct{ icao, end string }{{"LKPR", "24"}, {"LKPR", "06"}, {"TNCM", "10"}} {
		g := landingGraph(t, ap.icao)
		if g == nil {
			continue
		}
		for _, c := range kinds {
			name := ap.icao + "-" + ap.end + "-" + c.model
			if c.flat {
				name += "-flat"
			}
			t.Run(name, func(t *testing.T) {
				ps := landingPoses(t, g, ap.end, c.model, c.cg, c.struts)
				if len(ps) < 60 {
					t.Fatalf("only %d writes", len(ps))
				}
				checkLandingPoses(t, ps, c.minDerotation, c.flat)
			})
		}
	}
}

// landingPitchRateMax: the written pitch moves at most this many degrees a
// second through the flare and derotation; landingAltJerkFt: the altitude's
// change from one write to the next departs from the previous one by at
// most this (a step; the glide path, flare and blend change it gently).
const (
	landingPitchRateMax = 3.5
	landingAltJerkFt    = 0.05
)

type landingWrite struct {
	p      types.SIMCONNECT_DATA_INITPOSITION
	ground float64
}

func checkLandingPoses(t *testing.T, ps []landingWrite, minDerotation float64, flat bool) {
	t.Helper()
	const dt = 1.0 / 60
	nosedUp, downAt, levelAt := -1, -1, -1
	worstRate, worstJerk := 0.0, 0.0
	for k := 1; k < len(ps); k++ {
		a, b := ps[k-1].p, ps[k].p
		if nosedUp < 0 && b.Pitch < -3 {
			nosedUp = k
		}
		if downAt < 0 && b.OnGround != 0 {
			downAt = k
		}
		if nosedUp >= 0 && levelAt < 0 && b.Pitch > -0.5 {
			levelAt = k
		}
		rate := math.Abs(b.Pitch-a.Pitch) / dt
		if rate > worstRate {
			worstRate = rate
		}
		if rate > landingPitchRateMax {
			t.Errorf("write %d: pitch %.2f → %.2f (%.1f°/s), onGround %d, alt %.2f → %.2f", k, a.Pitch, b.Pitch, rate, b.OnGround, a.Altitude, b.Altitude)
		}
		if k >= 2 {
			jerk := math.Abs((b.Altitude - a.Altitude) - (a.Altitude - ps[k-2].p.Altitude))
			if jerk > worstJerk {
				worstJerk = jerk
			}
			if jerk > landingAltJerkFt {
				t.Errorf("write %d: altitude %.3f → %.3f → %.3f (step %.3f ft), pitch %.2f → %.2f, onGround %d", k, ps[k-2].p.Altitude, a.Altitude, b.Altitude, jerk, a.Pitch, b.Pitch, b.OnGround)
			}
		}
	}
	if downAt < 0 || levelAt < 0 {
		t.Fatalf("no touchdown (%d) or no level nose (%d)", downAt, levelAt)
	}
	derot := float64(levelAt-nosedUp) * dt
	t.Logf("%d writes, worst pitch rate %.2f°/s, worst altitude step %.3f ft; on ground at write %d, nose up from %d, level at %d", len(ps), worstRate, worstJerk, downAt, nosedUp, levelAt)
	switch {
	case flat && downAt < levelAt:
		t.Errorf("lands flat: the on-ground flag (write %d) before the nose is down (write %d)", downAt, levelAt)
	case !flat && levelAt <= downAt:
		t.Errorf("nose down (write %d) before the on-ground flag (write %d)", levelAt, downAt)
	case !flat && ps[downAt].p.Pitch > -3:
		t.Errorf("on the ground at pitch %.2f, not the flare attitude", ps[downAt].p.Pitch)
	}
	if derot < minDerotation {
		t.Errorf("nose down %.2f s after the flare attitude, want ≥ %.1f", derot, minDerotation)
	}
}

// landingGraph is icao's taxi graph: the test data, else the
// simconnect-data clone ($SIMCONNECT_DATA); nil (skipped) without either.
func landingGraph(t *testing.T, icao string) *airport.Graph {
	t.Helper()
	if _, err := os.Stat("../airport/testdata/" + icao + ".json"); err == nil {
		return airportGraph(t, icao)
	}
	dir := os.Getenv("SIMCONNECT_DATA")
	if dir == "" {
		dir = filepath.Join("..", "..", "..", "simconnect-data")
	}
	b, err := os.ReadFile(filepath.Join(dir, icao, "layout.json"))
	if err != nil {
		t.Logf("no %s layout (SIMCONNECT_DATA): skipped", icao)
		return nil
	}
	var raw airport.RawAirport
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	l, err := airport.BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	g, err := airport.BuildGraph(l)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// landingPoses flies an injected arrival of model onto end and returns the
// positions written from 60 ft above the runway to a few seconds after
// the rollout handover; its static CG height cg and struts as learnt.
func landingPoses(t *testing.T, g *airport.Graph, end, model string, cg, struts float64) []landingWrite {
	t.Helper()
	return landingPosesOn(t, g, end, model, cg, struts, false)
}

// landingPosesOn is landingPoses, with no simulator behind the injector
// (SimlessAI, a host such as Vector) when simless.
func landingPosesOn(t *testing.T, g *airport.Graph, end, model string, cg, struts float64, simless bool) []landingWrite {
	t.Helper()
	ec := &eventClient{}
	inj := NewInjector(ec)
	if simless {
		inj = NewInjector(NewSimlessAI(ec))
	}
	var ctl *ArrivalController
	var err error
	exits := []*airport.RunwayExit{nil}
	if xs, xerr := g.RunwayExits(end); xerr == nil {
		for k := len(xs) - 1; k >= 0; k-- {
			exits = append(exits, &xs[k])
		}
	}
	started := false
	for _, x := range exits {
		for st := range g.Layout.Parking {
			if started || !g.Layout.Parking[st].IsGate() {
				continue
			}
			ctl = NewArrivalController(NewFleet(ec), ArrivalWithInjector(inj))
			if err = ctl.Start(ArrivalRequest{Graph: g, Runway: end, Parking: st, Model: model, Tail: "OKABC", Exit: x,
				InjectApproach: true, RollThroughChance: -1, AfterLandingDwell: time.Second}); err == nil {
				started = true
			}
		}
	}
	if !started {
		t.Fatalf("no stand to land for: %v", err)
	}
	go func() {
		for range ctl.Events() {
		}
	}()
	p := ctl.Plan()
	mon := DefaultArrivalRequestBase + arrReqMonitor
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	ctl.now = func() time.Time { return now }
	inj.now = func() time.Time { return now }
	groundFt := p.Runway.Altitude / 0.3048
	inj.mu.Lock()
	inj.struts[strutsKey(cg)] = struts
	inj.mu.Unlock()
	ctl.Handle(assignedMsg(DefaultArrivalRequestBase, 77))
	inj.Handle(groundMsg(DefaultInjectRequestBase+1, 77, groundFt, cg))
	var out []landingWrite
	handed := -1
	for i := 0; i < 60*1200 && ctl.State() < ArrivalVacating; i++ {
		now = now.Add(time.Second / 60)
		n := len(placements(ec))
		ctl.Handle(arrivalPositionMsg(mon, 77, p.End.Threshold, 0, 0, 0, false))
		ps := placements(ec)
		if len(ps) == n {
			continue
		}
		w := ps[len(ps)-1]
		if w.Altitude-groundFt > 60+cg && len(out) == 0 {
			continue
		}
		out = append(out, landingWrite{p: w, ground: groundFt})
		if handed < 0 && ctl.approach == nil {
			handed = len(out)
		}
		if handed >= 0 && len(out) > handed+60*3 {
			break
		}
	}
	return out
}
