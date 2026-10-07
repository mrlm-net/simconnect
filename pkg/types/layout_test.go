package types

import (
	"encoding/binary"
	"math"
	"testing"
	"unsafe"
)

// The RECV structs are cast onto SimConnect's messages, so their Go layout
// must be the packed layout of SimConnect.h (MSFS 2024 SDK): sizes and
// offsets below are counted from the header by hand.
func TestRecvLayout(t *testing.T) {
	var (
		open   SIMCONNECT_RECV_OPEN
		frame  SIMCONNECT_RECV_EVENT_FRAME
		fname  SIMCONNECT_RECV_EVENT_FILENAME
		state  SIMCONNECT_RECV_SYSTEM_STATE
		lt     SIMCONNECT_RECV_LIST_TEMPLATE
		fl     SIMCONNECT_RECV_FACILITIES_LIST
		min    SIMCONNECT_RECV_FACILITY_MINIMAL_LIST
		inputs SIMCONNECT_RECV_ENUMERATE_INPUT_EVENTS
		desc   SIMCONNECT_INPUT_EVENT_DESCRIPTOR
		ctl    SIMCONNECT_RECV_CONTROLLERS_LIST
		item   SIMCONNECT_CONTROLLER_ITEM
		liv    SIMCONNECT_RECV_ENUMERATE_SIMOBJECT_AND_LIVERY_LIST
		get    SIMCONNECT_RECV_GET_INPUT_EVENT
		sub    SIMCONNECT_RECV_SUBSCRIBE_INPUT_EVENT
		fd     SIMCONNECT_RECV_FACILITY_DATA
		obj    SIMCONNECT_RECV_SIMOBJECT_DATA
		exc    SIMCONNECT_RECV_EXCEPTION
		race   SIMCONNECT_RECV_EVENT_RACE_END
		cam    SIMCONNECT_RECV_CAMERA_STATUS
		cb     SIMCONNECT_RECV_COMM_BUS
	)
	for _, c := range []struct {
		name      string
		got, want uintptr
	}{
		{"RECV", unsafe.Sizeof(SIMCONNECT_RECV{}), 12},
		{"OPEN.szApplicationName", unsafe.Offsetof(open.SzApplicationName), 12},
		{"OPEN.dwApplicationVersionMajor", unsafe.Offsetof(open.DwApplicationVersionMajor), 268},
		{"OPEN.dwSimConnectVersionMajor", unsafe.Offsetof(open.DwSimConnectVersionMajor), 284},
		{"OPEN", unsafe.Sizeof(open), 308},
		{"EXCEPTION", unsafe.Sizeof(exc), 24},
		{"EVENT", unsafe.Sizeof(SIMCONNECT_RECV_EVENT{}), 24},
		{"EVENT_EX1", unsafe.Sizeof(SIMCONNECT_RECV_EVENT_EX1{}), 40},
		{"EVENT_FRAME.fFrameRate", unsafe.Offsetof(frame.FFrameRate), 24},
		{"EVENT_FRAME.fSimSpeed", unsafe.Offsetof(frame.FSimSpeed), 28},
		{"EVENT_FRAME", unsafe.Sizeof(frame), 32},
		{"EVENT_FILENAME.dwFlags", unsafe.Offsetof(fname.DwFlags), 284},
		{"EVENT_OBJECT_ADDREMOVE", unsafe.Sizeof(SIMCONNECT_RECV_EVENT_OBJECT_ADDREMOVE{}), 28},
		{"EVENT_RACE_END.RacerData", unsafe.Offsetof(race.RacerData), 28},
		{"DATA_RACE_RESULT", unsafe.Sizeof(SIMCONNECT_DATA_RACE_RESULT{}), 1080},
		{"SIMOBJECT_DATA.dwData", unsafe.Offsetof(obj.DwData), 40},
		{"SYSTEM_STATE.szString", unsafe.Offsetof(state.SzString), 24},
		{"ASSIGNED_OBJECT_ID", unsafe.Sizeof(SIMCONNECT_RECV_ASSIGNED_OBJECT_ID{}), 20},
		{"RESERVED_KEY", unsafe.Sizeof(SIMCONNECT_RECV_RESERVED_KEY{}), 92},
		{"LIST_TEMPLATE", unsafe.Sizeof(lt), 28},
		{"FACILITIES_LIST", unsafe.Sizeof(fl), 28},
		{"FACILITY_MINIMAL_LIST", unsafe.Sizeof(min), 28},
		{"AIRPORT_LIST", unsafe.Sizeof(SIMCONNECT_RECV_AIRPORT_LIST{}), 28},
		{"WAYPOINT_LIST", unsafe.Sizeof(SIMCONNECT_RECV_WAYPOINT_LIST{}), 28},
		{"NDB_LIST", unsafe.Sizeof(SIMCONNECT_RECV_NDB_LIST{}), 28},
		{"VOR_LIST", unsafe.Sizeof(SIMCONNECT_RECV_VOR_LIST{}), 28},
		{"JETWAY_DATA", unsafe.Sizeof(SIMCONNECT_RECV_JETWAY_DATA{}), 28},
		{"FACILITY_DATA.Data", unsafe.Offsetof(fd.Data), 40},
		{"FACILITY_DATA_END", unsafe.Sizeof(SIMCONNECT_RECV_FACILITY_DATA_END{}), 16},
		{"ICAO", unsafe.Sizeof(SIMCONNECT_ICAO{}), 18},
		{"ENUMERATE_INPUT_EVENTS.rgData", unsafe.Offsetof(inputs.RgData), 28},
		{"INPUT_EVENT_DESCRIPTOR.Hash", unsafe.Offsetof(desc.HashBytes), 64},
		{"INPUT_EVENT_DESCRIPTOR.eType", unsafe.Offsetof(desc.Type), 72},
		{"INPUT_EVENT_DESCRIPTOR", unsafe.Sizeof(desc), InputEventDescriptorSize},
		{"GET_INPUT_EVENT.Value", unsafe.Offsetof(get.Value), 20},
		{"SUBSCRIBE_INPUT_EVENT.Hash", unsafe.Offsetof(sub.HashBytes), 12},
		{"SUBSCRIBE_INPUT_EVENT.Value", unsafe.Offsetof(sub.Value), 24},
		{"CONTROLLERS_LIST.rgData", unsafe.Offsetof(ctl.RgData), 28},
		{"CONTROLLER_ITEM.HardwareVersion", unsafe.Offsetof(item.HardwareVersion), 268},
		{"CONTROLLER_ITEM", unsafe.Sizeof(item), ControllerItemSize},
		{"ENUMERATE_SIMOBJECT_AND_LIVERY_LIST.rgData", unsafe.Offsetof(liv.RgData), 28},
		{"ENUMERATE_SIMOBJECT_LIVERY", unsafe.Sizeof(SIMCONNECT_ENUMERATE_SIMOBJECT_LIVERY{}), SimObjectLiverySize},
		{"FLOW_EVENT", unsafe.Sizeof(SIMCONNECT_RECV_FLOW_EVENT{}), 272},
		{"CAMERA_STATUS.bGameControlled", unsafe.Offsetof(cam.GameControlled), 16},
		{"CAMERA_WORLD_LOCKER", unsafe.Sizeof(SIMCONNECT_RECV_CAMERA_WORLD_LOCKER{}), 16},
		{"COMM_BUS.uEventID", unsafe.Offsetof(cb.UEventID), 28},
		{"DATA_PBH", unsafe.Sizeof(SIMCONNECT_DATA_PBH{}), 12},
		{"DATA_INITPOSITION", unsafe.Sizeof(SIMCONNECT_DATA_INITPOSITION{}), 56},
		{"DATA_LATLONALT", unsafe.Sizeof(SIMCONNECT_DATA_LATLONALT{}), 24},
	} {
		if c.got != c.want {
			t.Errorf("%s: %d, SimConnect.h %d", c.name, c.got, c.want)
		}
	}
}

