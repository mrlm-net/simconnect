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

type AirportData struct {
	Latitude  float64
	Longitude float64
	Altitude  float64
	ICAO      [8]byte
	Name      [32]byte
	Name64    [64]byte
}

// runConnection handles a single connection lifecycle to the simulator.
// Returns nil when the simulator disconnects (allowing reconnection),
// or an error if cancelled via context.
func runConnection(ctx context.Context) error {
	// Initialize client with context
	client := simconnect.NewClient("GO Example - SimConnect Read facility and its data",
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

	// See remarks in docs for open/close usage
	// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/Facilities/SimConnect_AddToFacilityDefinition.htm#remarks
	client.AddToFacilityDefinition(3000, "OPEN AIRPORT")
	client.AddToFacilityDefinition(3000, "LATITUDE")
	client.AddToFacilityDefinition(3000, "LONGITUDE")
	client.AddToFacilityDefinition(3000, "ALTITUDE")
	client.AddToFacilityDefinition(3000, "ICAO")
	client.AddToFacilityDefinition(3000, "NAME")
	client.AddToFacilityDefinition(3000, "NAME64")
	client.AddToFacilityDefinition(3000, "CLOSE AIRPORT")
	client.RequestFacilityData(3000, 123, "LKPR", "")

	client.RequestFacilitiesList(3333, types.SIMCONNECT_FACILITY_LIST_WAYPOINT)

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

			case types.SIMCONNECT_RECV_ID_FACILITY_DATA:
				fmt.Println("🏗️  Received SIMCONNECT_RECV_ID_FACILITY_DATA message!")
				msg := msg.AsFacilityData()

				fmt.Printf("  UserRequestId: %d\n", msg.UserRequestId)
				fmt.Printf("  UniqueRequestId: %d\n", msg.UniqueRequestId)
				fmt.Printf("  ParentUniqueRequestId: %d\n", msg.ParentUniqueRequestId)
				fmt.Printf("  Type: %d\n", msg.Type)
				fmt.Printf("  IsListItem: %v\n", msg.IsListItem)
				fmt.Printf("  ItemIndex: %d\n", msg.ItemIndex)
				fmt.Printf("  ListSize: %d\n", msg.ListSize)

				if msg.UserRequestId == 123 {
					// Buffer of data. Have to cast it to a struct which matches the definition.
					data := engine.CastDataAs[AirportData](&msg.Data)
					fmt.Printf("  Data:\n")
					fmt.Printf("    Latitude: %f\n", data.Latitude)
					fmt.Printf("    Longitude: %f\n", data.Longitude)
					fmt.Printf("    Altitude: %f\n", data.Altitude)
					fmt.Printf("    ICAO: '%s'\n", engine.BytesToString(data.ICAO[:]))
					fmt.Printf("    Name: '%s'\n", engine.BytesToString(data.Name[:]))
					fmt.Printf("    Name64: '%s'\n", engine.BytesToString(data.Name64[:]))
				}
			case types.SIMCONNECT_RECV_ID_FACILITY_DATA_END:
				fmt.Println("🏁 Received SIMCONNECT_RECV_ID_FACILITY_DATA_END message!")
			case types.SIMCONNECT_RECV_ID_WAYPOINT_LIST:
				fmt.Println("🛤️  Received SIMCONNECT_RECV_ID_WAYPOINT_LIST message!")
				list := msg.AsWaypointList()
				fmt.Printf("  Number of waypoints: %d, size: %d\n", list.DwArraySize, list.DwSize)

				// Read array entries
				/*headerSize := unsafe.Sizeof(types.SIMCONNECT_RECV_FACILITIES_LIST{})
				actualDataSize := uintptr(msg.DwSize) - headerSize
				actualEntrySize := actualDataSize / uintptr(list.DwArraySize)
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

					fmt.Printf("  ✈️  Waypoint #%d: %s (%s) | 🌍 Lat: %.6f, Lon: %.6f | 📏 Alt: %.2fm\n",
						i+1,
						engine.BytesToString(ident[:]),
						engine.BytesToString(region[:]),
						lat, lon, alt,
					)
				}*/

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
