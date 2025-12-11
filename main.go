//go:build windows
// +build windows

package simconnect

import (
	"context"
	"log/slog"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/manager"
)

func New() manager.Manager {
	return manager.New("simconnect-example",
		manager.WithContext(context.Background()),
		manager.WithAutoReconnect(true),
		manager.WithLogger(slog.Default()),
	)
}

func NewClient(name string, options ...engine.Option) engine.Client {
	return engine.New(name, options...)
}

func WithContext(ctx context.Context) engine.Option {
	return engine.WithContext(ctx)
}

func WithBufferSize(size int) engine.Option {
	return engine.WithBufferSize(size)
}

func WithLogger(logger *slog.Logger) engine.Option {
	return engine.WithLogger(logger)
}

func WithDLLPath(path string) engine.Option {
	return engine.WithDLLPath(path)
}

func WithHeartbeat(frequency string) engine.Option {
	return engine.WithHeartbeat(frequency)
}
