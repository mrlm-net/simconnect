//go:build windows
// +build windows

package simconnect

import (
	"sync"
	"unsafe"

	"github.com/mrlm-net/simconnect/internal/dll"
)

func New(name string, config *Config) *SimConnect {
	return &SimConnect{
		0, dll.New(config.DLLPath), name, sync.RWMutex{},
	}
}

type SimConnect struct {
	// Add fields as necessary
	connection uintptr
	library    *dll.DLL
	name       string
	sync       sync.RWMutex
}

func (sc *SimConnect) getConnection() uintptr {
	sc.sync.RLock()
	defer sc.sync.RUnlock()
	return uintptr(sc.connection)
}

func (sc *SimConnect) getConnectionPtr() uintptr {
	sc.sync.RLock()
	defer sc.sync.RUnlock()
	return uintptr(unsafe.Pointer(&sc.connection))
}
