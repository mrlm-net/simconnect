//go:build windows

package flight

import (
	"testing"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/registry"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// TestRecVarsKnown: every SimVar recorded is a real one in a unit it takes.
func TestRecVarsKnown(t *testing.T) {
	for _, v := range recVars {
		if err := registry.Validate(v.name, v.unit); err != nil {
			t.Error(err)
		}
	}
}

type fakeClient struct {
	defs     []string
	requests []types.SIMCONNECT_PERIOD
}

func (f *fakeClient) AddToDataDefinition(_ uint32, name, _ string, _ types.SIMCONNECT_DATATYPE, _ float32, _ uint32) error {
	f.defs = append(f.defs, name)
	return nil
}

func (f *fakeClient) RequestDataOnSimObject(_, _, _ uint32, p types.SIMCONNECT_PERIOD, _ types.SIMCONNECT_DATA_REQUEST_FLAG, _, _, _ uint32) error {
	f.requests = append(f.requests, p)
	return nil
}

// frameMessage is a SIMOBJECT_DATA message of vals for request req.
func frameMessage(def, req uint32, vals []float64) (engine.Message, []byte) {
	var d types.SIMCONNECT_RECV_SIMOBJECT_DATA
	off := int(unsafe.Offsetof(d.DwData))
	buf := make([]byte, off+len(vals)*8)
	h := (*types.SIMCONNECT_RECV_SIMOBJECT_DATA)(unsafe.Pointer(&buf[0]))
	h.DwID = types.DWORD(types.SIMCONNECT_RECV_ID_SIMOBJECT_DATA)
	h.DwRequestID, h.DwDefineID = types.DWORD(req), types.DWORD(def)
	copy(unsafe.Slice((*float64)(unsafe.Pointer(&buf[off])), len(vals)), vals)
	return engine.Message{SIMCONNECT_RECV: &h.SIMCONNECT_RECV, Size: uint32(len(buf))}, buf
}

// TestRecorder: the frame defined once, asked every frame; a frame becomes
// a sample (pitch and bank turned nose up and right wing down positive);
// Stop asks no more and gives the Track.
func TestRecorder(t *testing.T) {
	c := &fakeClient{}
	r := NewRecorder(c, 0)
	var heard int
	r.OnSample = func(uint32, Sample) { heard++ }
	if err := r.Start(types.SIMCONNECT_OBJECT_ID_USER, RecordOptions{Model: "A320"}); err != nil {
		t.Fatal(err)
	}
	if err := r.Start(types.SIMCONNECT_OBJECT_ID_USER, RecordOptions{}); err == nil {
		t.Error("recorded twice")
	}
	if err := r.Start(42, RecordOptions{}); err != nil {
		t.Fatal(err)
	}
	if len(c.defs) != len(recVars) || len(c.requests) != 2 || c.requests[0] != types.SIMCONNECT_PERIOD_SIM_FRAME {
		t.Fatalf("%d definitions, requests %v", len(c.defs), c.requests)
	}
	vals := make([]float64, len(recVars))
	for i, v := range recVars {
		switch v.name {
		case "SIMULATION TIME":
			vals[i] = 12.5
		case "PLANE PITCH DEGREES":
			vals[i] = -8 // SimConnect: nose up negative
		case "PLANE BANK DEGREES":
			vals[i] = 3
		case "TURB ENG N1:2":
			vals[i] = 84
		case "LIGHT BEACON":
			vals[i] = 1
		}
	}
	msg, buf := frameMessage(r.base, r.byObj[types.SIMCONNECT_OBJECT_ID_USER], vals)
	if !r.Handle(msg) {
		t.Fatal("frame not taken")
	}
	_ = buf
	short, _ := frameMessage(r.base, r.byObj[types.SIMCONNECT_OBJECT_ID_USER], vals[:5])
	r.Handle(short) // cut short: not read
	tr := r.Stop(types.SIMCONNECT_OBJECT_ID_USER)
	if tr == nil || len(tr.Samples) != 1 || !tr.User || tr.Model != "A320" {
		t.Fatalf("track %+v", tr)
	}
	s := tr.Samples[0]
	if s.T != 12.5 || s.Pitch != 8 || s.Bank != -3 || s.N1[1] != 84 || s.Lights != LightBeacon || heard != 1 {
		t.Errorf("sample %+v, heard %d", s, heard)
	}
	if c.requests[len(c.requests)-1] != types.SIMCONNECT_PERIOD_NEVER {
		t.Error("not stopped")
	}
	if r.Snapshot(42) == nil || r.Snapshot(types.SIMCONNECT_OBJECT_ID_USER) != nil {
		t.Error("snapshots")
	}
}
