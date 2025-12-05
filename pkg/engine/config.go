//go:build windows
// +build windows

package engine

import (
	"context"
	"os"

	"log/slog"

	"github.com/mrlm-net/simconnect/internal/simconnect"
)

const (
	DEFAULT_BUFFER_SIZE = 256
	DEFAULT_DLL_PATH    = "C:/MSFS 2024 SDK/SimConnect SDK/lib/SimConnect.dll"
)

type Option func(*Config)

type Config struct {
	simconnect.Config
	Heartbeat string
	Logger    *slog.Logger
}

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

func WithContext(ctx context.Context) Option {
	return func(c *Config) {
		c.Context = ctx
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(c *Config) {
		c.Logger = logger
	}
}

func WithHeartbeat(frequency string) Option {
	return func(c *Config) {
		// TODO add validation and enum?
		c.Heartbeat = frequency
	}
}

func defaultConfig() *Config {
	return &Config{
		Config: simconnect.Config{
			BufferSize: DEFAULT_BUFFER_SIZE,
			Context:    context.Background(),
			DLLPath:    DEFAULT_DLL_PATH,
		},
		Heartbeat: "6Hz",
		Logger: slog.New(
			slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
				Level: slog.LevelWarn, // Set minimum log level to WARN
			}),
		),
	}
}
