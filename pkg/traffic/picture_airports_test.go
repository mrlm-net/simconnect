package traffic

import (
	"encoding/binary"
	"math"
	"testing"
)

// entries packs airports as the list does: ident[identLen], region[3],
// lat, lon, alt, padded to size, then pad bytes after the last.
func entries(size, identLen, pad int, aps ...AirportRef) []byte {
	var out []byte
	for _, a := range aps {
		e := make([]byte, size)
		copy(e, a.ICAO)
		copy(e[identLen:], a.Region)
		binary.LittleEndian.PutUint64(e[identLen+3:], math.Float64bits(a.Position.Lat))
		binary.LittleEndian.PutUint64(e[identLen+11:], math.Float64bits(a.Position.Lon))
		binary.LittleEndian.PutUint64(e[identLen+19:], math.Float64bits(a.AltM))
		out = append(out, e...)
	}
	return append(out, make([]byte, pad)...)
}

// TestDecodeAirportEntries: every known entry size decodes, also with
// padding that makes the division land on another accepted size (40-byte
// entries, 8 of them, 8 bytes over: 41, live garbage at LROP); garbage
// entries are left out.
func TestDecodeAirportEntries(t *testing.T) {
	aps := make([]AirportRef, 8)
	for i := range aps {
		aps[i] = AirportRef{ICAO: "LR" + string(rune('A'+i)) + "P", Region: "LR", AltM: 90 + float64(i)}
		aps[i].Position.Lat, aps[i].Position.Lon = 44.5+float64(i)/10, 26.0+float64(i)/10
	}
	for _, c := range []struct{ size, identLen, pad, n int }{
		{40, 9, 8, 8}, {40, 9, 0, 8}, {36, 9, 4, 3}, {41, 9, 0, 2}, {33, 6, 0, 5}, {33, 6, 3, 1}, {40, 9, 5, 1},
	} {
		got := decodeAirportEntries(entries(c.size, c.identLen, c.pad, aps[:c.n]...), c.n)
		if len(got) != c.n {
			t.Errorf("%d-byte entries ×%d +%d: %d decoded %+v", c.size, c.n, c.pad, len(got), got)
			continue
		}
		for i, a := range got {
			if a.ICAO != aps[i].ICAO || a.Region != "LR" || a.Position != aps[i].Position || a.AltM != aps[i].AltM {
				t.Errorf("%d-byte entries: %d is %+v, want %+v", c.size, i, a, aps[i])
			}
		}
	}
	bad := entries(40, 9, 0, AirportRef{ICAO: ",?R@@", Position: aps[0].Position}, aps[1])
	if got := decodeAirportEntries(bad, 2); len(got) != 1 || got[0].ICAO != aps[1].ICAO {
		t.Errorf("garbage left in: %+v", got)
	}
}
