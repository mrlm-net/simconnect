//go:build windows
// +build windows

// Command spike-designspeeds reads the user aircraft's design and stall
// speed SimVars once each, every one in its own definition (an unknown name
// fails alone): which of them MSFS fills, for computed take-off speeds.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

var vars = []struct{ name, unit string }{
	{"DESIGN SPEED VS0", "knots"},
	{"DESIGN SPEED VS1", "knots"},
	{"DESIGN SPEED VC", "knots"},
	{"DESIGN SPEED MIN ROTATION", "knots"},
	{"DESIGN SPEED CLIMB", "knots"},
	{"DESIGN TAKEOFF SPEED", "knots"},
	{"DESIGN SPEED VFE", "knots"},
	{"STALL ALPHA", "degrees"},
	{"TOTAL WEIGHT", "kilograms"},
	{"MAX GROSS WEIGHT", "kilograms"},
	{"EMPTY WEIGHT", "kilograms"},
	{"FLAPS HANDLE INDEX", "number"},
	{"FLAPS NUM HANDLE POSITIONS", "number"},
	{"FLAPS HANDLE PERCENT", "percent"},
	{"ATC TYPE", ""},
	{"ATC MODEL", ""},
	{"TITLE", ""},
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client := simconnect.NewClient("spike-designspeeds", engine.WithContext(ctx))
	if err := client.Connect(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer client.Disconnect()
	const base = 6300
	for i, v := range vars {
		dt := types.SIMCONNECT_DATATYPE_FLOAT64
		if v.unit == "" {
			dt = types.SIMCONNECT_DATATYPE_STRING256
		}
		client.AddToDataDefinition(uint32(base+i), v.name, v.unit, dt, 0, 0)
		client.RequestDataOnSimObject(uint32(base+i), uint32(base+i), types.SIMCONNECT_OBJECT_ID_USER, types.SIMCONNECT_PERIOD_ONCE, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)
	}
	got := 0
	for got < len(vars) {
		select {
		case <-ctx.Done():
			fmt.Println("timeout,", got, "of", len(vars))
			return
		case m, ok := <-client.Stream():
			if !ok {
				return
			}
			if m.SIMCONNECT_RECV == nil {
				continue
			}
			switch types.SIMCONNECT_RECV_ID(m.DwID) {
			case types.SIMCONNECT_RECV_ID_EXCEPTION:
				e := m.AsException()
				fmt.Println("exception", e.DwException, "send", e.DwSendID, "index", e.DwIndex)
			case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
				d := m.AsSimObjectData()
				i := int(d.DwRequestID) - base
				if i < 0 || i >= len(vars) {
					continue
				}
				got++
				if vars[i].unit == "" {
					b := engine.CastDataAs[[256]byte](&d.DwData)
					n := 0
					for n < len(b) && b[n] != 0 {
						n++
					}
					fmt.Printf("%-28s %q\n", vars[i].name, string(b[:n]))
				} else {
					fmt.Printf("%-28s %.2f %s\n", vars[i].name, *engine.CastDataAs[float64](&d.DwData), vars[i].unit)
				}
			}
		}
	}
}
