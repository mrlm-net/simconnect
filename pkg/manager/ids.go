package manager

const (
	// Manager ID Allocation Strategy:
	// ==============================
	//
	// The manager uses high-number IDs near the end of the uint32 scale (999999900-999999999) to provide
	// maximum flexibility for user-defined requests. This strategy:
	//
	// RATIONALE:
	// - Reserves a dedicated, easily-identifiable range for internal manager operations
	// - Provides plenty of space (999,999,899 IDs) for user-defined request/definition IDs (1-999999899)
	// - Avoids collisions with typical application ID assignments (which often start from 1)
	// - Follows the principle of defensive ID allocation by using very high numbers
	// - Simplifies ID range validation and conflict detection
	//
	// USAGE GUIDELINES FOR END USERS:
	// - Use IDs from 1 to 999999899 for your own data definitions and requests
	// - NEVER use IDs in the 999999900-999999999 range (reserved for manager)
	// - Consider organizing your IDs in logical sub-ranges if managing multiple concurrent requests
	// - Example: Use 1000-1999 for aircraft data, 2000-2999 for environment data, etc.
	// - Use the IsValidUserID() function to validate your chosen IDs before use

	// Camera System IDs - Used for internal camera state polling
	// These IDs manage the camera state (position, type) data that is continuously requested
	// from the simulator and used to update the manager's SimState.
	CameraDefinitionID uint32 = 999999900 // Definition ID for camera state data structure
	CameraRequestID    uint32 = 999999901 // Request ID for periodic camera state data polling

	// Event System IDs - Used for internal system event subscriptions
	// These IDs are used for request registry tracking (manager reserved range).
	// The actual SimConnect subscription uses standard event IDs (1000, 1001).
	PauseEventID uint32 = 999999998 // Manager ID for tracking pause event subscription
	SimEventID   uint32 = 999999997 // Manager ID for tracking sim event subscription
	// Crash and Sound Events
	CrashedEventID    uint32 = 999999991 // Manager ID for tracking 'Crashed' system event
	CrashResetEventID uint32 = 999999990 // Manager ID for tracking 'Crash Reset' system event
	SoundEventID      uint32 = 999999989 // Manager ID for tracking 'Sound' system event
	ViewEventID                    uint32 = 999999988 // Manager ID for tracking 'View' system event
	FlightPlanDeactivatedEventID   uint32 = 999999987 // Manager ID for tracking 'FlightPlanDeactivated' system event

	// Additional Manager Event IDs
	// These IDs map internal manager subscriptions for SimConnect system events.
	FlightLoadedEventID        uint32 = 999999996 // Flight file loaded event
	AircraftLoadedEventID      uint32 = 999999995 // Aircraft (AIR) loaded/changed event
	ObjectAddedEventID         uint32 = 999999994 // Object (AI/traffic) added
	ObjectRemovedEventID       uint32 = 999999993 // Object (AI/traffic) removed
	FlightPlanActivatedEventID uint32 = 999999992 // Flight plan activated
	// Position change event removed

	// ID Range Documentation:
	// User-Available Range: 1 - 999999899 (999,999,899 IDs available for user requests)
	// Manager Reserved Range: 999999900 - 999999999 (100 IDs reserved for manager operations)
)

// IDRange defines the boundaries for ID allocation
var IDRange = struct {
	UserMin    uint32
	UserMax    uint32
	ManagerMin uint32
	ManagerMax uint32
}{
	UserMin:    1,
	UserMax:    999999899,
	ManagerMin: 999999900,
	ManagerMax: 999999999,
}

// IsManagerID checks if an ID is reserved for manager use
func IsManagerID(id uint32) bool {
	return id >= IDRange.ManagerMin && id <= IDRange.ManagerMax
}

// IsValidUserID checks if an ID is within the allowed user range
func IsValidUserID(id uint32) bool {
	return id >= IDRange.UserMin && id <= IDRange.UserMax
}
