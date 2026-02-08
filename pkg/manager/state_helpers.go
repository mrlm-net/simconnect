//go:build windows
// +build windows

package manager

// IsInGame reports whether the simulator is in any playable or interactive state.
// Returns true for running game, loading screen, in-game menu, or drone camera.
func IsInGame(state SimState) bool {
	return IsInRunningGame(state) ||
		IsInLoadingGame(state) ||
		IsInGameMenuStates(state) ||
		IsInDroneCamera(state)
}

// IsInGameChange reports whether the in-game state changed between two states.
// Useful for detecting transitions into or out of the game.
func IsInGameChange(old, new SimState) bool {
	return IsInGame(old) != IsInGame(new)
}

// IsInRunningGame reports whether the simulator is actively running a flight.
// Requires SimRunning and initialized camera and substate.
func IsInRunningGame(state SimState) bool {
	return state.SimRunning &&
		state.Camera != CameraStateUninitialized &&
		state.Substate != CameraSubstateUninitialized
}

// IsInLoadingGame reports whether the simulator is showing a loading screen.
func IsInLoadingGame(state SimState) bool {
	return state.Camera == CameraStateInGameLoading
}

// IsInGameMenuMainBug reports whether a state change matches the known MSFS bug
// where camera state oscillates between InGameMenu and MainMenu rapidly.
func IsInGameMenuMainBug(old, new SimState) bool {
	return (old.Camera == CameraStateInGameMenu && new.Camera == CameraStateMainMenu) ||
		(old.Camera == CameraStateMainMenu && new.Camera == CameraStateInGameMenu)
}

// IsInGameMenuStates reports whether the simulator is in any in-game menu state.
// Includes in-game menu, in-game RTC, and in-game menu animation.
func IsInGameMenuStates(state SimState) bool {
	return state.Camera == CameraStateInGameMenu ||
		state.Camera == CameraStateInGameRTC ||
		state.Camera == CameraStateInGameMenuAnimation
}

// IsInDroneCamera reports whether the simulator is in drone camera mode.
func IsInDroneCamera(state SimState) bool {
	return state.Camera == CameraStateDrone
}

// IsPlaying reports whether the simulator is actively playing a flight.
// Requires the game to be unpaused, running, and in an active flight state.
func IsPlaying(state SimState) bool {
	return !state.Paused && state.SimRunning && IsInRunningGame(state)
}
