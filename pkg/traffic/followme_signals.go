package traffic

import (
	"errors"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// A GSX follow-me car's roof light and sign (FSDT_FollowMe_Hilux,
// FSDT_FollowMe_class_B, sim.cfg [Couatl]): the light bar is WAGON FRONT
// LINK ORIENTATION, its two lamps frames 1 and 2 ("both": 1 and 2 by
// turns, 500 ms each); the sign on its back WAGON BACK LINK ORIENTATION,
// 6 "FOLLOW ME". GSX's own program writes them; ours does, as it drives
// the car: the light flashing all the time it is out, the sign on while
// it waits for and leads the aircraft (the user, live LKPR 2026-11-11:
// "can also follow me use roof light?").
const (
	followMeFlashSeconds = 0.5
	followMeSignFollowMe = 6.0
)

// followMeSignals: the car's time out (s), and the light and sign last
// written (written: once at least).
type followMeSignals struct {
	t           float64
	light, sign float64
	written     bool
}

// signal writes the light and sign when they change.
func (f *SimObjectFollowMe) signal() error {
	light := 1.0
	if int(f.sig.t/followMeFlashSeconds)%2 == 1 {
		light = 2
	}
	sign := 0.0
	if f.phase == followMeWaiting || f.phase == followMeLeading {
		sign = followMeSignFollowMe
	}
	if f.sig.written && light == f.sig.light && sign == f.sig.sign {
		return nil
	}
	err := f.inj.SetFollowMeSignals(f.objectID, light, sign)
	if errors.Is(err, ErrNotInjected) {
		return nil
	}
	if err == nil {
		f.sig.light, f.sig.sign, f.sig.written = light, sign, true
	}
	return err
}

// SetFollowMeSignals writes a GSX follow-me car's roof light (WAGON FRONT
// LINK ORIENTATION) and sign (WAGON BACK LINK ORIENTATION).
func (i *Injector) SetFollowMeSignals(objectID uint32, light, sign float64) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if o, ok := i.objects[objectID]; !ok || !o.taken {
		return ErrNotInjected
	}
	x := [2]float64{light, sign}
	return i.track("follow-me signals", i.client.SetDataOnSimObject(i.defBase+injDefSignals, objectID, types.SIMCONNECT_DATA_SET_FLAG_DEFAULT, 0, uint32(unsafe.Sizeof(x)), unsafe.Pointer(&x)))
}
