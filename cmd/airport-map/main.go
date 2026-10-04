//go:build windows
// +build windows

// Command airport-map loads the complete ground layout of an airport from the
// simulator with pkg/airport (runways, taxi paths, taxi points, taxi names and
// parking spots) and serves it on an interactive Leaflet map at
// http://127.0.0.1:8080, with departure routes computed by pkg/airport.
//
// It is a visual debugging tool for taxi routing and the front end of the
// traffic engine (pkg/traffic/world, #710): its page and its voice.
//
//	go run ./cmd/airport-map                      # live, default LKPR
//	go run ./cmd/airport-map -dump                # also save <ICAO>.json
//	go run ./cmd/airport-map -file LKPR.json      # offline, no simulator
package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

// The page: web/index.html with its styles and scripts. web/classic.html
// is the previous page, served at /classic during the redesign.
//
//go:embed web
var webFiles embed.FS

// serve serves the page, the traffic engine's API and the voice on addr
// until ctx ends.
func serve(ctx context.Context, addr string, w *world.World) error {
	w.SetListenAddr(addr)
	mux := http.NewServeMux()
	web, _ := fs.Sub(webFiles, "web")
	// The app manifest's type, which Go does not know by itself.
	_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")
	mux.Handle("GET /", http.FileServerFS(web))
	mux.HandleFunc("GET /classic", func(rw http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(rw, r, web, "classic.html")
	})
	w.Register(mux)
	registerVoice(mux, radioVoice)
	registerNetwork(mux, w) // the radio as clips, for clients on the network
	radioVoice.setATIS(w.ATISOn)
	// POST /api/voice/atis?icao=LKPR — the airport panel's 🔊: the current
	// ATIS said once through the voice.
	mux.HandleFunc("POST /api/voice/atis", func(rw http.ResponseWriter, r *http.Request) {
		icao := strings.ToUpper(r.URL.Query().Get("icao"))
		text, ok := w.ATISText(icao)
		if !ok {
			http.Error(rw, "no ATIS yet", http.StatusNotFound)
			return
		}
		if !radioVoice.sayOnce(traffic.Transmission{Airport: icao, Position: traffic.PosATIS, Intent: traffic.IntentATIS, Text: text}) {
			http.Error(rw, radioVoice.state().Status, http.StatusServiceUnavailable)
			return
		}
		rw.WriteHeader(http.StatusNoContent)
	})

	srv := &http.Server{Addr: addr, Handler: world.Guard(mux), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()
	fmt.Printf("🗺️  Map at http://%s\n", addr)
	for role, links := range world.ShareLinks() {
		for _, l := range links {
			fmt.Printf("   %s link: %s\n", role, l)
		}
	}
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(v)
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "HTTP listen address")
	icao := flag.String("icao", "LKPR", "airport to open on the map")
	dump := flag.Bool("dump", false, "write each fetched airport's raw facility records to <ICAO>.json")
	scenes := flag.String("scenes", "", "directory of camera scenes (*.json), read on every play; the built-in ones otherwise")
	dumpDir := flag.String("dump-dir", ".", "directory for -dump files")
	file := flag.String("file", "", "serve airport data from a -dump JSON file instead of the simulator")
	logDir := flag.String("log-dir", ".", "directory for the traffic control log (traffic-*.log)")
	airways := flag.String("airways", "pkg/nav/testdata/LKPR-airways.json", "airway graph for flight plans (see examples/spike-airways); \"\" for direct routes")
	airspaceFlag := flag.String("airspace", "D", "class of the managed airports' control zones for VFR rules: C, D, E or G (#570)")
	piperPath := flag.String("piper", "bin/piper/piper.exe", "piper executable for the voice (#419; see the README)")
	voicesDir := flag.String("voices", "", "folder of piper voice models (\"\": voice-goio's user data folder)")
	accents := flag.Bool("accents", false, "controllers speak English with their airport's accent (the country's voice model, see the README)")
	controlToken := flag.String("token", "", "network play: the token another device needs to control the traffic (\"auto\": a random one; \"\": none needed)")
	viewToken := flag.String("view-token", "", "network play: a token to watch only, as a spectator (\"auto\": a random one)")
	flag.Parse()
	zone, err := world.ParseAirspaceClass(*airspaceFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	world.SetTokens(*controlToken, *viewToken)
	startPprof()
	// The default piper is beside the map: its folder in the checkout, or
	// next to the executable once installed (go install, #664).
	if *piperPath == "bin/piper/piper.exe" {
		if _, err := os.Stat(*piperPath); err != nil {
			if exe, err := os.Executable(); err == nil {
				p := filepath.Join(filepath.Dir(exe), "bin", "piper", "piper.exe")
				if _, err := os.Stat(p); err == nil {
					*piperPath = p
				}
			}
		}
	}
	radioVoice.piperPath, radioVoice.voicesDir, radioVoice.accents = *piperPath, *voicesDir, *accents
	// The default is the repo's graph, from the repo root or from this
	// example's folder (its own module: go run . here).
	if *airways == "pkg/nav/testdata/LKPR-airways.json" {
		if _, err := os.Stat(*airways); err != nil {
			*airways = "../../pkg/nav/testdata/LKPR-airways.json"
		}
	}
	var graph *nav.AirwayGraph
	if *airways != "" {
		if g, err := nav.LoadAirwayGraph(*airways); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  airways: %v (flight plans fly direct)\n", err)
		} else {
			graph = g
			fmt.Printf("🛣️  airways: %d fixes, %d airways\n", len(g.Fixes), len(g.Airways))
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	dumpTo := ""
	if *dump {
		dumpTo = *dumpDir
	}
	var w *world.World
	cameraCut = func(t traffic.Transmission) { w.HeardOnCamera(t) }
	w = world.New(world.Options{LogDir: *logDir, Airways: graph, Airspace: zone, DataDir: *dumpDir, DumpDir: dumpTo, Scenes: *scenes,
		// The voice says what the radio carries; it cuts the camera as heard,
		// and without it the camera cuts as it is said.
		OnTransmission: func(t traffic.Transmission) {
			radioVoice.hear(t) // when on (#419)
			if !radioVoice.state().On {
				w.HeardOnCamera(t)
			}
		},
		OnCom1: radioVoice.com1, // the voice follows it when synced
		OnTune: radioVoice.setTune,
		SceneFrequency: func(f string) {
			if s := radioVoice.state(); s.On && !s.SyncCom && f != s.Frequency {
				radioVoice.set(true, f)
			}
		}})

	if *file != "" {
		l, err := w.LoadFile(*file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ %v\n", err)
			os.Exit(1)
		}
		*icao = l
		fmt.Printf("📂 Loaded %s from %s — offline mode\n", l, *file)
	} else {
		go w.Run(ctx)
	}

	fmt.Printf("ℹ️  Open http://%s/?icao=%s (Ctrl+C to exit)\n", *addr, *icao)
	if err := serve(ctx, *addr, w); err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}
}
