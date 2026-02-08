//go:build windows
// +build windows

package facilities

import "github.com/mrlm-net/simconnect/pkg/datasets"

func NewWaypointFacilityDataset() *datasets.FacilityDataSet {
	return &datasets.FacilityDataSet{
		Definitions: []datasets.FacilityDataDefinition{
			"OPEN WAYPOINT",
			"LATITUDE",
			"LONGITUDE",
			"ALTITUDE",
			"TYPE",
			"MAGVAR",
			"N_ROUTES",
			"ICAO",
			"REGION",
			"IS_TERMINAL_WPT",
			"CLOSE WAYPOINT",
		},
	}
}

func NewRouteFacilityDataset() *datasets.FacilityDataSet {
	return &datasets.FacilityDataSet{
		Definitions: []datasets.FacilityDataDefinition{
			"OPEN ROUTE",
			"NAME",
			"TYPE",
			"NEXT_ICAO",
			"NEXT_REGION",
			"NEXT_TYPE",
			"NEXT_LATITUDE",
			"NEXT_LONGITUDE",
			"NEXT_ALTITUDE",
			"PREV_ICAO",
			"PREV_REGION",
			"PREV_TYPE",
			"PREV_ALTITUDE",
			"CLOSE ROUTE",
		},
	}
}
