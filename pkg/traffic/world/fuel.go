package world

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
	// stairs (#831): GSX staircases (their base models), MSFS's own; gpus
	// (#832) likewise.
	stairs, stairsStock []string
	gpus, gpusStock     []string
	// buses (#887): GSX's apron buses (base models), MSFS's own.
	buses, busesStock []string
}

func (cc *controlCenter) requestFuelTitles() error {
	return cc.sim.ListGroundVehicles()
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
	var titles []string
	for i := uint32(0); i < n; i++ {
		entry := (*types.SIMCONNECT_ENUMERATE_SIMOBJECT_LIVERY)(unsafe.Pointer(base + uintptr(i*size)))
		titles = append(titles, engine.BytesToString(entry.AircraftTitle[:]))
	}
	cc.addGroundTitles(titles)
	if cc.onGroundTitles != nil {
		go cc.onGroundTitles(titles)
	}
}

// addGroundTitles keeps the fuel vehicles among ground vehicle titles.
func (cc *controlCenter) addGroundTitles(titles []string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	for _, t := range titles {
		switch {
		case strings.HasPrefix(t, "FSDT_FuelTruck_"):
			cc.fuelTitles.trucks = insertSorted(cc.fuelTitles.trucks, t)
		case strings.HasPrefix(t, "FSDT_Fuel_Hydrant_"):
			cc.fuelTitles.hydrants = insertSorted(cc.fuelTitles.hydrants, t)
		case strings.HasPrefix(t, "Fuel Truck Long"):
			cc.fuelTitles.stock = insertSorted(cc.fuelTitles.stock, t)
		case gsxStairs[t]:
			cc.fuelTitles.stairs = insertSorted(cc.fuelTitles.stairs, t)
		case t == "ASO_Boarding_Stairs":
			cc.fuelTitles.stairsStock = insertSorted(cc.fuelTitles.stairsStock, t)
		case t == "FSDT_GPU_TLD_406" || t == "FSDT_GPU_Hobart_4400":
			cc.fuelTitles.gpus = insertSorted(cc.fuelTitles.gpus, t)
		case t == "Car Ground Power Unit":
			cc.fuelTitles.gpusStock = insertSorted(cc.fuelTitles.gpusStock, t)
		case t == gsxBusEurope || t == gsxBusOther:
			cc.fuelTitles.buses = insertSorted(cc.fuelTitles.buses, t)
		case t == "Bus Apron 02":
			cc.fuelTitles.busesStock = insertSorted(cc.fuelTitles.busesStock, t)
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
	cc.giveATC(f, g.Layout, "fuel truck")
	return f
}

// gsxStairs are GSX's passenger staircases, their base models (each has
// some 630 liveries by handler; the cargo ones and the ladder left out).
var gsxStairs = map[string]bool{
	"FSDT_Staircase_CDS_2438": true, "FSDT_Staircase_CDS_2445": true, "FSDT_Staircase_TLD_ABS-580": true,
	"FSDT_Staircase_TLD_ABS-1740": true, "FSDT_Staircase_FW2458PE": true, "FSDT_Staircase_Aviramp_Continental": true,
}

// stairs are the boarding stairs of a departure (#831): an airliner (not
// GA or cargo) on a remote stand (not a gate), when the simulator has
// stairs; created with the third last request ID of the aircraft's block.
func (cc *controlCenter) stairs(r SpawnRequest, g *airport.Graph, reqBase uint32, prof traffic.MotionProfile) *traffic.SimObjectStairs {
	if r.Kind != "departure" || r.Circuit || r.StandUse != standAirline || r.Stand < 0 || r.Stand >= len(g.Layout.Parking) {
		return nil
	}
	stand := g.Layout.Parking[r.Stand]
	if stand.IsGate() {
		return nil // a jetway
	}
	cc.mu.Lock()
	list := cc.fuelTitles.stairs
	if len(list) == 0 {
		list = cc.fuelTitles.stairsStock
	}
	cc.mu.Unlock()
	if len(list) == 0 {
		return nil
	}
	h := fnv.New32a()
	h.Write([]byte(g.Layout.ICAO))
	title := list[int(h.Sum32()%uint32(len(list)))] // one handler's stairs per airport
	s := traffic.NewSimObjectStairs(cc.client, cc.inj, title, reqBase+controlIDBlock-3, prof)
	s.Layout = g.Layout
	cc.giveATC(s, g.Layout, "stairs")
	return s
}

// gpu is the ground power unit of a departure (#832): an airliner on a
// remote stand (a gate has its own power), when the simulator has one;
// created with the fourth last request ID of the aircraft's block.
func (cc *controlCenter) gpu(r SpawnRequest, g *airport.Graph, reqBase uint32, prof traffic.MotionProfile) *traffic.SimObjectFuelTruck {
	if r.Kind != "departure" || r.Circuit || r.StandUse != standAirline || r.Stand < 0 || r.Stand >= len(g.Layout.Parking) {
		return nil
	}
	if g.Layout.Parking[r.Stand].IsGate() {
		return nil
	}
	cc.mu.Lock()
	list := cc.fuelTitles.gpus
	if len(list) == 0 {
		list = cc.fuelTitles.gpusStock
	}
	cc.mu.Unlock()
	if len(list) == 0 {
		return nil
	}
	h := fnv.New32a()
	h.Write([]byte(g.Layout.ICAO))
	return newGPU(cc, list[int(h.Sum32()%uint32(len(list)))], g, reqBase, prof)
}

// newGPU is a GPU of title: a fuel-truck-driven vehicle parking at the nose.
func newGPU(cc *controlCenter, title string, g *airport.Graph, reqBase uint32, prof traffic.MotionProfile) *traffic.SimObjectFuelTruck {
	f := traffic.NewSimObjectFuelTruck(cc.client, cc.inj, title, reqBase+controlIDBlock-4, prof)
	f.Layout = g.Layout
	f.Spot = traffic.GPUSpot
	cc.giveATC(f, g.Layout, "GPU")
	return f
}

// GSX's apron buses (#887), base models (each has some 465 liveries by
// handler): the Cobus where GSX's rules_passengerbus.cfg puts it (ICAO
// regions E, L, K, C), the Neoplan in O, Z, V, W, U, either elsewhere. Live
// 2026-10-08 MSFS 2024 also offered its own "Bus Apron 02".
const (
	gsxBusEurope = "FSDT_Cobus_3000"
	gsxBusOther  = "FSDT_neoplan_bus"
)

// busTitle is the bus model for an airport: GSX's for its region, else
// either GSX bus, else MSFS's own; "" when the simulator offers none.
func (cc *controlCenter) busTitle(icao string) string {
	cc.mu.Lock()
	gsx, stock := cc.fuelTitles.buses, cc.fuelTitles.busesStock
	cc.mu.Unlock()
	want := gsxBusEurope
	if icao != "" && strings.ContainsRune("OZVWU", rune(icao[0])) {
		want = gsxBusOther
	}
	if i := sort.SearchStrings(gsx, want); i < len(gsx) && gsx[i] == want {
		return want
	}
	if len(gsx) > 0 {
		return gsx[0]
	}
	if len(stock) > 0 {
		return stock[0]
	}
	return ""
}

// buses are the passenger buses of a departure (#887), with its stairs
// only: BusesFor the aircraft, created with the sixth and fifth last
// request IDs of its block.
func (cc *controlCenter) buses(g *airport.Graph, reqBase uint32, prof traffic.MotionProfile, stairs traffic.FuelService) []traffic.FuelService {
	if stairs == nil {
		return nil
	}
	title := cc.busTitle(g.Layout.ICAO)
	if title == "" {
		return nil
	}
	var out []traffic.FuelService
	for i := range traffic.BusesFor(prof) {
		out = append(out, newBus(cc, cc.client, title, g, reqBase, prof, i))
	}
	return out
}

// newBus is bus n of a departure, of title: it parks beside the stairs
// (traffic.BusSpot).
func newBus(cc *controlCenter, client engine.Client, title string, g *airport.Graph, reqBase uint32, prof traffic.MotionProfile, n int) *traffic.SimObjectBus {
	b := traffic.NewSimObjectBus(client, cc.inj, title, reqBase+controlIDBlock-5-uint32(n), n, prof)
	b.Layout = g.Layout
	cc.giveATC(b, g.Layout, "bus")
	return b
}
