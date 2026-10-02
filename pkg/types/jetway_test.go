//go:build windows
// +build windows

package types

import (
	"encoding/binary"
	"math"
	"testing"
)

// TestDecodeJetwayData: a packed 160-byte entry as MSFS 2024 sends it
// (measured live at LKPR: status at 48, the object IDs at 152 and 156).
func TestDecodeJetwayData(t *testing.T) {
	b := make([]byte, JetwayDataSize)
	le := binary.LittleEndian
	copy(b, "LKPR")
	le.PutUint32(b[8:], 10)
	le.PutUint64(b[12:], math.Float64bits(50.10885))
	le.PutUint64(b[20:], math.Float64bits(14.26451))
	le.PutUint32(b[44:], math.Float32bits(34))
	le.PutUint32(b[48:], 7)
	le.PutUint64(b[80:], math.Float64bits(1.5))
	le.PutUint32(b[152:], 93552640)
	le.PutUint32(b[156:], 119914502)
	j, ok := DecodeJetwayData(b)
	if !ok || string(j.AirportIcao[:4]) != "LKPR" || j.ParkingIndex != 10 || j.LLA.Latitude != 50.10885 || j.PBH.Heading != 34 ||
		j.Status != 7 || j.MainHandlePos.X != 1.5 || j.JetwayObjectId != 93552640 || j.AttachedObjectId != 119914502 {
		t.Errorf("decoded %+v", j)
	}
	if _, ok := DecodeJetwayData(b[:100]); ok {
		t.Error("decoded a short entry")
	}
}
