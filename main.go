//go:build windows
// +build windows

package simconnect

import (
	"context"
	"log/slog"
	"time"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/manager"
)

// HeartbeatFrequency re-exports the engine HeartbeatFrequency type so callers
// can refer to `simconnect.HeartbeatFrequency` instead of importing
// `pkg/engine` directly.
type HeartbeatFrequency = engine.HeartbeatFrequency

// HEARTBEAT_* constants moved from the old `types` package into `pkg/engine`.
// They are re-exported here for convenience as `simconnect.HEARTBEAT_*`.
const (
	HEARTBEAT_6HZ   HeartbeatFrequency = engine.HEARTBEAT_6HZ
	HEARTBEAT_1SEC  HeartbeatFrequency = engine.HEARTBEAT_1SEC
	HEARTBEAT_4SEC  HeartbeatFrequency = engine.HEARTBEAT_4SEC
	HEARTBEAT_FRAME HeartbeatFrequency = engine.HEARTBEAT_FRAME
)

func New(name string, options ...manager.Option) manager.Manager {
	return manager.New(name, options...)
}

func NewClient(name string, options ...engine.Option) engine.Client {
	return engine.New(name, options...)
}

// ====================
// Client (Engine) Options
// ====================

// ClientWithContext sets the context for the client
func ClientWithContext(ctx context.Context) engine.Option {
	return engine.WithContext(ctx)
}

// ClientWithBufferSize sets the buffer size for the client.
// Default is 256.
func ClientWithBufferSize(size int) engine.Option {
	return engine.WithBufferSize(size)
}

// ClientWithLogger sets the logger for the client
func ClientWithLogger(logger *slog.Logger) engine.Option {
	return engine.WithLogger(logger)
}

// ClientWithLogLevel sets the minimum level for the client's default logger.
func ClientWithLogLevel(level slog.Level) engine.Option {
	return engine.WithLogLevel(level)
}

// ClientWithLogLevelFromString sets the client's default logger level from a
// textual representation like "debug" or "info".
func ClientWithLogLevelFromString(level string) engine.Option {
	return engine.WithLogLevelFromString(level)
}

// ClientWithDLLPath sets the path to the SimConnect DLL.
// Default is "C:/MSFS 2024 SDK/SimConnect SDK/lib/SimConnect.dll".
func ClientWithDLLPath(path string) engine.Option {
	return engine.WithDLLPath(path)
}

// ClientWithHeartbeat sets the heartbeat frequency.
// Valid values: HEARTBEAT_6HZ, HEARTBEAT_1SEC, HEARTBEAT_4SEC, HEARTBEAT_FRAME.
// These constants moved from the `types` package into `pkg/engine` and
// are re-exported here as `simconnect.HEARTBEAT_*` for convenience.
// Default is HEARTBEAT_6HZ.
func ClientWithHeartbeat(frequency engine.HeartbeatFrequency) engine.Option {
	return engine.WithHeartbeat(frequency)
}

// ====================
// Manager Options
// ====================

// WithContext sets the context for the manager
func WithContext(ctx context.Context) manager.Option {
	return manager.WithContext(ctx)
}

// WithLogger sets the logger for the manager
func WithLogger(logger *slog.Logger) manager.Option {
	return manager.WithLogger(logger)
}

// WithLogLevel sets the manager's default logger level.
func WithLogLevel(level slog.Level) manager.Option {
	return manager.WithLogLevel(level)
}

// WithLogLevelFromString sets the manager's default logger level from a
// textual representation like "debug" or "info".
func WithLogLevelFromString(level string) manager.Option {
	return manager.WithLogLevelFromString(level)
}

// WithRetryInterval sets the fixed delay between connection attempts.
// Default is 15 seconds.
func WithRetryInterval(d time.Duration) manager.Option {
	return manager.WithRetryInterval(d)
}

// WithConnectionTimeout sets the timeout for each connection attempt.
// Default is 30 seconds.
func WithConnectionTimeout(d time.Duration) manager.Option {
	return manager.WithConnectionTimeout(d)
}

// WithReconnectDelay sets the delay before reconnecting after disconnect.
// Default is 30 seconds.
func WithReconnectDelay(d time.Duration) manager.Option {
	return manager.WithReconnectDelay(d)
}

// WithShutdownTimeout sets the timeout for graceful shutdown of subscriptions.
// Default is 10 seconds.
func WithShutdownTimeout(d time.Duration) manager.Option {
	return manager.WithShutdownTimeout(d)
}

// WithMaxRetries sets the maximum number of connection retries.
// 0 means unlimited retries. Default is 0.
func WithMaxRetries(n int) manager.Option {
	return manager.WithMaxRetries(n)
}

// WithAutoReconnect enables or disables automatic reconnection.
// Default is true.
func WithAutoReconnect(enabled bool) manager.Option {
	return manager.WithAutoReconnect(enabled)
}

// WithEngineOptions passes options through to the underlying engine.
// Note: Context and Logger options passed here will be ignored as the manager
// controls these settings. Use WithContext and WithLogger instead.
func WithEngineOptions(opts ...engine.Option) manager.Option {
	return manager.WithEngineOptions(opts...)
}

// WithBufferSize sets the buffer size for the underlying engine.
// This is a convenience wrapper. Default is 256.
func WithBufferSize(size int) manager.Option {
	return manager.WithBufferSize(size)
}

// WithDLLPath sets the path to the SimConnect DLL for the underlying engine.
// Default is "C:/MSFS 2024 SDK/SimConnect SDK/lib/SimConnect.dll".
func WithDLLPath(path string) manager.Option {
	return manager.WithDLLPath(path)
}

// WithHeartbeat sets the heartbeat frequency for the underlying engine.
// Valid values: HEARTBEAT_6HZ, HEARTBEAT_1SEC, HEARTBEAT_4SEC, HEARTBEAT_FRAME.
// These constants moved from the `types` package into `pkg/engine` and
// are re-exported here as `simconnect.HEARTBEAT_*` for convenience.
// Default is HEARTBEAT_6HZ.
func WithHeartbeat(frequency engine.HeartbeatFrequency) manager.Option {
	return manager.WithHeartbeat(frequency)
}
