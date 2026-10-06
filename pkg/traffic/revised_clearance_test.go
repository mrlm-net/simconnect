package traffic

import "testing"

// A revised departure clearance after a runway change: ICAO repeats the
// whole clearance after "hold position" (CAP 413 2.73, 4.38), FAA changes
// the SID (JO 7110.65 4-2-5); the crew reads back "Holding, …"; an unable
// gives its reason (CAP 413 2.72).
func TestRevisedDepartureClearance(t *testing.T) {
	c := DepartureClearance{Destination: "Budapest", SID: "VOZ 5D", Runway: "06", Level: "flight level 100", Squawk: "4521"}
	tx := RevisedDepartureClearance("CSA123", c, "VOZ 5M", RevisedHolding, "")
	want := "CSA123, hold position, cleared to Budapest, VOZ 5D departure, flight planned route, runway 06, climb via SID to flight level 100, squawk 4521"
	if tx.Text != want || tx.Position != PosTower {
		t.Errorf("ICAO: %q at %s\nwant %q", tx.Text, tx.Position, want)
	}
	rb, ok := Readback(tx)
	if wantRB := "Holding, cleared to Budapest, VOZ 5D departure, flight planned route, runway 06, climb via SID to flight level 100, squawk 4521, CSA123"; !ok || rb.Text != wantRB {
		t.Errorf("readback %q\nwant %q", rb.Text, wantRB)
	}
	faa := RevisedDepartureClearance("AAL12", c, "VOZ 5M", RevisedHolding, PhraseologyFAA)
	if want := "AAL12, hold position, change VOZ 5M departure to read VOZ 5D departure"; faa.Text != want {
		t.Errorf("FAA: %q, want %q", faa.Text, want)
	}
	if rb, _ := Readback(faa); rb.Text != "Hold position, change VOZ 5M departure to read VOZ 5D departure, AAL12" { // FAA: as given (AIM 4-4-7)
		t.Errorf("FAA readback %q", rb.Text)
	}
	// Before taxi (the usual case) from delivery, taxiing from ground: no
	// "hold position".
	for at, pos := range map[RevisedAt]Position{RevisedBeforeTaxi: PosDelivery, RevisedTaxiing: PosGround} {
		tx := RevisedDepartureClearance("CSA123", c, "VOZ 5M", at, "")
		if tx.Position != pos || tx.Text[:len("CSA123, cleared")] != "CSA123, cleared" {
			t.Errorf("%d: %q at %s", at, tx.Text, tx.Position)
		}
		if rb, _ := Readback(tx); rb.Text[:len("Cleared to")] != "Cleared to" {
			t.Errorf("%d: readback %q", at, rb.Text)
		}
	}
	if u := Unable(PosTower, "CSA123", "VOZ 5D departure", "due performance"); u.Text != "Unable VOZ 5D departure, due performance, CSA123" {
		t.Errorf("unable %q", u.Text)
	}
	if a := UnableAcknowledged(PosTower, "CSA123", true, "advise when ready"); a.Text != "CSA123, roger, hold position, advise when ready" {
		t.Errorf("ack %q", a.Text)
	}
}
