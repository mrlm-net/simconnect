//go:build !windows

package world

import (
	"context"
	"errors"
)

// runConnection: the World's own connection needs SimConnect, on Windows;
// elsewhere a host runs it on its client (RunOn) or the actuator's (#710).
func runConnection(ctx context.Context, st *state, requests <-chan string, dumpDir string) error {
	return errors.New("world: its own simulator connection needs Windows; use RunOn")
}
