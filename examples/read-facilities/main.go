//go:build windows
// +build windows

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// AirportEntry represents a single airport facility entry (36 bytes total)
// Must be packed to match exact memory layout from SimConnect
type AirportEntry struct {
	Ident  [6]byte // Offset 0-5
	Region [3]byte // Offset 6-8
	_      [3]byte // Offset 9-11 (padding)
	Lat    float64 // Offset 12-19
	Lon    float64 // Offset 20-27
	Alt    float64 // Offset 28-35
}

// runConnection handles a single connection lifecycle to the simulator.
// Returns nil when the simulator disconnects (allowing reconnection),
// or an error if cancelled via context.
func runConnection(ctx context.Context) error {
	// Initialize client with context
	client := simconnect.NewClient("GO Example - SimConnect Read facilities and their data",
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

	client.RequestFacilitiesListEX1(2000, types.SIMCONNECT_FACILITY_LIST_AIRPORT)

	// Wait for SIMCONNECT_RECV_ID_OPEN message to confirm connection is ready
	stream := client.Stream()
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

			case types.SIMCONNECT_RECV_ID_AIRPORT_LIST:
				list := msg.AsAirportList()

				fmt.Printf("🏢 Received facility list:\n")
				fmt.Printf("  📋 Request ID: %d\n", list.DwRequestID)
				fmt.Printf("  📊 Array Size: %d\n", list.DwArraySize)
				fmt.Printf("  📦 Packet: %d of %d\n", list.DwEntryNumber, list.DwOutOf)

				if list.DwArraySize == 0 {
					fmt.Println("  No airports in this message")
					continue
				}

				// Calculate actual entry size from the message
				headerSize := unsafe.Sizeof(types.SIMCONNECT_RECV_FACILITIES_LIST{})
				actualDataSize := uintptr(msg.DwSize) - headerSize
				actualEntrySize := actualDataSize / uintptr(list.DwArraySize)

				fmt.Printf("  📏 Header size: %d bytes\n", headerSize)
				fmt.Printf("  📏 Actual entry size: %d bytes\n", actualEntrySize)
				fmt.Printf("  📏 Struct size: %d bytes\n", unsafe.Sizeof(AirportEntry{}))

				// dataStart points to the beginning of the array data (after the header)
				dataStart := unsafe.Pointer(uintptr(unsafe.Pointer(list)) + headerSize)

				for i := uint32(0); i < uint32(list.DwArraySize); i++ {
					entryOffset := uintptr(i) * actualEntrySize
					entryPtr := unsafe.Pointer(uintptr(dataStart) + entryOffset)

					// Read fields at exact offsets - can't use struct due to Go's alignment rules
					var ident [6]byte
					var region [3]byte
					copy(ident[:], (*[6]byte)(entryPtr)[:])
					copy(region[:], (*[3]byte)(unsafe.Pointer(uintptr(entryPtr) + 6))[:])

					lat := *(*float64)(unsafe.Pointer(uintptr(entryPtr) + 12))
					lon := *(*float64)(unsafe.Pointer(uintptr(entryPtr) + 20))
					alt := *(*float64)(unsafe.Pointer(uintptr(entryPtr) + 28))

					fmt.Printf("  ✈️  Airport #%d: %s (%s) | 🌍 Lat: %.6f, Lon: %.6f | 📏 Alt: %.2fm\n",
						i+1,
						engine.BytesToString(ident[:]),
						engine.BytesToString(region[:]),
						lat, lon, alt,
					)
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
