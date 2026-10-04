package airport

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"os"
	"reflect"
	"slices"
	"testing"
	"time"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

type fakeClient struct {
	defs     map[uint32][]string
	requests []struct {
		def, req uint32
		icao     string
	}
	failRequest error
}

func (f *fakeClient) AddToFacilityDefinition(def uint32, field string) error {
	if f.defs == nil {
		f.defs = map[uint32][]string{}
	}
	f.defs[def] = append(f.defs[def], field)
	return nil
}

func (f *fakeClient) RequestFacilityData(def, req uint32, icao, region string) error {
	if f.failRequest != nil {
		return f.failRequest
	}
	f.requests = append(f.requests, struct {
		def, req uint32
		icao     string
	}{def, req, icao})
	return nil
}

// facilityMsg builds a FACILITY_DATA message carrying payload.
func facilityMsg(req uint32, typ types.SIMCONNECT_FACILITY_DATA_TYPE, item int, payload []byte) engine.Message {
	var hdr types.SIMCONNECT_RECV_FACILITY_DATA
	off := int(unsafe.Offsetof(hdr.Data))
	buf := make([]byte, off+max(len(payload), 4)+64) // slack past the record, as in real receive buffers
	h := (*types.SIMCONNECT_RECV_FACILITY_DATA)(unsafe.Pointer(&buf[0]))
	h.DwSize = types.DWORD(len(buf))
	h.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_FACILITY_DATA)
	h.UserRequestId = types.DWORD(req)
	h.Type = typ
	h.ItemIndex = types.DWORD(item)
	copy(buf[off:], payload)
	return engine.Message{SIMCONNECT_RECV: (*types.SIMCONNECT_RECV)(unsafe.Pointer(&buf[0]))}
}

func endMsg(req uint32) engine.Message {
	h := &types.SIMCONNECT_RECV_FACILITY_DATA_END{RequestId: types.DWORD(req)}
	h.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_FACILITY_DATA_END)
	return engine.Message{SIMCONNECT_RECV: &h.SIMCONNECT_RECV}
}

func bytesOf[T any](v *T) []byte {
	return append([]byte(nil), unsafe.Slice((*byte)(unsafe.Pointer(v)), unsafe.Sizeof(*v))...)
}

func runwayBytes(r RawRunway) []byte {
	b := make([]byte, runwayWireSize)
	binary.LittleEndian.PutUint64(b[0:], math.Float64bits(r.Latitude))
	binary.LittleEndian.PutUint64(b[8:], math.Float64bits(r.Longitude))
	binary.LittleEndian.PutUint64(b[16:], math.Float64bits(r.Altitude))
	binary.LittleEndian.PutUint32(b[24:], math.Float32bits(r.Heading))
	binary.LittleEndian.PutUint32(b[28:], math.Float32bits(r.Length))
	binary.LittleEndian.PutUint32(b[32:], math.Float32bits(r.Width))
	for i, v := range []int32{r.PrimaryNumber, r.PrimaryDesignator, r.SecondaryNumber, r.SecondaryDesignator} {
		binary.LittleEndian.PutUint32(b[36+4*i:], uint32(v))
	}
	return b
}

func lkprRaw(t *testing.T) RawAirport {
	t.Helper()
	b, err := os.ReadFile("testdata/LKPR.json")
	if err != nil {
		t.Fatal(err)
	}
	var raw RawAirport
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	return raw
}

