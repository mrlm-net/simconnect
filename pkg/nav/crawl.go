//go:build windows
// +build windows

package nav

import (
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/engine"
)

// Crawl defaults.
const (
	DefaultCrawlRadiusNM    = 250.0
	DefaultCrawlMaxRequests = 3000
)

// CrawlOptions bound an AirwayCrawler: fixes farther than RadiusNM from
// Center are kept as airway ends but not requested, and at most
// MaxRequests fixes are requested.
type CrawlOptions struct {
	Center      airport.LatLon
	RadiusNM    float64
	MaxRequests int
}

// AirwayCrawler builds an AirwayGraph by following the airways from seed
// fixes breadth first: every fix it loads names its previous and next fix
// on each airway through it, and those within the radius are loaded next.
//
// Start it with the seeds, feed it every message with Handle and call Tick
// now and then (it expires unanswered requests); Done reports when the
// queue is empty, and Graph returns what was found.
type AirwayCrawler struct {
	loader *NavLoader
	opts   CrawlOptions

	queue    []FixKey
	seen     map[FixKey]bool
	requests int
	fixes    map[FixKey]Fix
	order    []FixKey
	links    map[FixKey][]RouteLink
	missing  []FixKey
}

// NewAirwayCrawler creates a crawler loading fixes through loader.
func NewAirwayCrawler(loader *NavLoader, opts CrawlOptions) *AirwayCrawler {
	if opts.RadiusNM <= 0 {
		opts.RadiusNM = DefaultCrawlRadiusNM
	}
	if opts.MaxRequests <= 0 {
		opts.MaxRequests = DefaultCrawlMaxRequests
	}
	return &AirwayCrawler{loader: loader, opts: opts, seen: map[FixKey]bool{}, fixes: map[FixKey]Fix{}, links: map[FixKey][]RouteLink{}}
}

// Start queues the seed fixes (whatever their distance) and sends the
// first requests.
func (c *AirwayCrawler) Start(seeds ...FixKey) error {
	for _, k := range seeds {
		c.enqueue(Key(k.Ident, k.Region, k.Kind))
	}
	return c.pump()
}

func (c *AirwayCrawler) enqueue(k FixKey) {
	if !c.seen[k] {
		c.seen[k] = true
		c.queue = append(c.queue, k)
	}
}

// pump sends queued requests while the loader has free slots.
func (c *AirwayCrawler) pump() error {
	for len(c.queue) > 0 && c.requests < c.opts.MaxRequests && c.loader.Free() > 0 {
		k := c.queue[0]
		c.queue = c.queue[1:]
		if err := c.loader.Request(k); err != nil {
			return err
		}
		c.requests++
	}
	return nil
}

// Handle feeds a message to the loader and follows the airways of the fix
// it completes. It returns true once the crawl is done.
func (c *AirwayCrawler) Handle(msg engine.Message) (bool, error) {
	if res, ok := c.loader.Handle(msg); ok {
		c.add(res)
		if err := c.pump(); err != nil {
			return false, err
		}
	}
	return c.Done(), nil
}

// Tick expires unanswered requests and keeps the crawl going; it returns
// true once the crawl is done.
func (c *AirwayCrawler) Tick(now time.Time) (bool, error) {
	for _, res := range c.loader.Expire(now) {
		c.add(res)
	}
	if err := c.pump(); err != nil {
		return false, err
	}
	return c.Done(), nil
}

// Done reports whether nothing is queued (or the request cap is reached)
// and nothing is loading.
func (c *AirwayCrawler) Done() bool {
	return (len(c.queue) == 0 || c.requests >= c.opts.MaxRequests) && c.loader.Pending() == 0
}

// Requests returns how many fixes were requested so far.
func (c *AirwayCrawler) Requests() int { return c.requests }

// Missing returns the requested fixes the simulator did not know.
func (c *AirwayCrawler) Missing() []FixKey { return c.missing }

func (c *AirwayCrawler) add(res NavResult) {
	if !res.Found {
		c.missing = append(c.missing, res.Key)
		return
	}
	k := res.Fix.Key()
	if _, ok := c.fixes[k]; !ok {
		c.order = append(c.order, k)
	}
	c.fixes[k] = res.Fix
	c.links[k] = res.Routes
	for _, rl := range res.Routes {
		for _, ref := range []*FixRef{rl.Prev, rl.Next} {
			if ref == nil || c.seen[ref.Key] {
				continue
			}
			if calc.HaversineNM(c.opts.Center.Lat, c.opts.Center.Lon, ref.Position.Lat, ref.Position.Lon) <= c.opts.RadiusNM {
				c.enqueue(ref.Key)
			}
		}
	}
}

// Graph builds the airway graph from the fixes loaded so far.
func (c *AirwayCrawler) Graph() *AirwayGraph {
	fixes := make([]Fix, 0, len(c.order))
	links := make(map[FixKey][]RouteLink, len(c.links))
	for _, k := range c.order {
		fixes = append(fixes, c.fixes[k])
		links[k] = c.links[k]
	}
	g := BuildAirwayGraph(fixes, links)
	g.Center, g.RadiusNM = c.opts.Center, c.opts.RadiusNM
	return g
}
