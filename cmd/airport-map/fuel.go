//go:build windows
// +build windows

package main

import (
	"hash/fnv"
	"sort"
	"strings"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/traffic"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// Fuel trucks (#582): the simulator's ground vehicles are enumerated once
// at connect, and the fuel vehicles kept: GSX fuel trucks and hydrant
// dispensers (FSDT_FuelTruck_*, FSDT_Fuel_Hydrant_*, one per fuel
// company), else MSFS's own fuel trucks. Live 2026-10-02 the simulator
// listed about 35,000 ground vehicle titles, some 500 of them GSX fuel
// trucks.
const reqGroundVehicles uint32 = 2005

// fuelTitles are the fuel vehicles the simulator offers.
type fuelTitles struct {
	trucks, hydrants, stock []string
}

func (cc *controlCenter) requestFuelTitles() error {
	return cc.client.EnumerateSimObjectsAndLiveries(reqGroundVehicles, types.SIMCONNECT_SIMOBJECT_TYPE_GROUND)
}

// addFuelTitles keeps the fuel vehicles of one enumeration message.
func (cc *controlCenter) addFuelTitles(msg engine.Message) {
	e := msg.AsSimObjectAndLiveryEnumeration()
	n := uint32(e.DwArraySize)
	header := uint32(unsafe.Sizeof(types.SIMCONNECT_RECV_LIST_TEMPLATE{}))
	size := uint32(unsafe.Sizeof(types.SIMCONNECT_ENUMERATE_SIMOBJECT_LIVERY{}))
	if n == 0 || n*size > uint32(msg.DwSize)-header {
		return
	}
	base := uintptr(unsafe.Pointer(e)) + uintptr(header)
	cc.mu.Lock()
	defer cc.mu.Unlock()
	for i := uint32(0); i < n; i++ {
		entry := (*types.SIMCONNECT_ENUMERATE_SIMOBJECT_LIVERY)(unsafe.Pointer(base + uintptr(i*size)))
		t := engine.BytesToString(entry.AircraftTitle[:])
		switch {
		case strings.HasPrefix(t, "FSDT_FuelTruck_"):
			cc.fuelTitles.trucks = insertSorted(cc.fuelTitles.trucks, t)
		case strings.HasPrefix(t, "FSDT_Fuel_Hydrant_"):
			cc.fuelTitles.hydrants = insertSorted(cc.fuelTitles.hydrants, t)
		case strings.HasPrefix(t, "Fuel Truck Long"):
			cc.fuelTitles.stock = insertSorted(cc.fuelTitles.stock, t)
		}
	}
}

func insertSorted(list []string, t string) []string {
	i := sort.SearchStrings(list, t)
	if i < len(list) && list[i] == t {
		return list
	}
	return append(list[:i], append([]string{t}, list[i:]...)...)
}

// fuelTitle is the fuel vehicle for a stand: a hydrant dispenser at a gate
// (fuel pits there), a fuel truck elsewhere, MSFS's own truck without GSX;
// "" when the simulator offers none. Each airport has its own two fuel
// companies (by its ICAO), the stands taking them in turn.
func (cc *controlCenter) fuelTitle(icao string, stand airport.Parking) string {
	cc.mu.Lock()
	ft := cc.fuelTitles
	cc.mu.Unlock()
	list := ft.trucks
	if stand.IsGate() && len(ft.hydrants) > 0 {
		list = ft.hydrants
	}
	if len(list) == 0 {
		list = ft.stock
	}
	if len(list) == 0 {
		return ""
	}
	h := fnv.New32a()
	h.Write([]byte(icao))
	k := int(h.Sum32()%uint32(len(list))) + int(stand.Number)%2
	return list[k%len(list)]
}

// fuelTruck is the fuel vehicle of a departure, if asked for (r.Fuel) and
// the simulator has one: created with the second last request ID of the
// aircraft's block (the tug has the last).
func (cc *controlCenter) fuelTruck(r SpawnRequest, g *airport.Graph, reqBase uint32, prof traffic.MotionProfile) *traffic.SimObjectFuelTruck {
	if !r.Fuel || r.Kind != "departure" || r.Stand < 0 || r.Stand >= len(g.Layout.Parking) {
		return nil
	}
	title := cc.fuelTitle(g.Layout.ICAO, g.Layout.Parking[r.Stand])
	if title == "" {
		return nil
	}
	f := traffic.NewSimObjectFuelTruck(cc.client, cc.inj, title, reqBase+controlIDBlock-2, prof)
	f.Layout = g.Layout
	return f
}
