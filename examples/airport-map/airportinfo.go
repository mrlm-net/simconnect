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
func registerAirportInfo(mux *http.ServeMux, st *state) {
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
			st.mu.Lock()
			if st.atis == nil {
				st.atis = map[string]*nav.ATISService{}
			}
			svc := st.atis[icao]
			if svc == nil {
				opts := []nav.ATISOption{}
				if hasProcs {
					opts = append(opts, nav.ATISWithMagVar(p.MagVar))
				}
				svc = nav.NewATISService(l.Name, l, nav.RunwayLimitsFrom(lim), int(lim.TransitionAltitudeFt), opts...)
				st.atis[icao] = svc
			}
			st.mu.Unlock()
			a, _ := svc.Update(*wx, time.Now())
			out.ATIS = &atisInfo{Letter: nav.Phonetic(a.Letter), Text: a.Text(), Spoken: a.Spoken()}
		}
		writeJSON(w, out)
	})
}
