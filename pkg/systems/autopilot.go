//go:build windows

package systems

// The autopilot and the flight controls (#962), by generic name: values a
// profile reads, actions Controls takes. The default profile gives the
// standard SimVars and key events (MSFS SDK: Autopilot/Assistant
// Variables, Autopilot/Flight Assist, Flight Control, Engine and Landing
// Gear events); an add-on that keeps its own (the Fenix's FCU) overrides
// them by these names from its profile JSON.
const (
	// AP master, flight director, autothrust armed and active.
	APMaster   = "apMaster"
	FD         = "fd"
	ATHR       = "athr"
	ATHRActive = "athrActive"
	// Selected values: heading (magnetic), altitude (feet), vertical speed
	// (feet per minute), speed (knots), Mach.
	APHeadingSel  = "apHeadingSel"
	APAltitudeSel = "apAltitudeSel"
	APVSSel       = "apVSSel"
	APSpeedSel    = "apSpeedSel"
	APMachSel     = "apMachSel"
	// Managed (pushed, the FMS's) rather than selected (pulled): heading,
	// speed, altitude, vertical speed. As actions, on pushes, off pulls.
	APHeadingManaged  = "apHeadingManaged"
	APSpeedManaged    = "apSpeedManaged"
	APAltitudeManaged = "apAltitudeManaged"
	APVSManaged       = "apVSManaged"
	// Modes engaged: heading, altitude hold, vertical speed, level change
	// (open climb/descent), speed, Mach, NAV (LNAV), LOC, approach,
	// glideslope; armed: approach, glideslope, altitude capture. As
	// actions, on engages (arms LOC, APPR), off disengages.
	APHeadingHold   = "apHeadingHold"
	APAltitudeHold  = "apAltitudeHold"
	APVSHold        = "apVSHold"
	APFLC           = "apFLC"
	APSpeedHold     = "apSpeedHold"
	APMachHold      = "apMachHold"
	APNav           = "apNav"
	APLoc           = "apLoc"
	APApproach      = "apApproach"
	APGlideslope    = "apGlideslope"
	APApproachArmed = "apApproachArmed"
	APGSArmed       = "apGSArmed"
	APAltitudeArmed = "apAltitudeArmed"
)

// Flight controls, actions for a pilot flying by hand (SetValue): the
// elevator, ailerons and rudder −100…100 (nose up, right roll, right yaw
// positive), the throttles 0…100 (all, or ThrottleN(n)), the flap lever
// 0…100 of its travel (the nearest detent), FlapsUp / FlapsDown one detent;
// the gear handle and the ground spoilers' arming are GearDown and
// SpoilersArmed (Set).
const (
	Elevator   = "elevator"
	Aileron    = "aileron"
	Rudder     = "rudder"
	Throttle   = "throttle"
	FlapsLever = "flapsLever"
	FlapsUp    = "flapsUp"
	FlapsDown  = "flapsDown"
	// Reversers: thrust reversers out on all engines (Set), read as any
	// engine's engaged; the throttles then set the reverse thrust.
	Reversers = "reversers"
	// BrakeLeft, BrakeRight: the wheel brakes 0…100 (SetValue).
	BrakeLeft  = "brakeLeft"
	BrakeRight = "brakeRight"
	// ReverseThrust: reverse thrust 0…100 (reverse idle to full) while the
	// reversers are out (SetValue); the default sets the throttles, as
	// they set reverse thrust then.
	ReverseThrust = "reverseThrust"
	// TOGA: take-off / go-around thrust (Press), the autothrottle's go-around
	// mode where it has one.
	TOGA = "toga"
)

// ThrottleN is engine n's (1–4) throttle, 0…100 (SetValue).
func ThrottleN(n int) string { return "throttle" + string(rune('0'+n)) }

