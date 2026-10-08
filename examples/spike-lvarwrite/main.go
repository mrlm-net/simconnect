//go:build windows
// +build windows

// Command spike-lvarwrite writes an L:var of our own on the user aircraft
// (SetDataOnSimObject on a definition of "L:NAME") and reads it back: can
// an app create L:vars for others (gauges, GSX, FSUIPC) to read? Usage:
// spike-lvarwrite NAME VALUE.
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

func main() {
	name := os.Args[1]
	val, _ := strconv.ParseFloat(os.Args[2], 64)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	client := simconnect.NewClient("spike-lvarwrite", engine.WithContext(ctx), engine.WithCallTrace())
	if err := client.Connect(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer client.Disconnect()
	const def, req = 6600, 6600
	if err := client.AddToDataDefinition(def, "L:"+name, "number", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 0); err != nil {
		fmt.Println("define:", err)
		return
	}
	if err := client.SetDataOnSimObject(def, types.SIMCONNECT_OBJECT_ID_USER, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, 8, unsafe.Pointer(&val)); err != nil {
		fmt.Println("set:", err)
		return
	}
	time.Sleep(300 * time.Millisecond)
	client.RequestDataOnSimObject(req, def, types.SIMCONNECT_OBJECT_ID_USER, types.SIMCONNECT_PERIOD_ONCE, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)
	for {
		select {
		case <-ctx.Done():
			fmt.Println("timeout")
			return
		case m := <-client.Stream():
			if m.SIMCONNECT_RECV == nil {
				continue
			}
			switch types.SIMCONNECT_RECV_ID(m.DwID) {
			case types.SIMCONNECT_RECV_ID_EXCEPTION:
				e := m.AsException()
				fmt.Println("exception", e.DwException, "send", e.DwSendID)
			case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
				fmt.Printf("L:%s reads back %g\n", name, *engine.CastDataAs[float64](&m.AsSimObjectData().DwData))
				return
			}
		}
	}
}
