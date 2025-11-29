//go:build windows
// +build windows

package engine

import "github.com/mrlm-net/simconnect/internal/simconnect"

const (
	DEFAULT_BUFFER_SIZE = 256
	DEFAULT_DLL_PATH    = "C:/MSFS 2024 SDK/SimConnect SDK/lib/SimConnect.dll"
)

type Option func(*Config)

type Config = simconnect.Config

func WithBufferSize(size int) Option {
	return func(c *Config) {
		c.BufferSize = size
	}
}

func WithDLLPath(path string) Option {
	return func(c *Config) {
		c.DLLPath = path
	}
}

func defaultConfig() *Config {
	return &Config{
		BufferSize: DEFAULT_BUFFER_SIZE,
		DLLPath:    DEFAULT_DLL_PATH,
	}
}
