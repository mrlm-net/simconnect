//go:build windows
// +build windows

package manager

import (
	"github.com/mrlm-net/simconnect/pkg/engine"
)

// processSimStateData handles SIMCONNECT_RECV_ID_SIMOBJECT_DATA for camera/simstate.
func (m *Instance) processSimStateData(msg engine.Message) {
	simObjMsg := msg.AsSimObjectData()
	if uint32(simObjMsg.DwRequestID) != m.cameraRequestID || uint32(simObjMsg.DwDefineID) != m.cameraDefinitionID {
		return
	}

	// Extract camera state and substate from data
	stateData := engine.CastDataAs[simStateDataStruct](&simObjMsg.DwData)

	// Build new state from sim state data (no lock needed)
	newState := SimState{
		Camera:                     CameraState(stateData.CameraState),
		Substate:                   CameraSubstate(stateData.CameraSubstate),
		SimulationRate:             stateData.SimulationRate,
		SimulationTime:             stateData.SimulationTime,
		LocalTime:                  stateData.LocalTime,
		ZuluTime:                   stateData.ZuluTime,
		IsInVR:                     stateData.IsInVR == 1,
		IsUsingMotionControllers:   stateData.IsUsingMotionControllers == 1,
		IsUsingJoystickThrottle:    stateData.IsUsingJoystickThrottle == 1,
		IsInRTC:                    stateData.IsInRTC == 1,
		IsAvatar:                   stateData.IsAvatar == 1,
		IsAircraft:                 stateData.IsAircraft == 1,
		LocalDay:                   int(stateData.LocalDay),
		LocalMonth:                 int(stateData.LocalMonth),
		LocalYear:                  int(stateData.LocalYear),
		ZuluDay:                    int(stateData.ZuluDay),
		ZuluMonth:                  int(stateData.ZuluMonth),
		ZuluYear:                   int(stateData.ZuluYear),
		Realism:                    stateData.Realism,
		VisualModelRadius:          stateData.VisualModelRadius,
		SimDisabled:                stateData.SimDisabled == 1,
		RealismCrashDetection:      stateData.RealismCrashDetection == 1,
		RealismCrashWithOthers:     stateData.RealismCrashWithOthers == 1,
		TrackIREnabled:             stateData.TrackIREnabled == 1,
		UserInputEnabled:           stateData.UserInputEnabled == 1,
		SimOnGround:                stateData.SimOnGround == 1,
		AmbientTemperature:         stateData.AmbientTemperature,
		AmbientPressure:            stateData.AmbientPressure,
		AmbientWindVelocity:        stateData.AmbientWindVelocity,
		AmbientWindDirection:       stateData.AmbientWindDirection,
		AmbientVisibility:          stateData.AmbientVisibility,
		AmbientInCloud:             stateData.AmbientInCloud == 1,
		AmbientPrecipState:         uint32(stateData.AmbientPrecipState),
		BarometerPressure:          stateData.BarometerPressure,
		SeaLevelPressure:           stateData.SeaLevelPressure,
		GroundAltitude:             stateData.GroundAltitude,
		MagVar:                     stateData.MagVar,
		SurfaceType:                uint32(stateData.SurfaceType),
		Latitude:                   stateData.Latitude,
		Longitude:                  stateData.Longitude,
		Altitude:                   stateData.Altitude,
		IndicatedAltitude:          stateData.IndicatedAltitude,
		TrueHeading:                stateData.TrueHeading,
		MagneticHeading:            stateData.MagneticHeading,
		Pitch:                      stateData.Pitch,
		Bank:                       stateData.Bank,
		GroundSpeed:                stateData.GroundSpeed,
		IndicatedAirspeed:          stateData.IndicatedAirspeed,
		TrueAirspeed:               stateData.TrueAirspeed,
		VerticalSpeed:              stateData.VerticalSpeed,
		SmartCameraActive:          stateData.SmartCameraActive == 1,
		HandAnimState:              stateData.HandAnimState,
		HideAvatarInAircraft:       stateData.HideAvatarInAircraft == 1,
		MissionScore:               stateData.MissionScore,
		ParachuteOpen:              stateData.ParachuteOpen == 1,
		ZuluSunriseTime:            stateData.ZuluSunriseTime,
		ZuluSunsetTime:             stateData.ZuluSunsetTime,
		TimeZoneOffset:             stateData.TimeZoneOffset,
		TooltipUnits:               stateData.TooltipUnits,
		UnitsOfMeasure:             stateData.UnitsOfMeasure,
		AmbientInSmoke:             stateData.AmbientInSmoke == 1,
		EnvSmokeDensity:            stateData.EnvSmokeDensity,
		EnvCloudDensity:            stateData.EnvCloudDensity,
		DensityAltitude:            stateData.DensityAltitude,
		SeaLevelAmbientTemperature: stateData.SeaLevelAmbientTemperature,
	}

	// Short lock: preserve event-driven fields and compare
	m.mu.Lock()
	newState.Paused = m.simState.Paused
	newState.SimRunning = m.simState.SimRunning
	newState.Crashed = m.simState.Crashed
	newState.CrashReset = m.simState.CrashReset
	newState.Sound = m.simState.Sound

	if m.simState == newState {
		m.mu.Unlock()
		return
	}
	oldState := m.simState
	m.simState = newState
	m.mu.Unlock()

	if !oldState.Equal(newState) {
		// States are not equal
		// notify handlers of the change
		m.notifySimStateChange(oldState, newState)
	}
}
