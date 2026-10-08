//go:build windows
// +build windows

// Command spike-lvars watches variables of the user aircraft (L:vars or
// SimVars, one per argument; "NAME|unit" for a unit) every 200 ms and
// prints each change: which switch moves which variable, measured live.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/systems"
	"github.com/mrlm-net/simconnect/pkg/types"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	client := simconnect.NewClient("spike-lvars", engine.WithContext(ctx))
	if err := client.Connect(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer client.Disconnect()
	p := systems.Profile{Name: "watch", Values: map[string]systems.Value{}}
	for _, a := range os.Args[1:] {
		name, unit, _ := strings.Cut(a, "|")
		p.Values[a] = systems.Value{Vars: []string{name}, Unit: unit}
	}
	r := systems.NewReader(client, 6500, 6500)
	r.Use(p)
	if err := r.Request(types.SIMCONNECT_PERIOD_VISUAL_FRAME); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	last := map[string]float64{}
	first := true
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case m := <-client.Stream():
			s, ok := r.Handle(m)
			if !ok {
				continue
			}
			for _, a := range os.Args[1:] {
				v := s.Values[a]
				if old, seen := last[a]; first || !seen || old != v {
					fmt.Printf("%s %-36s %g\n", time.Now().Format("15:04:05.0"), a, v)
					last[a] = v
				}
			}
			first = false
		case <-tick.C:
		}
	}
}
