package jetway

import (
	"strings"

	"github.com/mrlm-net/simconnect/pkg/dict"
)

// DoorOffset puts a type's helper where its jetway meets our aircraft's
// door: AheadM metres ahead of our aircraft along its heading and RightM to
// its right (negative: behind, left). The jetway docks to the invisible
// helper, not to our aircraft, and where it meets the visible door differs
// by type: live at LKPR the A320neo's was right where the sim parks the
// helper (3.5–4 m ahead of ours), the A321's and the 737's not. A type
// without one keeps the sim's place.
type DoorOffset struct {
	Type   string  `json:"type"`
	AheadM float64 `json:"aheadM"`
	RightM float64 `json:"rightM"`
	Note   string  `json:"note,omitempty"`
}

// shippedDoorOffsets are the offsets measured live (the user's screenshots
// of the docked jetway); none yet.
var shippedDoorOffsets = []DoorOffset{}

var doorOffsetsNow dict.Value[[]DoorOffset]

func init() {
	dict.Register(dict.Keyed("jetway.doorOffsets", "type", "", "",
		func() []DoorOffset { return append([]DoorOffset(nil), shippedDoorOffsets...) },
		func(o DoorOffset) string { return strings.ToUpper(o.Type) },
		func(items []DoorOffset) { doorOffsetsNow.Store(items) }))
	_ = dict.Reset("jetway.doorOffsets")
}

// DoorOffsetFor is the offset of model (a type, or a title naming one: the
// longest type it contains); false none.
func DoorOffsetFor(model string) (DoorOffset, bool) {
	m := strings.ToUpper(model)
	var best DoorOffset
	for _, o := range doorOffsetsNow.Load() {
		t := strings.ToUpper(o.Type)
		if t != "" && strings.Contains(m, t) && len(t) > len(best.Type) {
			best = o
		}
	}
	return best, best.Type != ""
}

// DoorOffsets are the offsets in use.
func DoorOffsets() []DoorOffset { return append([]DoorOffset(nil), doorOffsetsNow.Load()...) }
