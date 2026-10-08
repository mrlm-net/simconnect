//go:build windows
// +build windows

// Command spike-gsx prints GSX's state on the user aircraft once (pkg/gsx).
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/gsx"
	"github.com/mrlm-net/simconnect/pkg/types"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	client := simconnect.NewClient("spike-gsx", engine.WithContext(ctx))
	if err := client.Connect(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer client.Disconnect()
	r := gsx.NewReader(client, 6700, 6700)
	if err := r.Request(types.SIMCONNECT_PERIOD_ONCE); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for {
		select {
		case <-ctx.Done():
			fmt.Println("timeout")
			return
		case m := <-client.Stream():
			if s, ok := r.Handle(m); ok {
				fmt.Printf("running %v; boarding %v, deboarding %v, catering %v, refueling %v, departure %v, deice %v\n",
					s.Running, s.Boarding, s.Deboarding, s.Catering, s.Refueling, s.Departure, s.Deice)
				fmt.Printf("pax %d (max %d), boarding %d/%d; cargo %.0f%%; fuel hose %v; frozen %v; gate %q; pilots %v crew %v; waiting for %v\n",
					s.Passengers, s.MaxPassengers, s.PassengersBoarding, s.PassengersBoardingTotal, s.CargoLoadedPct, s.FuelHose, s.Frozen, s.Gate, s.PilotsOnBoard, s.CrewOnBoard, s.WaitingFor)
				return
			}
		}
	}
}
