//go:build windows
// +build windows

package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/client"
	"github.com/mrlm-net/simconnect/pkg/types"
)

const (
	// Data definition IDs
	AIRCRAFT_STATE_DEFINITION = 1

	// Web server port
	WEB_PORT = "8080"
)

// AircraftData represents the raw SimConnect data structure
// MUST match the exact order and count of variables in setupDataDefinitions()
type AircraftData struct {
	Altitude          float64 // PLANE ALTITUDE in feet
	GroundSpeed       float64 // GROUND VELOCITY in knots
	Latitude          float64 // PLANE LATITUDE in radians (convert to degrees)
	Longitude         float64 // PLANE LONGITUDE in radians (convert to degrees)
	VerticalSpeed     float64 // VERTICAL SPEED in feet per minute
	PitchDegrees      float64 // PLANE PITCH DEGREES in radians (convert to degrees)
	BankDegrees       float64 // PLANE BANK DEGREES in radians (convert to degrees)
	HeadingTrue       float64 // PLANE HEADING DEGREES TRUE in radians (convert to degrees)
	HeadingMagnetic   float64 // PLANE HEADING DEGREES MAGNETIC in radians (convert to degrees)
	AirspeedIndicated float64 // AIRSPEED INDICATED in knots
	AirspeedTrue      float64 // AIRSPEED TRUE in knots
	AirspeedMach      float64 // AIRSPEED MACH in mach
	OnAnyRunway       float64 // ON ANY RUNWAY as boolean (0/1)
	ParkingState      float64 // PLANE IN PARKING STATE as boolean (0/1)
	SurfaceType       float64 // SURFACE TYPE as enum
	SurfaceCondition  float64 // SURFACE CONDITION as enum
	TotalAirTemp      float64 // TOTAL AIR TEMPERATURE in celsius
	StandardTemp      float64 // STANDARD ATM TEMPERATURE in rankine
	IsUserSim         float64 // IS USER SIM as boolean (0/1)
}
type AircraftState struct {
	// Basic Flight Data
	Altitude          float64 `json:"altitude"`
	AirspeedTrue      float64 `json:"airspeedTrue"`
	AirspeedIndicated float64 `json:"airspeedIndicated"`
	GroundSpeed       float64 `json:"groundSpeed"`
	VerticalSpeed     float64 `json:"verticalSpeed"`

	// Position and Attitude
	Latitude        float64 `json:"latitude"`
	Longitude       float64 `json:"longitude"`
	PitchDegrees    float64 `json:"pitchDegrees"`
	BankDegrees     float64 `json:"bankDegrees"`
	HeadingMagnetic float64 `json:"headingMagnetic"`
	HeadingTrue     float64 `json:"headingTrue"`

	// Surface Information
	OnAnyRunway      bool  `json:"onAnyRunway"`
	ParkingState     bool  `json:"parkingState"`
	SurfaceType      int32 `json:"surfaceType"`
	SurfaceCondition int32 `json:"surfaceCondition"`

	// Aircraft State
	Title     string `json:"title"`
	IsUserSim bool   `json:"isUserSim"`

	// Additional Flight Data
	Mach         float64 `json:"mach"`
	TotalAirTemp float64 `json:"totalAirTemp"`
	StandardTemp float64 `json:"standardTemp"`
}

var (
	currentState       *AircraftState
	aircraftStateMutex sync.Mutex
	simClient          *client.Engine
	requestID          uint32 = 1 // Counter for unique request IDs
)

