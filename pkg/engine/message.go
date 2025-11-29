//go:build windows
// +build windows

package engine

import "github.com/mrlm-net/simconnect/pkg/types"

type Message struct {
	*types.SIMCONNECT_RECV
	Size uint32
	Err  error
}
