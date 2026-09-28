//go:build windows
// +build windows

// Command spike-airlines checks what the facility API exposes about the
// airlines assigned to parking spots (#291): the N_AIRLINES count and
// whether the airline codes themselves can be read as a child record.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

type parkingWire struct {
	Name      int32
	Number    uint32
	NAirlines int32
}

func main() {
	icao := flag.String("icao", "LKPR", "airport ICAO")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	ctx, stop := context.WithTimeout(ctx, 30*time.Second)
	defer stop()

	client := simconnect.NewClient("GO Spike - airlines", engine.WithContext(ctx))
	for client.Connect() != nil {
		time.Sleep(2 * time.Second)
	}
	defer client.Disconnect()

	// Definition 1: the count. Definitions 2+: guesses at an airline child
	// record; an unknown name raises an exception for its field only.
	defs := [][]string{
		{"OPEN AIRPORT", "OPEN TAXI_PARKING", "NAME", "NUMBER", "N_AIRLINES", "CLOSE TAXI_PARKING", "CLOSE AIRPORT"},
		{"OPEN AIRPORT", "OPEN TAXI_PARKING", "OPEN AIRLINE", "NAME", "CLOSE AIRLINE", "CLOSE TAXI_PARKING", "CLOSE AIRPORT"},
		{"OPEN AIRPORT", "OPEN TAXI_PARKING", "OPEN TAXI_PARKING_AIRLINE", "CODE", "CLOSE TAXI_PARKING_AIRLINE", "CLOSE TAXI_PARKING", "CLOSE AIRPORT"},
		{"OPEN AIRPORT", "OPEN TAXI_PARKING", "AIRLINES", "CLOSE TAXI_PARKING", "CLOSE AIRPORT"},
	}
	sent := map[uint32]string{}
	for i, fields := range defs {
		for _, f := range fields {
			if err := client.AddToFacilityDefinition(uint32(100+i), f); err != nil {
				fmt.Printf("def %d %q: %v\n", i+1, f, err)
			}
			if id, err := client.GetLastSentPacketID(); err == nil {
				sent[id] = fmt.Sprintf("def %d field %q", i+1, f)
			}
		}
		if err := client.RequestFacilityData(uint32(100+i), uint32(200+i), *icao, ""); err != nil {
			fmt.Printf("def %d request: %v\n", i+1, err)
		}
		if id, err := client.GetLastSentPacketID(); err == nil {
			sent[id] = fmt.Sprintf("def %d request", i+1)
		}
	}

	withAirlines, parking := 0, 0
	other := map[string]int{}
	done := 0
	for msg := range client.Stream() {
		if msg.SIMCONNECT_RECV == nil {
			continue
		}
		switch types.SIMCONNECT_RECV_ID(msg.DwID) {
		case types.SIMCONNECT_RECV_ID_EXCEPTION:
			e := msg.AsException()
			fmt.Printf("exception %d at %s (index %d)\n", e.DwException, sent[uint32(e.DwSendID)], e.DwIndex)
		case types.SIMCONNECT_RECV_ID_FACILITY_DATA:
			d := msg.AsFacilityData()
			switch {
			case d.UserRequestId == 200 && d.Type == types.SIMCONNECT_FACILITY_DATA_TAXI_PARKING:
				p := engine.CastDataAs[parkingWire](&d.Data)
				parking++
				if p.NAirlines > 0 {
					withAirlines++
					if withAirlines <= 10 {
						fmt.Printf("parking %d (name %d number %d): %d airlines\n", d.ItemIndex, p.Name, p.Number, p.NAirlines)
					}
				}
			case d.UserRequestId > 200 && d.Type != types.SIMCONNECT_FACILITY_DATA_AIRPORT && d.Type != types.SIMCONNECT_FACILITY_DATA_TAXI_PARKING:
				key := fmt.Sprintf("def %d type %d", d.UserRequestId-199, d.Type)
				if other[key] == 0 {
					raw := engine.CastDataAs[[16]byte](&d.Data)
					fmt.Printf("%s first record: %q\n", key, engine.BytesToString(raw[:]))
				}
				other[key]++
			}
		case types.SIMCONNECT_RECV_ID_FACILITY_DATA_END:
			if done++; done == len(defs) {
				fmt.Printf("%d parking spots, %d with airlines; child records %v\n", parking, withAirlines, other)
				return
			}
		}
	}
}
