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

func main() {
	fmt.Println("SimConnect Message Parsing Example with Graceful Shutdown")
	fmt.Println("Features demonstrated:")
	fmt.Println("  - Signal handling (Ctrl+C)")
	fmt.Println("  - Graceful shutdown on timeout")
	fmt.Println("  - Proper resource cleanup")
	fmt.Println("  - SimConnect quit message handling")
	fmt.Println()

	// Create a new SimConnect client
	simClient := client.New("ExampleApp")
	if simClient == nil {
		log.Fatal("Failed to create SimConnect client")
	}

	// Connect to SimConnect
	err := simClient.Connect()
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
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

	// Start message streaming
	messageStream := simClient.Stream()

	// Set up a timeout for the example (30 seconds)
	timeout := time.After(30 * time.Second)

	fmt.Println("Listening for SimConnect messages...")
	fmt.Println("Press Ctrl+C to stop gracefully")

	// Message processing loop with graceful shutdown
	for {
		select {
		case msg, ok := <-messageStream:
			if !ok {
				// Channel was closed, connection terminated
				fmt.Println("Message stream closed, connection terminated")
				return
			}
			// Check if we should exit after handling this message
			if shouldExit := handleMessage(msg, simClient); shouldExit {
				fmt.Println("Received quit message, shutting down gracefully...")
				if err := simClient.Shutdown(); err != nil {
					log.Printf("Error during shutdown: %v", err)
				}
				return
			}

		case <-timeout:
			fmt.Println("Example timeout reached, shutting down gracefully...")
			if err := simClient.Shutdown(); err != nil {
				log.Printf("Error during shutdown: %v", err)
			}
			return

		case <-done:
			fmt.Println("Shutting down gracefully...")
			if err := simClient.Shutdown(); err != nil {
				log.Printf("Error during shutdown: %v", err)
			}
			return
		}
	}
}

