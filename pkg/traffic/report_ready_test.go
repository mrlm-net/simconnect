package traffic

import (
	"strings"
	"testing"
)

// A PUSH APPROVAL IS NOT A TAXI CLEARANCE, and the approval now says what
// the next call will be (#1268). The player's words: "when pushback is given
// it should also then say something like: when ready for taxi contact
// ground". The clearance is not offered on the stand any more, so without
// this the strip simply goes quiet and nothing explains why.
func TestWithReportReady(t *testing.T) {
	for _, tc := range []struct {
		name     string
		tx       Transmission
		want     string
		readback string
	}{
		{
			"pushback",
			WithReportReady(ClearedPushback("CSA1"), "ready for taxi"),
			"CSA1, pushback approved, report ready for taxi",
			"Pushback approved, wilco, CSA1",
		},
		{
			"push and start in one",
			WithReportReady(ClearedPushbackAndStartUp("CSA1"), "ready for taxi"),
			"CSA1, pushback and start up approved, report ready for taxi",
			"Pushback and start up approved, wilco, CSA1",
		},
		{
			// Composes with the facing, and in either order.
			"with a facing",
			WithFacing(WithReportReady(ClearedPushback("CSA1"), "ready for taxi"), "east"),
			"CSA1, pushback approved, facing east, report ready for taxi",
			"Pushback approved, facing east, wilco, CSA1",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.tx.Text != tc.want {
				t.Errorf("said:\n got %q\nwant %q", tc.tx.Text, tc.want)
			}
			rb, ok := Readback(tc.tx)
			if !ok {
				t.Fatal("want a readback")
			}
			if rb.Text != tc.readback {
				t.Errorf("read back:\n got %q\nwant %q", rb.Text, tc.readback)
			}
		})
	}
}

// Nothing to report, nothing said: an empty report, and any clearance that is
// not a push approval, are left exactly as they were.
func TestWithReportReadyLeavesOthersAlone(t *testing.T) {
	plain := ClearedPushback("CSA1")
	if got := WithReportReady(plain, ""); got.Text != plain.Text {
		t.Errorf("no report: want %q unchanged, got %q", plain.Text, got.Text)
	}
	start := ClearedStartUp("CSA1")
	if got := WithReportReady(start, "ready for taxi"); got.Text != start.Text {
		t.Errorf("start-up approval: want %q unchanged, got %q", start.Text, got.Text)
	}
	if strings.Contains(ClearedPushback("CSA1").Text, "report") {
		t.Error("a bare pushback approval must not say report by itself")
	}
}
