//go:build windows
// +build windows

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// probeNames tries each simvar on its own definition against the user
// aircraft and reports which names the simulator knows and their value.
func probeNames(ctx context.Context, client engine.Client, names []string) {
	sent := map[uint32]string{}
	for i, n := range names {
		def := uint32(100 + i)
		unit, dt := "feet", types.SIMCONNECT_DATATYPE_FLOAT64
		if len(n) > 4 && n[:4] == "XYZ " {
			n, dt = n[4:], types.SIMCONNECT_DATATYPE_XYZ
		}
		if n == "NUMBER OF CONTACT POINTS" {
			unit = "number"
		}
		client.AddToDataDefinition(def, n, unit, dt, 0, 0)
		if id, err := client.GetLastSentPacketID(); err == nil {
			sent[id] = n
		}
		client.RequestDataOnSimObjectType(uint32(100+i), def, 0, types.SIMCONNECT_SIMOBJECT_TYPE_USER)
		names[i] = n
	}
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-deadline:
			return
		case msg := <-client.Stream():
			if msg.SIMCONNECT_RECV == nil {
				continue
			}
			switch types.SIMCONNECT_RECV_ID(msg.DwID) {
			case types.SIMCONNECT_RECV_ID_EXCEPTION:
				e := msg.AsException()
				fmt.Printf("  unknown: %s (exception %d)\n", sent[uint32(e.DwSendID)], e.DwException)
			case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE:
				d := msg.AsSimObjectDataBType()
				i := int(d.DwRequestID) - 100
				if i >= 0 && i < len(names) {
					v := engine.CastDataAs[types.SIMCONNECT_DATA_XYZ](&d.DwData)
					fmt.Printf("  %-34s %8.2f %8.2f %8.2f\n", names[i], v.X, v.Y, v.Z)
				}
			}
		}
	}
}