func TestInitPositionAirspeedConstants(t *testing.T) {
	if uint32(SIMCONNECT_DATA_INITPOSITION_AIRSPEED_CRUISE) != math.MaxUint32 || uint32(SIMCONNECT_DATA_INITPOSITION_AIRSPEED_KEEP) != math.MaxUint32-1 {
		t.Error("INITPOSITION_AIRSPEED constants are not the DWORDs -1 and -2")
	}
}

// listMessage builds a list message: the 28-byte header, then the entries.
func listMessage(id SIMCONNECT_RECV_ID, n int, entries []byte) []byte {
	b := make([]byte, listHeaderSize+len(entries))
	le := binary.LittleEndian
	le.PutUint32(b[0:], uint32(len(b)))
	le.PutUint32(b[8:], uint32(id))
	le.PutUint32(b[16:], uint32(n))
	copy(b[listHeaderSize:], entries)
	return b
}

func TestAirportListEntries(t *testing.T) {
	le := binary.LittleEndian
	e := make([]byte, 2*FacilityAirportSize)
	for i, icao := range []string{"LKPR", "LKTB"} {
		o := i * FacilityAirportSize
		copy(e[o:], icao)
		copy(e[o+9:], "LK")
		le.PutUint64(e[o+12:], math.Float64bits(50+float64(i)))
		le.PutUint64(e[o+20:], math.Float64bits(14))
		le.PutUint64(e[o+28:], math.Float64bits(380))
	}
	b := listMessage(SIMCONNECT_RECV_ID_AIRPORT_LIST, 2, e)
	got := (*SIMCONNECT_RECV_AIRPORT_LIST)(unsafe.Pointer(&b[0])).Entries()
	if len(got) != 2 || string(got[1].Ident[:4]) != "LKTB" || string(got[1].Region[:2]) != "LK" || got[1].Latitude != 51 || got[0].Altitude != 380 {
		t.Fatalf("entries %+v", got)
	}
	// A count beyond the message: only what it holds.
	le.PutUint32(b[16:], 5)
	if n := len((*SIMCONNECT_RECV_AIRPORT_LIST)(unsafe.Pointer(&b[0])).Entries()); n != 2 {
		t.Errorf("%d entries from a 2-entry message", n)
	}
}