// simulate produces the messages SimConnect sends for the six requests of
// one airport starting at request ID base: per request, the AIRPORT record,
// its list items, then FACILITY_DATA_END.
func simulate(raw RawAirport, base uint32, reverseItems bool) []engine.Message {
	var msgs []engine.Message
	a := airportWire{Latitude: raw.Latitude, Longitude: raw.Longitude, Altitude: raw.Altitude}
	copy(a.ICAO[:], raw.ICAO)
	copy(a.Name64[:], raw.Name)
	empty := make([]byte, 4)
	parts := [][]engine.Message{
		{facilityMsg(base, types.SIMCONNECT_FACILITY_DATA_AIRPORT, 0, bytesOf(&a))},
		nil, nil, nil, nil, nil, nil,
	}
	for i, r := range raw.Runways {
		parts[partRunway] = append(parts[partRunway], facilityMsg(base+partRunway, types.SIMCONNECT_FACILITY_DATA_RUNWAY, i, runwayBytes(r)))
	}
	// A parking record is followed by its airline records, which name it as
	// their parent; reversing keeps each group together.
	var parkingGroups [][]engine.Message
	for i := range raw.Parking {
		m := facilityMsg(base+partParking, types.SIMCONNECT_FACILITY_DATA_TAXI_PARKING, i, bytesOf(&raw.Parking[i]))
		m.AsFacilityData().UniqueRequestId = types.DWORD(5000 + i)
		group := []engine.Message{m}
		for k, code := range raw.ParkingAirlines[i] {
			c := facilityMsg(base+partParking, types.SIMCONNECT_FACILITY_DATA_TAXI_PARKING_AIRLINE, k, []byte(code))
			c.AsFacilityData().ParentUniqueRequestId = types.DWORD(5000 + i)
			group = append(group, c)
		}
		parkingGroups = append(parkingGroups, group)
	}
	if reverseItems {
		slices.Reverse(parkingGroups)
	}
	parts[partParking] = slices.Concat(parkingGroups...)
	for i := range raw.TaxiPoints {
		parts[partTaxiPoint] = append(parts[partTaxiPoint], facilityMsg(base+partTaxiPoint, types.SIMCONNECT_FACILITY_DATA_TAXI_POINT, i, bytesOf(&raw.TaxiPoints[i])))
	}
	for i := range raw.TaxiPaths {
		parts[partTaxiPath] = append(parts[partTaxiPath], facilityMsg(base+partTaxiPath, types.SIMCONNECT_FACILITY_DATA_TAXI_PATH, i, bytesOf(&raw.TaxiPaths[i])))
	}
	for i, n := range raw.TaxiNames {
		var b [32]byte
		copy(b[:], n)
		parts[partTaxiName] = append(parts[partTaxiName], facilityMsg(base+partTaxiName, types.SIMCONNECT_FACILITY_DATA_TAXI_NAME, i, b[:]))
	}
	for i, fr := range raw.Frequencies {
		w := frequencyWire{Type: fr.Type, Hz: fr.Hz}
		copy(w.Name[:], fr.Name)
		parts[partFrequency] = append(parts[partFrequency], facilityMsg(base+partFrequency, types.SIMCONNECT_FACILITY_DATA_FREQUENCY, i, bytesOf(&w)))
	}
	for p := range parts {
		items := parts[p]
		if reverseItems && p != partParking {
			for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
				items[i], items[j] = items[j], items[i]
			}
		}
		if p != partAirport {
			// List requests open with the AIRPORT record, which carries no fields.
			msgs = append(msgs, facilityMsg(base+uint32(p), types.SIMCONNECT_FACILITY_DATA_AIRPORT, 0, empty))
		}
		msgs = append(msgs, items...)
		msgs = append(msgs, endMsg(base+uint32(p)))
	}
	return msgs
}

func feed(t *testing.T, l *Loader, msgs []engine.Message) []Result {
	t.Helper()
	var out []Result
	for _, m := range msgs {
		if r, ok := l.Handle(m); ok {
			out = append(out, r)
		}
	}
	return out
}

func TestLoaderRegistersAndRequests(t *testing.T) {
	c := &fakeClient{}
	l := NewLoader(c, LoaderWithIDs(500, 900))
	if err := l.Request(" lkpr "); err != nil {
		t.Fatal(err)
	}
	if len(c.defs) != 7 || !reflect.DeepEqual(c.defs[500], loaderDefinitions[0]) || !reflect.DeepEqual(c.defs[502], loaderDefinitions[2]) {
		t.Fatalf("definitions = %v", c.defs)
	}
	if len(c.requests) != 7 {
		t.Fatalf("requests = %d, want 7", len(c.requests))
	}
	for i, r := range c.requests {
		if r.def != 500+uint32(i) || r.req != 900+uint32(i) || r.icao != "LKPR" {
			t.Errorf("request %d = %+v", i, r)
		}
	}
	if err := l.Request("EDDM"); err != nil {
		t.Fatal(err)
	}
	if len(c.defs[500]) != len(loaderDefinitions[0]) {
		t.Error("definitions registered twice")
	}
	if got := l.Pending(); !reflect.DeepEqual(got, []string{"LKPR", "EDDM"}) {
		t.Errorf("Pending = %v", got)
	}
}

