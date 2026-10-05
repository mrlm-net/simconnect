// Command traffic-actuator is the simulator side of a split traffic World
// (#710): it runs beside Microsoft Flight Simulator, keeps its connection,
// the aircraft's controllers and their injection, and serves them to a
// traffic-director on the network.
//
//	traffic-actuator -listen :7710 -token s3cret
//
// Behind a router (multiplayer, #774) it dials the director instead:
//
//	traffic-actuator -director traffic.example.com:7710 -token s3cret
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/mrlm-net/simconnect/pkg/traffic/world"
)

func main() {
	listen := flag.String("listen", ":7710", "address directors connect to")
	token := flag.String("token", "", "token a director must give (\"\": none, trusted network only)")
	director := flag.String("director", "", "dial this director (host:port) instead of listening: for a PC behind a router (#774)")
	dataDir := flag.String("data-dir", ".", "directory for local settings (custom pushes, de-icing pads)")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	w := world.New(world.Options{DataDir: *dataDir})
	var err error
	if *director != "" {
		fmt.Printf("actuator dialling director %s\n", *director)
		err = world.DialActuator(ctx, w, *director, *token)
	} else {
		fmt.Printf("actuator on %s\n", *listen)
		err = world.ServeActuator(ctx, w, *listen, *token)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "❌", err)
		os.Exit(1)
	}
}
