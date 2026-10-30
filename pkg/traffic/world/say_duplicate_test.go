package world

import (
	"strings"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// TestSayDuplicateLogged: the same ATC call again at once is said once, and the
// log says so (#1312).
func TestSayDuplicateLogged(t *testing.T) {
	l := &trafficLog{}
	it := &controlled{cc: &controlCenter{log: l}, lastATC: "OKABC, hold position", lastATCAt: traffic.Now()}
	it.say(traffic.Transmission{Text: "OKABC, hold position", Frequency: "121.9"})
	if got := strings.Join(l.recent(1), ""); !strings.Contains(got, "said once") {
		t.Errorf("log: %q", got)
	}
}
