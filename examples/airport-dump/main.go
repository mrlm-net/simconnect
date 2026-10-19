// Command airport-dump writes airports' offline data from the simulator —
// layout, SIDs, STARs and approaches, airways and ILS — one directory per
// airport (<out>/<ICAO>/layout.json, procedures.json, airways.json,
// ils.json), for World.LoadAirportDir: the World without a simulator.
//
//	airport-dump -out C:\data LKPR LKKB EGLL EDDF KJFK
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

func main() {
	out := flag.String("out", "airport-data", "the directory to write <ICAO>/ into")
	wait := flag.Duration("wait", 3*time.Minute, "how long to wait for each airport's data")
	flag.Parse()
	icaos := flag.Args()
	if len(icaos) == 0 {
		fmt.Fprintln(os.Stderr, "usage: airport-dump [-out dir] ICAO...")
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	data, err := os.MkdirTemp("", "airport-dump-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer os.RemoveAll(data)
	// A fresh data directory: airways read from the simulator, not a cache.
	w := world.New(world.Options{DataDir: data, AirportDumpDir: *out})
	go w.Run(ctx)
	for !w.Connected() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
	failed := 0
	for _, icao := range icaos {
		icao = strings.ToUpper(icao)
		var v any
		if err := w.Get("/api/airport?icao="+icao, &v); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", icao, err)
			failed++
			continue
		}
		dir := filepath.Join(*out, icao)
		if missing := waitFor(ctx, dir, *wait, "layout.json", "procedures.json"); len(missing) > 0 {
			fmt.Fprintf(os.Stderr, "%s: no %s after %s\n", icao, strings.Join(missing, ", "), *wait)
			failed++
			continue
		}
		// Airways and ILS where it has them (a field without a STAR or an
		// ILS has none): waited for a while.
		if missing := waitFor(ctx, dir, *wait/2, "airways.json", "ils.json"); len(missing) > 0 {
			fmt.Printf("%s: done, none: %s\n", icao, strings.Join(missing, ", "))
			continue
		}
		fmt.Printf("%s: done\n", icao)
	}
	if failed > 0 {
		os.Exit(1)
	}
}

// waitFor waits up to d for dir's files want and names those still
// missing.
func waitFor(ctx context.Context, dir string, d time.Duration, want ...string) []string {
	until := time.Now().Add(d)
	for {
		var missing []string
		for _, f := range want {
			if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
				missing = append(missing, f)
			}
		}
		if len(missing) == 0 || time.Now().After(until) {
			return missing
		}
		select {
		case <-ctx.Done():
			return missing
		case <-time.After(2 * time.Second):
		}
	}
}
