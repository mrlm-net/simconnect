//go:build windows
// +build windows

package client

import (
	"context"
	"syscall"
)

type Engine struct {
	ctx    context.Context
	dll    *syscall.LazyDLL   // The DLL handle for the SimConnect.dll library
	handle uintptr            // The handle to the SimConnect connection
	name   string             // The name of the SimConnect client
	queue  chan ParsedMessage // Channel for parsed message queueing
}
