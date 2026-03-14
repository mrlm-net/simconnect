//go:build windows
// +build windows

// Package navigation provides pre-built SimConnect dataset definitions for
// navigation instruments: COM/NAV/ADF radios and GPS position/track data.
//
// Companion structs must be used for data unmarshalling; field order must
// remain in sync with the order of DataDefinition entries.
package navigation

import (
	"github.com/mrlm-net/simconnect/pkg/datasets"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// RadioDataset is the companion struct for NewRadioDataset.
// Fields must remain in the same order as the DataDefinitions slice.
type RadioDataset struct {
	Com1ActiveFreq  float64
	Com1StandbyFreq float64
	Nav1ActiveFreq  float64
	Nav1StandbyFreq float64
	Adf1ActiveFreq  float64
}

// NewRadioDataset returns a dataset for COM1, NAV1, and ADF1 radio frequencies.
// Data is requested using the indexed SimVar names for radio 1.
func NewRadioDataset() *datasets.DataSet {
	return &datasets.DataSet{
		Definitions: []datasets.DataDefinition{
			{Name: "COM ACTIVE FREQUENCY:1", Unit: "mhz", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "COM STANDBY FREQUENCY:1", Unit: "mhz", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "NAV ACTIVE FREQUENCY:1", Unit: "mhz", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "NAV STANDBY FREQUENCY:1", Unit: "mhz", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "ADF ACTIVE FREQUENCY:1", Unit: "hz", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
		},
	}
}

// GPSDataset is the companion struct for NewGPSDataset.
// Fields must remain in the same order as the DataDefinitions slice.
type GPSDataset struct {
	Latitude    float64
	Longitude   float64
	GroundSpeed float64
	GroundTrack float64
}

// NewGPSDataset returns a dataset for GPS-derived position and track data.
func NewGPSDataset() *datasets.DataSet {
	return &datasets.DataSet{
		Definitions: []datasets.DataDefinition{
			{Name: "GPS POSITION LAT", Unit: "degrees", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "GPS POSITION LON", Unit: "degrees", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "GPS GROUND SPEED", Unit: "meters per second", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
			{Name: "GPS GROUND MAGNETIC TRACK", Unit: "degrees", Type: types.SIMCONNECT_DATATYPE_FLOAT64, Epsilon: 0},
		},
	}
}
