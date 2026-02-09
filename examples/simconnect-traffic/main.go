//go:build windows
// +build windows

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync/atomic"
	"time"

	"github.com/mrlm-net/simconnect/pkg/datasets/traffic"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/manager"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// AircraftData wraps the traffic dataset with helper methods
type AircraftData traffic.AircraftDataset

func (data *AircraftData) TitleAsString() string {
	return engine.BytesToString(data.Title[:])
}

func (data *AircraftData) CategoryAsString() string {
	return engine.BytesToString(data.Category[:])
}

func (data *AircraftData) LiveryNameAsString() string {
	return engine.BytesToString(data.LiveryName[:])
}

func (data *AircraftData) LiveryFolderAsString() string {
	return engine.BytesToString(data.LiveryFolder[:])
}

func (data *AircraftData) ATCIDAsString() string {
	return engine.BytesToString(data.AtcID[:])
}

func (data *AircraftData) AtcAirlineAsString() string {
	return engine.BytesToString(data.AtcAirline[:])
}

// setupDataDefinitions registers the traffic dataset when connection is available
func setupDataDefinitions(mgr manager.Manager) {
	fmt.Println("✅ Setting up aircraft data definitions...")
	// Register the traffic dataset with define ID 3000
	if err := mgr.RegisterDataset(3000, traffic.NewAircraftDataset()); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to register dataset: %v\n", err)
		return
	}
	// Initial request for all aircraft within 25km radius
	if err := mgr.RequestDataOnSimObjectType(4001, 3000, 25000, types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to request aircraft data: %v\n", err)
	}
}

func main() {
	// Create cancellable context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())

	// Setup signal handler for Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	fmt.Println("ℹ️  (Press Ctrl+C to exit)")

	// Create the manager with automatic reconnection
	mgr := manager.New("GO Example - SimConnect Traffic Monitor",
		manager.WithContext(ctx),
		manager.WithAutoReconnect(true),
		manager.WithBufferSize(512),
		manager.WithHeartbeat("6Hz"),
	)

	// Use atomic bool to prevent starting multiple tickers on reconnect
	var tickerStarted atomic.Bool

	// Register connection state change handler
	_ = mgr.OnConnectionStateChange(func(oldState, newState manager.ConnectionState) {
		fmt.Printf("🔄 Connection state changed: %s -> %s\n", oldState, newState)

		switch newState {
		case manager.StateConnecting:
			fmt.Println("⏳ Connecting to simulator...")
		case manager.StateConnected:
			fmt.Println("✅ Connected to SimConnect")
			// Setup data definitions when connected
			setupDataDefinitions(mgr)
		case manager.StateAvailable:
			fmt.Println("🚀 Simulator connection is AVAILABLE")
			// Start periodic data requests only once
			if tickerStarted.CompareAndSwap(false, true) {
				ticker := time.NewTicker(5 * time.Second)
				go func() {
					for {
						select {
						case <-ctx.Done():
							ticker.Stop()
							return
						case <-ticker.C:
							if err := mgr.RequestDataOnSimObjectType(4001, 3000, 25000, types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT); err != nil {
								fmt.Fprintf(os.Stderr, "❌ Failed to request aircraft data: %v\n", err)
							}
						}
					}
				}()
			}
		case manager.StateReconnecting:
			fmt.Println("🔄 Reconnecting...")
		case manager.StateDisconnected:
			fmt.Println("📴 Disconnected...")
		}
	})

	// Register message handler
	_ = mgr.OnMessage(func(msg engine.Message) {
		if msg.Err != nil {
			fmt.Fprintf(os.Stderr, "❌ Error: %v\n", msg.Err)
			return
		}

		// Handle specific messages
		switch types.SIMCONNECT_RECV_ID(msg.DwID) {
		case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE:
			simObjData := msg.AsSimObjectDataBType()
			fmt.Printf("     Request ID: %d, Define ID: %d, Object ID: %d, Flags: %d, Out of: %d, DefineCount: %d\n",
				simObjData.DwRequestID,
				simObjData.DwDefineID,
				simObjData.DwObjectID,
				simObjData.DwFlags,
				simObjData.DwOutOf,
				simObjData.DwDefineCount,
			)
			if simObjData.DwDefineID == 3000 {
				aircraftData := engine.CastDataAs[AircraftData](&simObjData.DwData)
				fmt.Printf("     Aircraft Title: %s, Category: %s, Livery Name: %s, Livery Folder: %s, Lat: %f, Lon: %f, Alt: %f, Head: %f, GroundSpeed: %f, AtcID: %s, AtcAirline: %s\n",
					aircraftData.TitleAsString(),
					aircraftData.CategoryAsString(),
					aircraftData.LiveryNameAsString(),
					aircraftData.LiveryFolderAsString(),
					aircraftData.Lat,
					aircraftData.Lon,
					aircraftData.Alt,
					aircraftData.Head,
					aircraftData.GroundSpeed,
					aircraftData.ATCIDAsString(),
					aircraftData.AtcAirlineAsString(),
				)
			}
		default:
			// Other message types can be handled here
		}
	})

	// Setup signal handler goroutine
	go func() {
		<-sigChan
		fmt.Println("🛑 Received interrupt signal, shutting down...")
		mgr.Stop()
		cancel()
	}()

	// Start the manager - this blocks until context is cancelled
	if err := mgr.Start(); err != nil {
		fmt.Printf("⚠️  Manager stopped: %v\n", err)
	}

	fmt.Println("👋 Goodbye!")
}
