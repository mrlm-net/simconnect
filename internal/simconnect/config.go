//go:build windows
// +build windows

package simconnect

import "context"

type Config struct {
	BufferSize int
	Context    context.Context
	DLLPath    string
}
