//go:build windows
// +build windows

package client

import (
	"context"
	"syscall"
)

const (
	DLL_DEFAULT_PATH = "C:/MSFS 2024 SDK/SimConnect SDK/lib/SimConnect.dll"
	// Default buffer size for the message stream channel
	DEFAULT_STREAM_BUFFER_SIZE = 100
)

func New(name string) *Engine {
	client := &Engine{
		ctx:    context.Background(),
		dll:    syscall.NewLazyDLL(DLL_DEFAULT_PATH),
		handle: 0, // Initially no connection
		name:   name,
		queue:  make(chan any, DEFAULT_STREAM_BUFFER_SIZE), // Buffered channel for message queueing
	}
	if err := client.bootstrap(); err != nil {
		return nil
	}
	return client
}
