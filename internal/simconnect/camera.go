//go:build windows
// +build windows

package simconnect

import (
	"fmt"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// The add-on camera API (MSFS 2024 only).
//
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Camera/SimConnect_CameraAcquire.htm

// cameraCall calls a camera procedure and turns its HRESULT into an error.
func (sc *SimConnect) cameraCall(name string, args ...uintptr) error {
	procedure := sc.proc(name)
	hresult, _, _ := procedure.Call(append([]uintptr{sc.getConnection()}, args...)...)
	if !isHRESULTSuccess(hresult) {
		return fmt.Errorf("%s failed with HRESULT: 0x%08X", name, uint32(hresult))
	}
	return nil
}

// CameraAcquire acquires the add-on camera for clientID; a
// SIMCONNECT_RECV_CAMERA_STATUS follows.
func (sc *SimConnect) CameraAcquire(clientID string) error {
	s, err := stringToBytePtr(clientID)
	if err != nil {
		return err
	}
	return sc.cameraCall("SimConnect_CameraAcquire", uintptr(unsafe.Pointer(s)))
}

// CameraRelease gives the camera back, switching to camera definition
// cameraDef ("" the simulator's choice).
func (sc *SimConnect) CameraRelease(cameraDef string) error {
	s, err := stringToBytePtr(cameraDef)
	if err != nil {
		return err
	}
	return sc.cameraCall("SimConnect_CameraRelease", uintptr(unsafe.Pointer(s)))
}

// CameraGetStatus asks for a SIMCONNECT_RECV_CAMERA_STATUS.
func (sc *SimConnect) CameraGetStatus() error {
	return sc.cameraCall("SimConnect_CameraGetStatus")
}

// CameraSet places the acquired camera: the members of data in mask.
// The struct is passed by value; on x64 a struct this size goes as a
// pointer to a copy.
func (sc *SimConnect) CameraSet(data types.SIMCONNECT_DATA_CAMERA, mask types.SIMCONNECT_CAMERA_DATA_MASK) error {
	b := data.Bytes()
	return sc.cameraCall("SimConnect_CameraSet", uintptr(unsafe.Pointer(&b[0])), uintptr(mask))
}

// CameraGet asks for a SIMCONNECT_RECV_CAMERA_DATA in referential.
func (sc *SimConnect) CameraGet(referential types.SIMCONNECT_POSITION_REFERENTIAL) error {
	return sc.cameraCall("SimConnect_CameraGet", uintptr(referential))
}

// CameraEnableFlag and CameraDisableFlag set the camera's flags.
func (sc *SimConnect) CameraEnableFlag(flag types.SIMCONNECT_CAMERA_FLAG) error {
	return sc.cameraCall("SimConnect_CameraEnableFlag", uintptr(flag))
}

func (sc *SimConnect) CameraDisableFlag(flag types.SIMCONNECT_CAMERA_FLAG) error {
	return sc.cameraCall("SimConnect_CameraDisableFlag", uintptr(flag))
}

// SubscribeToCameraStatusUpdate sends a status on every change.
func (sc *SimConnect) SubscribeToCameraStatusUpdate() error {
	return sc.cameraCall("SimConnect_SubscribeToCameraStatusUpdate")
}

func (sc *SimConnect) UnsubscribeToCameraStatusUpdate() error {
	return sc.cameraCall("SimConnect_UnsubscribeToCameraStatusUpdate")
}

// EnumerateCameraDefinitions asks for a SIMCONNECT_RECV_CAMERA_DEFINITION_LIST.
func (sc *SimConnect) EnumerateCameraDefinitions() error {
	return sc.cameraCall("SimConnect_EnumerateCameraDefinitions")
}

// CameraSetUsingCameraDefinition switches to a camera definition.
func (sc *SimConnect) CameraSetUsingCameraDefinition(cameraDef string) error {
	s, err := stringToBytePtr(cameraDef)
	if err != nil {
		return err
	}
	return sc.cameraCall("SimConnect_CameraSetUsingCameraDefinition", uintptr(unsafe.Pointer(s)))
}

// RequestCameraWorldLocker keeps the terrain, scenery and objects around
// position loaded while the camera is away from the user aircraft
// (performance heavy: delete it when done). A
// SIMCONNECT_RECV_CAMERA_WORLD_LOCKER follows.
func (sc *SimConnect) RequestCameraWorldLocker(position types.SIMCONNECT_DATA_XYZ, referential types.SIMCONNECT_POSITION_REFERENTIAL, objectID uint32) error {
	p := position // by value: on x64 a pointer to a copy
	return sc.cameraCall("SimConnect_RequestCameraWorldLocker", uintptr(unsafe.Pointer(&p)), uintptr(referential), uintptr(objectID))
}

// DeleteCameraWorldLocker lets the world unload again.
func (sc *SimConnect) DeleteCameraWorldLocker() error {
	return sc.cameraCall("SimConnect_DeleteCameraWorldLocker")
}

// SubscribeToCameraWorldLockerStatusUpdate sends the locker's status on
// every change.
func (sc *SimConnect) SubscribeToCameraWorldLockerStatusUpdate() error {
	return sc.cameraCall("SimConnect_SubscribeToCameraWorldLockerStatusUpdate")
}
