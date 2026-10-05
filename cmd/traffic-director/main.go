// Command traffic-director takes a split traffic World's decisions (#710):
// schedule, ATC, sequencing, separation, conflicts. It needs no simulator —
// it runs anywhere, Linux included — and drives a traffic-actuator beside
// the simulator over the network. It serves the World's HTTP API (the
// airport map's), and the map's page with -web.
//
//	traffic-director -actuator simpc:7710 -token s3cret -addr :8080 -web cmd/airport-map/web
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/mrlm-net/simconnect/pkg/nav"
	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

func main() {
	actuator := flag.String("actuator", "127.0.0.1:7710", "the traffic-actuator to drive")
	token := flag.String("token", "", "the actuator's token")
	listen := flag.String("listen", "", "wait on this address (\":7710\") for an actuator that dials in, instead of dialling -actuator (#774)")
	addr := flag.String("addr", "127.0.0.1:8080", "HTTP address of the World's API")
	web := flag.String("web", "", "directory of the airport map's page to serve (cmd/airport-map/web); \"\": the API only")
	airways := flag.String("airways", "", "airway graph for flight plans; \"\": direct routes")
	logDir := flag.String("log-dir", ".", "directory for the traffic log")
	dataDir := flag.String("data-dir", ".", "directory for local settings")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	var graph *nav.AirwayGraph
	if *airways != "" {
		g, err := nav.LoadAirwayGraph(*airways)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  airways: %v (flight plans fly direct)\n", err)
		} else {
			graph = g
		}
	}
	w := world.New(world.Options{LogDir: *logDir, Airways: graph, DataDir: *dataDir})
	if *listen != "" {
		// Actuators behind routers dial in (#774).
		go func() {
			if err := world.ListenDirector(ctx, w, *listen, *token); err != nil {
				fmt.Fprintln(os.Stderr, "❌", err)
				cancel()
			}
		}()
	} else {
		go world.DialDirector(ctx, w, *actuator, *token)
	}

	mux := http.NewServeMux()
	if *web != "" {
		mux.Handle("GET /", http.FileServer(http.Dir(*web)))
	}
	w.Register(mux)
	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shut, done := context.WithTimeout(context.Background(), 2*time.Second)
		defer done()
		srv.Shutdown(shut)
	}()
	of := *actuator
	if *listen != "" {
		of = "actuators dialling " + *listen
	}
	fmt.Printf("director of %s, API on http://%s\n", of, *addr)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, "❌", err)
		os.Exit(1)
	}
}
