//go:build windows
// +build windows

// Command spike-transition reads the AIRPORT facility's TRANSITION_ALTITUDE
// and TRANSITION_LEVEL for a few airports and prints the raw bytes under
// several readings: a probe of what the sim gives before using it.
package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

type raw struct {
	ICAO [8]byte
	B    [16]byte
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	client := simconnect.NewClient("spike-transition", engine.WithContext(ctx))
	if err := client.Connect(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer client.Disconnect()
	for _, f := range []string{"OPEN AIRPORT", "ICAO", "TRANSITION_ALTITUDE", "TRANSITION_LEVEL", "CLOSE AIRPORT"} {
		client.AddToFacilityDefinition(4000, f)
	}
	airports := []string{"LKPR", "EDDM", "LOWW", "EGLL", "LFPG", "KJFK", "LIRF", "EHAM"}
	for i, a := range airports {
		client.RequestFacilityData(4000, uint32(500+i), a, "")
	}
	got := 0
	for {
		select {
		case <-ctx.Done():
			fmt.Println("timeout,", got, "of", len(airports))
			return
		case m, ok := <-client.Stream():
			if !ok {
				return
			}
			if types.SIMCONNECT_RECV_ID(m.SIMCONNECT_RECV.DwID) != types.SIMCONNECT_RECV_ID_FACILITY_DATA {
				continue
			}
			d := m.AsFacilityData()
			r := engine.CastDataAs[raw](&d.Data)
			b := r.B[:]
			f32 := func(o int) float32 { return math.Float32frombits(binary.LittleEndian.Uint32(b[o:])) }
			f64 := func(o int) float64 { return math.Float64frombits(binary.LittleEndian.Uint64(b[o:])) }
			i32 := func(o int) int32 { return int32(binary.LittleEndian.Uint32(b[o:])) }
			fmt.Printf("%-5s bytes % x\n      f32 %.1f %.1f | f64 %.1f %.1f | i32 %d %d\n",
				engine.BytesToString(r.ICAO[:]), b, f32(0), f32(4), f64(0), f64(8), i32(0), i32(4))
			if got++; got == len(airports) {
				return
			}
		}
	}
}
