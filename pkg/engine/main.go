//go:build windows
// +build windows

package engine

import (
	"context"
	"log/slog"
	"sync"

	"github.com/mrlm-net/simconnect/internal/simconnect"
)

func New(name string, options ...Option) *Engine {
	config := defaultConfig()
	for _, option := range options {
		option(config)
	}
	ctx, cancel := context.WithCancel(config.Context)
	return &Engine{
		api:    simconnect.New(name, &config.Config),
		cancel: cancel,
		config: config,
		ctx:    ctx,
		logger: config.Logger,
	}
}

type Engine struct {
	api          simconnect.API
	cancel       context.CancelFunc
	config       *Config
	ctx          context.Context
	dispatchOnce sync.Once
	logger       *slog.Logger
	queue        chan Message
	sync         sync.WaitGroup
}
