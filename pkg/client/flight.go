//go:build windows
// +build windows

package client

import (
	"fmt"

	"github.com/mrlm-net/simconnect/pkg/helpers"
)

func (e *Engine) FlightSave(fileName string, title string, description string) error {
	// Convert strings to C-style for SimConnect
	fileNamePtr, err := helpers.StringToBytePtr(fileName)
	if err != nil {
		return fmt.Errorf("invalid variable name: %v", err)
	}

	titlePtr, err := helpers.StringToBytePtr(title)
	if err != nil {
		return fmt.Errorf("invalid variable name: %v", err)
	}

	descriptionPtr, err := helpers.StringToBytePtr(description)
	if err != nil {
		return fmt.Errorf("invalid variable name: %v", err)
	}

	hresult, _, _ := SimConnect_FlightLoad.Call(
		e.handle,       // hSimConnect (use handle directly, not getHandle())
		fileNamePtr,    // FiledName
		titlePtr,       // Title
		descriptionPtr, // Description
		0,              // Flags (0 for default - based on SimConnect documentation it is unused)
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_FlightLoad failed: 0x%08X", uint32(hresult))
	}

	return nil
}

func (e *Engine) FlightLoad(fileName string) error {
	// Convert strings to C-style for SimConnect
	fileNamePtr, err := helpers.StringToBytePtr(fileName)
	if err != nil {
		return fmt.Errorf("invalid variable name: %v", err)
	}

	hresult, _, _ := SimConnect_FlightLoad.Call(
		e.handle,    // hSimConnect (use handle directly, not getHandle())
		fileNamePtr, // FiledName
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_FlightLoad failed: 0x%08X", uint32(hresult))
	}

	return nil
}

func (e *Engine) FlightPlanLoad(fileName string) error {
	// Convert strings to C-style for SimConnect
	fileNamePtr, err := helpers.StringToBytePtr(fileName)
	if err != nil {
		return fmt.Errorf("invalid variable name: %v", err)
	}

	hresult, _, _ := SimConnect_FlightPlanLoad.Call(
		e.handle,    // hSimConnect (use handle directly, not getHandle())
		fileNamePtr, // FiledName
	)

	if !helpers.IsHRESULTSuccess(hresult) {
		return fmt.Errorf("SimConnect_FlightPlanLoad failed: 0x%08X", uint32(hresult))
	}

	return nil
}
