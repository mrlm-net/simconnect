//go:build windows
// +build windows

package handlers

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/manager/internal/instance"
)

// GenerateUUID generates a simple UUID v4 with optimized allocation using math/rand/v2
func GenerateUUID() string {
	var b [16]byte
	binary.LittleEndian.PutUint64(b[0:8], rand.Uint64())
	binary.LittleEndian.PutUint64(b[8:16], rand.Uint64())
	// Set version (4) and variant bits
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	// Use pre-allocated buffer for hex encoding
	var buf [36]byte
	hex.Encode(buf[0:8], b[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], b[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], b[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], b[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], b[10:16])
	return string(buf[:])
}

// RegisterStateHandler adds a connection state change handler and returns its unique ID
func RegisterStateHandler(mu *sync.RWMutex, handlers *[]instance.StateHandlerEntry, handler interface{}, logger *slog.Logger) string {
	id := GenerateUUID()
	mu.Lock()
	*handlers = append(*handlers, instance.StateHandlerEntry{ID: id, Fn: handler})
	mu.Unlock()
	if logger != nil {
		logger.Debug("[manager] Registered state handler", "id", id)
	}
	return id
}

// RemoveStateHandler removes a connection state change handler by ID
func RemoveStateHandler(mu *sync.RWMutex, handlers *[]instance.StateHandlerEntry, id string, logger *slog.Logger) error {
	mu.Lock()
	defer mu.Unlock()
	for i, e := range *handlers {
		if e.ID == id {
			*handlers = append((*handlers)[:i], (*handlers)[i+1:]...)
			if logger != nil {
				logger.Debug("[manager] Removed state handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("state handler not found: %s", id)
}

// RegisterMessageHandler adds a message handler and returns its unique ID
func RegisterMessageHandler(mu *sync.RWMutex, handlers *[]instance.MessageHandlerEntry, handler interface{}, logger *slog.Logger) string {
	id := GenerateUUID()
	mu.Lock()
	*handlers = append(*handlers, instance.MessageHandlerEntry{ID: id, Fn: handler})
	mu.Unlock()
	if logger != nil {
		logger.Debug("[manager] Registered message handler", "id", id)
	}
	return id
}

// RemoveMessageHandler removes a message handler by ID
func RemoveMessageHandler(mu *sync.RWMutex, handlers *[]instance.MessageHandlerEntry, id string, logger *slog.Logger) error {
	mu.Lock()
	defer mu.Unlock()
	for i, e := range *handlers {
		if e.ID == id {
			*handlers = append((*handlers)[:i], (*handlers)[i+1:]...)
			if logger != nil {
				logger.Debug("[manager] Removed message handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("message handler not found: %s", id)
}

// RegisterOpenHandler adds a connection open handler and returns its unique ID
func RegisterOpenHandler(mu *sync.RWMutex, handlers *[]instance.OpenHandlerEntry, handler interface{}, logger *slog.Logger) string {
	id := GenerateUUID()
	mu.Lock()
	*handlers = append(*handlers, instance.OpenHandlerEntry{ID: id, Fn: handler})
	mu.Unlock()
	if logger != nil {
		logger.Debug("[manager] Registered open handler", "id", id)
	}
	return id
}

// RemoveOpenHandler removes a connection open handler by ID
func RemoveOpenHandler(mu *sync.RWMutex, handlers *[]instance.OpenHandlerEntry, id string, logger *slog.Logger) error {
	mu.Lock()
	defer mu.Unlock()
	for i, e := range *handlers {
		if e.ID == id {
			*handlers = append((*handlers)[:i], (*handlers)[i+1:]...)
			if logger != nil {
				logger.Debug("[manager] Removed open handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("open handler not found: %s", id)
}

// RegisterQuitHandler adds a connection quit handler and returns its unique ID
func RegisterQuitHandler(mu *sync.RWMutex, handlers *[]instance.QuitHandlerEntry, handler interface{}, logger *slog.Logger) string {
	id := GenerateUUID()
	mu.Lock()
	*handlers = append(*handlers, instance.QuitHandlerEntry{ID: id, Fn: handler})
	mu.Unlock()
	if logger != nil {
		logger.Debug("[manager] Registered quit handler", "id", id)
	}
	return id
}

// RemoveQuitHandler removes a connection quit handler by ID
func RemoveQuitHandler(mu *sync.RWMutex, handlers *[]instance.QuitHandlerEntry, id string, logger *slog.Logger) error {
	mu.Lock()
	defer mu.Unlock()
	for i, e := range *handlers {
		if e.ID == id {
			*handlers = append((*handlers)[:i], (*handlers)[i+1:]...)
			if logger != nil {
				logger.Debug("[manager] Removed quit handler", "id", id)
			}
			return nil
		}
	}
	return fmt.Errorf("quit handler not found: %s", id)
}
