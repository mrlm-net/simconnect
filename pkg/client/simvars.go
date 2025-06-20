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
		e.handle,           // hSimConnect (use handle directly, not getHandle())
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

// RequestDataOnSimObjectType implements types. Client.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_RequestDataOnSimObjectType.htm
func (e *Engine) RequestDataOnSimObjectType(reguest int, definition int, radius string, objectType types.SIMCONNECT_SIMOBJECT_TYPE) error {

	// Convert strings to C-style for SimConnect
	varRadiusPtr, err := helpers.StringToBytePtr(radius)
	if err != nil {
		return fmt.Errorf("invalid variable name: %v", err)
	}
	// Call SimConnect_RequestDataOnSimObject
	hresult, _, _ := SimConnect_RequestDataOnSimObjectType.Call(
		e.handle,            // hSimConnect (use handle directly, not getHandle())
		uintptr(reguest),    // RequestID
		uintptr(definition), // DefineID
		varRadiusPtr,        // Radius from user aircraft
		uintptr(objectType), // SimObjectType
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_RequestDataOnSimObjectType failed: 0x%08X", uint32(hresult))
	}
	return nil
}

// RequestDataOnSimObject implements types. Client.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_RequestDataOnSimObject.htm
func (e *Engine) RequestDataOnSimObject(reguest int, definition int, object int, period types.SIMCONNECT_PERIOD, flags types.SIMCONNECT_DATA_REQUEST_FLAG, origin int, interval int, limit int) error {
	// Call SimConnect_RequestDataOnSimObject
	hresult, _, _ := SimConnect_RequestDataOnSimObject.Call(
		e.handle,            // hSimConnect (use handle directly, not getHandle())
		uintptr(reguest),    // RequestID
		uintptr(definition), // DefineID
		uintptr(object),     // ObjectID (user aircraft)
		uintptr(period),     // Period
		uintptr(flags),      // Flags (use the parameter passed in)
		uintptr(origin),     // Origin
		uintptr(interval),   // Interval
		uintptr(limit),      // Limit
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_RequestDataOnSimObject failed: 0x%08X", uint32(hresult))
	}
	return nil
}

// SetDataOnSimObject implements types. Client.
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_SetDataOnSimObject.htm
func (e *Engine) SetDataOnSimObject(definition int, object int, flags types.SIMCONNECT_DATA_SET_FLAG, arrayCount int, unitSize int, data uintptr) error {
	// Call SimConnect_SetDataOnSimObject
	hresult, _, _ := SimConnect_SetDataOnSimObject.Call(
		e.handle,            // hSimConnect (use handle directly, not getHandle())
		uintptr(definition), // DefineID
		uintptr(object),     // ObjectID
		uintptr(types.SIMCONNECT_DATA_SET_FLAG_DEFAULT), // Flags
		uintptr(arrayCount),                             // ArrayCount (0 for single values)
		uintptr(unitSize),                               // cbUnitSize
		uintptr(data),                                   // pDataSet
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_SetDataOnSimObject failed: 0x%08X", uint32(hresult))
	}

	return nil
}
