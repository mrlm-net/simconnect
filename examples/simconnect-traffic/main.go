//go:build windows
// +build windows

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/manager"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// defWaypoints is a SimConnect data definition ID registered for "AI Waypoint List".
// Must be registered once when the connection is established.
const defWaypoints uint32 = 2000

// Request IDs used for spawning and waypoint operations.
// Pick values outside the manager's reserved range (< 999_999_900).
const (
	reqParked  uint32 = 5001
	reqEnroute uint32 = 5002
	reqNonATC  uint32 = 5003
	reqRelease uint32 = 5004
	reqRemove  uint32 = 5005
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	fmt.Println("SimConnect Traffic — pkg/traffic demo")
	fmt.Println("Press Ctrl+C to remove all aircraft and exit")

	mgr := manager.New("GO Example - SimConnect Traffic",
		manager.WithContext(ctx),
		manager.WithAutoReconnect(true),
		manager.WithBufferSize(512),
		manager.WithHeartbeat("6Hz"),
	)

	// Register message handler — watch for ASSIGNED_OBJECT_ID to acknowledge spawns.
	_ = mgr.OnMessage(func(msg engine.Message) {
		if msg.Err != nil {
			fmt.Fprintln(os.Stderr, "message error:", msg.Err)
			return
		}
		switch types.SIMCONNECT_RECV_ID(msg.DwID) {
		case types.SIMCONNECT_RECV_ID_ASSIGNED_OBJECT_ID:
			assigned := msg.AsAssignedObjectID()
			a, ok := mgr.Fleet().Acknowledge(uint32(assigned.DwRequestID), uint32(assigned.DwObjectID))
			if !ok {
				return // not our request
			}
			fmt.Printf("✅ spawned  tail=%-10s objectID=%d\n", a.Tail, a.ObjectID)

			// NonATC aircraft need control released before waypoints are accepted.
			if a.Kind == traffic.KindNonATC {
				if err := mgr.TrafficReleaseControl(a.ObjectID, reqRelease); err != nil {
					fmt.Fprintln(os.Stderr, "release control failed:", err)
					return
				}
				sendWaypoints(mgr, a.ObjectID)
			}
		}
	})

	// On connection, register the waypoint data definition and spawn aircraft.
	_ = mgr.OnConnectionStateChange(func(old, new manager.ConnectionState) {
		fmt.Printf("connection: %s → %s\n", old, new)
		if new != manager.StateConnected {
			return
		}

		// Register waypoint data definition (required for SetWaypoints).
		if err := mgr.AddToDataDefinition(
			defWaypoints,
			"AI Waypoint List", "",
			types.SIMCONNECT_DATATYPE_WAYPOINT,
			0, defWaypoints,
		); err != nil {
			fmt.Fprintln(os.Stderr, "register waypoints def failed:", err)
		}

		spawnAircraft(mgr)
	})

	// Signal handler: clean up fleet then shut down.
	go func() {
		<-sigChan
		fmt.Println("\nshutting down — removing all AI aircraft...")
		if err := mgr.Fleet().RemoveAll(reqRemove); err != nil {
			fmt.Fprintln(os.Stderr, "RemoveAll error:", err)
		}
		// Brief pause so removal requests reach the sim before disconnect.
		time.Sleep(500 * time.Millisecond)
		mgr.Stop()
		cancel()
	}()

	// Periodic fleet status log.
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				aircraft := mgr.Fleet().List()
				if len(aircraft) == 0 {
					fmt.Println("fleet: empty")
					continue
				}
				fmt.Printf("fleet: %d aircraft\n", len(aircraft))
				for _, a := range aircraft {
					fmt.Printf("  tail=%-10s objectID=%d kind=%d\n", a.Tail, a.ObjectID, a.Kind)
				}
			}
		}
	}()

	if err := mgr.Start(); err != nil {
		fmt.Println("manager stopped:", err)
	}
	fmt.Println("goodbye")
}

// spawnAircraft issues creation requests for all three aircraft kinds.
// Swap model titles and tail numbers to match what you have installed.
func spawnAircraft(mgr manager.Manager) {
	// Parked — placed at a gate, managed by ATC.
	if err := mgr.TrafficParked(traffic.ParkedOpts{
		Model:   "FSLTL A320 Air France SL",
		Livery:  "",
		Tail:    "AFR001",
		Airport: "LFPG",
	}, reqParked); err != nil {
		fmt.Fprintln(os.Stderr, "TrafficParked failed:", err)
	}

	// Enroute — follows a .PLN flight plan from the start.
	// Uncomment and provide a valid plan path to test this kind.
	// if err := mgr.TrafficEnroute(traffic.EnrouteOpts{
	// 	Model:        "FSLTL A321 Iberia SL",
	// 	Tail:         "IBE001",
	// 	FlightNumber: 1,
	// 	FlightPlan:   `C:\Plans\LEMD-LEBL.pln`,
	// 	Phase:        0.0,
	// }, reqEnroute); err != nil {
	// 	fmt.Fprintln(os.Stderr, "TrafficEnroute failed:", err)
	// }

	// NonATC — placed at an explicit position, then given a waypoint chain.
	// Coordinates below are approximate gate area at LKPR (Prague).
	if err := mgr.TrafficNonATC(traffic.NonATCOpts{
		Model:  "FSLTL A320 CSA SL",
		Livery: "",
		Tail:   "CSA100",
		Position: types.SIMCONNECT_DATA_INITPOSITION{
			Latitude:  50.1008,
			Longitude: 14.2600,
			Altitude:  1247, // ft MSL — LKPR field elevation
			Heading:   258,
			OnGround:  1,
			Airspeed:  0,
		},
	}, reqNonATC); err != nil {
		fmt.Fprintln(os.Stderr, "TrafficNonATC failed:", err)
	}
}

// sendWaypoints assigns a pushback→taxi→takeoff chain to a NonATC aircraft.
// Coordinates are approximate for LKPR runway 24.
func sendWaypoints(mgr manager.Manager, objectID uint32) {
	const (
		rwyLat  = 50.0982
		rwyLon  = 14.2560
		rwyHdg  = 258.0
		fieldFt = 1247.0
	)

	wps := []types.SIMCONNECT_DATA_WAYPOINT{
		traffic.PushbackWaypoint(50.1008, 14.2595, fieldFt, 3),
		traffic.TaxiWaypoint(50.1000, 14.2580, fieldFt, 15),
		traffic.LineupWaypoint(rwyLat, rwyLon, fieldFt),
	}
	wps = append(wps, traffic.TakeoffClimb(rwyLat, rwyLon, rwyHdg)...)

	if err := mgr.TrafficSetWaypoints(objectID, defWaypoints, wps); err != nil {
		fmt.Fprintln(os.Stderr, "SetWaypoints failed:", err)
		return
	}
	fmt.Printf("waypoints set  objectID=%d  count=%d\n", objectID, len(wps))
}
