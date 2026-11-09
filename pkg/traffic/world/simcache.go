package world

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// What the simulator was asked for at every connect, built on its main
// thread each time though it rarely changes, is kept on disk in the
// data directory (Options.DataDir, -dump-dir), simcache/, and read from
// there while fresh: the worldwide airport list, the aircraft and ground
// vehicle titles, and each airport's layout and procedures (the same
// files as Options.AirportDumpDir: <ICAO>/layout.json, procedures.json,
// so the folder also serves LoadAirportDir). The titles, layouts and
// procedures are kept for one set of installed packages (the Community
// and Official folders' stamp, packageStamp): a package added, removed or
// updated asks the simulator again. ?refresh on /api/airport always does.
const (
	simCacheDir = "simcache"
	// simCacheMaxAge: older, asked of the simulator again.
	simCacheMaxAge = 7 * 24 * time.Hour
	simCacheModels = "models.json"
	simCacheStamp  = "stamp.txt"
)

// simCache is the disk cache; nil (no data directory) keeps nothing.
type simCache struct {
	dir   string
	stamp string // the packages' stamp now ("" unknown: titles not kept)
	ok    bool   // the files were written for this stamp
	now   func() time.Time
	// enumerations finished, of reqModels and reqGroundVehicles.
	modelsDone, groundDone bool
	pending                bool      // both done, not yet kept
	stampAt                time.Time // when the stamp was written: packaged files older are not read
}

// newSimCache opens the cache under dataDir for packages stamped stamp.
func newSimCache(dataDir, stamp string) *simCache {
	if dataDir == "" {
		return nil
	}
	c := &simCache{dir: filepath.Join(dataDir, simCacheDir), stamp: stamp, now: time.Now}
	b, err := os.ReadFile(filepath.Join(c.dir, simCacheStamp))
	c.ok = err == nil && stamp != "" && strings.TrimSpace(string(b)) == stamp
	if fi, err := os.Stat(filepath.Join(c.dir, simCacheStamp)); c.ok && err == nil {
		c.stampAt = fi.ModTime()
	}
	return c
}

// path is name's file, icao's when given.
func (c *simCache) path(icao, name string) string {
	if icao != "" {
		return filepath.Join(c.dir, strings.ToUpper(icao), name)
	}
	return filepath.Join(c.dir, name)
}

// read decodes name into v when it is fresh; packaged: only for this
// stamp.
func (c *simCache) read(icao, name string, packaged bool, v any) bool {
	if c == nil || packaged && !c.ok {
		return false
	}
	p := c.path(icao, name)
	fi, err := os.Stat(p)
	if err != nil || c.now().Sub(fi.ModTime()) > simCacheMaxAge || packaged && fi.ModTime().Before(c.stampAt) {
		return false
	}
	b, err := os.ReadFile(p)
	return err == nil && json.Unmarshal(b, v) == nil
}

// write keeps v as name; packaged: stamped with the packages first (a
// stamp changed: what was kept for the old one is not read).
func (c *simCache) write(icao, name string, packaged bool, v any) {
	if c == nil {
		return
	}
	if packaged && !c.ok {
		if c.stamp == "" {
			return
		}
		if err := os.MkdirAll(c.dir, 0o755); err != nil {
			return
		}
		// Older packaged files stay until rewritten, unread: their mtime
		// is before the stamp's.
		sp := filepath.Join(c.dir, simCacheStamp)
		if err := os.WriteFile(sp, []byte(c.stamp), 0o644); err != nil {
			return
		}
		fi, err := os.Stat(sp)
		if err != nil {
			return
		}
		c.ok, c.stampAt = true, fi.ModTime()
	}
	p := c.path(icao, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return
	}
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	tmp := p + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		os.Rename(tmp, p)
	}
}

// airports is the worldwide airport list, when kept.
func (c *simCache) airports() (map[string]traffic.AirportRef, bool) {
	var list []traffic.AirportRef
	if !c.read("", dumpAirportList, false, &list) || len(list) == 0 {
		return nil, false
	}
	return airportRefs(list), true
}

func airportRefs(list []traffic.AirportRef) map[string]traffic.AirportRef {
	refs := make(map[string]traffic.AirportRef, len(list))
	for _, a := range list {
		refs[strings.ToUpper(a.ICAO)] = a
	}
	return refs
}