// defaultAutopilot adds the autopilot's and flight controls' standard
// SimVars and key events to the default profile's values v and actions a.
func defaultAutopilot(v map[string]Value, a map[string]Action) {
	one := func(name, unit string) Value { return Value{Vars: []string{name}, Unit: unit} }
	// The slot indexes: 1 the panel's (selected), 2 the FMS's (managed), as
	// the stock Airbus uses them (to verify live).
	managed := func(name string) Value {
		return Value{Vars: []string{name}, Unit: "number", TrueAt: []float64{2}, Note: "slot 2: managed (assumed)"}
	}
	for k, x := range map[string]Value{
		APMaster:          one("AUTOPILOT MASTER", "bool"),
		FD:                one("AUTOPILOT FLIGHT DIRECTOR ACTIVE", "bool"),
		ATHR:              one("AUTOPILOT THROTTLE ARM", "bool"),
		ATHRActive:        one("AUTOPILOT MANAGED THROTTLE ACTIVE", "bool"),
		APHeadingSel:      one("AUTOPILOT HEADING LOCK DIR", "degrees"),
		APAltitudeSel:     one("AUTOPILOT ALTITUDE LOCK VAR", "feet"),
		APVSSel:           one("AUTOPILOT VERTICAL HOLD VAR", "feet per minute"),
		APSpeedSel:        one("AUTOPILOT AIRSPEED HOLD VAR", "knots"),
		APMachSel:         one("AUTOPILOT MACH HOLD VAR", "number"),
		APHeadingManaged:  managed("AUTOPILOT HEADING SLOT INDEX"),
		APSpeedManaged:    managed("AUTOPILOT SPEED SLOT INDEX"),
		APAltitudeManaged: managed("AUTOPILOT ALTITUDE SLOT INDEX"),
		APVSManaged:       managed("AUTOPILOT VS SLOT INDEX"),
		APHeadingHold:     one("AUTOPILOT HEADING LOCK", "bool"),
		APAltitudeHold:    one("AUTOPILOT ALTITUDE LOCK", "bool"),
		APVSHold:          one("AUTOPILOT VERTICAL HOLD", "bool"),
		APFLC:             one("AUTOPILOT FLIGHT LEVEL CHANGE", "bool"),
		APSpeedHold:       one("AUTOPILOT AIRSPEED HOLD", "bool"),
		APMachHold:        one("AUTOPILOT MACH HOLD", "bool"),
		APNav:             one("AUTOPILOT NAV1 LOCK", "bool"),
		APApproach:        one("AUTOPILOT APPROACH HOLD", "bool"),
		APGlideslope:      one("AUTOPILOT GLIDESLOPE HOLD", "bool"),
		APApproachArmed:   one("AUTOPILOT APPROACH ARM", "bool"),
		APGSArmed:         one("AUTOPILOT GLIDESLOPE ARM", "bool"),
		APAltitudeArmed:   one("AUTOPILOT ALTITUDE ARM", "bool"),
	} {
		v[k] = x
	}
	one2, two := 1.0, 2.0
	slot := func(event string) Action {
		return Action{Event: event, On: &two, Off: &one2, Note: "slot 2 managed, 1 selected (assumed)"}
	}
	onOff := func(on, off string) Action { return Action{Event: on, OffEvent: off} }
	value := func(event string, scale float64) Action { return Action{Event: event, Value: true, Scale: &scale} }
	for k, x := range map[string]Action{
		APMaster:          onOff("AUTOPILOT_ON", "AUTOPILOT_OFF"),
		FD:                {Event: "TOGGLE_FLIGHT_DIRECTOR", Toggle: true},
		ATHR:              {Event: "AUTO_THROTTLE_ARM", Toggle: true},
		TOGA:              {Event: "AUTO_THROTTLE_TO_GA"},
		ReverseThrust:     value("AXIS_THROTTLE_SET", 163.83),
		APHeadingSel:      value("HEADING_BUG_SET", 1),
		APAltitudeSel:     value("AP_ALT_VAR_SET_ENGLISH", 1),
		APVSSel:           value("AP_VS_VAR_SET_ENGLISH", 1),
		APSpeedSel:        value("AP_SPD_VAR_SET", 1),
		APMachSel:         value("AP_MACH_VAR_SET", 100), // Mach × 100
		APHeadingManaged:  slot("HEADING_SLOT_INDEX_SET"),
		APSpeedManaged:    slot("SPEED_SLOT_INDEX_SET"),
		APAltitudeManaged: slot("ALTITUDE_SLOT_INDEX_SET"),
		APVSManaged:       slot("VS_SLOT_INDEX_SET"),
		APHeadingHold:     onOff("AP_HDG_HOLD_ON", "AP_HDG_HOLD_OFF"),
		APAltitudeHold:    onOff("AP_ALT_HOLD_ON", "AP_ALT_HOLD_OFF"),
		APVSHold:          onOff("AP_VS_ON", "AP_VS_OFF"),
		APFLC:             onOff("FLIGHT_LEVEL_CHANGE_ON", "FLIGHT_LEVEL_CHANGE_OFF"),
		APSpeedHold:       onOff("AP_AIRSPEED_ON", "AP_AIRSPEED_OFF"),
		APMachHold:        onOff("AP_MACH_ON", "AP_MACH_OFF"),
		APNav:             onOff("AP_NAV1_HOLD_ON", "AP_NAV1_HOLD_OFF"),
		APLoc:             onOff("AP_LOC_HOLD_ON", "AP_LOC_HOLD_OFF"),
		APApproach:        onOff("AP_APR_HOLD_ON", "AP_APR_HOLD_OFF"),
		// Flight controls: axes −16383…16383, throttles and the flap lever
		// 0…16383, from the percentages SetValue takes.
		Elevator:      value("AXIS_ELEVATOR_SET", -163.83), // assumed: the axis positive pushed forward (to verify live)
		Aileron:       value("AXIS_AILERONS_SET", 163.83),
		Rudder:        value("AXIS_RUDDER_SET", 163.83),
		Throttle:      value("AXIS_THROTTLE_SET", 163.83),
		FlapsLever:    value("FLAPS_SET", 163.83),
		FlapsUp:       {Event: "FLAPS_DECR"},
		FlapsDown:     {Event: "FLAPS_INCR"},
		GearDown:      onOff("GEAR_DOWN", "GEAR_UP"),
		SpoilersArmed: onOff("SPOILERS_ARM_ON", "SPOILERS_ARM_OFF"),
	} {
		a[k] = x
	}
	for n := 1; n <= 4; n++ {
		a[ThrottleN(n)] = value("AXIS_THROTTLE"+string(rune('0'+n))+"_SET", 163.83)
	}
}

