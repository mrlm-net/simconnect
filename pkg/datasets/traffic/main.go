//go:build windows
// +build windows

package traffic

import (
	"github.com/mrlm-net/simconnect/pkg/datasets"
	"github.com/mrlm-net/simconnect/pkg/types"
)

func NewAircraftDataset(name string, id uint32) *datasets.DataSet {
	return &datasets.DataSet{
		Name:         name,
		DefinitionID: id,
		Definitions: []datasets.DataDefinition{
			{Name: "TITLE", Unit: "", Type: types.SIMCONNECT_DATATYPE_STRING128, Epsilon: 0},
			{Name: "CATEGORY", Unit: "", Type: types.SIMCONNECT_DATATYPE_STRING128, Epsilon: 0},
			{Name: "LIVERY_NAME", Unit: "", Type: types.SIMCONNECT_DATATYPE_STRING128, Epsilon: 0},
			{Name: "LIVERY_FOLDER", Unit: "", Type: types.SIMCONNECT_DATATYPE_STRING128, Epsilon: 0},
			{Name: "LATITUDE", Unit: "degrees", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "LONGITUDE", Unit: "degrees", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "ALTITUDE", Unit: "feet", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "HEADING", Unit: "degrees", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "MAGNETIC_HEADING", Unit: "degrees", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "VERTICAL_SPEED", Unit: "feet per minute", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "PITCH", Unit: "degrees", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "BANK", Unit: "degrees", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "GROUND_SPEED", Unit: "knots", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "INDICATED_AIRSPEED", Unit: "knots", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "TRUE_AIRSPEED", Unit: "knots", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "ON_ANY_RUNWAY", Unit: "bool", Type: types.SIMCONNECT_DATATYPE_INT32, Epsilon: 0},
			{Name: "SURFACE_TYPE", Unit: "enum", Type: types.SIMCONNECT_DATATYPE_INT32, Epsilon: 0},
			{Name: "SIM_ON_GROUND", Unit: "bool", Type: types.SIMCONNECT_DATATYPE_INT32, Epsilon: 0},
			{Name: "ATC_ID", Unit: "", Type: types.SIMCONNECT_DATATYPE_STRING32, Epsilon: 0},
			{Name: "ATC_AIRLINE", Unit: "", Type: types.SIMCONNECT_DATATYPE_STRING32, Epsilon: 0},
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
}
