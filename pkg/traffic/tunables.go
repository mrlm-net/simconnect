//go:build windows
// +build windows

package traffic

import "time"

// Taxi speeds in knots.
// Expedited (#510): the waits shrink to RushDelayFactor, an arrival
// leaves the runway RushExitKts faster.
const (
	RushDelayFactor = 0.35
	RushExitKts     = 6.0
)

const (
	// PushbackSpeedKts matches a pushback tug's walking pace.
	PushbackSpeedKts = 3.0
	// TaxiSpeedKts is the straight-line taxi speed; airline SOPs use 10–20 kt.
	TaxiSpeedKts = 15.0
	// TurnSpeedKts applies before turns of TurnAngleDeg or more.
	TurnSpeedKts = 8.0
	// SharpTurnSpeedKts applies before turns of SharpTurnAngleDeg or more.
	SharpTurnSpeedKts = 5.0
	// HoldShortApproachSpeedKts is used over the last HoldShortApproachMeters.
	HoldShortApproachSpeedKts = 5.0
	// LineUpSpeedKts is used entering the runway.
	LineUpSpeedKts = 6.0
)

// Taxi geometry.
const (
	// TurnAngleDeg and SharpTurnAngleDeg are heading changes at a route point
	// that call for TurnSpeedKts and SharpTurnSpeedKts.
	TurnAngleDeg      = 30.0
	SharpTurnAngleDeg = 60.0
	// MaxWaypointSpacingMeters splits longer route segments so the AI keeps to
	// the taxiway centreline on long straights.
	MaxWaypointSpacingMeters = 40.0
	// SimplifyAngleDeg is the smallest bend kept as a waypoint; straighter
	// route points are dropped so the AI can reach taxi speed.
	SimplifyAngleDeg = 3.0
	// TurnInMeters is the minimum distance from the taxiway junction to the
	// first forward waypoint after pushback. The aircraft leaves the pushback
	// facing the stand and turns onto the taxiway; a closer waypoint makes the
	// AI circle to reach it.
	TurnInMeters = 50.0
	// HoldShortApproachMeters is the distance before the hold-short point over
	// which the aircraft slows to HoldShortApproachSpeedKts.
	HoldShortApproachMeters = 60.0
	// HoldShortArrivalMeters is how close to the hold-short point the aircraft
	// must stop to count as holding short.
	HoldShortArrivalMeters = 25.0
	// StoppedKts is the ground speed below which the aircraft counts as stopped.
	StoppedKts = 1.5
	// LineUpAlignMeters is how far down the runway from the entry point the
	// line-up waypoint lies, so the aircraft is aligned before the take-off roll.
	LineUpAlignMeters = 80.0
)

// StuckTimeout is how long a taxiing aircraft may stand still away from the
// hold-short point before the controller reports ErrTaxiStuck.
const StuckTimeout = 90 * time.Second

// Default SimConnect IDs used by a TaxiController. Each controller uses
// taxiDefinitionCount definition IDs and taxiRequestCount request IDs from its
// bases; give concurrent controllers distinct bases with TaxiWithIDs.
const (
	DefaultTaxiDefinitionBase uint32 = 7300
	DefaultTaxiRequestBase    uint32 = 7400
	taxiDefinitionCount              = 2
	taxiRequestCount                 = 4
)

// Arrival profile. Measured in MSFS 2024 (#289, #301): an FSLTL A320 with its
// gear down touched down 580–650 m past the threshold at about 130 kt and
// slowed below 35 kt within about 1,500 m.
const (
	// DefaultSpawnNm is the distance out on final where arrivals spawn.
	DefaultSpawnNm = 5.0
	// GlidePathFtPerNm is a 3° glide path: 318 ft per nautical mile.
	GlidePathFtPerNm = 318.0
	// ThresholdCrossingFt is the height over the threshold.
	ThresholdCrossingFt = 50.0
	// ApproachSpeedKts is requested on final. MSFS AI ignores airborne speed
	// requests (it flew about 164 kt); kept for when it does not.
	ApproachSpeedKts = 135.0
	// TouchdownMeters and TouchdownSpeedKts place the touchdown waypoint.
	TouchdownMeters   = 300.0
	TouchdownSpeedKts = 125.0
	// RolloutDecel is the planned braking deceleration in m/s² (autobrake low
	// to medium), and RolloutSpacingMeters the spacing of rollout waypoints.
	RolloutDecel         = 1.5
	RolloutSpacingMeters = 250.0
	// ExitHighSpeedKts and ExitSpeedKts are the speeds for turning off onto a
	// high-speed (≤ 45°) or a standard exit.
	ExitHighSpeedKts = 25.0
	ExitSpeedKts     = 10.0
	// StandApproachSpeedKts is the speed along the stand's PARKING path, and
	// StandOvershootMeters how far past the stand the last waypoint lies:
	// MSFS AI stops short of its last ground waypoint (46 m in #289).
	StandApproachSpeedKts = 3.0
	StandOvershootMeters  = 10.0
	// StandTaxiSpeedKts is the speed on the PARKING path until
	// StandSlowMeters before the stand; StandStopMeters is how close to the
	// stand the controller stops the aircraft with a single waypoint at its
	// current position (the AI otherwise rolls on towards the overshoot point).
	StandTaxiSpeedKts = 5.0
	StandSlowMeters   = 30.0
	StandStopMeters   = 2.0
	// ParkedMeters is how close to the stand a stopped aircraft counts as parked.
	ParkedMeters = 15.0
)

