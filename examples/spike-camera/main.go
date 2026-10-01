//go:build windows
// +build windows

// Command spike-camera probes the MSFS 2024 add-on camera API: it reads the
// camera in world coordinates beside the user aircraft's position (to tell
// which of x, y, z is latitude, longitude, altitude, and in what units),
// then places the camera 40 m behind and 8 m above the user aircraft
// looking at it, holds it there, and gives it back.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

const (
	defPos = 7100
	reqPos = 7101
)

type pos struct{ Lat, Lon, AltFt, Hdg float64 }

func main() {
	hold := flag.Duration("hold", 8*time.Second, "how long to keep the camera")
	x := flag.Float64("x", 0, "camera offset x from the aircraft, m")
	y := flag.Float64("y", 8, "camera offset y (up), m")
	z := flag.Float64("z", -40, "camera offset z (forward), m")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	ctx, stop := context.WithTimeout(ctx, *hold+30*time.Second)
	defer stop()

	c := simconnect.NewClient("GO Spike - camera", engine.WithContext(ctx))
	for c.Connect() != nil {
		time.Sleep(2 * time.Second)
	}
	defer c.Disconnect()
	e := c.(*engine.Engine)
	step := func(what string, err error) {
		fmt.Printf("%-40s %v\n", what, err)
	}
	for i, v := range [][2]string{{"PLANE LATITUDE", "degrees"}, {"PLANE LONGITUDE", "degrees"}, {"PLANE ALTITUDE", "feet"}, {"PLANE HEADING DEGREES TRUE", "degrees"}} {
		c.AddToDataDefinition(defPos, v[0], v[1], types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i))
	}
	step("RequestDataOnSimObject user", c.RequestDataOnSimObject(reqPos, defPos, types.SIMCONNECT_OBJECT_ID_USER, types.SIMCONNECT_PERIOD_ONCE, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0))
	step("SubscribeToCameraStatusUpdate", e.SubscribeToCameraStatusUpdate())
	step("CameraGetStatus", e.CameraGetStatus())
	step("CameraAcquire", e.CameraAcquire("GO Spike - camera"))

	stream := c.Stream()
	acquired, placed := false, time.Time{}
	var released bool
	for {
		select {
		case <-ctx.Done():
			if !released {
				step("CameraRelease (timeout)", e.CameraRelease(""))
			}
			return
		case msg := <-stream:
			if msg.SIMCONNECT_RECV == nil {
				continue
			}
			switch types.SIMCONNECT_RECV_ID(msg.DwID) {
			case types.SIMCONNECT_RECV_ID_EXCEPTION:
				x := msg.AsException()
				fmt.Printf("exception %d (send %d, index %d)\n", x.DwException, x.DwSendID, x.DwIndex)
			case types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA:
				d := msg.AsSimObjectData()
				p := engine.CastDataAs[pos](&d.DwData)
				fmt.Printf("user aircraft: lat %.6f lon %.6f alt %.1f ft (%.1f m) hdg %.1f\n", p.Lat, p.Lon, p.AltFt, p.AltFt*0.3048, p.Hdg)
			case types.SIMCONNECT_RECV_ID_CAMERA_STATUS:
				s := msg.AsCameraStatus()
				fmt.Printf("camera status: acquired %d, game controlled %d\n", s.AcquiredState, s.GameControlled)
				if s.AcquiredState == types.SIMCONNECT_CAMERA_ACQUIRED && !acquired {
					acquired = true
					step("CameraGet world", e.CameraGet(types.SIMCONNECT_POSITION_REFERENTIAL_WORLD))
					step("CameraGet simobject", e.CameraGet(types.SIMCONNECT_POSITION_REFERENTIAL_SIMOBJECT))
					cam := types.SIMCONNECT_DATA_CAMERA{
						Position:            types.SIMCONNECT_DATA_XYZ{X: *x, Y: *y, Z: *z},
						PositionReferential: types.SIMCONNECT_POSITION_REFERENTIAL_SIMOBJECT,
						TargetedPos:         types.SIMCONNECT_DATA_XYZ{X: 0, Y: 2, Z: 0},
						RotationReferential: types.SIMCONNECT_POSITION_REFERENTIAL_SIMOBJECT,
						Fov:                 0.9,
					}
					step("CameraSet behind, targeted", e.CameraSet(cam, types.SIMCONNECT_CAMERA_DATA_MASK_ALL_TARGETED))
					placed = time.Now()
				}
			case types.SIMCONNECT_RECV_ID_CAMERA_DATA:
				d, ok := msg.AsCameraData()
				fmt.Printf("camera data (%v): %+v\n", ok, d)
			}
		case <-time.After(250 * time.Millisecond):
		}
		if !placed.IsZero() && !released && time.Since(placed) > *hold {
			step("CameraGet world (placed)", e.CameraGet(types.SIMCONNECT_POSITION_REFERENTIAL_WORLD))
			step("CameraGet simobject (placed)", e.CameraGet(types.SIMCONNECT_POSITION_REFERENTIAL_SIMOBJECT))
			released = true
			go func() { time.Sleep(time.Second); step("CameraRelease", e.CameraRelease("")); time.Sleep(500 * time.Millisecond); stop() }()
		}
	}
}
