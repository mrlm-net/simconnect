//go:build windows
// +build windows

// Command glitch-watch traces every aircraft the airport map drives, every
// sim frame, and reports what looks wrong on screen: on the ground a jump
// in height (the shake as a tug disconnects), pitch, bank or heading, a
// position jump, wheels below the surface; in the air a jump in altitude or
// heading. Each glitch is printed once (then counted for 5 s) with the
// aircraft's state on the map, and the frames around it (2 s before, 1 s
// after) are written to glitch-<tail>-<time>.csv in -dir. Usage:
// glitch-watch [-map http://127.0.0.1:8080] [-dir .]. With -near meters it
// needs no map: it watches every aircraft and ground vehicle (tugs, GPUs,
// stairs) within that distance of the user aircraft, named by title, from
// the sim alone (a host whose API needs a login, as MyCrew's).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

type frame struct {
	Lat, Lon, Alt, Ground, Pitch, Bank, Hdg, OnGround, GS, StaticCG, SimTime float64
}

type sample struct {
	t time.Time
	f frame
}

// Thresholds per frame.
const (
	groundHeightFt = 0.25 // on the ground: CG height above the ground
	groundPitchDeg = 0.4
	groundBankDeg  = 0.4
	groundHdgDeg   = 2.0
	airAltFt       = 30.0
	airHdgDeg      = 5.0
	sunkFt         = 1.5 // CG this far below its static height: wheels in the ground
	repeatQuiet    = 5 * time.Second
	keepBefore     = 2 * time.Second
	keepAfter      = time.Second
)

type tracked struct {
	tail     string
	obj      uint32
	buf      []sample
	last     map[string]time.Time // kind → last reported
	counts   map[string]int
	pending  []*dump
	lastSeen time.Time
}

type dump struct {
	file  string
	until time.Time
	rows  []sample
}

