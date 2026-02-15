//go:build windows
// +build windows

package instance

// Handler entry types store handlers with identifiers for removal.
// All entry types are exported so they can be accessed by other internal packages.
// The Fn field uses interface{} to avoid import cycles with pkg/manager.
// Callers must type-assert to the correct handler function type.

// StateHandlerEntry stores a connection state change handler with an identifier
type StateHandlerEntry struct {
	ID string
	Fn interface{} // ConnectionStateChangeHandler
}

// SimStateHandlerEntry stores a simulator state change handler with an identifier
type SimStateHandlerEntry struct {
	ID string
	Fn interface{} // SimStateChangeHandler
}

// MessageHandlerEntry stores a message handler with an identifier
type MessageHandlerEntry struct {
	ID string
	Fn interface{} // MessageHandler
}

// FlightLoadedHandlerEntry stores a FlightLoaded handler with an identifier
type FlightLoadedHandlerEntry struct {
	ID string
	Fn interface{} // FlightLoadedHandler
}

// ObjectChangeHandlerEntry stores an ObjectAdded/ObjectRemoved handler with an identifier
type ObjectChangeHandlerEntry struct {
	ID string
	Fn interface{} // ObjectChangeHandler
}

// CrashedHandlerEntry stores a Crashed handler with an identifier
type CrashedHandlerEntry struct {
	ID string
	Fn interface{} // CrashedHandler
}

// CrashResetHandlerEntry stores a CrashReset handler with an identifier
type CrashResetHandlerEntry struct {
	ID string
	Fn interface{} // CrashResetHandler
}

// SoundEventHandlerEntry stores a Sound handler with an identifier
type SoundEventHandlerEntry struct {
	ID string
	Fn interface{} // SoundEventHandler
}

// ViewHandlerEntry stores a View handler with an identifier
type ViewHandlerEntry struct {
	ID string
	Fn interface{} // ViewHandler
}

// FlightPlanDeactivatedHandlerEntry stores a FlightPlanDeactivated handler with an identifier
type FlightPlanDeactivatedHandlerEntry struct {
	ID string
	Fn interface{} // FlightPlanDeactivatedHandler
}

// PauseHandlerEntry stores a Pause handler with an identifier
type PauseHandlerEntry struct {
	ID string
	Fn interface{} // PauseHandler
}

// SimRunningHandlerEntry stores a SimRunning handler with an identifier
type SimRunningHandlerEntry struct {
	ID string
	Fn interface{} // SimRunningHandler
}

// OpenHandlerEntry stores a connection open handler with an identifier
type OpenHandlerEntry struct {
	ID string
	Fn interface{} // ConnectionOpenHandler
}

// QuitHandlerEntry stores a connection quit handler with an identifier
type QuitHandlerEntry struct {
	ID string
	Fn interface{} // ConnectionQuitHandler
}

// CustomSystemEventHandlerEntry stores a custom system event handler with an identifier
type CustomSystemEventHandlerEntry struct {
	ID string
	Fn interface{} // CustomSystemEventHandler
}

// CustomSystemEvent tracks a user-registered custom system event
type CustomSystemEvent struct {
	Name     string
	ID       uint32
	Handlers []CustomSystemEventHandlerEntry
}
