//go:build windows
// +build windows

package manager

import "testing"

func TestIsInGame(t *testing.T) {
	tests := []struct {
		name  string
		state SimState
		want  bool
	}{
		{
			name:  "cockpit running game",
			state: SimState{SimRunning: true, Camera: CameraStateCockpit, Substate: CameraSubstateUnlocked},
			want:  true,
		},
		{
			name:  "loading screen",
			state: SimState{Camera: CameraStateInGameLoading},
			want:  true,
		},
		{
			name:  "in-game menu",
			state: SimState{Camera: CameraStateInGameMenu},
			want:  true,
		},
		{
			name:  "drone camera",
			state: SimState{Camera: CameraStateDrone},
			want:  true,
		},
		{
			name:  "main menu",
			state: SimState{Camera: CameraStateMainMenu},
			want:  false,
		},
		{
			name:  "uninitialized",
			state: SimState{Camera: CameraStateUninitialized, Substate: CameraSubstateUninitialized},
			want:  false,
		},
		{
			name:  "world map",
			state: SimState{Camera: CameraStateWorldMap},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsInGame(tt.state); got != tt.want {
				t.Errorf("IsInGame() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsInGameChange(t *testing.T) {
	tests := []struct {
		name     string
		old, new SimState
		want     bool
	}{
		{
			name: "cockpit to main menu (game to no game)",
			old:  SimState{SimRunning: true, Camera: CameraStateCockpit, Substate: CameraSubstateUnlocked},
			new:  SimState{Camera: CameraStateMainMenu},
			want: true,
		},
		{
			name: "cockpit to drone (game to game)",
			old:  SimState{SimRunning: true, Camera: CameraStateCockpit, Substate: CameraSubstateUnlocked},
			new:  SimState{Camera: CameraStateDrone},
			want: false,
		},
		{
			name: "main menu to main menu (no game to no game)",
			old:  SimState{Camera: CameraStateMainMenu},
			new:  SimState{Camera: CameraStateMainMenu},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsInGameChange(tt.old, tt.new); got != tt.want {
				t.Errorf("IsInGameChange() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsInRunningGame(t *testing.T) {
	tests := []struct {
		name  string
		state SimState
		want  bool
	}{
		{
			name:  "running with cockpit and unlocked substate",
			state: SimState{SimRunning: true, Camera: CameraStateCockpit, Substate: CameraSubstateUnlocked},
			want:  true,
		},
		{
			name:  "not running",
			state: SimState{SimRunning: false, Camera: CameraStateCockpit, Substate: CameraSubstateUnlocked},
			want:  false,
		},
		{
			name:  "uninitialized camera",
			state: SimState{SimRunning: true, Camera: CameraStateUninitialized, Substate: CameraSubstateUnlocked},
			want:  false,
		},
		{
			name:  "uninitialized substate",
			state: SimState{SimRunning: true, Camera: CameraStateCockpit, Substate: CameraSubstateUninitialized},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsInRunningGame(tt.state); got != tt.want {
				t.Errorf("IsInRunningGame() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsInLoadingGame(t *testing.T) {
	tests := []struct {
		name  string
		state SimState
		want  bool
	}{
		{
			name:  "loading screen",
			state: SimState{Camera: CameraStateInGameLoading},
			want:  true,
		},
		{
			name:  "cockpit",
			state: SimState{Camera: CameraStateCockpit},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsInLoadingGame(tt.state); got != tt.want {
				t.Errorf("IsInLoadingGame() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsInGameMenuMainBug(t *testing.T) {
	tests := []struct {
		name     string
		old, new SimState
		want     bool
	}{
		{
			name: "InGameMenu to MainMenu",
			old:  SimState{Camera: CameraStateInGameMenu},
			new:  SimState{Camera: CameraStateMainMenu},
			want: true,
		},
		{
			name: "MainMenu to InGameMenu",
			old:  SimState{Camera: CameraStateMainMenu},
			new:  SimState{Camera: CameraStateInGameMenu},
			want: true,
		},
		{
			name: "Cockpit to MainMenu",
			old:  SimState{Camera: CameraStateCockpit},
			new:  SimState{Camera: CameraStateMainMenu},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsInGameMenuMainBug(tt.old, tt.new); got != tt.want {
				t.Errorf("IsInGameMenuMainBug() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsInGameMenuStates(t *testing.T) {
	tests := []struct {
		name  string
		state SimState
		want  bool
	}{
		{
			name:  "in-game menu",
			state: SimState{Camera: CameraStateInGameMenu},
			want:  true,
		},
		{
			name:  "in-game RTC",
			state: SimState{Camera: CameraStateInGameRTC},
			want:  true,
		},
		{
			name:  "in-game menu animation",
			state: SimState{Camera: CameraStateInGameMenuAnimation},
			want:  true,
		},
		{
			name:  "main menu",
			state: SimState{Camera: CameraStateMainMenu},
			want:  false,
		},
		{
			name:  "cockpit",
			state: SimState{Camera: CameraStateCockpit},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsInGameMenuStates(tt.state); got != tt.want {
				t.Errorf("IsInGameMenuStates() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsInDroneCamera(t *testing.T) {
	tests := []struct {
		name  string
		state SimState
		want  bool
	}{
		{
			name:  "drone camera",
			state: SimState{Camera: CameraStateDrone},
			want:  true,
		},
		{
			name:  "drone aircraft (not drone)",
			state: SimState{Camera: CameraStateDroneAircraft},
			want:  false,
		},
		{
			name:  "cockpit",
			state: SimState{Camera: CameraStateCockpit},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsInDroneCamera(tt.state); got != tt.want {
				t.Errorf("IsInDroneCamera() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsPlaying(t *testing.T) {
	tests := []struct {
		name  string
		state SimState
		want  bool
	}{
		{
			name:  "unpaused running cockpit unlocked",
			state: SimState{Paused: false, SimRunning: true, Camera: CameraStateCockpit, Substate: CameraSubstateUnlocked},
			want:  true,
		},
		{
			name:  "paused",
			state: SimState{Paused: true, SimRunning: true, Camera: CameraStateCockpit, Substate: CameraSubstateUnlocked},
			want:  false,
		},
		{
			name:  "not running",
			state: SimState{Paused: false, SimRunning: false, Camera: CameraStateCockpit, Substate: CameraSubstateUnlocked},
			want:  false,
		},
		{
			name:  "uninitialized",
			state: SimState{Paused: false, SimRunning: true, Camera: CameraStateUninitialized, Substate: CameraSubstateUninitialized},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPlaying(tt.state); got != tt.want {
				t.Errorf("IsPlaying() = %v, want %v", got, tt.want)
			}
		})
	}
}