func main() {
	mapURL := flag.String("map", "http://127.0.0.1:8080", "airport map")
	dir := flag.String("dir", ".", "where the glitch CSVs go")
	near := flag.Uint("near", 0, "watch every aircraft and ground vehicle within this many meters of the user aircraft (no map)")
	flag.Parse()
	client := simconnect.NewClient("glitch-watch")
	if err := client.Connect(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer client.Disconnect()
	const def = 7000
	for i, v := range []struct{ n, u string }{
		{"PLANE LATITUDE", "degrees"}, {"PLANE LONGITUDE", "degrees"}, {"PLANE ALTITUDE", "feet"}, {"GROUND ALTITUDE", "feet"},
		{"PLANE PITCH DEGREES", "degrees"}, {"PLANE BANK DEGREES", "degrees"}, {"PLANE HEADING DEGREES TRUE", "degrees"},
		{"SIM ON GROUND", "bool"}, {"GROUND VELOCITY", "knots"}, {"STATIC CG TO GROUND", "feet"}, {"SIMULATION TIME", "seconds"},
	} {
		client.AddToDataDefinition(def, v.n, v.u, types.SIMCONNECT_DATATYPE_FLOAT64, 0, uint32(i))
	}

	var mu sync.Mutex
	byReq := map[uint32]*tracked{} // request ID → aircraft
	byObj := map[uint32]uint32{}   // object → request ID
	states := map[string]string{}  // tail → state on the map
	nextReq := uint32(7001)
	totals := map[string]int{}

	// -near: the objects around the user aircraft, every 2 s (named by
	// title; the user aircraft left out), dropped 5 s after last seen.
	const defTitle, reqNearAir, reqNearGround = 7100, 6990, 6991
	client.AddToDataDefinition(defTitle, "TITLE", "", types.SIMCONNECT_DATATYPE_STRING256, 0, 0)
	seenNear := map[uint32]time.Time{}
	if *near > 0 {
		go func() {
			for {
				client.RequestDataOnSimObjectType(reqNearAir, defTitle, uint32(*near), types.SIMCONNECT_SIMOBJECT_TYPE_AIRCRAFT)
				client.RequestDataOnSimObjectType(reqNearGround, defTitle, uint32(*near), types.SIMCONNECT_SIMOBJECT_TYPE_GROUND)
				time.Sleep(2 * time.Second)
				mu.Lock()
				for obj, at := range seenNear {
					if time.Since(at) > 5*time.Second {
						if req, ok := byObj[obj]; ok {
							client.RequestDataOnSimObject(req, def, obj, types.SIMCONNECT_PERIOD_NEVER, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)
							delete(byObj, obj)
							delete(byReq, req)
						}
						delete(seenNear, obj)
					}
				}
				mu.Unlock()
			}
		}()
	}

	// The map's aircraft of ours and their states, every 2 s.
	go func() {
		for *near == 0 {
			ours, st := poll(*mapURL)
			mu.Lock()
			for tail, s := range st {
				states[tail] = s
			}
			seen := map[uint32]bool{}
			for obj, tail := range ours {
				seen[obj] = true
				if _, ok := byObj[obj]; ok {
					continue
				}
				req := nextReq
				nextReq++
				byObj[obj] = req
				byReq[req] = &tracked{tail: tail, obj: obj, last: map[string]time.Time{}, counts: map[string]int{}}
				client.RequestDataOnSimObject(req, def, obj, types.SIMCONNECT_PERIOD_SIM_FRAME, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)
			}
			for obj, req := range byObj {
				if !seen[obj] {
					client.RequestDataOnSimObject(req, def, obj, types.SIMCONNECT_PERIOD_NEVER, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)
					delete(byObj, obj)
					delete(byReq, req)
				}
			}
			mu.Unlock()
			time.Sleep(2 * time.Second)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	summary := time.NewTicker(10 * time.Minute)
	defer summary.Stop()
	report := func() {
		mu.Lock()
		defer mu.Unlock()
		kinds := make([]string, 0, len(totals))
		for k := range totals {
			kinds = append(kinds, k)
		}
		sort.Strings(kinds)
		var b strings.Builder
		for _, k := range kinds {
			fmt.Fprintf(&b, " %s=%d", k, totals[k])
		}
		fmt.Printf("%s  SUMMARY%s\n", time.Now().Format("15:04:05"), b.String())
	}
	if *near > 0 {
		fmt.Printf("%s  watching every aircraft and ground vehicle within %d m\n", time.Now().Format("15:04:05"), *near)
	} else {
		fmt.Printf("%s  watching the aircraft of %s\n", time.Now().Format("15:04:05"), *mapURL)
	}
	for {
		select {
		case <-stop:
			report()
			return
		case <-summary.C:
			report()
		case m, ok := <-client.Stream():
			if !ok {
				report()
				return
			}
			if m.SIMCONNECT_RECV != nil && types.SIMCONNECT_RECV_ID(m.DwID) == types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA_BYTYPE {
				d := m.AsSimObjectDataBType()
				if r := uint32(d.DwRequestID); r != reqNearAir && r != reqNearGround {
					continue
				}
				obj := uint32(d.DwObjectID)
				if obj == 1 {
					continue // the user aircraft
				}
				title := engine.BytesToString((*engine.CastDataAs[[256]byte](&d.DwData))[:])
				mu.Lock()
				seenNear[obj] = time.Now()
				if _, ok := byObj[obj]; !ok {
					req := nextReq
					nextReq++
					byObj[obj] = req
					byReq[req] = &tracked{tail: fmt.Sprintf("%s#%d", title, obj), obj: obj, last: map[string]time.Time{}, counts: map[string]int{}}
					client.RequestDataOnSimObject(req, def, obj, types.SIMCONNECT_PERIOD_SIM_FRAME, types.SIMCONNECT_DATA_REQUEST_FLAG_DEFAULT, 0, 0, 0)
				}
				mu.Unlock()
				continue
			}
			if m.SIMCONNECT_RECV == nil || types.SIMCONNECT_RECV_ID(m.DwID) != types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA {
				continue
			}
			d := m.AsSimObjectData()
			mu.Lock()
			a := byReq[uint32(d.DwRequestID)]
			if a == nil {
				mu.Unlock()
				continue
			}
			s := sample{time.Now(), *engine.CastDataAs[frame](&d.DwData)}
			for _, g := range a.check(s) {
				totals[g.kind]++
				a.counts[g.kind]++
				if t, ok := a.last[g.kind]; ok && s.t.Sub(t) < repeatQuiet {
					continue
				}
				a.last[g.kind] = s.t
				fmt.Printf("%s  %-14s %-8s %-16s %s\n", s.t.Format("15:04:05.000"), g.kind, a.tail, states[a.tail], g.detail)
				file := filepath.Join(*dir, fmt.Sprintf("glitch-%s-%s-%s.csv", a.tail, g.kind, s.t.Format("150405")))
				a.pending = append(a.pending, &dump{file: file, until: s.t.Add(keepAfter), rows: append([]sample(nil), a.buf...)})
			}
			a.push(s)
			mu.Unlock()
		}
	}
}

type glitch struct{ kind, detail string }

// check compares s with the frame before.
func (a *tracked) check(s sample) []glitch {
	if len(a.buf) == 0 {
		return nil
	}
	p := a.buf[len(a.buf)-1].f
	f := s.f
	var out []glitch
	dHdg := math.Abs(math.Mod(f.Hdg-p.Hdg+540, 360) - 180)
	dist := calc.HaversineMeters(p.Lat, p.Lon, f.Lat, f.Lon)
	dt := math.Max(f.SimTime-p.SimTime, 1.0/60) // the sim's clock (frames arrive in bursts); a frame with the same sim time is still a frame
	expect := math.Max(f.GS, p.GS)*0.5144*dt*2 + 1.5
	if dist > expect {
		out = append(out, glitch{"position-jump", fmt.Sprintf("%.1f m in %.0f ms at %.0f kt", dist, dt*1000, f.GS)})
	}
	if f.OnGround != 0 && p.OnGround != 0 {
		h, ph := f.Alt-f.Ground, p.Alt-p.Ground
		if d := h - ph; math.Abs(d) > groundHeightFt {
			out = append(out, glitch{"height-jump", fmt.Sprintf("%+.2f ft (%.2f → %.2f above the ground, static %.2f) at %.0f kt", d, ph, h, f.StaticCG, f.GS)})
		}
		if d := f.Pitch - p.Pitch; math.Abs(d) > groundPitchDeg {
			out = append(out, glitch{"pitch-jump", fmt.Sprintf("%+.2f° (%.2f → %.2f) at %.0f kt", d, p.Pitch, f.Pitch, f.GS)})
		}
		if d := f.Bank - p.Bank; math.Abs(d) > groundBankDeg {
			out = append(out, glitch{"bank-jump", fmt.Sprintf("%+.2f° at %.0f kt", d, f.GS)})
		}
		if dHdg > groundHdgDeg && f.GS < 40 {
			out = append(out, glitch{"heading-jump", fmt.Sprintf("%.1f° in a frame at %.0f kt", dHdg, f.GS)})
		}
		if f.StaticCG > 0 && h < f.StaticCG-sunkFt {
			out = append(out, glitch{"sunk", fmt.Sprintf("%.2f ft above the ground, static %.2f", h, f.StaticCG)})
		}
	}
	if f.OnGround == 0 && p.OnGround == 0 {
		if d := f.Alt - p.Alt; math.Abs(d) > airAltFt {
			out = append(out, glitch{"alt-jump", fmt.Sprintf("%+.0f ft in a frame", d)})
		}
		if dHdg > airHdgDeg {
			out = append(out, glitch{"air-heading-jump", fmt.Sprintf("%.1f° in a frame", dHdg)})
		}
	}
	return out
}

// push keeps s for the dumps and writes those complete.
func (a *tracked) push(s sample) {
	a.buf = append(a.buf, s)
	for len(a.buf) > 0 && s.t.Sub(a.buf[0].t) > keepBefore {
		a.buf = a.buf[1:]
	}
	kept := a.pending[:0]
	for _, d := range a.pending {
		d.rows = append(d.rows, s)
		if s.t.Before(d.until) {
			kept = append(kept, d)
			continue
		}
		write(d)
	}
	a.pending = kept
}

func write(d *dump) {
	var b strings.Builder
	b.WriteString("t,lat,lon,above_ground_ft,static_cg,pitch,bank,hdg,on_ground,gs\n")
	for _, r := range d.rows {
		f := r.f
		fmt.Fprintf(&b, "%s,%.7f,%.7f,%.2f,%.2f,%.2f,%.2f,%.2f,%.0f,%.1f\n", r.t.Format("15:04:05.000"), f.Lat, f.Lon, f.Alt-f.Ground, f.StaticCG, f.Pitch, f.Bank, f.Hdg, f.OnGround, f.GS)
	}
	_ = os.WriteFile(d.file, []byte(b.String()), 0o644)
}

// poll is the map's aircraft of ours (object → tail) and their states.
func poll(mapURL string) (map[uint32]string, map[string]string) {
	ours, states := map[uint32]string{}, map[string]string{}
	if r, err := http.Get(mapURL + "/api/traffic"); err == nil {
		var all []struct {
			Tail     string `json:"tail"`
			Ours     bool   `json:"ours"`
			ObjectID uint32 `json:"objectId"`
		}
		_ = json.NewDecoder(r.Body).Decode(&all)
		r.Body.Close()
		for _, a := range all {
			if a.Ours && a.ObjectID != 0 {
				ours[a.ObjectID] = a.Tail
			}
		}
	}
	if r, err := http.Get(mapURL + "/api/control"); err == nil {
		var views []struct {
			Tail  string `json:"tail"`
			State string `json:"state"`
		}
		_ = json.NewDecoder(r.Body).Decode(&views)
		r.Body.Close()
		for _, v := range views {
			states[v.Tail] = v.State
		}
	}
	return ours, states
}
