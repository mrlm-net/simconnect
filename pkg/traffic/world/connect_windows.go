//go:build windows

package world

import (
	"context"
	"fmt"
	"time"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/engine"
)

// runConnection handles one connection lifecycle. It returns nil when the
// simulator disconnects (so the caller reconnects) and ctx.Err() on shutdown.
func runConnection(ctx context.Context, st *state, requests <-chan string, dumpDir string) error {
	client := simconnect.NewClient("GO Example - airport map", engine.WithContext(ctx))

	fmt.Println("⏳ Waiting for simulator to start...")
	for {
		if err := client.Connect(); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	fmt.Println("✅ Connected to SimConnect")
	defer client.Disconnect()
	return runOn(ctx, st, client, client.Stream(), requests, dumpDir)
}
