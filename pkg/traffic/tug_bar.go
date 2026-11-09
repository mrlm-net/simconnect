package traffic

import (
	"strings"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// GSX's stick tugs (FSDT_Pushback_Trepel_700 and the like) are a towbar
// model with the tug hung on the bar's far end (FSDT_TowbarAngle), turned
// by WAGON BACK LINK ORIENTATION: 0 to 180, TugBarStraight with the tug in
// line behind the bar. Never written it is 0: the tug stood across the bar,
// over an E195's nose at the stand (live LKPR, OE-LWG) and crabbing sideways
// on the roads. The model origin is the bar's hook end, which goes on the
// nose wheel (TugAheadMeters); the tug's rear axle is stickTugs' rearAxleM
// behind it, the point that tracks its way when it drives.
const TugBarStraight = 90.0

// The bar's angle as the tug turns on it: TugBarStraight + TugBarSign x the
// tug's heading off the bar's (degrees clockwise). Frame 0 hangs the tug on
// the model's left (+x in the glTF, the side of its SX wheels), facing right:
// 90° clockwise. The tug turns at most TugMaxTugDeg off the bar, is written
// again after TugBarResendDeg, and swings back in line over about
// TugBarEaseSeconds once off the nose.
const (
	TugBarSign        = -1.0
	TugMaxTugDeg      = 80.0
	TugBarResendDeg   = 0.1
	TugBarEaseSeconds = 1.5
)

// stickTug is a stick tug's model, measured from the GSX glTF models with
// the bar straight (2026-11-08): rearAxleM is how far its rear axle is
// behind the model origin, barM its bar's pivot (FSDT_TowbarAngle).
type stickTug struct {
	rearAxleM, barM float64
}

// stickTugs by title family (a livery title, FSDT_Pushback_Trepel_700_AUA,
// is its family's).
var stickTugs = map[string]stickTug{
	"FSDT_Pushback_Trepel_700": {rearAxleM: 11.04, barM: 4.35},
	"FSDT_Pushback_Trepel_280": {rearAxleM: 9.93, barM: 5.37},
	"FSDT_Pushback_TMX_150":    {rearAxleM: 9.43, barM: 5.37},
	"FSDT_Pushback_M1A":        {rearAxleM: 5.53, barM: 3.43},
}

// stickTugFor is the stick tug of title, false for any other tug.
func stickTugFor(title string) (stickTug, bool) {
	t := strings.ToUpper(title)
	for k, s := range stickTugs {
		k = strings.ToUpper(k)
		if t == k || strings.HasPrefix(t, k+"_") {
			return s, true
		}
	}
	return stickTug{}, false
}

// SetTowbarAngle writes a stick tug's towbar angle (WAGON BACK LINK
// ORIENTATION; TugBarStraight in line).
func (i *Injector) SetTowbarAngle(objectID uint32, v float64) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if o, ok := i.objects[objectID]; !ok || !o.taken {
		return ErrNotInjected
	}
	x := [1]float64{v}
	return i.track("towbar", i.client.SetDataOnSimObject(i.defBase+injDefTowbar, objectID, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, uint32(unsafe.Sizeof(x)), unsafe.Pointer(&x)))
}
