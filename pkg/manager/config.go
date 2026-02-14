//go:build windows
// +build windows

package manager

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

const (
	DEFAULT_RETRY_INTERVAL     = 15 * time.Second // Delay between connection attempts
	DEFAULT_RECONNECT_DELAY    = 30 * time.Second // Delay before reconnecting after disconnect
	DEFAULT_CONNECTION_TIMEOUT = 30 * time.Second // Timeout for each connection attempt
	DEFAULT_SHUTDOWN_TIMEOUT   = 10 * time.Second // Timeout for graceful shutdown
	DEFAULT_MAX_RETRIES        = 0                // 0 = unlimited retries
	DEFAULT_AUTO_RECONNECT     = true
)

// Config holds the configuration for the Manager
type Config struct {
	// Context for the manager lifecycle
	Context context.Context

	// Logger for manager operations
	Logger *slog.Logger
	// LogLevel controls the minimum level used when the package constructs
	// a default logger. If `Logger` is provided via `WithLogger`, that
	// logger takes precedence.
	LogLevel slog.Level

	// Connection retry settings
	RetryInterval     time.Duration // Fixed delay between connection attempts
	ConnectionTimeout time.Duration // Timeout for each connection attempt
	ReconnectDelay    time.Duration // Delay before reconnecting after disconnect
	ShutdownTimeout   time.Duration // Timeout for graceful shutdown of subscriptions
	MaxRetries        int           // Maximum number of connection retries (0 = unlimited)

	// Behavior settings
	AutoReconnect bool // Whether to automatically reconnect on disconnect

	// SimStatePeriod controls how often the manager requests SimState data from SimConnect.
	// Default is SIMCONNECT_PERIOD_SIM_FRAME (every simulation frame).
	// Use SIMCONNECT_PERIOD_SECOND for lower-frequency updates (1Hz) to reduce CPU usage.
	SimStatePeriod types.SIMCONNECT_PERIOD

	// Engine options to pass through
	EngineOptions []engine.Option
}

// Option is a function that configures the manager
type Option func(*Config)

// WithContext sets the context for the manager
func WithContext(ctx context.Context) Option {
	return func(c *Config) {
		c.Context = ctx
	}
}

// WithLogger sets the logger for the manager
func WithLogger(logger *slog.Logger) Option {
	return func(c *Config) {
		c.Logger = logger
	}
}

// WithLogLevel sets the minimum level for the manager's default logger.
// If the caller provides a custom logger via WithLogger, that logger wins.
func WithLogLevel(level slog.Level) Option {
	return func(c *Config) {
		c.LogLevel = level
	}
}

// WithLogLevelFromString parses a textual level and sets the manager's level.
func WithLogLevelFromString(level string) Option {
	return func(c *Config) {
		c.LogLevel = parseLogLevel(level)
	}
}

// WithRetryInterval sets the fixed delay between connection attempts
func WithRetryInterval(d time.Duration) Option {
	return func(c *Config) {
		c.RetryInterval = d
	}
}

// WithConnectionTimeout sets the timeout for each connection attempt
func WithConnectionTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.ConnectionTimeout = d
	}
}

// WithReconnectDelay sets the delay before reconnecting after disconnect
func WithReconnectDelay(d time.Duration) Option {
	return func(c *Config) {
		c.ReconnectDelay = d
	}
}

// WithShutdownTimeout sets the timeout for graceful shutdown of subscriptions
func WithShutdownTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.ShutdownTimeout = d
	}
}

// WithMaxRetries sets the maximum number of connection retries (0 = unlimited)
func WithMaxRetries(n int) Option {
	return func(c *Config) {
		c.MaxRetries = n
	}
}

// WithAutoReconnect enables or disables automatic reconnection
func WithAutoReconnect(enabled bool) Option {
	return func(c *Config) {
		c.AutoReconnect = enabled
	}
}

// WithSimStatePeriod sets the update frequency for internal SimState data requests.
// Controls how often the manager polls simulator state variables (camera, position, weather, etc.).
//
// Supported values:
//   - types.SIMCONNECT_PERIOD_SIM_FRAME — every simulation frame (~30-60Hz, default)
//   - types.SIMCONNECT_PERIOD_VISUAL_FRAME — every visual frame
//   - types.SIMCONNECT_PERIOD_SECOND — once per second (1Hz, lower CPU usage)
//   - types.SIMCONNECT_PERIOD_ONCE — single snapshot (no periodic updates)
//   - types.SIMCONNECT_PERIOD_NEVER — disable SimState data requests entirely
func WithSimStatePeriod(period types.SIMCONNECT_PERIOD) Option {
	return func(c *Config) {
		c.SimStatePeriod = period
	}
}

// WithEngineOptions passes options through to the underlying engine.
// Note: Context and Logger options passed here will be ignored as the manager
// controls these settings. Use WithContext and WithLogger on the manager instead.
func WithEngineOptions(opts ...engine.Option) Option {
	return func(c *Config) {
		c.EngineOptions = append(c.EngineOptions, opts...)
	}
}

// WithBufferSize sets the buffer size for the underlying engine.
// This is a convenience wrapper for engine.WithBufferSize.
// Default is 256.
func WithBufferSize(size int) Option {
	return func(c *Config) {
		c.EngineOptions = append(c.EngineOptions, engine.WithBufferSize(size))
	}
}

// WithDLLPath sets the path to the SimConnect DLL for the underlying engine.
// This is a convenience wrapper for engine.WithDLLPath.
// Default is "C:/MSFS 2024 SDK/SimConnect SDK/lib/SimConnect.dll".
func WithDLLPath(path string) Option {
	return func(c *Config) {
		c.EngineOptions = append(c.EngineOptions, engine.WithDLLPath(path))
	}
}

// WithHeartbeat sets the heartbeat frequency for the underlying engine.
// This is a convenience wrapper for engine.WithHeartbeat.
// Valid values: "1Hz", "6Hz", etc. Default is "6Hz".
func WithHeartbeat(frequency engine.HeartbeatFrequency) Option {
	return func(c *Config) {
		c.EngineOptions = append(c.EngineOptions, engine.WithHeartbeat(frequency))
	}
}

// WithAutoDetect enables automatic detection of SimConnect.dll by searching
// environment variables and common SDK installation paths.
// This is a convenience wrapper for engine.WithAutoDetect.
func WithAutoDetect() Option {
	return func(c *Config) {
		c.EngineOptions = append(c.EngineOptions, engine.WithAutoDetect())
	}
}

// defaultConfig returns a Config with default values
func defaultConfig() *Config {
	return &Config{
		Context: context.Background(),
		// Defer creating a concrete logger until constructor time so that
		// WithLogLevel and WithLogger options can be applied in any order.
		Logger:            nil,
		LogLevel:          slog.LevelInfo,
		RetryInterval:     DEFAULT_RETRY_INTERVAL,
		ConnectionTimeout: DEFAULT_CONNECTION_TIMEOUT,
		ReconnectDelay:    DEFAULT_RECONNECT_DELAY,
		ShutdownTimeout:   DEFAULT_SHUTDOWN_TIMEOUT,
		MaxRetries:        DEFAULT_MAX_RETRIES,
		AutoReconnect:     DEFAULT_AUTO_RECONNECT,
		SimStatePeriod:    types.SIMCONNECT_PERIOD_SIM_FRAME,
		EngineOptions:     []engine.Option{},
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
