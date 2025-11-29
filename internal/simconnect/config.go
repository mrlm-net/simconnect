//go:build windows
// +build windows

package simconnect

type Config struct {
	BufferSize int
	DLLPath    string
}
