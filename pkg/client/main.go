//go:build windows
// +build windows

package client

import (
	"context"
	"fmt"
	"syscall"
)

const (
	DLL_DEFAULT_PATH = "C:/MSFS 2024 SDK/SimConnect SDK/lib/SimConnect.dll"
	// Default buffer size for the message stream channel
	DEFAULT_STREAM_BUFFER_SIZE = 100
)

func New(name string) *Engine {
	return NewWithDLL(name, DLL_DEFAULT_PATH)
}

func NewWithDLL(name string, dllPath string) *Engine {
	ctx, cancel := context.WithCancel(context.Background())
	client := &Engine{
		ctx:    ctx,
		cancel: cancel,
		dll:    syscall.NewLazyDLL(dllPath),
		handle: 0, // Initially no connection
		name:   name,
		queue:  make(chan ParsedMessage, DEFAULT_STREAM_BUFFER_SIZE), // Buffered channel for parsed message queueing
	}
	if err := client.bootstrap(); err != nil {
		cancel() // Clean up context if bootstrap fails
		fmt.Println("Failed to bootstrap SimConnect client:", err)
		return nil
	}
	return client
}
