//go:build windows
// +build windows

// Command spike-airways reads waypoints, navaids and their airways from the
// facility API (#328). With -raw N it dumps one stage of the record layout
// for a couple of fixes; otherwise it crawls the airway network around a
// centre with nav.AirwayCrawler and writes the graph as JSON.
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
	raw := flag.Int("raw", 0, "dump raw records of definition stage 1-4 instead of crawling")
	fixes := flag.String("fixes", "VOZ:LK:V,GOLOP:LK:W", "raw: fixes as IDENT:REGION:KIND")
	seeds := flag.String("seeds", "VOZ:LK:V,VLM:LK:V,OKL:LK:V,GOLOP:LK:W,LOMKI:LK:W,APRAQ:LK:W,ARTUP:LK:W,BALTU:LK:W,DOBEN:LK:W,VENOX:LK:W",
		"crawl: seed fixes as IDENT:REGION:KIND")
	lat := flag.Float64("lat", 50.1008, "crawl: centre latitude (default LKPR)")
	lon := flag.Float64("lon", 14.26, "crawl: centre longitude")
	radius := flag.Float64("radius", nav.DefaultCrawlRadiusNM, "crawl: radius in NM")
	maxReq := flag.Int("max", nav.DefaultCrawlMaxRequests, "crawl: maximum fix requests")
	out := flag.String("out", "airways.json", "crawl: write the graph as JSON here")
	timeout := flag.Duration("timeout", 10*time.Minute, "give up after this long")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	ctx, stop := context.WithTimeout(ctx, *timeout)
	defer stop()
	client := simconnect.NewClient("GO Spike - airways", engine.WithContext(ctx))
	for client.Connect() != nil {
		time.Sleep(2 * time.Second)
	}
	defer client.Disconnect()
	if *raw > 0 {
		var targets []rawTarget
		for _, f := range strings.Split(*fixes, ",") {
			p := strings.Split(f, ":")
			if len(p) != 3 || p[2] == "" {
				fmt.Println("bad fix", f)
				return
			}
			targets = append(targets, rawTarget{ident: p[0], region: p[1], kind: p[2][0]})
		}
		rawDump(ctx, client, *raw, targets)
		return
	}
	crawl(ctx, client, *seeds, *lat, *lon, *radius, *maxReq, *out)
}

// crawl follows the airways from the seeds and writes the graph to out.
func crawl(ctx context.Context, client engine.Client, seedList string, lat, lon, radius float64, maxReq int, out string) {
	var seeds []nav.FixKey
	for _, s := range strings.Split(seedList, ",") {
		k, err := nav.ParseFixKey(s)
		if err != nil {
			fmt.Println(err)
			return
		}
		seeds = append(seeds, k)
	}
	c := nav.NewAirwayCrawler(nav.NewNavLoader(client), nav.CrawlOptions{Center: airport.LatLon{Lat: lat, Lon: lon}, RadiusNM: radius, MaxRequests: maxReq})
	if err := c.Start(seeds...); err != nil {
		fmt.Println(err)
		return
	}
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	started := time.Now()
	done := false
	for !done {
		var err error
		select {
		case <-ctx.Done():
			fmt.Println("stopped:", ctx.Err())
			done = true
		case now := <-tick.C:
			done, err = c.Tick(now)
			fmt.Printf("\r%d requests", c.Requests())
		case msg := <-client.Stream():
			done, err = c.Handle(msg)
		}
		if err != nil {
			fmt.Println(err)
			return
		}
	}
	g := c.Graph()
	fmt.Printf("\n%d requests in %s, %d fixes, %d airways, %d segments, %d not found: %v\n", c.Requests(), time.Since(started).Round(time.Millisecond),
		len(g.Fixes), len(g.Airways), g.SegmentCount(), len(c.Missing()), c.Missing())
	if out != "" {
		if err := g.SaveJSON(out); err != nil {
			fmt.Println(err)
		}
	}
}
