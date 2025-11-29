//go:build windows
// +build windows

package simconnect

import (
	"unsafe"

	"github.com/mrlm-net/simconnect/internal/dll"
)

func New(config *Config) *SimConnect {
	return &SimConnect{
		dll.New(config.DLLPath), 0,
	}
}

type SimConnect struct {
	// Add fields as necessary
	library    *dll.DLL
	connection uintptr
}

func (sc *SimConnect) getConnection() uintptr {
	return uintptr(sc.connection)
}

func (sc *SimConnect) getConnectionPtr() uintptr {
	return uintptr(unsafe.Pointer(&sc.connection))
}
