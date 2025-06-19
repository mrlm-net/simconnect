//go:build windows
// +build windows

package client

import (
	"fmt"

	"github.com/mrlm-net/simconnect/pkg/helpers"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// AddToDataDefinition adds a data definition to the SimConnect client.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_AddToDataDefinition.htm
func (e *Engine) AddToDataDefinition(defineID int, datumName string, unitsName string, datumType types.SIMCONNECT_DATATYPE, epsilon float32, datumID int) error {

	//e.RegisterDataType(uint32(defineID), datumType)

	// Convert strings to C-style for SimConnect
	varNamePtr, err := helpers.StringToBytePtr(datumName)
	if err != nil {
		return fmt.Errorf("invalid variable name: %v", err)
	}

	unitsPtr, err := helpers.StringToBytePtr(unitsName)
	if err != nil {
		return fmt.Errorf("invalid units: %v", err)
	}

	// Call SimConnect_AddToDataDefinition with the specified data type
	hresult, _, _ := SimConnect_AddToDataDefinition.Call(
		e.getHandle(),      // hSimConnect
		uintptr(defineID),  // DefineID
		varNamePtr,         // DatumName
		unitsPtr,           // UnitsName
		uintptr(datumType), // DatumType
		uintptr(epsilon),   // fEpsilon
		uintptr(datumID),   // DatumID
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_AddToDataDefinition failed: 0x%08X", uint32(hresult))
	}

	return nil
}
