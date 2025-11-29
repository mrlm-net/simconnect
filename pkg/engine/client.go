//go:build windows
// +build windows

package engine

type Client interface {
	Connect() error
	Disconnect() error
}
