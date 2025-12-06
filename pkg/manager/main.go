//go:build windows
// +build windows

package manager

import "github.com/mrlm-net/simconnect/pkg/engine"

func New(name string) Manager {
	return &Instance{
		engine: engine.New(name),
	}
}

type Instance struct {
	engine *engine.Engine
}
