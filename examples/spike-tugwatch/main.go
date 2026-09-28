//go:build windows
// +build windows

// Command spike-tugwatch samples every ground vehicle and aircraft near a
// point twice a second and prints the pushback tugs and the aircraft next to
// them: position relative to the aircraft nose axis, heading and heights,
// to check the injected pushback tug (#304).
package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

type sample struct {
	Title    [256]byte
	Lat, Lon float64
	AltFt    float64
	AGLFt    float64
	Heading  float64
	GS       float64
	Ground   float64
}

func main() {
	secs := flag.Int("seconds", 60, "how long to watch")
	radius := flag.Uint("radius", 3000, "meters around the user aircraft")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	ctx, stop := context.WithTimeout(ctx, time.Duration(*secs)*time.Second)
	defer stop()

	client := simconnect.NewClient("GO Spike - tug watch", engine.WithContext(ctx))
	for client.Connect() != nil {
		time.Sleep(2 * time.Second)
	}
	defer client.Disconnect()
	const def = 1
	for i, v := range []struct{ name, unit string }{
		{"PLANE LATITUDE", "degrees"}, {"PLANE LONGITUDE", "degrees"}, {"PLANE ALTITUDE", "feet"},
		{"PLANE ALT ABOVE GROUND", "feet"}, {"PLANE HEADING DEGREES TRUE", "degrees"}, {"GROUND VELOCITY", "knots"}, {"SIM ON GROUND", "bool"},
	} {
		if i == 0 {
			client.AddToDataDefinition(def, "TITLE", "", types.SIMCONNECT_DATATYPE_STRING256, 0, 0)
		}
		client.AddToDataDefinition(def, v.name, v.unit, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i+1))
	}
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	var tugs, planes []sample
	stream := client.Stream()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			report(tugs, planes)
			tugs, planes = tugs[:0], planes[:0]
			client.RequestDataOnSimObjectType(10, def, uint32(*radius), types.SIMCONNECT_SIMOBJECT_TYPE_GROUND)
			client.RequestDataOnSimObjectType(11, def, uint32(*radius), types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT)
		case msg := <-stream:
			if msg.SIMCONNECT_RECV == nil || types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE {
				continue
			}
			d := msg.AsSimObjectDataBType()
			s := *engine.CastDataAs[sample](&d.DwData)
			switch d.DwRequestID {
			case 10:
				if strings.Contains(strings.ToLower(engine.BytesToString(s.Title[:])), "pushback") {
					tugs = append(tugs, s)
				}
			case 11:
				planes = append(planes, s)
			}
		}
	}
}

func report(tugs, planes []sample) {
	for _, t := range tugs {
		// The nearest aircraft; the tug relative to its reference point.
		best, bd := -1, math.Inf(1)
		for i, p := range planes {
			if d := dist(t, p); d < bd {
				best, bd = i, d
			}
		}
		line := fmt.Sprintf("%s tug %-32s hdg %5.1f gs %4.1f alt %7.1f agl %5.2f ground %.0f",
			time.Now().Format("15:04:05.0"), engine.BytesToString(t.Title[:]), t.Heading, t.GS, t.AltFt, t.AGLFt, t.Ground)
		if best >= 0 && bd < 80 {
			p := planes[best]
			ahead, right := along(p, t)
			line += fmt.Sprintf(" | %s %.1f m ahead %.1f m right of ref, hdg %5.1f (Δ%+.0f), alt %7.1f agl %5.2f",
				engine.BytesToString(p.Title[:]), ahead, right, p.Heading, diff(p.Heading, t.Heading), p.AltFt, p.AGLFt)
		}
		fmt.Println(line)
	}
}

func dist(a, b sample) float64 {
	x, y := offset(a, b)
	return math.Hypot(x, y)
}

// offset is b from a in meters east, north.
func offset(a, b sample) (float64, float64) {
	const m = 111320.0
	return (b.Lon - a.Lon) * m * math.Cos(a.Lat*math.Pi/180), (b.Lat - a.Lat) * m
}

// along is b relative to aircraft a: meters ahead along its heading, and
// to its right.
func along(a, b sample) (float64, float64) {
	x, y := offset(a, b)
	h := a.Heading * math.Pi / 180
	return x*math.Sin(h) + y*math.Cos(h), x*math.Cos(h) - y*math.Sin(h)
}

func diff(a, b float64) float64 { return math.Mod(b-a+540, 360) - 180 }
