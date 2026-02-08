//go:build windows
// +build windows

package facilities

import "github.com/mrlm-net/simconnect/pkg/datasets"

func NewTaxiPointFacilityDataset() *datasets.FacilityDataSet {
	return &datasets.FacilityDataSet{
		Definitions: []datasets.FacilityDataDefinition{
			"OPEN TAXI_POINT",
			"TYPE",
			"ORIENTATION",
			"BIAS_X",
			"BIAS_Z",
			"CLOSE TAXI_POINT",
		},
	}
}

func NewTaxiPathFacilityDataset() *datasets.FacilityDataSet {
	return &datasets.FacilityDataSet{
		Definitions: []datasets.FacilityDataDefinition{
			"OPEN TAXI_PATH",
			"TYPE",
			"WIDTH",
			"LEFT_HALF_WIDTH",
			"RIGHT_HALF_WIDTH",
			"WEIGHT",
			"RUNWAY_NUMBER",
			"RUNWAY_DESIGNATOR",
			"LEFT_EDGE",
			"LEFT_EDGE_LIGHTED",
			"RIGHT_EDGE",
			"RIGHT_EDGE_LIGHTED",
			"CENTER_LINE",
			"CENTER_LINE_LIGHTED",
			"START",
			"END",
			"NAME_INDEX",
			"CLOSE TAXI_PATH",
		},
	}
}

func NewTaxiNameFacilityDataset() *datasets.FacilityDataSet {
	return &datasets.FacilityDataSet{
		Definitions: []datasets.FacilityDataDefinition{
			"OPEN TAXI_NAME",
			"NAME",
			"CLOSE TAXI_NAME",
		},
	}
}
