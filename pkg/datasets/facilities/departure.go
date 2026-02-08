//go:build windows
// +build windows

package facilities

import "github.com/mrlm-net/simconnect/pkg/datasets"

func NewDepartureFacilityDataset() *datasets.FacilityDataSet {
	return &datasets.FacilityDataSet{
		Definitions: []datasets.FacilityDataDefinition{
			"OPEN DEPARTURE",
			"NAME",
			"N_RUNWAY_TRANSITIONS",
			"N_ENROUTE_TRANSITIONS",
			"N_APPROACH_LEGS",
			"CLOSE DEPARTURE",
		},
	}
}

func NewArrivalFacilityDataset() *datasets.FacilityDataSet {
	return &datasets.FacilityDataSet{
		Definitions: []datasets.FacilityDataDefinition{
			"OPEN ARRIVAL",
			"NAME",
			"N_RUNWAY_TRANSITIONS",
			"N_ENROUTE_TRANSITIONS",
			"N_APPROACH_LEGS",
			"CLOSE ARRIVAL",
		},
	}
}

func NewRunwayTransitionFacilityDataset() *datasets.FacilityDataSet {
	return &datasets.FacilityDataSet{
		Definitions: []datasets.FacilityDataDefinition{
			"OPEN RUNWAY_TRANSITION",
			"RUNWAY_NUMBER",
			"RUNWAY_DESIGNATOR",
			"N_APPROACH_LEGS",
			"CLOSE RUNWAY_TRANSITION",
		},
	}
}

func NewEnrouteTransitionFacilityDataset() *datasets.FacilityDataSet {
	return &datasets.FacilityDataSet{
		Definitions: []datasets.FacilityDataDefinition{
			"OPEN ENROUTE_TRANSITION",
			"NAME",
			"N_APPROACH_LEGS",
			"CLOSE ENROUTE_TRANSITION",
		},
	}
}