func main() {
	fmt.Println("SimConnect Aircraft State Monitor with Web GUI")
	fmt.Println("============================================")
	fmt.Println("This demo displays real-time aircraft state data in a web interface")
	fmt.Println()

	// Initialize aircraft state
	currentState = &AircraftState{}

	// Create SimConnect client
	simClient = client.New("AircraftStateMonitor")
	if simClient == nil {
		log.Fatal("Failed to create SimConnect client")
	}

	// Connect to SimConnect
	err := simClient.Connect()
	if err != nil {
		log.Fatalf("Failed to connect to SimConnect: %v", err)
	}
	defer simClient.Disconnect()

	fmt.Println("Connected to SimConnect successfully!")

	// Setup data definitions
	err = setupDataDefinitions()
	if err != nil {
		log.Fatalf("Failed to setup data definitions: %v", err)
	}

	// Start data requests
	err = requestAircraftData()
	if err != nil {
		log.Fatalf("Failed to request aircraft data: %v", err)
	}

	// Start web server in a goroutine
	go startWebServer()

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	done := make(chan bool, 1)

	go func() {
		sig := <-sigChan
		fmt.Printf("\nReceived %v signal, shutting down...\n", sig)
		done <- true
	}()

	// Start message processing
	fmt.Printf("Web interface available at: http://localhost:%s\n", WEB_PORT)
	fmt.Println("Processing SimConnect messages...")

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

			switch {
			case msg.IsSimObjectData():
				handleAircraftData(msg)

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
		}
	}
}

