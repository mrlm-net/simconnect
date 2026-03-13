//go:build windows
// +build windows

package environment

import (
	"github.com/mrlm-net/simconnect/pkg/datasets"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// NewWeatherDataset returns a dataset for ambient environmental and weather conditions.
func NewWeatherDataset() *datasets.DataSet {
	return &datasets.DataSet{
		Definitions: []datasets.DataDefinition{
			{Name: "AMBIENT TEMPERATURE", Unit: "celsius", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "AMBIENT PRESSURE", Unit: "inHg", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "AMBIENT WIND DIRECTION", Unit: "degrees", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "AMBIENT WIND VELOCITY", Unit: "knots", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "AMBIENT VISIBILITY", Unit: "meters", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "AMBIENT PRECIP RATE", Unit: "millimeters of water", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "AMBIENT PRECIP STATE", Unit: "mask", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
		},
	}
}

// WeatherDataset is the companion struct for NewWeatherDataset.
// Fields must remain in the same order as the DataDefinitions slice.
type WeatherDataset struct {
	Temperature   float64
	Pressure      float64
	WindDirection float64
	WindVelocity  float64
	Visibility    float64
	PrecipRate    float64
	PrecipState   float64 // bitmask: 2=None, 4=Rain, 8=Snow; cast to uint32 before bit-testing
}

// NewTimeDataset returns a dataset for simulation time and date variables.
func NewTimeDataset() *datasets.DataSet {
	return &datasets.DataSet{
		Definitions: []datasets.DataDefinition{
			{Name: "LOCAL TIME", Unit: "seconds", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "ZULU TIME", Unit: "seconds", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "SIMULATION RATE", Unit: "number", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "ZULU DAY OF MONTH", Unit: "number", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "ZULU MONTH OF YEAR", Unit: "number", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "ZULU YEAR", Unit: "number", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
		},
	}
}

// TimeDataset is the companion struct for NewTimeDataset.
// Fields must remain in the same order as the DataDefinitions slice.
type TimeDataset struct {
	LocalTime      float64
	ZuluTime       float64
	SimulationRate float64
	ZuluDay        float64
	ZuluMonth      float64
	ZuluYear       float64
}
