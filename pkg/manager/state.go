//go:build windows
// +build windows

package manager

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
	Crashed    bool
	CrashReset bool
	Sound      uint32
	// Date fields (local and Zulu)
	LocalDay   int // LOCAL DAY OF MONTH
	LocalMonth int // LOCAL MONTH OF YEAR
	LocalYear  int // LOCAL YEAR
	ZuluDay    int // ZULU DAY OF MONTH
	ZuluMonth  int // ZULU MONTH OF YEAR
	ZuluYear   int // ZULU YEAR
	// Miscellaneous simulation variables
	Realism                float64 // REALISM
	VisualModelRadius      float64 // VISUAL MODEL RADIUS (meters)
	SimDisabled            bool    // SIM DISABLED
	RealismCrashDetection  bool    // REALISM CRASH DETECTION
	RealismCrashWithOthers bool    // REALISM CRASH WITH OTHERS
	TrackIREnabled         bool    // TRACK IR ENABLE
	UserInputEnabled       bool    // USER INPUT ENABLED
	SimOnGround            bool    // SIM ON GROUND
	// Environment variables
	AmbientTemperature   float64 // AMBIENT TEMPERATURE (Celsius)
	AmbientPressure      float64 // AMBIENT PRESSURE (inHg)
	AmbientWindVelocity  float64 // AMBIENT WIND VELOCITY (Knots)
	AmbientWindDirection float64 // AMBIENT WIND DIRECTION (Degrees)
	AmbientVisibility    float64 // AMBIENT VISIBILITY (Meters)
	AmbientInCloud       bool    // AMBIENT IN CLOUD
	AmbientPrecipState   uint32  // AMBIENT PRECIP STATE (Mask: 2=None, 4=Rain, 8=Snow)
	BarometerPressure    float64 // BAROMETER PRESSURE (Millibars)
	SeaLevelPressure     float64 // SEA LEVEL PRESSURE (Millibars)
	GroundAltitude       float64 // GROUND ALTITUDE (Feet)
	MagVar               float64 // MAGVAR (Degrees, magnetic variation)
	SurfaceType          uint32  // SURFACE TYPE (Surface type enum)
	// Aircraft position and orientation
	Latitude          float64 // PLANE LATITUDE (degrees)
	Longitude         float64 // PLANE LONGITUDE (degrees)
	Altitude          float64 // PLANE ALTITUDE (feet MSL)
	IndicatedAltitude float64 // INDICATED ALTITUDE (feet)
	TrueHeading       float64 // PLANE HEADING DEGREES TRUE (degrees)
	MagneticHeading   float64 // PLANE HEADING DEGREES MAGNETIC (degrees)
	Pitch             float64 // PLANE PITCH DEGREES (degrees)
	Bank              float64 // PLANE BANK DEGREES (degrees)
	// Aircraft speed
	GroundSpeed       float64 // GROUND VELOCITY (knots)
	IndicatedAirspeed float64 // AIRSPEED INDICATED (knots)
	TrueAirspeed      float64 // AIRSPEED TRUE (knots)
	VerticalSpeed     float64 // VERTICAL SPEED (feet per minute)
	// Camera extended
	SmartCameraActive bool // SMART CAMERA ACTIVE
	// Miscellaneous
	HandAnimState        int32   // HAND ANIM STATE (Enum: 0-12 frame IDs)
	HideAvatarInAircraft bool    // HIDE AVATAR IN AIRCRAFT
	MissionScore         float64 // MISSION SCORE
	ParachuteOpen        bool    // PARACHUTE OPEN
	// Environment time
	ZuluSunriseTime float64 // ZULU SUNRISE TIME (seconds since midnight Zulu)
	ZuluSunsetTime  float64 // ZULU SUNSET TIME (seconds since midnight Zulu)
	TimeZoneOffset  float64 // TIME ZONE OFFSET (seconds, local minus Zulu)
	// Environment units
	TooltipUnits   int32 // TOOLTIP UNITS (Enum: 0=Default, 1=Metric, 2=US)
	UnitsOfMeasure int32 // UNITS OF MEASURE (Enum: 0=English, 1=Metric/feet, 2=Metric/meters)
	// Environment weather (extended)
	AmbientInSmoke             bool    // AMBIENT IN SMOKE
	EnvSmokeDensity            float64 // ENV SMOKE DENSITY (Percent Over 100)
	EnvCloudDensity            float64 // ENV CLOUD DENSITY (Percent Over 100)
	DensityAltitude            float64 // DENSITY ALTITUDE (Feet)
	SeaLevelAmbientTemperature float64 // SEA LEVEL AMBIENT TEMPERATURE (Celsius)
}

