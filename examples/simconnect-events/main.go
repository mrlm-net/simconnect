// Demonstrates all manager event handlers and channel subscriptions.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mrlm-net/simconnect/pkg/manager"
	"github.com/mrlm-net/simconnect/pkg/types"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mgr := manager.New("simconnect-events",
		manager.WithContext(ctx),
		manager.WithAutoReconnect(true),
	)

	// ──────────────────────────────────────────────────────────────────────
	// Connection state — log all transitions so progress is visible
	// ──────────────────────────────────────────────────────────────────────

	mgr.OnConnectionStateChange(func(oldState, newState manager.ConnectionState) {
		fmt.Printf("[state] %s -> %s\n", oldState, newState)
	})

	// ──────────────────────────────────────────────────────────────────────
	// Callback handlers — invoked on the dispatch goroutine
	// ──────────────────────────────────────────────────────────────────────

	// Pause / Sim running
	mgr.OnPause(func(paused bool) {
		if paused {
			fmt.Println("[handler] Simulator paused")
		} else {
			fmt.Println("[handler] Simulator resumed")
		}
	})

	mgr.OnSimRunning(func(running bool) {
		if running {
			fmt.Println("[handler] Simulator started")
		} else {
			fmt.Println("[handler] Simulator stopped")
		}
	})

	// Crash / Sound
	mgr.OnCrashed(func() {
		fmt.Println("[handler] Crashed event received")
	})

	mgr.OnCrashReset(func() {
		fmt.Println("[handler] CrashReset event received")
	})

	mgr.OnSoundEvent(func(soundID uint32) {
		fmt.Printf("[handler] Sound event: id=%d\n", soundID)
	})

	// View / Flight plan deactivated
	mgr.OnView(func(viewID uint32) {
		fmt.Printf("[handler] View changed: id=%d\n", viewID)
	})

	mgr.OnFlightPlanDeactivated(func() {
		fmt.Println("[handler] Flight plan deactivated")
	})

	// Filename events
	mgr.OnFlightLoaded(func(filename string) {
		fmt.Printf("[handler] FlightLoaded: %s\n", filename)
	})

	mgr.OnAircraftLoaded(func(filename string) {
		fmt.Printf("[handler] AircraftLoaded: %s\n", filename)
	})

	mgr.OnFlightPlanActivated(func(filename string) {
		fmt.Printf("[handler] FlightPlanActivated: %s\n", filename)
	})

	// Object events
	mgr.OnObjectAdded(func(objectID uint32, objType types.SIMCONNECT_SIMOBJECT_TYPE) {
		fmt.Printf("[handler] ObjectAdded: id=%d type=%d\n", objectID, objType)
	})

	mgr.OnObjectRemoved(func(objectID uint32, objType types.SIMCONNECT_SIMOBJECT_TYPE) {
		fmt.Printf("[handler] ObjectRemoved: id=%d type=%d\n", objectID, objType)
	})

	// ──────────────────────────────────────────────────────────────────────
	// Channel subscriptions — consumed in the select loop below
	// ──────────────────────────────────────────────────────────────────────

	// Filename subscriptions (typed events with .Events() channel)
	subFL := mgr.SubscribeOnFlightLoaded("sub-flight", 8)
	defer subFL.Unsubscribe()

	subAC := mgr.SubscribeOnAircraftLoaded("sub-aircraft", 8)
	defer subAC.Unsubscribe()

	subFP := mgr.SubscribeOnFlightPlanActivated("sub-fplan", 8)
	defer subFP.Unsubscribe()

	// Object subscriptions (typed events with .Events() channel)
	subAdd := mgr.SubscribeOnObjectAdded("sub-obj-add", 32)
	defer subAdd.Unsubscribe()

	subRem := mgr.SubscribeOnObjectRemoved("sub-obj-rem", 32)
	defer subRem.Unsubscribe()

	// Raw message subscriptions (use .Messages() channel)
	subPause := mgr.SubscribeOnPause("sub-pause", 8)
	defer subPause.Unsubscribe()

	subSim := mgr.SubscribeOnSimRunning("sub-sim", 8)
	defer subSim.Unsubscribe()

	subCrash := mgr.SubscribeOnCrashed("sub-crash", 8)
	defer subCrash.Unsubscribe()

	subReset := mgr.SubscribeOnCrashReset("sub-crashreset", 8)
	defer subReset.Unsubscribe()

	subSound := mgr.SubscribeOnSoundEvent("sub-sound", 8)
	defer subSound.Unsubscribe()

	subView := mgr.SubscribeOnView("sub-view", 8)
	defer subView.Unsubscribe()

	subDeactivated := mgr.SubscribeOnFlightPlanDeactivated("sub-deactivated", 8)
	defer subDeactivated.Unsubscribe()

	// ──────────────────────────────────────────────────────────────────────
	// Custom system event — registered after connection is available
	// ──────────────────────────────────────────────────────────────────────

	mgr.OnConnectionStateChange(func(_, newState manager.ConnectionState) {
		if newState == manager.StateAvailable {
			// Subscribe to the "1sec" custom timer event
			sub, err := mgr.SubscribeToCustomSystemEvent("1sec", 4)
			if err != nil {
				log.Printf("Failed to subscribe to custom event '1sec': %v", err)
				return
			}
			fmt.Println("[custom] Subscribed to '1sec' event")

			// Consume custom event in a dedicated goroutine
			go func() {
				for {
					select {
					case msg := <-sub.Messages():
						if ev := msg.AsEvent(); ev != nil {
							fmt.Printf("[custom] 1sec tick: data=%d\n", ev.DwData)
						}
					case <-sub.Done():
						return
					}
				}
			}()
		}
	})

	// ──────────────────────────────────────────────────────────────────────
	// Start manager in background
	// ──────────────────────────────────────────────────────────────────────

	go func() {
		if err := mgr.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "manager stopped: %v\n", err)
			cancel()
		}
	}()

	fmt.Println("Listening for SimConnect events... (Ctrl+C to quit)")

	// ──────────────────────────────────────────────────────────────────────
	// Main select loop — processes channel subscriptions and signals
	// ──────────────────────────────────────────────────────────────────────

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

