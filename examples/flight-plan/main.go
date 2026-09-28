//go:build windows
// +build windows

// Command flight-plan plans an IFR flight between two airports loaded
// from the simulator (layouts and procedures), with the weather at the
// user aircraft choosing the departure runway, over the airways of a
// saved airway graph. It prints the plan and writes an MSFS .pln file.
//
//	go run ./examples/flight-plan -from LKPR -to LOWW -type A20N -out LKPRLOWW.pln
package main

import (
	"context"
	"encoding/json"
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
	from := flag.String("from", "LKPR", "departure airport")
	to := flag.String("to", "LOWW", "destination airport")
	typ := flag.String("type", "A20N", "ICAO aircraft type")
	fl := flag.Int("fl", 0, "cruise flight level (0: chosen)")
	depRwy := flag.String("dep-rwy", "", "departure runway (default: in use for the weather at the user aircraft)")
	arrRwy := flag.String("arr-rwy", "", "arrival runway (default: the longest)")
	prefer := flag.String("prefer", "24,06", "preferential departure runway ends, comma separated")
	airways := flag.String("airways", "pkg/nav/testdata/LKPR-airways.json", "airway graph JSON (spike-airways); empty for direct")
	out := flag.String("out", "", "write the .pln here")
	asJSON := flag.String("json", "", "write the plan as JSON here")
	flag.Parse()

	var g *nav.AirwayGraph
	if *airways != "" {
		var err error
		if g, err = nav.LoadAirwayGraph(*airways); err != nil {
			fmt.Println("airways:", err)
			return
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	ctx, stop := context.WithTimeout(ctx, 60*time.Second)
	defer stop()
	client := simconnect.NewClient("GO Example - flight plan", engine.WithContext(ctx))
	for client.Connect() != nil {
		fmt.Println("waiting for the simulator...")
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
	defer client.Disconnect()

	// Layouts, procedures and the weather, all through one message loop.
	loader := airport.NewLoader(client)
	procs := airport.NewProcedureLoader(client)
	wx := nav.NewWeatherReader(client, 7400, 7401)
	dep, arr := strings.ToUpper(*from), strings.ToUpper(*to)
	for _, icao := range []string{dep, arr} {
		if err := loader.Request(icao); err != nil {
			fmt.Println(err)
			return
		}
		if err := procs.Request(icao); err != nil {
			fmt.Println(err)
			return
		}
	}
	if err := wx.Request(); err != nil {
		fmt.Println(err)
		return
	}
	layouts := map[string]*airport.Layout{}
	procedures := map[string]*airport.Procedures{}
	var weather *nav.Weather
	for msg := range client.Stream() {
		if res, ok := loader.Handle(msg); ok {
			if res.Err != nil {
				fmt.Println(res.ICAO, res.Err)
				return
			}
			layouts[res.ICAO] = res.Layout
		}
		if p, ok := procs.Handle(msg); ok {
			procedures[p.ICAO] = &p
		}
		if w, ok := wx.Handle(msg); ok {
			weather = &w
		}
		if len(layouts) == 2 && len(procedures) == 2 && weather != nil {
			break
		}
	}
	if len(layouts) < 2 || len(procedures) < 2 || weather == nil {
		fmt.Println("no data before the timeout")
		return
	}

	req := nav.FlightPlanRequest{
		Departure:       nav.AirportInfo{ICAO: dep, Layout: layouts[dep], Procedures: procedures[dep]},
		Arrival:         nav.AirportInfo{ICAO: arr, Layout: layouts[arr], Procedures: procedures[arr]},
		Type:            *typ,
		CruiseFL:        *fl,
		DepartureRunway: *depRwy,
		ArrivalRunway:   *arrRwy,
		DepWeather:      weather,
		DepLimits:       nav.RunwayLimits{Preferred: strings.Split(*prefer, ",")},
	}
	fp, err := nav.Plan(req, g)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%s: %d SIDs, %d STARs, %d approaches; %s: %d SIDs, %d STARs, %d approaches\n",
		dep, len(procedures[dep].Departures), len(procedures[dep].Arrivals), len(procedures[dep].Approaches),
		arr, len(procedures[arr].Departures), len(procedures[arr].Arrivals), len(procedures[arr].Approaches))
	fmt.Printf("weather at the user aircraft: wind %03.0f°T %.0f kt\n\n", weather.WindDirTrue, weather.WindKts)
	fmt.Print(fp)

	if *out != "" {
		b, err := fp.PLN()
		if err == nil {
			err = os.WriteFile(*out, b, 0o644)
		}
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println("\nwrote", *out)
	}
	if *asJSON != "" {
		b, _ := json.MarshalIndent(fp, "", " ")
		if err := os.WriteFile(*asJSON, b, 0o644); err != nil {
			fmt.Println(err)
		}
	}
}
