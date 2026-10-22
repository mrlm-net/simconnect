---
title: "Jetways"
description: "How the World docks the simulator's jetways to its own traffic: invisible ParkedATC helpers, gate placeholders, which gates and helper types, timings, and what other programs see."
order: 12
section: "traffic"
---

# Jetways

Our traffic can board through the simulator's own jetways. It is off by default: `World.SetJetways(true)` or `POST /api/jetways {"on": true}` turns it on, and `GET /api/jetways` (`World.Jetways()`, a `JetwayView`) shows each gate's state. It needs the helper package in the Community folder (`go run ./examples/jetway-helpers`, then a simulator restart). Without it, or with jetways off, traffic parks and pushes as before. The backend is `pkg/jetway` (`jetway.Helpers`, behind the `jetway.Service` interface).

## Why a workaround

Our traffic is NonATC, which we drive frame by frame. MSFS 2024 has no way to dock a jetway to such an aircraft: no SimConnect call, event or variable does it, and `TOGGLE_JETWAY` works only for the user aircraft. This was probed live, sent to our aircraft and to the jetway, with and without control released. Only aircraft MSFS ATC parks (`AICreateParkedATCAircraft`, `AICreateEnrouteATCAircraft`) get jetways. Developers have asked for jetway control of any aircraft:

