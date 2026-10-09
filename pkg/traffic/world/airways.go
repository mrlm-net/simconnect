package world

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/nav"
)

// Airways from the simulator (#799): the airway graph around every airport
// the World loads, read from the sim's own navigation data (nav
// AirwayCrawler, seeded from the airport's SID and STAR fixes), so flight
// plans fly airways everywhere, not only where a graph was given
// (Options.Airways). Each airport's graph is kept in DataDir/airways/
// <ICAO>.json and read from the sim again once it is older than
// Options.AirwaysMaxAge (the sim's navigation data is updated with its
// AIRAC cycles), also while the World runs.

// DefaultAirwaysMaxAge: an airport's airways are read again after this.
const DefaultAirwaysMaxAge = 7 * 24 * time.Hour

// airwayCrawlRadiusNM: the airways read around an airport, enough for the
// flights that start or end there to join them (nav.DefaultCrawlRadiusNM).
const airwayCrawlRadiusNM = nav.DefaultCrawlRadiusNM

// airwayKeeper reads airports' airways on the connection's goroutine, one
// airport at a time, and hands each graph to the feed.
type airwayKeeper struct {
	client           nav.FacilityClient
	feed             simFeed
	dir              string
	maxAge           time.Duration
	defBase, reqBase uint32
	logf             func(string, ...any)

	queue []airwayJob
	cur   *airwayJob
	read  map[string]time.Time // airport → when its graph was read or loaded
	seeds map[string]airwayJob // airport → its job, to read it again
}

type airwayJob struct {
	icao    string
	center  airport.LatLon
	seeds   []nav.FixKey
	crawler *nav.AirwayCrawler
	started time.Time
	// A route's airways (route): the crawl's bounds, and who waits for it.
	opts *nav.CrawlOptions
	done []chan struct{}
}

// airwayRoute asks for the airways along a flight's way: key names it
// ("LPMA-LPPT", its cache file), seeds start the crawl, corridor bounds it;
// done is closed once they are in the flight plans' graph (or none found).
type airwayRoute struct {
	key      string
	seeds    []nav.FixKey
	corridor nav.Corridor
	done     chan struct{}
}

// Route crawls: the band either side of the great circle, and requests
// allowed per NM of it on top of a whole airport's crawl.
const (
	routeCorridorHalfNM  = 60.0
	routeCrawlPerNM      = 6
	routeCrawlMaxRequest = 15000
)

// route reads the airways along r's way: from the cache when fresh, else
// from the sim after the crawls queued before it.
func (k *airwayKeeper) route(r airwayRoute, now time.Time) {
	key := "route-" + strings.ToUpper(r.key)
	if k.cur != nil && k.cur.icao == key {
		k.cur.done = append(k.cur.done, r.done)
		return
	}
	for i := range k.queue {
		if k.queue[i].icao == key {
			k.queue[i].done = append(k.queue[i].done, r.done)
			return
		}
	}
	if info, err := os.Stat(k.path(key)); err == nil && now.Sub(info.ModTime()) < k.maxAge {
		if g, err := nav.LoadAirwayGraph(k.path(key)); err == nil {
			k.feed.Airways(key, g)
			close(r.done)
			return
		}
	}
	if len(r.seeds) == 0 {
		close(r.done)
		return
	}
	c := r.corridor
	lengthNM := calc.HaversineNM(c.From.Lat, c.From.Lon, c.To.Lat, c.To.Lon)
	opts := nav.CrawlOptions{Corridor: &c, MaxRequests: min(routeCrawlMaxRequest, nav.DefaultCrawlMaxRequests+int(lengthNM)*routeCrawlPerNM)}
	k.queue = append(k.queue, airwayJob{icao: key, seeds: r.seeds, opts: &opts, done: []chan struct{}{r.done}})
}

func newAirwayKeeper(client nav.FacilityClient, feed simFeed, dataDir string, maxAge time.Duration, defBase, reqBase uint32, logf func(string, ...any)) *airwayKeeper {
	if maxAge <= 0 {
		maxAge = DefaultAirwaysMaxAge
	}
	return &airwayKeeper{client: client, feed: feed, dir: filepath.Join(dataDir, "airways"), maxAge: maxAge,
		defBase: defBase, reqBase: reqBase, logf: logf, read: map[string]time.Time{}, seeds: map[string]airwayJob{}}
}

// want has p's airport's airways: from the cache when it is fresh (a stale
// one is used while it is read again), else read from the sim.
func (k *airwayKeeper) want(p airport.Procedures, now time.Time) {
	icao := strings.ToUpper(p.ICAO)
	if _, ok := k.read[icao]; ok || icao == "" {
		return
	}
	job, ok := airwayJobOf(p)
	if !ok {
		return // no SID or STAR fix to start from
	}
	k.seeds[icao] = job
	path := k.path(icao)
	if info, err := os.Stat(path); err == nil {
		if g, err := nav.LoadAirwayGraph(path); err == nil {
			k.feed.Airways(icao, g)
			k.read[icao] = info.ModTime()
			if now.Sub(info.ModTime()) < k.maxAge {
				return
			}
			k.logf("airways %s: cache from %s, reading them again", icao, info.ModTime().Format("2006-01-02"))
		}
	}
	k.read[icao] = now
	k.queue = append(k.queue, job)
}