// Equal returns true if two SimState values have equivalent significant state.
// Only compares discrete state fields that represent meaningful changes,
// ignoring continuously-changing values like time, position, weather, and speed.
func (s SimState) Equal(other SimState) bool {
	return s.Camera == other.Camera &&
		s.Substate == other.Substate &&
		s.Paused == other.Paused &&
		s.SimRunning == other.SimRunning &&
		s.SimulationRate == other.SimulationRate &&
		s.IsInVR == other.IsInVR &&
		s.IsUsingMotionControllers == other.IsUsingMotionControllers &&
		s.IsUsingJoystickThrottle == other.IsUsingJoystickThrottle &&
		s.IsInRTC == other.IsInRTC &&
		s.IsAvatar == other.IsAvatar &&
		s.IsAircraft == other.IsAircraft &&
		s.Crashed == other.Crashed &&
		s.CrashReset == other.CrashReset &&
		s.Sound == other.Sound &&
		s.Realism == other.Realism &&
		s.VisualModelRadius == other.VisualModelRadius &&
		s.SimDisabled == other.SimDisabled &&
		s.RealismCrashDetection == other.RealismCrashDetection &&
		s.RealismCrashWithOthers == other.RealismCrashWithOthers &&
		s.TrackIREnabled == other.TrackIREnabled &&
		s.UserInputEnabled == other.UserInputEnabled &&
		s.SimOnGround == other.SimOnGround &&
		s.SmartCameraActive == other.SmartCameraActive &&
		s.HandAnimState == other.HandAnimState &&
		s.HideAvatarInAircraft == other.HideAvatarInAircraft &&
		s.MissionScore == other.MissionScore &&
		s.ParachuteOpen == other.ParachuteOpen
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

// simStateDataStruct is the structure for simulator state data received from SimConnect.
// All fields are float64 regardless of logical type. SimConnect converts integer SimVars
// to float64 when SIMCONNECT_DATATYPE_FLOAT64 is requested.
// Using a uniform float64 layout eliminates Go struct alignment padding that would
// otherwise corrupt fields after any int32→float64 transition at an odd 4-byte boundary.
type simStateDataStruct struct {
	CameraState                float64
	CameraSubstate             float64
	SimulationRate             float64
	SimulationTime             float64
	LocalTime                  float64
	ZuluTime                   float64
	IsInVR                     float64
	IsUsingMotionControllers   float64
	IsUsingJoystickThrottle    float64
	IsInRTC                    float64
	IsAvatar                   float64
	IsAircraft                 float64
	LocalDay                   float64
	LocalMonth                 float64
	LocalYear                  float64
	ZuluDay                    float64
	ZuluMonth                  float64
	ZuluYear                   float64
	Realism                    float64
	VisualModelRadius          float64
	SimDisabled                float64
	RealismCrashDetection      float64
	RealismCrashWithOthers     float64
	TrackIREnabled             float64
	UserInputEnabled           float64
	SimOnGround                float64
	AmbientTemperature         float64
	AmbientPressure            float64
	AmbientWindVelocity        float64
	AmbientWindDirection       float64
	AmbientVisibility          float64
	AmbientInCloud             float64
	AmbientPrecipState         float64
	BarometerPressure          float64
	SeaLevelPressure           float64
	GroundAltitude             float64
	MagVar                     float64
	SurfaceType                float64
	Latitude                   float64
	Longitude                  float64
	Altitude                   float64
	IndicatedAltitude          float64
	TrueHeading                float64
	MagneticHeading            float64
	Pitch                      float64
	Bank                       float64
	GroundSpeed                float64
	IndicatedAirspeed          float64
	TrueAirspeed               float64
	VerticalSpeed              float64
	SmartCameraActive          float64
	HandAnimState              float64
	HideAvatarInAircraft       float64
	MissionScore               float64
	ParachuteOpen              float64
	ZuluSunriseTime            float64
	ZuluSunsetTime             float64
	TimeZoneOffset             float64
	TooltipUnits               float64
	UnitsOfMeasure             float64
	AmbientInSmoke             float64
	EnvSmokeDensity            float64
	EnvCloudDensity            float64
	DensityAltitude            float64
	SeaLevelAmbientTemperature float64
}
