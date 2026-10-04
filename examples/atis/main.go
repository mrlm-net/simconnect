//go:build windows
// +build windows

// Command atis reads the weather at the user aircraft, loads an airport's
// layout, chooses the runway in use and prints the airport's ATIS once,
// written and spelled for a voice.
//
//	go run ./examples/atis -icao LKPR -name Ruzyne -prefer 24,06 -ta 5000 -magvar 356
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/nav"
)

func main() {
	icao := flag.String("icao", "LKPR", "airport")
	name := flag.String("name", "", "broadcast name (default: the airport name)")
	prefer := flag.String("prefer", "24,06", "preferential runway ends, comma separated")
	ta := flag.Int("ta", 5000, "transition altitude in feet")
	magVar := flag.Float64("magvar", 0, "magnetic variation as the facility data gives it (magnetic = true + magvar); LKPR 356")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	ctx, stop := context.WithTimeout(ctx, 60*time.Second)
	defer stop()
	client := simconnect.NewClient("GO Example - ATIS", engine.WithContext(ctx))
	for client.Connect() != nil {
		fmt.Println("waiting for the simulator...")
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
	defer client.Disconnect()

	loader := airport.NewLoader(client)
	if err := loader.Request(*icao); err != nil {
		fmt.Println(err)
		return
	}
	wx := nav.NewWeatherReader(client, 10000, 10001)
	if err := wx.Request(); err != nil {
		fmt.Println(err)
		return
	}

	var layout *airport.Layout
	var weather *nav.Weather
	for msg := range client.Stream() {
		if res, ok := loader.Handle(msg); ok {
			if res.Err != nil {
				fmt.Println(res.Err)
				return
			}
			layout = res.Layout
		}
		if w, ok := wx.Handle(msg); ok {
			weather = &w
		}
		if layout != nil && weather != nil {
			break
		}
	}
	if layout == nil || weather == nil {
		fmt.Println("no data before the timeout")
		return
	}

	if *name == "" {
		*name = layout.Name
	}
	lim := nav.RunwayLimits{Preferred: strings.Split(*prefer, ",")}
	svc := nav.NewATISService(*name, layout, lim, *ta, nav.ATISWithMagVar(*magVar))
	a, _ := svc.Update(*weather, time.Now())

	w := *weather
	fmt.Printf("weather at the user aircraft: wind %03.0f°T %.1f kt, visibility %.0f m, %.1f °C, QNH %.1f hPa, precip %s, in cloud %v\n",
		w.WindDirTrue, w.WindKts, w.VisibilityM, w.TempC, w.QNHhPa, w.Precip, w.InCloud)
	fmt.Printf("runway in use: departure %s, arrival %s (headwind %.1f kt, crosswind %.1f kt, within limits %v, approach %s)\n\n",
		a.Use.Departure.Name, a.Use.Arrival.Name, a.Use.HeadwindKts, a.Use.CrosswindKts, a.Use.WithinLimits, a.Use.Approach)
	fmt.Println(a.Text())
	fmt.Println()
	fmt.Println(a.Spoken())
}