// airwayJobOf seeds a crawl with the waypoints, VORs and NDBs of p's SIDs
// and STARs, centred on them.
func airwayJobOf(p airport.Procedures) (airwayJob, bool) {
	job := airwayJob{icao: strings.ToUpper(p.ICAO)}
	seen := map[nav.FixKey]bool{}
	var lat, lon float64
	add := func(legs []airport.Leg) {
		for _, l := range legs {
			if l.Fix == "" || l.Region == "" || len(l.FixKind) != 1 || !strings.Contains("WVN", l.FixKind) {
				continue
			}
			key := nav.Key(l.Fix, l.Region, nav.FixKind(l.FixKind[0]))
			if seen[key] {
				continue
			}
			seen[key] = true
			job.seeds = append(job.seeds, key)
			lat, lon = lat+l.Position.Lat, lon+l.Position.Lon
		}
	}
	for _, list := range [][]airport.Procedure{p.Departures, p.Arrivals} {
		for _, pr := range list {
			add(pr.Legs)
			for _, t := range append(append([]airport.Transition(nil), pr.RunwayTransitions...), pr.EnrouteTransitions...) {
				add(t.Legs)
			}
		}
	}
	if len(job.seeds) == 0 {
		return job, false
	}
	n := float64(len(job.seeds))
	job.center = airport.LatLon{Lat: lat / n, Lon: lon / n}
	return job, true
}

func (k *airwayKeeper) path(icao string) string { return filepath.Join(k.dir, icao+".json") }

// handle feeds msg to the crawl running; true when it was its.
func (k *airwayKeeper) handle(msg engine.Message) bool {
	if k.cur == nil {
		return false
	}
	ok, err := k.cur.crawler.Handle(msg)
	if err != nil {
		k.logf("airways %s: %v", k.cur.icao, err)
	}
	return ok
}

// tick keeps the crawl going, delivers a finished graph and starts the
// next; airports read more than maxAge ago are read again.
func (k *airwayKeeper) tick(now time.Time) {
	k.dueAgain(now)
	if k.cur != nil {
		if _, err := k.cur.crawler.Tick(now); err != nil {
			k.logf("airways %s: %v", k.cur.icao, err)
		}
		if !k.cur.crawler.Done() {
			return
		}
		k.finish(now)
	}
	if len(k.queue) == 0 {
		return
	}
	job := k.queue[0]
	k.queue = k.queue[1:]
	loader := nav.NewNavLoaderWithIDs(k.client, k.defBase, k.reqBase, airwayCrawlSlots)
	opts := nav.CrawlOptions{Center: job.center, RadiusNM: airwayCrawlRadiusNM}
	if job.opts != nil {
		opts = *job.opts
	}
	job.crawler = nav.NewAirwayCrawler(loader, opts)
	job.started = now
	if err := job.crawler.Start(job.seeds...); err != nil {
		k.logf("airways %s: %v", job.icao, err)
		for _, d := range job.done {
			close(d)
		}
		return
	}
	k.cur = &job
	k.logf("airways %s: reading from the sim, %d seed fixes", job.icao, len(job.seeds))
}

// dueAgain queues the airports read maxAge or longer ago; true when any.
func (k *airwayKeeper) dueAgain(now time.Time) bool {
	due := false
	for icao, at := range k.read {
		if now.Sub(at) < k.maxAge || k.queued(icao) {
			continue
		}
		if job, ok := k.seeds[icao]; ok {
			k.read[icao] = now
			k.queue = append(k.queue, job)
			due = true
		}
	}
	return due
}

// airwayCrawlSlots: facility requests in flight at once while crawling.
const airwayCrawlSlots = 8

func (k *airwayKeeper) queued(icao string) bool {
	if k.cur != nil && k.cur.icao == icao {
		return true
	}
	for _, j := range k.queue {
		if j.icao == icao {
			return true
		}
	}
	return false
}

// finish delivers and keeps the crawl's graph.
func (k *airwayKeeper) finish(now time.Time) {
	job := k.cur
	k.cur = nil
	defer func() {
		for _, d := range job.done {
			close(d) // after the graph is in (or none was found)
		}
	}()
	g := job.crawler.Graph()
	if g.SegmentCount() == 0 {
		k.logf("airways %s: none found (%d fixes asked, %d unknown)", job.icao, job.crawler.Requests(), len(job.crawler.Missing()))
		return
	}
	k.logf("airways %s: %d fixes, %d segments in %s", job.icao, len(g.Fixes), g.SegmentCount(), now.Sub(job.started).Round(time.Second))
	k.feed.Airways(job.icao, g)
	k.read[job.icao] = now
	if err := os.MkdirAll(k.dir, 0o755); err == nil {
		err = g.SaveJSON(k.path(job.icao))
		if err != nil {
			k.logf("airways %s: not kept: %v", job.icao, err)
		}
	}
}

// addAirways merges an airport's airways into the graph flight plans use.
func (st *state) addAirways(icao string, g *nav.AirwayGraph) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.airwaysBy == nil {
		st.airwaysBy = map[string]*nav.AirwayGraph{}
	}
	st.airwaysBy[icao] = g
	all := []*nav.AirwayGraph{st.airwaysGiven}
	for _, x := range st.airwaysBy {
		all = append(all, x)
	}
	st.airways = nav.MergeAirwayGraphs(all...)
	fmt.Fprintf(stdout, "🛣️  airways %s added: %d fixes in all\n", icao, len(st.airways.Fixes))
}

// The crawl's IDs (nav loader: definitions +0..2, requests +0..15): clear of
// the World's nav loader (8700/8800) and procedures (8400/8500).
const (
	airwayCrawlDefBase = 8600
	airwayCrawlReqBase = 8610
)
