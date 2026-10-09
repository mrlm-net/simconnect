package flight

import (
	"math"
	"slices"
)

// Learned is how a type is flown, learned from recorded flights (#966):
// the take-off, the climb-out, the configuration on the approach and the
// landing. Speeds knots, heights feet above the ground, pitch degrees nose
// up, vertical speed feet per minute. A value no flight showed is 0 (nil
// for the maps). Each is the median of the flights that show it.
type Learned struct {
	Model   string `json:"model"`
	Flights int    `json:"flights"`

	RotateKts     float64 `json:"rotateKts,omitempty"`     // the nose starts up
	LiftoffKts    float64 `json:"liftoffKts,omitempty"`    // the wheels leave
	RotatePitch   float64 `json:"rotatePitch,omitempty"`   // the pitch at lift-off
	ClimbPitch    float64 `json:"climbPitch,omitempty"`    // the initial climb, lift-off to 1000 ft
	GearUpAGLFt   float64 `json:"gearUpAglFt,omitempty"`   // the gear handle up
	AccelAGLFt    float64 `json:"accelAglFt,omitempty"`    // the first flap retraction
	ApproachKts   float64 `json:"approachKts,omitempty"`   // 1000 to 200 ft on final
	GearDownAGLFt float64 `json:"gearDownAglFt,omitempty"` // the gear handle down on the approach
	FlareAGLFt    float64 `json:"flareAglFt,omitempty"`    // the nose comes up before touchdown
	TouchdownFpm  float64 `json:"touchdownFpm,omitempty"`  // the sink rate at touchdown (negative)
	// FlapsUpKts[k]: the speed the lever left detent k for k−1, climbing;
	// FlapsDownKts[k]: the speed it reached detent k, on the approach;
	// FlapsDownAGLFt[k] the height then.
	FlapsUpKts     map[int]float64 `json:"flapsUpKts,omitempty"`
	FlapsDownKts   map[int]float64 `json:"flapsDownKts,omitempty"`
	FlapsDownAGLFt map[int]float64 `json:"flapsDownAglFt,omitempty"`
}

// agl is s's wheels above the ground.
func agl(s Sample) float64 { return s.AltFt - s.GroundFt - s.CGFt }

// flown is what one Track shows; the zero of a value: not shown.
type flown struct {
	rotate, liftoff, rotatePitch, climbPitch, gearUp, accel float64
	approach, gearDown, flare, touchdown                    float64
	flapsUp, flapsDown, flapsDownAGL                        map[int]float64
}

// Learn learns how model is flown from tracks (those of other models are
// skipped; "" takes every track).
func Learn(model string, tracks ...*Track) Learned {
	l := Learned{Model: model}
	var fs []flown
	for _, t := range tracks {
		if t == nil || len(t.Samples) < 10 || (model != "" && t.Model != model) {
			continue
		}
		fs = append(fs, learnTrack(t.Samples))
	}
	l.Flights = len(fs)
	pick := func(get func(flown) float64) float64 {
		var vs []float64
		for _, f := range fs {
			if v := get(f); v != 0 {
				vs = append(vs, v)
			}
		}
		return median(vs)
	}
	pickMap := func(get func(flown) map[int]float64) map[int]float64 {
		by := map[int][]float64{}
		for _, f := range fs {
			for k, v := range get(f) {
				by[k] = append(by[k], v)
			}
		}
		if len(by) == 0 {
			return nil
		}
		out := map[int]float64{}
		for k, vs := range by {
			out[k] = median(vs)
		}
		return out
	}
	l.RotateKts = pick(func(f flown) float64 { return f.rotate })
	l.LiftoffKts = pick(func(f flown) float64 { return f.liftoff })
	l.RotatePitch = pick(func(f flown) float64 { return f.rotatePitch })
	l.ClimbPitch = pick(func(f flown) float64 { return f.climbPitch })
	l.GearUpAGLFt = pick(func(f flown) float64 { return f.gearUp })
	l.AccelAGLFt = pick(func(f flown) float64 { return f.accel })
	l.ApproachKts = pick(func(f flown) float64 { return f.approach })
	l.GearDownAGLFt = pick(func(f flown) float64 { return f.gearDown })
	l.FlareAGLFt = pick(func(f flown) float64 { return f.flare })
	l.TouchdownFpm = pick(func(f flown) float64 { return f.touchdown })
	l.FlapsUpKts = pickMap(func(f flown) map[int]float64 { return f.flapsUp })
	l.FlapsDownKts = pickMap(func(f flown) map[int]float64 { return f.flapsDown })
	l.FlapsDownAGLFt = pickMap(func(f flown) map[int]float64 { return f.flapsDownAGL })
	return l
}