// AutopilotState is the autopilot as read (State.AP).
type AutopilotState struct {
	// Has: the profile reads the autopilot.
	Has                                                         bool
	Master, FD, ATHR                                            bool
	ATHRActive                                                  bool
	HeadingSel, AltitudeSel, VSSel, SpeedSel, MachSel           float64
	HeadingManaged, SpeedManaged, AltitudeManaged, VSManaged    bool
	HeadingHold, AltitudeHold, VSHold, FLC, SpeedHold, MachHold bool
	Nav, Loc, Approach, Glideslope                              bool
	ApproachArmed, GSArmed, AltitudeArmed                       bool
}

// autopilotOf reads the autopilot's values of s.
func autopilotOf(p Profile, s State) AutopilotState {
	on := func(k string) bool { return s.Values[k] != 0 }
	_, has := p.Values[APMaster]
	return AutopilotState{Has: has,
		Master: on(APMaster), FD: on(FD), ATHR: on(ATHR), ATHRActive: on(ATHRActive),
		HeadingSel: s.Values[APHeadingSel], AltitudeSel: s.Values[APAltitudeSel], VSSel: s.Values[APVSSel],
		SpeedSel: s.Values[APSpeedSel], MachSel: s.Values[APMachSel],
		HeadingManaged: on(APHeadingManaged), SpeedManaged: on(APSpeedManaged), AltitudeManaged: on(APAltitudeManaged), VSManaged: on(APVSManaged),
		HeadingHold: on(APHeadingHold), AltitudeHold: on(APAltitudeHold), VSHold: on(APVSHold), FLC: on(APFLC),
		SpeedHold: on(APSpeedHold), MachHold: on(APMachHold), Nav: on(APNav), Loc: on(APLoc), Approach: on(APApproach), Glideslope: on(APGlideslope),
		ApproachArmed: on(APApproachArmed), GSArmed: on(APGSArmed), AltitudeArmed: on(APAltitudeArmed),
	}
}

// defaultRollout adds the reversers and the wheel brakes: SET_REVERSE_THRUST_ON
// and _OFF for all engines, AXIS_LEFT/RIGHT_BRAKE_SET from −16383 (none) to
// +16383 (full) for 0…100 (SDK: Engine and Landing Gear / Brakes events).
func defaultRollout(v map[string]Value, a map[string]Action) {
	var revs []string
	for n := 1; n <= 4; n++ {
		revs = append(revs, fmtIndexed("GENERAL ENG REVERSE THRUST ENGAGED", n))
	}
	v[Reversers] = Value{Vars: revs, Unit: "bool", Combine: "any"}
	a[Reversers] = Action{Event: "SET_REVERSE_THRUST_ON", OffEvent: "SET_REVERSE_THRUST_OFF"}
	scale, offset := 327.66, -16383.0
	a[BrakeLeft] = Action{Event: "AXIS_LEFT_BRAKE_SET", Value: true, Scale: &scale, Offset: &offset}
	a[BrakeRight] = Action{Event: "AXIS_RIGHT_BRAKE_SET", Value: true, Scale: &scale, Offset: &offset}
}
