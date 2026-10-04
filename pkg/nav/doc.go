// Package nav holds the navigation side of an airport environment:
// navigation data, weather and ATIS, flight plans.
//
// Weather comes from the simulator through [WeatherReader], or is set by the
// application with [StaticWeather]. [ActiveRunways] picks the runway ends in
// use from the wind and an airport's [airport.Layout], and [ATIS] turns both
// into an ICAO style broadcast, written ([ATIS.Text]) or with digits spelled
// for a voice ([ATIS.Spoken]). [ATISService] keeps the current broadcast and
// advances its letter when the runway, wind or QNH change significantly.
//
// [Plan] builds an IFR flight plan from two airports' layouts and
// procedures and the [AirwayGraph]: runways in use, SID, airways, STAR and
// approach, cruise level, vertical profile, time and fuel; [FlightPlan.PLN]
// writes it as an MSFS .pln file.
//
// SimConnect reports ambient weather at the user aircraft only, not per
// airport: the reader is accurate for the airport the user is at or near
// (the world centre of the traffic), and only approximate for others.
package nav
