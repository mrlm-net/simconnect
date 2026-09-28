//go:build windows
// +build windows

// Command spike-procedures reads an airport's departures (SIDs), arrivals
// (STARs) and approaches with their transitions and legs from the facility
// API and prints them, to check the record layouts (#312).
package main

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"strings"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

var legFields = []string{"TYPE", "FIX_ICAO", "FIX_REGION", "FIX_TYPE", "FIX_LATITUDE", "FIX_LONGITUDE", "FIX_ALTITUDE",
	"FLY_OVER", "TURN_DIRECTION", "COURSE", "ROUTE_DISTANCE", "APPROACH_ALT_DESC", "ALTITUDE1", "ALTITUDE2", "SPEED_LIMIT",
	"ARC_CENTER_FIX_LATITUDE", "ARC_CENTER_FIX_LONGITUDE", "RHO", "IS_IAF", "IS_FAF", "IS_MAP"}

func legs(open string) []string {
	out := []string{"OPEN " + open}
	out = append(out, legFields...)
	return append(out, "CLOSE "+open)
}

func main() {
	icao := flag.String("icao", "LKPR", "airport")
	useLoader := flag.Bool("loader", false, "load with airport.ProcedureLoader instead of the raw dump")
	dump := flag.String("dump", "", "with -loader: write the procedures as JSON here")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	ctx, stop := context.WithTimeout(ctx, 30*time.Second)
	defer stop()
	client := simconnect.NewClient("GO Spike - procedures", engine.WithContext(ctx))
	for client.Connect() != nil {
		time.Sleep(2 * time.Second)
	}
	defer client.Disconnect()
	if *useLoader {
		viaLoader(ctx, client, *icao, *dump)
		return
	}

	proc := func(kind string) []string {
		d := []string{"OPEN AIRPORT", "OPEN " + kind, "NAME", "N_RUNWAY_TRANSITIONS", "N_ENROUTE_TRANSITIONS", "N_APPROACH_LEGS"}
		d = append(d, legs("APPROACH_LEG")...)
		d = append(d, "OPEN RUNWAY_TRANSITION", "RUNWAY_NUMBER", "RUNWAY_DESIGNATOR", "N_APPROACH_LEGS")
		d = append(d, legs("APPROACH_LEG")...)
		d = append(d, "CLOSE RUNWAY_TRANSITION", "OPEN ENROUTE_TRANSITION", "NAME", "N_APPROACH_LEGS")
		d = append(d, legs("APPROACH_LEG")...)
		return append(d, "CLOSE ENROUTE_TRANSITION", "CLOSE "+kind, "CLOSE AIRPORT")
	}
	appr := []string{"OPEN AIRPORT", "OPEN APPROACH", "TYPE", "SUFFIX", "RUNWAY_NUMBER", "RUNWAY_DESIGNATOR", "N_TRANSITIONS", "N_FINAL_APPROACH_LEGS", "N_MISSED_APPROACH_LEGS",
		"OPEN APPROACH_TRANSITION", "TYPE", "NAME", "N_APPROACH_LEGS"}
	appr = append(appr, legs("APPROACH_LEG")...)
	appr = append(appr, "CLOSE APPROACH_TRANSITION")
	appr = append(appr, legs("FINAL_APPROACH_LEG")...)
	appr = append(appr, legs("MISSED_APPROACH_LEG")...)
	appr = append(appr, "CLOSE APPROACH", "CLOSE AIRPORT")

	sent := map[uint32]string{}
	for i, def := range [][]string{proc("DEPARTURE"), proc("ARRIVAL"), appr} {
		for _, f := range def {
			client.AddToFacilityDefinition(uint32(300+i), f)
			if id, err := client.GetLastSentPacketID(); err == nil {
				sent[id] = f
			}
		}
		client.RequestFacilityData(uint32(300+i), uint32(400+i), *icao, "")
	}
	sizes := map[string]map[int]int{}
	shown := map[string]int{}
	done := 0
	for msg := range client.Stream() {
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
			name := fmt.Sprintf("req%d type%d", d.UserRequestId, d.Type)
			if sizes[name] == nil {
				sizes[name] = map[int]int{}
			}
			sizes[name][n]++
			if shown[name] < 4 {
				shown[name]++
				fmt.Printf("%s item %d parent %d unique %d size %d: %s\n", name, d.ItemIndex, d.ParentUniqueRequestId, d.UniqueRequestId, n, describe(d.Type, b))
			}
		case types.SIMCONNECT_RECV_ID_FACILITY_DATA_END:
			if done++; done == 3 {
				fmt.Println("record sizes:", sizes)
				return
			}
		}
	}
}

func describe(t types.SIMCONNECT_FACILITY_DATA_TYPE, b []byte) string {
	r := reader{b: b}
	switch t {
	case types.SIMCONNECT_FACILITY_DATA_DEPARTURE, types.SIMCONNECT_FACILITY_DATA_ARRIVAL:
		return fmt.Sprintf("name %q rwyTr %d enrTr %d legs %d", r.str(8), r.i32(), r.i32(), r.i32())
	case types.SIMCONNECT_FACILITY_DATA_RUNWAY_TRANSITION:
		return fmt.Sprintf("runway %d/%d legs %d", r.i32(), r.i32(), r.i32())
	case types.SIMCONNECT_FACILITY_DATA_ENROUTE_TRANSITION:
		return fmt.Sprintf("name %q legs %d", r.str(8), r.i32())
	case types.SIMCONNECT_FACILITY_DATA_APPROACH:
		return fmt.Sprintf("type %d suffix %d runway %d/%d trans %d final %d missed %d", r.i32(), r.i32(), r.i32(), r.i32(), r.i32(), r.i32(), r.i32())
	case types.SIMCONNECT_FACILITY_DATA_APPROACH_TRANSITION:
		return fmt.Sprintf("type %d name %q legs %d", r.i32(), r.str(8), r.i32())
	case types.SIMCONNECT_FACILITY_DATA_APPROACH_LEG, types.SIMCONNECT_FACILITY_DATA_FINAL_APPROACH_LEG, types.SIMCONNECT_FACILITY_DATA_MISSED_APPROACH_LEG:
		return fmt.Sprintf("leg type %d fix %q region %q fixtype %d at %.4f,%.4f alt %.0f flyover %d turn %d course %.1f dist %.1f altdesc %d alt1 %.0f alt2 %.0f spd %.0f arc %.4f,%.4f rho %.1f iaf %d faf %d map %d",
			r.i32(), r.str(8), r.str(8), r.i32(), r.f64(), r.f64(), r.f64(), r.i32(), r.i32(), r.f32(), r.f32(), r.i32(), r.f32(), r.f32(), r.f32(), r.f64(), r.f64(), r.f32(), r.i32(), r.i32(), r.i32())
	}
	return fmt.Sprintf("% x", b[:min(32, len(b))])
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
func (r *reader) i32() int32 { return int32(binary.LittleEndian.Uint32(r.take(4))) }
func (r *reader) f32() float32 {
	return math.Float32frombits(binary.LittleEndian.Uint32(r.take(4)))
}
func (r *reader) f64() float64 {
	return math.Float64frombits(binary.LittleEndian.Uint64(r.take(8)))
}
func (r *reader) str(n int) string { return strings.TrimRight(string(r.take(n)), "\x00") }
