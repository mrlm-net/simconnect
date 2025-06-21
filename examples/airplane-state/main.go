//go:build windows
// +build windows

package main

import (
	"encoding/json"
	"flag"
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
	verbose            bool       // Verbose logging flag
)

func main() {
	// Parse command-line flags
	flag.BoolVar(&verbose, "v", false, "Enable verbose output")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose output")
	flag.Parse()

	fmt.Println("SimConnect Aircraft State Monitor with Web GUI")
	if verbose {
		fmt.Println("============================================")
		fmt.Println("This demo displays real-time aircraft state data in a web interface")
		fmt.Println()
	}

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

	if verbose {
		fmt.Println("Connected to SimConnect successfully!")
	}

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
	if verbose {
		fmt.Println("Processing SimConnect messages...")
	}

	messageStream := simClient.Stream()

	for {
		select {
		case <-done:
			fmt.Println("Shutting down...")
			return
		case msg := <-messageStream:
			if msg.Error != nil {
				if verbose {
					fmt.Printf("Message error: %v\n", msg.Error)
				}
				continue
			}

			switch {
			case msg.IsSimObjectData():
				handleAircraftData(msg)

			case msg.IsException():
				if exception, ok := msg.GetException(); ok {
					if verbose {
						fmt.Printf("SimConnect Exception: %v\n", exception)
					}
				}

			case msg.IsOpen():
				if verbose {
					fmt.Println("SimConnect connection confirmed")
				}

			case msg.IsQuit():
				if verbose {
					fmt.Println("SimConnect quit received")
				}
				done <- true
				return
			}
		}
	}
}

func setupDataDefinitions() error {
	if verbose {
		fmt.Println("Setting up aircraft state data definitions...")
	}

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

	if verbose {
		fmt.Println("Aircraft state data definitions setup complete (19 variables)")
	}
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
			if verbose {
				fmt.Printf("Debug: DefineCount: %d elements expected\n", data.DwDefineCount)
			}

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

			if verbose {
				fmt.Printf("SimConnect data (fields=%d):\n", data.DwDefineCount)
				fmt.Printf("  Position: %.6f°N, %.6f°E, %.1f ft\n", latDeg, lonDeg, aircraftData.Altitude)
				fmt.Printf("  Speed: IAS=%.1f TAS=%.1f GS=%.1f VS=%.1f M=%.3f\n",
					aircraftData.AirspeedIndicated, aircraftData.AirspeedTrue, aircraftData.GroundSpeed, aircraftData.VerticalSpeed, aircraftData.AirspeedMach)
				fmt.Printf("  Attitude: P=%.2f° B=%.2f° H(T)=%.1f° H(M)=%.1f°\n", pitchDeg, bankDeg, headingTrueDeg, headingMagDeg)
				fmt.Printf("  Surface: Type=%d Cond=%d Runway=%v Parking=%v\n",
					int(aircraftData.SurfaceType), int(aircraftData.SurfaceCondition), aircraftData.OnAnyRunway > 0.5, aircraftData.ParkingState > 0.5)
				fmt.Printf("  Environment: TAT=%.1f°C SAT=%.1f°F UserSim=%v\n",
					aircraftData.TotalAirTemp, standardTempF, aircraftData.IsUserSim > 0.5)
			}

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
			currentState.StandardTemp = standardTempF // Set title
			currentState.Title = "Real-time Data"
			aircraftStateMutex.Unlock()

			// Only show this message in verbose mode since it repeats frequently
			if verbose {
				fmt.Printf("✈️  Aircraft Data Update: Position %.6f°N, %.6f°E, Alt %.2f ft, Speed %.1f kts\n",
					currentState.Latitude, currentState.Longitude, currentState.Altitude, currentState.GroundSpeed)
			}
		}
	}
}

