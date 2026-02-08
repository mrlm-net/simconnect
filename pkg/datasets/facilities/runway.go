//go:build windows
// +build windows

package facilities

import "github.com/mrlm-net/simconnect/pkg/datasets"

func NewRunwayFacilityDataset() *datasets.FacilityDataSet {
	return &datasets.FacilityDataSet{
		Definitions: []datasets.FacilityDataDefinition{
			"OPEN RUNWAY",
			"LATITUDE",
			"LONGITUDE",
			"ALTITUDE",
			"HEADING",
			"LENGTH",
			"WIDTH",
			"PATTERN_ALTITUDE",
			"SLOPE",
			"TRUE_SLOPE",
			"SURFACE",
			"PRIMARY_ILS_ICAO",
			"PRIMARY_ILS_REGION",
			"PRIMARY_ILS_TYPE",
			"PRIMARY_NUMBER",
			"PRIMARY_DESIGNATOR",
			"PRIMARY_THRESHOLD",
			"PRIMARY_BLASTPAD",
			"PRIMARY_OVERRUN",
			"PRIMARY_APPROACH_LIGHTS",
			"PRIMARY_LEFT_VASI",
			"PRIMARY_RIGHT_VASI",
			"SECONDARY_ILS_ICAO",
			"SECONDARY_ILS_REGION",
			"SECONDARY_ILS_TYPE",
			"SECONDARY_NUMBER",
			"SECONDARY_DESIGNATOR",
			"SECONDARY_THRESHOLD",
			"SECONDARY_BLASTPAD",
			"SECONDARY_OVERRUN",
			"SECONDARY_APPROACH_LIGHTS",
			"SECONDARY_LEFT_VASI",
			"SECONDARY_RIGHT_VASI",
			"CLOSE RUNWAY",
		},
	}
}