// Default SimConnect IDs used by an ArrivalController: 4 definition IDs and
// 4 request IDs from these bases.
const (
	DefaultArrivalDefinitionBase uint32 = 7500
	DefaultArrivalRequestBase    uint32 = 7600
)

// Exit shaping, tuned after the first live arrivals (#295): closely spaced
// exit waypoints made the AI overshoot one and loop back.
const (
	// MinWaypointSpacingMeters drops taxi-in route points closer than this to
	// the previous waypoint (except the stand approach).
	MinWaypointSpacingMeters = 30.0
	// RunwayClearMeters is how far beyond the runway half-width an aircraft
	// must be to count as off the runway.
	RunwayClearMeters = 10.0
)

// DefaultNoseOffsetMeters is the distance from an aircraft's reference point
// to its nose, used to stop on a stand with the nose at the front of the
// parking circle. About right for an A320 family aircraft.
const DefaultNoseOffsetMeters = 17.0

// Stop detection: MSFS AI keeps reporting its last commanded ground speed
// after it has stopped (#295), so "stopped" means the position moved less
// than StationaryMeters over StationarySeconds.
const (
	StationaryMeters  = 1.0 // MSFS AI creeps at ~0.4 kt towards its last waypoint
	StationarySeconds = 3
)

// After landing: the aircraft rolls clear of the runway, stops, switches
// to taxi lights and waits for taxi clearance (#295).
const (
	// VacateOffsetMeters is how far from the runway centreline the vacate
	// stop lies when no hold-short is on the route (runway holding positions
	// are typically 60–90 m from the centreline).
	VacateOffsetMeters = 100.0
	// VacateStopKts is the speed over the last meters before the vacate stop.
	VacateStopKts = 5.0
	// VacateArriveMeters is how close to the vacate stop (along the route) a
	// stopped aircraft counts as arrived there.
	VacateArriveMeters = 40.0
	// DefaultAfterLandingDwell is how long the aircraft waits clear of the
	// runway before taxiing on when HoldForClearance is off.
	DefaultAfterLandingDwell = 15 * time.Second
)

// Injected ground movement (#309), see MotionProfile for the per-aircraft
// values.
const (
	// GroundPathSmoothingPasses rounds route corners (Chaikin passes).
	GroundPathSmoothingPasses = 6
	// TurnWindowMeters is the distance either side of a point over which the
	// turn radius is measured.
	TurnWindowMeters = 8.0
	// TurnLookaheadMeters: the aircraft is already at a turn's speed this far
	// before it.
	TurnLookaheadMeters = 10.0
	// SpeedResponseSeconds is how quickly the speed chases the planned speed.
	SpeedResponseSeconds = 2.0
	// StopApproachMeters is the distance before a stop point from which the
	// mover brakes exactly onto it.
	StopApproachMeters = 40.0
	// CornerMeters bounds how far from a route corner the rounding starts.
	CornerMeters = 25.0
	// MergeMeters merges route points closer than this before rounding.
	MergeMeters = 8.0
	// InjectHz is the recommended rate for Injector.Place; 60 Hz looked
	// smooth live, 30 Hz is acceptable.
	InjectHz = 60
)

// EngineStartTime is how long one engine takes to start; they start one
// after the other once the tug has gone, before the taxi.
const EngineStartTime = 30 * time.Second

// Default SimConnect IDs used by an Injector: 9 definition IDs, 2 request
// IDs per aircraft (up to injectMaxAircraft: aircraft and their tugs, 40+
// aircraft at once, #370) and injectEventCount event IDs.
const (
	DefaultInjectDefinitionBase uint32 = 7700
	DefaultInjectRequestBase    uint32 = 7800
	DefaultInjectEventBase      uint32 = 7900
	injectMaxAircraft                  = 96
)

// Hybrid arrival (ArrivalWithInjector): the injector takes over on the
// runway once the aircraft has been on the ground TakeoverAfterTouchdown and
// slowed to TakeoverKts, at least TakeoverBeforeExitMeters before the exit
// (otherwise clear of the runway), and drives the rest of the rollout at
// RolloutDecel down to the exit speed.
const (
	TakeoverKts              = 70.0
	TakeoverAfterTouchdown   = 2 * time.Second
	TakeoverBeforeExitMeters = 150.0
	// RolloutJerk (m/s³) lets runway braking build up quickly.
	RolloutJerk = 0.6
)

