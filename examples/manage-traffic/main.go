//go:build windows
// +build windows

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/mrlm-net/simconnect"
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

type AircraftData struct {
	Title             [128]byte
	Category          [128]byte
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
	client := simconnect.NewClient("GO Example - SimConnect Read Messages and their data",
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

	fmt.Println("✈️  Ready for plane spotting???")

	//client.AICreateParkedATCAircraft("FSLTL A320 VLG Vueling", "N12345", "LKPR", 5000)
	//client.AICreateParkedATCAircraft("FSLTL_A359_CAL-China Airlines", "N12346", "LKPR", 5001)
	// FSLTL A320 Air France SL

	//client.FlightPlanLoad("C:\\MSFS-TEST-PLANS\\LKPRLKPD_M24_06Dec25")
	client.AICreateNonATCAircraftEX1("FSLTL A320 Air France SL", "N12347", "123", types.SIMCONNECT_DATA_INITPOSITION{
		Latitude:  50.102602,
		Longitude: 14.242601,
		Altitude:  0,
		Heading:   244,
		Pitch:     0,
		Bank:      0,
		OnGround:  1,
	}, 5006)

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
					fmt.Printf("     Aircraft Title: %s, Category: %s, Livery Name: %s, Livery Folder: %s, Lat: %f, Lon: %f, Alt: %f, Head: %f, HeadMag: %f, VS: %f, Pitch: %f, Bank: %f, GroundSpeed: %f, AirspeedIndicated: %f, AirspeedTrue: %f, OnAnyRunway: %d, SurfaceType: %d, SimOnGround: %d, AtcID: %s\n",
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

func addPlanesRequestDataset(client engine.Client) {
	// Define data structure for plane request dataset
	client.AddToDataDefinition(3000, "TITLE", "", types.SIMCONNECT_DATATYPE_STRING128, 0, 0)
	client.AddToDataDefinition(3000, "CATEGORY", "", types.SIMCONNECT_DATATYPE_STRING128, 0, 1)
	client.AddToDataDefinition(3000, "LIVERY NAME", "", types.SIMCONNECT_DATATYPE_STRING128, 0, 2)
	client.AddToDataDefinition(3000, "LIVERY FOLDER", "", types.SIMCONNECT_DATATYPE_STRING128, 0, 3)
	client.AddToDataDefinition(3000, "PLANE LATITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 4)
	client.AddToDataDefinition(3000, "PLANE LONGITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 5)
	client.AddToDataDefinition(3000, "PLANE ALTITUDE", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 6)
	client.AddToDataDefinition(3000, "PLANE HEADING DEGREES TRUE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 7)
	client.AddToDataDefinition(3000, "PLANE HEADING DEGREES MAGNETIC", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 8)
	client.AddToDataDefinition(3000, "VERTICAL SPEED", "feet per second", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 9)
	client.AddToDataDefinition(3000, "PLANE PITCH DEGREES", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 10)
	client.AddToDataDefinition(3000, "PLANE BANK DEGREES", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 11)
	client.AddToDataDefinition(3000, "GROUND VELOCITY", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 12)
	client.AddToDataDefinition(3000, "AIRSPEED INDICATED", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 13)
	client.AddToDataDefinition(3000, "AIRSPEED TRUE", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 14)
	client.AddToDataDefinition(3000, "ON ANY RUNWAY", "bool", types.SIMCONNECT_DATATYPE_INT32, 0, 15)
	client.AddToDataDefinition(3000, "SURFACE TYPE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 16)
	client.AddToDataDefinition(3000, "SIM ON GROUND", "bool", types.SIMCONNECT_DATATYPE_INT32, 0, 17)
	client.AddToDataDefinition(3000, "ATC ID", "", types.SIMCONNECT_DATATYPE_STRING32, 0, 18)
	client.AddToDataDefinition(3000, "ATC AIRLINE", "", types.SIMCONNECT_DATATYPE_STRING32, 0, 19)
}
