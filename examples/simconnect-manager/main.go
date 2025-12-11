//go:build windows
// +build windows

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/manager"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// CameraData represents the data structure for CAMERA STATE and CAMERA SUBSTATE
// The fields must match the order of AddToDataDefinition calls
type CameraData struct {
	CameraState    int32
	CameraSubstate int32
	Category       [260]byte // String260
}

// AircraftData represents the data structure for aircraft information
type AircraftData struct {
	Title             [128]byte
	LiveryName        [128]byte
	LiveryFolder      [128]byte
	Lat               float64
	Lon               float64
	Alt               float64
	Head              float64
	HeadMag           float64
	Vs                float64
	Pitch             float64
	Bank              float64
	GroundSpeed       float64
	AirspeedIndicated float64
	AirspeedTrue      float64
	OnAnyRunway       int32
	SurfaceType       int32
	SimOnGround       int32
	AtcID             [32]byte
	AtcAirline        [32]byte
}

// setupDataDefinitions registers all event subscriptions and data definitions
// when the connection becomes available
func setupDataDefinitions(client engine.Client) {
	fmt.Println("✅ Setting up data definitions and event subscriptions...")

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
	client.AddToDataDefinition(2000, "CAMERA STATE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 0)
	client.AddToDataDefinition(2000, "CAMERA SUBSTATE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 1)
	client.AddToDataDefinition(2000, "CATEGORY", "", types.SIMCONNECT_DATATYPE_STRING260, 0, 2)

	client.RequestDataOnSimObject(2001, 2000, types.SIMCONNECT_OBJECT_ID_USER, types.SIMCONNECT_PERIOD_SECOND, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)

	client.AddToDataDefinition(3000, "TITLE", "", types.SIMCONNECT_DATATYPE_STRING128, 0, 0)
	client.AddToDataDefinition(3000, "LIVERY NAME", "", types.SIMCONNECT_DATATYPE_STRING128, 0, 1)
	client.AddToDataDefinition(3000, "LIVERY FOLDER", "", types.SIMCONNECT_DATATYPE_STRING128, 0, 2)
	client.AddToDataDefinition(3000, "PLANE LATITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 3)
	client.AddToDataDefinition(3000, "PLANE LONGITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 4)
	client.AddToDataDefinition(3000, "PLANE ALTITUDE", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 5)
	client.AddToDataDefinition(3000, "PLANE HEADING DEGREES TRUE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 6)
	client.AddToDataDefinition(3000, "PLANE HEADING DEGREES MAGNETIC", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 7)
	client.AddToDataDefinition(3000, "VERTICAL SPEED", "feet per second", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 8)
	client.AddToDataDefinition(3000, "PLANE PITCH DEGREES", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 9)
	client.AddToDataDefinition(3000, "PLANE BANK DEGREES", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 10)
	client.AddToDataDefinition(3000, "GROUND VELOCITY", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 11)
	client.AddToDataDefinition(3000, "AIRSPEED INDICATED", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 12)
	client.AddToDataDefinition(3000, "AIRSPEED TRUE", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 13)
	client.AddToDataDefinition(3000, "ON ANY RUNWAY", "bool", types.SIMCONNECT_DATATYPE_INT32, 0, 14)
	client.AddToDataDefinition(3000, "SURFACE TYPE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 15)
	client.AddToDataDefinition(3000, "SIM ON GROUND", "bool", types.SIMCONNECT_DATATYPE_INT32, 0, 16)
	client.AddToDataDefinition(3000, "ATC ID", "", types.SIMCONNECT_DATATYPE_STRING32, 0, 17)
	client.AddToDataDefinition(3000, "ATC AIRLINE", "", types.SIMCONNECT_DATATYPE_STRING32, 0, 18)

	// Request data for all aircraft within 10km radius
	client.RequestDataOnSimObjectType(4001, 3000, 10000, types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT)
}

// handleMessage processes incoming messages from the simulator
func handleMessage(msg engine.Message) {
	if msg.Err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error: %v\n", msg.Err)
		return
	}

	fmt.Println("📨 Message received - ", types.SIMCONNECT_RECV_ID(msg.SIMCONNECT_RECV.DwID))

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
	case types.SIMCONNECT_RECV_ID_OPEN:
		fmt.Println("🟢 Connection ready (SIMCONNECT_RECV_ID_OPEN received)")
		openMsg := msg.AsOpen()
		fmt.Println("📡 Received SIMCONNECT_RECV_OPEN message!")
		fmt.Printf("  Application Name: '%s'\n", engine.BytesToString(openMsg.SzApplicationName[:]))
		fmt.Printf("  Application Version: %d.%d\n", openMsg.DwApplicationVersionMajor, openMsg.DwApplicationVersionMinor)
		fmt.Printf("  Application Build: %d.%d\n", openMsg.DwApplicationBuildMajor, openMsg.DwApplicationBuildMinor)
		fmt.Printf("  SimConnect Version: %d.%d\n", openMsg.DwSimConnectVersionMajor, openMsg.DwSimConnectVersionMinor)
		fmt.Printf("  SimConnect Build: %d.%d\n", openMsg.DwSimConnectBuildMajor, openMsg.DwSimConnectBuildMinor)
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
		if simObjData.DwDefineID == 2000 {
			cameraData := engine.CastDataAs[CameraData](&simObjData.DwData)
			fmt.Printf("     Camera State: %d, Camera Substate: %d, Category: %s \n",
				cameraData.CameraState,
				cameraData.CameraSubstate,
				cameraData.Category,
			)
		}
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
			fmt.Printf("     Aircraft Title: %s, Livery Name: %s, Livery Folder: %s, Lat: %f, Lon: %f, Alt: %f, Head: %f, HeadMag: %f, VS: %f, Pitch: %f, Bank: %f, GroundSpeed: %f, AirspeedIndicated: %f, AirspeedTrue: %f, OnAnyRunway: %d, SurfaceType: %d, SimOnGround: %d, AtcID: %s\n",
				engine.BytesToString(aircraftData.Title[:]),
				engine.BytesToString(aircraftData.LiveryName[:]),
				engine.BytesToString(aircraftData.LiveryFolder[:]),
				aircraftData.Lat,
				aircraftData.Lon,
				aircraftData.Alt,
				aircraftData.Head,
				aircraftData.HeadMag,
				aircraftData.Vs,
				aircraftData.Pitch,
				aircraftData.Bank,
				aircraftData.GroundSpeed,
				aircraftData.AirspeedIndicated,
				aircraftData.AirspeedTrue,
				aircraftData.OnAnyRunway,
				aircraftData.SurfaceType,
				aircraftData.SimOnGround,
				engine.BytesToString(aircraftData.AtcID[:]),
			)
		}
	default:
		// Other message types can be handled here
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

	// Create the manager with automatic reconnection
	mgr := manager.New("GO Example - SimConnect Manager",
		manager.WithContext(ctx),
		manager.WithAutoReconnect(true),
	)

	// Register state change handler to setup data definitions when available
	mgr.OnStateChange(func(oldState, newState manager.ConnectionState) {
		fmt.Printf("🔄 State changed: %s -> %s\n", oldState, newState)

		switch newState {
		case manager.StateConnecting:
			fmt.Println("⏳ Connecting to simulator...")
		case manager.StateConnected:
			fmt.Println("✅ Connected to SimConnect, simulator is loading...")
			// Connection is ready - setup data definitions
			if client := mgr.Client(); client != nil {
				setupDataDefinitions(client)
			}
		case manager.StateAvailable:
			// Connection is fully available so messages can be processed
			fmt.Println("🚀 Simulator connection is AVAILABLE. Ready to process messages...")
		case manager.StateReconnecting:
			fmt.Println("🔄 Reconnecting to simulator...")
		case manager.StateDisconnected:
			fmt.Println("📴 Disconnected from simulator...")
		}
	})

	// Register message handler for processing events and data
	mgr.OnMessage(handleMessage)

	// Start the manager - this blocks until context is cancelled
	// The manager handles connection lifecycle automatically
	if err := mgr.Start(); err != nil {
		fmt.Printf("⚠️  Manager stopped: %v\n", err)
	}
}
