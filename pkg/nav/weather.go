package nav

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Precipitation as Weather.Precip reports it.
const (
	PrecipNone = "none"
	PrecipRain = "rain"
	PrecipSnow = "snow"
)

// Weather is the surface weather at an airport.
type Weather struct {
	WindDirTrue float64 // degrees true the wind blows from
	WindKts     float64
	GustKts     float64 // 0 when there are no gusts
	VisibilityM float64
	CeilingFt   float64 // 0 when there is no ceiling or it is unknown
	TempC       float64
	// DewpointC is NaN when unknown; SimConnect has no dewpoint variable,
	// so WeatherReader always reports NaN.
	DewpointC float64
	QNHhPa    float64
	Precip    string // PrecipNone, PrecipRain, PrecipSnow or "" when unknown
	InCloud   bool   // the user aircraft is in a cloud (WeatherReader only)
	Time      time.Time
}

// StaticWeather builds Weather set by the application rather than read from
// the simulator, e.g. for tests or a fixed scenario. Gusts, ceiling and
// precipitation are left empty; Time is zero.
func StaticWeather(windDirTrue, windKts, visibilityM, tempC, dewpointC, qnhHPa float64) Weather {
	return Weather{
		WindDirTrue: math.Mod(windDirTrue+360, 360),
		WindKts:     windKts,
		VisibilityM: visibilityM,
		TempC:       tempC,
		DewpointC:   dewpointC,
		QNHhPa:      qnhHPa,
		Precip:      PrecipNone,
	}
}

// IsCalm reports whether the wind rounds to 0 knots.
func (w Weather) IsCalm() bool { return math.Round(w.WindKts) < 1 }

// Components returns the headwind (negative for a tailwind) and crosswind
// (always ≥ 0) components of the mean wind on a runway heading in degrees
// true.
func (w Weather) Components(headingTrue float64) (headwind, crosswind float64) {
	d := (w.WindDirTrue - headingTrue) * math.Pi / 180
	return w.WindKts * math.Cos(d), math.Abs(w.WindKts * math.Sin(d))
}

// DataClient is the part of engine.Client (and manager.Manager) the
// WeatherReader uses.
type DataClient interface {
	AddToDataDefinition(definitionID uint32, datumName string, unitsName string, datumType types.SIMCONNECT_DATATYPE, epsilon float32, datumID uint32) error
	RequestDataOnSimObject(requestID uint32, definitionID uint32, objectID uint32, period types.SIMCONNECT_PERIOD, flags types.SIMCONNECT_DATA_REQUEST_FLAG, origin uint32, interval uint32, limit uint32) error
}

// weatherVars are the SimVars of the reader's data definition; the order and
// units must match weatherWire.
var weatherVars = [...]struct{ name, unit string }{
	{"AMBIENT WIND DIRECTION", "degrees"},
	{"AMBIENT WIND VELOCITY", "knots"},
	{"AMBIENT VISIBILITY", "meters"},
	{"AMBIENT TEMPERATURE", "celsius"},
	{"SEA LEVEL PRESSURE", "millibars"},
	{"AMBIENT PRECIP STATE", "mask"},
	{"AMBIENT IN CLOUD", "bool"},
}

type weatherWire struct {
	WindDir, WindKts, VisibilityM, TempC, SeaLevelMb, PrecipState, InCloud float64
}

// AMBIENT PRECIP STATE bits.
const (
	precipStateRain = 4
	precipStateSnow = 8
)

// WeatherReader reads the ambient weather at the user aircraft through an
// application's own message loop, like airport.Loader: call Request (once)
// or Subscribe (every second while it changes), then pass every received
// message to Handle.
//
//	wx := nav.NewWeatherReader(client, 10000, 10001)
//	wx.Request()
//	for msg := range client.Stream() {
//	    if w, ok := wx.Handle(msg); ok {
//	        // w is the weather now
//	    }
//	}
//
// SimConnect has no per-airport weather: the values are the weather where
// the user aircraft is. That is the airport's weather while the user is on
// the ground there or nearby, which is the case when the airport is the
// world centre around the user. Gusts, ceiling and dewpoint have no SimVar
// and are left 0, 0 and NaN.
//
// A WeatherReader is safe for concurrent use. After a reconnect, call Reset
// so the data definition is registered again on the new connection.
type WeatherReader struct {
	mu         sync.Mutex
	client     DataClient
	defID      uint32
	reqID      uint32
	registered bool
	last       Weather
	have       bool
}

