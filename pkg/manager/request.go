package manager

import (
	"sync"
	"time"
)

// RequestType indicates the category of a SimConnect request
// This is used to classify and track different types of SimConnect operations
// in the request registry for debugging, monitoring, and correlation purposes.
type RequestType int

const (
	RequestTypeDataDefinition RequestType = iota // AddToDataDefinition, ClearDataDefinition
	RequestTypeDataRequest                       // RequestDataOnSimObject, RequestDataOnSimObjectType
	RequestTypeDataSet                           // SetDataOnSimObject
	RequestTypeEvent                             // SubscribeToSystemEvent, UnsubscribeFromSystemEvent
	RequestTypeObject                            // RequestSimulatorState, RequestFacilities, etc.
	RequestTypeCustom                            // User-defined or other request types
)

// RequestInfo tracks metadata about an active SimConnect request
// This structure maintains information about requests made to the simulator,
// enabling correlation of responses with the original requests and providing
// context for debugging and request tracking.
//
// Example usage:
//
//	registry := NewRequestRegistry()
//	info := registry.Register(1000, RequestTypeDataDefinition, "My Camera Definition")
//	info.Context["purpose"] = "tracking_aircraft"
//	info.Context["user_callback"] = myCallbackFunc
type RequestInfo struct {
	// ID is the SimConnect Definition or Request ID used in the simulator call
	ID uint32

	// Type classifies the kind of request (data definition, event subscription, etc.)
	Type RequestType

	// Description is a human-readable label for the request (e.g., "Camera State Definition")
	Description string

	// Timestamp records when the request was created/registered
	Timestamp time.Time

	// Context stores arbitrary user-provided metadata for the request
	// Users can store any additional information needed to correlate responses
	// Examples: user callbacks, request parameters, correlation IDs, etc.
	Context map[string]interface{}

	// userHandler stores an optional reference to the user's callback or handler
	userHandler interface{}
}

// RequestRegistry maintains a mapping of active requests for correlation with responses
// It provides thread-safe storage and retrieval of request metadata, enabling the manager
// to track which requests are outstanding and correlate incoming responses.
//
// The registry is primarily used internally by the manager for:
// - Tracking active data requests and subscriptions
// - Validating responses against known requests
// - Providing diagnostic information about outstanding requests
// - Cleaning up resources when requests are completed or connection closes
//
// Thread Safety: All methods are protected by mutex locks (RWMutex).
type RequestRegistry struct {
	mu       sync.RWMutex
	requests map[uint32]*RequestInfo
}

// NewRequestRegistry creates and initializes a new request registry
// Returns an empty registry ready to track SimConnect requests.
func NewRequestRegistry() *RequestRegistry {
	return &RequestRegistry{
		requests: make(map[uint32]*RequestInfo),
	}
}

// Register adds a new request to the registry with the specified metadata.
// This should be called when making a SimConnect request (e.g., AddToDataDefinition, RequestDataOnSimObject).
//
// Parameters:
//   - id: The SimConnect Definition or Request ID being used
//   - reqType: The category of request (DataDefinition, DataRequest, Event, etc.)
//   - description: A human-readable description (e.g., "Aircraft Position Data Request")
//
// Returns a pointer to the RequestInfo, allowing the caller to add custom context data:
//
//	info := registry.Register(1000, RequestTypeDataDefinition, "Position Definition")
//	info.Context["aircraft"] = "user_aircraft"
func (r *RequestRegistry) Register(id uint32, reqType RequestType, description string) *RequestInfo {
	r.mu.Lock()
	defer r.mu.Unlock()

	info := &RequestInfo{
		ID:          id,
		Type:        reqType,
		Description: description,
		Timestamp:   time.Now(),
		Context:     make(map[string]interface{}),
	}
	r.requests[id] = info
	return info
}

// Get retrieves request information by ID.
// Returns the RequestInfo and a boolean indicating whether the request exists in the registry.
// This is useful for validating responses against known requests.
func (r *RequestRegistry) Get(id uint32) (*RequestInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, exists := r.requests[id]
	return info, exists
}

// Unregister removes a request from the registry.
// Should be called when a request completes or is cancelled.
// Returns true if the request was found and removed, false if it wasn't in the registry.
func (r *RequestRegistry) Unregister(id uint32) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.requests[id]; exists {
		delete(r.requests, id)
		return true
	}
	return false
}

// GetAll returns a copy of all registered requests.
// Useful for diagnostics and monitoring all outstanding requests.
// The returned map is a snapshot; modifications won't affect the registry.
func (r *RequestRegistry) GetAll() map[uint32]*RequestInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[uint32]*RequestInfo, len(r.requests))
	for k, v := range r.requests {
		result[k] = v
	}
	return result
}

// Clear removes all requests from the registry.
// Called internally when disconnecting to ensure a clean state for the next connection.
func (r *RequestRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	clear(r.requests)
}

// Count returns the number of currently registered requests.
// Useful for monitoring the number of outstanding requests.
func (r *RequestRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.requests)
}