func median(vs []float64) float64 {
	if len(vs) == 0 {
		return 0
	}
	vs = slices.Clone(vs)
	slices.Sort(vs)
	n := len(vs)
	if n%2 == 1 {
		return vs[n/2]
	}
	return (vs[n/2-1] + vs[n/2]) / 2
}

// learnTrack reads one flight: the first take-off and the last landing.
func learnTrack(ss []Sample) flown {
	f := flown{flapsUp: map[int]float64{}, flapsDown: map[int]float64{}, flapsDownAGL: map[int]float64{}}
	lift, touch := -1, -1
	for i := 1; i < len(ss); i++ {
		if lift < 0 && ss[i-1].OnGround && !ss[i].OnGround && ss[i].IAS > 40 {
			lift = i
		}
		if lift >= 0 && !ss[i-1].OnGround && ss[i].OnGround {
			touch = i
		}
	}
	if lift > 0 {
		learnTakeoff(ss, lift, &f)
	}
	if touch > 0 && touch > lift {
		learnLanding(ss, lift, touch, &f)
	}
	return f
}

// learnTakeoff: the rotation (the nose 1° above the roll's pitch), the
// lift-off, the initial climb pitch, the gear up, the flaps up.
func learnTakeoff(ss []Sample, lift int, f *flown) {
	f.liftoff, f.rotatePitch = ss[lift].IAS, ss[lift].Pitch
	// The roll's pitch: the median of the last 20 s on the ground.
	start := lift - 1
	for start > 0 && ss[start].OnGround && ss[lift].T-ss[start].T < 20 {
		start--
	}
	var roll []float64
	for i := start; i < lift; i++ {
		roll = append(roll, ss[i].Pitch)
	}
	base := median(roll)
	for i := lift - 1; i > start; i-- {
		if ss[i].Pitch <= base+1 {
			f.rotate = ss[min(i+1, lift)].IAS
			break
		}
	}
	var climb []float64
	gearDone := false
	for i := lift; i < len(ss) && agl(ss[i]) < 10000; i++ {
		s := ss[i]
		if a := agl(s); a > 100 && a < 1000 {
			climb = append(climb, s.Pitch)
		}
		if !gearDone && i > lift && ss[i-1].GearHandle && !s.GearHandle {
			f.gearUp, gearDone = agl(s), true
		}
		if i > lift && s.FlapsIndex < ss[i-1].FlapsIndex {
			from := ss[i-1].FlapsIndex
			if _, ok := f.flapsUp[from]; !ok {
				f.flapsUp[from] = s.IAS
			}
			if f.accel == 0 {
				f.accel = agl(s)
			}
		}
		if s.OnGround || s.VS < -500 { // landed, or the climb-out over
			break
		}
	}
	f.climbPitch = median(climb)
}

// learnLanding: from the last time the aircraft came below 10,000 ft
// before touchdown, the flaps and gear going down, the final approach
// speed, the flare (where the pitch rose over the final's) and the sink rate.
func learnLanding(ss []Sample, lift, touch int, f *flown) {
	from := touch - 1
	for from > lift && agl(ss[from]) < 10000 && !ss[from].OnGround {
		from--
	}
	gearSeen := false
	var final, finalPitch []float64
	for i := from + 1; i <= touch; i++ {
		s, p := ss[i], ss[i-1]
		if s.FlapsIndex > p.FlapsIndex {
			f.flapsDown[s.FlapsIndex], f.flapsDownAGL[s.FlapsIndex] = s.IAS, agl(s)
		}
		if !gearSeen && s.GearHandle && !p.GearHandle {
			f.gearDown, gearSeen = agl(s), true
		}
		if a := agl(s); a <= 1000 && a >= 200 && !s.OnGround {
			final = append(final, s.IAS)
			finalPitch = append(finalPitch, s.Pitch)
		}
	}
	f.approach = median(final)
	f.touchdown = ss[touch-1].VS
	base := median(finalPitch)
	if len(finalPitch) > 0 {
		for i := touch - 1; i > from; i-- {
			if a := agl(ss[i]); a > 200 {
				break
			}
			if ss[i].Pitch <= base+0.5 { // where the nose started up
				if i+1 < touch {
					f.flare = math.Max(0, agl(ss[i]))
				}
				break
			}
		}
	}
}
