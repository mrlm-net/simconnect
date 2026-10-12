package camera

// MoveInfo describes one of the drone moves (Moves): its name, what it
// shows (its doc), wide or detail, and the function that makes the shot.
type MoveInfo struct {
	Name string   `json:"name"`
	Doc  string   `json:"doc"`
	Kind Kind     `json:"kind"`
	Func MoveFunc `json:"-"`
}

// Moves are the drone moves on an aircraft, for a shot designer to list
// rather than know them (TrackZoom is not one: it takes an eye point).
func Moves() []MoveInfo {
	return []MoveInfo{
		{"RevealRise", "Low behind the aircraft, rising over it as it comes into view.", Wide, RevealRise},
		{"Flyover", "The drone passes over the aircraft from nose to tail, looking down at it.", Wide, Flyover},
		{"SpiralDescend", "Circling a quarter of the way round while dropping from high and wide to low and close.", Wide, SpiralDescend},
		{"LeadChase", "Ahead of the aircraft flying backwards, sliding to its side as it closes on the nose.", Wide, LeadChase},
		{"ParallaxTrack", "Alongside at a distance, overtaking it slowly: the background slides behind it.", Wide, ParallaxTrack},
		{"HeroLowPushIn", "Low at the front quarter, pushing in and narrowing: the aircraft looms.", Wide, HeroLowPushIn},
		{"PullBackReveal", "From close on the engine back and up to the whole aircraft.", Wide, PullBackReveal},
		{"TopDown", "High above, looking almost straight down, drifting forward.", Wide, TopDown},
		{"ChaseRise", "Behind the aircraft climbing out, rising and falling back.", Wide, ChaseRise},
		{"TopOrbit", "High above, looking down, turning slowly round: the tug coming in under the nose, the push starting.", Wide, TopOrbit},
		{"SideDolly", "Alongside at the aircraft's own speed, low, edging forward along it: the take-off roll, the landing.", Wide, SideDolly},
		{"HeadOnPass", "A drone flying at the aircraft head-on, low, and up over it at the last moment.", Wide, HeadOnPass},
		{"EngineDetail", "Close on the engine on side, drifting along it.", Detail, EngineDetail},
		{"NoseGearDetail", "Low by the nose gear.", Detail, NoseGearDetail},
		{"CockpitDetail", "Beside the cockpit windows, easing round them.", Detail, CockpitDetail},
		{"TailDetail", "Up by the fin and the livery.", Detail, TailDetail},
		{"WingtipAlong", "By the wingtip, looking along the wing to the fuselage.", Detail, WingtipAlong},
		{"MainGearDetail", "Low by the main gear, the wheels rolling.", Detail, MainGearDetail},
		{"LightsDetail", "Low ahead of the nose, the landing and taxi lights coming on towards the camera.", Detail, LightsDetail},
	}
}

// String names a Kind: "wide" or "detail".
func (k Kind) String() string {
	if k == Detail {
		return "detail"
	}
	return "wide"
}

// MarshalText writes a Kind as its name.
func (k Kind) MarshalText() ([]byte, error) { return []byte(k.String()), nil }
