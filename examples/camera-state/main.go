//go:build windows
// +build windows

package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/client"
	"github.com/mrlm-net/simconnect/pkg/types"
)

const (
	// Data definition IDs
	CAMERA_STATE_DEFINITION = 1

	// Request IDs
	CAMERA_STATE_REQUEST = 1
	// Camera state values (from MSFS documentation)
	// https://docs.flightsimulator.com/html/Programming_Tools/SimVars/Camera_Variables.htm
	CAMERA_COCKPIT        = 2  // Cockpit
	CAMERA_EXTERNAL_CHASE = 3  // External/Chase
	CAMERA_DRONE          = 4  // Drone
	CAMERA_FIXED_ON_PLANE = 5  // Fixed on Plane
	CAMERA_ENVIRONMENT    = 6  // Environment
	CAMERA_SIX_DOF        = 7  // Six DoF
	CAMERA_GAMEPLAY       = 8  // Gameplay
	CAMERA_SHOWCASE       = 9  // Showcase
	CAMERA_DRONE_AIRCRAFT = 10 // Drone Aircraft
)

// CameraStateData represents the camera state simvar data
type CameraStateData struct {
	CameraState int32
}

func main() {
	fmt.Println("SimConnect Camera State Monitor and Control Demo")
	fmt.Println("==============================================")
	fmt.Println("This demo demonstrates:")
	fmt.Println("  - Monitoring CAMERA STATE simvar in real-time")
	fmt.Println("  - Changing camera views programmatically")
	fmt.Println("  - Graceful shutdown handling")
	fmt.Println()

	// Create a new SimConnect client
	simClient := client.New("CameraStateDemo")
	if simClient == nil {
		log.Fatal("Failed to create SimConnect client")
	}

	// Connect to SimConnect
	err := simClient.Connect()
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer simClient.Disconnect()

	fmt.Println("Connected to SimConnect successfully!")

	// Set up camera state data definition
	err = setupCameraStateDefinition(simClient)
	if err != nil {
		log.Fatalf("Failed to setup camera state definition: %v", err)
	}

	// Request initial camera state data
	err = requestCameraStateData(simClient)
	if err != nil {
		log.Fatalf("Failed to request camera state data: %v", err)
	}

	// Set up graceful shutdown on system signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Channel to coordinate shutdown
	done := make(chan bool, 1)

	// Goroutine to handle signals
	go func() {
		sig := <-sigChan
		fmt.Printf("\nReceived %v signal, initiating graceful shutdown...\n", sig)
		done <- true
	}()

	// Start keyboard input handler for camera control
	go keyboardInputHandler(simClient)

	// Print usage instructions
	printUsageInstructions()

	// Main message processing loop
	fmt.Println("Starting message processing loop...")
	fmt.Println("Monitoring camera state changes...")

	messageLoop(simClient, done)

	fmt.Println("Demo completed successfully!")
}

// setupCameraStateDefinition configures the data definition for CAMERA STATE simvar
func setupCameraStateDefinition(simClient *client.Engine) error {
	fmt.Println("Setting up CAMERA STATE data definition...")

	// Add CAMERA STATE to data definition
	err := simClient.AddToDataDefinition(
		CAMERA_STATE_DEFINITION,         // Define ID
		"CAMERA STATE",                  // SimVar name
		"enum",                          // Units (enum type)
		types.SIMCONNECT_DATATYPE_INT32, // Data type
		0.0,                             // Epsilon (not used for integers)
		0,                               // Datum ID
	)
	if err != nil {
		return fmt.Errorf("failed to add CAMERA STATE to data definition: %v", err)
	}

	fmt.Println("CAMERA STATE data definition setup complete!")
	return nil
}

// requestCameraStateData requests camera state data from the simulator
func requestCameraStateData(simClient *client.Engine) error {
	// Request data on sim object (user aircraft) - get updates every second
	err := simClient.RequestDataOnSimObject(
		CAMERA_STATE_REQUEST,                       // Request ID
		CAMERA_STATE_DEFINITION,                    // Definition ID
		int(types.SIMCONNECT_OBJECT_ID_USER),       // Object ID (user aircraft)
		types.SIMCONNECT_PERIOD_SECOND,             // Period (every second)
		types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, // Get data every second
		0, // Origin
		0, // Interval (1 second)
		0, // Limit
	)
	if err != nil {
		return fmt.Errorf("failed to request camera state data: %v", err)
	}

	return nil
}

// setCameraState changes the camera view in the simulator using events
func setCameraState(simClient *client.Engine, cameraState int32) error {
	fmt.Printf("Setting camera state to: %d (%s)\n", cameraState, getCameraStateName(cameraState))

	// Instead of setting data, we should use SimConnect events to change camera
	// For now, let's still try the data approach but add a note that events might be better

	// Create data to send
	data := CameraStateData{CameraState: cameraState}
	dataPtr := uintptr(unsafe.Pointer(&data))

	// Set data on sim object
	err := simClient.SetDataOnSimObject(
		CAMERA_STATE_DEFINITION,                // Definition ID
		int(types.SIMCONNECT_OBJECT_ID_USER),   // Object ID (user aircraft)
		types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, // Flags
		0,                                      // Array count
		int(unsafe.Sizeof(data)),               // Unit size
		dataPtr,                                // Data pointer
	)
	if err != nil {
		return fmt.Errorf("failed to set camera state: %v", err)
	}

	// Note: If this doesn't work, we should switch to using SimConnect events like:
	// - NEXT_SUB_VIEW, PREV_SUB_VIEW
	// - VIEW_COCKPIT_FORWARD, VIEW_EXTERNAL1, etc.

	return nil
}

