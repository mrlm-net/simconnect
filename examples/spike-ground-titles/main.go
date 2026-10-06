//go:build windows
// +build windows

// Command spike-ground-titles lists the simulator's ground vehicle titles
// that match a pattern (read only): which stairs, GPUs, loaders and buses
// can be spawned for our traffic's ground services (#831, #832).
package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

func main() {
	pat := regexp.MustCompile(`(?i)` + os.Args[1])
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	client := simconnect.NewClient("spike-ground-titles", engine.WithContext(ctx))
	if err := client.Connect(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer client.Disconnect()
	if err := client.EnumerateSimObjectsAndLiveries(9901, types.SIMCONNECT_SIMOBJECT_TYPE_GROUND); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	seen := map[string]bool{}
	last := time.Now()
	for {
		select {
		case <-ctx.Done():
			goto done
		case m, ok := <-client.Stream():
			if !ok {
				goto done
			}
			if types.SIMCONNECT_RECV_ID(m.SIMCONNECT_RECV.DwID) != types.SIMCONNECT_RECV_ID_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST {
				if time.Since(last) > 5*time.Second && len(seen) > 0 {
					goto done
				}
				continue
			}
			last = time.Now()
			e := m.AsSimObjectAndLiveryEnumeration()
			n := uint32(e.DwArraySize)
			header := uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_LIST_TEMPLATE{}))
			size := uint32(unsafe.Sizeof(types.SIMCONNECT_ENUMERATE_SIMOBJECT_LIVERY{}))
			base := uintptr(unsafe.Pointer(e)) + uintptr(header)
			for i := uint32(0); i < n; i++ {
				entry := (*types.SIMCONNECT_ENUMERATE_SIMOBJECT_LIVERY)(unsafe.Pointer(base + uintptr(i*size)))
				t := engine.BytesToString(entry.AircraftTitle[:])
				if pat.MatchString(t) {
					seen[t] = true
				}
			}
			if e.DwEntryNumber+1 >= e.DwOutOf {
				goto done
			}
		}
	}
done:
	var out []string
	for t := range seen {
		out = append(out, t)
	}
	sort.Strings(out)
	for _, t := range out {
		fmt.Println(t)
	}
	fmt.Fprintln(os.Stderr, len(out), "titles")
}
