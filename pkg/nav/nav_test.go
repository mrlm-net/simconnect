//go:build windows
// +build windows

package nav

import (
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// loadLKPR builds the LKPR layout from the facility data captured in MSFS
// 2024 (runways 06/24 and 12/30, plus the closed 04/22).
func loadLKPR(t testing.TB) *airport.Layout {
	t.Helper()
	b, err := os.ReadFile("../airport/testdata/LKPR.json")
	if err != nil {
		t.Fatal(err)
	}
	var raw airport.RawAirport
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	l, err := airport.BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

var lkprLimits = RunwayLimits{Preferred: []string{"24", "06"}}

func TestActiveRunwaysLKPR(t *testing.T) {
	l := loadLKPR(t)
	_, e24, _ := l.RunwayEnd("24")
	tests := []struct {
		name string
		w    Weather
		lim  RunwayLimits
		want string
	}{
		{"westerly 240/15", StaticWeather(240, 15, 10000, 15, 8, 1013), lkprLimits, "24"},
		{"easterly 060/10", StaticWeather(60, 10, 10000, 15, 8, 1013), lkprLimits, "06"},
		{"calm", StaticWeather(0, 0, 10000, 15, 8, 1013), lkprLimits, "24"},
		// 4 kt straight down 24: within the 5 kt tailwind limit, so the
		// preferred 24 stays although 06 has a headwind.
		{"tailwind 4 kt on 24", StaticWeather(e24.Heading+180, 4, 10000, 15, 8, 1013), lkprLimits, "24"},
		// 7 kt tailwind is over the limit: 06.
		{"tailwind 7 kt on 24", StaticWeather(e24.Heading+180, 7, 10000, 15, 8, 1013), lkprLimits, "06"},
		// 30 kt across 06/24 is over the 25 kt crosswind limit there; 12/30
		// takes it nearly head on.
		{"crosswind", StaticWeather(e24.Heading+90, 30, 10000, 15, 8, 1013), lkprLimits, "30"},
		// Without preference, the most headwind wins.
		{"no preference 300/20", StaticWeather(300, 20, 10000, 15, 8, 1013), RunwayLimits{}, "30"},
		{"preference 300/20", StaticWeather(300, 20, 10000, 15, 8, 1013), lkprLimits, "24"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u := ActiveRunways(l, tc.w, tc.lim)
			if u.Departure.Name != tc.want || u.Arrival.Name != tc.want {
				t.Errorf("got departure %s arrival %s, want %s (head %.1f cross %.1f)",
					u.Departure.Name, u.Arrival.Name, tc.want, u.HeadwindKts, u.CrosswindKts)
			}
			if !u.WithinLimits {
				t.Error("not within limits")
			}
		})
	}
}

func TestActiveRunwaysGustsAndFallback(t *testing.T) {
	l := loadLKPR(t)
	_, e24, _ := l.RunwayEnd("24")
	// Mean 3 kt tailwind is fine, gusts to 9 kt are not.
	w := StaticWeather(e24.Heading+180, 3, 10000, 15, 8, 1013)
	w.GustKts = 9
	if u := ActiveRunways(l, w, lkprLimits); u.Arrival.Name != "06" {
		t.Errorf("gusting tailwind: %s", u.Arrival.Name)
	}
	// 60 kt from 096, between 06/24 and 12/30: over the 20 kt crosswind
	// limit on every runway, so the most headwind is taken and flagged.
	w = StaticWeather(96, 60, 10000, 15, 8, 1013)
	u := ActiveRunways(l, w, RunwayLimits{MaxCrosswindKts: 20})
	if u.WithinLimits || u.HeadwindKts <= 0 {
		t.Errorf("fallback: %+v", u)
	}
	// Separate arrival preference.
	u = ActiveRunways(l, StaticWeather(0, 0, 10000, 15, 8, 1013), RunwayLimits{Preferred: []string{"24"}, PreferredArrival: []string{"6"}})
	if u.Departure.Name != "24" || u.Arrival.Name != "06" || u.Single() {
		t.Errorf("split: %s/%s", u.Departure.Name, u.Arrival.Name)
	}
	if u := ActiveRunways(nil, w, lkprLimits); u.Arrival.Name != "" || u.WithinLimits {
		t.Errorf("no layout: %+v", u)
	}
}

func TestApproachFor(t *testing.T) {
	w := StaticWeather(240, 8, 10000, 15, 8, 1013)
	if ApproachFor(w) != ApproachVisual {
		t.Error("CAVOK-ish weather should be visual")
	}
	w.VisibilityM = 3000
	if ApproachFor(w) != ApproachILS {
		t.Error("3000 m should be ILS")
	}
	w.VisibilityM, w.CeilingFt = 10000, 800
	if ApproachFor(w) != ApproachILS {
		t.Error("800 ft ceiling should be ILS")
	}
}

func TestTransitionLevel(t *testing.T) {
	for _, tc := range []struct {
		qnh  float64
		want int
	}{{1030, 60}, {1014, 60}, {1013, 70}, {990, 70}, {977, 70}, {976, 80}} {
		if got := TransitionLevel(5000, tc.qnh); got != tc.want {
			t.Errorf("QNH %v: FL%d, want FL%d", tc.qnh, got, tc.want)
		}
	}
}

func lkprATIS(t *testing.T, w Weather) ATIS {
	l := loadLKPR(t)
	now := time.Date(2026, 9, 28, 13, 20, 0, 0, time.UTC)
	return NewATIS("Ruzyne", 'A', now, w, ActiveRunways(l, w, lkprLimits), 5000, 0)
}

func TestATISText(t *testing.T) {
	a := lkprATIS(t, StaticWeather(240, 8, 12000, 15, 8, 1013.6))
	want := "Ruzyne information Alpha, time 1320, runway in use 24, wind 240 degrees 8 knots, " +
		"visibility 10 kilometers or more, temperature 15, dewpoint 8, QNH 1013, transition level 70, " +
		"advise on initial contact you have information Alpha."
	if got := a.Text(); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}

	w := StaticWeather(65, 12, 2400, -3, math.NaN(), 1021)
	w.GustKts, w.CeilingFt, w.Precip = 25, 700, PrecipSnow
	a = lkprATIS(t, w)
	a.MagVar = 356 // LKPR: magnetic = true - 4
	got := a.Text()
	for _, s := range []string{"runway in use 06", "expect ILS approach", "wind 060 degrees 12 knots gusting 25 knots",
		"visibility 2400 meters", "snow", "ceiling 700 feet", "temperature minus 3, QNH 1021", "transition level 60"} {
		if !strings.Contains(got, s) {
			t.Errorf("%q missing in\n%s", s, got)
		}
	}
	if strings.Contains(got, "dewpoint") {
		t.Errorf("unknown dewpoint reported: %s", got)
	}
	if got := lkprATIS(t, StaticWeather(0, 0.4, 7300, 15, 8, 1013)).Text(); !strings.Contains(got, "wind calm, visibility 7 kilometers") {
		t.Errorf("calm: %s", got)
	}
}

