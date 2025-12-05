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

// CameraData represents the data structure for CAMERA STATE and CAMERA SUBSTATE
// The fields must match the order of AddToDataDefinition calls
type CameraData struct {
	CameraState    int32
	CameraSubstate int32
}

// runConnection handles a single connection lifecycle to the simulator.
// Returns nil when the simulator disconnects (allowing reconnection),
// or an error if cancelled via context.
func runConnection(ctx context.Context) error {
	// Initialize client with context
	client := simconnect.New("GO Example - SimConnect Read Messages and their data",
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
	// Wait for SIMCONNECT_RECV_ID_OPEN message to confirm connection is ready
	stream := client.Stream()
	// We can already register data definitions and requests here

	// Example: Subscribe to a system event (Pause, Sim, Sound, etc.)
	// --------------------------------------------
	// - Pause event occurs when user pauses/unpauses the simulator.
	//   State is returned in dwData field as number (0=unpaused, 1=paused)
	client.SubscribeToSystemEvent(1000, "Pause")
	// --------------------------------------------
	// - Sim event occurs when simulator starts/stops.
	//   State is returned in dwData field as number (0=stopped, 1=started)
	client.SubscribeToSystemEvent(1001, "Sim")
	// --------------------------------------------
	// - Sound event occurs when simulator master sound is toggled.
	//   State is returned in dwData field as number (0=off, 1=on)
	client.SubscribeToSystemEvent(1002, "Sound")
	// --------------------------------------------
	// - Define data structure for CAMERA STATE and CAMERA SUBSTATE
	//   and request updates every second
	// --------------------------------------------
	client.AddToDataDefinition(4, "CAMERA STATE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 0)
	client.AddToDataDefinition(4, "CAMERA SUBSTATE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 1)
	client.RequestDataOnSimObject(2000, 4, types.SIMCONNECT_OBJECT_ID_USER, types.SIMCONNECT_PERIOD_SECOND, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)
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

			fmt.Printf("📨 Message received - ID: %d, Size: %d bytes\n", msg.DwID, msg.Size)

			// Handle specific messages
			// This could be done based on type and also if needed request IDs
			switch types.SIMCONNECT_RECV_ID(msg.DwID) {
			case types.SIMCONNECT_RECV_ID_EVENT:
				eventMsg := msg.AsEvent()
				fmt.Printf("  Event ID: %d, Data: %d\n", eventMsg.UEventID, eventMsg.DwData)
				// Check if this is the Pause event (ID 1000)
				if eventMsg.UEventID == 1000 {
					if eventMsg.DwData == 1 {
						fmt.Println("  ⏸️  Simulator is PAUSED")
					} else {
						fmt.Println("  ▶️  Simulator is UNPAUSED")
					}
				}
				if eventMsg.UEventID == 1001 {
					if eventMsg.DwData == 0 {
						fmt.Println("  🛑 Simulator SIM STOPPED")
					} else {
						fmt.Println("  🏁 Simulator SIM STARTED")
					}
				}
				if eventMsg.UEventID == 1002 {
					if eventMsg.DwData == 0 {
						fmt.Println("  🔇 Simulator SOUND OFF")
					} else {
						fmt.Println("  🔊 Simulator SOUND ON")
					}
				}
				// Add more cases here for other message types as needed
			case types.SIMCONNECT_RECV_ID_OPEN:
				fmt.Println("🟢 Connection ready (SIMCONNECT_RECV_ID_OPEN received)")
				msg := msg.AsOpen()
				fmt.Println("📡 Received SIMCONNECT_RECV_OPEN message!")
				fmt.Printf("  Application Name: '%s'\n", engine.BytesToString(msg.SzApplicationName[:]))
				fmt.Printf("  Application Version: %d.%d\n", msg.DwApplicationVersionMajor, msg.DwApplicationVersionMinor)
				fmt.Printf("  Application Build: %d.%d\n", msg.DwApplicationBuildMajor, msg.DwApplicationBuildMinor)
				fmt.Printf("  SimConnect Version: %d.%d\n", msg.DwSimConnectVersionMajor, msg.DwSimConnectVersionMinor)
				fmt.Printf("  SimConnect Build: %d.%d\n", msg.DwSimConnectBuildMajor, msg.DwSimConnectBuildMinor)
			case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
				fmt.Println("  => Received SimObject data event")
				simObjData := msg.AsSimObjectData()
				fmt.Printf("     Request ID: %d, Define ID: %d, Object ID: %d, Flags: %d, Out of: %d, DefineCount: %d\n",
					simObjData.DwRequestID,
					simObjData.DwDefineID,
					simObjData.DwObjectID,
					simObjData.DwFlags,
					simObjData.DwOutOf,
					simObjData.DwDefineCount,
				)
				// Cast the data pointer to CameraData struct
				// The DwData field is the start of the actual data block
				cameraData := (*CameraData)(unsafe.Pointer(&simObjData.DwData))
				fmt.Printf("     Camera State: %d, Camera Substate: %d\n",
					cameraData.CameraState,
					cameraData.CameraSubstate,
				)
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
