//go:build windows
// +build windows

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/types"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	// Setup signal handler for Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		fmt.Println("🛑 Received interrupt signal, shutting down...")
		cancel()
	}()
	// This is a placeholder main function.
	// The actual implementation would go here.
	client := simconnect.New("GO Example - SimConnect Basic Connection")

	if err := client.Connect(); err != nil {
		fmt.Fprintln(os.Stderr, "❌ Error:", err)
		return
	}
	defer func() {
		if err := client.Disconnect(); err != nil {
			fmt.Fprintln(os.Stderr, "❌ Disconnect error:", err)
			return
		}

		fmt.Println("👋 Disconnected from SimConnect...")

	}()

	// Application logic would go here.
	fmt.Println("✅ Connected to SimConnect...")
	fmt.Println("⏳ Sleeping for 2 seconds...")
	time.Sleep(2 * time.Second)

	client.AddToDataDefinition(1000, "CAMERA STATE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 0)
	client.RequestDataOnSimObject(1000, 1000, types.SIMCONNECT_OBJECT_ID_USER, types.SIMCONNECT_PERIOD_SECOND, 0, 0, 0, 0)

	queue := client.Stream()
	fmt.Println("✈️  Ready for takeoff!")

	for {
		select {
		case <-ctx.Done():
			fmt.Println("🔌 Context cancelled, disconnecting...")
			if err := client.Disconnect(); err != nil {
				fmt.Fprintf(os.Stderr, "❌ Disconnect error: %v\n", err)
			}
			return
		case msg, ok := <-queue:
			if !ok {
				fmt.Println("📴 Stream closed (simulator disconnected)")
				return
			}

			if msg.Err != nil {
				fmt.Fprintf(os.Stderr, "❌ Error: %v\n", msg.Err)
				continue
			}
			fmt.Println(msg, ok)

		}

	}
}
