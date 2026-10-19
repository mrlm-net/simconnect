package world

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/nav"
)

// Weather without a simulator (Vector): the World reads the weather from
// the simulator at the user aircraft; with none it has no wind, QNH or
// ATIS, and chooses runways without wind. A host sets it (SetWeather) or
// ships it with an airport's offline data (weather.json, LoadAirportDir).

// dumpWeather is an airport's offline weather file (LoadAirportDir).
const dumpWeather = "weather.json"

// WeatherFile is weather.json: a representative day at the airport.
type WeatherFile struct {
	WindDirTrue float64 `json:"windDirTrue"`
	WindKts     float64 `json:"windKts"`
	Variable    bool    `json:"variable,omitempty"`
	GustKts     float64 `json:"gustKts,omitempty"`
	VisibilityM float64 `json:"visibilityM"`
	CeilingFt   float64 `json:"ceilingFt,omitempty"`
	TempC       float64 `json:"tempC"`
	// DewpointC: omitted unknown.
	DewpointC *float64 `json:"dewpointC,omitempty"`
	QNHhPa    float64  `json:"qnhHPa"`
	Precip    string   `json:"precip,omitempty"` // none, rain, snow
}

// Weather is the file as nav.Weather.
func (f WeatherFile) Weather() nav.Weather {
	w := nav.StaticWeather(f.WindDirTrue, f.WindKts, f.VisibilityM, f.TempC, math.NaN(), f.QNHhPa)
	w.Variable, w.GustKts, w.CeilingFt = f.Variable, f.GustKts, f.CeilingFt
	if f.DewpointC != nil {
		w.DewpointC = *f.DewpointC
	}
	if f.Precip != "" {
		w.Precip = f.Precip
	}
	if w.VisibilityM <= 0 {
		w.VisibilityM = 9999
	}
	if w.QNHhPa <= 0 {
		w.QNHhPa = 1013
	}
	return w
}

// SetWeather sets the weather the World uses — wind, QNH, the ATIS, the
// runway in use — and keeps it over the simulator's; nil gives it back to
// the simulator. A game changes it as it likes.
func (w *World) SetWeather(wx *nav.Weather) {
	w.st.mu.Lock()
	if wx == nil {
		w.st.weatherSet = false
	} else {
		c := *wx
		w.st.weather, w.st.weatherSet = &c, true
	}
	w.st.mu.Unlock()
	if wx == nil {
		w.st.core.log.printf("weather: the simulator's again")
		return
	}
	w.st.core.log.printf("weather: set, wind %03.0f°/%.0f kt, QNH %.0f, visibility %.0f m", wx.WindDirTrue, wx.WindKts, wx.QNHhPa, wx.VisibilityM)
}

// loadWeatherFile reads path (weather.json) as the World's weather.
func (w *World) loadWeatherFile(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var f WeatherFile
	if err := json.NewDecoder(strings.NewReader(string(b))).Decode(&f); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	wx := f.Weather()
	w.SetWeather(&wx)
	return nil
}
