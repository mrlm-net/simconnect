//go:build windows
// +build windows

package engine

import (
	"context"
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
		api:    simconnect.New(name, config),
		cancel: cancel,
		config: config,
		ctx:    ctx,
	}
}

type Engine struct {
	api    simconnect.API
	cancel context.CancelFunc
	config *Config
	ctx    context.Context
	queue  chan Message
	sync   sync.WaitGroup
}
