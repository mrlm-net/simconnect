//go:build windows
// +build windows

package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
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
	client := simconnect.NewClient("GO Example - SimConnect Emit Events (Interactive Door Toggle)",
		engine.WithContext(ctx),
	)

	// Retry connection until simulator is running
	fmt.Println("⏳ Waiting for simulator to start...")
	for {
		select {
		case <-ctx.Done():
			fmt.Println("🛑 Cancelled while waiting for simulator")
			return ctx.Err()
		default:
			if err := client.Connect(); err != nil {
				fmt.Printf("🔄 Connection attempt failed: %v, retrying in 2 seconds...\n", err)
				time.Sleep(2 * time.Second)
				continue
			}
			goto connected
		}
	}

connected:
	fmt.Println("✅ Connected to SimConnect, listening for messages...")
	// We can already register data definitions and requests here
	// Map events
	client.MapClientEventToSimEvent(2010, "TOGGLE_AIRCRAFT_EXIT")
	// Add to notification group
	client.AddClientEventToNotificationGroup(3000, 2010, false)
	client.SetNotificationGroupPriority(3000, 1000)

	// Wait for SIMCONNECT_RECV_ID_OPEN message to confirm connection is ready
	stream := client.Stream()

	// Track if connection is ready to start accepting input
	connectionReady := false
	inputStarted := false
	// Main message processing loop
	for {
		select {
		case <-ctx.Done():
			fmt.Println("🔌 Context cancelled, disconnecting...")
			if err := client.Disconnect(); err != nil {
				fmt.Fprintf(os.Stderr, "❌ Disconnect error: %v\n", err)
			}
			//fmt.Println("Disconnected from SimConnect")
			return ctx.Err()
		case msg, ok := <-stream:
			if !ok {
				fmt.Println("📴 Stream closed (simulator disconnected)")
				return nil // Return nil to allow reconnection
			}

			if msg.Err != nil {
				fmt.Fprintf(os.Stderr, "❌ Error: %v\n", msg.Err)
				continue
			}

			fmt.Println("📨 Message received - ", types.SIMCONNECT_RECV_ID(msg.SIMCONNECT_RECV.DwID))

			//fmt.Printf("📨 Message received - ID: %d, Size: %d bytes\n", msg, msg.Size)

			// Handle specific messages
			// This could be done based on type and also if needed request IDs
			switch types.SIMCONNECT_RECV_ID(msg.DwID) {
			case types.SIMCONNECT_RECV_ID_OPEN:
				fmt.Println("🟢 Connection ready (SIMCONNECT_RECV_ID_OPEN received)")
				msg := msg.AsOpen()
				fmt.Println("📡 Received SIMCONNECT_RECV_OPEN message!")
				fmt.Printf("  Application Name: '%s'\n", engine.BytesToString(msg.SzApplicationName[:]))
				fmt.Printf("  Application Version: %d.%d\n", msg.DwApplicationVersionMajor, msg.DwApplicationVersionMinor)
				fmt.Printf("  Application Build: %d.%d\n", msg.DwApplicationBuildMajor, msg.DwApplicationBuildMinor)
				fmt.Printf("  SimConnect Version: %d.%d\n", msg.DwSimConnectVersionMajor, msg.DwSimConnectVersionMinor)
				fmt.Printf("  SimConnect Build: %d.%d\n", msg.DwSimConnectBuildMajor, msg.DwSimConnectBuildMinor)

				// Mark connection as ready and start input goroutine
				connectionReady = true
				if !inputStarted {
					inputStarted = true
					fmt.Println("\n🚪 Interactive Door Toggle")
					fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
					fmt.Println("Enter a door/exit index number and press Enter to toggle it.")
					fmt.Println("Common indices: 1 (main door), 2-25 (additional doors/exits)")
					fmt.Println("Note: Valid door indices vary by aircraft model.")
					fmt.Println("Press Ctrl+C to exit.")
					fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

					// Start input goroutine
					go func() {
						scanner := bufio.NewScanner(os.Stdin)
						for {
							select {
							case <-ctx.Done():
								return
							default:
								fmt.Print("Door index: ")
								if !scanner.Scan() {
									// EOF or error
									return
								}

								input := strings.TrimSpace(scanner.Text())
								if input == "" {
									continue
								}

								// Parse door index
								doorIndex, err := strconv.ParseUint(input, 10, 32)
								if err != nil {
									fmt.Printf("❌ Invalid input '%s'. Please enter a positive integer.\n\n", input)
									continue
								}

								if doorIndex == 0 {
									fmt.Println("❌ Door index must be greater than 0.")
									continue
								}

								// Transmit the event
								if !connectionReady {
									fmt.Println("⚠️  Connection not ready yet. Please wait...")
									continue
								}

								err = client.TransmitClientEvent(
									types.SIMCONNECT_OBJECT_ID_USER,
									2010,
									uint32(doorIndex),
									3000,
									0,
								)

								if err != nil {
									fmt.Printf("❌ Failed to transmit event: %v\n\n", err)
								} else {
									fmt.Printf("✅ Toggled door/exit index %d\n\n", doorIndex)
								}
							}
						}
					}()
				}

			default:
				// Other message types can be handled here
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
		fmt.Println("🛑 Received interrupt signal, shutting down...")
		cancel()
	}()

	fmt.Println("ℹ️  (Press Ctrl+C to exit)")

	// Reconnection loop - keeps trying to connect when simulator disconnects
	for {
		err := runConnection(ctx)
		if err != nil {
			// Context cancelled (Ctrl+C) - exit completely
			fmt.Printf("⚠️  Connection ended: %v\n", err)
			return
		}

		// Simulator disconnected (err == nil) - wait and retry
		fmt.Println("⏳ Waiting 5 seconds before reconnecting...")
		select {
		case <-ctx.Done():
			fmt.Println("🛑 Shutdown requested, not reconnecting")
			return
		case <-time.After(5 * time.Second):
			fmt.Println("🔄 Attempting to reconnect...")
		}
	}
}
