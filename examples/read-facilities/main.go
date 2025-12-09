//go:build windows
// +build windows

package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"os/signal"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// runConnection handles a single connection lifecycle to the simulator.
// Returns nil when the simulator disconnects (allowing reconnection),
// or an error if cancelled via context.
func runConnection(ctx context.Context) error {
	// Initialize client with context
	client := simconnect.New("GO Example - SimConnect Read facilities and their data",
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
				// Work with raw message pointer to avoid struct issues
				list := msg.AsAirportList()
				// Read header fields manually
				// SIMCONNECT_RECV (12 bytes: Size, Version, ID) + dwRequestID (4) + dwArraySize (4) + dwEntryNumber (4) + dwOutOf (4) = 28 bytes

				fmt.Printf("🏢 Received facility list:\n")
				fmt.Printf("  📋 Request ID: %d\n", list.DwRequestID)
				fmt.Printf("  📊 Array Size: %d\n", list.DwArraySize)
				fmt.Printf("  📦 Packet: %d of %d\n", list.DwEntryNumber, list.DwOutOf)

				if list.DwArraySize == 0 {
					fmt.Println("  No airports in this message")
					continue
				}

				// Calculate actual entry size from the message
				// SIMCONNECT_RECV_FACILITIES_LIST header is 28 bytes
				headerSize := types.DWORD(28)
				actualDataSize := msg.DwSize - headerSize
				actualEntrySize := actualDataSize / types.DWORD(list.DwArraySize)

				fmt.Printf("  Actual entry size: %d bytes\n", actualEntrySize)

				// Determine offsets based on entry size
				var latOffset, lonOffset, altOffset uintptr

				switch actualEntrySize {
				case 33: // Packed (1-byte alignment)
					// Ident(6) + Region(3) = 9
					latOffset, lonOffset, altOffset = 9, 17, 25
				case 36: // 4-byte alignment
					// Ident(6) + Region(3) + Padding(3) = 12
					latOffset, lonOffset, altOffset = 12, 20, 28
				case 40: // 8-byte alignment
					// Ident(6) + Region(3) + Padding(7) = 16
					latOffset, lonOffset, altOffset = 16, 24, 32
				default:
					// Fallback: Try 4-byte alignment as it's common for mixed C++ structs
					latOffset, lonOffset, altOffset = 12, 20, 28
					fmt.Printf("⚠️  Unknown entry size %d, defaulting to 4-byte aligned offsets (12/20/28)\n", actualEntrySize)
				}

				// Access the raw data buffer directly from the message
				dataStart := unsafe.Pointer(uintptr(unsafe.Pointer(list)) + uintptr(headerSize))

				for i := uint32(0); i < uint32(list.DwArraySize); i++ {
					// Calculate pointer to the start of this entry
					entryOffset := uintptr(i) * uintptr(actualEntrySize)
					entryPtr := unsafe.Pointer(uintptr(dataStart) + entryOffset)

					// 1. Read Ident (6 bytes) - Always at offset 0
					var ident [6]byte
					for j := 0; j < 6; j++ {
						ident[j] = *(*byte)(unsafe.Pointer(uintptr(entryPtr) + uintptr(j)))
					}

					// 2. Read Region (3 bytes) - Always at offset 6
					var region [3]byte
					for j := 0; j < 3; j++ {
						region[j] = *(*byte)(unsafe.Pointer(uintptr(entryPtr) + 6 + uintptr(j)))
					}

					// 3. Read Latitude
					latBytes := make([]byte, 8)
					for j := 0; j < 8; j++ {
						latBytes[j] = *(*byte)(unsafe.Pointer(uintptr(entryPtr) + latOffset + uintptr(j)))
					}
					lat := math.Float64frombits(binary.LittleEndian.Uint64(latBytes))

					// 4. Read Longitude
					lonBytes := make([]byte, 8)
					for j := 0; j < 8; j++ {
						lonBytes[j] = *(*byte)(unsafe.Pointer(uintptr(entryPtr) + lonOffset + uintptr(j)))
					}
					lon := math.Float64frombits(binary.LittleEndian.Uint64(lonBytes))

					// 5. Read Altitude
					altBytes := make([]byte, 8)
					for j := 0; j < 8; j++ {
						altBytes[j] = *(*byte)(unsafe.Pointer(uintptr(entryPtr) + altOffset + uintptr(j)))
					}
					alt := math.Float64frombits(binary.LittleEndian.Uint64(altBytes))

					fmt.Printf("  ✈️  Airport #%d: %s (%s) | 🌍 Lat: %.6f, Lon: %.6f | 📏 Alt: %.2fm\n",
						i+1,
						engine.BytesToString(ident[:]),
						engine.BytesToString(region[:]),
						lat,
						lon,
						alt,
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
