//go:build windows
// +build windows

package lvars

import (
	"testing"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

type fakeClient struct {
	defs map[uint32]string
	set  map[string]float64
}

func (f *fakeClient) AddToDataDefinition(def uint32, name, unit string, typ types.SIMCONNECT_DATATYPE, e float32, id uint32) error {
	f.defs[def] = name
	return nil
}

func (f *fakeClient) SetDataOnSimObject(def, obj uint32, flags types.SIMCONNECT_DATA_SET_FLAG, n, size uint32, data unsafe.Pointer) error {
	f.set[f.defs[def]] = *(*float64)(data)
	return nil
}

// An L:var is defined once per connection, written as asked, by name with
// or without "L:"; Reset defines it again.
func TestWriter(t *testing.T) {
	c := &fakeClient{defs: map[uint32]string{}, set: map[string]float64{}}
	w := NewWriter(c, 0x9000, 2)
	for _, v := range []float64{1, 2} {
		if err := w.Set("MYCREW_BOARDING", v); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Set("L:FSDT_GSX_NUMPASSENGERS", 111); err != nil {
		t.Fatal(err)
	}
	if len(c.defs) != 2 || c.set["L:MYCREW_BOARDING"] != 2 || c.set["L:FSDT_GSX_NUMPASSENGERS"] != 111 {
		t.Errorf("defs %v, set %v", c.defs, c.set)
	}
	if err := w.Set("THIRD", 1); err == nil {
		t.Error("a third L:var with max 2")
	}
	w.Reset(nil)
	if err := w.Set("THIRD", 3); err != nil || c.set["L:THIRD"] != 3 {
		t.Errorf("after Reset: %v %v", err, c.set)
	}
}
