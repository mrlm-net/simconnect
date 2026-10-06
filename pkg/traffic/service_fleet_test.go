package traffic

import "testing"

// A fleet hands out so many of each kind: the next one waits (told once),
// one given back is free again; a holder taking again keeps its own (#830).
func TestVehicleFleet(t *testing.T) {
	f := NewVehicleFleet(map[VehicleKind]int{VehicleTug: 2})
	var waits []string
	f.OnWait = func(k VehicleKind, owner string, busy int) { waits = append(waits, owner) }
	if !f.Take(VehicleTug, "A") || !f.Take(VehicleTug, "B") || !f.Take(VehicleTug, "A") {
		t.Fatal("two tugs: A and B each get one, A again keeps its own")
	}
	if f.Take(VehicleTug, "C") || f.Take(VehicleTug, "C") {
		t.Fatal("a third with both out")
	}
	if len(waits) != 1 || waits[0] != "C" {
		t.Errorf("waits told %v, want C once", waits)
	}
	f.Give(VehicleTug, "A")
	if !f.Take(VehicleTug, "C") {
		t.Error("A's tug given back: C still waits")
	}
	if out, size := f.Out(VehicleTug); out != 2 || size != 2 {
		t.Errorf("out %d of %d", out, size)
	}
	if !f.Take(VehicleFuel, "X") {
		t.Error("no fuel trucks sized: unlimited")
	}
	if s := DefaultFleetSize(75); s[VehicleTug] != 8 || s[VehicleFuel] != 5 {
		t.Errorf("75 stands: %v", s)
	}
	if s := DefaultFleetSize(4); s[VehicleTug] != MinTugs || s[VehicleFuel] != MinFuelTrucks {
		t.Errorf("4 stands: %v", s)
	}
}

// A departure with the airport's only tug out waits for it: no tug sent,
// the push held, until the tug comes back; then it goes (#830).
func TestDepartureWaitsForFleetTug(t *testing.T) {
	fleet := NewVehicleFleet(map[VehicleKind]int{VehicleTug: 1})
	fleet.Take(VehicleTug, "OTHER")
	tug := &fakeTug{doneAfter: 30}
	ctl, _, run, _ := injectedDeparture(t, TaxiRequest{Tug: tug}, TaxiWithServices(fleet))
	go func() {
		for range ctl.Events() {
		}
	}()
	if !run(TaxiAwaitingPushback, 600) {
		t.Fatalf("state %v", ctl.State())
	}
	ctl.ClearPushback()
	run(TaxiPushback, 60*30)
	if len(tug.attached) != 0 || ctl.State() != TaxiAwaitingPushback {
		t.Fatalf("with the only tug out: attached %d, state %v", len(tug.attached), ctl.State())
	}
	fleet.Give(VehicleTug, "OTHER")
	if !run(TaxiAwaitingTaxi, 60*600) {
		t.Fatalf("tug back: state %v", ctl.State())
	}
	if len(tug.attached) != 1 {
		t.Errorf("attached %d", len(tug.attached))
	}
	run(TaxiTaxiing, 60*60)
	if out, _ := fleet.Out(VehicleTug); out != 0 {
		t.Errorf("after the push the tug is not given back: %d out", out)
	}
}
