//go:build windows
// +build windows

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/datasets/traffic"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

var cwd, _ = os.Getwd()

type ParkedAircraft struct {
	Airport         string `json:"airport"`
	Plane           string `json:"plane"`
	FlightPlan      string `json:"plan,omitempty"`
	FlightClearance int    `json:"clearance,omitempty"`
	Number          string `json:"number"`
}

type IFRAircraft struct {
	Plane      string  `json:"plane"`
	Number     string  `json:"number"`
	FlightPlan string  `json:"plan"`
	InitPhase  float64 `json:"phase"`
}

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

// runConnection handles a single connection lifecycle to the simulator.
// Returns nil when the simulator disconnects (allowing reconnection),
// or an error if cancelled via context.
func runConnection(ctx context.Context) error {
	// Initialize client with context
	client := simconnect.NewClient("GO Example - SimConnect Read Messages and their data by using Datasets",
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
	// We can already register data definitions and requests here
	addPlanesRequestDataset(client)
	// Load parked aircraft from JSON
	planes, err := loadParkedAircraft("planes.json")
	if err != nil {
		fmt.Println("Error:", err)
		return err
	}
	// Add parked aircraft and set flight plans with delays if specified
	for i, p := range planes {
		//wg := &sync.WaitGroup{}

		if p.FlightClearance >= 0 && p.FlightPlan != "" {
			//wg.Add(1)
			// Simple per-plane delay (tweak as needed or load from data):
			delay := time.Duration(p.FlightClearance) * time.Second
			time.AfterFunc(delay, func(p ParkedAircraft) func() {
				return func() {
					//defer wg.Done()
					// Demo: assign a flight plan or any follow-up work here.
					// Replace with real SimConnect call if desired.
					fmt.Printf("📝 Assigning flight plan for %s (%s) after %s\n", p.Plane, p.Number, delay)
					// Example placeholder: filepath.Join(cwd, "plans", p.FlightPlan)
					client.AICreateEnrouteATCAircraft(p.Plane, p.Number, 123+uint32(i),
						filepath.Join(cwd, "plans", p.FlightPlan), 0.0, false, 2000+uint32(i))
				}
			}(p))
		} else {
			fmt.Printf("📝 Adding parked plane - plane=%s number=%s\n", p.Plane, p.Number)
			client.AICreateParkedATCAircraft(p.Plane, p.Number, p.Airport, 1000+uint32(i))
		}
	}
	fmt.Println("✈️  Ready for plane spotting???")

	//client.AICreateParkedATCAircraft("FSLTL A320 VLG Vueling", "N12345", "LKPR", 5000)
	//client.AICreateParkedATCAircraft("FSLTL_A359_CAL-China Airlines", "N12346", "LKPR", 5001)
	// FSLTL A320 Air France SL

	//client.FlightPlanLoad("C:\\MSFS-TEST-PLANS\\LKPRLKPD_M24_06Dec25")
	//client.AICreateEnrouteATCAircraft("FSLTL A320 Air France SL", "N12347", 123, "C:\\MSFS-TEST-PLANS\\LKPREDDN_MFS_NoProc_07Dec25", 0.0, false, 5006)

	// Request data for all aircraft within 50km radius
	client.RequestDataOnSimObjectType(4001, 3000, 25000, types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT)

	// create ticker to periodically request data
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				client.RequestDataOnSimObjectType(4001, 3000, 25000, types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT)
			}
		}
	}()

	// Wait for SIMCONNECT_RECV_ID_OPEN message to confirm connection is ready
	stream := client.Stream()
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

			fmt.Println("📨 Message received - ", types.SIMCONNECT_RECV_ID(msg.SIMCONNECT_RECV.DwID))

			// Handle specific messages
			// This could be done based on type and also if needed request IDs
			switch types.SIMCONNECT_RECV_ID(msg.DwID) {
			case types.SIMCONNECT_RECV_ID_OPEN:
				fmt.Println("🟢 Connection ready (SIMCONNECT_RECV_ID_OPEN received)")
				msg := msg.AsOpen()
				fmt.Println("📡 Received SIMCONNECT_RECV_OPEN message!")
				fmt.Printf("  Application Name: '%s'\n", engine.BytesToString(msg.SzApplicationName[:]))
				fmt.Printf("  Application Version: %d.%d\n", msg.DwApplicationVersionMajor, msg.DwApplicationVersionMinor)
				fmt.Printf("  Application Build: %d.%d\n", msg.DwApplicationBuildMajor, msg.DwApplicationBuildMinor)
				fmt.Printf("  SimConnect Version: %d.%d\n", msg.DwSimConnectVersionMajor, msg.DwSimConnectVersionMinor)
				fmt.Printf("  SimConnect Build: %d.%d\n", msg.DwSimConnectBuildMajor, msg.DwSimConnectBuildMinor)
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
					// Verbose whole data dump
					//fmt.Printf("     Aircraft Data: %+v\n", aircraftData)
					// Print selected fields
					fmt.Printf("     Aircraft Title: %s, Category: %s, Livery Name: %s, Livery Folder: %s, Lat: %f, Lon: %f, Alt: %f, Head: %f, HeadMag: %f, VS: %f, Pitch: %f, Bank: %f, GroundSpeed: %f, AirspeedIndicated: %f, AirspeedTrue: %f, OnAnyRunway: %d, SurfaceType: %d, SimOnGround: %d, AtcID: %s, AmbientInCloud: %d, IsUserSim: %d, IsTowConnected: %d, AltAboveGround: %f, WingSpan: %f \n",
						aircraftData.TitleAsString(),
						aircraftData.CategoryAsString(),
						aircraftData.LiveryNameAsString(),
						aircraftData.LiveryFolderAsString(),
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
						aircraftData.ATCIDAsString(),
						aircraftData.AmbientInCloud,
						aircraftData.IsUserSim,
						aircraftData.IsTowConnected,
						aircraftData.AltAboveGround,
						aircraftData.WingSpan,
					)
				}
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

func addPlanesRequestDataset(client engine.Client) {
	// Define data structure for plane request dataset
	client.RegisterDataset(
		traffic.NewAircraftDataset("AircraftDataset", 3000),
	)
}
