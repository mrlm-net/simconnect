//go:build windows
// +build windows

package facilities

import "github.com/mrlm-net/simconnect/pkg/datasets"

func NewParkingFacilityDataset() *datasets.FacilityDataSet {
	return &datasets.FacilityDataSet{
		Definitions: []datasets.FacilityDataDefinition{
			"OPEN TAXI_PARKING",
			"TYPE",
			"TAXI_POINT_TYPE",
			"NAME",
			"SUFFIX",
			"NUMBER",
			"ORIENTATION",
			"HEADING",
			"RADIUS",
			"BIAS_X",
			"BIAS_Z",
			"CLOSE TAXI_PARKING",
		},
	}
}
