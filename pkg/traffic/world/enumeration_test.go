package world

import (
	"slices"
	"testing"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// enumeration is a SIMCONNECT_RECV_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST of
// titles, laid out as the simulator sends it.
func enumeration(titles ...string) engine.Message {
	header := int(unsafe.Sizeof(types.SIMCONNECT_RECV_LIST_TEMPLATE{}))
	size := int(unsafe.Sizeof(types.SIMCONNECT_ENUMERATE_SIMOBJECT_LIVERY{}))
	buf := make([]byte, header+len(titles)*size)
	l := (*types.SIMCONNECT_RECV_LIST_TEMPLATE)(unsafe.Pointer(&buf[0]))
	l.DwSize, l.DwID = types.DWORD(len(buf)), types.DWORD(types.SIMCONNECT_RECV_ID_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST)
	l.DwArraySize = types.DWORD(len(titles))
	for i, t := range titles {
		e := (*types.SIMCONNECT_ENUMERATE_SIMOBJECT_LIVERY)(unsafe.Pointer(&buf[header+i*size]))
		copy(e.AircraftTitle[:], t)
	}
	return engine.Message{SIMCONNECT_RECV: &l.SIMCONNECT_RECV, Size: uint32(len(buf))}
}

// TestEnumerationEntries: every entry of an enumeration read, past the
// first, with no pointer kept as a uintptr (under -race checkptr killed
// the process: "pointer arithmetic result points to invalid allocation").
func TestEnumerationEntries(t *testing.T) {
	cc := &controlCenter{models: map[string]bool{}}
	msg := enumeration("FSDT_FuelTruck_A", "FSDT_Pushback_B", "Fuel Truck Long", "Car Ground Power Unit")
	cc.addFuelTitles(msg)
	if !slices.Contains(cc.fuelTitles.trucks, "FSDT_FuelTruck_A") || !slices.Contains(cc.fuelTitles.tugs, "FSDT_Pushback_B") || len(cc.groundTitles) != 4 {
		t.Errorf("trucks %v, tugs %v, ground %v", cc.fuelTitles.trucks, cc.fuelTitles.tugs, cc.groundTitles)
	}
	cc.addModels(enumeration("A320 one", "B738 two", "E190 three"))
	if len(cc.models) != 3 || !cc.models["E190 three"] {
		t.Errorf("models %v", cc.models)
	}
}
