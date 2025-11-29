//go:build windows
// +build windows

package simconnect

import (
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
