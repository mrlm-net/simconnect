//go:build windows
// +build windows

// Command spike-touchdown traces an injected landing frame by frame (#854):
// what the simulator shows of the aircraft — altitude, ground, height of
// the wheels, on-ground flag, pitch — every sim frame, to see whether it
// dips below the runway at touchdown. Usage: spike-touchdown <objectID>
// [seconds]; one CSV line per frame on stdout.
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

type frame struct{ Alt, Ground, AGL, AGLMinusCG, OnGround, Pitch, VS, GS float64 }

func main() {
	id64, _ := strconv.ParseUint(os.Args[1], 10, 32)
	secs := 120.0
	if len(os.Args) > 2 {
		secs, _ = strconv.ParseFloat(os.Args[2], 64)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(secs*float64(time.Second)))
	defer cancel()
	client := simconnect.NewClient("spike-touchdown", engine.WithContext(ctx))
	if err := client.Connect(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer client.Disconnect()
	const def, req = 6200, 6201
	for i, v := range []struct{ n, u string }{
		{"PLANE ALTITUDE", "feet"}, {"GROUND ALTITUDE", "feet"}, {"PLANE ALT ABOVE GROUND", "feet"},
		{"PLANE ALT ABOVE GROUND MINUS CG", "feet"}, {"SIM ON GROUND", "bool"}, {"PLANE PITCH DEGREES", "degrees"},
		{"VERTICAL SPEED", "feet per minute"}, {"GROUND VELOCITY", "knots"},
	} {
		client.AddToDataDefinition(def, v.n, v.u, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i))
	}
	client.RequestDataOnSimObject(req, def, uint32(id64), types.SIMCONNECT_PERIOD_SIM_FRAME, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)
	start := time.Now()
	fmt.Println("t,alt,ground,agl,agl_minus_cg,on_ground,pitch,vs,gs")
	for {
		select {
		case <-ctx.Done():
			return
		case m, ok := <-client.Stream():
			if !ok {
				return
			}
			switch types.SIMCONNECT_RECV_ID(m.SIMCONNECT_RECV.DwID) {
			case types.SIMCONNECT_RECV_ID_EXCEPTION:
				e := m.AsException()
				fmt.Fprintln(os.Stderr, "exception", e.DwException, "send", e.DwSendID, "index", e.DwIndex)
			case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
				f := engine.CastDataAs[frame](&m.AsSimObjectData().DwData)
				fmt.Printf("%.3f,%.3f,%.3f,%.3f,%.3f,%.0f,%.2f,%.0f,%.1f\n", time.Since(start).Seconds(),
					f.Alt, f.Ground, f.AGL, f.AGLMinusCG, f.OnGround, f.Pitch, f.VS, f.GS)
			}
		}
	}
}
