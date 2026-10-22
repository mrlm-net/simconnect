---
title: "Traffic Overview"
description: "Start here: what the traffic engine does, which package to use, and a map of every traffic page with a reading path from the World down to the motion of one aircraft."
order: 1
section: "traffic"
---

# Traffic Overview

The traffic engine fills Microsoft Flight Simulator airports with scheduled airline and general aviation traffic and works it with its own ATC: delivery, ground, tower, approach, departure and centre. Aircraft board at their stands, push back with a tug, taxi along real taxiway routes, take off on SIDs, fly STARs and approaches, land, vacate, taxi in and park, all spoken on the radio. It runs at every airport the simulator knows; generic rules do the work, and per-airport data (stations, stand rules, VFR points, intersections) lives in tables a host can override.

## Which package

| You want | Use | Start at |
|---|---|---|
| Traffic with its ATC, as the airport map runs it, in your own program | `pkg/traffic/world` | [Traffic World](traffic-world.md) |
| The same engine split: decisions on a server, motion beside the simulator | `traffic-director`, `traffic-actuator` | [Traffic World: Split](traffic-world.md#split-a-director-anywhere-an-actuator-beside-the-simulator) |
| Your own engine from the parts: timetables, controllers, sequencing | `pkg/traffic` | [Traffic Manager](traffic-manager.md) and the pages below |
| A single AI aircraft, no ATC | `pkg/traffic` (`Fleet`) | [Fleet & AI Objects](traffic-guide.md) |

Most hosts only need the World: create it, run it on its own connection or yours, turn on a schedule, and read its snapshot and transmissions. Everything below is what it does inside, and the parts it is built from.

## Reading path

**1. The engine.** [Traffic World](traffic-world.md): running it, options and hooks, the HTTP API, flights at a chosen time, real-world traffic, TCAS, the player beside the World's traffic, the runway in use and closures, offline data, ground services, the split and multiplayer.

**2. Where the flights come from.** [Traffic Manager](traffic-manager.md) turns a timetable into traffic: spawns, turnarounds and situation checks. [Traffic Schedules](traffic-schedules.md) builds the timetable: airlines, fleets, routes and waves.

**3. On the ground.** [Departure Taxi](traffic-taxi.md) takes a departure from its stand to the take-off; [Arrivals & Parking](traffic-arrival.md) takes an arrival from the approach to its stand. [Ground Decisions](traffic-decisions-ground.md) explains, with the numbers, how a pushback is chosen, who gives way and how a taxi route is found.

**4. In the air.** [Airborne Separation](traffic-separation.md) is the API: wake categories, spacing on final, the landing sequence, delays, the runway controller, go-arounds and conflicts. [Runway & Air Decisions](traffic-decisions-air.md) explains how the map applies it: take-off clearances and restrictions, release and merge planning, absorbing delays, holds and conflict resolution.

**5. The unusual.** [Emergencies & Scenarios](traffic-emergencies.md): MAYDAY and PAN PAN, priority, rescue vehicles, held runways, scripted scenarios and camera scenes. [ATC Game & Staffed Stations](atc-game.md): a human controller on chosen positions. [Jetways](traffic-jetways.md): docking the simulator's jetways to our traffic.

**6. What is said.** [Radio](traffic-radio.md) is every transmission as structured data, positions and stations, for a log or a voice. [Phraseology](traffic-phraseology.md) lists the wording and its sources. [VFR Traffic](traffic-vfr.md) covers light aircraft, circuits and reporting points.

**7. The building blocks.** [Traffic Picture](traffic-picture.md) is everything around a centre, ours and the simulator's. [ATC Commands](traffic-commands.md) lists every clearance a controller can give. [Fleet & AI Objects](traffic-guide.md) creates aircraft. [Injected Ground Movement](traffic-motion.md) moves them frame by frame on the ground and on final. [Aircraft Profiles and Telemetry](traffic-profiles.md) gives each type its sizes and speeds. [Traffic Smoothness](traffic-smoothness.md) is how the motion is kept free of jumps, and the rules for new code.

Related: [Camera](camera.md) films the traffic; [Airport Layout & Taxi Routing](airport-layout.md) is the taxiway graph under it all; [Weather, Runway in Use & ATIS](nav-weather.md#keeping-the-runway-in-use) chooses the runway in use.

## Principles

A few rules run through every page:

- **Plan up front.** Conflicts are planned away before they happen: arrivals are released onto their STARs spaced, merges are planned ahead, and a departure gets its climb restriction with its take-off clearance. The conflict watch is the safety net, not the method.
- **Speed and vectors, not holds.** A delay is lost with speed and at most a vector or a longer downwind. Holding is only for an emergency, the weather or a closure.
- **Crossing restrictions before level-offs.** A climb that would meet traffic is given a fix to cross at or below a level; a stopped climb is the fallback.
- **Give way at the junction.** On the ground, the one giving way stops short of where the paths meet, and an oncoming aircraft keeps the junction clear.
- **Other traffic shows only what the simulator gives.** What is unknown about traffic that is not ours (its destination, its ATC) stays unknown; it is never guessed.
- **No jumps.** Any frame jump of a moving aircraft is a bug ([Traffic Smoothness](traffic-smoothness.md)).
