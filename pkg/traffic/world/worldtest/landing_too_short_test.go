package worldtest

import (
	"math"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// landingShortLDAM: XDEC's 27 declared this long to land on is shorter
// than an A320's landing run (traffic.RequiredLandingRun, 1368 m).
const landingShortLDAM = 1300.0

// TestLandingTooShort (#1718): at XDEC (declared_field_test.go) with the
// wind down 27. With landingShortLDAM to land on, an A320 arrival asked
// for there is refused, the reason said ("too short"), and none appears:
// it is not planned onto a landing distance it cannot stop in, nor onto
// 09 against the flow. In declaredLDAM (1500 m, which one rolled past at
// 31-43 kt before it flew its short-field landing there), and in exactly
// its landing run, it lands on 27, is down to its exit speed by the end
// of the landing distance, and parks; a DHC-6 lands in 1500 m.
func TestLandingTooShort(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	t.Run("refused", func(t *testing.T) {
		h, _ := landingField(t, landingShortLDAM)
		_, err := h.World.Do(http.MethodPost, "/api/control", world.SpawnRequest{Kind: "arrival", ICAO: declaredICAO, Tail: "XDA1", Stand: 2, Model: declaredJet, InjectApproach: true, SpawnNm: 8})
		if err == nil || !strings.Contains(err.Error(), "too short") {
			t.Fatalf("A320 to XDEC 27 (%.0f m to land on): %v, want refused as too short", landingShortLDAM, err)
		}
		t.Logf("refused: %v", err)
		h.Advance(30 * time.Second)
		if v, ok := h.Aircraft("XDA1"); ok {
			t.Errorf("XDA1 there after all: %+v", v)
		}
	})
	t.Run("fits", func(t *testing.T) { landingRoll(t, declaredJet, declaredLDAM) })
	t.Run("exactly", func(t *testing.T) { landingRoll(t, declaredJet, math.Ceil(traffic.RequiredLandingRun(declaredJet))) })
	t.Run("light", func(t *testing.T) { landingRoll(t, declaredTitle, declaredLDAM) })
}

// landingField is XDEC with 27's landing distance ldaM, the wind down 27,
// and the harness on it; 27's end.
func landingField(t *testing.T, ldaM float64) (*Harness, airport.RunwayEnd) {
	t.Helper()
	dir := declaredFieldDir(t, false)
	if ldaM != declaredLDAM {
		p := filepath.Join(dir, declaredICAO, "layout.json")
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		from := `"secondaryLdaM":1500`
		if !strings.Contains(string(b), from) {
			t.Fatalf("no %s in XDEC's layout", from)
		}
		b = []byte(strings.Replace(string(b), from, `"secondaryLdaM":`+strconv.FormatFloat(ldaM, 'f', -1, 64), 1))
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	o := Options{DataDir: dir, ICAO: declaredICAO, Manual: true, FrameRate: 10,
		AircraftTitles: append(slices.Clone(DefaultAircraftTitles), declaredTitle)}
	o.World.Output = &lockedBuf{}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	h.SetUser(func(u *Object) { u.Lat, u.AltFt, u.OnGround, u.GroundKts = u.Lat+1.5, 35000, false, 450 })
	_, e, ok := h.Layout.RunwayEnd("27")
	if !ok || e.LDAM != ldaM {
		t.Fatalf("27 lands in %.0f m, want %.0f", e.LDAM, ldaM)
	}
	wx := nav.StaticWeather(270, 3, 9999, 15, 5, 1013)
	h.World.SetWeather(&wx)
	h.Advance(2 * time.Second)
	return h, e
}

// landingRoll has model land on XDEC's 27 declared ldaM to land on: down
// to its exit speed (and 3 kt) by the end of it, parked, and not jumping.
func landingRoll(t *testing.T, model string, ldaM float64) {
	h, e := landingField(t, ldaM)
	jumps := len(h.Sim.Jumps())
	if _, err := h.World.Do(http.MethodPost, "/api/control", world.SpawnRequest{Kind: "arrival", ICAO: declaredICAO, Tail: "XDA1", Stand: 2, Model: model, InjectApproach: true, SpawnNm: 8}); err != nil {
		t.Fatalf("%s to 27 (%.0f m to land on): %v", model, ldaM, err)
	}
	slowKts := traffic.ExitSpeedKts + 3
	var landed, parked bool
	pastKts, pastAt := 0.0, ""
	h.Until(30*time.Minute, func() bool {
		v, ok := h.Aircraft("XDA1")
		if !ok || v.ObjectID == 0 {
			return false
		}
		for _, ob := range h.Sim.Objects() {
			if ob.ID != v.ObjectID {
				continue
			}
			along, across := alongAcross(e.LandingThreshold(), airport.LatLon{Lat: ob.Lat, Lon: ob.Lon}, e.Heading)
			if ob.OnGround && math.Abs(across) < 15 && along > 0 && math.Abs(math.Remainder(ob.Heading-e.Heading, 360)) < 45 {
				landed = true
				if along > ldaM && ob.GroundKts > slowKts && ob.GroundKts > pastKts {
					pastKts, pastAt = ob.GroundKts, v.State
				}
			}
		}
		parked = parked || v.State == "parked"
		return parked
	})
	if !landed || !parked {
		t.Errorf("%s on 27 (%.0f m to land on): landed %v, parked %v", model, ldaM, landed, parked)
	}
	if pastKts > 0 {
		t.Errorf("%s rolled at %.0f kt past 27's landing distance (%.0f m), over %.0f kt (%s)", model, pastKts, ldaM, slowKts, pastAt)
	}
	for _, j := range h.Sim.Jumps()[jumps:] {
		if j.Tail == "XDA1" {
			t.Errorf("XDA1 jumped: %+v", j)
		}
	}
}
