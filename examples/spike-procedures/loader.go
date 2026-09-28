//go:build windows
// +build windows

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
)

// viaLoader loads the procedures with airport.ProcedureLoader, prints a
// summary and writes them as JSON to dump (if set).
func viaLoader(ctx context.Context, client engine.Client, icao, dump string) {
	l := airport.NewProcedureLoader(client)
	if err := l.Request(icao); err != nil {
		fmt.Println(err)
		return
	}
	for {
		select {
		case <-ctx.Done():
			fmt.Println("timed out")
			return
		case msg := <-client.Stream():
			p, ok := l.Handle(msg)
			if !ok {
				continue
			}
			fmt.Printf("%s: %d SIDs, %d STARs, %d approaches\n", p.ICAO, len(p.Departures), len(p.Arrivals), len(p.Approaches))
			for _, d := range p.Departures[:min(3, len(p.Departures))] {
				fmt.Printf("  SID %s runways %v common legs %d enroute %d\n", d.Name, d.Runways(), len(d.Legs), len(d.EnrouteTransitions))
			}
			for _, a := range p.Approaches {
				fmt.Printf("  %s: %d transitions, final %d legs, missed %d\n", a.Name, len(a.Transitions), len(a.Final), len(a.Missed))
			}
			if dump != "" {
				b, _ := json.MarshalIndent(p, "", " ")
				os.WriteFile(dump, b, 0o644)
			}
			return
		}
	}
}
