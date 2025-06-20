//go:build windows
// +build windows

package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mrlm-net/simconnect/pkg/client"
)

const (
	// Event IDs
	EVENT_TOGGLE_AIRCRAFT_EXIT = 1

	// Group IDs
	GROUP_AIRCRAFT_DOORS = 1
)

func main() {
	fmt.Println("SimConnect Aircraft Doors Controller")
	fmt.Println("===================================")
	fmt.Println("This demo allows toggling individual aircraft doors using TOGGLE_AIRCRAFT_EXIT event")
	fmt.Println("Documentation: https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/Key_Events/Aircraft_Misc_Events.htm")
	fmt.Println()

	// Create a new SimConnect client
	simClient := client.New("AircraftDoorsController")
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

	// Set up aircraft door toggle event
	err = setupDoorEvent(simClient)
	if err != nil {
		log.Fatalf("Failed to setup door event: %v", err)
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

	// Start keyboard input handler for door control
	go keyboardInputHandler(simClient)

	// Print usage instructions
	printUsageInstructions()

	// Main message processing loop
	fmt.Println("Starting message processing loop...")
	messageStream := simClient.Stream()

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

		case <-time.After(1 * time.Second):
			// Keep connection alive
		}
	}
}

// setupDoorEvent sets up the event for toggling aircraft doors
func setupDoorEvent(simClient *client.Engine) error {
	fmt.Println("Setting up aircraft door toggle event...")

	// Map client event to SimConnect event
	err := simClient.MapClientEventToSimEvent(EVENT_TOGGLE_AIRCRAFT_EXIT, "TOGGLE_AIRCRAFT_EXIT")
	if err != nil {
		return fmt.Errorf("failed to map TOGGLE_AIRCRAFT_EXIT event: %v", err)
	}

	// Add event to notification group
	err = simClient.AddClientEventToNotificationGroup(GROUP_AIRCRAFT_DOORS, EVENT_TOGGLE_AIRCRAFT_EXIT)
	if err != nil {
		return fmt.Errorf("failed to add event to notification group: %v", err)
	}

	// Set notification group priority
	err = simClient.SetNotificationGroupPriority(GROUP_AIRCRAFT_DOORS, 1000) // High priority
	if err != nil {
		return fmt.Errorf("failed to set notification group priority: %v", err)
	}

	fmt.Println("Aircraft door event setup complete")
	return nil
}

// keyboardInputHandler handles keyboard input for controlling aircraft doors
func keyboardInputHandler(simClient *client.Engine) {
	var input string
	for {
		fmt.Printf("Enter command: ")
		fmt.Scanln(&input)

		input = strings.TrimSpace(input)

		switch {
		case input == "q" || input == "Q":
			fmt.Println("Quit requested via keyboard")
			os.Exit(0)

		case input == "h" || input == "H":
			printUsageInstructions()

		case strings.HasPrefix(input, "d") || strings.HasPrefix(input, "D"):
			// Parse door number from input like "d1", "D1", "d 1", etc.
			doorNumStr := strings.TrimSpace(input[1:])
			if doorNumStr == "" {
				fmt.Println("Please specify a door number (e.g., 'd1', 'd2', etc.)")
				continue
			}

			doorNum, err := strconv.Atoi(doorNumStr)
			if err != nil {
				fmt.Printf("Invalid door number '%s'. Please enter a valid number.\n", doorNumStr)
				continue
			}

			if doorNum < 1 || doorNum > 4 {
				fmt.Println("Door number must be between 1 and 4")
				continue
			}

			toggleDoor(simClient, doorNum)

		default:
			if input != "" {
				fmt.Printf("Unknown command '%s'. Use 'H' for help.\n", input)
			}
		}
	}
}

// toggleDoor sends the toggle aircraft exit event for a specific door
func toggleDoor(simClient *client.Engine, doorNumber int) {
	fmt.Printf("Toggling aircraft door #%d...\n", doorNumber)
	err := simClient.TransmitClientEvent(
		0, // User aircraft
		EVENT_TOGGLE_AIRCRAFT_EXIT,
		doorNumber, // Door number parameter
		GROUP_AIRCRAFT_DOORS,
	)

	if err != nil {
		fmt.Printf("Failed to toggle door #%d: %v\n", doorNumber, err)
	} else {
		fmt.Printf("Successfully sent toggle command for door #%d\n", doorNumber)
	}
}

// printUsageInstructions prints usage instructions for the user
func printUsageInstructions() {
	fmt.Println("\n=== AIRCRAFT DOORS CONTROLLER ===")
	fmt.Println("Commands:")
	fmt.Println("  D[1-4]    : Toggle door by number (D1, D2, D3, D4)")
	fmt.Println("  H or h    : Show this help")
	fmt.Println("  Q or q    : Quit application")
	fmt.Println("  Ctrl+C    : Graceful shutdown")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  D1        : Toggle door #1 (front left passenger door)")
	fmt.Println("  D2        : Toggle door #2 (front right passenger door)")
	fmt.Println("  D3        : Toggle door #3 (rear left passenger door)")
	fmt.Println("  D4        : Toggle door #4 (rear right passenger door)")
	fmt.Println("==================================")
	fmt.Println()
	fmt.Println("Note: Since doors don't have readable state variables, this demo only")
	fmt.Println("sends toggle commands. Check the aircraft visually for door states.")
	fmt.Println()
}
