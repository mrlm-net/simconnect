package worldtest

import (
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// vfrAttitudeStepDeg: the most bank or pitch may change from one write to
// the next: a few degrees.
const vfrAttitudeStepDeg = 3.0

// TestVFRCircuitAttitude: at TFFJ a VFR departure rolls into and out of
// its first turns at a rate, its pitch blended, and so does a VFR arrival
// round its circuit and onto its final (#1636: bank 0 to 25° and pitch 9
// to 4° in one write at the departure's hand-over, level again in one
// write where the approach takes a circuit over).
func TestVFRCircuitAttitude(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	t.Run("departure", func(t *testing.T) { vfrAttitudeRun(t, "departure", false) })
	t.Run("circuit", func(t *testing.T) { vfrAttitudeRun(t, "arrival", true) })
}

func vfrAttitudeRun(t *testing.T, kind string, circuit bool) {
	const icao, rwy, cs, title = "TFFJ", "10", "VFR1", "FSLTL_DHC6_WIA-Winair"
	d := dataDir(t)
	if m, _ := filepath.Glob(filepath.Join(d, "fields", "*", icao, "layout.json")); len(m) == 0 && !fileExists(filepath.Join(d, icao, "layout.json")) {
		t.Skipf("no %s data", icao)
	}
	log := &lockedBuf{}
	o := Options{DataDir: d, ICAO: icao, Manual: true, FrameRate: 10, AircraftTitles: append(slices.Clone(DefaultAircraftTitles), title)}
	var out io.Writer = log
	if testing.Verbose() {
		out = io.MultiWriter(log, os.Stderr)
	}
	o.World.Output = out
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	h.SetUser(func(u *Object) { u.Lat, u.AltFt, u.OnGround, u.GroundKts = u.Lat+1.5, 35000, false, 450 })
	_, end, ok := h.Layout.RunwayEnd(rwy)
	if !ok {
		t.Fatalf("no runway %s at %s", rwy, icao)
	}
	wx := nav.StaticWeather(math.Mod(end.Heading+360, 360), 5, 9999, 100, 25, 1013)
	h.World.SetWeather(&wx)
	h.World.UseRunway(icao, rwy)
	h.Advance(2 * time.Second)
	back := math.Mod(end.Heading+180, 360)
	var exit *float64
	if kind == "departure" {
		exit = &back
	}
	if _, err := h.World.Do(http.MethodPost, "/api/control", world.SpawnRequest{Kind: kind, ExitBearing: exit, ICAO: icao, Tail: cs, Runway: rwy, Stand: -1, Model: title, Procedure: !circuit, Circuit: circuit, InjectApproach: true}); err != nil {
		t.Fatal(err)
	}
	var last Object
	var seen, flew bool
	var airborne time.Time
	worstBank, worstPitch, maxBank := 0.0, 0.0, 0.0
	if !h.Until(30*time.Minute, func() bool {
		now := h.Clock.Now()
		for _, o := range h.Sim.Objects() {
			if o.ATCID != cs {
				continue
			}
			if !o.OnGround {
				if !flew {
					airborne, flew = now, true
				}
				maxBank = math.Max(maxBank, math.Abs(o.Bank))
				if seen {
					if b := math.Abs(o.Bank - last.Bank); b > worstBank {
						worstBank = b
						if b > vfrAttitudeStepDeg {
							t.Logf("bank %.1f to %.1f at %.0f ft, pitch %.1f to %.1f", last.Bank, o.Bank, o.AltFt, last.Pitch, o.Pitch)
						}
					}
					if p := math.Abs(o.Pitch - last.Pitch); p > worstPitch {
						worstPitch = p
						if p > vfrAttitudeStepDeg {
							t.Logf("pitch %.1f to %.1f at %.0f ft", last.Pitch, o.Pitch, o.AltFt)
						}
					}
				}
				last, seen = o, true
			} else {
				seen = false
				if flew {
					return true
				}
			}
		}
		v, ok := h.Aircraft(cs)
		if kind == "departure" {
			return flew && now.Sub(airborne) > 4*time.Minute
		}
		return ok && v.Done && flew
	}) {
		t.Fatalf("%s did not fly in 30 minutes", cs)
	}
	t.Logf("bank up to %.1f°; a write changed bank by %.2f° and pitch by %.2f° at most", maxBank, worstBank, worstPitch)
	if maxBank < 5 {
		t.Errorf("never turned")
	}
	if worstBank > vfrAttitudeStepDeg {
		t.Errorf("bank stepped %.1f° in one write, want at most %.0f", worstBank, vfrAttitudeStepDeg)
	}
	if worstPitch > vfrAttitudeStepDeg {
		t.Errorf("pitch stepped %.1f° in one write, want at most %.0f", worstPitch, vfrAttitudeStepDeg)
	}
}
