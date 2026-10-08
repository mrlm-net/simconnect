package traffic

import "reflect"

// withDefaults is given with each zero field taken from def: a request
// giving only some fields of a profile (MotionProfile, ApproachProfile,
// RolloutProfile, TakeoffProfile) keeps the defaults of the others. Given
// whole or not at all, it was: a profile with one field set flew with the
// rest at 0 (E34).
func withDefaults[T any](given, def T) T {
	out := given
	o, d := reflect.ValueOf(&out).Elem(), reflect.ValueOf(def)
	if o.Kind() != reflect.Struct {
		return given
	}
	for i := 0; i < o.NumField(); i++ {
		if f := o.Field(i); f.CanSet() && f.IsZero() {
			f.Set(d.Field(i))
		}
	}
	return out
}

// noteSent records which call send ID id belongs to, for exception reports,
// keeping the last sentKeep sends: the maps grew for the life of a
// controller (E32). Send IDs only increase. GetLastSentPacketID is the
// connection's, so a call made on another goroutine at the same moment can
// still take the label.
func noteSent(m map[uint32]string, id uint32, call string) {
	m[id] = call
	if len(m) <= 2*sentKeep {
		return
	}
	for k := range m {
		if id-k > sentKeep {
			delete(m, k)
		}
	}
}

// sentKeep: send IDs this far back are kept for exceptions (they arrive
// within a few sends).
const sentKeep = 512
