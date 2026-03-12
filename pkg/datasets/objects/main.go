//go:build windows
// +build windows

package objects

import (
	"github.com/mrlm-net/simconnect/pkg/datasets"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// NewSimObjectPositionDataset returns a dataset for SimObject spatial position and ground state.
func NewSimObjectPositionDataset() *datasets.DataSet {
	return &datasets.DataSet{
		Definitions: []datasets.DataDefinition{
			{Name: "PLANE LATITUDE", Unit: "degrees", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "PLANE LONGITUDE", Unit: "degrees", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "PLANE ALTITUDE", Unit: "feet", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "PLANE PITCH DEGREES", Unit: "degrees", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "PLANE BANK DEGREES", Unit: "degrees", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "PLANE HEADING DEGREES TRUE", Unit: "degrees", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "GROUND VELOCITY", Unit: "knots", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "SIM ON GROUND", Unit: "bool", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
		},
	}
}

// SimObjectPositionDataset is the companion struct for NewSimObjectPositionDataset.
// Fields must remain in the same order as the DataDefinitions slice.
type SimObjectPositionDataset struct {
	Latitude    float64
	Longitude   float64
	Altitude    float64
	Pitch       float64
	Bank        float64
	HeadingTrue float64
	GroundSpeed float64
	SimOnGround float64
}
