//go:build windows
// +build windows

package handlers

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/manager/internal/instance"
)

// RegisterObjectAddedHandler adds an ObjectAdded handler and returns its unique ID
func RegisterObjectAddedHandler(mu *sync.RWMutex, handlers *[]instance.ObjectChangeHandlerEntry, handler interface{}, logger *slog.Logger) string {
	id := GenerateUUID()
	mu.Lock()
	*handlers = append(*handlers, instance.ObjectChangeHandlerEntry{ID: id, Fn: handler})
	mu.Unlock()
	if logger != nil {
		logger.Debug("[manager] Registered ObjectAdded handler", "id", id)
	}
	return id
}

// RemoveObjectAddedHandler removes an ObjectAdded handler by ID
func RemoveObjectAddedHandler(mu *sync.RWMutex, handlers *[]instance.ObjectChangeHandlerEntry, id string, logger *slog.Logger) error {
	mu.Lock()
	defer mu.Unlock()
	for i, e := range *handlers {
		if e.ID == id {
			*handlers = append((*handlers)[:i], (*handlers)[i+1:]...)
			if logger != nil {
				logger.Debug("[manager] Removed ObjectAdded handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("ObjectAdded handler not found: %s", id)
}

// RegisterObjectRemovedHandler adds an ObjectRemoved handler and returns its unique ID
func RegisterObjectRemovedHandler(mu *sync.RWMutex, handlers *[]instance.ObjectChangeHandlerEntry, handler interface{}, logger *slog.Logger) string {
	id := GenerateUUID()
	mu.Lock()
	*handlers = append(*handlers, instance.ObjectChangeHandlerEntry{ID: id, Fn: handler})
	mu.Unlock()
	if logger != nil {
		logger.Debug("[manager] Registered ObjectRemoved handler", "id", id)
	}
	return id
}

// RemoveObjectRemovedHandler removes an ObjectRemoved handler by ID
func RemoveObjectRemovedHandler(mu *sync.RWMutex, handlers *[]instance.ObjectChangeHandlerEntry, id string, logger *slog.Logger) error {
	mu.Lock()
	defer mu.Unlock()
	for i, e := range *handlers {
		if e.ID == id {
			*handlers = append((*handlers)[:i], (*handlers)[i+1:]...)
			if logger != nil {
				logger.Debug("[manager] Removed ObjectRemoved handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("ObjectRemoved handler not found: %s", id)
}
