package world

import (
	"testing"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TestNoStubPhrase: with nothing due, a clearance and a handoff have no words.
//
// WHAT THIS PINS. phraseView's switch ends in a catch-all that builds a transmission
// out of the action's NAME, which is right for the verbs that have no phrase of their
// own and wrong for these two: once the departure clearance has been given,
// `GET /api/control/{id}/clearance/phrase` answered "UAE168, clearance" attributed to
// Ground — not the words, not the position, and the page put it in the radio log.
//
// The first attempt at this fix put the guard on the handoff case by mistake, where it
// read correctly and did nothing, and the clearance went on answering the stub. A
// hundred and twenty-two of them in a two-minute run; nothing failed.
func TestNoStubPhrase(t *testing.T) {
	for _, action := range []string{"clearance", "handoff"} {
		it := &controlled{Tail: "UAE168"}
		got := it.phraseView(ControlView{Tail: "UAE168"}, nil, action, -1)
		if got.Intent != "" || got.Text != "" {
			t.Errorf("%s with nothing due: intent %q, text %q — want neither",
				action, got.Intent, got.Text)
		}
	}
}

// TestClearancePhraseWhenDue: and when there IS one, it is still said — the guard must
// not silence the real thing.
func TestClearancePhraseWhenDue(t *testing.T) {
	it := &controlled{Tail: "UAE168"}
	it.clearanceDue = &traffic.DepartureClearance{Runway: "24", SID: "VENO 5A"}
	got := it.phraseView(ControlView{Tail: "UAE168", Runway: "24"}, nil, "clearance", -1)
	if got.Intent == "" {
		t.Fatalf("a clearance IS due and phraseView said nothing: %+v", got)
	}
}
