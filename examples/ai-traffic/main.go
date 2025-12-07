//go:build windows
// +build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect"
)

type ParkedAircraft struct {
	Airport    string `json:"airport"`
	Plane      string `json:"plane"`
	FlightPlan string `json:"plan,omitempty"`
	Number     string `json:"number"`
}

type IFRAircraft struct {
	Plane      string  `json:"plane"`
	Number     string  `json:"number"`
	FlightPlan string  `json:"plan"`
	InitPhase  float64 `json:"phase"`
}

var wg sync.WaitGroup

func main() {
	execPath, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "❌ Error getting executable path:", err)
		return
	}
	execDir := filepath.Dir(execPath)

	// This is a placeholder main function.
	// The actual implementation would go here.
	client := simconnect.New("GO Example - SimConnect Basic Connection")

	if err := client.Connect(); err != nil {
		fmt.Fprintln(os.Stderr, "❌ Error:", err)
		return
	}
	defer func() {
		if err := client.Disconnect(); err != nil {
			fmt.Fprintln(os.Stderr, "❌ Disconnect error:", err)
			return
		}

		fmt.Println("👋 Disconnected from SimConnect...")

	}()

	// Application logic would go here.
	fmt.Println("✅ Connected to SimConnect...")

	planes, err := loadParkedAircraft("planes-parked.json")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	for i, p := range planes {
		fmt.Printf("Adding parked plane - plane=%s number=%s\n", p.Plane, p.Number)
		client.AICreateParkedATCAircraft(p.Plane, p.Number, p.Airport, uint32(1000+i))

		// Simple per-plane delay (tweak as needed or load from data):
		delay := time.Duration(500*i) * time.Millisecond

		wg.Add(1)
		time.AfterFunc(delay, func(p ParkedAircraft) func() {
			return func() {
				defer wg.Done()
				// Demo: assign a flight plan or any follow-up work here.
				// Replace with real SimConnect call if desired.
				fmt.Printf("📝 Assigning flight plan for %s (%s) after %s\n", p.Plane, p.Number, delay)
				// Example placeholder:
				client.AISetAircraftFlightPlan(0, filepath.Join(execDir, "plans", p.FlightPlan), 2000+uint32(i))
			}
		}(p))
	}
	fmt.Println("✈️ Ready for takeoff!")

	fmt.Println("⏳ Waiting for all flight plan assignments to complete...")
	wg.Wait()
	fmt.Println("✈️ All done!")
}

func loadParkedAircraft(path string) ([]ParkedAircraft, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var planes []ParkedAircraft
	if err := json.Unmarshal(data, &planes); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return planes, nil
}
