//go:build windows
// +build windows

// Command spike-ground-near lists the ground objects within a radius of
// the user aircraft whose title matches a pattern, with their position and
// heading (read only): are the stand services there, and where.
package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

type obj struct {
	Title [256]byte
	Lat   float64
	Lon   float64
	Hdg   float64
}

func main() {
	pat := regexp.MustCompile(`(?i)` + os.Args[1])
	radius, _ := strconv.Atoi(os.Args[2])
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client := simconnect.NewClient("spike-ground-near", engine.WithContext(ctx))
	if err := client.Connect(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer client.Disconnect()
	client.AddToDataDefinition(9950, "TITLE", "", types.SIMCONNECT_DATATYPE_STRING256, 0, 0)
	client.AddToDataDefinition(9950, "PLANE LATITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 1)
	client.AddToDataDefinition(9950, "PLANE LONGITUDE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 2)
	client.AddToDataDefinition(9950, "PLANE HEADING DEGREES TRUE", "degrees", types.SIMCONNECT_DATATYPE_FLOAT64, 0, 3)
	client.RequestDataOnSimObjectType(9951, 9950, uint32(radius), types.SIMCONNECT_SIMOBJECT_TYPE_GROUND)
	n := 0
	for {
		select {
		case <-ctx.Done():
			fmt.Fprintln(os.Stderr, n, "objects")
			return
		case m, ok := <-client.Stream():
			if !ok {
				return
			}
			if types.SIMCONNECT_RECV_ID(m.SIMCONNECT_RECV.DwID) != types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE {
				continue
			}
			d := m.AsSimObjectDataBType()
			o := engine.CastDataAs[obj](&d.DwData)
			n++
			if t := engine.BytesToString(o.Title[:]); pat.MatchString(t) {
				fmt.Printf("%-40s id %d  %.6f %.6f  hdg %.0f\n", t, d.DwObjectID, o.Lat, o.Lon, o.Hdg)
			}
		}
	}
}
