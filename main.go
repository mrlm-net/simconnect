//go:build windows
// +build windows

package simconnect

import (
	"context"

	"github.com/mrlm-net/simconnect/pkg/engine"
)

func New(name string, options ...engine.Option) engine.Client {
	return engine.New(name, options...)
}

func WithContext(ctx context.Context) engine.Option {
	return engine.WithContext(ctx)
}
