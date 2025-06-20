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
	EXTERNAL_POWER_DEFINITION = 1

	// Request IDs
	EXTERNAL_POWER_REQUEST = 1

	// Event IDs
	EVENT_TOGGLE_EXTERNAL_POWER = 1

	// Group IDs
	GROUP_EXTERNAL_POWER = 1
)

// ExternalPowerData represents the external power state simvar data
type ExternalPowerData struct {
	ExternalPowerOn int32 // Bool represented as int32
}

func main() {
	fmt.Println("SimConnect External Power Controller")
	fmt.Println("===================================")
	fmt.Println("This demo monitors EXTERNAL POWER ON state and allows toggling via TOGGLE_EXTERNAL_POWER event")
	fmt.Println()

	// Create a new SimConnect client
	simClient := client.New("ExternalPowerController")
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

	// Set up external power data definition
	err = setupExternalPowerDefinition(simClient)
	if err != nil {
		log.Fatalf("Failed to setup external power definition: %v", err)
	}

	// Set up external power toggle event
	err = setupExternalPowerEvent(simClient)
	if err != nil {
		log.Fatalf("Failed to setup external power event: %v", err)
	}

	// Request initial external power state data
	err = requestExternalPowerData(simClient)
	if err != nil {
		log.Fatalf("Failed to request external power data: %v", err)
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

	// Start keyboard input handler for external power control
	go keyboardInputHandler(simClient)

	// Print usage instructions
	printUsageInstructions()

	// Main message processing loop
	fmt.Println("Starting message processing loop...")
	messageStream := simClient.Stream()

	lastExternalPowerState := -1 // Track last known state to avoid spam

	for {
		select {
		case <-done:
			fmt.Println("Shutting down...")
			return

		case msg := <-messageStream:
			if msg.Error != nil {
				fmt.Printf("Message error: %v\n", msg.Error)
				continue
			}

			// Handle different message types
			switch {
			case msg.IsSimObjectData():
				handleExternalPowerData(msg, &lastExternalPowerState)

			case msg.IsException():
				if exception, ok := msg.GetException(); ok {
					fmt.Printf("SimConnect Exception: %v\n", exception)
				}

			case msg.IsOpen():
				fmt.Println("SimConnect connection confirmed")

			case msg.IsQuit():
				fmt.Println("SimConnect quit received")
				done <- true
				return
			}

		case <-time.After(5 * time.Second):
			// Request fresh data every 5 seconds
			requestExternalPowerData(simClient)
		}
	}
}

// setupExternalPowerDefinition sets up the data definition for external power state
func setupExternalPowerDefinition(simClient *client.Engine) error {
	fmt.Println("Setting up external power data definition...")

	// Add EXTERNAL POWER ON simvar to data definition
	err := simClient.AddToDataDefinition(
		EXTERNAL_POWER_DEFINITION,
		"EXTERNAL POWER ON:1", // External power source 1
		"Bool",
		types.SIMCONNECT_DATATYPE_INT32,
		0.0,
		0,
	)
	if err != nil {
		return fmt.Errorf("failed to add EXTERNAL POWER ON to data definition: %v", err)
	}

	fmt.Println("External power data definition setup complete")
	return nil
}

// setupExternalPowerEvent sets up the event for toggling external power
func setupExternalPowerEvent(simClient *client.Engine) error {
	fmt.Println("Setting up external power toggle event...")

	// Map client event to SimConnect event
	err := simClient.MapClientEventToSimEvent(EVENT_TOGGLE_EXTERNAL_POWER, "TOGGLE_EXTERNAL_POWER")
	if err != nil {
		return fmt.Errorf("failed to map TOGGLE_EXTERNAL_POWER event: %v", err)
	}

	// Add event to notification group
	err = simClient.AddClientEventToNotificationGroup(GROUP_EXTERNAL_POWER, EVENT_TOGGLE_EXTERNAL_POWER)
	if err != nil {
		return fmt.Errorf("failed to add event to notification group: %v", err)
	}

	// Set notification group priority
	err = simClient.SetNotificationGroupPriority(GROUP_EXTERNAL_POWER, 1000) // High priority
	if err != nil {
		return fmt.Errorf("failed to set notification group priority: %v", err)
	}

	fmt.Println("External power event setup complete")
	return nil
}

// requestExternalPowerData requests external power state data from SimConnect
func requestExternalPowerData(simClient *client.Engine) error {
	return simClient.RequestDataOnSimObject(
		EXTERNAL_POWER_REQUEST,
		EXTERNAL_POWER_DEFINITION,
		0, // User aircraft
		types.SIMCONNECT_PERIOD_ONCE,
		types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT,
		0, // Origin
		0, // Interval
		0, // Limit
	)
}

// handleExternalPowerData processes external power state data messages
func handleExternalPowerData(msg client.ParsedMessage, lastState *int) {
	if data, ok := msg.GetSimObjectData(); ok {
		if data.DwDefineID == EXTERNAL_POWER_DEFINITION {
			// Parse the external power data
			externalPowerData := (*ExternalPowerData)(unsafe.Pointer(&data.DwData))

			currentState := int(externalPowerData.ExternalPowerOn)

			// Only print if state changed to avoid spam
			if currentState != *lastState {
				*lastState = currentState
				if currentState != 0 {
					fmt.Printf("External Power: ON\n")
				} else {
					fmt.Printf("External Power: OFF\n")
				}
			}
		}
	}
}

// keyboardInputHandler handles keyboard input for controlling external power
func keyboardInputHandler(simClient *client.Engine) {
	var input string
	for {
		fmt.Scanln(&input)
		switch input {
		case "t", "T":
			toggleExternalPower(simClient)
		case "q", "Q":
			fmt.Println("Quit requested via keyboard")
			os.Exit(0)
		default:
			fmt.Println("Unknown command. Use 'T' to toggle, 'Q' to quit.")
		}
	}
}

// toggleExternalPower sends the toggle external power event
func toggleExternalPower(simClient *client.Engine) {
	fmt.Println("Toggling external power...")
	err := simClient.TransmitClientEvent(
		0, // User aircraft
		EVENT_TOGGLE_EXTERNAL_POWER,
		1, // Parameter (external power source 1)
		GROUP_EXTERNAL_POWER,
	)
	if err != nil {
		fmt.Printf("Failed to toggle external power: %v\n", err)
	}
}

// printUsageInstructions prints usage instructions for the user
func printUsageInstructions() {
	fmt.Println("\n=== USAGE INSTRUCTIONS ===")
	fmt.Println("T or t : Toggle external power")
	fmt.Println("Q or q : Quit application")
	fmt.Println("Ctrl+C : Graceful shutdown")
	fmt.Println("==========================")
	fmt.Println()
	fmt.Println("Current external power state will be displayed when it changes.")
	fmt.Printf("Type your command and press Enter: ")
}
