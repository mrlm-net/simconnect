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
	CAMERA_COCKPIT    = 2
	CAMERA_EXTERNAL   = 3
	CAMERA_CHASE      = 4
	CAMERA_TOWER      = 5
	CAMERA_DRONE      = 6
	CAMERA_FIXED      = 7
	CAMERA_SMART      = 8
	CAMERA_SHOWCASE   = 9
	CAMERA_INSTRUMENT = 10
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
	// Request data on sim object (user aircraft) with visual frame period for real-time updates
	err := simClient.RequestDataOnSimObject(
		CAMERA_STATE_REQUEST,                       // Request ID
		CAMERA_STATE_DEFINITION,                    // Definition ID
		int(types.SIMCONNECT_SIMOBJECT_TYPE_USER),  // Object ID (user aircraft)
		types.SIMCONNECT_PERIOD_VISUAL_FRAME,       // Period (every visual frame)
		types.SIMCONNECT_DATA_REQUEST_FLAG_CHANGED, // Only when changed
		0, // Origin
		0, // Interval
		0, // Limit
	)
	if err != nil {
		return fmt.Errorf("failed to request camera state data: %v", err)
	}

	return nil
}

// setCameraState changes the camera view in the simulator
func setCameraState(simClient *client.Engine, cameraState int32) error {
	fmt.Printf("Setting camera state to: %d (%s)\n", cameraState, getCameraStateName(cameraState))

	// Create data to send
	data := CameraStateData{CameraState: cameraState}
	dataPtr := uintptr(unsafe.Pointer(&data))

	// Set data on sim object
	err := simClient.SetDataOnSimObject(
		CAMERA_STATE_DEFINITION,                   // Definition ID
		int(types.SIMCONNECT_SIMOBJECT_TYPE_USER), // Object ID (user aircraft)
		types.SIMCONNECT_DATA_SET_FLAG_DEFAULT,    // Flags
		0,                                         // Array count
		int(unsafe.Sizeof(data)),                  // Unit size
		dataPtr,                                   // Data pointer
	)
	if err != nil {
		return fmt.Errorf("failed to set camera state: %v", err)
	}

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
	}

	// Parse the camera state data
	if len(msg.RawData) >= int(simObjectData.DwSize) {
		dataStart := int(unsafe.Sizeof(*simObjectData))
		if len(msg.RawData) >= dataStart+int(unsafe.Sizeof(CameraStateData{})) {
			cameraData := (*CameraStateData)(unsafe.Pointer(&msg.RawData[dataStart]))

			// Only log if camera state changed
			if cameraData.CameraState != *lastCameraState {
				fmt.Printf("Camera state changed: %d (%s)\n",
					cameraData.CameraState,
					getCameraStateName(cameraData.CameraState))
				*lastCameraState = cameraData.CameraState
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
			setCameraState(simClient, CAMERA_COCKPIT)
		case "2":
			setCameraState(simClient, CAMERA_EXTERNAL)
		case "3":
			setCameraState(simClient, CAMERA_CHASE)
		case "4":
			setCameraState(simClient, CAMERA_TOWER)
		case "5":
			setCameraState(simClient, CAMERA_DRONE)
		case "6":
			setCameraState(simClient, CAMERA_FIXED)
		case "7":
			setCameraState(simClient, CAMERA_SMART)
		case "8":
			setCameraState(simClient, CAMERA_SHOWCASE)
		case "9":
			setCameraState(simClient, CAMERA_INSTRUMENT)
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
	fmt.Println("1 - Cockpit view")
	fmt.Println("2 - External view")
	fmt.Println("3 - Chase view")
	fmt.Println("4 - Tower view")
	fmt.Println("5 - Drone camera")
	fmt.Println("6 - Fixed camera")
	fmt.Println("7 - Smart camera")
	fmt.Println("8 - Showcase camera")
	fmt.Println("9 - Instrument camera")
	fmt.Println("h - Show this help")
	fmt.Println("q - Quit")
	fmt.Println("Ctrl+C - Graceful shutdown")
	fmt.Println("===============================")
}

// getCameraStateName returns a human-readable name for camera state values
func getCameraStateName(state int32) string {
	switch state {
	case CAMERA_COCKPIT:
		return "Cockpit"
	case CAMERA_EXTERNAL:
		return "External"
	case CAMERA_CHASE:
		return "Chase"
	case CAMERA_TOWER:
		return "Tower"
	case CAMERA_DRONE:
		return "Drone"
	case CAMERA_FIXED:
		return "Fixed"
	case CAMERA_SMART:
		return "Smart"
	case CAMERA_SHOWCASE:
		return "Showcase"
	case CAMERA_INSTRUMENT:
		return "Instrument"
	default:
		return fmt.Sprintf("Unknown (%d)", state)
	}
}
