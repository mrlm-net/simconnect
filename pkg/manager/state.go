//go:build windows
// +build windows

package manager

// CameraState represents the current camera view type in the simulator
// Based on CAMERA STATE SimVar from MSFS documentation
type CameraState int

const (
	// CameraStateUninitialized indicates camera state has not been determined yet
	CameraStateUninitialized CameraState = -1
	// CameraStateCockpit represents the Cockpit camera view
	CameraStateCockpit CameraState = 2
	// CameraStateExternalChase represents the External/Chase camera view
	CameraStateExternalChase CameraState = 3
	// CameraStateDrone represents the Drone camera view
	CameraStateDrone CameraState = 4
	// CameraStateFixedOnPlane represents the Fixed on Plane camera view
	CameraStateFixedOnPlane CameraState = 5
	// CameraStateEnvironment represents the Environment camera view
	CameraStateEnvironment CameraState = 6
	// CameraStateSixDoF represents the Six DoF camera view
	CameraStateSixDoF CameraState = 7
	// CameraStateGameplay represents the Gameplay camera view
	CameraStateGameplay CameraState = 8
	// CameraStateShowcase represents the Showcase camera view
	CameraStateShowcase CameraState = 9
	// CameraStateDroneAircraft represents the Drone Aircraft camera view
	CameraStateDroneAircraft CameraState = 10
	// CameraStateWaiting represents the Waiting state
	CameraStateWaiting CameraState = 11
	// CameraStateWorldMap represents the World Map view
	CameraStateWorldMap CameraState = 12
	// CameraStateHangarRTC represents the Hangar RTC view
	CameraStateHangarRTC CameraState = 13
	// CameraStateHangarCustom represents the Hangar Custom view
	CameraStateHangarCustom CameraState = 14
	// CameraStateMenuRTC represents the Menu RTC view
	CameraStateMenuRTC CameraState = 15
	// CameraStateInGameRTC represents the In-Game RTC view
	CameraStateInGameRTC CameraState = 16
	// CameraStateReplay represents the Replay view
	CameraStateReplay CameraState = 17
	// CameraStateDroneTopDown represents the Drone Top-Down camera view
	CameraStateDroneTopDown CameraState = 19
	// CameraStateHangar represents the Hangar view
	CameraStateHangar CameraState = 21
	// CameraStateGround represents the Ground camera view
	CameraStateGround CameraState = 24
	// CameraStateFollowTrafficAircraft represents the Follow Traffic Aircraft camera view
	CameraStateFollowTrafficAircraft CameraState = 25

	// Note: Some camera states from MSFS documentation are omitted due to being unused or deprecated
	CameraStateInGameMenuAnimation CameraState = 29
	CameraStateInGameLoading       CameraState = 30
	CameraStateMainMenu            CameraState = 32
	CameraStateInGameMenu          CameraState = 34
	CameraStateMainMenuAnimation   CameraState = 35
)

// String returns a human-readable representation of the camera state
func (s CameraState) String() string {
	switch s {
	case CameraStateUninitialized:
		return "Uninitialized"
	case CameraStateCockpit:
		return "Cockpit"
	case CameraStateExternalChase:
		return "External/Chase"
	case CameraStateDrone:
		return "Drone"
	case CameraStateFixedOnPlane:
		return "Fixed on Plane"
	case CameraStateEnvironment:
		return "Environment"
	case CameraStateSixDoF:
		return "Six DoF"
	case CameraStateGameplay:
		return "Gameplay"
	case CameraStateShowcase:
		return "Showcase"
	case CameraStateDroneAircraft:
		return "Drone Aircraft"
	case CameraStateWaiting:
		return "Waiting"
	case CameraStateWorldMap:
		return "World Map"
	case CameraStateHangarRTC:
		return "Hangar RTC"
	case CameraStateHangarCustom:
		return "Hangar Custom"
	case CameraStateMenuRTC:
		return "Menu RTC"
	case CameraStateInGameRTC:
		return "In-Game RTC"
	case CameraStateReplay:
		return "Replay"
	case CameraStateDroneTopDown:
		return "Drone Top-Down"
	case CameraStateHangar:
		return "Hangar"
	case CameraStateGround:
		return "Ground"
	case CameraStateFollowTrafficAircraft:
		return "Follow Traffic Aircraft"
	case CameraStateInGameLoading:
		return "In-Game Loading"
	case CameraStateMainMenu:
		return "Main Menu"
	case CameraStateMainMenuAnimation:
		return "Main Menu Animation"
	case CameraStateInGameMenuAnimation:
		return "In-Game Menu Animation"
	case CameraStateInGameMenu:
		return "In-Game Menu"
	default:
		return "Unknown"
	}
}

// CameraSubstate represents the sub-state of the camera
// Based on CAMERA SUBSTATE SimVar from MSFS documentation
type CameraSubstate int

