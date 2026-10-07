//go:build windows
// +build windows

// Command spike-throttle probes whether an AI aircraft's engines follow a
// throttle set from outside: its N1 and throttle before, with the throttle
// at 90 %, and after (our injected take-offs sounded at idle). Usage:
// spike-throttle <objectID>.
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

type eng struct{ N1, Throttle, Comb float64 }

func main() {
	id64, _ := strconv.ParseUint(os.Args[1], 10, 32)
	id := uint32(id64)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	client := simconnect.NewClient("spike-throttle", engine.WithContext(ctx))
	if err := client.Connect(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer client.Disconnect()
	const def, set, req = 6100, 6101, 6102
	client.AddToDataDefinition(def, "TURB ENG N1:1", "percent", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0)
	client.AddToDataDefinition(def, "GENERAL ENG THROTTLE LEVER POSITION:1", "percent", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 1)
	client.AddToDataDefinition(def, "GENERAL ENG COMBUSTION:1", "bool", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 2)
	client.AddToDataDefinition(set, "GENERAL ENG THROTTLE LEVER POSITION:1", "percent", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0)
	client.AddToDataDefinition(set, "GENERAL ENG THROTTLE LEVER POSITION:2", "percent", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 1)
	client.RequestDataOnSimObject(req, def, id, types.SIMCONNECT_PERIOD_SECOND, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)
	start := time.Now()
	phase := 0
	setThrottle := func(pct float64) {
		v := [2]float64{pct, pct}
		if err := client.SetDataOnSimObject(set, id, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, uint32(unsafe.Sizeof(v)), unsafe.Pointer(&v)); err != nil {
			fmt.Println("set:", err)
		}
	}
	for {
		select {
		case <-ctx.Done():
			setThrottle(0)
			return
		case m, ok := <-client.Stream():
			if !ok {
				return
			}
			switch types.SIMCONNECT_RECV_ID(m.SIMCONNECT_RECV.DwID) {
			case types.SIMCONNECT_RECV_ID_EXCEPTION:
				e := m.AsException()
				fmt.Println("exception", e.DwException, "send", e.DwSendID, "index", e.DwIndex)
			case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
				d := m.AsSimObjectData()
				e := engine.CastDataAs[eng](&d.DwData)
				t := time.Since(start).Seconds()
				fmt.Printf("%5.1fs N1 %5.1f throttle %5.1f combustion %.0f\n", t, e.N1, e.Throttle, e.Comb)
				switch {
				case phase == 0 && t > 4:
					phase = 1
					fmt.Println("-> throttle 90 %")
					setThrottle(90)
				case phase == 1 && t > 7:
					setThrottle(90) // again: AI may reset it
					phase = 2
				case phase == 2 && t > 16:
					phase = 3
					fmt.Println("-> throttle 0 %")
					setThrottle(0)
				case phase == 3 && t > 22:
					return
				}
			}
		}
	}
}
