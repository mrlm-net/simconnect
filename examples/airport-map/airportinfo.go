//go:build windows
// +build windows

package main

import (
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/convert"
	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// The loaded airport at a glance (#357): its runways and limits, the
// weather (at the user aircraft, as SimConnect reports it), the runway in
// use and the ATIS.

// SimConnect IDs of the weather reader.
const (
	weatherDefID = 7500
	weatherReqID = 7501
)

type runwayInfo struct {
	Name     string  `json:"name"`    // "06/24"
	Heading  float64 `json:"heading"` // magnetic, of the primary end
	LengthM  float64 `json:"lengthM"`
	WidthM   float64 `json:"widthM"`
	Approach string  `json:"approach,omitempty"` // best approach per end: "ILS 06 · RNAV 24"
}

type airportInfo struct {
	ICAO        string         `json:"icao"`
	Name        string         `json:"name"`
	ElevationFt float64        `json:"elevationFt"`
	MagVar      float64        `json:"magVar"` // east positive
	Runways     []runwayInfo   `json:"runways"`
	Limits      airport.Limits `json:"limits"`
	Weather     *weatherInfo   `json:"weather,omitempty"`
	Use         *useInfo       `json:"use,omitempty"`
	ATIS        *atisInfo      `json:"atis,omitempty"`
}

type weatherInfo struct {
	WindDirTrue, WindKts, GustKts float64
	VisibilityM, CeilingFt, TempC float64
	DewpointC                     *float64 `json:",omitempty"` // nil when unknown (JSON has no NaN)
	QNHhPa                        float64
	Precip                        string
	InCloud                       bool
	Icing                         bool // de-icing required (nav.IcingConditions)
	// DistanceNM is how far from the airport the user aircraft is, where
	// SimConnect measures the weather.
	DistanceNM float64 `json:"distanceNM"`
}

type useInfo struct {
	Departure    string  `json:"departure"`
	Arrival      string  `json:"arrival"`
	HeadwindKts  float64 `json:"headwindKts"`
	CrosswindKts float64 `json:"crosswindKts"`
	WithinLimits bool    `json:"withinLimits"`
	Approach     string  `json:"approach"`
}

type atisInfo struct {
	Letter string `json:"letter"`
	Text   string `json:"text"`
	Spoken string `json:"spoken"`
}

// magVarEast is the facility MAGVAR (356 = 4° east) as east-positive
// degrees.
func magVarEast(v float64) float64 {
	if v > 180 {
		v -= 360
	}
	return -v
}

// registerAirportInfo serves GET /api/airportinfo?icao=X.
// atisService is the ATIS of icao, made on first use; ok is false while
// the airport is not loaded.
func (st *state) atisService(icao string) (*nav.ATISService, bool) {
	l, ok := st.cache.Layout(icao)
	if !ok {
		return nil, false
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.atis == nil {
		st.atis = map[string]*nav.ATISService{}
	}
	if svc := st.atis[icao]; svc != nil {
		return svc, true
	}
	p, hasProcs := st.procedures[icao]
	var procs *airport.Procedures
	opts := []nav.ATISOption{}
	if hasProcs {
		procs = &p
		opts = append(opts, nav.ATISWithMagVar(p.MagVar))
	}
	lim := airport.LimitsFor(l, procs)
	svc := nav.NewATISService(l.Name, l, nav.RunwayLimitsFrom(lim), int(lim.TransitionAltitudeFt), opts...)
	st.atis[icao] = svc
	return svc, true
}

// atisTick keeps the ATIS of every airport with traffic of ours current
// (#418): a new information is broadcast on the radio, on the ATIS
// frequency, and our pilots give its letter on their first call.
func (st *state) atisTick(now time.Time, cc *controlCenter, airports []string) {
	st.mu.Lock()
	wx := st.weather
	st.mu.Unlock()
	if wx == nil {
		return
	}
	for _, icao := range airports {
		svc, ok := st.atisService(icao)
		if !ok {
			continue
		}
		if a, isNew := svc.Update(*wx, now); isNew {
			cc.radio.Transmit(icao, traffic.ATISInformation(nav.Phonetic(a.Letter), a.Text()))
		}
	}
}

// atisLetter is icao's current information letter, phonetic ("" none yet).
func (st *state) atisLetter(icao string) string {
	st.mu.Lock()
	svc := st.atis[icao]
	st.mu.Unlock()
	if svc == nil {
		return ""
	}
	if a, ok := svc.Current(); ok {
		return nav.Phonetic(a.Letter)
	}
	return ""
}

func registerAirportInfo(mux *http.ServeMux, st *state) {
	// GET /api/radio/atis?icao=LKPR — the current ATIS: letter, text, spoken
	// form and frequency, for a voice to loop (#418).
	mux.HandleFunc("GET /api/radio/atis", func(w http.ResponseWriter, r *http.Request) {
		icao := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("icao")))
		svc, ok := st.atisService(icao)
		if !ok {
			http.Error(w, "airport not loaded", http.StatusNotFound)
			return
		}
		a, have := svc.Current()
		if !have {
			http.Error(w, "no ATIS yet", http.StatusNotFound)
			return
		}
		freq := ""
		if l, ok := st.cache.Layout(icao); ok {
			if f, ok := l.FrequencyFor(airport.FreqATIS); ok {
				freq = f.String()
			}
		}
		writeJSON(w, map[string]string{"icao": icao, "letter": nav.Phonetic(a.Letter), "text": a.Text(), "spoken": a.Spoken(), "frequency": freq})
	})

	mux.HandleFunc("GET /api/airportinfo", func(w http.ResponseWriter, r *http.Request) {
		icao := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("icao")))
		l, ok := st.cache.Layout(icao)
		if !ok {
			http.Error(w, "airport not loaded", http.StatusNotFound)
			return
		}
		st.mu.Lock()
		p, hasProcs := st.procedures[icao]
		wx, ac := st.weather, st.aircraft
		st.mu.Unlock()
		var procs *airport.Procedures
		mv := 0.0
		if hasProcs {
			procs, mv = &p, magVarEast(p.MagVar)
		}
		lim := airport.LimitsFor(l, procs)
		lim.DeicingPads = st.pads.forAirport(l)
		out := airportInfo{ICAO: l.ICAO, Name: l.Name, ElevationFt: convert.MetersToFeet(l.Altitude), MagVar: mv, Limits: lim}
		for _, rw := range l.Runways {
			ri := runwayInfo{Name: rw.Name(), Heading: math.Mod(rw.Heading-mv+360, 360), LengthM: rw.Length, WidthM: rw.Width}
			if procs != nil {
				var apps []string
				for _, e := range []string{rw.Primary.Name, rw.Secondary.Name} {
					if a, ok := procs.BestApproach(e); ok {
						apps = append(apps, a.Name)
					}
				}
				ri.Approach = strings.Join(apps, " · ")
			}
			out.Runways = append(out.Runways, ri)
		}
		if wx != nil {
			wi := &weatherInfo{WindDirTrue: wx.WindDirTrue, WindKts: wx.WindKts, GustKts: wx.GustKts, VisibilityM: wx.VisibilityM,
				CeilingFt: wx.CeilingFt, TempC: wx.TempC, QNHhPa: wx.QNHhPa, Precip: wx.Precip, InCloud: wx.InCloud, Icing: nav.IcingConditions(*wx)}
			if !math.IsNaN(wx.DewpointC) {
				d := wx.DewpointC
				wi.DewpointC = &d
			}
			if ac != nil {
				wi.DistanceNM = calc.HaversineMeters(ac.Latitude, ac.Longitude, l.Latitude, l.Longitude) / 1852
			}
			out.Weather = wi
			use := nav.ActiveRunways(l, *wx, nav.RunwayLimitsFrom(lim))
			out.Use = &useInfo{Departure: use.Departure.Name, Arrival: use.Arrival.Name, HeadwindKts: use.HeadwindKts,
				CrosswindKts: use.CrosswindKts, WithinLimits: use.WithinLimits, Approach: use.Approach}
			// The runway in use as traffic uses it: held through wind shifts.
			st.mu.Lock()
			cc := st.control
			st.mu.Unlock()
			if cc != nil {
				if g, err := cc.graph(icao); err == nil {
					out.Use.Departure, out.Use.Arrival = cc.activeRunway(g, false), cc.activeRunway(g, true)
					if _, end, ok := l.RunwayEnd(out.Use.Arrival); ok {
						out.Use.HeadwindKts, out.Use.CrosswindKts = wx.Components(end.Heading)
					}
				}
			}
			svc, _ := st.atisService(icao)
			// The broadcast as the traffic tick keeps it: made here only
			// before the first tick, on traffic time (wall time here and
			// traffic time there looked like an ATIS past its age: a new
			// letter at every look).
			a, ok := svc.Current()
			if !ok {
				now := time.Now()
				st.mu.Lock()
				if st.control != nil {
					now = st.control.clock.Now()
				}
				st.mu.Unlock()
				a, _ = svc.Update(*wx, now)
			}
			out.ATIS = &atisInfo{Letter: nav.Phonetic(a.Letter), Text: a.Text(), Spoken: a.Spoken()}
		}
		writeJSON(w, out)
	})
}