func TestATISSpoken(t *testing.T) {
	w := StaticWeather(240, 19, 1500, 9, -1, 1009)
	w.CeilingFt = 1200
	got := lkprATIS(t, w).Spoken()
	for _, s := range []string{"Ruzyne information Alpha", "time one three two zero", "runway in use two four",
		"expect ILS approach", "wind two four zero degrees one niner knots", "visibility one thousand five hundred meters",
		"ceiling one thousand two hundred feet", "temperature niner, dewpoint minus one", "QNH one zero zero niner",
		"transition level seven zero", "you have information Alpha."} {
		if !strings.Contains(got, s) {
			t.Errorf("%q missing in\n%s", s, got)
		}
	}
	n := numbers{spoken: true}
	if got := n.runway("27L"); got != "two seven left" {
		t.Errorf("runway: %s", got)
	}
	if got := n.hundreds(10000); got != "one zero thousand" {
		t.Errorf("hundreds: %s", got)
	}
}

func TestATISServiceLetters(t *testing.T) {
	l := loadLKPR(t)
	s := NewATISService("Ruzyne", l, lkprLimits, 5000)
	t0 := time.Date(2026, 9, 28, 13, 20, 0, 0, time.UTC)
	step := func(w Weather, at time.Duration, wantLetter byte, wantChanged bool) ATIS {
		t.Helper()
		a, changed := s.Update(w, t0.Add(at))
		if a.Letter != wantLetter || changed != wantChanged {
			t.Errorf("at %v: letter %c changed %v, want %c %v", at, a.Letter, changed, wantLetter, wantChanged)
		}
		return a
	}
	step(StaticWeather(240, 8, 10000, 15, 8, 1013), 0, 'A', true)
	// Small changes keep Alpha.
	step(StaticWeather(250, 10, 9000, 16, 8, 1013.8), 10*time.Minute, 'A', false)
	// QNH change: Bravo.
	step(StaticWeather(250, 10, 9000, 16, 8, 1014), 20*time.Minute, 'B', true)
	// Wind swings to the east: runway change, Charlie.
	a := step(StaticWeather(60, 12, 9000, 16, 8, 1014), 30*time.Minute, 'C', true)
	if a.Use.Arrival.Name != "06" {
		t.Errorf("runway %s", a.Use.Arrival.Name)
	}
	// An hour on without a change: Delta.
	step(StaticWeather(60, 12, 9000, 16, 8, 1014), 90*time.Minute, 'D', true)
	if cur, ok := s.Current(); !ok || cur.Letter != 'D' {
		t.Errorf("current %c", cur.Letter)
	}
	// Zulu wraps to Alpha.
	z := NewATISService("Ruzyne", l, lkprLimits, 5000, ATISWithLetter('z'))
	z.Update(StaticWeather(240, 8, 10000, 15, 8, 1013), t0)
	if a, _ := z.Update(StaticWeather(240, 8, 10000, 15, 8, 1020), t0); a.Letter != 'A' {
		t.Errorf("after Zulu: %c", a.Letter)
	}
}

