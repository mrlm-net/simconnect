//go:build windows
// +build windows

package simconnect

import "github.com/mrlm-net/simconnect/pkg/engine"

func New(name string, options ...engine.Option) engine.Client {
	return engine.New(name, options...)
}
