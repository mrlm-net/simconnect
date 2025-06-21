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
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/client"
	"github.com/mrlm-net/simconnect/pkg/types"
)

const (
	// Data definition IDs
	CAMERA_DATA_DEFINITION      = 1
	CAMERA_STATE_SET_DEFINITION = 2
	CAMERA_PARAM_SET_DEFINITION = 3

	// Web server port
	WEB_PORT = "8080"

	// Camera states
	CAMERA_COCKPIT        = 2
	CAMERA_EXTERNAL_CHASE = 3
	CAMERA_DRONE          = 4
	CAMERA_FIXED_ON_PLANE = 5
	CAMERA_ENVIRONMENT    = 6
	CAMERA_SIX_DOF        = 7
	CAMERA_GAMEPLAY       = 8
	CAMERA_SHOWCASE       = 9
	CAMERA_DRONE_AIRCRAFT = 10

	// Camera substates
	CAMERA_LOCKED     = 1
	CAMERA_UNLOCKED   = 2
	CAMERA_QUICKVIEW  = 3
	CAMERA_SMART      = 4
	CAMERA_INSTRUMENT = 5

	// Camera view types
	CAMERA_VIEW_UNKNOWN       = 0
	CAMERA_VIEW_PILOT         = 1
	CAMERA_VIEW_INSTRUMENTS   = 2
	CAMERA_VIEW_QUICKVIEW     = 3
	CAMERA_VIEW_QUICKVIEW_EXT = 4
	CAMERA_VIEW_VIEW          = 5

	// Focus modes
	FOCUS_AUTO   = 0
	FOCUS_MANUAL = 1

	// Headlook modes
	HEADLOOK_FREELOOK = 1
	HEADLOOK_HEADLOOK = 2
)

// CameraStateData for setting camera state
type CameraStateData struct {
	CameraState float64 // CAMERA STATE
}

// CameraParameterData for setting individual camera parameters
type CameraParameterData struct {
	Value float64
}

// CameraData represents the raw SimConnect camera data structure
type CameraData struct {
	// Basic camera state
	CameraState    float64 // CAMERA STATE
	CameraSubstate float64 // CAMERA SUBSTATE

	// View type and index
	CameraViewType  float64 // CAMERA VIEW TYPE AND INDEX:0
	CameraViewIndex float64 // CAMERA VIEW TYPE AND INDEX:1
	CameraViewMax   float64 // CAMERA VIEW TYPE AND INDEX MAX:1

	// Gameplay camera
	GameplayPitch       float64 // CAMERA GAMEPLAY PITCH YAW:0
	GameplayYaw         float64 // CAMERA GAMEPLAY PITCH YAW:1
	GameplayCameraFocus float64 // GAMEPLAY CAMERA FOCUS

	// Cockpit camera
	CockpitHeadlook          float64 // COCKPIT CAMERA HEADLOOK
	CockpitHeight            float64 // COCKPIT CAMERA HEIGHT
	CockpitMomentum          float64 // COCKPIT CAMERA MOMENTUM
	CockpitSpeed             float64 // COCKPIT CAMERA SPEED
	CockpitZoom              float64 // COCKPIT CAMERA ZOOM
	CockpitZoomSpeed         float64 // COCKPIT CAMERA ZOOM SPEED
	CockpitUpperPosition     float64 // COCKPIT CAMERA UPPER POSITION
	CockpitInstrumentAutosel float64 // COCKPIT CAMERA INSTRUMENT AUTOSELECT

	// Chase (External) camera
	ChaseHeadlook  float64 // CHASE CAMERA HEADLOOK
	ChaseMomentum  float64 // CHASE CAMERA MOMENTUM
	ChaseSpeed     float64 // CHASE CAMERA SPEED
	ChaseZoom      float64 // CHASE CAMERA ZOOM
	ChaseZoomSpeed float64 // CHASE CAMERA ZOOM SPEED

	// Drone camera
	DroneFocus         float64 // DRONE CAMERA FOCUS
	DroneFocusMode     float64 // DRONE CAMERA FOCUS MODE
	DroneFollow        float64 // DRONE CAMERA FOLLOW
	DroneFOV           float64 // DRONE CAMERA FOV
	DroneLocked        float64 // DRONE CAMERA LOCKED
	DroneSpeedRotation float64 // DRONE CAMERA SPEED ROTATION
	DroneSpeedTravel   float64 // DRONE CAMERA SPEED TRAVELLING
	// Smart camera
	SmartCameraActive       float64 // SMART CAMERA ACTIVE
	SmartCameraTargetCount  float64 // SMART CAMERA INFO:0
	SmartCameraCurrentIndex float64 // SMART CAMERA INFO:1	// Environment variables
	LocalTime               float64 // LOCAL TIME
	LocalDayOfMonth         float64 // LOCAL DAY OF MONTH
	LocalMonthOfYear        float64 // LOCAL MONTH OF YEAR
	LocalYear               float64 // LOCAL YEAR
	SimulationTime          float64 // SIMULATION TIME
}

// CameraState represents the processed camera state for the web interface
type CameraState struct {
	// Basic camera state
	CameraState    int    `json:"cameraState"`
	CameraSubstate int    `json:"cameraSubstate"`
	StateText      string `json:"stateText"`
	SubstateText   string `json:"substateText"`

	// View type and index
	ViewType     int    `json:"viewType"`
	ViewIndex    int    `json:"viewIndex"`
	ViewMaxIndex int    `json:"viewMaxIndex"`
	ViewTypeText string `json:"viewTypeText"`

	// Gameplay camera
	GameplayPitch       float64 `json:"gameplayPitch"`
	GameplayYaw         float64 `json:"gameplayYaw"`
	GameplayCameraFocus int     `json:"gameplayCameraFocus"`

	// Cockpit camera
	CockpitHeadlook          int     `json:"cockpitHeadlook"`
	CockpitHeight            float64 `json:"cockpitHeight"`
	CockpitMomentum          float64 `json:"cockpitMomentum"`
	CockpitSpeed             float64 `json:"cockpitSpeed"`
	CockpitZoom              float64 `json:"cockpitZoom"`
	CockpitZoomSpeed         float64 `json:"cockpitZoomSpeed"`
	CockpitUpperPosition     bool    `json:"cockpitUpperPosition"`
	CockpitInstrumentAutosel bool    `json:"cockpitInstrumentAutosel"`

	// Chase (External) camera
	ChaseHeadlook  int     `json:"chaseHeadlook"`
	ChaseMomentum  float64 `json:"chaseMomentum"`
	ChaseSpeed     float64 `json:"chaseSpeed"`
	ChaseZoom      float64 `json:"chaseZoom"`
	ChaseZoomSpeed float64 `json:"chaseZoomSpeed"`

	// Drone camera
	DroneFocus         float64 `json:"droneFocus"`
	DroneFocusMode     int     `json:"droneFocusMode"`
	DroneFollow        bool    `json:"droneFollow"`
	DroneFOV           float64 `json:"droneFOV"`
	DroneLocked        bool    `json:"droneLocked"`
	DroneSpeedRotation float64 `json:"droneSpeedRotation"`
	DroneSpeedTravel   float64 `json:"droneSpeedTravel"`
	// Smart camera
	SmartCameraActive       bool   `json:"smartCameraActive"`
	SmartCameraTargetCount  int    `json:"smartCameraTargetCount"`
	SmartCameraCurrentIndex int    `json:"smartCameraCurrentIndex"` // Environment variables
	LocalTime               string `json:"localTime"`
	LocalDate               string `json:"localDate"`
	SimulationTime          string `json:"simulationTime"`

	// Meta
	Title       string `json:"title"`
	LastUpdated string `json:"lastUpdated"`
}

var (
	currentState     *CameraState
	cameraStateMutex sync.Mutex
	simClient        *client.Engine
	requestID        uint32 = 1
	verbose          bool
)