// Injected lights after landing: the taxi light comes on TaxiLightDelay
// after the landing lights go off at the vacate stop; crossing lights stay
// on until the main gear is CrossingTailMeters past the far hold-short line,
// a moment after the tail has cleared it.
const (
	TaxiLightDelay     = 1500 * time.Millisecond
	CrossingTailMeters = 40.0
	// CrossingOnMeters: the crossing lights come on once the nose gear is
	// this far past the first hold-short line.
	CrossingOnMeters = 10.0
)

// Variation after landing (injected arrivals): the wait clear of the runway
// varies by ±DwellJitter, and with DefaultRollThroughChance the aircraft
// only slows to RollThroughKts at the vacate point
// and taxis on (a rolling clearance).
const (
	DwellJitter              = 0.1
	DefaultRollThroughChance = 0.3
	RollThroughKts           = 0.5
)

// HoldShortStopMeters is how far before a hold-short line an injected
// aircraft stops its nose gear, so the nose stays behind the line.
const HoldShortStopMeters = 7.0

// FlapsRetractSeconds is how long an injected arrival takes to retract its
// flaps once clear of the runway.
const FlapsRetractSeconds = 20.0

// Injected landing (InjectApproach): ground spoilers deploy over
// SpoilerDeploySeconds at main gear touchdown and stow once clear of the
// runway.
const (
	SpoilerDeploySeconds = 1.0
)

// Injected approach flaps: ApproachFlapsPct (flaps 3 on an A320) from the
// start, running to full over FlapsFullSeconds when passing FlapsFullFt,
// the stabilised-approach gate.
const (
	ApproachFlapsPct = 75.0
	FlapsFullFt      = 1400.0
	FlapsFullSeconds = 5.0
)

// Injected departure gates (TaxiWithInjector, without HoldForClearances):
// each clears itself after about these waits (±DwellJitter). The beacon
// comes on BeaconLeadTime before the push; the tail is pushed
// PushTailMeters past a wheelbase beyond the taxiway junction. The gear
// comes up on a positive climb: above GearUpFt, GearUpDelaySeconds after
// lift-off and climbing at GearUpFpm or more. The aircraft is handed to
// MSFS AI for the climb-out at ClimbHandoverFt, or at the airport's
// (TaxiRequest.Airport, airport.LimitsFor).
const (
	PushbackDelay      = 5 * time.Second
	BeaconLeadTime     = 3 * time.Second
	TaxiAfterPushDelay = 15 * time.Second
	LineUpDelay        = 6 * time.Second
	TakeoffDelay       = 5 * time.Second
	PushTailMeters     = 20.0
	// The pushback is fitted to each stand (pushPlan): the main gear turns
	// on the widest arc up to PushbackArcMeters that the distances and the
	// neighbouring stands allow, but not tighter than PushbackMinArcMeters;
	// it pushes at least PushStraightMeters straight first and ends
	// PushAlignMeters along the taxiway after the arc.
	PushbackArcMeters    = 45.0
	PushbackMinArcMeters = 14.0
	PushStraightMeters   = 6.0
	PushAlignMeters      = 10.0
	GearUpFt             = 50.0
	GearUpDelaySeconds   = 4.5
	GearUpFpm            = 500.0
	ClimbHandoverFt      = 1500.0
)

// Injected departure configuration: take-off flaps (TakeoffFlapsPct, 1+F on
// an A320) set over FlapsSetSeconds when the taxi starts, retracted from
// FlapsRetractFt over FlapsRetractClimbSeconds; DefaultRollingTakeoffChance
// of departures without held gates roll straight into the take-off.
const (
	TakeoffFlapsPct = 25.0
	FlapsSetSeconds = 8.0
	FlapsRetractFt  = 1000.0
	// HandoverCleanMarginFt: a take-off is handed to MSFS AI with the
	// flaps up, at most this far above the hand-over height.
	HandoverCleanMarginFt       = 2500.0
	FlapsRetractClimbSeconds    = 10.0
	DefaultRollingTakeoffChance = 0.3
)

// TurnAroundMeters scales the turn-around loop onto a self-manoeuvring
// (face-out) stand; an A320 turns on about 15-20 m radius.
const TurnAroundMeters = 18.0