func TestLoaderBuildsLKPR(t *testing.T) {
	raw := lkprRaw(t)
	raw.Frequencies = lkprFrequencies // (the captured data has none)
	want, err := BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, reverse := range []bool{false, true} {
		l := NewLoader(&fakeClient{})
		if err := l.Request("LKPR"); err != nil {
			t.Fatal(err)
		}
		res := feed(t, l, simulate(raw, DefaultLoaderRequestBase, reverse))
		if len(res) != 1 || res[0].Err != nil {
			t.Fatalf("reverse=%v: results %+v", reverse, res)
		}
		got := res[0].Layout
		if !reflect.DeepEqual(got, want) {
			t.Errorf("reverse=%v: loaded layout differs from BuildLayout of the same records", reverse)
		}
		if len(l.Pending()) != 0 {
			t.Errorf("Pending after completion = %v", l.Pending())
		}
	}
}

func TestLoaderIgnoresForeignMessages(t *testing.T) {
	raw := lkprRaw(t)
	l := NewLoader(&fakeClient{})
	if err := l.Request("LKPR"); err != nil {
		t.Fatal(err)
	}
	msgs := simulate(raw, DefaultLoaderRequestBase, false)
	// Another component's facility traffic with request IDs outside the Loader's range.
	foreign := simulate(RawAirport{ICAO: "EDDM", Latitude: 48}, 42, false)
	var mixed []engine.Message
	for i := range max(len(msgs), len(foreign)) {
		if i < len(foreign) {
			mixed = append(mixed, foreign[i])
		}
		if i < len(msgs) {
			mixed = append(mixed, msgs[i])
		}
	}
	mixed = append(mixed, engine.Message{})
	res := feed(t, l, mixed)
	if len(res) != 1 || res[0].ICAO != "LKPR" || res[0].Err != nil || len(res[0].Layout.TaxiPaths) != 2350 {
		t.Fatalf("results %+v", res)
	}
}

func TestLoaderConcurrentAirports(t *testing.T) {
	raw := lkprRaw(t)
	other := RawAirport{ICAO: "TEST", Name: "Test", Latitude: 50, Longitude: 14, TaxiPoints: raw.TaxiPoints[:2],
		TaxiPaths: []RawTaxiPath{{Type: 4, Start: 0, End: 1}}}
	l := NewLoader(&fakeClient{})
	for _, icao := range []string{"LKPR", "TEST"} {
		if err := l.Request(icao); err != nil {
			t.Fatal(err)
		}
	}
	a := simulate(raw, DefaultLoaderRequestBase, false)
	b := simulate(other, DefaultLoaderRequestBase+uint32(len(loaderDefinitions)), false)
	res := feed(t, l, append(b, a...))
	if len(res) != 2 || res[0].ICAO != "TEST" || res[1].ICAO != "LKPR" {
		t.Fatalf("results %+v", res)
	}
	if len(res[0].Layout.TaxiPoints) != 2 || len(res[1].Layout.TaxiPoints) != 1967 {
		t.Error("airports mixed up")
	}
}

func TestLoaderExpire(t *testing.T) {
	l := NewLoader(&fakeClient{}, LoaderWithTimeout(time.Second))
	if err := l.Request("ZZZZ"); err != nil {
		t.Fatal(err)
	}
	if res := l.Expire(time.Now()); len(res) != 0 {
		t.Fatalf("expired early: %+v", res)
	}
	res := l.Expire(time.Now().Add(2 * time.Second))
	if len(res) != 1 || !errors.Is(res[0].Err, ErrTimeout) || res[0].ICAO != "ZZZZ" {
		t.Fatalf("Expire = %+v", res)
	}
	// Late data for the expired request is ignored.
	if r, ok := l.Handle(endMsg(DefaultLoaderRequestBase)); ok {
		t.Errorf("late END produced %+v", r)
	}
}

func TestLoaderNoData(t *testing.T) {
	l := NewLoader(&fakeClient{})
	if err := l.Request("ZZZZ"); err != nil {
		t.Fatal(err)
	}
	var res []Result
	for p := range loaderDefinitions {
		if r, ok := l.Handle(endMsg(DefaultLoaderRequestBase + uint32(p))); ok {
			res = append(res, r)
		}
	}
	if len(res) != 1 || !errors.Is(res[0].Err, ErrNoData) {
		t.Fatalf("results %+v", res)
	}
}

