//go:build windows
// +build windows

// Command ai-arrival lands an AI aircraft and taxis it to a stand with the
// pkg/traffic ArrivalController: spawn on final, gear down, approach,
// touchdown, rollout to a runway exit and taxi-in to the stand.
//
//	go run ./examples/ai-arrival                          # LKPR RWY 24 → C22
//	go run ./examples/ai-arrival -runway 30 -stand B2 -ground-agl
//
// Ctrl+C removes the aircraft and exits.
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
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

func main() {
	icao := flag.String("icao", "LKPR", "airport ICAO code")
	runway := flag.String("runway", "24", "runway end to land on")
	stand := flag.String("stand", "C22", "parking stand label")
	model := flag.String("model", "FSLTL A320 Air France SL", "aircraft container title")
	livery := flag.String("livery", "", "livery folder name")
	tail := flag.String("tail", "CSA456", "tail number / call sign")
	spawnNm := flag.Float64("spawn-nm", traffic.DefaultSpawnNm, "distance out on final to spawn")
	groundAGL := flag.Bool("ground-agl", false, "ground waypoints at 0 ft AGL instead of airport elevation")
	keep := flag.Bool("keep", false, "leave the parked aircraft in the sim on exit")
	noStop := flag.Bool("no-stop", false, "disable the active stop waypoint at the stand (comparison)")
	nose := flag.Float64("nose", 0, "reference-point-to-nose distance in meters (0 = default)")
	hold := flag.Bool("hold", false, "hold clear of the runway until Enter (taxi clearance)")
	dwell := flag.Duration("dwell", 0, "after-landing stop before taxiing on (0 = default)")
	inject := flag.Bool("inject", false, "hybrid: MSFS AI lands, position injection takes over during the rollout and drives the ground phase (#309)")
	injectApproach := flag.Bool("inject-approach", false, "with -inject: fly the final approach, flare and touchdown by injection too (#318)")
	holdCrossings := flag.Bool("hold-crossings", false, "with -inject: hold short of runway crossings until cleared (the demo clears after -cross-after)")
	crossAfter := flag.Duration("cross-after", 15*time.Second, "demo ATC: crossing clearance delay with -hold-crossings")
	rollThrough := flag.Float64("roll-through", 0, "with -inject: chance 0..1 of a rolling clearance at the vacate point (0 = default 0.3, negative = never)")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	client := simconnect.NewClient("GO Example - AI arrival", engine.WithContext(ctx))
	fmt.Println("⏳ Waiting for simulator...")
	for client.Connect() != nil {
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
	defer client.Disconnect()
	fmt.Println("✅ Connected")

	cache := airport.NewCache()
	loader := airport.NewLoader(client, airport.LoaderWithCache(cache))
	inj := traffic.NewInjector(client)
	var opts []traffic.ArrivalOption
	if *inject {
		opts = append(opts, traffic.ArrivalWithInjector(inj))
	}
	ctl := traffic.NewArrivalController(traffic.NewFleet(client), opts...)
	if err := loader.Request(*icao); err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		return
	}

	var (
		events     = ctl.Events()
		lastState  traffic.ArrivalState
		lastLights traffic.Lights
		lastPrint  time.Time
		parked     bool
	)
	enter := make(chan struct{}, 1)
	go func() {
		b := make([]byte, 64)
		for {
			if n, err := os.Stdin.Read(b); err != nil || n == 0 {
				return
			}
			select {
			case enter <- struct{}{}:
			default:
			}
		}
	}()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	stream := client.Stream()
	for {
		select {
		case <-ctx.Done():
			if parked && *keep {
				fmt.Println("🛑 Exiting; aircraft left at the stand")
				return
			}
			if err := ctl.Cancel(); err != nil {
				fmt.Fprintf(os.Stderr, "❌ Cancel: %v\n", err)
			}
			if parked {
				// Parked controllers are terminal; remove through the fleet.
				traffic.NewFleet(client).Remove(ctl.ObjectID(), traffic.DefaultArrivalRequestBase+3)
			}
			fmt.Println("🛑 Aircraft removed, exiting")
			return

		case <-enter:
			if ctl.State() == traffic.ArrivalAwaitingTaxi {
				fmt.Println("🟢 Cleared to taxi")
				ctl.ClearToTaxi()
			}

		case now := <-tick.C:
			for _, res := range loader.Expire(now) {
				fmt.Fprintf(os.Stderr, "❌ %v\n", res.Err)
				return
			}

		case ev, ok := <-events:
			if !ok {
				events = nil
				fmt.Println("ℹ️  Controller finished. Ctrl+C to remove the aircraft and exit.")
				continue
			}
			if ev.Err != nil {
				fmt.Fprintf(os.Stderr, "⚠️  %s: %v\n", ev.State, ev.Err)
			}
			if ev.Lights != lastLights && ev.State >= traffic.ArrivalRollout {
				fmt.Printf("💡 %s  %s → %s (%s)\n", time.Now().Format("15:04:05.000"), lastLights, ev.Lights, ev.State)
			}
			lastLights = ev.Lights
			if ev.State != lastState {
				lastState = ev.State
				extra := ""
				switch ev.State {
				case traffic.ArrivalRollout:
					extra = fmt.Sprintf(" — touchdown %.0f m past the threshold at %.0f kt, %.0f fpm", ev.Touchdown, ev.GroundSpeed, ev.TouchdownFpm)
				case traffic.ArrivalAwaitingTaxi:
					if *hold {
						extra = " — holding clear of the runway, press Enter to clear to taxi"
					}
				case traffic.ArrivalHoldingShort:
					// The demo's ATC: cleared to cross after a short wait.
					extra = fmt.Sprintf(" %s — cleared to cross in %s", ev.HoldingShortOf, *crossAfter)
					time.AfterFunc(*crossAfter, ctl.ClearToCross)
				case traffic.ArrivalParked:
					parked = true
				}
				fmt.Printf("✈️  %s: %s%s\n", *tail, ev.State, extra)
				continue
			}
			if time.Since(lastPrint) >= time.Second {
				lastPrint = time.Now()
				if ev.State >= traffic.ArrivalRollout {
					fmt.Printf("   %-6s %5.0f m to stand · %5.1f kt · hdg %3.0f° · lights %s\n", orDash(ev.Taxiway), ev.Remaining, ev.GroundSpeed, ev.Heading, ev.Lights)
				} else {
					fmt.Printf("   %5.0f ft AGL · %5.1f kt · hdg %3.0f°\n", ev.AGL, ev.GroundSpeed, ev.Heading)
				}
			}

		case msg, ok := <-stream:
			if !ok {
				fmt.Println("📴 Simulator disconnected")
				return
			}
			if msg.Err != nil {
				continue
			}
			if res, done := loader.Handle(msg); done {
				if res.Err != nil {
					fmt.Fprintf(os.Stderr, "❌ %v\n", res.Err)
					return
				}
				g, err := cache.Graph(res.Layout.ICAO)
				if err == nil {
					var parking int
					if parking, err = res.Layout.ParkingIndex(*stand); err == nil {
						err = ctl.Start(traffic.ArrivalRequest{Graph: g, Runway: *runway, Parking: parking, Model: *model,
							Livery: *livery, Tail: *tail, SpawnNm: *spawnNm, GroundAGL: *groundAGL, NoStopWaypoint: *noStop, NoseOffset: *nose, HoldForClearance: *hold, AfterLandingDwell: *dwell, RollThroughChance: *rollThrough, HoldAtCrossings: *holdCrossings, InjectApproach: *injectApproach})
					}
				}
				if err != nil {
					fmt.Fprintf(os.Stderr, "❌ %v\n", err)
					return
				}
				p := ctl.Plan()
				fmt.Printf("🗺️  %s RWY %s → %s: spawn %.0f nm, exit %s at %.0f m (%.0f°%s), taxi-in %.0f m via %s",
					res.Layout.ICAO, p.End.Name, *stand, p.SpawnNm, p.Exit.Taxiway, p.Exit.Along, p.Exit.Angle,
					map[bool]string{true: ", high-speed", false: ""}[p.Exit.HighSpeed], p.Route.Length, strings.Join(p.Route.Taxiways, " → "))
				if len(p.Route.RunwayCrossings) > 0 {
					fmt.Printf(", crossing %s", strings.Join(p.Route.RunwayCrossings, ", "))
				}
				fmt.Printf(" (%d waypoints)\n", len(p.Waypoints))
				continue
			}
			if ok, err := inj.Handle(msg); ok {
				if err != nil {
					fmt.Fprintf(os.Stderr, "⚠️  %v\n", err)
				}
				continue
			}
			if ctl.Handle(msg) {
				continue
			}
			if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_EXCEPTION {
				e := msg.AsException()
				fmt.Fprintf(os.Stderr, "⚠️  SimConnect exception %d (sendID=%d, index=%d)\n", e.DwException, e.DwSendID, e.DwIndex)
			}
		}
	}
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