// handleMessage demonstrates how to handle different types of parsed messages
// Returns true if the application should exit (e.g., on quit message)
func handleMessage(msg client.ParsedMessage, simClient *client.Engine) bool {
	// Check for parsing errors first
	if msg.Error != nil {
		log.Printf("Message parsing error: %v", msg.Error)
		return false
	}

	// Log basic message info
	fmt.Printf("Received message type: %v, size: %d bytes\n", msg.MessageType, len(msg.RawData))

	// Handle different message types using the convenience methods
	switch {
	case msg.IsOpen():
		fmt.Println("✅ SimConnect connection established successfully!")

		// Extract and display detailed open message information
		if openData, ok := msg.GetOpen(); ok {
			// Convert the application name from byte array to string
			appName := string(openData.SzApplicationName[:])
			// Find the null terminator and trim the string
			if nullIndex := findNullTerminator(openData.SzApplicationName[:]); nullIndex >= 0 {
				appName = string(openData.SzApplicationName[:nullIndex])
			}

			fmt.Printf("   📋 Connection Details:\n")
			fmt.Printf("      Application Name: %s\n", appName)
			fmt.Printf("      Application Version: %d.%d\n",
				openData.DwApplicationVersionMajor, openData.DwApplicationVersionMinor)
			fmt.Printf("      Application Build: %d.%d\n",
				openData.DwApplicationBuildMajor, openData.DwApplicationBuildMinor)
			fmt.Printf("      SimConnect Version: %d.%d\n",
				openData.DwSimConnectVersionMajor, openData.DwSimConnectVersionMinor)
			fmt.Printf("      SimConnect Build: %d.%d\n",
				openData.DwSimConnectBuildMajor, openData.DwSimConnectBuildMinor)
			fmt.Printf("      Reserved1: %d, Reserved2: %d\n",
				openData.DwReserved1, openData.DwReserved2)
		}

		// Request all system states for testing
		fmt.Println("   🔍 Requesting system states...")
		requestSystemStates(simClient)
	case msg.IsQuit():
		fmt.Println("❌ SimConnect connection closed")

		// Extract and display detailed quit message information
		if quitData, ok := msg.GetQuit(); ok {
			fmt.Printf("   📋 Disconnection Details:\n")
			fmt.Printf("      Message Size: %d bytes\n", quitData.DwSize)
			fmt.Printf("      Message Version: %d\n", quitData.DwVersion)
			fmt.Printf("      Message ID: %d (SIMCONNECT_RECV_ID_QUIT)\n", quitData.DwID)
			fmt.Printf("      Note: This indicates SimConnect is shutting down or the connection was terminated\n")
		}
		// Quit message indicates SimConnect is shutting down, return gracefully
		fmt.Println("SimConnect indicated shutdown, exiting gracefully...")
		return true

	case msg.IsException():
		if exception, ok := msg.GetException(); ok {
			fmt.Printf("⚠️  SimConnect Exception: Code=%d, SendID=%d, Index=%d\n",
				exception.DwException, exception.DwSendID, exception.DwIndex)
		}

	case msg.IsEvent():
		if event, ok := msg.GetEvent(); ok {
			fmt.Printf("🎯 Event received: GroupID=%d, EventID=%d, Data=%d\n",
				event.UGroupID, event.UEventID, event.DwData)
		}

	case msg.IsSimObjectData():
		if simData, ok := msg.GetSimObjectData(); ok {
			fmt.Printf("📊 SimObject Data: RequestID=%d, ObjectID=%d, DefineID=%d, Flags=%d\n",
				simData.DwRequestID, simData.DwObjectID, simData.DwDefineID, simData.DwFlags)

			// You can extract the actual data here based on your data definition
			// The data starts at the DwData field and continues for DwDefineCount elements
			handleSimObjectData(simData, msg.RawData)
		}

	default:
		// Handle other message types
		switch msg.MessageType {
		case types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID:
			if assignedID, ok := msg.Data.(*types.SIMCONNECT_RECV_ASSIGNED_OBJECT_ID); ok {
				fmt.Printf("🆔 Assigned Object ID: ObjectID=%d, RequestID=%d\n",
					assignedID.DwObjectID, assignedID.DwRequestID)
			}

		case types.SIMCONNECT_RECV_ID_SYSTEM_STATE:
			if sysState, ok := msg.Data.(*types.SIMCONNECT_RECV_SYSTEM_STATE); ok {
				handleSystemStateResponse(sysState)
			}
		default:
			fmt.Printf("📦 Unhandled message type: %v\n", msg.MessageType)
		}
	}

	return false // Continue processing messages
}

// findNullTerminator finds the index of the first null byte in a byte slice
func findNullTerminator(data []byte) int {
	for i, b := range data {
		if b == 0 {
			return i
		}
	}
	return -1
}

// handleSimObjectData demonstrates how to extract actual flight data
func handleSimObjectData(simData *types.SIMCONNECT_RECV_SIMOBJECT_DATA, rawData []byte) {
	// Calculate the offset to the actual data
	headerSize := uint32(unsafe.Sizeof(*simData))

	if uint32(len(rawData)) <= headerSize {
		fmt.Println("   No data payload")
		return
	}

	// The actual data starts after the header
	dataPayload := rawData[headerSize:]

	fmt.Printf("   Data payload size: %d bytes (%d elements)\n",
		len(dataPayload), simData.DwDefineCount)

	// Example: If you know your data definition contains doubles (8 bytes each)
	// You would parse them like this:
	if len(dataPayload) >= 8 && simData.DwDefineCount > 0 {
		// Convert first 8 bytes to float64 (example for altitude, speed, etc.)
		value := *(*float64)(unsafe.Pointer(&dataPayload[0]))
		fmt.Printf("   First data element (as float64): %f\n", value)
	}

	// For multiple data elements, you would iterate based on your data definition:
	// for i := uint32(0); i < simData.DwDefineCount; i++ {
	//     offset := i * 8 // Assuming 8-byte doubles
	//     if offset+8 <= uint32(len(dataPayload)) {
	//         value := *(*float64)(unsafe.Pointer(&dataPayload[offset]))
	//         fmt.Printf("   Data element %d: %f\n", i, value)
	//     }
	// }
}