// Stand allocation (StandAllocator). DefaultHalfSpanMeters is half an A320's
// span; two aircraft on overlapping stands need StandWingtipClearanceMeters
// between their wingtips. A scanned aircraft on the ground below
// StandDetectKts holds the stand it stands on. Assign routes the
// standRankCandidates stands nearest the runway to rank them by taxi-in.
const (
	DefaultStandDefinitionBase uint32 = 8200
	DefaultStandRequestBase    uint32 = 8300

	DefaultHalfSpanMeters       = 17.9
	StandWingtipClearanceMeters = 3.0
	StandDetectKts              = 2.0
	standRankCandidates         = 12
	// StandSpread and StandSpreadMeters: stands whose taxi-in is within
	// StandSpread of the best plus StandSpreadMeters are picked at random.
	StandSpread       = 0.4
	StandSpreadMeters = 300.0
)

// Pushback tug (SimObjectTug). The tug's reference point sits TugAheadMeters
// ahead of the aircraft's nose gear, turned TugYawDeg from the aircraft
// heading (180: facing the aircraft, towbar to the nose wheel). After the
// push it waits TugDisconnectSeconds, backs TugBackOffMeters away from the
// nose, then drives off turning TugDriveOffTurnDeg for TugDriveOffMeters at
// up to TugDriveOffKts and is removed. DefaultTugTitle is GSX's classic
// towbar tug, checked live against an A320.
const (
	DefaultTugTitle      = "FSDT_Pushback_03"
	TugAheadMeters       = 3.0
	TugYawDeg            = 180.0
	TugDisconnectSeconds = 8.0
	TugBackOffMeters     = 15.0 // more than the mover's 10 m look-ahead, or it never starts
	TugDriveOffMeters    = 35.0
	TugDriveOffTurnDeg   = 90.0
	TugDriveOffKts       = 8.0
	TugMaxBarDeg         = 80.0 // tow bar angle off the aircraft axis, at most
	TugBarSeconds        = 0.6  // how quickly the bar follows the nose wheel
	// From and to its depot (SimObjectTug.Layout): on the vehicle roads at
	// TugRoadKts, the last TugApproachMeters straight onto the nose gear.
	TugRoadKts        = 15.0
	TugApproachMeters = 12.0
	// TugArriveTimeout: a tug not at the nose this long after it was sent
	// for is given up.
	TugArriveTimeout = 4 * time.Minute
	// TugCreateTimeout: a tug the simulator has not created this long after
	// it was asked for (a model it does not have) is given up.
	TugCreateTimeout = 20 * time.Second
)

// Take-off pitch (TakeoffMover): on the runway the pitch stays
// TailstrikeMarginDeg below TakeoffProfile.TailstrikePitch. After lift-off
// it is held until the main wheels are PositiveClimbFt up, then may rise
// one degree per TailClearFtPerDeg of height (the tail rises with the
// aircraft) up to the climb pitch.
const (
	TailstrikeMarginDeg = 2.0
	PositiveClimbFt     = 15.0
	TailClearFtPerDeg   = 3.0
)

// Ground traffic (GroundPicture, #334): a taxiing aircraft looks
// TrafficLookMeters ahead along its path for another aircraft's body within
// its wingspan corridor and stops with its nose TrafficGapMeters behind it.
// Reports older than TrafficStaleAfter are ignored; bodies are sampled every
// trafficBodyStep meters.
const (
	TrafficLookMeters = 200.0
	// TrafficBrakeFactor: braking for traffic found closer than a normal
	// stop is up to this many times the profile's deceleration (and jerk);
	// TrafficOverrunMeters how far into the gap behind it the aircraft may
	// still roll, braking, rather than stop dead.
	TrafficBrakeFactor   = 3.0
	TrafficJerkFactor    = 6.0
	TrafficOverrunMeters = 8.0
	TrafficGapMeters     = 15.0
	TrafficStaleAfter    = 3 * time.Second
	trafficBodyStep      = 5.0
	// TrafficCheckEvery is how often a taxiing aircraft looks ahead.
	TrafficCheckEvery = 100 * time.Millisecond
	// GiveWayLookMeters is how far ahead a taxiing aircraft reports where
	// it will drive (up to its next stop) and checks for routes crossing
	// or merging with its own; GiveWayMarginMeters is added to both
	// half-spans for the paths to conflict.
	GiveWayLookMeters   = 250.0
	GiveWayMarginMeters = 10.0
	// junctionBranchMeters is how much of a junction's other branches an
	// aircraft facing oncoming traffic keeps clear (#444).
	junctionBranchMeters = 60.0
	// PushClearMarginMeters: a pushback waits (or stops) while another
	// aircraft's fuselage is within the half-span plus this of the corridor
	// the push sweeps, or while its taxi path comes within both half-spans
	// plus GiveWayMarginMeters of it.
	PushClearMarginMeters = 3.0
)

// TakeoffRunMargin lengthens the distance to 35 ft into the runway an
// aircraft needs (RequiredTakeoffRun): the 115% of certified take-off
// distances.
const TakeoffRunMargin = 1.15
