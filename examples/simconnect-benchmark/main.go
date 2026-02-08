//go:build windows
// +build windows

package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/manager"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// CameraData represents the data structure for CAMERA STATE and CAMERA SUBSTATE
type CameraData struct {
	CameraState    int32
	CameraSubstate int32
	Category       [260]byte // String260
}

// AircraftData represents the data structure for aircraft information
type AircraftData struct {
	Title             [128]byte
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

// CLI flags
var (
	duration = flag.Duration("duration", 60*time.Second, "Benchmark duration")
	pprof    = flag.Bool("pprof", false, "Enable net/http/pprof on :6060")
	interval = flag.Duration("interval", 5*time.Second, "Stats reporting interval")
	buffer   = flag.Int("buffer", 512, "Engine buffer size")
)

// Atomic counters for tracking
var (
	messagesReceived  atomic.Uint64
	stateChanges      atomic.Uint64
	subDeliveries     atomic.Uint64
	facilityResponses atomic.Uint64
)

// Peak tracking (accessed from single goroutine, no atomics needed)
var (
	peakHeapMB     float64
	peakGoroutines int
)

// setupDataDefinitions registers high-frequency data definitions to generate load
func setupDataDefinitions(client engine.Client) {
	slog.Info("Setting up data definitions for stress testing")

	// Camera data - request at SIM_FRAME rate (highest frequency)
	client.AddToDataDefinition(2000, "CAMERA STATE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 0)
	client.AddToDataDefinition(2000, "CAMERA SUBSTATE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 1)
	client.AddToDataDefinition(2000, "CATEGORY", "", types.SIMCONNECT_DATATYPE_STRING260, 0, 2)
	client.RequestDataOnSimObject(2001, 2000, types.SIMCONNECT_OBJECT_ID_USER, types.SIMCONNECT_PERIOD_SIM_FRAME, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)

	// Aircraft data - full telemetry at SIM_FRAME rate
	client.AddToDataDefinition(3000, "TITLE", "", types.SIMCONNECT_DATATYPE_STRING128, 0, 0)
	client.AddToDataDefinition(3000, "LIVERY NAME", "", types.SIMCONNECT_DATATYPE_STRING128, 0, 1)
	client.AddToDataDefinition(3000, "LIVERY FOLDER", "", types.SIMCONNECT_DATATYPE_STRING128, 0, 2)
	client.AddToDataDefinition(3000, "PLANE LATITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 3)
	client.AddToDataDefinition(3000, "PLANE LONGITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 4)
	client.AddToDataDefinition(3000, "PLANE ALTITUDE", "feet", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 5)
	client.AddToDataDefinition(3000, "PLANE HEADING DEGREES TRUE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 6)
	client.AddToDataDefinition(3000, "PLANE HEADING DEGREES MAGNETIC", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 7)
	client.AddToDataDefinition(3000, "VERTICAL SPEED", "feet per second", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 8)
	client.AddToDataDefinition(3000, "PLANE PITCH DEGREES", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 9)
	client.AddToDataDefinition(3000, "PLANE BANK DEGREES", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 10)
	client.AddToDataDefinition(3000, "GROUND VELOCITY", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 11)
	client.AddToDataDefinition(3000, "AIRSPEED INDICATED", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 12)
	client.AddToDataDefinition(3000, "AIRSPEED TRUE", "knots", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 13)
	client.AddToDataDefinition(3000, "ON ANY RUNWAY", "bool", types.SIMCONNECT_DATATYPE_INT32, 0, 14)
	client.AddToDataDefinition(3000, "SURFACE TYPE", "", types.SIMCONNECT_DATATYPE_INT32, 0, 15)
	client.AddToDataDefinition(3000, "SIM ON GROUND", "bool", types.SIMCONNECT_DATATYPE_INT32, 0, 16)
	client.AddToDataDefinition(3000, "ATC ID", "", types.SIMCONNECT_DATATYPE_STRING32, 0, 17)
	client.AddToDataDefinition(3000, "ATC AIRLINE", "", types.SIMCONNECT_DATATYPE_STRING32, 0, 18)
	client.RequestDataOnSimObject(3001, 3000, types.SIMCONNECT_OBJECT_ID_USER, types.SIMCONNECT_PERIOD_SIM_FRAME, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)

	// Request traffic data within 50km radius
	client.RequestDataOnSimObjectType(4001, 3000, 50000, types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT)

	// Request facility list for airports (generates periodic responses)
	client.RequestFacilitiesList(5000, types.SIMCONNECT_FACILITY_LIST_AIRPORT)

	slog.Info("Data definitions registered", "camera", 2000, "aircraft", 3000, "traffic_radius_m", 50000)
}

// statsReporter periodically prints benchmark statistics
func statsReporter(ctx context.Context, startTime time.Time) {
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			collectAndPrintStats(startTime)
		}
	}
}

// collectAndPrintStats reads runtime stats and prints a formatted line
func collectAndPrintStats(startTime time.Time) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	elapsed := time.Since(startTime)
	heapMB := float64(m.HeapAlloc) / 1024 / 1024
	sysMB := float64(m.HeapSys) / 1024 / 1024
	pauseMS := float64(m.PauseTotalNs) / 1e6
	goroutines := runtime.NumGoroutine()

	// Track peaks
	if heapMB > peakHeapMB {
		peakHeapMB = heapMB
	}
	if goroutines > peakGoroutines {
		peakGoroutines = goroutines
	}

	// Machine-parseable output format
	fmt.Fprintf(os.Stderr, "[BENCH] t=%ds msgs=%d state=%d subs=%d fac=%d heap=%.1fMB sys=%.1fMB objs=%d gc=%d pause=%.1fms goroutines=%d\n",
		int(elapsed.Seconds()),
		messagesReceived.Load(),
		stateChanges.Load(),
		subDeliveries.Load(),
		facilityResponses.Load(),
		heapMB,
		sysMB,
		m.HeapObjects,
		m.NumGC,
		pauseMS,
		goroutines,
	)
}

// printFinalSummary prints aggregate metrics at the end of the benchmark
func printFinalSummary(startTime time.Time, totalAlloc uint64, finalGC uint32, finalPause uint64) {
	elapsed := time.Since(startTime).Seconds()
	msgs := messagesReceived.Load()
	state := stateChanges.Load()
	subs := subDeliveries.Load()
	fac := facilityResponses.Load()

	fmt.Fprintln(os.Stderr, "\n=== Benchmark Summary ===")
	fmt.Fprintf(os.Stderr, "Duration:        %.1fs\n", elapsed)
	fmt.Fprintf(os.Stderr, "Messages:        %d (%.1f/s)\n", msgs, float64(msgs)/elapsed)
	fmt.Fprintf(os.Stderr, "State Changes:   %d (%.1f/s)\n", state, float64(state)/elapsed)
	fmt.Fprintf(os.Stderr, "Sub Deliveries:  %d (%.1f/s)\n", subs, float64(subs)/elapsed)
	fmt.Fprintf(os.Stderr, "Facility Resp:   %d (%.1f/s)\n", fac, float64(fac)/elapsed)
	fmt.Fprintf(os.Stderr, "Peak Heap:       %.1f MB\n", peakHeapMB)
	fmt.Fprintf(os.Stderr, "Total Alloc:     %.1f MB\n", float64(totalAlloc)/1024/1024)
	fmt.Fprintf(os.Stderr, "GC Cycles:       %d\n", finalGC)
	fmt.Fprintf(os.Stderr, "Total GC Pause:  %.1f ms\n", float64(finalPause)/1e6)
	fmt.Fprintf(os.Stderr, "Peak Goroutines: %d\n", peakGoroutines)
}

func main() {
	flag.Parse()

	// Configure slog for structured logging
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	// Start pprof server if enabled
	if *pprof {
		go func() {
			slog.Info("Starting pprof server", "address", ":6060")
			if err := http.ListenAndServe(":6060", nil); err != nil {
				slog.Error("pprof server failed", "error", err)
			}
		}()
	}

	// Create cancellable context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handler for Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	// Setup duration timer
	durationTimer := time.NewTimer(*duration)

	slog.Info("Starting SimConnect benchmark",
		"duration", *duration,
		"buffer", *buffer,
		"interval", *interval,
		"pprof", *pprof,
	)

	// Create manager with auto-detection and high buffer size
	mgr := manager.New("GO Benchmark - SimConnect",
		manager.WithContext(ctx),
		manager.WithAutoDetect(),
		manager.WithAutoReconnect(true),
		manager.WithBufferSize(*buffer),
		manager.WithHeartbeat("6Hz"),
	)

	// Track start time for elapsed calculations
	startTime := time.Now()

	// Setup signal handler goroutine
	go func() {
		select {
		case <-sigChan:
			slog.Info("Received interrupt signal, shutting down")
			durationTimer.Stop()
		case <-durationTimer.C:
			slog.Info("Duration elapsed, shutting down")
		}
		mgr.Stop()
		cancel()
	}()

	// Register connection state change handler
	mgr.OnConnectionStateChange(func(oldState, newState manager.ConnectionState) {
		stateChanges.Add(1)
		slog.Info("Connection state changed",
			"from", oldState,
			"to", newState,
		)

		if newState == manager.StateConnected {
			// Setup data definitions when connected
			if client := mgr.Client(); client != nil {
				setupDataDefinitions(client)
			}
		}
	})

	// Register sim state change handler
	mgr.OnSimStateChange(func(oldState, newState manager.SimState) {
		stateChanges.Add(1)
	})

	// Register message handler (just count messages, don't print)
	mgr.OnMessage(func(msg engine.Message) {
		messagesReceived.Add(1)

		// Track facility responses separately
		if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_FACILITY_DATA ||
			types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_AIRPORT_LIST {
			facilityResponses.Add(1)
		}
	})

	// Create channel subscriptions to exercise the full stack
	msgSub := mgr.Subscribe("msg-sub", 100)
	stateSub := mgr.SubscribeConnectionStateChange("state-sub", 10)
	simStateSub := mgr.SubscribeSimStateChange("sim-state-sub", 10)

	// Consume subscription channels in separate goroutines
	go func() {
		for {
			select {
			case <-msgSub.Done():
				return
			case <-msgSub.Messages():
				subDeliveries.Add(1)
			}
		}
	}()

	go func() {
		for {
			select {
			case <-stateSub.Done():
				return
			case <-stateSub.ConnectionStateChanges():
				subDeliveries.Add(1)
			}
		}
	}()

	go func() {
		for {
			select {
			case <-simStateSub.Done():
				return
			case <-simStateSub.SimStateChanges():
				subDeliveries.Add(1)
			}
		}
	}()

	// Start periodic stats reporting
	go statsReporter(ctx, startTime)

	// Start the manager - blocks until context is cancelled
	if err := mgr.Start(); err != nil {
		slog.Warn("Manager stopped", "error", err)
	}

	// Unsubscribe all channels
	msgSub.Unsubscribe()
	stateSub.Unsubscribe()
	simStateSub.Unsubscribe()

	// Collect final stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Print final summary
	printFinalSummary(startTime, m.TotalAlloc, m.NumGC, m.PauseTotalNs)

	// Small delay to allow goroutines to complete cleanup
	time.Sleep(100 * time.Millisecond)
	slog.Info("Benchmark complete")
}