const (
	// CameraSubstateUninitialized indicates substate has not been determined
	CameraSubstateUninitialized CameraSubstate = 0
	// CameraSubstateLocked indicates camera is locked in position (Fixed look or chase lock)
	CameraSubstateLocked CameraSubstate = 1
	// CameraSubstateUnlocked indicates camera is unlocked (Head look or Chase normal)
	CameraSubstateUnlocked CameraSubstate = 2
	// CameraSubstateQuickview indicates camera is using a Quickview
	CameraSubstateQuickview CameraSubstate = 3
	// CameraSubstateSmart indicates camera has Smart camera active
	CameraSubstateSmart CameraSubstate = 4
	// CameraSubstateInstrument indicates camera is focused on an instruments panel
	CameraSubstateInstrument CameraSubstate = 5
)

// String returns a human-readable representation of the camera substate
func (s CameraSubstate) String() string {
	switch s {
	case CameraSubstateUninitialized:
		return "Uninitialized"
	case CameraSubstateLocked:
		return "Locked"
	case CameraSubstateUnlocked:
		return "Unlocked"
	case CameraSubstateQuickview:
		return "Quickview"
	case CameraSubstateSmart:
		return "Smart"
	case CameraSubstateInstrument:
		return "Instrument"
	default:
		return "Unknown"
	}
}

// SimState represents the current simulator state with all monitored substates
type SimState struct {
	Camera     CameraState
	Substate   CameraSubstate
	Paused     bool
	SimRunning bool
	// Simulation variables
	SimulationRate float64 // SIMULATION RATE
	SimulationTime float64 // SIMULATION TIME (seconds since simulation start)
	LocalTime      float64 // LOCAL TIME (seconds since midnight local)
	ZuluTime       float64 // ZULU TIME (seconds since midnight Zulu)

	// Boolean environment flags (IS_*)
	IsInVR                   bool
	IsUsingMotionControllers bool
	IsUsingJoystickThrottle  bool
	IsInRTC                  bool
	IsAvatar                 bool
	IsAircraft               bool
	// Crash and sound flags
	Crashed     bool
	CrashReset  bool
	Sound uint32
	// Date fields (local and Zulu)
	LocalDay   int // LOCAL DAY OF MONTH
	LocalMonth int // LOCAL MONTH OF YEAR
	LocalYear  int // LOCAL YEAR
	ZuluDay    int // ZULU DAY OF MONTH
	ZuluMonth  int // ZULU MONTH OF YEAR
	ZuluYear   int // ZULU YEAR
}

// Equal returns true if two SimState values are equivalent
func (s SimState) Equal(other SimState) bool {
	return s.Camera == other.Camera &&
		s.Substate == other.Substate &&
		s.Paused == other.Paused &&
		s.SimRunning == other.SimRunning &&
		s.SimulationRate == other.SimulationRate &&
		s.SimulationTime == other.SimulationTime &&
		s.LocalTime == other.LocalTime &&
		s.ZuluTime == other.ZuluTime &&
		s.IsInVR == other.IsInVR &&
		s.IsUsingMotionControllers == other.IsUsingMotionControllers &&
		s.IsUsingJoystickThrottle == other.IsUsingJoystickThrottle &&
		s.IsInRTC == other.IsInRTC &&
		s.IsAvatar == other.IsAvatar &&
		s.IsAircraft == other.IsAircraft &&
		// Crash and sound flags
		s.Crashed == other.Crashed &&
		s.CrashReset == other.CrashReset &&
		s.Sound == other.Sound &&
		s.LocalDay == other.LocalDay &&
		s.LocalMonth == other.LocalMonth &&
		s.LocalYear == other.LocalYear &&
		s.ZuluDay == other.ZuluDay &&
		s.ZuluMonth == other.ZuluMonth &&
		s.ZuluYear == other.ZuluYear
}

// SimStateChange represents a simulator state transition event
type SimStateChange struct {
	OldState SimState
	NewState SimState
}

// SimStateChangeHandler is a callback function invoked when simulator state changes
type SimStateChangeHandler func(oldState, newState SimState)

// SimStateSubscription represents an active state change subscription that can be cancelled
type SimStateSubscription interface {
	// ID returns the unique identifier of the subscription
	ID() string

	// SimStateChanges returns the channel for receiving state changes
	SimStateChanges() <-chan SimStateChange

	// Done returns a channel that is closed when the subscription ends.
	// Use this to detect when to exit your consumer goroutine.
	Done() <-chan struct{}

	// Unsubscribe cancels the subscription and closes the channel.
	// Blocks until any pending change delivery completes.
	Unsubscribe()
}

// cameraDataStruct is the structure for camera data received from SimConnect
type cameraDataStruct struct {
	CameraState              int32
	CameraSubstate           int32
	SimulationRate           float64
	SimulationTime           float64
	LocalTime                float64
	ZuluTime                 float64
	IsInVR                   int32
	IsUsingMotionControllers int32
	IsUsingJoystickThrottle  int32
	IsInRTC                  int32
	IsAvatar                 int32
	IsAircraft               int32
	LocalDay                 int32
	LocalMonth               int32
	LocalYear                int32
	ZuluDay                  int32
	ZuluMonth                int32
	ZuluYear                 int32
}
