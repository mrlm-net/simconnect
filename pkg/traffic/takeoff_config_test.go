package traffic

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/convert"
)

// TestShortFieldFits2300: on a 2,300 m runway at sea level, ISA (TNCM),
// the types that operate such runways take off short-field (#1566): the
// heavies need it, and every one lifts off and passes 35 ft before the
// end, its RequiredTakeoffRun within the runway (dispatched).
func TestShortFieldFits2300(t *testing.T) {
	const tora = 2300.0
	for _, c := range []struct {
		model      string
		shortField bool
	}{{"FSLTL A320", false}, {"FSLTL B738", false}, {"FSLTL A359", true}, {"FSLTL B77W", true}, {"FSLTL B744", true}} {
		a := ProfileFor(c.model)
		p, cfg := ChooseTakeoff(a.Takeoff, a.Flaps, TakeoffConditions{}, tora)
		lift := liftoffRun(p)
		if cfg.ShortField != c.shortField || cfg.RequiredM > tora || lift >= tora {
			t.Errorf("%s: %+v, lift-off at %.0f m of %.0f", c.model, cfg, lift, tora)
		}
		if need := RequiredTakeoffRun(a.Takeoff, TakeoffConditions{}); need > tora {
			t.Errorf("%s needs %.0f m short-field, the runway is %.0f", c.model, need, tora)
		}
		// Short-field, the type itself: full thrust and the higher flaps.
		sf := ShortFieldTakeoff(a.Takeoff, 1)
		if sf.RollAccel <= a.Takeoff.RollAccel || sf.RotateKts >= a.Takeoff.RotateKts || a.Flaps.ShortTakeoffPct <= a.Flaps.TakeoffPct {
			t.Errorf("%s short-field %+v, flaps %+v", c.model, sf, a.Flaps)
		}
		if c.shortField && (cfg.Thrust != TakeoffThrustFull || cfg.FlapsPct != a.Flaps.ShortTakeoffPct || cfg.Flaps != a.Flaps.ShortTakeoffDetent) {
			t.Errorf("%s: %+v, want full thrust, flaps %s", c.model, cfg, a.Flaps.ShortTakeoffDetent)
		}
		t.Logf("%s: %s flaps %s, weight %.0f%%, lift-off %.0f m (%.0f%%), needs %.0f m", c.model, cfg.Thrust, cfg.Flaps, cfg.WeightPct, lift, 100*lift/tora, cfg.RequiredM)
	}
}

// TestTakeoffNormalOnLongRunways: at LKPR (06/24, 12/30) the types whose
// normal take-off fits keep it unchanged: the same profile, derated, the
// usual flaps.
func TestTakeoffNormalOnLongRunways(t *testing.T) {
	g := lkprGraph(t)
	cond := TakeoffConditions{ElevationFt: convert.MetersToFeet(g.Layout.Altitude)}
	for _, rwy := range g.Layout.Runways {
		for _, model := range []string{"FSLTL A320", "FSLTL A319", "FSLTL A321", "FSLTL B738", "FSLTL E190", "FSLTL CRJ9", "ATR 72", "FSLTL A359", "FSLTL B789"} {
			a := ProfileFor(model)
			p, cfg := ChooseTakeoff(a.Takeoff, a.Flaps, cond, rwy.Length)
			if p != a.Takeoff || cfg.ShortField || cfg.Thrust != TakeoffThrustDerate || cfg.FlapsPct != a.Flaps.TakeoffPct || cfg.WeightPct != 0 {
				t.Errorf("%s on %s (%.0f m): %+v", model, rwy.Name(), rwy.Length, cfg)
			}
		}
	}
	if _, cfg := ChooseTakeoff(DefaultTakeoffProfile(), DefaultAircraftProfile().Flaps, cond, 0); cfg.ShortField || cfg.Flaps != "1+F" {
		t.Errorf("runway not known: %+v", cfg)
	}
}

// TestTakeoffNeverPastTheEnd: whatever the runway, the chosen take-off
// leaves the ground before its end; where RequiredTakeoffRun fits, the
// configuration's run fits too.
func TestTakeoffNeverPastTheEnd(t *testing.T) {
	for _, model := range []string{"FSLTL A320", "FSLTL B738", "FSLTL A359", "FSLTL B77W", "FSLTL B744", "FSLTL A388", "FSLTL E190"} {
		a := ProfileFor(model)
		for _, cond := range []TakeoffConditions{{}, {ElevationFt: 1247, ISADeviationC: 15}} {
			need := RequiredTakeoffRun(a.Takeoff, cond)
			for tora := 1400.0; tora <= 4000; tora += 100 {
				p, cfg := ChooseTakeoff(a.Takeoff, a.Flaps, cond, tora)
				if lift := liftoffRun(p); lift >= tora {
					t.Errorf("%s %+v on %.0f m: lift-off at %.0f m", model, cond, tora, lift)
				}
				if need <= tora && cfg.RequiredM > tora {
					t.Errorf("%s %+v on %.0f m: needs %.0f m (%+v), RequiredTakeoffRun %.0f", model, cond, tora, cfg.RequiredM, cfg, need)
				}
			}
		}
	}
}

// TestTakeoffConfigToldAndFlown: an injected departure from LKPR 24 tells
// its configuration (normal for an A320) and flies the profile unchanged.
func TestTakeoffConfigToldAndFlown(t *testing.T) {
	ctl, _, run, _ := injectedDeparture(t, TaxiRequest{RollingTakeoffChance: -1})
	go func() {
		for range ctl.Events() {
		}
	}()
	if !run(TaxiDeparting, 60*1500) {
		t.Fatalf("state %v, want departing", ctl.State())
	}
	cfg := ctl.last.Takeoff
	if cfg == nil || cfg.ShortField || cfg.Thrust != TakeoffThrustDerate || cfg.Flaps != "1+F" || cfg.AvailableM <= cfg.RequiredM {
		t.Fatalf("configuration %+v", cfg)
	}
	if p := ctl.takeoffProfile(); p != DefaultTakeoffProfile() {
		t.Errorf("flies %+v, want the A320's %+v", p, DefaultTakeoffProfile())
	}
	if ctl.flaps.target != TakeoffFlapsPct {
		t.Errorf("flaps %.1f%%, want %.1f%%", ctl.flaps.target, TakeoffFlapsPct)
	}
}