func TestVORDecode(t *testing.T) {
	le := binary.LittleEndian
	e := make([]byte, FacilityVORSize)
	copy(e, "OKL")
	le.PutUint32(e[36:], math.Float32bits(4.5))
	le.PutUint32(e[40:], 112600000)
	le.PutUint32(e[44:], 3)
	le.PutUint32(e[48:], math.Float32bits(243.5))
	le.PutUint64(e[52:], math.Float64bits(50.1))
	le.PutUint64(e[68:], math.Float64bits(370))
	le.PutUint32(e[76:], math.Float32bits(3))
	b := listMessage(SIMCONNECT_RECV_ID_VOR_LIST, 1, e)
	got := (*SIMCONNECT_RECV_VOR_LIST)(unsafe.Pointer(&b[0])).Entries()
	if len(got) != 1 {
		t.Fatalf("%d entries", len(got))
	}
	v := got[0]
	if string(v.Ident[:3]) != "OKL" || v.FMagVar != 4.5 || v.FFrequency != 112600000 || v.Flags != 3 || v.FLocalizer != 243.5 ||
		v.GlideLat != 50.1 || v.GlideAlt != 370 || v.FGlideSlopeAngle != 3 {
		t.Errorf("decoded %+v", v)
	}
}

func TestFacilityMinimalDecode(t *testing.T) {
	e := make([]byte, 2*FacilityMinimalSize)
	o := FacilityMinimalSize
	e[o] = 'V'
	copy(e[o+1:], "OKL")
	copy(e[o+10:], "LK")
	copy(e[o+13:], "LKPR")
	binary.LittleEndian.PutUint64(e[o+26:], math.Float64bits(14.2))
	b := listMessage(SIMCONNECT_RECV_ID_FACILITY_MINIMAL_LIST, 2, e)
	got := (*SIMCONNECT_RECV_FACILITY_MINIMAL_LIST)(unsafe.Pointer(&b[0])).Entries()
	if len(got) != 2 || got[1].ICAO.Type != 'V' || string(got[1].ICAO.Airport[:4]) != "LKPR" || got[1].LLA.Longitude != 14.2 {
		t.Errorf("entries %+v", got)
	}
}

func TestInputEventEntries(t *testing.T) {
	e := make([]byte, 3*InputEventDescriptorSize)
	for i := 0; i < 3; i++ {
		o := i * InputEventDescriptorSize
		copy(e[o:], "EVENT")
		e[o+5] = byte('A' + i)
		binary.LittleEndian.PutUint64(e[o+64:], 0x1122334455667700+uint64(i))
		binary.LittleEndian.PutUint32(e[o+72:], uint32(SIMCONNECT_INPUT_EVENT_TYPE_DOUBLE))
	}
	b := listMessage(SIMCONNECT_RECV_ID_ENUMERATE_INPUT_EVENTS, 3, e)
	got := (*SIMCONNECT_RECV_ENUMERATE_INPUT_EVENTS)(unsafe.Pointer(&b[0])).Entries()
	if len(got) != 3 {
		t.Fatalf("%d entries", len(got))
	}
	for i, d := range got {
		if d.Hash() != 0x1122334455667700+uint64(i) || d.Type != SIMCONNECT_INPUT_EVENT_TYPE_DOUBLE || d.Name[5] != byte('A'+i) {
			t.Errorf("entry %d: hash %x type %d name %q", i, d.Hash(), d.Type, d.Name[:6])
		}
	}
}

func TestEmptyList(t *testing.T) {
	b := listMessage(SIMCONNECT_RECV_ID_CONTROLLERS_LIST, 0, nil)
	if got := (*SIMCONNECT_RECV_CONTROLLERS_LIST)(unsafe.Pointer(&b[0])).Entries(); got != nil {
		t.Errorf("entries %v", got)
	}
}
