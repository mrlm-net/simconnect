//go:build windows
// +build windows

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

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
	// NOTE: The manager internally subscribes to Pause, Sim, Sound, Crashed, CrashReset,
	// FlightLoaded, AircraftLoaded, FlightPlanActivated, ObjectAdded, and ObjectRemoved events.
	// Do NOT subscribe to these events manually as it will cause SimConnect exceptionID=1.
	// Instead, use the manager's callback handlers: OnSimStateChange, OnCrashed, OnCrashReset, OnSoundEvent.
	// --------------------------------------------
	// - Pause event occurs when user pauses/unpauses the simulator.
	//   State is returned in dwData field as number (0=unpaused, 1=paused)
	//client.SubscribeToSystemEvent(1000, "Pause")
	// --------------------------------------------
	// - Sim event occurs when simulator starts/stops.
	//   State is returned in dwData field as number (0=stopped, 1=started)
	//client.SubscribeToSystemEvent(1001, "Sim")
	// --------------------------------------------
	// - Sound event occurs when simulator master sound is toggled.
	//   State is returned in dwData field as number (0=off, 1=on)
	//client.SubscribeToSystemEvent(1002, "Sound")
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

	// Skip printing message received for SIMCONNECT_RECV_ID_SIMOBJECT_DATA with manager's internal request IDs
	// Manager uses 999000/999001 for camera data, so filter those out to avoid clutter
	shouldPrintMessage := true
	if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA {
		simObjData := msg.AsSimObjectData()
		if simObjData != nil && ((simObjData.DwDefineID == types.DWORD(manager.CameraDefinitionID) && simObjData.DwRequestID == types.DWORD(manager.CameraRequestID)) || (simObjData.DwDefineID == 2000 && simObjData.DwRequestID == 2001)) {
			shouldPrintMessage = false
		}
	}

	if shouldPrintMessage {
		//fmt.Println("📨 Message received - ", types.SIMCONNECT_RECV_ID(msg.SIMCONNECT_RECV.DwID))
	}

	// Handle specific messages
	// This could be done based on type and also if needed request IDs
	// NOTE: System events (Pause, Sim, Sound, Crashed, CrashReset, etc.) are handled internally
	// by the manager and delivered via OnSimStateChange, OnCrashed, OnCrashReset, OnSoundEvent callbacks.
	switch types.SIMCONNECT_RECV_ID(msg.DwID) {
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
		simObjData := msg.AsSimObjectData()
		// Skip printing for manager's internal camera data (999000/999001) and DID 2000/RID 2001
		isFilteredMessage := (simObjData.DwDefineID == types.DWORD(manager.CameraDefinitionID) && simObjData.DwRequestID == types.DWORD(manager.CameraRequestID)) || (simObjData.DwDefineID == 2000 && simObjData.DwRequestID == 2001)
		if !isFilteredMessage {
			fmt.Println("  => Received SimObject data event")
			fmt.Printf("     Request ID: %d, Define ID: %d, Object ID: %d, Flags: %d, Out of: %d, DefineCount: %d\n",
				simObjData.DwRequestID,
				simObjData.DwDefineID,
				simObjData.DwObjectID,
				simObjData.DwFlags,
				simObjData.DwOutOf,
				simObjData.DwDefineCount,
			)
		}
		// Cast the data pointer to CameraData struct
		// The DwData field is the start of the actual data block
		if simObjData.DwDefineID == 2000 && simObjData.DwRequestID != 2001 {
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

	fmt.Println("ℹ️  (Press Ctrl+C to exit)")

	// Create the manager with automatic reconnection
	// Demonstrates convenience options for common engine settings:
	// - WithBufferSize: sets the message buffer size (default: 256)
	// - WithHeartbeat: sets the heartbeat frequency (default: "6Hz")
	// - WithDLLPath: sets custom SimConnect DLL path if needed
	// Note: WithContext and WithLogger on manager take precedence over
	// any context/logger passed via WithEngineOptions.
	mgr := manager.New("GO Example - SimConnect Subscribe",
		manager.WithContext(ctx),
		manager.WithAutoReconnect(true),
		manager.WithBufferSize(512),  // Optional: increase buffer for high-frequency data
		manager.WithHeartbeat("6Hz"), // Optional: set heartbeat frequency
		// Optional: set SimState update frequency (default: every sim frame)
		// Use types.SIMCONNECT_PERIOD_SECOND for lower CPU usage (1Hz updates)
		// manager.WithSimStatePeriod(types.SIMCONNECT_PERIOD_SECOND),
	)

	// Setup signal handler goroutine - calls mgr.Stop() for graceful shutdown
	go func() {
		<-sigChan
		fmt.Println("🛑 Received interrupt signal, shutting down...")
		mgr.Stop() // Gracefully stop manager (handles shutdown timeout internally)
		cancel()
	}()

	// Register SimState change handler to monitor significant discrete state changes
	// NOTE: As of issue #15, this handler only fires on discrete state changes (camera, pause, sim running, VR, realism settings)
	// and NOT on noisy SimVars like time, position, weather, or speed to reduce false positives.
	_ = mgr.OnSimStateChange(func(oldState, newState manager.SimState) {
		oldSimStatus := "Started"
		if !oldState.SimRunning {
			oldSimStatus = "Stopped"
		}
		oldPauseStatus := "▶️  Running"
		if oldState.Paused {
			oldPauseStatus = "⏸️  Paused"
		}
		newSimStatus := "Started"
		if !newState.SimRunning {
			newSimStatus = "Stopped"
		}
		newPauseStatus := "▶️  Running"
		if newState.Paused {
			newPauseStatus = "⏸️  Paused"
		}
		fmt.Printf("🎥 SimState changed (discrete state change detected):\n")
		fmt.Printf("   Old: Camera=%s [%d], Substate=%s, %s, Sim=%s\n",
			oldState.Camera, oldState.Camera, oldState.Substate, oldPauseStatus, oldSimStatus)
		fmt.Printf("   New: Camera=%s [%d], Substate=%s, %s, Sim=%s\n",
			newState.Camera, newState.Camera, newState.Substate, newPauseStatus, newSimStatus)
		fmt.Printf("   Realism: %.2f, VisualModelRadius: %.2f m, SimDisabled: %v\n",
			newState.Realism, newState.VisualModelRadius, newState.SimDisabled)
		fmt.Printf("   CrashDetection: %v, CrashWithOthers: %v, TrackIR: %v, UserInput: %v, OnGround: %v\n",
			newState.RealismCrashDetection, newState.RealismCrashWithOthers, newState.TrackIREnabled, newState.UserInputEnabled, newState.SimOnGround)
	})

	// Register Crashed event handler (added in issue #22)
	_ = mgr.OnCrashed(func() {
		fmt.Println("💥 [OnCrashed Callback] Aircraft crashed!")
	})

	// Register CrashReset event handler (added in issue #22)
	_ = mgr.OnCrashReset(func() {
		fmt.Println("🔄 [OnCrashReset Callback] Crash reset - aircraft restored!")
	})

	// Register Sound event handler (added in issue #22)
	_ = mgr.OnSoundEvent(func(soundID uint32) {
		soundState := "ON"
		if soundID == 0 {
			soundState = "OFF"
		}
		fmt.Printf("🔊 [OnSoundEvent Callback] Master sound toggled: %s (soundID=%d)\n", soundState, soundID)
	})

	// Register connection state change handler to setup data definitions when available
	_ = mgr.OnConnectionStateChange(func(oldState, newState manager.ConnectionState) {
		fmt.Printf("🔄 Connection state changed: %s -> %s\n", oldState, newState)

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
			// Note: SimState will track all state changes independently from this point on
			currentSimState := mgr.SimState()
			simStatus := "Started"
			if !currentSimState.SimRunning {
				simStatus = "Stopped"
			}
			pauseStatus := "▶️  Running"
			if currentSimState.Paused {
				pauseStatus = "⏸️  Paused"
			}
			fmt.Printf("📊 Current SimState:\n")
			fmt.Printf("   Camera: %s [%d] (Substate: %s)\n", currentSimState.Camera, currentSimState.Camera, currentSimState.Substate)
			fmt.Printf("   Status: %s, Sim: %s\n", pauseStatus, simStatus)
			fmt.Printf("   Realism: %.2f, Visual Model Radius: %.2f m\n", currentSimState.Realism, currentSimState.VisualModelRadius)
			fmt.Printf("   Sim Disabled: %v, Crash Detection: %v, Crash with Others: %v\n", currentSimState.SimDisabled, currentSimState.RealismCrashDetection, currentSimState.RealismCrashWithOthers)
			fmt.Printf("   TrackIR: %v, User Input: %v, On Ground: %v\n", currentSimState.TrackIREnabled, currentSimState.UserInputEnabled, currentSimState.SimOnGround)
		case manager.StateReconnecting:
			fmt.Println("🔄 Reconnecting to simulator...")
		case manager.StateDisconnected:
			fmt.Println("📴 Disconnected from simulator...")
		}
	})

	// Register OnOpen handler to capture simulator connection open event with version info
	_ = mgr.OnOpen(func(data types.ConnectionOpenData) {
		fmt.Println("🔓 [OnOpen Callback] Simulator connection opened!")
		fmt.Printf("   App: %s (v%d.%d.%d.%d)\n",
			data.ApplicationName,
			data.ApplicationVersionMajor,
			data.ApplicationVersionMinor,
			data.ApplicationBuildMajor,
			data.ApplicationBuildMinor,
		)
		fmt.Printf("   SimConnect: v%d.%d.%d.%d\n",
			data.SimConnectVersionMajor,
			data.SimConnectVersionMinor,
			data.SimConnectBuildMajor,
			data.SimConnectBuildMinor,
		)
	})

	// Register OnQuit handler to capture simulator quit event
	_ = mgr.OnQuit(func(data types.ConnectionQuitData) {
		fmt.Println("🔒 [OnQuit Callback] Simulator is quitting!")
	})

	// Subscribe to connection state changes via channel
	// This is equivalent to using OnConnectionStateChange but with channel-based consumption
	connStateSub := mgr.SubscribeConnectionStateChange("connection-state-subscriber", 16)

	// Start a goroutine to process connection state changes from the subscription channel
	go func() {
		fmt.Println("📬 Connection state subscription started, waiting for changes...")
		for {
			select {
			case change, ok := <-connStateSub.ConnectionStateChanges():
				if !ok {
					// Channel closed, subscription ended
					fmt.Println("📭 Connection state subscription channel closed")
					return
				}
				// Log connection state changes received via subscription (complementary to callback)
				fmt.Printf("📡 [Connection State Subscription] Changed: %s -> %s\n", change.OldState, change.NewState)
			case <-connStateSub.Done():
				// Subscription was cancelled
				fmt.Println("📭 Connection state subscription cancelled")
				return
			}
		}
	}()

	// Subscribe to connection open events via channel
	// This demonstrates the SubscribeOnOpen pattern for receiving version info on connection
	openSub := mgr.SubscribeOnOpen("open-subscriber", 16)

	// Start a goroutine to process open events from the subscription channel
	go func() {
		fmt.Println("📬 OnOpen subscription started, waiting for connection open event...")
		for {
			select {
			case data, ok := <-openSub.Opens():
				if !ok {
					// Channel closed, subscription ended
					fmt.Println("📭 OnOpen subscription channel closed")
					return
				}
				// Log open event received via subscription
				fmt.Println("📡 [OnOpen Subscription] Simulator connection opened!")
				fmt.Printf("   App: %s (v%d.%d.%d.%d)\n",
					data.ApplicationName,
					data.ApplicationVersionMajor,
					data.ApplicationVersionMinor,
					data.ApplicationBuildMajor,
					data.ApplicationBuildMinor,
				)
				fmt.Printf("   SimConnect: v%d.%d.%d.%d\n",
					data.SimConnectVersionMajor,
					data.SimConnectVersionMinor,
					data.SimConnectBuildMajor,
					data.SimConnectBuildMinor,
				)
			case <-openSub.Done():
				// Subscription was cancelled
				fmt.Println("📭 OnOpen subscription cancelled")
				return
			}
		}
	}()

	// Subscribe to connection quit events via channel
	// This demonstrates the SubscribeOnQuit pattern for detecting simulator quit
	quitSub := mgr.SubscribeOnQuit("quit-subscriber", 16)

	// Start a goroutine to process quit events from the subscription channel
	go func() {
		fmt.Println("📬 OnQuit subscription started, waiting for quit event...")
		for {
			select {
			case _, ok := <-quitSub.Quits():
				if !ok {
					// Channel closed, subscription ended
					fmt.Println("📭 OnQuit subscription channel closed")
					return
				}
				// Log quit event received via subscription
				fmt.Println("📡 [OnQuit Subscription] Simulator is quitting!")
			case <-quitSub.Done():
				// Subscription was cancelled
				fmt.Println("📭 OnQuit subscription cancelled")
				return
			}
		}
	}()

	// Subscribe to simulator state changes via channel
	// This monitors camera state and other simulator state variables
	simStateSub := mgr.SubscribeSimStateChange("sim-state-subscriber", 16)

	// Start a goroutine to process simulator state changes from the subscription channel
	go func() {
		fmt.Println("📬 SimState subscription started, monitoring camera state...")
		for {
			select {
			case change, ok := <-simStateSub.SimStateChanges():
				if !ok {
					// Channel closed, subscription ended
					fmt.Println("📭 SimState subscription channel closed")
					return
				}
				// Log simulator state changes received via subscription
				oldSimStatus := "Started"
				if !change.OldState.SimRunning {
					oldSimStatus = "Stopped"
				}
				oldPauseStatus := "▶️  Running"
				if change.OldState.Paused {
					oldPauseStatus = "⏸️  Paused"
				}
				newSimStatus := "Started"
				if !change.NewState.SimRunning {
					newSimStatus = "Stopped"
				}
				newPauseStatus := "▶️  Running"
				if change.NewState.Paused {
					newPauseStatus = "⏸️  Paused"
				}
				fmt.Printf("📡 [SimState Subscription] State changed:\n")
				fmt.Printf("   Old: Camera=%s [%d], Substate=%s, %s, Sim=%s\n",
					change.OldState.Camera, change.OldState.Camera, change.OldState.Substate, oldPauseStatus, oldSimStatus)
				fmt.Printf("   New: Camera=%s [%d], Substate=%s, %s, Sim=%s\n",
					change.NewState.Camera, change.NewState.Camera, change.NewState.Substate, newPauseStatus, newSimStatus)
				fmt.Printf("   Realism: %.2f, VisualModelRadius: %.2f m, SimDisabled: %v\n",
					change.NewState.Realism, change.NewState.VisualModelRadius, change.NewState.SimDisabled)
				fmt.Printf("   CrashDetection: %v, CrashWithOthers: %v, TrackIR: %v, UserInput: %v, OnGround: %v\n",
					change.NewState.RealismCrashDetection, change.NewState.RealismCrashWithOthers, change.NewState.TrackIREnabled, change.NewState.UserInputEnabled, change.NewState.SimOnGround)
				// Could trigger additional logic here based on camera state
				if change.NewState.Camera == manager.CameraStateExternalChase {
					fmt.Println("   🎥 Now viewing from EXTERNAL/CHASE camera")
				} else if change.NewState.Camera == manager.CameraStateCockpit {
					fmt.Println("   🛩️  Now viewing from COCKPIT camera")
				} else if change.NewState.Camera == manager.CameraStateDrone {
					fmt.Println("   🚁 Now viewing from DRONE camera")
				}
			case <-simStateSub.Done():
				// Subscription was cancelled
				fmt.Println("📭 SimState subscription cancelled")
				return
			}
		}
	}()

	// Create a subscription to receive messages via channel instead of callback
	// This demonstrates the Subscribe pattern for message handling
	// - First parameter is the subscription ID (empty string for auto-generated UUID)
	// - Second parameter is the channel buffer size
	sub := mgr.Subscribe("main-subscriber", 256)

	// Start a goroutine to process messages from the subscription channel
	go func() {
		fmt.Println("📬 Message subscription started, waiting for messages...")
		for {
			select {
			case msg, ok := <-sub.Messages():
				if !ok {
					// Channel closed, subscription ended
					fmt.Println("📭 Subscription channel closed")
					return
				}
				// Process the message using the same handler
				handleMessage(msg)
			case <-sub.Done():
				// Subscription was cancelled
				fmt.Println("📭 Subscription cancelled")
				return
			}
		}
	}()

	// Start the manager - this blocks until context is cancelled
	// The manager handles connection lifecycle automatically
	if err := mgr.Start(); err != nil {
		fmt.Printf("⚠️  Manager stopped: %v\n", err)
	}

	// Unsubscribe when done (cleanup)
	sub.Unsubscribe()
	connStateSub.Unsubscribe()
	openSub.Unsubscribe()
	quitSub.Unsubscribe()
	simStateSub.Unsubscribe()

	// Small delay to allow goroutines to complete cleanup
	time.Sleep(100 * time.Millisecond)
	fmt.Println("👋 Goodbye!")
}
