package traffic

import "testing"

// TestSelfAnnounce: the calls to a field's traffic, said to "<Field>
// Traffic" and closed with the field's name, nobody answering; the IFR
// release and cancellation with approach (#1296).
func TestSelfAnnounce(t *testing.T) {
	for _, c := range []struct {
		tx   Transmission
		want string
	}{
		{SelfTaxiing("Henderson", "OKABC", "24"), "Henderson Traffic, OKABC, taxiing to runway 24, Henderson"},
		{SelfDeparting("Henderson", "OKABC", "24", "to the north"), "Henderson Traffic, OKABC, departing runway 24, to the north, Henderson"},
		{SelfDeparting("Henderson", "OKABC", "24", PatternStay), "Henderson Traffic, OKABC, departing runway 24, remaining in the pattern, Henderson"},
		{SelfInbound("Henderson", "OKABC", 10, "south"), "Henderson Traffic, OKABC, 10 miles south, inbound for landing, Henderson"},
		{SelfInbound("Henderson", "OKABC", 1, ""), "Henderson Traffic, OKABC, 1 mile, inbound for landing, Henderson"},
		{SelfPattern("Henderson", "OKABC", "left downwind", "24"), "Henderson Traffic, OKABC, left downwind runway 24, Henderson"},
		{SelfClear("Henderson", "OKABC", "24"), "Henderson Traffic, OKABC, clear of runway 24, Henderson"},
		{ReleasedForDeparture("OKABC"), "OKABC, released for departure, report airborne"},
		{CancelIFR("Portland Approach", "OKABC"), "Portland Approach, OKABC, cancel IFR"},
		{IFRCancelled("OKABC"), "OKABC, IFR cancellation received"},
	} {
		if c.tx.Text != c.want {
			t.Errorf("%q, want %q", c.tx.Text, c.want)
		}
	}
	tx := SelfTaxiing("Henderson", "OKABC", "24")
	if tx.Position != PosTraffic || !tx.Pilot || tx.Intent != IntentSelfAnnounce || tx.Params[ParamSelf] != "taxiing" || tx.Params[ParamRunway] != "24" {
		t.Errorf("%+v", tx)
	}
}
