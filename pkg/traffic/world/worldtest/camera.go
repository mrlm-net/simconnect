package worldtest

import (
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// The add-on camera (camera.API): no picture offline, but the World's
// camera and scenes run on it as on MSFS 2024. CameraSets counts the poses
// set.

// CameraAcquire takes the camera.
func (l *Loopback) CameraAcquire(clientID string) error {
	l.mu.Lock()
	l.cameraOwned = true
	l.mu.Unlock()
	l.cameraStatus(types.SIMCONNECT_CAMERA_ACQUIRED)
	return nil
}

// CameraRelease gives the camera back.
func (l *Loopback) CameraRelease(cameraDef string) error {
	l.mu.Lock()
	l.cameraOwned = false
	l.mu.Unlock()
	l.cameraStatus(types.SIMCONNECT_CAMERA_NOT_ACQUIRED)
	return nil
}

// cameraStatus tells the client whose the camera is, as the simulator does.
func (l *Loopback) cameraStatus(state types.SIMCONNECT_CAMERA_AVAILABILITY) {
	b := make([]byte, unsafe.Sizeof(types.SIMCONNECT_RECV_CAMERA_STATUS{}))
	m := (*types.SIMCONNECT_RECV_CAMERA_STATUS)(unsafe.Pointer(&b[0]))
	m.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_CAMERA_STATUS)
	m.AcquiredState = state
	l.emit(b)
}

// CameraSet sets the camera's pose.
func (l *Loopback) CameraSet(data types.SIMCONNECT_DATA_CAMERA, mask types.SIMCONNECT_CAMERA_DATA_MASK) error {
	l.mu.Lock()
	l.cameraSets++
	l.cameraPose = data
	l.mu.Unlock()
	return nil
}

// CameraPose is the camera's last pose set.
func (l *Loopback) CameraPose() types.SIMCONNECT_DATA_CAMERA {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.cameraPose
}

// CameraSets is how many camera poses were set.
func (l *Loopback) CameraSets() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.cameraSets
}
