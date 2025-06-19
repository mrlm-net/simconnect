//go:build windows
// +build windows

package client

import "syscall"

type Engine struct {
	dll    *syscall.LazyDLL // The DLL handle for the SimConnect.dll library
	handle uintptr          // The handle to the SimConnect connection
	name   string           // The name of the SimConnect client
}
