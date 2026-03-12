//go:build windows
// +build windows

package simulator

import (
	"github.com/mrlm-net/simconnect/pkg/datasets"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// NewSimStateDataset returns a dataset for core simulator state variables.
func NewSimStateDataset() *datasets.DataSet {
	return &datasets.DataSet{
		Definitions: []datasets.DataDefinition{
			{Name: "SIM ON GROUND", Unit: "bool", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "SURFACE TYPE", Unit: "enum", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "IS USER SIM", Unit: "bool", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "TOTAL WEIGHT", Unit: "pounds", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "CRASH FLAG", Unit: "enum", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
		},
	}
}

// SimStateDataset is the companion struct for NewSimStateDataset.
// Fields must remain in the same order as the DataDefinitions slice.
type SimStateDataset struct {
	SimOnGround float64
	SurfaceType float64
	IsUserSim   float64
	TotalWeight float64
	CrashFlag   float64 // bitmask: cast to uint32 before bit-testing
}

// NewCameraDataset returns a dataset for camera and view state variables.
func NewCameraDataset() *datasets.DataSet {
	return &datasets.DataSet{
		Definitions: []datasets.DataDefinition{
			{Name: "CAMERA STATE", Unit: "enum", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "CAMERA SUBSTATE", Unit: "enum", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "CAMERA VIEW TYPE INDEX:0", Unit: "number", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
		},
	}
}

// CameraDataset is the companion struct for NewCameraDataset.
// Fields must remain in the same order as the DataDefinitions slice.
type CameraDataset struct {
	CameraState    float64
	CameraSubstate float64
	CameraViewType float64
}
