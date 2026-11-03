package traffic

import "testing"

// Delivery acknowledges the readback and transfers to ground in one call
// (#1425), read back as the transfer.
func TestReadbackCorrectAndHandoff(t *testing.T) {
	tx := ReadbackCorrectAndHandoff(Handoff("PLL610", PosDelivery, PosGround, "Gatwick Ground", "121.805"))
	if want := "PLL610, readback correct, contact Gatwick Ground 121.805"; tx.Text != want {
		t.Errorf("said %q, want %q", tx.Text, want)
	}
	if rb, ok := Readback(tx); !ok || rb.Text == "" {
		t.Errorf("no readback of the transfer: %+v", rb)
	}
}
