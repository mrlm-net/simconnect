//go:build windows
// +build windows

package facilities

import "github.com/mrlm-net/simconnect/pkg/datasets"

func NewPavementFacilityDataset() *datasets.FacilityDataSet {
	return &datasets.FacilityDataSet{
		Definitions: []datasets.FacilityDataDefinition{
			"OPEN PAVEMENT",
			"LENGTH",
			"WIDTH",
			"ENABLE",
			"CLOSE PAVEMENT",
		},
	}
}

func NewApproachLightsFacilityDataset() *datasets.FacilityDataSet {
	return &datasets.FacilityDataSet{
		Definitions: []datasets.FacilityDataDefinition{
			"OPEN APPROACHLIGHTS",
			"SYSTEM",
			"STROBE_COUNT",
			"HAS_END_LIGHTS",
			"HAS_REIL_LIGHTS",
			"HAS_TOUCHDOWN_LIGHTS",
			"ON_GROUND",
			"ENABLE",
			"OFFSET",
			"SPACING",
			"SLOPE",
			"CLOSE APPROACHLIGHTS",
		},
	}
}

func NewVASIFacilityDataset() *datasets.FacilityDataSet {
	return &datasets.FacilityDataSet{
		Definitions: []datasets.FacilityDataDefinition{
			"OPEN VASI",
			"TYPE",
			"BIAS_X",
			"BIAS_Z",
			"SPACING",
			"ANGLE",
			"CLOSE VASI",
		},
	}
}

func NewStartFacilityDataset() *datasets.FacilityDataSet {
	return &datasets.FacilityDataSet{
		Definitions: []datasets.FacilityDataDefinition{
			"OPEN START",
			"LATITUDE",
			"LONGITUDE",
			"ALTITUDE",
			"HEADING",
			"NUMBER",
			"DESIGNATOR",
			"TYPE",
			"CLOSE START",
		},
	}
}
