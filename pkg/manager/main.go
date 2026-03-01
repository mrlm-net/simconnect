//go:build windows

package manager

import (
	"context"
	"log/slog"
	"os"
	"reflect"
	"runtime"
	"strings"

	"github.com/mrlm-net/simconnect/pkg/manager/internal/instance"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// defaultSimState returns a new SimState with all fields initialized to their default/uninitialized values.
// This centralizes the initialization to avoid duplication and ensure consistency.
func defaultSimState() SimState {
	return SimState{
		Camera:                   CameraStateUninitialized,
		Substate:                 CameraSubstateUninitialized,
		Paused:                   false,
		SimRunning:               false,
		SimulationRate:           0,
		SimulationTime:           0,
		LocalTime:                0,
		ZuluTime:                 0,
		IsInVR:                   false,
		IsUsingMotionControllers: false,
		IsUsingJoystickThrottle:  false,
		IsInRTC:                  false,
		IsAvatar:                 false,
		IsAircraft:               false,
		Crashed:                  false,
		CrashReset:               false,
		Sound:                    0,
		LocalDay:                 0,
		LocalMonth:               0,
		LocalYear:                0,
		ZuluDay:                  0,
		ZuluMonth:                0,
		ZuluYear:                 0,
		Realism:                  0,
		VisualModelRadius:        0,
		SimDisabled:              false,
		RealismCrashDetection:    false,
		RealismCrashWithOthers:   false,
		TrackIREnabled:           false,
		UserInputEnabled:         false,
		SimOnGround:              false,
		AmbientTemperature:       0,
		AmbientPressure:          0,
		AmbientWindVelocity:      0,
		AmbientWindDirection:     0,
		AmbientVisibility:        0,
		AmbientInCloud:           false,
		AmbientPrecipState:       0,
		BarometerPressure:        0,
		SeaLevelPressure:         0,
		GroundAltitude:           0,
		MagVar:                   0,
		SurfaceType:              0,
	}
}

// safeCallHandler executes a handler function with panic recovery.
// If the handler panics, the panic is logged and execution continues.
func safeCallHandler(logger *slog.Logger, name string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("[manager] Handler panic recovered", "handler", name, "panic", r)
		}
	}()
	fn()
}

// New creates a new Manager instance with the given application name and options
func New(name string, opts ...Option) Manager {
	config := defaultConfig()
	for _, opt := range opts {
		opt(config)
	}

	// If caller did not provide a logger, construct one using the configured
	// LogLevel so manager logs reflect the requested verbosity.
	if config.Logger == nil {
		config.Logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: config.LogLevel}))
	}

	// Validate EngineOptions once at construction time
	for _, eo := range config.EngineOptions {
		if eo != nil {
			pc := reflect.ValueOf(eo).Pointer()
			if fn := runtime.FuncForPC(pc); fn != nil {
				name := fn.Name()
				if strings.Contains(name, "WithContext") {
					config.Logger.Warn("[manager] EngineOptions contains WithContext, will be overridden by manager context")
				}
				if strings.Contains(name, "WithLogger") {
					config.Logger.Warn("[manager] EngineOptions contains WithLogger, will be overridden by manager logger")
				}
			}
		}
	}

	ctx, cancel := context.WithCancel(config.Context)

	return &Instance{
		name:                         name,
		config:                       config,
		ctx:                          ctx,
		cancel:                       cancel,
		logger:                       config.Logger,
		state:                        StateDisconnected,
		stateHandlers:                []instance.StateHandlerEntry{},
		messageHandlers:              []instance.MessageHandlerEntry{},
		openHandlers:                 []instance.OpenHandlerEntry{},
		quitHandlers:                 []instance.QuitHandlerEntry{},
		subscriptions:                make(map[string]*subscription),
		connectionStateSubscriptions: make(map[string]*connectionStateSubscription),
		openSubscriptions:            make(map[string]*connectionOpenSubscription),
		quitSubscriptions:            make(map[string]*connectionQuitSubscription),
		simState:                     defaultSimState(),
		simStateHandlers:             []instance.SimStateHandlerEntry{},
		simStateSubscriptions:        make(map[string]*simStateSubscription),
		cameraDefinitionID:           CameraDefinitionID,
		cameraRequestID:              CameraRequestID,
		pauseEventID:                 PauseEventID,
		simEventID:                   SimEventID,
		flightLoadedEventID:          FlightLoadedEventID,
		aircraftLoadedEventID:        AircraftLoadedEventID,
		objectAddedEventID:           ObjectAddedEventID,
		objectRemovedEventID:         ObjectRemovedEventID,
		flightPlanActivatedEventID:   FlightPlanActivatedEventID,
		crashedEventID:               CrashedEventID,
		crashResetEventID:            CrashResetEventID,
		soundEventID:                 SoundEventID,
		crashedHandlers:              []instance.CrashedHandlerEntry{},
		crashResetHandlers:           []instance.CrashResetHandlerEntry{},
		soundEventHandlers:           []instance.SoundEventHandlerEntry{},
		viewEventID:                    ViewEventID,
		flightPlanDeactivatedEventID:   FlightPlanDeactivatedEventID,
		viewHandlers:                    []instance.ViewHandlerEntry{},
		flightPlanDeactivatedHandlers:   []instance.FlightPlanDeactivatedHandlerEntry{},
		pauseHandlers:          []instance.PauseHandlerEntry{},
		simRunningHandlers:     []instance.SimRunningHandlerEntry{},
		customSystemEvents:     make(map[string]*instance.CustomSystemEvent),
		customEventIDAlloc:     CustomEventIDMin,
		requestRegistry:        NewRequestRegistry(),
		fleet:                  traffic.NewFleet(nil),
	}
}
