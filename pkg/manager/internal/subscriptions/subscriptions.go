//go:build windows
// +build windows

package subscriptions

import (
	"github.com/mrlm-net/simconnect/pkg/manager/internal/handlers"
)

// Default buffer size for subscriptions when invalid value is provided
const DefaultBufferSize = 16

// GenerateID generates a UUID for a subscription ID
// If id is empty, generates a new UUID
func GenerateID(id string) string {
	if id == "" {
		return handlers.GenerateUUID()
	}
	return id
}

// ValidateBufferSize ensures buffer size is positive, returns DefaultBufferSize if <= 0
func ValidateBufferSize(bufferSize int) int {
	if bufferSize <= 0 {
		return DefaultBufferSize
	}
	return bufferSize
}
