//go:build windows
// +build windows

package traffic

import "time"

// Taxi speeds in knots.
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
	ExitHighSpeedKts = 20.0
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

// Default SimConnect IDs used by an ArrivalController: 3 definition IDs and
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
