//go:build windows
// +build windows

package simconnect

import (
	"context"
	"log/slog"

	"github.com/mrlm-net/simconnect/pkg/engine"
)

func New(name string, options ...engine.Option) engine.Client {
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
