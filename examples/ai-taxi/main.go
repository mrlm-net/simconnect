//go:build windows
// +build windows

// Command ai-taxi spawns an AI aircraft at a stand and taxis it to a runway
// with pkg/airport routing and the pkg/traffic departure taxi controller:
// pushback, taxi to the hold-short point, then line-up and take-off once
// cleared.
//
//	go run ./examples/ai-taxi                                  # LKPR C22 → RWY 24
//	go run ./examples/ai-taxi -stand B2 -runway 30 -takeoff-after 10s
//
// Press Enter at the hold-short point to clear the aircraft for take-off.
// Ctrl+C removes the aircraft and exits.
package main

import (
	"bufio"
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
	stand := flag.String("stand", "C22", "parking stand label, e.g. C22")
	runway := flag.String("runway", "24", "runway end to depart from")
	model := flag.String("model", "FSLTL A320 Air France SL", "aircraft container title")
	livery := flag.String("livery", "", "livery folder name (default livery if empty)")
	tail := flag.String("tail", "CSA123", "tail number / call sign")
	after := flag.Duration("takeoff-after", 0, "clear for take-off automatically this long after holding short (0 = wait for Enter)")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	client := simconnect.NewClient("GO Example - AI taxi", engine.WithContext(ctx))
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
	fleet := traffic.NewFleet(client)
	ctl := traffic.NewTaxiController(fleet)
	if err := loader.Request(*icao); err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		return
	}
	fmt.Printf("🛫 Loading %s...\n", strings.ToUpper(*icao))

	enter := make(chan struct{}, 1)
	go func() {
		s := bufio.NewScanner(os.Stdin)
		for s.Scan() {
			select {
			case enter <- struct{}{}:
			default:
			}
		}
	}()

	var (
		clearTimer <-chan time.Time
		lastPrint  time.Time
		lastState  traffic.TaxiState
		events     = ctl.Events()
	)
	clearForTakeoff := func() {
		if err := ctl.ClearForTakeoff(); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Clear for take-off: %v\n", err)
			return
		}
		fmt.Printf("🟢 %s cleared for take-off runway %s\n", *tail, ctl.Route().RunwayEnd)
	}

	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	stream := client.Stream()
	for {
		select {
		case <-ctx.Done():
			if err := ctl.Cancel(); err != nil {
				fmt.Fprintf(os.Stderr, "❌ Cancel: %v\n", err)
			}
			fmt.Println("🛑 Aircraft removed, exiting")
			return

		case now := <-tick.C:
			for _, res := range loader.Expire(now) {
				fmt.Fprintf(os.Stderr, "❌ %v\n", res.Err)
				return
			}

		case <-enter:
			if ctl.State() == traffic.TaxiHoldingShort {
				clearForTakeoff()
			}

		case <-clearTimer:
			clearForTakeoff()

		case ev, ok := <-events:
			if !ok {
				events = nil // terminal; keep running until Ctrl+C so the aircraft keeps flying
				fmt.Println("ℹ️  Controller finished. Ctrl+C to remove the aircraft and exit.")
				continue
			}
			if ev.Err != nil {
				fmt.Fprintf(os.Stderr, "⚠️  %s: %v\n", ev.State, ev.Err)
			}
			if ev.State != lastState {
				lastState = ev.State
				fmt.Printf("✈️  %s: %s\n", *tail, ev.State)
				if ev.State == traffic.TaxiHoldingShort {
					if *after > 0 {
						fmt.Printf("⏱️  Clearing for take-off in %s\n", *after)
						clearTimer = time.After(*after)
					} else {
						fmt.Println("⏎  Press Enter to clear for take-off")
					}
				}
				continue
			}
			if time.Since(lastPrint) >= 5*time.Second && (ev.State == traffic.TaxiPushback || ev.State == traffic.TaxiTaxiing) {
				lastPrint = time.Now()
				fmt.Printf("   %-8s %5.0f m to hold short · %4.1f kt · hdg %3.0f°\n",
					orDash(ev.Taxiway), ev.Remaining, ev.GroundSpeed, ev.Heading)
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
				if err := start(ctl, cache, res.Layout, *stand, *runway, *model, *livery, *tail); err != nil {
					fmt.Fprintf(os.Stderr, "❌ %v\n", err)
					return
				}
				continue
			}
			if ctl.Handle(msg) {
				continue
			}
			if types.SIMCONNECT_RECV_ID(msg.DwID) == types.SIMCONNECT_RECV_ID_EXCEPTION {
				e := msg.AsException()
				fmt.Fprintf(os.Stderr, "⚠️  SimConnect exception %d (sendID=%d, index=%d)", e.DwException, e.DwSendID, e.DwIndex)
				if types.SIMCONNECT_EXCEPTION(e.DwException) == types.SIMCONNECT_EXCEPTION_CREATE_OBJECT_FAILED {
					fmt.Fprintf(os.Stderr, " — could not create %q; check -model", *model)
				}
				fmt.Fprintln(os.Stderr)
			}
		}
	}
}

// start plans the route and spawns the aircraft once the layout is loaded.
func start(ctl *traffic.TaxiController, cache *airport.Cache, l *airport.Layout, stand, runway, model, livery, tail string) error {
	g, err := cache.Graph(l.ICAO)
	if err != nil {
		return err
	}
	parking, err := l.ParkingIndex(stand)
	if err != nil {
		return err
	}
	if err := ctl.Start(traffic.TaxiRequest{
		Graph: g, Parking: parking, Runway: runway,
		Model: model, Livery: livery, Tail: tail,
	}); err != nil {
		return err
	}
	r := ctl.Route()
	fmt.Printf("🗺️  %s %s → runway %s: %.0f m via %s", l.ICAO, stand, r.RunwayEnd, r.Length, strings.Join(r.Taxiways, " → "))
	if len(r.RunwayCrossings) > 0 {
		fmt.Printf(", crossing %s", strings.Join(r.RunwayCrossings, ", "))
	}
	fmt.Println()
	return nil
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
