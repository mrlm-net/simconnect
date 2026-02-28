//go:build windows
// +build windows

// ⚠️  MSFS 2024 ONLY — SimConnect_SubscribeToFlowEvent and
// SimConnect_UnsubscribeToFlowEvent are available in Microsoft Flight Simulator 2024
// only and will return E_FAIL on Microsoft Flight Simulator 2020.

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

func runConnection(ctx context.Context) error {
	client := simconnect.NewClient("GO Example - Flow Events",
		engine.WithContext(ctx),
	)

	fmt.Println("⏳ Waiting for simulator...")
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := client.Connect(); err != nil {
				fmt.Printf("🔄 Retrying in 2s: %v\n", err)
				time.Sleep(2 * time.Second)
				continue
			}
			goto connected
		}
	}

connected:
	fmt.Println("✅ Connected to SimConnect")

	// Subscribe to all flow events. A single call covers all SIMCONNECT_FLOW_EVENT
	// values; there is no per-event filtering at the subscription level.
	if err := client.SubscribeToFlowEvent(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ SubscribeToFlowEvent: %v\n", err)
		fmt.Fprintf(os.Stderr, "   (Running on MSFS 2020? Flow events require MSFS 2024.)\n")
	} else {
		fmt.Println("📡 Subscribed to flow events — waiting for simulator activity...")
	}

	stream := client.Stream()
	for {
		select {
		case <-ctx.Done():
			fmt.Println("🔌 Shutting down...")
			if err := client.UnsubscribeFromFlowEvent(); err != nil {
				fmt.Fprintf(os.Stderr, "⚠️  UnsubscribeFromFlowEvent: %v\n", err)
			}
			if err := client.Disconnect(); err != nil {
				fmt.Fprintf(os.Stderr, "❌ Disconnect: %v\n", err)
			}
			return ctx.Err()

		case msg, ok := <-stream:
			if !ok {
				fmt.Println("📴 Stream closed (simulator disconnected)")
				return nil
			}
			if msg.Err != nil {
				fmt.Fprintf(os.Stderr, "❌ Stream error: %v\n", msg.Err)
				continue
			}

			switch types.SIMCONNECT_RECV_ID(msg.DwID) {

			case types.SIMCONNECT_RECV_ID_OPEN:
				o := msg.AsOpen()
				fmt.Printf("🟢 Simulator: %s v%d.%d\n",
					engine.BytesToString(o.SzApplicationName[:]),
					o.DwApplicationVersionMajor, o.DwApplicationVersionMinor)

			case types.SIMCONNECT_RECV_ID_FLOW_EVENT:
				fe := msg.AsFlowEvent()
				path := engine.BytesToString(fe.FltPath[:])
				if path == "" {
					path = "(none)"
				}
				fmt.Printf("🌊 Flow event: %-24s  flt=%s\n", fe.FlowEvent, path)

			case types.SIMCONNECT_RECV_ID_EXCEPTION:
				ex := msg.AsException()
				fmt.Printf("🚨 SimConnect exception %d at send index %d\n",
					ex.DwException, ex.DwIndex)
			}
		}
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("🛑 Interrupt received, shutting down...")
		cancel()
	}()

	fmt.Println("ℹ️  Press Ctrl+C to exit")
	fmt.Println("⚠️  MSFS 2024 only — will not work on MSFS 2020")

	for {
		if err := runConnection(ctx); err != nil {
			fmt.Printf("⚠️  %v\n", err)
			return
		}
		fmt.Println("⏳ Waiting 5s before reconnecting...")
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
			fmt.Println("🔄 Reconnecting...")
		}
	}
}
