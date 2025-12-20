//go:build windows
// +build windows

package traffic

import (
	"github.com/mrlm-net/simconnect/pkg/datasets"
	"github.com/mrlm-net/simconnect/pkg/types"
)

func NewAircraftDataset() *datasets.DataSet {
	return &datasets.DataSet{
		Definitions: []datasets.DataDefinition{
			{Name: "TITLE", Unit: "", Type: types.SIMCONNECT_DATATYPE_STRING128, Epsilon: 0},
			{Name: "CATEGORY", Unit: "", Type: types.SIMCONNECT_DATATYPE_STRING128, Epsilon: 0},
			{Name: "LIVERY NAME", Unit: "", Type: types.SIMCONNECT_DATATYPE_STRING128, Epsilon: 0},
			{Name: "LIVERY FOLDER", Unit: "", Type: types.SIMCONNECT_DATATYPE_STRING128, Epsilon: 0},
			{Name: "PLANE LATITUDE", Unit: "degrees", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "PLANE LONGITUDE", Unit: "degrees", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "PLANE ALTITUDE", Unit: "feet", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "PLANE HEADING DEGREES TRUE", Unit: "degrees", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "PLANE HEADING DEGREES MAGNETIC", Unit: "degrees", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "VERTICAL SPEED", Unit: "feet per minute", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "PLANE PITCH DEGREES", Unit: "degrees", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "PLANE BANK DEGREES", Unit: "degrees", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "GROUND VELOCITY", Unit: "knots", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "AIRSPEED INDICATED", Unit: "knots", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "AIRSPEED TRUE", Unit: "knots", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "ON ANY RUNWAY", Unit: "bool", Type: types.SIMCONNECT_DATATYPE_INT32, Epsilon: 0},
			{Name: "SURFACE TYPE", Unit: "enum", Type: types.SIMCONNECT_DATATYPE_INT32, Epsilon: 0},
			{Name: "SIM ON GROUND", Unit: "bool", Type: types.SIMCONNECT_DATATYPE_INT32, Epsilon: 0},
			{Name: "ATC ID", Unit: "", Type: types.SIMCONNECT_DATATYPE_STRING32, Epsilon: 0},
			{Name: "ATC AIRLINE", Unit: "", Type: types.SIMCONNECT_DATATYPE_STRING32, Epsilon: 0},
			{Name: "AMBIENT IN CLOUD", Unit: "bool", Type: types.SIMCONNECT_DATATYPE_INT32, Epsilon: 0},
			{Name: "IS USER SIM", Unit: "bool", Type: types.SIMCONNECT_DATATYPE_INT32, Epsilon: 0},
			{Name: "TOW CONNECTION", Unit: "bool", Type: types.SIMCONNECT_DATATYPE_INT32, Epsilon: 0},
			{Name: "PLANE ALT ABOVE GROUND", Unit: "feet", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "WING SPAN", Unit: "feet", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
		},
	}
}

type AircraftDataset struct {
	Title             [128]byte
	Category          [128]byte
	LiveryName        [128]byte
	LiveryFolder      [128]byte
	Lat               float64
	Lon               float64
	Alt               float64
	Head              float64
	HeadMag           float64
	Vs                float64
	Pitch             float64
	Bank              float64
	GroundSpeed       float64
	AirspeedIndicated float64
	AirspeedTrue      float64
	OnAnyRunway       int32
	SurfaceType       int32
	SimOnGround       int32
	AtcID             [32]byte
	AtcAirline        [32]byte
	AmbientInCloud    int32
	IsUserSim         int32
	IsTowConnected    int32
	AltAboveGround    float64
	WingSpan          float64
}