// messageLoop processes incoming SimConnect messages
func messageLoop(simClient *client.Engine, done <-chan bool) {
	msgStream := simClient.Stream()
	lastCameraState := int32(-1)

	for {
		select {
		case <-done:
			fmt.Println("Shutdown signal received, exiting message loop...")
			return
		case msg := <-msgStream:
			if msg.Error != nil {
				fmt.Printf("Message error: %v\n", msg.Error)
				continue
			}

			// Handle different message types
			switch {
			case msg.IsSimObjectData():
				handleCameraStateData(msg, &lastCameraState)
			case msg.IsOpen():
				fmt.Println("SimConnect connection opened successfully")
			case msg.IsQuit():
				fmt.Println("SimConnect quit message received")
				return
			case msg.IsException():
				if exception, ok := msg.GetException(); ok {
					fmt.Printf("SimConnect exception: %d\n", exception.DwException)
				}
			}

		case <-time.After(30 * time.Second):
			fmt.Println("Timeout: No messages received for 30 seconds, shutting down...")
			return
		}
	}
}

// handleCameraStateData processes camera state data messages
func handleCameraStateData(msg client.ParsedMessage, lastCameraState *int32) {
	simObjectData, ok := msg.GetSimObjectData()
	if !ok {
		return
	}

	// Check if this is our camera state request
	if simObjectData.DwRequestID != CAMERA_STATE_REQUEST {
		return
	} // Read the camera state from offset 40 (where the actual data is located)
	if len(msg.RawData) >= 44 {
		cameraState := *(*int32)(unsafe.Pointer(&msg.RawData[40]))

		// Validate that this looks like a reasonable camera state (1-15 range)
		if cameraState >= 1 && cameraState <= 15 {
			// Log if camera state changed
			if cameraState != *lastCameraState {
				fmt.Printf("Camera state changed: %d (%s)\n",
					cameraState,
					getCameraStateName(cameraState))
				*lastCameraState = cameraState
			}
		}
	}
}

// keyboardInputHandler handles keyboard input for camera control
func keyboardInputHandler(simClient *client.Engine) {
	fmt.Println("Keyboard input handler started (press Enter after each command)")

	for {
		var input string
		fmt.Print("Enter camera command (1-9, 'h' for help, 'q' to quit): ")
		_, err := fmt.Scanln(&input)
		if err != nil {
			continue
		}
		switch input {
		case "1":
			setCameraState(simClient, CAMERA_COCKPIT) // 2
		case "2":
			setCameraState(simClient, CAMERA_EXTERNAL_CHASE) // 3
		case "3":
			setCameraState(simClient, CAMERA_DRONE) // 4
		case "4":
			setCameraState(simClient, CAMERA_FIXED_ON_PLANE) // 5
		case "5":
			setCameraState(simClient, CAMERA_ENVIRONMENT) // 6
		case "6":
			setCameraState(simClient, CAMERA_SIX_DOF) // 7
		case "7":
			setCameraState(simClient, CAMERA_GAMEPLAY) // 8
		case "8":
			setCameraState(simClient, CAMERA_SHOWCASE) // 9
		case "9":
			setCameraState(simClient, CAMERA_DRONE_AIRCRAFT) // 10
		case "h", "H":
			printUsageInstructions()
		case "q", "Q":
			fmt.Println("Quit command received")
			os.Exit(0)
		default:
			fmt.Printf("Unknown command: %s (type 'h' for help)\n", input)
		}
	}
}

// printUsageInstructions displays the available commands
func printUsageInstructions() {
	fmt.Println("\n=== Camera Control Commands ===")
	fmt.Println("1 - Cockpit")
	fmt.Println("2 - External/Chase")
	fmt.Println("3 - Drone")
	fmt.Println("4 - Fixed on Plane")
	fmt.Println("5 - Environment")
	fmt.Println("6 - Six DoF")
	fmt.Println("7 - Gameplay")
	fmt.Println("8 - Showcase")
	fmt.Println("9 - Drone Aircraft")
	fmt.Println("h - Show this help")
	fmt.Println("q - Quit")
	fmt.Println("Ctrl+C - Graceful shutdown")
	fmt.Println("===============================")
}

// getCameraStateName returns a human-readable name for camera state values
// Based on MSFS documentation: https://docs.flightsimulator.com/html/Programming_Tools/SimVars/Camera_Variables.htm
func getCameraStateName(state int32) string {
	switch state {
	case 2:
		return "Cockpit"
	case 3:
		return "External/Chase"
	case 4:
		return "Drone"
	case 5:
		return "Fixed on Plane"
	case 6:
		return "Environment"
	case 7:
		return "Six DoF"
	case 8:
		return "Gameplay"
	case 9:
		return "Showcase"
	case 10:
		return "Drone Aircraft"
	case 11:
		return "Waiting"
	case 12:
		return "World Map"
	case 13:
		return "Hangar RTC"
	case 14:
		return "Hangar Custom"
	case 15:
		return "Menu RTC"
	case 16:
		return "In-Game RTC"
	case 17:
		return "Replay"
	case 19:
		return "Drone Top-Down"
	case 21:
		return "Hangar"
	case 24:
		return "Ground"
	case 25:
		return "Follow Traffic Aircraft"
	default:
		return fmt.Sprintf("Unknown (%d)", state)
	}
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
