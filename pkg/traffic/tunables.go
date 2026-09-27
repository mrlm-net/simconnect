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
