//go:build windows
// +build windows

// Command spike-speeds reads the user aircraft's systems state once with
// its profile (systems.For by title) and prints the take-off speeds.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/systems"
	"github.com/mrlm-net/simconnect/pkg/types"
)

func main() {
	title := "FenixA319 CFM WF HD"
	if len(os.Args) > 1 {
		title = os.Args[1]
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := simconnect.NewClient("spike-speeds", engine.WithContext(ctx))
	if err := client.Connect(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer client.Disconnect()
	r := systems.NewReader(client, 6400, 6400)
	r.Use(systems.For(systems.Aircraft{Title: title}))
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
				fmt.Printf("%s: weight %.0f kg, flaps %.0f; V1 %.0f VR %.0f V2 %.0f from %q, speed check %d; design VR %.0f V2 %.0f, max %.0f kg\n",
					title, s.Values[systems.Weight], s.Values[systems.FlapsIndex], s.V1Kt, s.VRKt, s.V2Kt, s.SpeedsFrom, s.SpeedCheckKt,
					s.Values[systems.DesignVR], s.Values[systems.DesignV2], s.Values[systems.MaxWeight])
				return
			}
		}
	}
}
