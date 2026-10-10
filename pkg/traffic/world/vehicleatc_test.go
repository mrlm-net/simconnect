package world

import "testing"

// Each kind of vehicle is called by its own word on the radio (a follow-me
// car was "Tug 2" at LOWI), numbered per word.
func TestVehicleNames(t *testing.T) {
	v := &vehicleATC{names: map[uint32]string{}, n: map[string]int{}}
	for _, c := range []struct {
		id   uint32
		kind string
		want string
	}{
		{1, "tug", "Tug 1"}, {2, "follow-me", "Follow-me 1"}, {3, "fuel truck", "Fuel 1"},
		{4, "stairs", "Stairs 1"}, {5, "GPU", "GPU 1"}, {6, "bus", "Bus 1"},
		{7, "follow-me", "Follow-me 2"}, {8, "tug", "Tug 2"}, {2, "follow-me", "Follow-me 1"},
	} {
		if got := v.name(c.id, c.kind); got != c.want {
			t.Errorf("name(%d, %q) = %q, want %q", c.id, c.kind, got, c.want)
		}
	}
}
