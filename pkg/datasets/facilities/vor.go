//go:build windows
// +build windows

package facilities

import "github.com/mrlm-net/simconnect/pkg/datasets"

func NewVORFacilityDataset() *datasets.FacilityDataSet {
	return &datasets.FacilityDataSet{
		Definitions: []datasets.FacilityDataDefinition{
			"OPEN VOR",
			"VOR_LATITUDE",
			"VOR_LONGITUDE",
			"VOR_ALTITUDE",
			"DME_LATITUDE",
			"DME_LONGITUDE",
			"DME_ALTITUDE",
			"GS_LATITUDE",
			"GS_LONGITUDE",
			"GS_ALTITUDE",
			"TACAN_LATITUDE",
			"TACAN_LONGITUDE",
			"TACAN_ALTITUDE",
			"IS_NAV",
			"IS_DME",
			"IS_TACAN",
			"HAS_GLIDE_SLOPE",
			"DME_AT_NAV",
			"DME_AT_GLIDE_SLOPE",
			"HAS_BACK_COURSE",
			"FREQUENCY",
			"TYPE",
			"NAV_RANGE",
			"MAGVAR",
			"LOCALIZER",
			"LOCALIZER_WIDTH",
			"GLIDE_SLOPE",
			"NAME",
			"CLOSE VOR",
		},
	}
}
