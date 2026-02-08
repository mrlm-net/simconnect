//go:build windows
// +build windows

package engine

import (
	"context"
	"strings"

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
	Heartbeat HeartbeatFrequency
	Logger    *slog.Logger
	// LogLevel controls the minimum level used when the package constructs
	// a default logger. If `Logger` is provided via `WithLogger`, that
	// logger takes precedence.
	LogLevel slog.Level
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

// WithAutoDetect enables automatic detection of SimConnect.dll by searching
// environment variables and common SDK installation paths.
// A user-specified path (via WithDLLPath) takes precedence over auto-detection.
func WithAutoDetect() Option {
	return func(c *Config) {
		c.AutoDetect = true
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

// WithLogLevel sets the minimum log level for the default logger constructed
// by the engine when the caller does not provide a custom logger.
func WithLogLevel(level slog.Level) Option {
	return func(c *Config) {
		c.LogLevel = level
	}
}

// WithLogLevelFromString parses a textual level (e.g. "debug", "info") and
// sets the effective log level on the config.
func WithLogLevelFromString(level string) Option {
	return func(c *Config) {
		c.LogLevel = parseLogLevel(level)
	}
}

func WithHeartbeat(frequency HeartbeatFrequency) Option {
	return func(c *Config) {
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
		Heartbeat: HEARTBEAT_6HZ,
		// Defer creating the concrete logger until constructor time so
		// options that set `LogLevel` or `Logger` are applied in the
		// expected order. Default to INFO when no option is provided.
		Logger:   nil,
		LogLevel: slog.LevelInfo,
	}
}

// parseLogLevel maps common textual level names to slog.Level. Unknown
// values fall back to INFO.
func parseLogLevel(v string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "debug", "d":
		return slog.LevelDebug
	case "info", "i":
		return slog.LevelInfo
	case "warn", "warning", "w":
		return slog.LevelWarn
	case "error", "err", "e":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
