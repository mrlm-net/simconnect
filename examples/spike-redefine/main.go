//go:build windows
// +build windows

// Command spike-redefine checks what a second airport loader on the same
// connection gets (#841: the World restarted on an open connection lost
// LKPR's runways): two loaders, the second loading LKPR, with or without
// clearing the definitions first ("clear" argument).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	client := simconnect.NewClient("spike-redefine", engine.WithContext(ctx))
	if err := client.Connect(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer client.Disconnect()
	load := func(name string) {
		l := airport.NewLoader(client, airport.LoaderWithIDs(0xA000, 0xA000+100))
		if err := l.Request("LKPR"); err != nil {
			fmt.Println(name, "request:", err)
			return
		}
		for {
			select {
			case <-ctx.Done():
				fmt.Println(name, "timeout")
				return
			case m, ok := <-client.Stream():
				if !ok {
					return
				}
				if r, done := l.Handle(m); done {
					if r.Err != nil {
						fmt.Println(name, "error:", r.Err)
					} else {
						_, jerr := json.Marshal(r.Layout)
						fmt.Println(name, "runways", len(r.Layout.Runways), "parking", len(r.Layout.Parking), "json:", jerr)
					}
					return
				}
			}
		}
	}
	load("first")
	if len(os.Args) > 1 && os.Args[1] == "clear" {
		for i := range uint32(16) {
			client.ClearDataDefinition(airport.DefaultLoaderDefinitionBase + i)
		}
	}
	load("second")
}
