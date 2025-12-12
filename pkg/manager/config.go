//go:build windows
// +build windows

package manager

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/mrlm-net/simconnect/pkg/engine"
)

const (
	DEFAULT_RETRY_INTERVAL     = 30 * time.Second // Delay between connection attempts
	DEFAULT_CONNECTION_TIMEOUT = 15 * time.Second // Timeout for each connection attempt
	DEFAULT_RECONNECT_DELAY    = 5 * time.Second  // Delay before reconnecting after disconnect
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

	// Connection retry settings
	RetryInterval     time.Duration // Fixed delay between connection attempts
	ConnectionTimeout time.Duration // Timeout for each connection attempt
	ReconnectDelay    time.Duration // Delay before reconnecting after disconnect
	ShutdownTimeout   time.Duration // Timeout for graceful shutdown of subscriptions
	MaxRetries        int           // Maximum number of connection retries (0 = unlimited)

	// Behavior settings
	AutoReconnect bool // Whether to automatically reconnect on disconnect

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

// WithEngineOptions passes options through to the underlying engine
func WithEngineOptions(opts ...engine.Option) Option {
	return func(c *Config) {
		c.EngineOptions = append(c.EngineOptions, opts...)
	}
}

// defaultConfig returns a Config with default values
func defaultConfig() *Config {
	return &Config{
		Context:           context.Background(),
		Logger:            slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})),
		RetryInterval:     DEFAULT_RETRY_INTERVAL,
		ConnectionTimeout: DEFAULT_CONNECTION_TIMEOUT,
		ReconnectDelay:    DEFAULT_RECONNECT_DELAY,
		ShutdownTimeout:   DEFAULT_SHUTDOWN_TIMEOUT,
		MaxRetries:        DEFAULT_MAX_RETRIES,
		AutoReconnect:     DEFAULT_AUTO_RECONNECT,
		EngineOptions:     []engine.Option{},
	}
}