type fakeDataClient struct {
	fields []string
	reqs   []types.SIMCONNECT_PERIOD
}

func (f *fakeDataClient) AddToDataDefinition(def uint32, name, unit string, typ types.SIMCONNECT_DATATYPE, eps float32, id uint32) error {
	f.fields = append(f.fields, name)
	return nil
}

func (f *fakeDataClient) RequestDataOnSimObject(req, def, obj uint32, period types.SIMCONNECT_PERIOD, flags types.SIMCONNECT_DATA_REQUEST_FLAG, origin, interval, limit uint32) error {
	f.reqs = append(f.reqs, period)
	return nil
}

func TestWeatherReader(t *testing.T) {
	c := &fakeDataClient{}
	r := NewWeatherReader(c, 10, 20)
	if err := r.Request(); err != nil {
		t.Fatal(err)
	}
	if err := r.Subscribe(); err != nil {
		t.Fatal(err)
	}
	if len(c.fields) != len(weatherVars) || len(c.reqs) != 2 {
		t.Fatalf("fields %v requests %v", c.fields, c.reqs)
	}
	var hdr types.SIMCONNECT_RECV_SIMOBJECT_DATA
	off := unsafe.Offsetof(hdr.DwData)
	buf := make([]byte, off+unsafe.Sizeof(weatherWire{}))
	h := (*types.SIMCONNECT_RECV_SIMOBJECT_DATA)(unsafe.Pointer(&buf[0]))
	h.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA)
	h.DwRequestID = 20
	*(*weatherWire)(unsafe.Pointer(&buf[off])) = weatherWire{-120, 14, 8000, 11.5, 1008.9, 4, 1}
	msg := engine.Message{SIMCONNECT_RECV: (*types.SIMCONNECT_RECV)(unsafe.Pointer(&buf[0]))}
	w, ok := r.Handle(msg)
	if !ok || w.WindDirTrue != 240 || w.WindKts != 14 || w.VisibilityM != 8000 || w.QNHhPa != 1008.9 ||
		w.Precip != PrecipRain || !w.InCloud || !math.IsNaN(w.DewpointC) || w.Time.IsZero() {
		t.Errorf("decoded %+v", w)
	}
	if last, ok := r.Last(); !ok || last.WindKts != 14 {
		t.Error("Last")
	}
	h.DwRequestID = 21
	if _, ok := r.Handle(msg); ok {
		t.Error("foreign request handled")
	}
}
