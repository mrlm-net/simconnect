package worldtest

import "github.com/mrlm-net/simconnect/pkg/types"

// The add-on camera (camera.API): no picture offline, but the World's
// camera and scenes run on it as on MSFS 2024. CameraSets counts the poses
// set.

// CameraAcquire takes the camera.
func (l *Loopback) CameraAcquire(clientID string) error {
	l.mu.Lock()
	l.cameraOwned = true
	l.mu.Unlock()
	return nil
}

// CameraRelease gives the camera back.
func (l *Loopback) CameraRelease(cameraDef string) error {
	l.mu.Lock()
	l.cameraOwned = false
	l.mu.Unlock()
	return nil
}

// CameraSet sets the camera's pose.
func (l *Loopback) CameraSet(data types.SIMCONNECT_DATA_CAMERA, mask types.SIMCONNECT_CAMERA_DATA_MASK) error {
	l.mu.Lock()
	l.cameraSets++
	l.mu.Unlock()
	return nil
}

// CameraSets is how many camera poses were set.
func (l *Loopback) CameraSets() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.cameraSets
}
