//go:build windows
// +build windows

package aircraft

import (
	"github.com/mrlm-net/simconnect/pkg/datasets"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// NewPositionDataset returns a dataset for aircraft spatial position and orientation.
func NewPositionDataset() *datasets.DataSet {
	return &datasets.DataSet{
		Definitions: []datasets.DataDefinition{
			{Name: "PLANE LATITUDE", Unit: "degrees", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "PLANE LONGITUDE", Unit: "degrees", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "PLANE ALTITUDE", Unit: "feet", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "PLANE PITCH DEGREES", Unit: "degrees", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "PLANE BANK DEGREES", Unit: "degrees", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "PLANE HEADING DEGREES TRUE", Unit: "degrees", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "VERTICAL SPEED", Unit: "feet per minute", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "GROUND VELOCITY", Unit: "knots", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
		},
	}
}

// PositionDataset is the companion struct for NewPositionDataset.
// Fields must remain in the same order as the DataDefinitions slice.
type PositionDataset struct {
	Latitude      float64
	Longitude     float64
	Altitude      float64
	Pitch         float64
	Bank          float64
	HeadingTrue   float64
	VerticalSpeed float64
	GroundSpeed   float64
}

// NewAirspeedDataset returns a dataset for aircraft airspeed indicators.
func NewAirspeedDataset() *datasets.DataSet {
	return &datasets.DataSet{
		Definitions: []datasets.DataDefinition{
			{Name: "AIRSPEED INDICATED", Unit: "knots", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "AIRSPEED TRUE", Unit: "knots", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "AIRSPEED MACH", Unit: "mach", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
		},
	}
}

// AirspeedDataset is the companion struct for NewAirspeedDataset.
// Fields must remain in the same order as the DataDefinitions slice.
type AirspeedDataset struct {
	AirspeedIndicated float64
	AirspeedTrue      float64
	AirspeedMach      float64
}

// NewEngineDataset returns a dataset for engine state across up to four engines and fuel.
func NewEngineDataset() *datasets.DataSet {
	return &datasets.DataSet{
		Definitions: []datasets.DataDefinition{
			{Name: "ENG RPM:1", Unit: "rpm", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "ENG RPM:2", Unit: "rpm", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "ENG RPM:3", Unit: "rpm", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "ENG RPM:4", Unit: "rpm", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "THROTTLE LOWER LIMIT", Unit: "percent", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "FUEL TOTAL QUANTITY", Unit: "gallons", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "ENG FUEL FLOW GPH:1", Unit: "gallons per hour", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
		},
	}
}

// EngineDataset is the companion struct for NewEngineDataset.
// Fields must remain in the same order as the DataDefinitions slice.
type EngineDataset struct {
	EngRPM1            float64
	EngRPM2            float64
	EngRPM3            float64
	EngRPM4            float64
	ThrottleLowerLimit float64
	FuelTotalQuantity  float64
	FuelFlowGPH1       float64
}

// NewControlSurfacesDataset returns a dataset for flight control surface positions.
func NewControlSurfacesDataset() *datasets.DataSet {
	return &datasets.DataSet{
		Definitions: []datasets.DataDefinition{
			{Name: "AILERON POSITION", Unit: "position", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "ELEVATOR POSITION", Unit: "position", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "RUDDER POSITION", Unit: "position", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "FLAPS HANDLE INDEX", Unit: "number", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "GEAR HANDLE POSITION", Unit: "bool", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
		},
	}
}

// ControlSurfacesDataset is the companion struct for NewControlSurfacesDataset.
// Fields must remain in the same order as the DataDefinitions slice.
type ControlSurfacesDataset struct {
	AileronPosition  float64
	ElevatorPosition float64
	RudderPosition   float64
	FlapsHandleIndex float64
	GearHandlePos    float64
}
