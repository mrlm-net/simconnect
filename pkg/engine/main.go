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
		config: config,
		ctx:    ctx,
		cancel: cancel,
		queue:  make(chan Message, config.BufferSize),
	}
}

type Engine struct {
	api    SimConnect
	config *Config
	ctx    context.Context
	cancel context.CancelFunc
	queue  chan Message
	sync   sync.WaitGroup
}
