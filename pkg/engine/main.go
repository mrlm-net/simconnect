//go:build windows

package engine

import (
	"context"
	"log/slog"
	"os"
	"sync"

	"github.com/mrlm-net/simconnect/internal/simconnect"
)

func New(name string, options ...Option) *Engine {
	config := defaultConfig()
	for _, option := range options {
		option(config)
	}
	// If caller did not provide a logger, construct one using the configured
	// LogLevel. This defers logger creation until after options have been
	// applied so `WithLogLevel` and `WithLogger` behave intuitively.
	if config.Logger == nil {
		config.Logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: config.LogLevel}))
	}
	ctx, cancel := context.WithCancel(config.Context)
	return &Engine{
		api:    simconnect.New(name, &config.Config),
		cancel: cancel,
		config: config,
		ctx:    ctx,
		logger: config.Logger,
		// Initialize queue here to prevent race condition between Stream() and dispatch()
		queue: make(chan Message, config.BufferSize),
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
	closeOnce    sync.Once // Ensures queue is closed only once
}

// HeartbeatFrequency represents the valid heartbeat frequencies for SimConnect system events.
type HeartbeatFrequency string

const (
	HEARTBEAT_6HZ   HeartbeatFrequency = "6Hz"
	HEARTBEAT_1SEC  HeartbeatFrequency = "1sec"
	HEARTBEAT_4SEC  HeartbeatFrequency = "4sec"
	HEARTBEAT_FRAME HeartbeatFrequency = "Frame"
)
