package nav

import (
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// DefaultATISMaxAge is how long an ATIS stays current without a significant
// change before ATISService issues the next letter, as with hourly METARs.
const DefaultATISMaxAge = time.Hour

// ftPerHPa is the altitude change per hPa of QNH near the surface.
const ftPerHPa = 27

// ATIS is one automatic terminal information broadcast.
type ATIS struct {
	Airport string // the name the broadcast opens with, e.g. "Ruzyne"
	Letter  byte   // 'A'–'Z'
	Time    time.Time
	Weather Weather
	Use     RunwayUse
	// TransitionAltitudeFt is the airport's transition altitude, e.g. 5000
	// at LKPR; TransitionLevel the flight level (e.g. 70) derived from it.
	TransitionAltitudeFt int
	TransitionLevel      int
	QNH                  int // hPa, rounded down
	// MagVar turns the true wind into the magnetic wind the broadcast
	// reports, in the facility data convention of airport.Procedures.MagVar:
	// magnetic = true + MagVar (LKPR: 356). 0 reports the wind true.
	MagVar float64
}

// NewATIS assembles a broadcast: QNH is Weather.QNHhPa rounded down, the
// transition level follows from it and transitionAltitudeFt (0: none).
func NewATIS(airportName string, letter byte, now time.Time, w Weather, use RunwayUse, transitionAltitudeFt int, magVar float64) ATIS {
	a := ATIS{
		Airport:              airportName,
		Letter:               letter,
		Time:                 now,
		Weather:              w,
		Use:                  use,
		TransitionAltitudeFt: transitionAltitudeFt,
		QNH:                  int(math.Floor(w.QNHhPa)),
		MagVar:               magVar,
	}
	if transitionAltitudeFt > 0 {
		a.TransitionLevel = TransitionLevel(transitionAltitudeFt, float64(a.QNH))
	}
	return a
}

// TransitionLevel returns the lowest flight level, in tens (FL 60, 70, ...),
// at least 1000 ft above the transition altitude at the given QNH. At LKPR
// (TA 5000 ft) that is FL 60 from QNH 1014 up and FL 70 from 1013 down to
// 977.
func TransitionLevel(transitionAltitudeFt int, qnhHPa float64) int {
	need := float64(transitionAltitudeFt + 1000)
	for fl := 10; ; fl += 10 {
		if float64(fl*100)+(qnhHPa-1013.25)*ftPerHPa >= need-1e-9 {
			return fl
		}
	}
}

// Phonetic returns the ICAO spelling of a letter, e.g. "Alpha" for 'A'.
func Phonetic(letter byte) string {
	if letter >= 'a' && letter <= 'z' {
		letter -= 'a' - 'A'
	}
	if letter < 'A' || letter > 'Z' {
		return string(letter)
	}
	return phonetic[letter-'A']
}

var phonetic = [...]string{"Alpha", "Bravo", "Charlie", "Delta", "Echo", "Foxtrot", "Golf", "Hotel", "India",
	"Juliett", "Kilo", "Lima", "Mike", "November", "Oscar", "Papa", "Quebec", "Romeo", "Sierra", "Tango",
	"Uniform", "Victor", "Whiskey", "X-ray", "Yankee", "Zulu"}

// Text returns the broadcast in ICAO phraseology with numbers as digits:
//
//	Ruzyne information Alpha, time 1320, runway in use 24, wind 240 degrees
//	8 knots, visibility 10 kilometers or more, temperature 15, dewpoint 8,
//	QNH 1013, transition level 70, advise on initial contact you have
//	information Alpha.
func (a ATIS) Text() string { return a.render(false) }

// Spoken returns the broadcast for a voice: numbers spelled digit by digit
// ("two four", "one zero one three", "niner"), whole hundreds and thousands
// as words ("one thousand two hundred feet") and runway suffixes as
// "left", "right" and "center".
func (a ATIS) Spoken() string { return a.render(true) }

func (a ATIS) render(spoken bool) string {
	n := numbers{spoken}
	w := a.Weather
	letter := Phonetic(a.Letter)
	var parts []string
	add := func(s ...string) { parts = append(parts, strings.Join(s, " ")) }

	add(a.Airport, "information", letter)
	if !a.Time.IsZero() {
		add("time", n.digits(a.Time.UTC().Format("1504")))
	}
	// The runways in use: "runways in use 26L and 26R" for parallels used
	// together, landing and departure runways when they differ.
	runways := func(ends []airport.RunwayEnd, one string) []string {
		if len(ends) == 0 {
			return []string{one}
		}
		var out []string
		for _, e := range ends {
			out = append(out, n.runway(e.Name))
		}
		return out
	}
	list := func(s []string) string {
		if len(s) < 2 {
			return strings.Join(s, "")
		}
		return strings.Join(s[:len(s)-1], ", ") + " and " + s[len(s)-1]
	}
	plural := func(s []string, one string) string {
		if len(s) > 1 {
			return one + "s"
		}
		return one
	}
	switch u := a.Use; {
	case u.Arrival.Name == "" && u.Departure.Name == "":
	case u.Single() && len(u.Departures) == len(u.Arrivals):
		r := runways(u.Arrivals, n.runway(u.Arrival.Name))
		add(plural(r, "runway"), "in use", list(r))
	default:
		ar, dr := runways(u.Arrivals, n.runway(u.Arrival.Name)), runways(u.Departures, n.runway(u.Departure.Name))
		add("landing", plural(ar, "runway"), list(ar))
		add("departure", plural(dr, "runway"), list(dr))
	}
	if a.Use.Approach == ApproachILS {
		add("expect ILS approach")
	}
	if w.IsCalm() {
		add("wind calm")
	} else {
		dir := int(math.Round(math.Mod(w.WindDirTrue+a.MagVar+720, 360)/10)) * 10
		if dir == 0 {
			dir = 360
		}
		s := []string{"wind", n.digits(pad3(dir)), "degrees", n.digits(itoa(math.Round(w.WindKts))), "knots"}
		if w.GustKts >= w.WindKts+10 {
			s = append(s, "gusting", n.digits(itoa(math.Round(w.GustKts))), "knots")
		}
		add(s...)
	}
	switch v := w.VisibilityM; {
	case v <= 0:
	case v >= 10000:
		add("visibility", n.digits("10"), "kilometers or more")
	case v >= 5000:
		add("visibility", n.digits(itoa(math.Floor(v/1000))), "kilometers")
	case v >= 800:
		add("visibility", n.hundreds(int(v)/100*100), "meters")
	default:
		add("visibility", n.hundreds(int(v)/50*50), "meters")
	}
	if w.Precip == PrecipRain || w.Precip == PrecipSnow {
		add(w.Precip)
	}
	if w.CeilingFt > 0 {
		add("ceiling", n.hundreds(int(w.CeilingFt)/100*100), "feet")
	}
	add("temperature", n.signed(w.TempC))
	if !math.IsNaN(w.DewpointC) {
		add("dewpoint", n.signed(w.DewpointC))
	}
	if a.QNH > 0 {
		add("QNH", n.digits(strconv.Itoa(a.QNH)))
	}
	if a.TransitionLevel > 0 {
		add("transition level", n.digits(strconv.Itoa(a.TransitionLevel)))
	}
	add("advise on initial contact you have information", letter)
	return strings.Join(parts, ", ") + "."
}

// numbers formats numbers as digits, or spelled for a voice.
type numbers struct{ spoken bool }

var digitWords = [...]string{"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "niner"}

// digits spells every digit of s; other characters are kept.
func (n numbers) digits(s string) string {
	if !n.spoken {
		return s
	}
	var out []string
	for _, c := range s {
		if c >= '0' && c <= '9' {
			out = append(out, digitWords[c-'0'])
		} else {
			out = append(out, string(c))
		}
	}
	return strings.Join(out, " ")
}

// hundreds spells a whole number of hundreds as thousands and hundreds, e.g.
// "one thousand five hundred"; values that are not whole hundreds are spelled
// digit by digit.
func (n numbers) hundreds(v int) string {
	if !n.spoken {
		return strconv.Itoa(v)
	}
	if v <= 0 || v%100 != 0 {
		return n.digits(strconv.Itoa(v))
	}
	var out []string
	if th := v / 1000; th > 0 {
		out = append(out, n.digits(strconv.Itoa(th)), "thousand")
	}
	if h := v % 1000 / 100; h > 0 {
		out = append(out, digitWords[h], "hundred")
	}
	return strings.Join(out, " ")
}

// signed formats a rounded temperature, "minus 3" below zero.
func (n numbers) signed(v float64) string {
	r := math.Round(v)
	if r < 0 {
		return "minus " + n.digits(itoa(-r))
	}
	return n.digits(itoa(r))
}

// runway formats a runway end name, spelling "24L" as "two four left".
func (n numbers) runway(name string) string {
	if !n.spoken {
		return name
	}
	i := 0
	for i < len(name) && isDigit(name[i]) {
		i++
	}
	s := n.digits(name[:i])
	switch name[i:] {
	case "":
	case "L":
		s += " left"
	case "R":
		s += " right"
	case "C":
		s += " center"
	default:
		s = strings.TrimSpace(s + " " + name[i:])
	}
	return s
}

func itoa(v float64) string { return strconv.Itoa(int(v)) }

func pad3(v int) string {
	s := strconv.Itoa(v)
	for len(s) < 3 {
		s = "0" + s
	}
	return s
}

// ATISOption configures an ATISService.
type ATISOption func(*ATISService)

// ATISWithMagVar sets the magnetic variation the wind is reported with, in
// the convention of ATIS.MagVar (airport.Procedures.MagVar).
func ATISWithMagVar(magVar float64) ATISOption {
	return func(s *ATISService) { s.magVar = magVar }
}

// ATISWithMaxAge sets how long an ATIS stays current without a significant
// change; 0 or less never expires it.
func ATISWithMaxAge(d time.Duration) ATISOption {
	return func(s *ATISService) { s.maxAge = d }
}

// ATISWithLetter sets the letter of the first broadcast (default 'A').
func ATISWithLetter(letter byte) ATISOption {
	return func(s *ATISService) {
		if l := Phonetic(letter); len(l) > 1 {
			s.next = l[0]
		}
	}
}

// ATISService keeps an airport's current ATIS. Update it with every new
// weather; it issues a broadcast with the next letter when the change is
// significant:
//
//   - the departure or arrival runway, the QNH or the transition level
//     changes;
//   - the wind direction turns 60° or more with a wind of 10 kt or more, or
//     the wind or gust speed changes by 10 kt or more;
//   - visibility crosses 800, 1500, 3000 or 5000 m, or the approach hint or
//     precipitation changes;
//   - the broadcast is older than the maximum age (DefaultATISMaxAge).
//
// Smaller changes keep the current broadcast, as a real ATIS does between
// reports. An ATISService is safe for concurrent use.
type ATISService struct {
	mu     sync.Mutex
	name   string
	layout *airport.Layout
	lim    RunwayLimits
	taFt   int
	magVar float64
	maxAge time.Duration
	next   byte
	cur    ATIS
	have   bool
	// sel keeps the runway in use through wind shifts near a limit: the
	// ATIS says what the traffic uses (#454).
	sel *RunwaySelector
}

// ATISWithSelector has the ATIS take the runway in use from sel, the one
// the traffic uses (#454). Without it the ATIS keeps a selector of its own:
// either way it does not change runway with every wind shift near a limit.
func ATISWithSelector(sel *RunwaySelector) ATISOption {
	return func(s *ATISService) {
		if sel != nil {
			s.sel = sel
		}
	}
}

// NewATISService creates the ATIS of the airport l under the broadcast name
// name (e.g. "Ruzyne"), choosing runways with lim.
func NewATISService(name string, l *airport.Layout, lim RunwayLimits, transitionAltitudeFt int, opts ...ATISOption) *ATISService {
	s := &ATISService{name: name, layout: l, lim: lim, taFt: transitionAltitudeFt, maxAge: DefaultATISMaxAge, next: 'A', sel: &RunwaySelector{}}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Update feeds the weather at now. It returns the current ATIS and whether it
// is a new broadcast (always true the first time).
func (s *ATISService) Update(w Weather, now time.Time) (ATIS, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := NewATIS(s.name, s.next, now, w, s.sel.Choose(now, s.layout, w, s.lim), s.taFt, s.magVar)
	if s.have && !s.significant(s.cur, a) {
		return s.cur, false
	}
	s.cur, s.have = a, true
	s.next = 'A' + (a.Letter-'A'+1)%26
	return a, true
}

// Current returns the current ATIS; ok is false before the first Update.
func (s *ATISService) Current() (ATIS, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cur, s.have
}

func (s *ATISService) significant(old, new ATIS) bool {
	if s.maxAge > 0 && new.Time.Sub(old.Time) >= s.maxAge {
		return true
	}
	if old.Use.key() != new.Use.key() ||
		old.Use.Approach != new.Use.Approach || old.QNH != new.QNH || old.TransitionLevel != new.TransitionLevel {
		return true
	}
	ow, nw := old.Weather, new.Weather
	if math.Abs(ow.WindKts-nw.WindKts) >= 10 || math.Abs(ow.GustKts-nw.GustKts) >= 10 {
		return true
	}
	if max(ow.WindKts, nw.WindKts) >= 10 {
		d := math.Abs(math.Mod(ow.WindDirTrue-nw.WindDirTrue+540, 360) - 180)
		if d >= 60 {
			return true
		}
	}
	if visibilityBand(ow.VisibilityM) != visibilityBand(nw.VisibilityM) || ow.Precip != nw.Precip {
		return true
	}
	return false
}

func visibilityBand(v float64) int {
	b := 0
	for _, t := range []float64{800, 1500, 3000, 5000} {
		if v >= t {
			b++
		}
	}
	return b
}
