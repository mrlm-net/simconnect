package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mrlm-net/simconnect/pkg/manager"
	"github.com/mrlm-net/simconnect/pkg/types"
)

func main() {
	mgr := manager.New("simconnect-events")

	// Register callback handlers
	mgr.OnFlightLoaded(func(filename string) {
		fmt.Printf("[handler] FlightLoaded: %s\n", filename)
	})

	mgr.OnAircraftLoaded(func(filename string) {
		fmt.Printf("[handler] AircraftLoaded: %s\n", filename)
	})

	mgr.OnFlightPlanActivated(func(filename string) {
		fmt.Printf("[handler] FlightPlanActivated: %s\n", filename)
	})

	mgr.OnObjectAdded(func(objectID uint32, objType types.SIMCONNECT_SIMOBJECT_TYPE) {
		fmt.Printf("[handler] ObjectAdded: id=%d type=%d\n", objectID, objType)
	})

	mgr.OnObjectRemoved(func(objectID uint32, objType types.SIMCONNECT_SIMOBJECT_TYPE) {
		fmt.Printf("[handler] ObjectRemoved: id=%d type=%d\n", objectID, objType)
	})

	// New event handlers
	mgr.OnCrashed(func() {
		fmt.Println("[handler] Crashed event received")
	})

	mgr.OnCrashReset(func() {
		fmt.Println("[handler] CrashReset event received")
	})

	mgr.OnSoundEvent(func(soundID uint32) {
		fmt.Printf("[handler] Sound event: id=%d\n", soundID)
	})

	// Create channel subscriptions
	subFL := mgr.SubscribeOnFlightLoaded("sub-flight", 8)
	defer subFL.Unsubscribe()

	subAC := mgr.SubscribeOnAircraftLoaded("sub-aircraft", 8)
	defer subAC.Unsubscribe()

	subFP := mgr.SubscribeOnFlightPlanActivated("sub-fplan", 8)
	defer subFP.Unsubscribe()

	subAdd := mgr.SubscribeOnObjectAdded("sub-obj-add", 32)
	defer subAdd.Unsubscribe()

	subRem := mgr.SubscribeOnObjectRemoved("sub-obj-rem", 32)
	defer subRem.Unsubscribe()

	// Subscribe to crash/sound events (raw message subscriptions)
	subCrash := mgr.SubscribeOnCrashed("sub-crash", 8)
	defer subCrash.Unsubscribe()

	subReset := mgr.SubscribeOnCrashReset("sub-crashreset", 8)
	defer subReset.Unsubscribe()

	subSound := mgr.SubscribeOnSoundEvent("sub-sound", 8)
	defer subSound.Unsubscribe()

	// Start manager
	_, stop := context.WithCancel(context.Background())
	defer stop()
	go func() {
		if err := mgr.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "manager stopped: %v\n", err)
			stop()
		}
	}()

	// Watch for events and OS signals
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

loop:
	for {
		select {
		case ev := <-subFL.Events():
			fmt.Printf("[sub] FlightLoaded: %s\n", ev.Filename)

		case ev := <-subAC.Events():
			fmt.Printf("[sub] AircraftLoaded: %s\n", ev.Filename)

		case ev := <-subFP.Events():
			fmt.Printf("[sub] FlightPlanActivated: %s\n", ev.Filename)

		case ev := <-subAdd.Events():
			fmt.Printf("[sub] ObjectAdded: id=%d type=%d\n", ev.ObjectID, ev.ObjType)

		case ev := <-subRem.Events():
			fmt.Printf("[sub] ObjectRemoved: id=%d type=%d\n", ev.ObjectID, ev.ObjType)

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

		case s := <-sig:
			fmt.Printf("signal: %v, shutting down...\n", s)
			break loop

		case <-time.After(1 * time.Second):
			// heartbeat
		}
	}

	// Shutdown
	if err := mgr.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to stop manager: %v\n", err)
	}
}