// handleSystemStateResponse processes and displays system state responses
func handleSystemStateResponse(sysState *types.SIMCONNECT_RECV_SYSTEM_STATE) {
	// Validate request ID range
	if sysState.DwRequestID < 100 || sysState.DwRequestID > 104 {
		fmt.Printf("🖥️  System State Response (Invalid Request ID):\n")
		fmt.Printf("      Request ID: %d (CORRUPTED - Expected 100-104)\n", sysState.DwRequestID)
		fmt.Printf("      Raw Integer: %d\n", sysState.DwInteger)
		fmt.Printf("      Raw Float bytes: 0x%08X\n", sysState.DwFloat)
		return
	}

	// Map request IDs to their respective state names
	var stateName string
	var isStringState bool
	var isIntegerState bool

	switch sysState.DwRequestID {
	case 100:
		stateName = "Aircraft Loaded"
		isStringState = true
	case 101:
		stateName = "Dialog Mode"
		isIntegerState = true
	case 102:
		stateName = "Flight Loaded"
		isStringState = true
	case 103:
		stateName = "Flight Plan"
		isStringState = true
	case 104:
		stateName = "Sim State"
		isIntegerState = true
	}

	fmt.Printf("🖥️  System State Response:\n")
	fmt.Printf("      Request ID: %d\n", sysState.DwRequestID)
	fmt.Printf("      State Type: %s\n", stateName)

	// Handle string-based states (file paths)
	if isStringState {
		stringValue := ""
		if nullIndex := findNullTerminator(sysState.SzString[:]); nullIndex >= 0 {
			stringValue = string(sysState.SzString[:nullIndex])
		}

		fmt.Printf("      String Value: \"%s\"\n", stringValue)

		if stringValue != "" {
			fmt.Printf("      📁 File Path: %s\n", stringValue)
		} else {
			fmt.Printf("      📁 No file currently loaded\n")
		}
	}

	// Handle integer-based states (flags)
	if isIntegerState {
		fmt.Printf("      Integer Value: %d\n", sysState.DwInteger)

		switch sysState.DwRequestID {
		case 101: // Dialog mode
			if sysState.DwInteger == 1 {
				fmt.Printf("      💬 Simulation is in Dialog Mode\n")
			} else {
				fmt.Printf("      🎮 Simulation is not in Dialog Mode\n")
			}
		case 104: // Sim state
			if sysState.DwInteger == 1 {
				fmt.Printf("      🎮 User is in control of simulation\n")
			} else {
				fmt.Printf("      🖱️  User is navigating UI\n")
			}
		}
	}

	fmt.Println()
}

// requestSystemStates requests all available system states for testing
func requestSystemStates(client *client.Engine) {
	fmt.Println("      🔍 Requesting Aircraft Loaded state...")
	if err := client.RequestSystemStateAircraftLoaded(100); err != nil {
		fmt.Printf("      ❌ Failed to request aircraft loaded state: %v\n", err)
	}
	time.Sleep(10 * time.Millisecond) // Small delay between requests

	fmt.Println("      🔍 Requesting Dialog Mode state...")
	if err := client.RequestSystemStateDialogMode(101); err != nil {
		fmt.Printf("      ❌ Failed to request dialog mode state: %v\n", err)
	}
	time.Sleep(10 * time.Millisecond)

	fmt.Println("      🔍 Requesting Flight Loaded state...")
	if err := client.RequestSystemStateFlightLoaded(102); err != nil {
		fmt.Printf("      ❌ Failed to request flight loaded state: %v\n", err)
	}
	time.Sleep(10 * time.Millisecond)

	fmt.Println("      🔍 Requesting Flight Plan state...")
	if err := client.RequestSystemStateFlightPlan(103); err != nil {
		fmt.Printf("      ❌ Failed to request flight plan state: %v\n", err)
	}
	time.Sleep(10 * time.Millisecond)

	fmt.Println("      🔍 Requesting Sim state...")
	if err := client.RequestSystemStateSim(104); err != nil {
		fmt.Printf("      ❌ Failed to request sim state: %v\n", err)
	}
}
