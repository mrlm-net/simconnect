//go:build windows
// +build windows

package engine

import (
	"github.com/mrlm-net/simconnect/pkg/types"
)

type Client interface {
	Connect() error
	Disconnect() error

	Stream() <-chan Message

	RequestSystemState(requestID uint32, state types.SIMCONNECT_SYSTEM_STATE) error
	SubscribeToSystemEvent(eventID uint32, eventName string) error
}
