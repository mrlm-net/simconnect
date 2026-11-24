package world

import (
	"slices"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// A change the safety net held for the player (#1662) is given back only
// once clear of conflict (#1666).
func TestEnrouteRANetHoldWaits(t *testing.T) {
	b, tw := raBed(t)
	b.air = b.air[:1]
	b.cw.keepNetHold("KLM221", netHold{
		id: westID, icao: "LKPR", pos: traffic.PosCenter, since: b.now,
		r:       traffic.Resolution{Callsign: "KLM221", ObjectID: westID, Kind: traffic.ResolveLevel, AltFt: 36000},
		planned: slices.Clone(b.west.route),
	})
	b.look(0)
	if got := b.sim.routes(); len(got) != 1 {
		t.Fatalf("routes set %v, want the RA's only", got)
	}
	raOver(b, tw)
	b.look(2 * conflictEvery)
	if got := b.sim.routes(); len(got) != 2 {
		t.Errorf("clear of conflict: routes set %v, want its route again", got)
	}
}
