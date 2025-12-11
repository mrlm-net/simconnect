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
	DEFAULT_INITIAL_DELAY   = 2 * time.Second
	DEFAULT_MAX_DELAY       = 30 * time.Second
	DEFAULT_RECONNECT_DELAY = 5 * time.Second
	DEFAULT_MAX_RETRIES     = 0 // 0 = unlimited retries
	DEFAULT_BACKOFF_FACTOR  = 2.0
	DEFAULT_AUTO_RECONNECT  = true
)

// Config holds the configuration for the Manager
type Config struct {
	// Context for the manager lifecycle
	Context context.Context

	// Logger for manager operations
	Logger *slog.Logger

	// Connection backoff settings
	InitialDelay   time.Duration // Initial delay between connection attempts
	MaxDelay       time.Duration // Maximum delay between connection attempts
	ReconnectDelay time.Duration // Delay before reconnecting after disconnect
	MaxRetries     int           // Maximum number of connection retries (0 = unlimited)
	BackoffFactor  float64       // Multiplier for exponential backoff

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

// WithInitialDelay sets the initial delay between connection attempts
func WithInitialDelay(d time.Duration) Option {
	return func(c *Config) {
		c.InitialDelay = d
	}
}

// WithMaxDelay sets the maximum delay between connection attempts
func WithMaxDelay(d time.Duration) Option {
	return func(c *Config) {
		c.MaxDelay = d
	}
}

// WithReconnectDelay sets the delay before reconnecting after disconnect
func WithReconnectDelay(d time.Duration) Option {
	return func(c *Config) {
		c.ReconnectDelay = d
	}
}

// WithMaxRetries sets the maximum number of connection retries (0 = unlimited)
func WithMaxRetries(n int) Option {
	return func(c *Config) {
		c.MaxRetries = n
	}
}

// WithBackoffFactor sets the multiplier for exponential backoff
func WithBackoffFactor(f float64) Option {
	return func(c *Config) {
		c.BackoffFactor = f
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
		Context:        context.Background(),
		Logger:         slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})),
		InitialDelay:   DEFAULT_INITIAL_DELAY,
		MaxDelay:       DEFAULT_MAX_DELAY,
		ReconnectDelay: DEFAULT_RECONNECT_DELAY,
		MaxRetries:     DEFAULT_MAX_RETRIES,
		BackoffFactor:  DEFAULT_BACKOFF_FACTOR,
		AutoReconnect:  DEFAULT_AUTO_RECONNECT,
		EngineOptions:  []engine.Option{},
	}
}
