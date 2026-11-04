package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/camera"
)

// A shot's end gives the camera back only while that shot still has it
// (endShot): spotting set as it ended, or a newer shot, stays (live, the
// spotting session's 12 s shot ended with spotting set in the same second,
// and the camera went off).
func TestShotEndKeepsAModeSetSince(t *testing.T) {
	m := &cameraMan{dir: &camera.Director{}, mode: "shot"}
	n := shotPlays.Add(1)
	m.mode = "spotting" // set by the watcher as the shot ends
	m.endShot(n)
	if m.mode != "spotting" {
		t.Errorf("spotting set at the shot's end: mode %q after it, want spotting", m.mode)
	}
	m.mode = "shot"
	shotPlays.Add(1) // a newer shot playing
	m.endShot(n)
	if m.mode != "shot" {
		t.Errorf("an older shot's end: mode %q, want the newer shot kept", m.mode)
	}
}