func startWebServer() {
	http.HandleFunc("/", serveIndex)
	http.HandleFunc("/api/aircraft-state", serveAircraftState)

	if verbose {
		fmt.Printf("Starting web server on port %s...\n", WEB_PORT)
	}
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
                extend: {
                    colors: {
                        'flight-blue': '#1e40af',
                        'altitude-green': '#16a34a',
                        'surface-brown': '#a16207',
                        'neon-blue': '#00d4ff',
                        'neon-green': '#00ff88',
                        'neon-orange': '#ff8800',
                        'neon-purple': '#bb00ff'
                    },
                    animation: {
                        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
                        'fade-in': 'fadeIn 0.5s ease-in-out',
                        'slide-up': 'slideUp 0.3s ease-out'
                    },
                    keyframes: {
                        fadeIn: {
                            '0%': { opacity: '0', transform: 'translateY(10px)' },
                            '100%': { opacity: '1', transform: 'translateY(0)' }
                        },
                        slideUp: {
                            '0%': { transform: 'translateY(20px)', opacity: '0' },
                            '100%': { transform: 'translateY(0)', opacity: '1' }
                        }
                    },
                    boxShadow: {
                        'neon': '0 0 20px rgba(0, 212, 255, 0.3)',
                        'neon-green': '0 0 20px rgba(0, 255, 136, 0.3)',
                        'neon-orange': '0 0 20px rgba(255, 136, 0, 0.3)'
                    }
                }
            }
        }
    </script>
    <style>
        .glass-effect {
            background: rgba(255, 255, 255, 0.05);
            backdrop-filter: blur(10px);
            border: 1px solid rgba(255, 255, 255, 0.1);
        }
        .data-card {
            transition: all 0.3s ease;
            background: linear-gradient(135deg, rgba(45, 55, 72, 0.9), rgba(26, 32, 44, 0.9));
        }
        .data-card:hover {
            transform: translateY(-2px);
            box-shadow: 0 10px 25px rgba(0, 0, 0, 0.3);
        }
        .value-display {
            transition: all 0.2s ease;
        }
        .connection-pulse {
            animation: pulse 2s infinite;
        }
        .gradient-bg {
            background: linear-gradient(135deg, #1a202c 0%, #2d3748 50%, #4a5568 100%);
        }
        .panel-header {
            background: linear-gradient(90deg, rgba(59, 130, 246, 0.1), rgba(59, 130, 246, 0.05));
            border-bottom: 1px solid rgba(59, 130, 246, 0.2);
        }
        .data-row {
            background: rgba(55, 65, 81, 0.6);
            border: 1px solid rgba(75, 85, 99, 0.3);
            transition: all 0.2s ease;
        }
        .data-row:hover {
            background: rgba(75, 85, 99, 0.8);
            border-color: rgba(96, 165, 250, 0.5);
        }
    </style>
</head>
<body class="bg-gray-900 text-white min-h-screen gradient-bg">
    <div class="container mx-auto px-4 py-6">
        <!-- Header -->
        <div class="mb-8 text-center animate-fade-in">
            <h1 class="text-5xl font-bold bg-gradient-to-r from-blue-400 via-purple-500 to-cyan-400 bg-clip-text text-transparent mb-4">
                ✈️ Aircraft State Monitor
            </h1>
            <p class="text-xl text-gray-300 mb-6">Real-time flight simulator data visualization</p>
            <div class="glass-effect rounded-xl px-6 py-4 inline-block">
                <div class="flex justify-center items-center space-x-6">
                    <div class="flex items-center">
                        <div id="connection-indicator" class="w-4 h-4 bg-green-400 rounded-full mr-3 connection-pulse shadow-neon-green"></div>
                        <span id="connection-status" class="text-lg font-medium">Connected</span>
                    </div>
                    <div class="text-lg text-gray-300">
                        Last Update: <span id="last-update" class="text-cyan-400 font-mono">--</span>
                    </div>
                </div>
            </div>
        </div>        <!-- Main Grid -->
        <div class="grid grid-cols-1 xl:grid-cols-3 lg:grid-cols-2 gap-8 max-w-7xl mx-auto">
            
            <!-- Flight Data Panel -->
            <div class="animate-slide-up">
                <div class="data-card rounded-2xl p-6 shadow-2xl border border-blue-500/20 h-fit">
                    <div class="panel-header -mx-6 -mt-6 px-6 py-4 rounded-t-2xl mb-6">
                        <h2 class="text-2xl font-bold text-blue-400 flex items-center">
                            <span class="mr-3">🚀</span> Flight Data
                        </h2>
                    </div>
                    <div class="space-y-4">
                        <div class="data-row flex justify-between items-center p-4 rounded-xl">
                            <span class="text-gray-300 font-medium">🏔️ Altitude</span>
                            <span id="altitude" class="font-mono text-green-400 text-xl font-bold">-- ft</span>
                        </div>
                        <div class="data-row flex justify-between items-center p-4 rounded-xl">
                            <span class="text-gray-300 font-medium">🛩️ Airspeed (IAS)</span>
                            <span id="airspeed-indicated" class="font-mono text-blue-400 text-xl font-bold">-- kts</span>
                        </div>
                        <div class="data-row flex justify-between items-center p-4 rounded-xl">
                            <span class="text-gray-300 font-medium">✈️ Airspeed (TAS)</span>
                            <span id="airspeed-true" class="font-mono text-blue-400 text-xl font-bold">-- kts</span>
                        </div>
                        <div class="data-row flex justify-between items-center p-4 rounded-xl">
                            <span class="text-gray-300 font-medium">🌍 Ground Speed</span>
                            <span id="ground-speed" class="font-mono text-purple-400 text-xl font-bold">-- kts</span>
                        </div>
                        <div class="data-row flex justify-between items-center p-4 rounded-xl">
                            <span class="text-gray-300 font-medium">📈 Vertical Speed</span>
                            <span id="vertical-speed" class="font-mono text-yellow-400 text-xl font-bold">-- fpm</span>
                        </div>
                        <div class="data-row flex justify-between items-center p-4 rounded-xl">
                            <span class="text-gray-300 font-medium">⚡ Mach</span>
                            <span id="mach" class="font-mono text-red-400 text-xl font-bold">--</span>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Attitude and Position Panel -->
            <div class="animate-slide-up" style="animation-delay: 0.1s;">
                <div class="data-card rounded-2xl p-6 shadow-2xl border border-green-500/20 h-fit">
                    <div class="panel-header -mx-6 -mt-6 px-6 py-4 rounded-t-2xl mb-6" style="background: linear-gradient(90deg, rgba(34, 197, 94, 0.1), rgba(34, 197, 94, 0.05)); border-bottom: 1px solid rgba(34, 197, 94, 0.2);">
                        <h2 class="text-2xl font-bold text-green-400 flex items-center">
                            <span class="mr-3">🧭</span> Attitude & Heading
                        </h2>
                    </div>
                    <div class="space-y-4">
                        <div class="data-row flex justify-between items-center p-4 rounded-xl">
                            <span class="text-gray-300 font-medium">📐 Pitch</span>
                            <span id="pitch" class="font-mono text-green-400 text-xl font-bold">--°</span>
                        </div>
                        <div class="data-row flex justify-between items-center p-4 rounded-xl">
                            <span class="text-gray-300 font-medium">🔄 Bank</span>
                            <span id="bank" class="font-mono text-green-400 text-xl font-bold">--°</span>
                        </div>
                        <div class="data-row flex justify-between items-center p-4 rounded-xl">
                            <span class="text-gray-300 font-medium">🧲 Heading (MAG)</span>
                            <span id="heading-magnetic" class="font-mono text-blue-400 text-xl font-bold">--°</span>
                        </div>
                        <div class="data-row flex justify-between items-center p-4 rounded-xl">
                            <span class="text-gray-300 font-medium">🌟 Heading (TRUE)</span>
                            <span id="heading-true" class="font-mono text-blue-400 text-xl font-bold">--°</span>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Combined Position, Surface and Environmental Panel -->
            <div class="animate-slide-up xl:col-span-1 lg:col-span-2" style="animation-delay: 0.2s;">
                <!-- Position and Surface Info -->
                <div class="data-card rounded-2xl p-6 shadow-2xl border border-orange-500/20 mb-8">
                    <div class="panel-header -mx-6 -mt-6 px-6 py-4 rounded-t-2xl mb-6" style="background: linear-gradient(90deg, rgba(251, 146, 60, 0.1), rgba(251, 146, 60, 0.05)); border-bottom: 1px solid rgba(251, 146, 60, 0.2);">
                        <div class="flex justify-between items-center">
                            <h2 class="text-2xl font-bold text-orange-400 flex items-center">
                                <span class="mr-3">📍</span> Position & Surface
                            </h2>
                            <button id="open-maps" class="text-orange-400 hover:text-orange-300 transition-all duration-300 hover:scale-110" title="Open in Google Maps">
                                <svg class="w-7 h-7" fill="currentColor" viewBox="0 0 20 20">
                                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM4.332 8.027a6.012 6.012 0 011.912-2.706C6.512 5.73 6.974 6 7.5 6A1.5 1.5 0 019 7.5V8a2 2 0 004 0 2 2 0 011.523-1.943A5.977 5.977 0 0116 10c0 .34-.028.675-.083 1H15a2 2 0 00-2 2v2.197A5.973 5.973 0 0110 16v-2a2 2 0 00-2-2 2 2 0 01-2-2 2 2 0 00-1.668-1.973z" clip-rule="evenodd"></path>
                                </svg>
                            </button>
                        </div>
                    </div>
                    
                    <!-- Two column layout for position data -->
                    <div class="space-y-4">
                        <div class="data-row flex justify-between items-center p-4 rounded-xl">
                            <span class="text-gray-300 font-medium">🌐 Latitude</span>
                            <span id="latitude" class="font-mono text-orange-400 text-xl font-bold">--°</span>
                        </div>
                        <div class="data-row flex justify-between items-center p-4 rounded-xl">
                            <span class="text-gray-300 font-medium">🌐 Longitude</span>
                            <span id="longitude" class="font-mono text-orange-400 text-xl font-bold">--°</span>
                        </div>
                        <div class="data-row flex justify-between items-center p-4 rounded-xl">
                            <span class="text-gray-300 font-medium">🛬 On Runway</span>
                            <span id="on-runway" class="font-mono text-red-400 text-xl font-bold">--</span>
                        </div>
                        <div class="data-row flex justify-between items-center p-4 rounded-xl">
                            <span class="text-gray-300 font-medium">🅿️ Parking State</span>
                            <span id="parking-state" class="font-mono text-red-400 text-xl font-bold">--</span>
                        </div>
                        <div class="data-row flex justify-between items-center p-4 rounded-xl">
                            <span class="text-gray-300 font-medium">🏗️ Surface Type</span>
                            <span id="surface-type" class="font-mono text-brown-400 text-xl font-bold">--</span>
                        </div>
                        <div class="data-row flex justify-between items-center p-4 rounded-xl">
                            <span class="text-gray-300 font-medium">🌧️ Surface Condition</span>
                            <span id="surface-condition" class="font-mono text-brown-400 text-xl font-bold">--</span>
                        </div>
                    </div>
    
                </div>

                <!-- Environmental Data -->
                <div class="data-card rounded-2xl p-6 shadow-2xl border border-cyan-500/20">
                    <div class="panel-header -mx-6 -mt-6 px-6 py-4 rounded-t-2xl mb-6" style="background: linear-gradient(90deg, rgba(6, 182, 212, 0.1), rgba(6, 182, 212, 0.05)); border-bottom: 1px solid rgba(6, 182, 212, 0.2);">
                        <h2 class="text-2xl font-bold text-cyan-400 flex items-center">
                            <span class="mr-3">🌡️</span> Environmental
                        </h2>
                    </div>                    <div class="space-y-4">
                        <div class="data-row flex justify-between items-center p-4 rounded-xl">
                            <span class="text-gray-300 font-medium">🌡️ Total Air Temp</span>
                            <span id="total-air-temp" class="font-mono text-cyan-400 text-xl font-bold">--°C</span>
                        </div>
                        <div class="data-row flex justify-between items-center p-4 rounded-xl">
                            <span class="text-gray-300 font-medium">📊 Standard Temp</span>
                            <span id="standard-temp" class="font-mono text-cyan-400 text-xl font-bold">--°F</span>
                        </div>
                        <div class="data-row flex justify-between items-center p-4 rounded-xl">
                            <span class="text-gray-300 font-medium">👤 User Aircraft</span>
                            <span id="user-sim" class="font-mono text-green-400 text-xl font-bold">--</span>
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
                    document.getElementById('user-sim').textContent = data.isUserSim ? 'YES' : 'NO';                    // Update connection status
                    document.getElementById('connection-indicator').className = 'w-4 h-4 bg-green-400 rounded-full mr-3 connection-pulse shadow-neon-green';
                    document.getElementById('connection-status').textContent = 'Connected';
                    lastUpdateTime = new Date();
                    document.getElementById('last-update').textContent = lastUpdateTime.toLocaleTimeString();
                })
                .catch(error => {
                    console.error('Error fetching data:', error);
                    document.getElementById('connection-indicator').className = 'w-4 h-4 bg-red-500 rounded-full mr-3';
                    document.getElementById('connection-status').textContent = 'Disconnected';
                });}        // Update data every 50ms for smooth real-time updates (20 FPS)
        setInterval(updateData, 50);
        
        // Globe icon click handler
        document.getElementById('open-maps').addEventListener('click', function() {
            const lat = document.getElementById('latitude').textContent.replace('°', '');
            const lon = document.getElementById('longitude').textContent.replace('°', '');
            
            if (lat !== '--' && lon !== '--') {
                const url = 'https://www.google.com/maps/@' + lat + ',' + lon + ',15z';
                window.open(url, '_blank');
            }
        });
        
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
