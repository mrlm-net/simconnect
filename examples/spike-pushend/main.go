//go:build windows
// +build windows

// Command spike-pushend traces an aircraft every sim frame around the end
// of its pushback: where it moves (meters along and across its heading
// since the last frame), heading, pitch, bank and height, to find the
// shake seen as the tug disconnects. It follows the map's aircraft TAIL
// (by its object ID from /api/traffic) until it has stood 30 s after its
// push. Usage: spike-pushend TAIL; one CSV line per frame on stdout.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"time"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

type frame struct{ Lat, Lon, Hdg, Pitch, Bank, Alt, AGL, Ground, StaticCG float64 }

func objectOf(tail string) uint32 {
	r, err := http.Get("http://127.0.0.1:8080/api/traffic")
	if err != nil {
		return 0
	}
	defer r.Body.Close()
	var all []struct {
		Tail     string `json:"tail"`
		ObjectID uint32 `json:"objectId"`
	}
	_ = json.NewDecoder(r.Body).Decode(&all)
	for _, a := range all {
		if a.Tail == tail {
			return a.ObjectID
		}
	}
	return 0
}

func main() {
	tail := os.Args[1]
	obj := objectOf(tail)
	if obj == 0 {
		fmt.Fprintln(os.Stderr, "no", tail)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	client := simconnect.NewClient("spike-pushend", engine.WithContext(ctx))
	if err := client.Connect(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer client.Disconnect()
	const def, req = 6400, 6401
	for i, v := range []struct{ n, u string }{
		{"PLANE LATITUDE", "degrees"}, {"PLANE LONGITUDE", "degrees"}, {"PLANE HEADING DEGREES TRUE", "degrees"},
		{"PLANE PITCH DEGREES", "degrees"}, {"PLANE BANK DEGREES", "degrees"}, {"PLANE ALTITUDE", "feet"},
		{"PLANE ALT ABOVE GROUND", "feet"}, {"GROUND ALTITUDE", "feet"}, {"STATIC CG TO GROUND", "feet"},
	} {
		client.AddToDataDefinition(def, v.n, v.u, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i))
	}
	client.RequestDataOnSimObject(req, def, obj, types.SIMCONNECT_PERIOD_SIM_FRAME, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)
	start := time.Now()
	var prev frame
	var stillSince time.Time
	moved := false
	fmt.Println("t,along_m,across_m,hdg,pitch,bank,alt,agl,ground,static_cg")
	for {
		select {
		case <-ctx.Done():
			return
		case m, ok := <-client.Stream():
			if !ok || m.SIMCONNECT_RECV == nil {
				continue
			}
			if types.SIMCONNECT_RECV_ID(m.DwID) != types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA {
				continue
			}
			f := engine.CastDataAs[frame](&m.AsSimObjectData().DwData)
			along, across := 0.0, 0.0
			if prev.Lat != 0 {
				d := calc.HaversineMeters(prev.Lat, prev.Lon, f.Lat, f.Lon)
				b := calc.BearingDegrees(prev.Lat, prev.Lon, f.Lat, f.Lon)
				a := (b - f.Hdg) * math.Pi / 180
				along, across = d*math.Cos(a), d*math.Sin(a)
			}
			fmt.Printf("%.3f,%.3f,%.3f,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f\n", time.Since(start).Seconds(), along, across, f.Hdg, f.Pitch, f.Bank, f.Alt, f.AGL, f.Ground, f.StaticCG)
			// Done 30 s after it stopped moving, once it has moved.
			if math.Hypot(along, across) > 0.005 {
				moved, stillSince = true, time.Time{}
			} else if moved && stillSince.IsZero() {
				stillSince = time.Now()
			}
			if moved && !stillSince.IsZero() && time.Since(stillSince) > 30*time.Second {
				return
			}
			prev = *f
		}
	}
}
