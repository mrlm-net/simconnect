package traffic

import "testing"

// Low-cost airlines take stands without a jetway first, all day and at
// every airport (the user: "RYR on LKPR always outside the terminal"); the
// network airlines keep the gates (AUA).
func TestRemoteStands(t *testing.T) {
	for _, c := range []struct {
		model, airline string
		want           bool
	}{
		{"B738", "RYR", true}, {"A320", "EZY", true}, {"A321", "WZZ", true},
		{"A320", "AUA", false}, {"A320", "CSA", false}, {"A320", "", false},
		{"AT76", "CSA", NoJetway("AT76")}, // the type's own rule as before
	} {
		if got := RemoteStands(c.model, c.airline); got != c.want {
			t.Errorf("%s %s: %v, want %v", c.airline, c.model, got, c.want)
		}
	}
}
