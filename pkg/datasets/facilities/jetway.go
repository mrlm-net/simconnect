//go:build windows
// +build windows

package facilities

import "github.com/mrlm-net/simconnect/pkg/datasets"

func NewJetwayFacilityDataset() *datasets.FacilityDataSet {
	return &datasets.FacilityDataSet{
		Definitions: []datasets.FacilityDataDefinition{
			"OPEN JETWAY",
			"PARKING_GATE",
			"PARKING_SUFFIX",
			"PARKING_SPOT",
			"CLOSE JETWAY",
		},
	}
}
