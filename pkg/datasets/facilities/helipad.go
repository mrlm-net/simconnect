//go:build windows
// +build windows

package facilities

import "github.com/mrlm-net/simconnect/pkg/datasets"

func NewHelipadFacilityDataset() *datasets.FacilityDataSet {
	return &datasets.FacilityDataSet{
		Definitions: []datasets.FacilityDataDefinition{
			"OPEN HELIPAD",
			"LATITUDE",
			"LONGITUDE",
			"ALTITUDE",
			"HEADING",
			"LENGTH",
			"WIDTH",
			"SURFACE",
			"TYPE",
			"CLOSE HELIPAD",
		},
	}
}