- [Jetway control for non-default ATC controlled AI aircraft](https://devsupport.flightsimulator.com/t/jetway-control-for-non-default-atc-controlled-ai-aircraft/17529), 2026-03;
- earlier, in [Request for additions to Simconnect Facilities API](https://devsupport.flightsimulator.com/t/request-for-additions-to-simconnect-facilities-api/14870).

Vote there. With such a call, the helpers and their side effects below go away: a direct backend would sit behind the same `jetway.Service` calls.

## How it works

MSFS 2024 docks a jetway only to an aircraft in its ATC parking table. So:

1. **Placeholders hold the gates.** When an airport of ours is prepared, invisible ParkedATC placeholders (title "MyCrew Jetway Helper A320", ATC ID "JP…") are created one at a time and the simulator parks each on a free jetway gate. Each is then given a flight plan out of the airport (the retract plan), which sends home the jetway its creation claimed, and is kept on the gate. A placeholder that no jetway takes within `ClaimTimeout` (12 s) is removed. While none takes a gate the fill is tried again, up to `EmptyFillTries` (25, about 5 minutes for a simulator still loading its airport), and a fill that took no gate is tried again every `RefillEvery` (2 min).
2. **A helper docks.** For an aircraft parked on a gate, the gate's placeholder goes and, `HelperDelay` (3 s) later, an invisible helper (ATC ID "JW…") is created on the freed gate under our aircraft. The jetway docks to it. A dock not done within `DockTimeout` (90 s) has failed. No helper is created while another gate docked within `DockSettle` (2 min): the simulator counts that gate free for a while.
3. **It undocks.** Before the push, the docked helper is given the retract plan: its jetway goes home, and the helper stays on the gate as its placeholder, released `RetractWait` (25 s) later. Removing it instead left the gate to a new placeholder, which the simulator could put on a docked gate it counts free.

## Which gates

- **Dockable gates.** The simulator puts no ParkedATC next to an occupied gate, so only about every other jetway gate can be given a helper (at LKPR 19 of 27; `Helpers.Dockable`). With jetways on, the stand allocator gives airliners those gates first, before their airline's own gates without one (`StandRequirements.Prefer`). An aircraft on another gate gets no jetway.
- **Departures wait for the gates.** A departure spawned on a jetway gate before the airport's gates are filled would stand where no helper's door meets it, so its spawn waits (`traffic.ErrSpawnBlocked`: "the jetways at LKPR are not ready yet") and is tried again without counting an attempt. The wait ends once the gates are filled, or `jetwaysWaitMax` (6 min) after the jetways were switched on.
- **Not:** types with their own airstairs (`traffic.NoJetway`).

## Which helper

- **By type.** A helper is the installed model of a type with its door, `flight_model.cfg` `interactive_point.0`, which the jetway data reports as `ExitDoorRelativePos`. Our aircraft's type gets its own helper (`Helpers.HelperFor`); a type without one takes the A320's (`HelperOptions.FallbackType`).
- **Big gates.** The simulator gives a ParkedATC the best-fitting gate it counts free, a docked one among them. So at a gate bigger than others of the airport, the helper is one too wide for the smaller gates that still fits this one, when our aircraft's own would fit a smaller gate (`gateHelper`). At LKPR an A320 helper for C24 (28 m) or B14 (35 m) otherwise went to a docked 22 m gate and took its jetway.
- **Off the parking line.** The simulator parks each gate's ParkedATC a little off the stand's line, measured on its placeholder (`gatePlace`: at LKPR the nose gear 5.2 to 6.1 m inside the parking circle, up to ±0.55 m to the side). Our aircraft stands where the helper's door is. Where a helper variant is installed whose door makes up for the side offset (`jetway.HelperVariantTitle`, steps of `JetwayVariantStepM`, 0.1 m, up to `JetwayVariantSteps`, 6), that variant docks and our aircraft stands on the line.
- **Stand rules.** At a jetway gate with jetways on, a stand rule that moves the stop along the line ([stand rules](traffic-world.md#stands-and-stand-rules)) is not applied: the jetway reaches no further than where the simulator parks (LKPR C22, a B738 9 m back, the jetway docked short of it).
- **Door fit.** The helper stays where the simulator parks it, as the simulator's own AI would. On the A320neo the hood meets the door; on the 737 it can sit about 2 m forward of it.
  - **Why not moved:** a helper moved after the simulator placed it has its jetway report docked without the jetway moving at all (live, LKPR A4, C22, C30, 2026-10-12).
  - **Logged instead:** the door offset that would fit, from both models' doors or the table `jetway.doorOffsets` (`POST /api/jetways/offset`, `GET /api/jetways/offsets`), is logged for every dock ("the door wants 2.96 m ahead; not moved").
  - **Moving anyway:** `HelperOptions.MoveHelpers` moves the helper there; it is off by default.

## When a dock goes wrong

- **A helper on another gate.** The simulator now and then parks a helper on another gate or a ramp. That helper is sent the retract plan and removed, and the dock is asked again a minute later, up to `JetwayDockTries` (2) times for the same aircraft model.
- **A taken gate.** A placeholder or helper sometimes lands on a gate already docked: the simulator does not always count a docked helper's gate as taken, and its jetway follows the newcomer. That gate is cleared and docked again a minute later.
- **A gate that keeps failing.** A gate whose dock failed `JetwayGateFails` (3) times in a row, for any aircraft, rests for `JetwayGateRest` (1 hour): its helper went to another gate each time, taking a docked gate's jetway.
- **Boarding without it.** A departure waits at most `JetwayWaitMax` (3 min) for its jetway to dock, then boards without it.

## Boarding and the push

A departure asks for its push once its jetway has been on for `JetwayBoarding` (3 min). Ground answers once the jetway is home (at least 25 s after it left), and it is not docked again. For about half the departures (`TugAfterJetwayShare`, 0.5, drawn at the spawn) the tug sets off only once the jetway is home; the others' tug hooks up during boarding.

## What other programs see

The placeholders and helpers are real simulator aircraft (ParkedATC), invisible only because their model has no parts. With jetways on, at LKPR about 19 of them stand on the jetway gates. Anything that lists or uses the simulator's aircraft sees them:

- **SimConnect clients:** traffic maps, radar and TCAS displays, and other add-ons that enumerate aircraft (Little Navmap, traffic tools, other SimConnect apps) get them like any other aircraft:
  - title "MyCrew Jetway Helper <TYPE>";
  - ATC ID "JP…" (placeholder) or "JW…" (helper);
  - parked on a gate, the helpers right under one of our aircraft.

  A placeholder carries a flight plan to the nearest other airport (the retract plan), so a tool that reads plans may list it as a departure there. It stays parked; none was seen to leave.
- **Our own World and map:** they leave them out (`jetway.IsHelper` on the title). A host or SDK consumer listing traffic should do the same. The titles are also listed in the model rules, so they are never chosen as traffic.
- **MSFS ATC and its AI traffic:** the gates they hold count as taken; that is how the jetway is kept for our traffic. So MSFS's own AI traffic, and parking assigned by MSFS ATC to the user, go to other stands.
- **Other traffic add-ons:** those that park on free gates the same way find these gates taken. That is expected, not measured: FSLTL's injector, BeyondATC's traffic.
- **Ground services and multiplayer:** not measured. GSX and multiplayer sessions may see a parked aircraft on those gates as well.
- **Turning jetways off:** `SetJetways(false)` removes every placeholder and helper, and so does disconnecting.