func setupDataDefinitions() error {
	fmt.Println("Setting up aircraft state data definitions...")

	defineID := AIRCRAFT_STATE_DEFINITION

	// Basic flight data (0-11)
	err := simClient.AddToDataDefinition(defineID, "PLANE ALTITUDE", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0)
	if err != nil {
		return fmt.Errorf("failed to add PLANE ALTITUDE: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "GROUND VELOCITY", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 1)
	if err != nil {
		return fmt.Errorf("failed to add GROUND VELOCITY: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "PLANE LATITUDE", "radians", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 2)
	if err != nil {
		return fmt.Errorf("failed to add PLANE LATITUDE: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "PLANE LONGITUDE", "radians", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 3)
	if err != nil {
		return fmt.Errorf("failed to add PLANE LONGITUDE: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "VERTICAL SPEED", "feet per minute", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 4)
	if err != nil {
		return fmt.Errorf("failed to add VERTICAL SPEED: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "PLANE PITCH DEGREES", "radians", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 5)
	if err != nil {
		return fmt.Errorf("failed to add PLANE PITCH DEGREES: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "PLANE BANK DEGREES", "radians", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 6)
	if err != nil {
		return fmt.Errorf("failed to add PLANE BANK DEGREES: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "PLANE HEADING DEGREES TRUE", "radians", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 7)
	if err != nil {
		return fmt.Errorf("failed to add PLANE HEADING DEGREES TRUE: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "PLANE HEADING DEGREES MAGNETIC", "radians", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 8)
	if err != nil {
		return fmt.Errorf("failed to add PLANE HEADING DEGREES MAGNETIC: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "AIRSPEED INDICATED", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 9)
	if err != nil {
		return fmt.Errorf("failed to add AIRSPEED INDICATED: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "AIRSPEED TRUE", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 10)
	if err != nil {
		return fmt.Errorf("failed to add AIRSPEED TRUE: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "AIRSPEED MACH", "mach", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 11)
	if err != nil {
		return fmt.Errorf("failed to add AIRSPEED MACH: %v", err)
	}

	// Surface and aircraft state (12-18)
	err = simClient.AddToDataDefinition(defineID, "ON ANY RUNWAY", "bool", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 12)
	if err != nil {
		return fmt.Errorf("failed to add ON ANY RUNWAY: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "PLANE IN PARKING STATE", "bool", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 13)
	if err != nil {
		return fmt.Errorf("failed to add PLANE IN PARKING STATE: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "SURFACE TYPE", "enum", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 14)
	if err != nil {
		return fmt.Errorf("failed to add SURFACE TYPE: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "SURFACE CONDITION", "enum", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 15)
	if err != nil {
		return fmt.Errorf("failed to add SURFACE CONDITION: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "TOTAL AIR TEMPERATURE", "celsius", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 16)
	if err != nil {
		return fmt.Errorf("failed to add TOTAL AIR TEMPERATURE: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "STANDARD ATM TEMPERATURE", "rankine", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 17)
	if err != nil {
		return fmt.Errorf("failed to add STANDARD ATM TEMPERATURE: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "IS USER SIM", "bool", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 18)
	if err != nil {
		return fmt.Errorf("failed to add IS USER SIM: %v", err)
	}

	fmt.Println("Aircraft state data definitions setup complete (19 variables)")
	return nil
}

func requestAircraftData() error {
	// Use a unique request ID each time
	currentRequestID := requestID
	requestID++ // Increment for next request

	// Request data on every sim frame for real-time updates
	return simClient.RequestDataOnSimObject(
		int(currentRequestID),
		AIRCRAFT_STATE_DEFINITION,
		0, // User aircraft
		types.SIMCONNECT_PERIOD_SIM_FRAME,
		types.SIMCONNECT_DATA_REQUEST_FLAG_CHANGED,
		0, 0, 0,
	)
}

func handleAircraftData(msg client.ParsedMessage) {
	if data, ok := msg.GetSimObjectData(); ok {
		if data.DwDefineID == AIRCRAFT_STATE_DEFINITION {
			fmt.Printf("Debug: DefineCount: %d elements expected\n", data.DwDefineCount)

			// Parse using struct pointer casting (as in working examples)
			aircraftData := (*AircraftData)(unsafe.Pointer(&data.DwData))

			// Convert radians to degrees for attitude and position values
			latDeg := aircraftData.Latitude * 180.0 / math.Pi
			lonDeg := aircraftData.Longitude * 180.0 / math.Pi
			pitchDeg := aircraftData.PitchDegrees * 180.0 / math.Pi
			bankDeg := aircraftData.BankDegrees * 180.0 / math.Pi
			headingTrueDeg := aircraftData.HeadingTrue * 180.0 / math.Pi
			headingMagDeg := aircraftData.HeadingMagnetic * 180.0 / math.Pi

			// Convert standard ATM temperature from Rankine to Fahrenheit
			standardTempF := aircraftData.StandardTemp - 459.67

			fmt.Printf("SimConnect data (fields=%d):\n", data.DwDefineCount)
			fmt.Printf("  Position: %.6f°N, %.6f°E, %.1f ft\n", latDeg, lonDeg, aircraftData.Altitude)
			fmt.Printf("  Speed: IAS=%.1f TAS=%.1f GS=%.1f VS=%.1f M=%.3f\n",
				aircraftData.AirspeedIndicated, aircraftData.AirspeedTrue, aircraftData.GroundSpeed, aircraftData.VerticalSpeed, aircraftData.AirspeedMach)
			fmt.Printf("  Attitude: P=%.2f° B=%.2f° H(T)=%.1f° H(M)=%.1f°\n", pitchDeg, bankDeg, headingTrueDeg, headingMagDeg)
			fmt.Printf("  Surface: Type=%d Cond=%d Runway=%v Parking=%v\n",
				int(aircraftData.SurfaceType), int(aircraftData.SurfaceCondition), aircraftData.OnAnyRunway > 0.5, aircraftData.ParkingState > 0.5)
			fmt.Printf("  Environment: TAT=%.1f°C SAT=%.1f°F UserSim=%v\n",
				aircraftData.TotalAirTemp, standardTempF, aircraftData.IsUserSim > 0.5)

			// Update state struct for web display (with thread safety)
			aircraftStateMutex.Lock()
			currentState.Altitude = aircraftData.Altitude
			currentState.GroundSpeed = aircraftData.GroundSpeed
			currentState.Latitude = latDeg
			currentState.Longitude = lonDeg
			currentState.VerticalSpeed = aircraftData.VerticalSpeed
			currentState.PitchDegrees = pitchDeg
			currentState.BankDegrees = bankDeg
			currentState.HeadingTrue = headingTrueDeg
			currentState.HeadingMagnetic = headingMagDeg // Airspeeds and mach
			currentState.AirspeedIndicated = aircraftData.AirspeedIndicated
			currentState.AirspeedTrue = aircraftData.AirspeedTrue
			currentState.Mach = aircraftData.AirspeedMach

			// Surface and aircraft state
			currentState.OnAnyRunway = aircraftData.OnAnyRunway > 0.5
			currentState.ParkingState = aircraftData.ParkingState > 0.5
			currentState.SurfaceType = int32(aircraftData.SurfaceType)
			currentState.SurfaceCondition = int32(aircraftData.SurfaceCondition)
			currentState.IsUserSim = aircraftData.IsUserSim > 0.5

			// Environmental data
			currentState.TotalAirTemp = aircraftData.TotalAirTemp
			currentState.StandardTemp = standardTempF

			// Set title
			currentState.Title = "Real-time Data"
			aircraftStateMutex.Unlock()

			fmt.Printf("✈️  Aircraft Data Update: Position %.6f°N, %.6f°E, Alt %.2f ft, Speed %.1f kts\n",
				currentState.Latitude, currentState.Longitude, currentState.Altitude, currentState.GroundSpeed)
		}
	}
}

func startWebServer() {
	http.HandleFunc("/", serveIndex)
	http.HandleFunc("/api/aircraft-state", serveAircraftState)

	fmt.Printf("Starting web server on port %s...\n", WEB_PORT)
	log.Fatal(http.ListenAndServe(":"+WEB_PORT, nil))
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	htmlTemplate := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Aircraft State Monitor</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <script>
        tailwind.config = {
            theme: {
                extend: {                    colors: {
                        'flight-blue': '#1e40af',
                        'altitude-green': '#16a34a',
                        'surface-brown': '#a16207'
                    }
                }
            }
        }
    </script>
</head>
<body class="bg-gray-900 text-white min-h-screen">
    <div class="container mx-auto px-4 py-6">
        <!-- Header -->
        <div class="mb-8 text-center">
            <h1 class="text-4xl font-bold text-blue-400 mb-2">Aircraft State Monitor</h1>
            <p class="text-gray-300">Real-time flight simulator data visualization</p>
            <div class="mt-4 flex justify-center items-center space-x-4">
                <div class="flex items-center">
                    <div id="connection-indicator" class="w-3 h-3 bg-green-500 rounded-full mr-2"></div>
                    <span id="connection-status" class="text-sm">Connected</span>
                </div>
                <div class="text-sm text-gray-400">
                    Last Update: <span id="last-update">--</span>
                </div>
            </div>
        </div>

        <!-- Main Grid -->
        <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
            
            <!-- Flight Data Panel -->
            <div class="lg:col-span-1">
                <div class="bg-gray-800 rounded-lg p-6 shadow-lg">
                    <h2 class="text-xl font-semibold mb-4 text-blue-400">Flight Data</h2>
                    <div class="space-y-3">
                        <div class="flex justify-between items-center p-3 bg-gray-700 rounded">
                            <span class="text-gray-300">Altitude</span>
                            <span id="altitude" class="font-mono text-green-400 text-lg">-- ft</span>
                        </div>
                        <div class="flex justify-between items-center p-3 bg-gray-700 rounded">
                            <span class="text-gray-300">Airspeed (IAS)</span>
                            <span id="airspeed-indicated" class="font-mono text-blue-400 text-lg">-- kts</span>
                        </div>
                        <div class="flex justify-between items-center p-3 bg-gray-700 rounded">
                            <span class="text-gray-300">Airspeed (TAS)</span>
                            <span id="airspeed-true" class="font-mono text-blue-400 text-lg">-- kts</span>
                        </div>
                        <div class="flex justify-between items-center p-3 bg-gray-700 rounded">
                            <span class="text-gray-300">Ground Speed</span>
                            <span id="ground-speed" class="font-mono text-purple-400 text-lg">-- kts</span>
                        </div>
                        <div class="flex justify-between items-center p-3 bg-gray-700 rounded">
                            <span class="text-gray-300">Vertical Speed</span>
                            <span id="vertical-speed" class="font-mono text-yellow-400 text-lg">-- fpm</span>
                        </div>
                        <div class="flex justify-between items-center p-3 bg-gray-700 rounded">
                            <span class="text-gray-300">Mach</span>
                            <span id="mach" class="font-mono text-red-400 text-lg">--</span>
                        </div>
                    </div>                </div>

            </div>

            <!-- Attitude and Position Panel -->
            <div class="lg:col-span-1">
                <div class="bg-gray-800 rounded-lg p-6 shadow-lg">
                    <h2 class="text-xl font-semibold mb-4 text-green-400">Attitude & Heading</h2>
                    <div class="space-y-3">
                        <div class="flex justify-between items-center p-3 bg-gray-700 rounded">
                            <span class="text-gray-300">Pitch</span>
                            <span id="pitch" class="font-mono text-green-400 text-lg">--°</span>
                        </div>
                        <div class="flex justify-between items-center p-3 bg-gray-700 rounded">
                            <span class="text-gray-300">Bank</span>
                            <span id="bank" class="font-mono text-green-400 text-lg">--°</span>
                        </div>
                        <div class="flex justify-between items-center p-3 bg-gray-700 rounded">
                            <span class="text-gray-300">Heading (MAG)</span>
                            <span id="heading-magnetic" class="font-mono text-blue-400 text-lg">--°</span>
                        </div>
                        <div class="flex justify-between items-center p-3 bg-gray-700 rounded">
                            <span class="text-gray-300">Heading (TRUE)</span>
                            <span id="heading-true" class="font-mono text-blue-400 text-lg">--°</span>                        </div>
                    </div>
                </div>
            </div>

            <!-- Position and Surface Info -->
            <div class="lg:col-span-1">
                <div class="bg-gray-800 rounded-lg p-6 shadow-lg">
                    <h2 class="text-xl font-semibold mb-4 text-orange-400">Position & Surface</h2>
                    <div class="space-y-3">
                        <div class="flex justify-between items-center p-3 bg-gray-700 rounded">
                            <span class="text-gray-300">Latitude</span>
                            <span id="latitude" class="font-mono text-orange-400 text-lg">--°</span>
                        </div>
                        <div class="flex justify-between items-center p-3 bg-gray-700 rounded">
                            <span class="text-gray-300">Longitude</span>
                            <span id="longitude" class="font-mono text-orange-400 text-lg">--°</span>
                        </div>
                        <div class="flex justify-between items-center p-3 bg-gray-700 rounded">
                            <span class="text-gray-300">On Runway</span>
                            <span id="on-runway" class="font-mono text-red-400 text-lg">--</span>
                        </div>
                        <div class="flex justify-between items-center p-3 bg-gray-700 rounded">
                            <span class="text-gray-300">Parking State</span>
                            <span id="parking-state" class="font-mono text-red-400 text-lg">--</span>
                        </div>
                        <div class="flex justify-between items-center p-3 bg-gray-700 rounded">
                            <span class="text-gray-300">Surface Type</span>
                            <span id="surface-type" class="font-mono text-brown-400 text-lg">--</span>
                        </div>
                        <div class="flex justify-between items-center p-3 bg-gray-700 rounded">
                            <span class="text-gray-300">Surface Condition</span>
                            <span id="surface-condition" class="font-mono text-brown-400 text-lg">--</span>
                        </div>
                    </div>
                </div>                <!-- Environmental Data -->
                <div class="bg-gray-800 rounded-lg p-6 shadow-lg mt-6">
                    <h2 class="text-xl font-semibold mb-4 text-cyan-400">Environmental</h2>
                    <div class="space-y-3">
                        <div class="flex justify-between items-center p-3 bg-gray-700 rounded">
                            <span class="text-gray-300">Total Air Temp</span>
                            <span id="total-air-temp" class="font-mono text-cyan-400 text-lg">--°C</span>
                        </div>
                        <div class="flex justify-between items-center p-3 bg-gray-700 rounded">
                            <span class="text-gray-300">Standard Temp</span>
                            <span id="standard-temp" class="font-mono text-cyan-400 text-lg">--°F</span>
                        </div>
                        <div class="flex justify-between items-center p-3 bg-gray-700 rounded">
                            <span class="text-gray-300">User Aircraft</span>
                            <span id="user-sim" class="font-mono text-green-400 text-lg">--</span>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>

    <script>
        let lastUpdateTime = new Date();
        
        // Surface type mapping
        const surfaceTypes = {
            0: 'Concrete', 1: 'Grass', 2: 'Water', 3: 'Grass (Bumpy)', 4: 'Asphalt',
            5: 'Short Grass', 6: 'Long Grass', 7: 'Hard Turf', 8: 'Snow', 9: 'Ice',
            10: 'Urban', 11: 'Forest', 12: 'Dirt', 13: 'Coral', 14: 'Gravel',
            15: 'Oil Treated', 16: 'Steel Mats', 17: 'Bituminous', 18: 'Brick',
            19: 'Macadam', 20: 'Planks', 21: 'Sand', 22: 'Shale', 23: 'Tarmac',
            24: 'Wright Flyer Track'
        };
        
        const surfaceConditions = {
            0: 'Normal', 1: 'Wet', 2: 'Icy', 3: 'Snow'
        };

        function formatNumber(value, decimals = 1) {
            if (value === null || value === undefined || isNaN(value)) return '--';
            return Number(value).toFixed(decimals);
        }

        function updateData() {
            fetch('/api/aircraft-state')
                .then(response => response.json())
                .then(data => {
                    // Update flight data
                    document.getElementById('altitude').textContent = formatNumber(data.altitude, 0) + ' ft';
                    document.getElementById('airspeed-indicated').textContent = formatNumber(data.airspeedIndicated, 0) + ' kts';
                    document.getElementById('airspeed-true').textContent = formatNumber(data.airspeedTrue, 0) + ' kts';
                    document.getElementById('ground-speed').textContent = formatNumber(data.groundSpeed, 0) + ' kts';
                    document.getElementById('vertical-speed').textContent = formatNumber(data.verticalSpeed, 0) + ' fpm';
                    document.getElementById('mach').textContent = formatNumber(data.mach, 3);                    // Update attitude
                    document.getElementById('pitch').textContent = formatNumber(data.pitchDegrees, 1) + '°';
                    document.getElementById('bank').textContent = formatNumber(data.bankDegrees, 1) + '°';
                    document.getElementById('heading-magnetic').textContent = formatNumber(data.headingMagnetic, 0) + '°';
                    document.getElementById('heading-true').textContent = formatNumber(data.headingTrue, 0) + '°';

                    // Update position
                    document.getElementById('latitude').textContent = formatNumber(data.latitude, 6) + '°';
                    document.getElementById('longitude').textContent = formatNumber(data.longitude, 6) + '°';
                    document.getElementById('on-runway').textContent = data.onAnyRunway ? 'YES' : 'NO';
                    document.getElementById('parking-state').textContent = data.parkingState ? 'PARKED' : 'ACTIVE';
                    document.getElementById('surface-type').textContent = surfaceTypes[data.surfaceType] || 'Unknown';
                    document.getElementById('surface-condition').textContent = surfaceConditions[data.surfaceCondition] || 'Unknown';

                    // Update environmental
                    document.getElementById('total-air-temp').textContent = formatNumber(data.totalAirTemp, 1) + '°C';
                    document.getElementById('standard-temp').textContent = formatNumber(data.standardTemp, 1) + '°F';
                    document.getElementById('user-sim').textContent = data.isUserSim ? 'YES' : 'NO';

                    // Update connection status
                    document.getElementById('connection-indicator').className = 'w-3 h-3 bg-green-500 rounded-full mr-2';
                    document.getElementById('connection-status').textContent = 'Connected';
                    lastUpdateTime = new Date();
                    document.getElementById('last-update').textContent = lastUpdateTime.toLocaleTimeString();
                })
                .catch(error => {
                    console.error('Error fetching data:', error);
                    document.getElementById('connection-indicator').className = 'w-3 h-3 bg-red-500 rounded-full mr-2';
                    document.getElementById('connection-status').textContent = 'Disconnected';
                });        }

        // Update data every 50ms for smooth real-time updates (20 FPS)
        setInterval(updateData, 50);
        
        // Initial data load
        updateData();
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	tmpl := template.Must(template.New("index").Parse(htmlTemplate))
	tmpl.Execute(w, nil)
}

func serveAircraftState(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	aircraftStateMutex.Lock()
	defer aircraftStateMutex.Unlock()

	if currentState == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "No data available",
		})
		return
	}

	json.NewEncoder(w).Encode(currentState)
}