loop:
	for {
		select {
		// Filename events
		case ev := <-subFL.Events():
			fmt.Printf("[sub] FlightLoaded: %s\n", ev.Filename)

		case ev := <-subAC.Events():
			fmt.Printf("[sub] AircraftLoaded: %s\n", ev.Filename)

		case ev := <-subFP.Events():
			fmt.Printf("[sub] FlightPlanActivated: %s\n", ev.Filename)

		// Object events
		case ev := <-subAdd.Events():
			fmt.Printf("[sub] ObjectAdded: id=%d type=%d\n", ev.ObjectID, ev.ObjType)

		case ev := <-subRem.Events():
			fmt.Printf("[sub] ObjectRemoved: id=%d type=%d\n", ev.ObjectID, ev.ObjType)

		// Pause / Sim running
		case msg := <-subPause.Messages():
			if ev := msg.AsEvent(); ev != nil {
				fmt.Printf("[sub] Pause event: data=%d (1=paused, 0=unpaused)\n", ev.DwData)
			}

		case msg := <-subSim.Messages():
			if ev := msg.AsEvent(); ev != nil {
				fmt.Printf("[sub] Sim event: data=%d (1=running, 0=stopped)\n", ev.DwData)
			}

		// Crash / Sound
		case msg := <-subCrash.Messages():
			if ev := msg.AsEvent(); ev != nil {
				fmt.Printf("[sub] Crashed event data=%d\n", ev.DwData)
			}

		case msg := <-subReset.Messages():
			if ev := msg.AsEvent(); ev != nil {
				fmt.Printf("[sub] CrashReset event data=%d\n", ev.DwData)
			}

		case msg := <-subSound.Messages():
			if ev := msg.AsEvent(); ev != nil {
				fmt.Printf("[sub] Sound event id=%d data=%d\n", ev.UEventID, ev.DwData)
			}

		// View / FlightPlanDeactivated
		case msg := <-subView.Messages():
			if ev := msg.AsEvent(); ev != nil {
				fmt.Printf("[sub] View changed: viewID=%d\n", ev.DwData)
			}

		case <-subDeactivated.Messages():
			fmt.Println("[sub] Flight plan deactivated")

		// Signals
		case s := <-sig:
			fmt.Printf("\nSignal: %v, shutting down...\n", s)
			break loop

		case <-ctx.Done():
			break loop

		case <-time.After(30 * time.Second):
			fmt.Println("[heartbeat] Waiting for events...")
		}
	}

	// Shutdown
	if err := mgr.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to stop manager: %v\n", err)
	}
	fmt.Println("Shutdown complete.")
}