func (c *simCache) writeAirports(list []traffic.AirportRef) {
	if c == nil || len(list) == 0 {
		return
	}
	out := make([]traffic.AirportRef, len(list))
	copy(out, list)
	for i := range out {
		out[i].ICAO, out[i].DistanceNM = strings.ToUpper(out[i].ICAO), 0
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ICAO < out[j].ICAO })
	c.write("", dumpAirportList, false, out)
}

// simModels is the titles kept: aircraft (with their liveries) and ground
// vehicles.
type simModels struct {
	Aircraft []string `json:"aircraft"`
	Ground   []string `json:"ground"`
}

func (c *simCache) models() (simModels, bool) {
	var m simModels
	ok := c.read("", simCacheModels, true, &m) && len(m.Aircraft) > 0
	return m, ok
}

// noteEnumeration marks a title enumeration's last message; both done, the
// titles cc gathered are kept. Before cc handles msg: its titles are in cc
// once the next message comes, so the last is kept with cc's own.
func (c *simCache) noteEnumeration(msg engine.Message) {
	if c == nil || types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST {
		return
	}
	e := msg.AsSimObjectAndLiveryEnumeration()
	if uint32(e.DwEntryNumber)+1 < uint32(e.DwOutOf) {
		return
	}
	switch uint32(e.DwRequestID) {
	case reqModels:
		c.modelsDone = true
	case reqGroundVehicles:
		c.groundDone = true
	default:
		return
	}
	c.pending = c.modelsDone && c.groundDone
}

// flushModels keeps cc's titles once both enumerations are in (after cc
// handled the last message).
func (c *simCache) flushModels(cc *controlCenter) {
	if c == nil || !c.pending {
		return
	}
	c.pending = false
	m := simModels{Aircraft: cc.modelList()}
	cc.mu.Lock()
	m.Ground = append([]string(nil), cc.groundTitles...)
	cc.mu.Unlock()
	if len(m.Aircraft) > 0 {
		c.write("", simCacheModels, true, m)
	}
}

// feedModels hands cc the kept titles, as the enumerations would.
func (cc *controlCenter) feedModels(m simModels) {
	cc.mu.Lock()
	for _, t := range m.Aircraft {
		cc.models[t] = true
	}
	cc.mu.Unlock()
	if cc.onModels != nil && len(m.Aircraft) > 0 {
		go cc.onModels(m.Aircraft)
	}
	if len(m.Ground) > 0 {
		cc.addGroundTitles(m.Ground)
		if cc.onGroundTitles != nil {
			go cc.onGroundTitles(m.Ground)
		}
	}
}

// layout is icao's kept layout, built.
func (c *simCache) layout(icao string) (*airport.Layout, airport.RawAirport, bool) {
	var raw airport.RawAirport
	if !c.read(icao, dumpLayout, true, &raw) {
		return nil, raw, false
	}
	l, err := airport.BuildLayout(raw)
	if err != nil {
		return nil, raw, false
	}
	return l, raw, true
}

func (c *simCache) writeLayout(raw airport.RawAirport) {
	if raw.ICAO != "" {
		c.write(raw.ICAO, dumpLayout, true, raw)
	}
}

// procedures is icao's kept SIDs, STARs and approaches.
func (c *simCache) procedures(icao string) (airport.Procedures, bool) {
	var p airport.Procedures
	ok := c.read(icao, dumpProcedures, true, &p) && strings.EqualFold(p.ICAO, icao)
	return p, ok
}

// writeProcedures keeps p; none at all is not kept (a load that failed
// looks the same).
func (c *simCache) writeProcedures(p airport.Procedures) {
	if p.ICAO != "" && len(p.Departures)+len(p.Arrivals)+len(p.Approaches) > 0 {
		c.write(p.ICAO, dumpProcedures, true, p)
	}
}

// takeRefresh reports whether icao was asked for with ?refresh, and
// forgets it.
func (s *state) takeRefresh(icao string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	icao = strings.ToUpper(icao)
	r := s.refreshing[icao]
	delete(s.refreshing, icao)
	return r
}

// markRefresh records icao asked for with ?refresh: the disk cache not
// read for it.
func (s *state) markRefresh(icao string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refreshing == nil {
		s.refreshing = map[string]bool{}
	}
	s.refreshing[strings.ToUpper(icao)] = true
}
