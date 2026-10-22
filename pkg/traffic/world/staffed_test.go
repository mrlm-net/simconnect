package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TestStaffedAt: a whole position staffed covers its stations; a single
// station only itself; another airport nothing (#1096).
func TestStaffedAt(t *testing.T) {
	w := &World{st: &state{core: newCore(&trafficLog{})}}
	w.SetStaffed([]StaffedStation{{ICAO: "lkpr", Position: "Tower"}, {ICAO: "LKPR", Station: "Ruzyne Ground North"}})
	for _, c := range []struct {
		icao    string
		pos     traffic.Position
		station string
		want    bool
	}{
		{"LKPR", traffic.PosTower, "Ruzyne Tower", true},
		{"LKPR", traffic.PosGround, "Ruzyne Ground North", true},
		{"LKPR", traffic.PosGround, "Ruzyne Ground South", false},
		{"LKPR", traffic.PosApproach, "Praha Approach", false},
		{"EDDF", traffic.PosTower, "Frankfurt Tower", false},
	} {
		if _, got := w.st.core.staffedAt(c.icao, c.pos, c.station); got != c.want {
			t.Errorf("%s %s %q: staffed %v, want %v", c.icao, c.pos, c.station, got, c.want)
		}
	}
}

// TestHeldClearancePhrase: a departure clearance held for a staffed delivery
// is what `clearance` would say, and nothing is said when none is held
// (#1156). The words have to be previewable like every other clearance, or a
// client cannot show the player what they are about to transmit.
func TestHeldClearancePhrase(t *testing.T) {
	held := traffic.ClearedDeparture("CSA123", traffic.DepartureClearance{
		Destination: "Frankfurt", SID: "BALTU 7D", Runway: "24", Level: "5000 feet", Squawk: "4521",
	})
	for _, c := range []struct {
		name    string
		pending traffic.Transmission
		want    string
	}{
		{"held", held, held.Text},
		{"none", traffic.Transmission{}, ""},
	} {
		it := &controlled{Tail: "CSA123", pendingClearance: c.pending}
		if got := it.phraseView(ControlView{Runway: "24"}, nil, "clearance", -1); got.Text != c.want {
			t.Errorf("%s: phrase %q, want %q", c.name, got.Text, c.want)
		}
	}
}

// TestClearanceNeedsOneWaiting: `clearance` and an amendment are both refused
// when no clearance is waiting, and an amendment is refused for an arrival
// (#1156). Neither must reach the agenda with nothing to say.
func TestClearanceNeedsOneWaiting(t *testing.T) {
	dep := &controlled{Tail: "CSA123", dep: &fakeDep{}}
	if err := dep.act("clearance", -1); err == nil {
		t.Error("act(clearance) with none waiting: no error")
	}
	if err := dep.amendClearance("24", "", ""); err == nil {
		t.Error("amend with none waiting: no error")
	}
	// Nothing at all asked for is not an error: the clearance goes as filed.
	if err := dep.amendClearance("", "", ""); err != nil {
		t.Errorf("amend with nothing asked for: %v", err)
	}
	arr := &controlled{Tail: "CSA456"}
	if err := arr.amendClearance("24", "", ""); err == nil {
		t.Error("amend on an arrival: no error")
	}
}

// (Giving one is not unit-tested here: `clearance` goes through the agenda and
// the radio, which want a built control centre. It is covered end to end —
// with delivery staffed and the clearance never given, four minutes of traffic
// at LKTB produced only the crew's requests and no delivery answer at all.)
