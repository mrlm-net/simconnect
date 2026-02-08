//go:build windows
// +build windows

package facilities

import "github.com/mrlm-net/simconnect/pkg/datasets"

func NewAirportFacilityDataset() *datasets.FacilityDataSet {
	return &datasets.FacilityDataSet{
		Definitions: []datasets.FacilityDataDefinition{
			"OPEN AIRPORT",
			"LATITUDE",
			"LONGITUDE",
			"ALTITUDE",
			"MAGVAR",
			"NAME",
			"NAME64",
			"ICAO",
			"REGION",
			"TOWER_LATITUDE",
			"TOWER_LONGITUDE",
			"TOWER_ALTITUDE",
			"TRANSITION_ALTITUDE",
			"TRANSITION_LEVEL",
			"IS_CLOSED",
			"COUNTRY",
			"CITY_STATE",
			"CLOSE AIRPORT",
		},
	}
}
