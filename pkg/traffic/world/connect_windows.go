//go:build windows

package world

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/engine"
)

// runConnection handles one connection lifecycle. It returns nil when the
// simulator disconnects (so the caller reconnects) and ctx.Err() on shutdown.
func runConnection(ctx context.Context, st *state, requests <-chan string, dumpDir string) error {
	opts := []engine.Option{engine.WithContext(ctx)}
	// SIMCONNECT_CALL_TRACE=1: each exception logged with the call it was
	// raised for (engine.WithCallTrace), to track one down.
	if os.Getenv("SIMCONNECT_CALL_TRACE") == "1" {
		opts = append(opts, engine.WithCallTrace())
	}
	client := simconnect.NewClient("GO Example - airport map", opts...)

	fmt.Fprintln(stdout, "⏳ Waiting for simulator to start...")
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
	fmt.Fprintln(stdout, "✅ Connected to SimConnect")
	defer client.Disconnect()
	return runOn(ctx, st, client, client.Stream(), requests, dumpDir)
}
