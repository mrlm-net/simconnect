//go:build windows
// +build windows

package engine

import (
	"github.com/mrlm-net/simconnect/internal/simconnect"
)

func New(name string, options ...Option) *Engine {
	config := defaultConfig()
	for _, option := range options {
		option(config)
	}
	return &Engine{
		api:    simconnect.New(config),
		config: config,
	}
}

type Engine struct {
	api    Client
	config *Config
}
