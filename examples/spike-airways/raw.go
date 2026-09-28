//go:build windows
// +build windows

package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// rawStages are the facility definitions the raw dump grows through, one
// stage at a time, so a field the simulator dislikes shows up alone.
var rawStages = map[int][]string{
	// Stage 1: the waypoint record alone.
	1: {"OPEN WAYPOINT", "LATITUDE", "LONGITUDE", "TYPE", "ICAO", "REGION", "N_ROUTES", "CLOSE WAYPOINT"},
	// Stage 2: with its airway (ROUTE) children.
	2: {"OPEN WAYPOINT", "LATITUDE", "LONGITUDE", "TYPE", "ICAO", "REGION", "N_ROUTES",
		"OPEN ROUTE", "NAME", "TYPE", "NEXT_ICAO", "NEXT_REGION", "NEXT_TYPE", "NEXT_LATITUDE", "NEXT_LONGITUDE", "NEXT_ALTITUDE",
		"PREV_ICAO", "PREV_REGION", "PREV_TYPE", "PREV_LATITUDE", "PREV_LONGITUDE", "PREV_ALTITUDE", "CLOSE ROUTE", "CLOSE WAYPOINT"},
	// Stage 3: VOR frequency and name.
	3: {"OPEN VOR", "VOR_LATITUDE", "VOR_LONGITUDE", "FREQUENCY", "NAME", "CLOSE VOR"},
	// Stage 4: NDB frequency and name.
	4: {"OPEN NDB", "LATITUDE", "LONGITUDE", "FREQUENCY", "NAME", "CLOSE NDB"},
}

type rawTarget struct {
	ident, region string
	kind          byte
}

// rawDump registers one stage's definition, requests each target and
// prints every record and exception it gets back.
func rawDump(ctx context.Context, client engine.Client, stage int, targets []rawTarget) {
	def, ok := rawStages[stage]
	if !ok {
		fmt.Println("unknown stage", stage)
		return
	}
	sent := map[uint32]string{}
	for _, f := range def {
		if err := client.AddToFacilityDefinition(600, f); err != nil {
			fmt.Println("add", f, err)
			return
		}
		if id, err := client.GetLastSentPacketID(); err == nil {
			sent[id] = f
		}
	}
	for i, t := range targets {
		if err := client.RequestFacilityDataEX1(600, uint32(700+i), t.ident, t.region, t.kind); err != nil {
			fmt.Println("request", t.ident, err)
			return
		}
		if id, err := client.GetLastSentPacketID(); err == nil {
			sent[id] = "request " + t.ident
		}
	}
	done := 0
	for {
		select {
		case <-ctx.Done():
			fmt.Println("timed out")
			return
		case msg := <-client.Stream():
			if msg.SIMCONNECT_RECV == nil {
				continue
			}
			switch types.SIMCONNECT_RECV_ID(msg.DwID) {
			case types.SIMCONNECT_RECV_ID_EXCEPTION:
				e := msg.AsException()
				fmt.Printf("exception %d at %q\n", e.DwException, sent[uint32(e.DwSendID)])
			case types.SIMCONNECT_RECV_ID_FACILITY_DATA:
				d := msg.AsFacilityData()
				n := int(d.DwSize) - int(unsafe.Offsetof(d.Data))
				b := unsafe.Slice((*byte)(unsafe.Pointer(&d.Data)), n)
				fmt.Printf("req %d type %d item %d parent %d unique %d size %d: %s\n", d.UserRequestId, d.Type, d.ItemIndex,
					d.ParentUniqueRequestId, d.UniqueRequestId, n, describe(d.Type, b))
			case types.SIMCONNECT_RECV_ID_FACILITY_DATA_END:
				fmt.Println("end", msg.AsFacilityDataEnd().RequestId)
				if done++; done == len(targets) {
					return
				}
			case types.SIMCONNECT_RECV_ID_FACILITY_MINIMAL_LIST:
				fmt.Println("minimal list (ambiguous ident)")
			}
		}
	}
}

func describe(t types.SIMCONNECT_FACILITY_DATA_TYPE, b []byte) string {
	r := reader{b: b}
	switch t {
	case types.SIMCONNECT_FACILITY_DATA_WAYPOINT:
		return fmt.Sprintf("wpt %.5f,%.5f type %d icao %q region %q routes %d", r.f64(), r.f64(), r.i32(), r.str(8), r.str(8), r.i32())
	case types.SIMCONNECT_FACILITY_DATA_ROUTE:
		return fmt.Sprintf("route %q type %d next %q %q %d %.4f,%.4f alt %.0f prev %q %q %d %.4f,%.4f alt %.0f",
			r.str(32), r.i32(), r.str(8), r.str(8), r.i32(), r.f64(), r.f64(), r.f32(),
			r.str(8), r.str(8), r.i32(), r.f64(), r.f64(), r.f32())
	case types.SIMCONNECT_FACILITY_DATA_VOR:
		return fmt.Sprintf("vor %.5f,%.5f freq %d name %q", r.f64(), r.f64(), r.u32(), r.str(64))
	case types.SIMCONNECT_FACILITY_DATA_NDB:
		return fmt.Sprintf("ndb %.5f,%.5f freq %d name %q", r.f64(), r.f64(), r.u32(), r.str(64))
	}
	return fmt.Sprintf("% x", b[:min(48, len(b))])
}

type reader struct {
	b   []byte
	off int
}

func (r *reader) take(n int) []byte {
	if r.off+n > len(r.b) {
		r.off = len(r.b)
		return make([]byte, n)
	}
	s := r.b[r.off : r.off+n]
	r.off += n
	return s
}
func (r *reader) i32() int32  { return int32(binary.LittleEndian.Uint32(r.take(4))) }
func (r *reader) u32() uint32 { return binary.LittleEndian.Uint32(r.take(4)) }
func (r *reader) f32() float32 {
	return math.Float32frombits(binary.LittleEndian.Uint32(r.take(4)))
}
func (r *reader) f64() float64 {
	return math.Float64frombits(binary.LittleEndian.Uint64(r.take(8)))
}
func (r *reader) str(n int) string {
	s := string(r.take(n))
	if i := strings.IndexByte(s, 0); i >= 0 {
		s = s[:i]
	}
	return s
}
