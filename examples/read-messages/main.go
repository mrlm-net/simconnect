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
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// runConnection handles a single connection lifecycle to the simulator.
// Returns nil when the simulator disconnects (allowing reconnection),
// or an error if cancelled via context.
func runConnection(ctx context.Context) error {
	// Initialize client with context
	client := simconnect.New("GO Example - SimConnect Lifecycle Connection",
		engine.WithContext(ctx),
	)

	// Retry connection until simulator is running
	fmt.Println("Waiting for simulator to start...")
	for {
		select {
		case <-ctx.Done():
			fmt.Println("Cancelled while waiting for simulator")
			return ctx.Err()
		default:
			if err := client.Connect(); err != nil {
				fmt.Printf("Connection attempt failed: %v, retrying in 2 seconds...\n", err)
				time.Sleep(2 * time.Second)
				continue
			}
			goto connected
		}
	}

connected:
	fmt.Println("Connected to SimConnect, listening for messages...")

	// Wait for SIMCONNECT_RECV_ID_OPEN message to confirm connection is ready
	stream := client.Stream()

	// We can already register data definitions and requests here

	// Example: Subscribe to a system event (Pause)
	// --------------------------------------------
	// - Pause event occurs when user pauses/unpauses the simulator.
	//   State is returned in dwData field as number (0=unpaused, 1=paused)
	client.SubscribeToSystemEvent(1000, "Pause")

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Context cancelled, disconnecting...")
			if err := client.Disconnect(); err != nil {
				fmt.Fprintf(os.Stderr, "Disconnect error: %v\n", err)
			}
			//fmt.Println("Disconnected from SimConnect")
			return ctx.Err()
		case msg, ok := <-stream:
			if !ok {
				fmt.Println("Stream closed (simulator disconnected)")
				return nil // Return nil to allow reconnection
			}

			if msg.Err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", msg.Err)
				continue
			}

			// Log the connection ready message specially
			if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_OPEN {
				fmt.Println("Connection ready (SIMCONNECT_RECV_ID_OPEN received)")
			}

			fmt.Printf("Message received - ID: %d, Size: %d bytes\n", msg.DwID, msg.Size)

			// Handle specific messages
			// This could be done based on type and also if needed request IDs
			switch types.SIMCONNECT_RECV_ID(msg.DwID) {
			case types.SIMCONNECT_RECV_ID_EVENT:
				eventMsg := msg.AsEventType()
				fmt.Printf("  Event ID: %d, Data: %d\n", eventMsg.DwID, eventMsg.DwData)
				//fmt.Printf("  Event ID: %d, Data: %d\n", eventMsg.EventID, eventMsg.Data)
				// Add more cases here for other message types as needed
			}
		}
	}
}

func main() {
	// Create cancellable context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())

	// Setup signal handler for Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		fmt.Println("Received interrupt signal, shutting down...")
		cancel()
	}()

	fmt.Println("(Press Ctrl+C to exit)")

	// Reconnection loop - keeps trying to connect when simulator disconnects
	for {
		err := runConnection(ctx)
		if err != nil {
			// Context cancelled (Ctrl+C) - exit completely
			fmt.Printf("Connection ended: %v\n", err)
			return
		}

		// Simulator disconnected (err == nil) - wait and retry
		fmt.Println("Waiting 5 seconds before reconnecting...")
		select {
		case <-ctx.Done():
			fmt.Println("Shutdown requested, not reconnecting")
			return
		case <-time.After(5 * time.Second):
			fmt.Println("Attempting to reconnect...")
		}
	}
}