// NewWeatherReader creates a reader that uses one data definition ID and one
// request ID; keep both clear of the application's own IDs.
func NewWeatherReader(client DataClient, defID, reqID uint32) *WeatherReader {
	return &WeatherReader{client: client, defID: defID, reqID: reqID}
}

// Reset forgets the registered definition, e.g. after the simulator
// reconnects. A non-nil client replaces the current one.
func (r *WeatherReader) Reset(client DataClient) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if client != nil {
		r.client = client
	}
	r.registered = false
}

// Request asks for the weather once.
func (r *WeatherReader) Request() error {
	return r.request(types.SIMCONNECT_PERIOD_ONCE, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT)
}

// Subscribe asks for the weather every second, sent only when it changed.
func (r *WeatherReader) Subscribe() error {
	return r.request(types.SIMCONNECT_PERIOD_SECOND, types.SIMCONNECT_DATA_REQUEST_FLAG_CHANGED)
}

// Stop ends a subscription.
func (r *WeatherReader) Stop() error {
	return r.request(types.SIMCONNECT_PERIOD_NEVER, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT)
}

func (r *WeatherReader) request(period types.SIMCONNECT_PERIOD, flags types.SIMCONNECT_DATA_REQUEST_FLAG) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.registered {
		for i, v := range weatherVars {
			if err := r.client.AddToDataDefinition(r.defID, v.name, v.unit, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i)); err != nil {
				return fmt.Errorf("nav: define %s: %w", v.name, err)
			}
		}
		r.registered = true
	}
	if err := r.client.RequestDataOnSimObject(r.reqID, r.defID, types.SIMCONNECT_OBJECT_ID_USER, period, flags, 0, 0, 0); err != nil {
		return fmt.Errorf("nav: request weather: %w", err)
	}
	return nil
}

// Handle processes one message. It returns ok=true with the weather when msg
// is the reader's data; other messages are ignored.
func (r *WeatherReader) Handle(msg engine.Message) (Weather, bool) {
	if msg.SIMCONNECT_RECV == nil || types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA {
		return Weather{}, false
	}
	d := msg.AsSimObjectData()
	if uint32(d.DwRequestID) != r.reqID {
		return Weather{}, false
	}
	w := decodeWeather(engine.CastDataAs[weatherWire](&d.DwData), time.Now().UTC())
	r.mu.Lock()
	r.last, r.have = w, true
	r.mu.Unlock()
	return w, true
}

// Last returns the most recent weather Handle decoded.
func (r *WeatherReader) Last() (Weather, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.last, r.have
}

func decodeWeather(x *weatherWire, now time.Time) Weather {
	w := Weather{
		WindDirTrue: math.Mod(x.WindDir+360, 360),
		WindKts:     x.WindKts,
		VisibilityM: x.VisibilityM,
		TempC:       x.TempC,
		DewpointC:   math.NaN(),
		QNHhPa:      x.SeaLevelMb,
		Precip:      PrecipNone,
		InCloud:     x.InCloud != 0,
		Time:        now,
	}
	switch s := int(x.PrecipState); {
	case s&precipStateSnow != 0:
		w.Precip = PrecipSnow
	case s&precipStateRain != 0:
		w.Precip = PrecipRain
	}
	return w
}

// Icing conditions (#323): de-icing before departure is due at or below
// IcingMaxTempC with visible moisture — precipitation, visibility below
// IcingVisibilityM (fog, mist) or cloud at the aircraft.
var (
	IcingMaxTempC    = 3.0
	IcingVisibilityM = 1500.0
)

// IcingConditions reports weather in which departures need de-icing.
func IcingConditions(w Weather) bool {
	if w.TempC > IcingMaxTempC {
		return false
	}
	moisture := (w.Precip != "" && w.Precip != PrecipNone) || (w.VisibilityM > 0 && w.VisibilityM < IcingVisibilityM) || w.InCloud
	return moisture
}