func TestLoaderSlotsAndErrors(t *testing.T) {
	c := &fakeClient{}
	l := NewLoader(c)
	if err := l.Request(""); err == nil {
		t.Error("empty ICAO accepted")
	}
	for i := range loaderSlots {
		if err := l.Request("A" + string(rune('A'+i))); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.Request("FULL"); err == nil {
		t.Error("request accepted with every slot in flight")
	}
	l.Reset(nil)
	if len(l.Pending()) != 0 {
		t.Error("Reset kept in-flight requests")
	}
	c.failRequest = errors.New("not connected")
	if err := l.Request("LKPR"); err == nil || len(l.Pending()) != 0 {
		t.Errorf("failed request: err=%v pending=%v", err, l.Pending())
	}
	if len(c.defs[DefaultLoaderDefinitionBase]) != 2*len(loaderDefinitions[0]) {
		t.Error("Reset did not re-register definitions")
	}
}

// TestLoaderParkingAirlines: airline records that follow a parking record
// as its children end up on that stand, in order.
func TestLoaderParkingAirlines(t *testing.T) {
	raw := lkprRaw(t)
	raw.ParkingAirlines = map[int][]string{18: {"CSA", "AFR"}, 3: {"DLH"}}
	want, err := BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, reverse := range []bool{false, true} {
		l := NewLoader(&fakeClient{})
		if err := l.Request("LKPR"); err != nil {
			t.Fatal(err)
		}
		res := feed(t, l, simulate(raw, DefaultLoaderRequestBase, reverse))
		if len(res) != 1 || res[0].Err != nil {
			t.Fatalf("reverse=%v: results %+v", reverse, res)
		}
		got := res[0].Layout
		if !slices.Equal(got.Parking[18].Airlines, []string{"CSA", "AFR"}) || !slices.Equal(got.Parking[3].Airlines, []string{"DLH"}) || got.Parking[4].Airlines != nil {
			t.Errorf("reverse=%v: airlines C22 %v, 3 %v, 4 %v", reverse, got.Parking[18].Airlines, got.Parking[3].Airlines, got.Parking[4].Airlines)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("reverse=%v: loaded layout differs from BuildLayout of the same records", reverse)
		}
	}
}

// lkprFrequencies are LKPR's frequencies as FREQUENCY records.
var lkprFrequencies = []RawFrequency{
	{Type: int32(types.SIMCONNECT_FACILITY_FREQUENCY_TYPE_ATIS), Hz: 122155000, Name: "PRAHA ATIS"},
	{Type: int32(types.SIMCONNECT_FACILITY_FREQUENCY_TYPE_CLEARANCE), Hz: 120355000, Name: "PRAHA DELIVERY"},
	{Type: int32(types.SIMCONNECT_FACILITY_FREQUENCY_TYPE_GROUND), Hz: 121905000, Name: "PRAHA GROUND"},
	{Type: int32(types.SIMCONNECT_FACILITY_FREQUENCY_TYPE_TOWER), Hz: 118105000, Name: "PRAHA TOWER"},
	{Type: int32(types.SIMCONNECT_FACILITY_FREQUENCY_TYPE_APPROACH), Hz: 120530000, Name: "PRAHA APPROACH"},
}

// Frequencies load with the layout; each position finds its own, or the
// one ATC falls back to.
func TestFrequencies(t *testing.T) {
	raw := lkprRaw(t)
	raw.Frequencies = lkprFrequencies
	l, err := BuildLayout(raw)
	if err != nil {
		t.Fatal(err)
	}
	for kind, want := range map[string]string{
		FreqTower: "118.105", FreqGround: "121.905", FreqClearance: "120.355", FreqATIS: "122.155",
		FreqApproach: "120.53", FreqDeparture: "120.53", // no departure frequency: approach
	} {
		f, ok := l.FrequencyFor(kind)
		if !ok || f.String() != want {
			t.Errorf("%s: %v %v, want %s", kind, f, ok, want)
		}
	}
	if _, ok := l.FrequencyFor(FreqCenter); ok {
		t.Error("a centre frequency at LKPR")
	}
	if got := FormatMHz(121.9); got != "121.90" {
		t.Errorf("FormatMHz(121.9) = %s", got)
	}
}