func main() {
	// Parse command-line flags
	flag.BoolVar(&verbose, "v", false, "Enable verbose output")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose output")
	flag.Parse()

	fmt.Println("SimConnect Camera Controller with Web GUI")
	if verbose {
		fmt.Println("========================================")
		fmt.Println("This demo provides comprehensive camera control and monitoring")
		fmt.Println()
	}

	// Initialize camera state
	currentState = &CameraState{}

	// Create SimConnect client
	simClient = client.New("CameraController")
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

	// Setup setting data definitions
	err = setupSetDataDefinitions()
	if err != nil {
		log.Fatalf("Failed to setup set data definitions: %v", err)
	}

	// Start data requests
	err = requestCameraData()
	if err != nil {
		log.Fatalf("Failed to request camera data: %v", err)
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
	fmt.Printf("Camera Controller web interface available at: http://localhost:%s\n", WEB_PORT)
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
				handleCameraData(msg)

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
		fmt.Println("Setting up camera data definitions...")
	}

	defineID := CAMERA_DATA_DEFINITION

	// Basic camera state (0-2)
	err := simClient.AddToDataDefinition(defineID, "CAMERA STATE", "enum", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0)
	if err != nil {
		return fmt.Errorf("failed to add CAMERA STATE: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "CAMERA SUBSTATE", "enum", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 1)
	if err != nil {
		return fmt.Errorf("failed to add CAMERA SUBSTATE: %v", err)
	} // View type and index (2-4)
	err = simClient.AddToDataDefinition(defineID, "CAMERA VIEW TYPE AND INDEX:0", "enum", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 2)
	if err != nil {
		return fmt.Errorf("failed to add CAMERA VIEW TYPE: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "CAMERA VIEW TYPE AND INDEX:1", "number", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 3)
	if err != nil {
		return fmt.Errorf("failed to add CAMERA VIEW INDEX: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "CAMERA VIEW TYPE AND INDEX MAX:1", "number", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 4)
	if err != nil {
		return fmt.Errorf("failed to add CAMERA VIEW MAX: %v", err)
	}

	// Gameplay camera (5-7)
	err = simClient.AddToDataDefinition(defineID, "CAMERA GAMEPLAY PITCH YAW:0", "radians", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 5)
	if err != nil {
		return fmt.Errorf("failed to add GAMEPLAY PITCH: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "CAMERA GAMEPLAY PITCH YAW:1", "radians", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 6)
	if err != nil {
		return fmt.Errorf("failed to add GAMEPLAY YAW: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "GAMEPLAY CAMERA FOCUS", "enum", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 7)
	if err != nil {
		return fmt.Errorf("failed to add GAMEPLAY CAMERA FOCUS: %v", err)
	}

	// Cockpit camera (8-15)
	err = simClient.AddToDataDefinition(defineID, "COCKPIT CAMERA HEADLOOK", "enum", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 8)
	if err != nil {
		return fmt.Errorf("failed to add COCKPIT HEADLOOK: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "COCKPIT CAMERA HEIGHT", "percentage", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 9)
	if err != nil {
		return fmt.Errorf("failed to add COCKPIT HEIGHT: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "COCKPIT CAMERA MOMENTUM", "percentage", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 10)
	if err != nil {
		return fmt.Errorf("failed to add COCKPIT MOMENTUM: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "COCKPIT CAMERA SPEED", "percentage", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 11)
	if err != nil {
		return fmt.Errorf("failed to add COCKPIT SPEED: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "COCKPIT CAMERA ZOOM", "percentage", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 12)
	if err != nil {
		return fmt.Errorf("failed to add COCKPIT ZOOM: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "COCKPIT CAMERA ZOOM SPEED", "percentage", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 13)
	if err != nil {
		return fmt.Errorf("failed to add COCKPIT ZOOM SPEED: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "COCKPIT CAMERA UPPER POSITION", "bool", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 14)
	if err != nil {
		return fmt.Errorf("failed to add COCKPIT UPPER POSITION: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "COCKPIT CAMERA INSTRUMENT AUTOSELECT", "bool", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 15)
	if err != nil {
		return fmt.Errorf("failed to add COCKPIT INSTRUMENT AUTOSELECT: %v", err)
	}

	// Chase camera (16-20)
	err = simClient.AddToDataDefinition(defineID, "CHASE CAMERA HEADLOOK", "enum", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 16)
	if err != nil {
		return fmt.Errorf("failed to add CHASE HEADLOOK: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "CHASE CAMERA MOMENTUM", "percentage", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 17)
	if err != nil {
		return fmt.Errorf("failed to add CHASE MOMENTUM: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "CHASE CAMERA SPEED", "percentage", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 18)
	if err != nil {
		return fmt.Errorf("failed to add CHASE SPEED: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "CHASE CAMERA ZOOM", "percentage", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 19)
	if err != nil {
		return fmt.Errorf("failed to add CHASE ZOOM: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "CHASE CAMERA ZOOM SPEED", "percentage", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 20)
	if err != nil {
		return fmt.Errorf("failed to add CHASE ZOOM SPEED: %v", err)
	}

	// Drone camera (21-27)
	err = simClient.AddToDataDefinition(defineID, "DRONE CAMERA FOCUS", "number", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 21)
	if err != nil {
		return fmt.Errorf("failed to add DRONE FOCUS: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "DRONE CAMERA FOCUS MODE", "enum", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 22)
	if err != nil {
		return fmt.Errorf("failed to add DRONE FOCUS MODE: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "DRONE CAMERA FOLLOW", "bool", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 23)
	if err != nil {
		return fmt.Errorf("failed to add DRONE FOLLOW: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "DRONE CAMERA FOV", "percentage", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 24)
	if err != nil {
		return fmt.Errorf("failed to add DRONE FOV: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "DRONE CAMERA LOCKED", "bool", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 25)
	if err != nil {
		return fmt.Errorf("failed to add DRONE LOCKED: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "DRONE CAMERA SPEED ROTATION", "percentage", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 26)
	if err != nil {
		return fmt.Errorf("failed to add DRONE SPEED ROTATION: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "DRONE CAMERA SPEED TRAVELLING", "percentage", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 27)
	if err != nil {
		return fmt.Errorf("failed to add DRONE SPEED TRAVELLING: %v", err)
	}
	// Smart camera (28-30)
	err = simClient.AddToDataDefinition(defineID, "SMART CAMERA ACTIVE", "bool", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 28)
	if err != nil {
		return fmt.Errorf("failed to add SMART CAMERA ACTIVE: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "SMART CAMERA INFO:0", "number", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 29)
	if err != nil {
		return fmt.Errorf("failed to add SMART CAMERA TARGET COUNT: %v", err)
	}
	err = simClient.AddToDataDefinition(defineID, "SMART CAMERA INFO:1", "number", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 30)
	if err != nil {
		return fmt.Errorf("failed to add SMART CAMERA CURRENT INDEX: %v", err)
	}
	// Environment variables (31-35)
	err = simClient.AddToDataDefinition(defineID, "LOCAL TIME", "seconds", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 31)
	if err != nil {
		return fmt.Errorf("failed to add LOCAL TIME: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "LOCAL DAY OF MONTH", "number", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 32)
	if err != nil {
		return fmt.Errorf("failed to add LOCAL DAY OF MONTH: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "LOCAL MONTH OF YEAR", "number", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 33)
	if err != nil {
		return fmt.Errorf("failed to add LOCAL MONTH OF YEAR: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "LOCAL YEAR", "number", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 34)
	if err != nil {
		return fmt.Errorf("failed to add LOCAL YEAR: %v", err)
	}

	err = simClient.AddToDataDefinition(defineID, "SIMULATION TIME", "seconds", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 35)
	if err != nil {
		return fmt.Errorf("failed to add SIMULATION TIME: %v", err)
	}

	if verbose {
		fmt.Println("Camera data definitions setup complete (36 variables)")
	}
	return nil
}

func setupSetDataDefinitions() error {
	if verbose {
		fmt.Println("Setting up camera set data definitions...")
	}

	// Camera state setting definition
	err := simClient.AddToDataDefinition(CAMERA_STATE_SET_DEFINITION, "CAMERA STATE", "enum", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0)
	if err != nil {
		return fmt.Errorf("failed to add CAMERA STATE set definition: %v", err)
	}

	// Generic parameter setting definition (will be used for different parameters)
	err = simClient.AddToDataDefinition(CAMERA_PARAM_SET_DEFINITION, "COCKPIT CAMERA HEIGHT", "percentage", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0)
	if err != nil {
		return fmt.Errorf("failed to add parameter set definition: %v", err)
	}

	if verbose {
		fmt.Println("Camera set data definitions setup complete")
	}
	return nil
}

func requestCameraData() error {
	currentRequestID := requestID
	requestID++

	return simClient.RequestDataOnSimObject(
		int(currentRequestID),
		CAMERA_DATA_DEFINITION,
		0, // User aircraft
		types.SIMCONNECT_PERIOD_SIM_FRAME,
		types.SIMCONNECT_DATA_REQUEST_FLAG_CHANGED,
		0, 0, 0,
	)
}

func handleCameraData(msg client.ParsedMessage) {
	if data, ok := msg.GetSimObjectData(); ok {
		if data.DwDefineID == CAMERA_DATA_DEFINITION {
			cameraData := (*CameraData)(unsafe.Pointer(&data.DwData))

			// Update state struct for web display (with thread safety)
			cameraStateMutex.Lock()
			currentState.CameraState = int(cameraData.CameraState)
			currentState.CameraSubstate = int(cameraData.CameraSubstate)
			currentState.StateText = getCameraStateText(currentState.CameraState)
			currentState.SubstateText = getCameraSubstateText(currentState.CameraSubstate)

			currentState.ViewType = int(cameraData.CameraViewType)
			currentState.ViewIndex = int(cameraData.CameraViewIndex)
			currentState.ViewMaxIndex = int(cameraData.CameraViewMax)
			currentState.ViewTypeText = getViewTypeText(currentState.ViewType)

			// Convert radians to degrees for pitch/yaw
			currentState.GameplayPitch = cameraData.GameplayPitch * 180.0 / math.Pi
			currentState.GameplayYaw = cameraData.GameplayYaw * 180.0 / math.Pi
			currentState.GameplayCameraFocus = int(cameraData.GameplayCameraFocus)

			// Cockpit camera
			currentState.CockpitHeadlook = int(cameraData.CockpitHeadlook)
			currentState.CockpitHeight = cameraData.CockpitHeight
			currentState.CockpitMomentum = cameraData.CockpitMomentum
			currentState.CockpitSpeed = cameraData.CockpitSpeed
			currentState.CockpitZoom = cameraData.CockpitZoom
			currentState.CockpitZoomSpeed = cameraData.CockpitZoomSpeed
			currentState.CockpitUpperPosition = cameraData.CockpitUpperPosition > 0.5
			currentState.CockpitInstrumentAutosel = cameraData.CockpitInstrumentAutosel > 0.5

			// Chase camera
			currentState.ChaseHeadlook = int(cameraData.ChaseHeadlook)
			currentState.ChaseMomentum = cameraData.ChaseMomentum
			currentState.ChaseSpeed = cameraData.ChaseSpeed
			currentState.ChaseZoom = cameraData.ChaseZoom
			currentState.ChaseZoomSpeed = cameraData.ChaseZoomSpeed

			// Drone camera - use raw values as received from SimConnect
			currentState.DroneFocus = cameraData.DroneFocus
			currentState.DroneFocusMode = int(cameraData.DroneFocusMode)
			currentState.DroneFollow = cameraData.DroneFollow > 0.5
			currentState.DroneFOV = cameraData.DroneFOV
			currentState.DroneLocked = cameraData.DroneLocked > 0.5
			currentState.DroneSpeedRotation = cameraData.DroneSpeedRotation
			currentState.DroneSpeedTravel = cameraData.DroneSpeedTravel // Smart camera
			currentState.SmartCameraActive = cameraData.SmartCameraActive > 0.5
			currentState.SmartCameraTargetCount = int(cameraData.SmartCameraTargetCount)
			currentState.SmartCameraCurrentIndex = int(cameraData.SmartCameraCurrentIndex) // Environment variables
			currentState.LocalTime = formatTime(cameraData.LocalTime)
			currentState.LocalDate = formatDate(cameraData.LocalDayOfMonth, cameraData.LocalMonthOfYear, cameraData.LocalYear)
			currentState.SimulationTime = formatSimulationTime(cameraData.SimulationTime)

			currentState.Title = "Camera Controller"
			currentState.LastUpdated = time.Now().Format("15:04:05")
			cameraStateMutex.Unlock()

			if verbose {
				fmt.Printf("📹 Camera Update: State=%s (%d), Substate=%s (%d), View=%s\n",
					currentState.StateText, currentState.CameraState,
					currentState.SubstateText, currentState.CameraSubstate,
					currentState.ViewTypeText)
			}
		}
	}
}

func getCameraStateText(state int) string {
	switch state {
	case CAMERA_COCKPIT:
		return "Cockpit"
	case CAMERA_EXTERNAL_CHASE:
		return "External/Chase"
	case CAMERA_DRONE:
		return "Drone"
	case CAMERA_FIXED_ON_PLANE:
		return "Fixed on Plane"
	case CAMERA_ENVIRONMENT:
		return "Environment"
	case CAMERA_SIX_DOF:
		return "Six DoF"
	case CAMERA_GAMEPLAY:
		return "Gameplay"
	case CAMERA_SHOWCASE:
		return "Showcase"
	case CAMERA_DRONE_AIRCRAFT:
		return "Drone Aircraft"
	default:
		return fmt.Sprintf("Unknown (%d)", state)
	}
}

func getCameraSubstateText(substate int) string {
	switch substate {
	case CAMERA_LOCKED:
		return "Locked"
	case CAMERA_UNLOCKED:
		return "Unlocked"
	case CAMERA_QUICKVIEW:
		return "Quickview"
	case CAMERA_SMART:
		return "Smart"
	case CAMERA_INSTRUMENT:
		return "Instrument"
	default:
		return fmt.Sprintf("Unknown (%d)", substate)
	}
}

func getViewTypeText(viewType int) string {
	switch viewType {
	case CAMERA_VIEW_UNKNOWN:
		return "Unknown/Default"
	case CAMERA_VIEW_PILOT:
		return "Pilot View"
	case CAMERA_VIEW_INSTRUMENTS:
		return "Instruments"
	case CAMERA_VIEW_QUICKVIEW:
		return "Quickview"
	case CAMERA_VIEW_QUICKVIEW_EXT:
		return "Quickview External"
	case CAMERA_VIEW_VIEW:
		return "View"
	default:
		return fmt.Sprintf("Unknown (%d)", viewType)
	}
}

func formatTime(seconds float64) string {
	if seconds < 0 {
		return "--:--"
	}

	totalSeconds := int(seconds)
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	secs := totalSeconds % 60

	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, secs)
}

func formatDate(day, month, year float64) string {
	if day < 0 || month < 0 || year < 0 {
		return "--/--/----"
	}

	return fmt.Sprintf("%02.0f/%02.0f/%.0f", day, month, year)
}

func formatSimulationTime(seconds float64) string {
	if seconds < 0 {
		return "--:--:--"
	}

	// Format simulation running time
	totalHours := int(seconds / 3600)
	minutes := int((seconds - float64(totalHours*3600)) / 60)
	secs := int(seconds) % 60

	if totalHours >= 24 {
		days := totalHours / 24
		hours := totalHours % 24
		return fmt.Sprintf("%dd %02d:%02d:%02d", days, hours, minutes, secs)
	}

	return fmt.Sprintf("%02d:%02d:%02d", totalHours, minutes, secs)
}

func startWebServer() {
	http.HandleFunc("/", serveIndex)
	http.HandleFunc("/api/camera-state", serveCameraState)
	http.HandleFunc("/api/set-camera-state", setCameraState)
	http.HandleFunc("/api/set-camera-view-type", setCameraViewType)
	http.HandleFunc("/api/set-camera-parameter", setCameraParameter)
	http.HandleFunc("/api/camera-action", performCameraAction)

	if verbose {
		fmt.Printf("Starting web server on port %s...\n", WEB_PORT)
	}
	log.Fatal(http.ListenAndServe(":"+WEB_PORT, nil))
}

func setCameraState(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		State int `json:"state"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Set camera state using proper SimConnect approach
	data := CameraStateData{CameraState: float64(request.State)}
	dataPtr := uintptr(unsafe.Pointer(&data))

	err := simClient.SetDataOnSimObject(
		CAMERA_STATE_SET_DEFINITION,
		int(types.SIMCONNECT_OBJECT_ID_USER), // User aircraft
		types.SIMCONNECT_DATA_SET_FLAG_DEFAULT,
		0,                        // Array count
		int(unsafe.Sizeof(data)), // Size
		dataPtr,
	)

	if err != nil {
		if verbose {
			fmt.Printf("Error setting camera state: %v\n", err)
		}
		http.Error(w, "Failed to set camera state", http.StatusInternalServerError)
		return
	}

	if verbose {
		fmt.Printf("Camera state set to: %d\n", request.State)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func setCameraViewType(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		ViewType int `json:"viewType"`
		Index    int `json:"index"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Set view type first
	viewTypeDefID := CAMERA_PARAM_SET_DEFINITION + 300
	simClient.ClearDataDefinition(viewTypeDefID)

	err := simClient.AddToDataDefinition(viewTypeDefID, "CAMERA VIEW TYPE AND INDEX:0", "enum", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0)
	if err != nil {
		if verbose {
			fmt.Printf("Error adding view type definition: %v\n", err)
		}
		http.Error(w, "Failed to setup view type definition", http.StatusInternalServerError)
		return
	}

	data := CameraParameterData{Value: float64(request.ViewType)}
	dataPtr := uintptr(unsafe.Pointer(&data))

	err = simClient.SetDataOnSimObject(
		viewTypeDefID,
		int(types.SIMCONNECT_OBJECT_ID_USER),
		types.SIMCONNECT_DATA_SET_FLAG_DEFAULT,
		0,
		int(unsafe.Sizeof(data)),
		dataPtr,
	)

	if err != nil {
		if verbose {
			fmt.Printf("Error setting view type: %v\n", err)
		}
		http.Error(w, "Failed to set view type", http.StatusInternalServerError)
		return
	}

	// Set view index second
	viewIndexDefID := CAMERA_PARAM_SET_DEFINITION + 301
	simClient.ClearDataDefinition(viewIndexDefID)

	err = simClient.AddToDataDefinition(viewIndexDefID, "CAMERA VIEW TYPE AND INDEX:1", "number", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0)
	if err != nil {
		if verbose {
			fmt.Printf("Error adding view index definition: %v\n", err)
		}
		http.Error(w, "Failed to setup view index definition", http.StatusInternalServerError)
		return
	}

	indexData := CameraParameterData{Value: float64(request.Index)}
	indexDataPtr := uintptr(unsafe.Pointer(&indexData))

	err = simClient.SetDataOnSimObject(
		viewIndexDefID,
		int(types.SIMCONNECT_OBJECT_ID_USER),
		types.SIMCONNECT_DATA_SET_FLAG_DEFAULT,
		0,
		int(unsafe.Sizeof(indexData)),
		indexDataPtr,
	)

	if err != nil {
		if verbose {
			fmt.Printf("Error setting view index: %v\n", err)
		}
		http.Error(w, "Failed to set view index", http.StatusInternalServerError)
		return
	}

	if verbose {
		fmt.Printf("Camera view type set to: %d, index: %d\n", request.ViewType, request.Index)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func setCameraParameter(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Parameter string  `json:"parameter"`
		Value     float64 `json:"value"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Map parameter names to SimVar names
	parameterMap := map[string]string{
		"cockpitHeight":      "COCKPIT CAMERA HEIGHT",
		"cockpitMomentum":    "COCKPIT CAMERA MOMENTUM",
		"cockpitSpeed":       "COCKPIT CAMERA SPEED",
		"cockpitZoom":        "COCKPIT CAMERA ZOOM",
		"cockpitZoomSpeed":   "COCKPIT CAMERA ZOOM SPEED",
		"chaseMomentum":      "CHASE CAMERA MOMENTUM",
		"chaseSpeed":         "CHASE CAMERA SPEED",
		"chaseZoom":          "CHASE CAMERA ZOOM",
		"chaseZoomSpeed":     "CHASE CAMERA ZOOM SPEED",
		"droneFocus":         "DRONE CAMERA FOCUS",
		"droneFOV":           "DRONE CAMERA FOV",
		"droneSpeedRotation": "DRONE CAMERA SPEED ROTATION",
		"droneSpeedTravel":   "DRONE CAMERA SPEED TRAVELLING",
	}

	simVarName, exists := parameterMap[request.Parameter]
	if !exists {
		http.Error(w, "Unknown parameter", http.StatusBadRequest)
		return
	}

	// Create a unique data definition for this parameter
	paramDefID := CAMERA_PARAM_SET_DEFINITION + 100 // Use a higher ID to avoid conflicts

	// Clear any existing definition first
	simClient.ClearDataDefinition(paramDefID) // Add the specific SimVar to the definition
	var err error
	if request.Parameter == "droneFocus" {
		err = simClient.AddToDataDefinition(paramDefID, simVarName, "number", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0)
	} else if request.Parameter == "droneFOV" ||
		request.Parameter == "droneSpeedRotation" || request.Parameter == "droneSpeedTravel" ||
		strings.Contains(request.Parameter, "cockpit") || strings.Contains(request.Parameter, "chase") {
		err = simClient.AddToDataDefinition(paramDefID, simVarName, "percentage", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0)
	} else {
		err = simClient.AddToDataDefinition(paramDefID, simVarName, "number", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0)
	}
	if err != nil {
		if verbose {
			fmt.Printf("Error adding parameter definition %s: %v\n", simVarName, err)
		}
		http.Error(w, "Failed to setup parameter definition", http.StatusInternalServerError)
		return
	}

	// Set the parameter value
	data := CameraParameterData{Value: request.Value}
	dataPtr := uintptr(unsafe.Pointer(&data))

	err = simClient.SetDataOnSimObject(
		paramDefID,
		int(types.SIMCONNECT_OBJECT_ID_USER),
		types.SIMCONNECT_DATA_SET_FLAG_DEFAULT,
		0,
		int(unsafe.Sizeof(data)),
		dataPtr,
	)
	if err != nil {
		if verbose {
			fmt.Printf("Error setting camera parameter %s to %f: %v\n", simVarName, request.Value, err)
		}
		http.Error(w, "Failed to set camera parameter", http.StatusInternalServerError)
		return
	}

	if verbose {
		fmt.Printf("Camera parameter %s set to: %f\n", simVarName, request.Value)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func performCameraAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Action string `json:"action"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	var simVarName string
	var value float64 = 1.0 // Most camera actions use 1 to trigger
	switch request.Action {
	case "resetCockpit":
		simVarName = "CAMERA REQUEST ACTION"
	case "toggleSmartCamera":
		// Get current state first to toggle
		cameraStateMutex.Lock()
		isActive := currentState.SmartCameraActive
		cameraStateMutex.Unlock()
		simVarName = "SMART CAMERA ACTIVE"
		if isActive {
			value = 0.0 // Turn off
		} else {
			value = 1.0 // Turn on
		}
	default:
		http.Error(w, "Unknown action", http.StatusBadRequest)
		return
	}

	// Create a unique data definition for this action
	actionDefID := CAMERA_PARAM_SET_DEFINITION + 200

	// Clear any existing definition first
	simClient.ClearDataDefinition(actionDefID) // Add the action SimVar to the definition
	var err error
	if request.Action == "resetCockpit" {
		// Use enum data type for CAMERA REQUEST ACTION
		err = simClient.AddToDataDefinition(actionDefID, simVarName, "enum", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0)
	} else {
		err = simClient.AddToDataDefinition(actionDefID, simVarName, "bool", types.SIMCONNECT_DATATYPE_FLOAT64, 0.0, 0)
	}

	if err != nil {
		if verbose {
			fmt.Printf("Error adding action definition %s: %v\n", simVarName, err)
		}
		http.Error(w, "Failed to setup action definition", http.StatusInternalServerError)
		return
	} // Perform the action
	if request.Action == "resetCockpit" {
		// Use float64 data structure for enum SimVar (CAMERA REQUEST ACTION = 1)
		data := CameraParameterData{Value: 1.0} // 1 = Reset Active Camera
		dataPtr := uintptr(unsafe.Pointer(&data))

		err = simClient.SetDataOnSimObject(
			actionDefID,
			int(types.SIMCONNECT_OBJECT_ID_USER),
			types.SIMCONNECT_DATA_SET_FLAG_DEFAULT,
			0,
			int(unsafe.Sizeof(data)),
			dataPtr,
		)
	} else {
		// Use float64 data structure for other actions
		data := CameraParameterData{Value: value}
		dataPtr := uintptr(unsafe.Pointer(&data))

		err = simClient.SetDataOnSimObject(
			actionDefID,
			int(types.SIMCONNECT_OBJECT_ID_USER),
			types.SIMCONNECT_DATA_SET_FLAG_DEFAULT,
			0,
			int(unsafe.Sizeof(data)),
			dataPtr,
		)
	}

	if err != nil {
		if verbose {
			fmt.Printf("Error performing camera action %s: %v\n", request.Action, err)
		}
		http.Error(w, "Failed to perform camera action", http.StatusInternalServerError)
		return
	}

	if verbose {
		fmt.Printf("Camera action %s performed\n", request.Action)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func serveCameraState(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	cameraStateMutex.Lock()
	defer cameraStateMutex.Unlock()

	if currentState == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "No data available",
		})
		return
	}

	json.NewEncoder(w).Encode(currentState)
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	htmlTemplate := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Camera Controller</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <script>
        tailwind.config = {
            theme: {
                extend: {
                    colors: {
                        'camera-blue': '#1e40af',
                        'control-green': '#16a34a',
                        'monitor-purple': '#7c3aed',
                        'neon-blue': '#00d4ff',
                        'neon-green': '#00ff88',
                        'neon-orange': '#ff8800',
                        'neon-purple': '#bb00ff'
                    },
                    animation: {
                        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
                        'fade-in': 'fadeIn 0.5s ease-in-out',
                        'slide-up': 'slideUp 0.3s ease-out'
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
        .control-section {
            background: rgba(34, 197, 94, 0.1);
            border: 1px solid rgba(34, 197, 94, 0.2);
        }        .slider-container {
            background: rgba(55, 65, 81, 0.8);
            border-radius: 12px;
            padding: 16px;
            margin: 8px 0;
        }
        .camera-slider {
            height: 6px;
            -webkit-appearance: none;
            appearance: none;
            background: linear-gradient(to right, #374151, #6b7280);
            border-radius: 5px;
            outline: none;
            cursor: pointer;
            transition: all 0.3s ease;
        }
        .camera-slider::-webkit-slider-thumb {
            -webkit-appearance: none;
            appearance: none;
            width: 20px;
            height: 20px;
            border-radius: 50%;
            background: linear-gradient(135deg, #3b82f6, #1e40af);
            cursor: pointer;
            border: 2px solid #ffffff;
            box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
            transition: all 0.2s ease;
        }
        .camera-slider::-webkit-slider-thumb:hover {
            background: linear-gradient(135deg, #2563eb, #1d4ed8);
            transform: scale(1.1);
            box-shadow: 0 4px 12px rgba(59, 130, 246, 0.4);
        }
        .camera-slider::-moz-range-thumb {
            width: 20px;
            height: 20px;
            border-radius: 50%;
            background: linear-gradient(135deg, #3b82f6, #1e40af);
            cursor: pointer;
            border: 2px solid #ffffff;
            box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
            transition: all 0.2s ease;
        }
        .camera-slider::-moz-range-thumb:hover {
            background: linear-gradient(135deg, #2563eb, #1d4ed8);
            transform: scale(1.1);
            box-shadow: 0 4px 12px rgba(59, 130, 246, 0.4);
        }
        .camera-slider:focus {
            box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.3);
        }
        .camera-button {
            background: linear-gradient(135deg, #3b82f6, #1d4ed8);
            transition: all 0.3s ease;
            border: 2px solid transparent;
        }
        .camera-button:hover {
            background: linear-gradient(135deg, #2563eb, #1e40af);
            border-color: #60a5fa;
            transform: translateY(-2px);
        }
        .camera-button.active {
            background: linear-gradient(135deg, #16a34a, #15803d);
            border-color: #4ade80;
        }
    </style>
</head>
<body class="bg-gray-900 text-white min-h-screen gradient-bg">
    <div class="container-fluid mx-auto px-6 py-6">
        <!-- Header -->
        <div class="mb-8 text-center animate-fade-in">
            <h1 class="text-5xl font-bold bg-gradient-to-r from-blue-400 via-purple-500 to-cyan-400 bg-clip-text text-transparent mb-4">
                📹 Camera Controller
            </h1>
            <p class="text-xl text-gray-300 mb-6">Real-time camera monitoring and control for Microsoft Flight Simulator</p>
            <div class="glass-effect rounded-xl px-6 py-4 inline-block">
                <div class="flex justify-center items-center space-x-6">
                    <div class="flex items-center">
                        <div id="connection-indicator" class="w-4 h-4 bg-green-400 rounded-full mr-3 animate-pulse"></div>
                        <span id="connection-status" class="text-lg font-medium">Connected</span>
                    </div>
                    <div class="text-lg text-gray-300">
                        Last Update: <span id="last-update" class="text-cyan-400 font-mono">--</span>
                    </div>
                </div>
            </div>
        </div>        <!-- Main Dashboard Grid -->
        <div class="max-w-screen-2xl mx-auto">            <!-- Top Row: Camera State Monitor and Control Panel -->
            <div class="grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-2 2xl:grid-cols-2 gap-10 mb-10">
                
                <!-- Camera State Monitor -->
                <div class="animate-slide-up">
                    <div class="data-card rounded-2xl p-6 shadow-2xl border border-blue-500/20">
                        <div class="panel-header -mx-6 -mt-6 px-6 py-4 rounded-t-2xl mb-6">
                            <h2 class="text-2xl font-bold text-blue-400 flex items-center">
                                <span class="mr-3">📊</span> Camera State Monitor
                            </h2>
                        </div>
                        <div class="space-y-4">
                            <div class="data-row flex justify-between items-center p-4 rounded-xl">
                                <span class="text-gray-300 font-medium">🎥 Camera State</span>
                                <span id="camera-state" class="font-mono text-blue-400 text-xl font-bold">--</span>
                            </div>
                            <div class="data-row flex justify-between items-center p-4 rounded-xl">
                                <span class="text-gray-300 font-medium">🔧 Camera Substate</span>
                                <span id="camera-substate" class="font-mono text-purple-400 text-xl font-bold">--</span>
                            </div>
                            <div class="data-row flex justify-between items-center p-4 rounded-xl">
                                <span class="text-gray-300 font-medium">👁️ View Type</span>
                                <span id="view-type" class="font-mono text-green-400 text-xl font-bold">--</span>
                            </div>
                            <div class="data-row flex justify-between items-center p-4 rounded-xl">
                                <span class="text-gray-300 font-medium">📋 View Index</span>
                                <span id="view-index" class="font-mono text-yellow-400 text-xl font-bold">--</span>
                            </div>                            <div class="data-row flex justify-between items-center p-4 rounded-xl">
                                <span class="text-gray-300 font-medium">📐 Gameplay Pitch</span>
                                <span id="gameplay-pitch" class="font-mono text-orange-400 text-xl font-bold">--°</span>
                            </div>
                            <div class="data-row flex justify-between items-center p-4 rounded-xl">
                                <span class="text-gray-300 font-medium">🧭 Gameplay Yaw</span>
                                <span id="gameplay-yaw" class="font-mono text-cyan-400 text-xl font-bold">--°</span>
                            </div>                            <div class="data-row flex justify-between items-center p-4 rounded-xl">
                                <span class="text-gray-300 font-medium">🧠 Smart Camera</span>
                                <span id="smart-camera-active" class="font-mono text-purple-400 text-xl font-bold">--</span>
                            </div>
                            <div class="data-row flex justify-between items-center p-4 rounded-xl">
                                <span class="text-gray-300 font-medium">🕐 Local Time</span>
                                <span id="local-time" class="font-mono text-cyan-400 text-xl font-bold">--:--:--</span>
                            </div>                            <div class="data-row flex justify-between items-center p-4 rounded-xl">
                                <span class="text-gray-300 font-medium">📅 Local Date</span>
                                <span id="local-date" class="font-mono text-green-400 text-xl font-bold">--/--/----</span>
                            </div>
                            <div class="data-row flex justify-between items-center p-4 rounded-xl">
                                <span class="text-gray-300 font-medium">⏱️ Simulation Time</span>
                                <span id="simulation-time" class="font-mono text-yellow-400 text-xl font-bold">--:--:--</span>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Camera Control Panel -->
                <div class="animate-slide-up" style="animation-delay: 0.1s;">
                    <div class="data-card rounded-2xl p-6 shadow-2xl border border-green-500/20">
                        <div class="control-section -mx-6 -mt-6 px-6 py-4 rounded-t-2xl mb-6">
                            <h2 class="text-2xl font-bold text-green-400 flex items-center">
                                <span class="mr-3">🎮</span> Camera Control Panel
                            </h2>
                        </div>
                          <!-- Camera State Buttons -->
                        <div class="mb-6">
                            <h3 class="text-lg font-semibold text-green-400 mb-4">🎥 Camera State</h3>
                            <div class="grid grid-cols-2 gap-3">
                                <button onclick="setCameraState(2)" class="camera-button px-4 py-3 rounded-xl font-semibold text-white">
                                    🏠 Cockpit
                                </button>
                                <button onclick="setCameraState(3)" class="camera-button px-4 py-3 rounded-xl font-semibold text-white">
                                    🚁 External
                                </button>
                                <button onclick="setCameraState(4)" class="camera-button px-4 py-3 rounded-xl font-semibold text-white">
                                    🛸 Drone
                                </button>
                                <button onclick="setCameraState(5)" class="camera-button px-4 py-3 rounded-xl font-semibold text-white">
                                    📌 Fixed on Plane
                                </button>
                                <button onclick="setCameraState(6)" class="camera-button px-4 py-3 rounded-xl font-semibold text-white">
                                    🌍 Environment
                                </button>                                <button onclick="setCameraState(7)" class="camera-button px-4 py-3 rounded-xl font-semibold text-white">
                                    🎮 Six DoF
                                </button>
                            </div>                        </div>

                        <!-- Camera View Controls -->
                        <div class="mb-6">
                            <h3 class="text-lg font-semibold text-orange-400 mb-4">👁️ Camera View Controls</h3>
                            
                            <!-- View Type Buttons -->
                            <div class="mb-4">
                                <label class="block text-orange-300 font-medium mb-2">View Type:</label>
                                <div class="grid grid-cols-2 gap-2">
                                    <button onclick="setCameraViewType(1, 1)" class="camera-button px-3 py-2 rounded-lg font-medium text-white text-sm bg-gradient-to-r from-blue-600 to-blue-700 hover:from-blue-500 hover:to-blue-600">
                                        🧑‍✈️ Pilot
                                    </button>
                                    <button onclick="setCameraViewType(2, 0)" class="camera-button px-3 py-2 rounded-lg font-medium text-white text-sm bg-gradient-to-r from-green-600 to-green-700 hover:from-green-500 hover:to-green-600">
                                        📊 Instruments
                                    </button>
                                    <button onclick="setCameraViewType(3, 0)" class="camera-button px-3 py-2 rounded-lg font-medium text-white text-sm bg-gradient-to-r from-yellow-600 to-yellow-700 hover:from-yellow-500 hover:to-yellow-600">
                                        ⚡ Quickview
                                    </button>
                                    <button onclick="setCameraViewType(4, 0)" class="camera-button px-3 py-2 rounded-lg font-medium text-white text-sm bg-gradient-to-r from-purple-600 to-purple-700 hover:from-purple-500 hover:to-purple-600">
                                        🌍 Ext.Quickview
                                    </button>
                                </div>
                            </div>

                            <!-- Quick Pilot Presets -->
                            <div class="mb-4">
                                <label class="block text-orange-300 font-medium mb-2">Quick Pilot Presets:</label>
                                <div class="grid grid-cols-2 gap-2">
                                    <button onclick="setCameraViewType(1, 0)" class="camera-button px-3 py-2 rounded-lg font-medium text-white text-sm bg-gradient-to-r from-gray-600 to-gray-700 hover:from-gray-500 hover:to-gray-600">
                                        👀 Close
                                    </button>
                                    <button onclick="setCameraViewType(1, 1)" class="camera-button px-3 py-2 rounded-lg font-medium text-white text-sm bg-gradient-to-r from-gray-600 to-gray-700 hover:from-gray-500 hover:to-gray-600">
                                        🎯 Normal
                                    </button>
                                    <button onclick="setCameraViewType(1, 3)" class="camera-button px-3 py-2 rounded-lg font-medium text-white text-sm bg-gradient-to-r from-gray-600 to-gray-700 hover:from-gray-500 hover:to-gray-600">
                                        🛬 Landing
                                    </button>
                                    <button onclick="setCameraViewType(1, 4)" class="camera-button px-3 py-2 rounded-lg font-medium text-white text-sm bg-gradient-to-r from-gray-600 to-gray-700 hover:from-gray-500 hover:to-gray-600">
                                        👨‍✈️ Copilot
                                    </button>
                                </div>
                            </div>

                            <!-- Manual Index Control -->
                            <div>
                                <label class="block text-orange-300 font-medium mb-2">
                                    View Index: <span id="view-index-display" class="text-yellow-400">Option 0</span>
                                </label>
                                <div class="flex items-center space-x-3">
                                    <input type="range" id="view-index-slider" min="0" max="10" value="0" step="1"
                                           class="flex-1 camera-slider"
                                           oninput="updateViewIndexDisplay(this.value)"
                                           onchange="setCameraViewTypeCustom(this.value)">
                                    <span id="view-index-value" class="text-yellow-400 font-mono min-w-[3ch]">0</span>
                                </div>
                            </div>
                        </div>

                        <!-- Quick Actions -->
                        <div class="mb-6">
                            <h3 class="text-lg font-semibold text-purple-400 mb-4">⚡ Quick Actions</h3>
                            <div class="grid grid-cols-1 gap-3">                                <button onclick="resetCockpitView()" class="camera-button px-4 py-3 rounded-xl font-semibold text-white bg-gradient-to-r from-purple-600 to-purple-800">
                                    🎯 Center View
                                </button>
                                <button onclick="toggleSmartCamera()" class="camera-button px-4 py-3 rounded-xl font-semibold text-white bg-gradient-to-r from-cyan-600 to-cyan-800">
                                    🧠 Toggle Smart Camera
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            </div>            <!-- Bottom Row: Camera Control Panels -->
            <div class="grid grid-cols-1 lg:grid-cols-3 xl:grid-cols-3 2xl:grid-cols-3 gap-8">

                <!-- Cockpit Camera Controls -->
                <div class="animate-slide-up" style="animation-delay: 0.2s;">
                    <div class="data-card rounded-2xl p-6 shadow-2xl border border-yellow-500/20">
                        <div class="panel-header -mx-6 -mt-6 px-6 py-4 rounded-t-2xl mb-6" style="background: linear-gradient(90deg, rgba(245, 158, 11, 0.1), rgba(245, 158, 11, 0.05)); border-bottom: 1px solid rgba(245, 158, 11, 0.2);">
                            <h2 class="text-2xl font-bold text-yellow-400 flex items-center">
                                <span class="mr-3">🏠</span> Cockpit Camera Controls
                            </h2>
                        </div>
                        <div class="space-y-4">                            <!-- Cockpit Height -->
                            <div class="slider-container">
                                <label class="block text-yellow-300 font-semibold mb-2">📏 Height (<span id="cockpit-height-value">50</span>%)</label>
                                <input type="range" id="cockpit-height" min="0" max="100" value="50" step="1"
                                       class="w-full camera-slider"
                                       onmousedown="onSliderStart('cockpit-height')"
                                       onmouseup="onSliderEnd('cockpit-height')"
                                       ontouchstart="onSliderStart('cockpit-height')"
                                       ontouchend="onSliderEnd('cockpit-height')"
                                       oninput="updateSliderDisplay('cockpit-height', this.value)"
                                       onchange="setCameraParameter('cockpitHeight', this.value)">
                            </div>
                              <!-- Cockpit Zoom -->
                            <div class="slider-container">
                                <label class="block text-yellow-300 font-semibold mb-2">🔍 Zoom (<span id="cockpit-zoom-value">50</span>%)</label>
                                <input type="range" id="cockpit-zoom" min="0" max="100" value="50" step="1"
                                       class="w-full camera-slider"
                                       onmousedown="onSliderStart('cockpit-zoom')"
                                       onmouseup="onSliderEnd('cockpit-zoom')"
                                       ontouchstart="onSliderStart('cockpit-zoom')"
                                       ontouchend="onSliderEnd('cockpit-zoom')"
                                       oninput="updateSliderDisplay('cockpit-zoom', this.value)"
                                       onchange="setCameraParameter('cockpitZoom', this.value)">
                            </div>                            <!-- Cockpit Speed -->
                            <div class="slider-container">
                                <label class="block text-yellow-300 font-semibold mb-2">⚡ Speed (<span id="cockpit-speed-value">50</span>%)</label>
                                <input type="range" id="cockpit-speed" min="0" max="100" value="50" step="1"
                                       class="w-full camera-slider"
                                       onmousedown="onSliderStart('cockpit-speed')"
                                       onmouseup="onSliderEnd('cockpit-speed')"
                                       ontouchstart="onSliderStart('cockpit-speed')"
                                       ontouchend="onSliderEnd('cockpit-speed')"
                                       oninput="updateSliderDisplay('cockpit-speed', this.value)"
                                       onchange="setCameraParameter('cockpitSpeed', this.value)">
                            </div>
                              <!-- Cockpit Momentum -->
                            <div class="slider-container">
                                <label class="block text-yellow-300 font-semibold mb-2">🎯 Momentum (<span id="cockpit-momentum-value">50</span>%)</label>
                                <input type="range" id="cockpit-momentum" min="0" max="100" value="50" step="1"
                                       class="w-full camera-slider"
                                       onmousedown="onSliderStart('cockpit-momentum')"
                                       onmouseup="onSliderEnd('cockpit-momentum')"
                                       ontouchstart="onSliderStart('cockpit-momentum')"
                                       ontouchend="onSliderEnd('cockpit-momentum')"
                                       oninput="updateSliderDisplay('cockpit-momentum', this.value)"
                                       onchange="setCameraParameter('cockpitMomentum', this.value)">
                            </div>
                              <!-- Cockpit Zoom Speed -->
                            <div class="slider-container">
                                <label class="block text-yellow-300 font-semibold mb-2">🔄 Zoom Speed (<span id="cockpit-zoom-speed-value">50</span>%)</label>
                                <input type="range" id="cockpit-zoom-speed" min="0" max="100" value="50" step="1"
                                       class="w-full camera-slider"
                                       onmousedown="onSliderStart('cockpit-zoom-speed')"
                                       onmouseup="onSliderEnd('cockpit-zoom-speed')"
                                       ontouchstart="onSliderStart('cockpit-zoom-speed')"
                                       ontouchend="onSliderEnd('cockpit-zoom-speed')"
                                       oninput="updateSliderDisplay('cockpit-zoom-speed', this.value)"
                                       onchange="setCameraParameter('cockpitZoomSpeed', this.value)">
                            </div>
                            
                            <!-- Status Display -->
                            <div class="mt-6 space-y-2">
                                <div class="data-row flex justify-between items-center p-3 rounded-lg">
                                    <span class="text-gray-300">🎯 Headlook Mode</span>
                                    <span id="cockpit-headlook" class="font-mono text-yellow-400 font-bold">--</span>
                                </div>
                                <div class="data-row flex justify-between items-center p-3 rounded-lg">
                                    <span class="text-gray-300">⬆️ Upper Position</span>
                                    <span id="cockpit-upper" class="font-mono text-yellow-400 font-bold">--</span>
                                </div>
                            </div>                        </div>
                    </div>
                </div>                <!-- Chase Camera Controls -->
                <div class="animate-slide-up" style="animation-delay: 0.25s;">
                    <div class="data-card rounded-2xl p-6 shadow-2xl border border-orange-500/20">
                        <div class="panel-header -mx-6 -mt-6 px-6 py-4 rounded-t-2xl mb-6" style="background: linear-gradient(90deg, rgba(249, 115, 22, 0.1), rgba(249, 115, 22, 0.05)); border-bottom: 1px solid rgba(249, 115, 22, 0.2);">
                            <h2 class="text-2xl font-bold text-orange-400 flex items-center">
                                <span class="mr-3">🚁</span> Chase Camera Controls
                            </h2>
                        </div>
                        <div class="space-y-4">                            <!-- Chase Zoom -->
                            <div class="slider-container">
                                <label class="block text-orange-300 font-semibold mb-2">🔍 Zoom (<span id="chase-zoom-value">50</span>%)</label>
                                <input type="range" id="chase-zoom" min="0" max="100" value="50" step="1"
                                       class="w-full camera-slider"
                                       onmousedown="onSliderStart('chase-zoom')"
                                       onmouseup="onSliderEnd('chase-zoom')"
                                       ontouchstart="onSliderStart('chase-zoom')"
                                       ontouchend="onSliderEnd('chase-zoom')"
                                       oninput="updateSliderDisplay('chase-zoom', this.value)"
                                       onchange="setCameraParameter('chaseZoom', this.value)">
                            </div>
                              <!-- Chase Speed -->
                            <div class="slider-container">
                                <label class="block text-orange-300 font-semibold mb-2">⚡ Speed (<span id="chase-speed-value">50</span>%)</label>
                                <input type="range" id="chase-speed" min="0" max="100" value="50" step="1"
                                       class="w-full camera-slider"
                                       onmousedown="onSliderStart('chase-speed')"
                                       onmouseup="onSliderEnd('chase-speed')"
                                       ontouchstart="onSliderStart('chase-speed')"
                                       ontouchend="onSliderEnd('chase-speed')"
                                       oninput="updateSliderDisplay('chase-speed', this.value)"
                                       onchange="setCameraParameter('chaseSpeed', this.value)">
                            </div>
                              <!-- Chase Momentum -->
                            <div class="slider-container">
                                <label class="block text-orange-300 font-semibold mb-2">🎯 Momentum (<span id="chase-momentum-value">50</span>%)</label>
                                <input type="range" id="chase-momentum" min="0" max="100" value="50" step="1"
                                       class="w-full camera-slider"
                                       onmousedown="onSliderStart('chase-momentum')"
                                       onmouseup="onSliderEnd('chase-momentum')"
                                       ontouchstart="onSliderStart('chase-momentum')"
                                       ontouchend="onSliderEnd('chase-momentum')"
                                       oninput="updateSliderDisplay('chase-momentum', this.value)"
                                       onchange="setCameraParameter('chaseMomentum', this.value)">
                            </div>
                              <!-- Chase Zoom Speed -->
                            <div class="slider-container">
                                <label class="block text-orange-300 font-semibold mb-2">🔄 Zoom Speed (<span id="chase-zoom-speed-value">50</span>%)</label>
                                <input type="range" id="chase-zoom-speed" min="0" max="100" value="50" step="1"
                                       class="w-full camera-slider"
                                       onmousedown="onSliderStart('chase-zoom-speed')"
                                       onmouseup="onSliderEnd('chase-zoom-speed')"
                                       ontouchstart="onSliderStart('chase-zoom-speed')"
                                       ontouchend="onSliderEnd('chase-zoom-speed')"
                                       oninput="updateSliderDisplay('chase-zoom-speed', this.value)"
                                       onchange="setCameraParameter('chaseZoomSpeed', this.value)">
                            </div>
                            
                            <!-- Status Display -->
                            <div class="mt-6 space-y-2">
                                <div class="data-row flex justify-between items-center p-3 rounded-lg">
                                    <span class="text-gray-300">🎯 Headlook Mode</span>
                                    <span id="chase-headlook" class="font-mono text-orange-400 font-bold">--</span>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>                <!-- Drone Camera Controls -->
                <div class="animate-slide-up" style="animation-delay: 0.3s;">
                    <div class="data-card rounded-2xl p-6 shadow-2xl border border-cyan-500/20">
                        <div class="panel-header -mx-6 -mt-6 px-6 py-4 rounded-t-2xl mb-6" style="background: linear-gradient(90deg, rgba(6, 182, 212, 0.1), rgba(6, 182, 212, 0.05)); border-bottom: 1px solid rgba(6, 182, 212, 0.2);">
                            <h2 class="text-2xl font-bold text-cyan-400 flex items-center">
                                <span class="mr-3">🛸</span> Drone Camera Controls
                            </h2>
                        </div>
                        <div class="space-y-4">                            <!-- Drone FOV -->
                            <div class="slider-container">
                                <label class="block text-cyan-300 font-semibold mb-2">📷 Field of View (<span id="drone-fov-value">50</span>%)</label>
                                <input type="range" id="drone-fov" min="0" max="100" value="50" step="1"
                                       class="w-full camera-slider"
                                       onmousedown="onSliderStart('drone-fov')"
                                       onmouseup="onSliderEnd('drone-fov')"
                                       ontouchstart="onSliderStart('drone-fov')"
                                       ontouchend="onSliderEnd('drone-fov')"
                                       oninput="updateSliderDisplay('drone-fov', this.value)"
                                       onchange="setCameraParameter('droneFOV', this.value)">
                            </div>                            <!-- Drone Focus -->
                            <div class="slider-container">                                <label class="block text-cyan-300 font-semibold mb-2">🎯 Focus (<span id="drone-focus-value">5000</span>)</label>
                                <input type="range" id="drone-focus" min="0" max="50" value="25" step="1"
                                       class="w-full camera-slider"
                                       onmousedown="onSliderStart('drone-focus')"
                                       onmouseup="onSliderEnd('drone-focus')"
                                       ontouchstart="onSliderStart('drone-focus')"
                                       ontouchend="onSliderEnd('drone-focus')"
                                       oninput="updateSliderDisplay('drone-focus', this.value)"
                                       onchange="setCameraParameter('droneFocus', this.value)">
                            </div>                            <!-- Drone Speed Controls -->
                            <div class="slider-container">
                                <label class="block text-cyan-300 font-semibold mb-2">🔄 Rotation Speed (<span id="drone-rotation-value">90</span>°)</label>
                                <input type="range" id="drone-rotation" min="0" max="180" value="90" step="1"
                                       class="w-full camera-slider"
                                       onmousedown="onSliderStart('drone-rotation')"
                                       onmouseup="onSliderEnd('drone-rotation')"
                                       ontouchstart="onSliderStart('drone-rotation')"
                                       ontouchend="onSliderEnd('drone-rotation')"
                                       oninput="updateSliderDisplay('drone-rotation', this.value)"
                                       onchange="setCameraParameter('droneSpeedRotation', this.value)">
                            </div>                            <div class="slider-container">
                                <label class="block text-cyan-300 font-semibold mb-2">🚀 Travel Speed (<span id="drone-travel-value">125</span>)</label>
                                <input type="range" id="drone-travel" min="0" max="250" value="125" step="1"
                                       class="w-full camera-slider"
                                       onmousedown="onSliderStart('drone-travel')"
                                       onmouseup="onSliderEnd('drone-travel')"
                                       ontouchstart="onSliderStart('drone-travel')"
                                       ontouchend="onSliderEnd('drone-travel')"
                                       oninput="updateSliderDisplay('drone-travel', this.value)"
                                       onchange="setCameraParameter('droneSpeedTravel', this.value)">
                            </div>
                            
                            <!-- Status Display -->
                            <div class="mt-6 space-y-2">
                                <div class="data-row flex justify-between items-center p-3 rounded-lg">
                                    <span class="text-gray-300">🔒 Locked</span>
                                    <span id="drone-locked" class="font-mono text-cyan-400 font-bold">--</span>
                                </div>
                                <div class="data-row flex justify-between items-center p-3 rounded-lg">
                                    <span class="text-gray-300">🎯 Follow Mode</span>
                                    <span id="drone-follow" class="font-mono text-cyan-400 font-bold">--</span>
                                </div>
                                <div class="data-row flex justify-between items-center p-3 rounded-lg">
                                    <span class="text-gray-300">🎯 Focus Mode</span>
                                    <span id="drone-focus-mode" class="font-mono text-cyan-400 font-bold">--</span>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

            </div>
        </div>
    </div>    <script>
        let lastUpdateTime = new Date();
        let activeSliders = new Set(); // Track which sliders are being actively used
        
        function formatNumber(value, decimals = 1) {
            if (value === null || value === undefined || isNaN(value)) return '--';
            return Number(value).toFixed(decimals);
        }

        function updateData() {
            fetch('/api/camera-state')
                .then(response => response.json())
                .then(data => {
                    // Update camera state monitor
                    document.getElementById('camera-state').textContent = data.stateText + ' (' + data.cameraState + ')';
                    document.getElementById('camera-substate').textContent = data.substateText + ' (' + data.cameraSubstate + ')';
                    document.getElementById('view-type').textContent = data.viewTypeText + ' (' + data.viewType + ')';
                    document.getElementById('view-index').textContent = data.viewIndex + '/' + data.viewMaxIndex;
                    document.getElementById('gameplay-pitch').textContent = formatNumber(data.gameplayPitch, 1) + '°';
                    document.getElementById('gameplay-yaw').textContent = formatNumber(data.gameplayYaw, 1) + '°';
                    document.getElementById('smart-camera-active').textContent = data.smartCameraActive ? 'ACTIVE' : 'INACTIVE';                    // Update environment variables
                    document.getElementById('local-time').textContent = data.localTime || '--:--:--';
                    document.getElementById('local-date').textContent = data.localDate || '--/--/----';
                    document.getElementById('simulation-time').textContent = data.simulationTime || '--:--:--';

                    // Update view index slider to reflect current state
                    const viewIndexSlider = document.getElementById('view-index-slider');
                    const viewIndexDisplay = document.getElementById('view-index-display');
                    const viewIndexValue = document.getElementById('view-index-value');
                    if (viewIndexSlider && data.viewIndex !== undefined) {
                        viewIndexSlider.value = data.viewIndex;
                        viewIndexSlider.max = data.viewMaxIndex || 10;
                        if (viewIndexDisplay) viewIndexDisplay.textContent = 'Option ' + data.viewIndex;
                        if (viewIndexValue) viewIndexValue.textContent = data.viewIndex;                    }
                    
                    // Update cockpit camera controls
                    updateSliderValue('cockpit-height', data.cockpitHeight);
                    updateSliderValue('cockpit-zoom', data.cockpitZoom);
                    updateSliderValue('cockpit-speed', data.cockpitSpeed);
                    updateSliderValue('cockpit-momentum', data.cockpitMomentum);
                    updateSliderValue('cockpit-zoom-speed', data.cockpitZoomSpeed);
                    
                    // Update cockpit status
                    document.getElementById('cockpit-headlook').textContent = getHeadlookText(data.cockpitHeadlook);
                    document.getElementById('cockpit-upper').textContent = data.cockpitUpperPosition ? 'YES' : 'NO';
                    
                    // Update chase camera controls
                    updateSliderValue('chase-zoom', data.chaseZoom);
                    updateSliderValue('chase-speed', data.chaseSpeed);
                    updateSliderValue('chase-momentum', data.chaseMomentum);
                    updateSliderValue('chase-zoom-speed', data.chaseZoomSpeed);
                    
                    // Update chase status
                    document.getElementById('chase-headlook').textContent = getHeadlookText(data.chaseHeadlook);                    // Update drone camera controls
                    updateSliderValue('drone-fov', data.droneFOV);
                    // Use raw values directly - no scaling needed
                    updateSliderValue('drone-focus', data.droneFocus);
                    updateSliderValue('drone-rotation', data.droneSpeedRotation);
                    updateSliderValue('drone-travel', data.droneSpeedTravel);
                    
                    // Update drone status
                    document.getElementById('drone-locked').textContent = data.droneLocked ? 'YES' : 'NO';
                    document.getElementById('drone-follow').textContent = data.droneFollow ? 'YES' : 'NO';
                    document.getElementById('drone-focus-mode').textContent = getFocusModeText(data.droneFocusMode);
                    
                    // Update connection status
                    document.getElementById('connection-indicator').className = 'w-4 h-4 bg-green-400 rounded-full mr-3 animate-pulse';
                    document.getElementById('connection-status').textContent = 'Connected';
                    lastUpdateTime = new Date();
                    document.getElementById('last-update').textContent = lastUpdateTime.toLocaleTimeString();
                    
                    // Highlight active camera button
                    updateCameraButtons(data.cameraState);
                })
                .catch(error => {
                    console.error('Error fetching data:', error);
                    document.getElementById('connection-indicator').className = 'w-4 h-4 bg-red-500 rounded-full mr-3';
                    document.getElementById('connection-status').textContent = 'Disconnected';
                });
        }
          function updateSliderValue(sliderId, value) {
            // Don't update slider if user is actively dragging it
            if (activeSliders.has(sliderId)) {
                return;
            }
              const slider = document.getElementById(sliderId);
            const valueSpan = document.getElementById(sliderId + '-value');
            if (slider && valueSpan) {
                let normalizedValue = value;
                  // Handle different ranges for drone controls
                if (sliderId === 'drone-focus') {
                    // Drone focus: Keep raw SimConnect value (0-10000 range)
                    if (normalizedValue < 0) normalizedValue = 0;
                    if (normalizedValue > 10000) normalizedValue = 10000;
                    // Display the raw value from SimConnect
                } else if (sliderId === 'drone-rotation') {
                    // Drone rotation: 0-180 range
                    if (normalizedValue < 0) normalizedValue = 0;
                    if (normalizedValue > 180) normalizedValue = 180;
                } else if (sliderId === 'drone-travel') {
                    // Drone travel: 0-250 range
                    if (normalizedValue < 0) normalizedValue = 0;
                    if (normalizedValue > 250) normalizedValue = 250;
                } else {
                    // Standard percentage controls: 0-100 range
                    if (normalizedValue < 0) normalizedValue = 0;
                    if (normalizedValue > 100) normalizedValue = 100;
                }
                
                slider.value = Math.round(normalizedValue);
                valueSpan.textContent = Math.round(normalizedValue);
            }
        }
          function updateSliderDisplay(sliderId, value) {
            // Update the display value immediately for responsive feedback
            const valueSpan = document.getElementById(sliderId + '-value');
            if (valueSpan) {
                let normalizedValue = parseFloat(value);
                
                // Handle different ranges for drone controls
                if (sliderId === 'drone-focus') {
                    // Drone focus: 0-50 range
                    if (normalizedValue < 0) normalizedValue = 0;
                    if (normalizedValue > 50) normalizedValue = 50;
                } else if (sliderId === 'drone-rotation') {
                    // Drone rotation: 0-180 range
                    if (normalizedValue < 0) normalizedValue = 0;
                    if (normalizedValue > 180) normalizedValue = 180;
                } else if (sliderId === 'drone-travel') {
                    // Drone travel: 0-250 range
                    if (normalizedValue < 0) normalizedValue = 0;
                    if (normalizedValue > 250) normalizedValue = 250;
                } else {
                    // Standard percentage controls: 0-100 range
                    if (normalizedValue < 0) normalizedValue = 0;
                    if (normalizedValue > 100) normalizedValue = 100;
                }
                
                valueSpan.textContent = Math.round(normalizedValue);
            }
        }
        
        function onSliderStart(sliderId) {
            // Mark this slider as active when user starts dragging
            activeSliders.add(sliderId);
        }
        
        function onSliderEnd(sliderId) {
            // Mark this slider as inactive when user stops dragging
            setTimeout(() => {
                activeSliders.delete(sliderId);
            }, 200); // Small delay to prevent immediate overwrite
        }
        
        function updateCameraButtons(activeState) {
            const buttons = document.querySelectorAll('.camera-button');
            buttons.forEach(button => {
                button.classList.remove('active');
            });
            
            // Find and highlight the active button based on the camera state
            const buttonTexts = {
                2: '🏠 Cockpit',
                3: '🚁 External', 
                4: '🛸 Drone',
                9: '🎬 Showcase'
            };
            
            if (buttonTexts[activeState]) {
                buttons.forEach(button => {
                    if (button.textContent.trim() === buttonTexts[activeState]) {
                        button.classList.add('active');
                    }
                });
            }
        }
        
        function getHeadlookText(value) {
            switch(value) {
                case 1: return 'Freelook';
                case 2: return 'Headlook';
                default: return 'Unknown';
            }
        }
        
        function getFocusModeText(value) {
            switch(value) {
                case 1: return 'Deactivated';
                case 2: return 'Auto';
                case 3: return 'Manual';
                default: return 'Unknown';
            }
        }
          function setCameraState(state) {
            fetch('/api/set-camera-state', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ state: state })
            })
            .then(response => response.json())
            .then(data => {
                if (data.status === 'success') {
                    console.log('Camera state set to:', state);
                } else {
                    console.error('Failed to set camera state');
                }
            })
            .catch(error => {
                console.error('Error setting camera state:', error);
            });        }

        function setCameraViewType(viewType, index) {
            fetch('/api/set-camera-view-type', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ viewType: viewType, index: index })
            })
            .then(response => response.json())
            .then(data => {
                if (data.status === 'success') {
                    console.log('Camera view type set to:', viewType, 'index:', index);
                } else {
                    console.error('Failed to set camera view type');
                }
            })
            .catch(error => {
                console.error('Error setting camera view type:', error);
            });
        }

        function updateViewIndexDisplay(value) {
            document.getElementById('view-index-display').textContent = 'Option ' + value;
            document.getElementById('view-index-value').textContent = value;
        }

        function setCameraViewTypeCustom(index) {
            // Use the current view type (default to Pilot view = 1)
            setCameraViewType(1, parseInt(index));
        }

        // Debounce mechanism to reduce API calls during slider movement
        let parameterUpdateTimeouts = {};
          function setCameraParameter(parameter, value) {
            // Clear any existing timeout for this parameter
            if (parameterUpdateTimeouts[parameter]) {
                clearTimeout(parameterUpdateTimeouts[parameter]);
            }
            
            // Set a new timeout to delay the API call
            parameterUpdateTimeouts[parameter] = setTimeout(() => {                let normalizedValue = parseFloat(value);
                  // Apply range validation and scaling for SimConnect
                if (parameter === 'droneFocus') {
                    // Drone focus: UI range 0-50, send raw value to SimConnect
                    if (normalizedValue < 0) normalizedValue = 0;
                    if (normalizedValue > 50) normalizedValue = 50;
                    // Send raw value without scaling
                } else if (parameter === 'droneSpeedRotation') {
                    // Drone rotation: 0-180 range, send raw value
                    if (normalizedValue < 0) normalizedValue = 0;
                    if (normalizedValue > 180) normalizedValue = 180;
                } else if (parameter === 'droneSpeedTravel') {
                    // Drone travel: 0-250 range, send raw value
                    if (normalizedValue < 0) normalizedValue = 0;
                    if (normalizedValue > 250) normalizedValue = 250;
                } else {
                    // Standard percentage controls: 0-100 range
                    if (normalizedValue < 0) normalizedValue = 0;
                    if (normalizedValue > 100) normalizedValue = 100;
                }
                
                fetch('/api/set-camera-parameter', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({ 
                        parameter: parameter, 
                        value: normalizedValue 
                    })
                })
                .then(response => response.json())
                .then(data => {
                    if (data.status === 'success') {
                        console.log('Camera parameter set:', parameter, '=', normalizedValue);
                    } else {
                        console.error('Failed to set camera parameter');
                    }
                })
                .catch(error => {
                    console.error('Error setting camera parameter:', error);
                });
                
                // Clear the timeout reference
                delete parameterUpdateTimeouts[parameter];
            }, 150); // 150ms debounce delay
        }
          function resetCockpitView() {
            fetch('/api/camera-action', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ action: 'resetCockpit' })
            })
            .then(response => response.json())
            .then(data => {                if (data.status === 'success') {
                    console.log('Cockpit view centered');
                } else {
                    console.error('Failed to center cockpit view');
                }
            })
            .catch(error => {
                console.error('Error centering cockpit view:', error);
            });
        }
        
        function toggleSmartCamera() {
            fetch('/api/camera-action', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ action: 'toggleSmartCamera' })
            })
            .then(response => response.json())
            .then(data => {
                if (data.status === 'success') {
                    console.log('Smart camera toggled');
                } else {
                    console.error('Failed to toggle smart camera');
                }
            })
            .catch(error => {
                console.error('Error toggling smart camera:', error);
            });
        }
        
        // Update data every 100ms for smooth real-time updates
        setInterval(updateData, 100);
        
        // Initial data load
        updateData();
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	tmpl := template.Must(template.New("index").Parse(htmlTemplate))
	tmpl.Execute(w, nil)
}
