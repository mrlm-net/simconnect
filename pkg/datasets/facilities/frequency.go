//go:build windows
// +build windows

package facilities

import "github.com/mrlm-net/simconnect/pkg/datasets"

func NewFrequencyFacilityDataset() *datasets.FacilityDataSet {
	return &datasets.FacilityDataSet{
		Definitions: []datasets.FacilityDataDefinition{
			"OPEN FREQUENCY",
			"TYPE",
			"FREQUENCY",
			"NAME",
			"CLOSE FREQUENCY",
		},
	}
}
