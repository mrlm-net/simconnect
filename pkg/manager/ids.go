package manager

const (
	// Manager ID Allocation Strategy:
	// ==============================
	//
	// The manager uses high-number IDs at the end of the uint32 scale (999000-999999) to provide
	// maximum flexibility for user-defined requests. This strategy:
	//
	// RATIONALE:
	// - Reserves a dedicated, easily-identifiable range for internal manager operations
	// - Provides plenty of space (998,999 IDs) for user-defined request/definition IDs (1-998999)
	// - Avoids collisions with typical application ID assignments (which often start from 1)
	// - Follows the principle of defensive ID allocation by using high numbers
	// - Simplifies ID range validation and conflict detection
	//
	// USAGE GUIDELINES FOR END USERS:
	// - Use IDs from 1 to 998999 for your own data definitions and requests
	// - NEVER use IDs in the 999000-999999 range (reserved for manager)
	// - Consider organizing your IDs in logical sub-ranges if managing multiple concurrent requests
	// - Example: Use 1000-1999 for aircraft data, 2000-2999 for environment data, etc.
	// - Use the IsValidUserID() function to validate your chosen IDs before use
	//
	// INTERNAL MANAGER ID ALLOCATION:
	// The manager allocates IDs as follows:
	// - 999000-999099: Data Definitions (currently using 999000 for camera)
	// - 999100-999199: Event Subscriptions (currently using 999100 for pause)
	// - 999900-999999: System & Heartbeat (currently using 999999 for heartbeat)

	// Camera System IDs - Used for internal camera state polling
	// These IDs manage the camera state (position, type) data that is continuously requested
	// from the simulator and used to update the manager's SimState.
	CameraDefinitionID uint32 = 999000 // Definition ID for camera state data structure
	CameraRequestID    uint32 = 999001 // Request ID for periodic camera state data polling

	// Event System IDs - Used for internal system event subscriptions
	// These IDs are used for request registry tracking (manager reserved range).
	// The actual SimConnect subscription uses standard event IDs (1000, 1001).
	PauseEventID uint32 = 999100 // Manager ID for tracking pause event subscription
	SimEventID   uint32 = 999101 // Manager ID for tracking sim event subscription

	// Status and Heartbeat IDs - Used for internal status monitoring
	// These IDs are used for periodic status checks and heartbeat operations.
	HeartbeatEventID uint32 = 999999 // Event ID for heartbeat/status polling

	// ID Range Documentation:
	// User-Available Range: 1 - 998999 (998,999 IDs available for user requests)
	// Manager Reserved Range: 999000 - 999999 (1,000 IDs reserved for manager operations)
)

// IDRange defines the boundaries for ID allocation
var IDRange = struct {
	UserMin    uint32
	UserMax    uint32
	ManagerMin uint32
	ManagerMax uint32
}{
	UserMin:    1,
	UserMax:    998999,
	ManagerMin: 999000,
	ManagerMax: 999999,
}

// IsManagerID checks if an ID is reserved for manager use
func IsManagerID(id uint32) bool {
	return id >= IDRange.ManagerMin && id <= IDRange.ManagerMax
}

// IsValidUserID checks if an ID is within the allowed user range
func IsValidUserID(id uint32) bool {
	return id >= IDRange.UserMin && id <= IDRange.UserMax
}
