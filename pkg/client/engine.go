//go:build windows
// +build windows

package client

import (
	"context"
	"sync"
	"syscall"
)

type Engine struct {
	ctx       context.Context
	cancel    context.CancelFunc // Function to cancel the context
	dll       *syscall.LazyDLL   // The DLL handle for the SimConnect.dll library
	handle    uintptr            // The handle to the SimConnect connection
	name      string             // The name of the SimConnect client
	queue     chan ParsedMessage // Channel for parsed message queueing
	wg        sync.WaitGroup     // WaitGroup to coordinate goroutines
	once      sync.Once          // Ensure cleanup happens only once
	isClosing bool               // Flag to indicate if we're shutting down
	mu        sync.RWMutex       // Mutex to protect isClosing flag
}
