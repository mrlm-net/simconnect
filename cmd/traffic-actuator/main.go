// Command traffic-actuator is the simulator side of a split traffic World
// (#710): it runs beside Microsoft Flight Simulator, keeps its connection,
// the aircraft's controllers and their injection, and serves them to a
// traffic-director on the network.
//
//	traffic-actuator -listen :7710 -token s3cret
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
	dataDir := flag.String("data-dir", ".", "directory for local settings (custom pushes, de-icing pads)")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	w := world.New(world.Options{DataDir: *dataDir})
	fmt.Printf("🛰️  actuator on %s\n", *listen)
	if err := world.ServeActuator(ctx, w, *listen, *token); err != nil {
		fmt.Fprintln(os.Stderr, "❌", err)
		os.Exit(1)
	}
}
