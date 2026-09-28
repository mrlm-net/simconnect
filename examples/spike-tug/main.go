//go:build windows
// +build windows

// Command spike-tug lists the ground vehicle sim objects the simulator
// offers and picks the pushback tugs (#304): the injected pushback can then
// spawn one and move it with the aircraft.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

func main() {
	filter := flag.String("filter", "tug,push,tow,tract,gsx,baggage", "comma-separated title substrings to show (empty = all)")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	ctx, stop := context.WithTimeout(ctx, 60*time.Second)
	defer stop()

	client := simconnect.NewClient("GO Spike - tug", engine.WithContext(ctx))
	for client.Connect() != nil {
		time.Sleep(2 * time.Second)
	}
	defer client.Disconnect()

	titles := map[string]bool{}
	total := 0
	if err := client.EnumerateSimObjectsAndLiveries(9500, types.SIMCONNECT_SIMOBJECT_TYPE_GROUND); err != nil {
		fmt.Println(err)
		return
	}
	stream := client.Stream()
	for {
		select {
		case <-ctx.Done():
			report(titles, total, *filter)
			return
		case msg := <-stream:
			if msg.SIMCONNECT_RECV == nil || types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST {
				continue
			}
			e := msg.AsSimObjectAndLiveryEnumeration()
			n := uint32(e.DwArraySize)
			if n == 0 {
				continue
			}
			const header = 32
			size := (uint32(msg.DwSize) - header) / n
			base := uintptr(unsafe.Pointer(e)) + header
			for i := uint32(0); i < n; i++ {
				entry := (*types.SIMCONNECT_ENUMERATE_SIMOBJECT_LIVERY)(unsafe.Pointer(base + uintptr(i*size)))
				titles[engine.BytesToString(entry.AircraftTitle[:])] = true
				total++
			}
			if uint32(e.DwEntryNumber)+1 >= uint32(e.DwOutOf) {
				report(titles, total, *filter)
				return
			}
		}
	}
}

func report(titles map[string]bool, total int, filter string) {
	var keys []string
	for t := range titles {
		keys = append(keys, t)
	}
	sort.Strings(keys)
	fmt.Printf("%d ground vehicle entries, %d titles\n", total, len(keys))
	words := strings.Split(strings.ToLower(filter), ",")
	for _, t := range keys {
		lt := strings.ToLower(t)
		show := filter == ""
		for _, w := range words {
			show = show || (w != "" && strings.Contains(lt, w))
		}
		if show {
			fmt.Println("  ", t)
		}
	}
}
