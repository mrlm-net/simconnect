//go:build windows
// +build windows

package engine

import "github.com/mrlm-net/simconnect/pkg/types"

// The add-on camera (MSFS 2024 only): acquire it, place it, give it back.
// Its messages (SIMCONNECT_RECV_CAMERA_STATUS, _CAMERA_DATA) come on
// Stream(): AsCameraStatus, AsCameraData.

// CameraAcquire acquires the add-on camera for clientID.
func (e *Engine) CameraAcquire(clientID string) error { return e.api.CameraAcquire(clientID) }

// CameraRelease gives the camera back, to camera definition cameraDef ("" any).
func (e *Engine) CameraRelease(cameraDef string) error { return e.api.CameraRelease(cameraDef) }

// CameraGetStatus asks for the camera's status.
func (e *Engine) CameraGetStatus() error { return e.api.CameraGetStatus() }

// CameraSet places the acquired camera: the members of data in mask.
func (e *Engine) CameraSet(data types.SIMCONNECT_DATA_CAMERA, mask types.SIMCONNECT_CAMERA_DATA_MASK) error {
	return e.api.CameraSet(data, mask)
}

// CameraGet asks for the camera's placement in referential.
func (e *Engine) CameraGet(referential types.SIMCONNECT_POSITION_REFERENTIAL) error {
	return e.api.CameraGet(referential)
}

// CameraEnableFlag and CameraDisableFlag set the camera's flags.
func (e *Engine) CameraEnableFlag(flag types.SIMCONNECT_CAMERA_FLAG) error {
	return e.api.CameraEnableFlag(flag)
}

func (e *Engine) CameraDisableFlag(flag types.SIMCONNECT_CAMERA_FLAG) error {
	return e.api.CameraDisableFlag(flag)
}

// SubscribeToCameraStatusUpdate sends the camera's status on every change.
func (e *Engine) SubscribeToCameraStatusUpdate() error { return e.api.SubscribeToCameraStatusUpdate() }

// UnsubscribeToCameraStatusUpdate stops them.
func (e *Engine) UnsubscribeToCameraStatusUpdate() error {
	return e.api.UnsubscribeToCameraStatusUpdate()
}

// EnumerateCameraDefinitions asks for the camera definitions.
func (e *Engine) EnumerateCameraDefinitions() error { return e.api.EnumerateCameraDefinitions() }

// CameraSetUsingCameraDefinition switches to a camera definition.
func (e *Engine) CameraSetUsingCameraDefinition(cameraDef string) error {
	return e.api.CameraSetUsingCameraDefinition(cameraDef)
}

// RequestCameraWorldLocker keeps the world loaded around position while
// the camera is away from the user aircraft; delete it when done.
func (e *Engine) RequestCameraWorldLocker(position types.SIMCONNECT_DATA_XYZ, referential types.SIMCONNECT_POSITION_REFERENTIAL, objectID uint32) error {
	return e.api.RequestCameraWorldLocker(position, referential, objectID)
}

// DeleteCameraWorldLocker lets the world unload again.
func (e *Engine) DeleteCameraWorldLocker() error { return e.api.DeleteCameraWorldLocker() }

// SubscribeToCameraWorldLockerStatusUpdate sends the locker's status on every change.
func (e *Engine) SubscribeToCameraWorldLockerStatusUpdate() error {
	return e.api.SubscribeToCameraWorldLockerStatusUpdate()
}
