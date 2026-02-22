//go:build windows
// +build windows

package facilities

import "github.com/mrlm-net/simconnect/pkg/datasets"

func NewNDBFacilityDataset() *datasets.FacilityDataSet {
	return &datasets.FacilityDataSet{
		Definitions: []datasets.FacilityDataDefinition{
			"OPEN NDB",
			"ICAO",
			"REGION",
			"LATITUDE",
			"LONGITUDE",
			"ALTITUDE",
			"FREQUENCY",
			"TYPE",
			"RANGE",
			"MAGVAR",
			"IS_TERMINAL_NDB",
			"NAME",
			"CLOSE NDB",
		},
	}
}
