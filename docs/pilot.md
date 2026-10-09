---
title: "Pilot Flying"
description: "pkg/pilot flies the user aircraft as the copilot, pilot flying on the autopilot: the speed schedule, flaps and gear asked of the player as pilot monitoring, the approach armed when cleared, the controls handed back at minimums."
order: 15
section: "packages"
---

# Pilot Flying

`pkg/pilot` is a copilot flying the user aircraft. It is logic only: each tick it takes the aircraft's state and gives back what to do. The app sends the actions, voices what is said, and routes the requests to the player as pilot monitoring.

## Iteration A: the autopilot

The player flies the take-off and the landing. The engine flies everything between on the autopilot:

1. **Take-off.** Climbing through the engage height (1000 ft above the ground), the engine engages the autopilot and the autothrust and says "I have control".
2. **Climb.** It selects the cleared altitude (the plan's cruise without a clearance) in level change, with NAV or an assigned heading. It flies 250 kt below 10,000 ft and 290 kt above. Above the acceleration height (1500 ft) it asks for the flaps up step by step as the speed allows, and asks for the gear up if it is still down.
3. **Cruise** at the cleared or cruise level.
4. **Descent** from the 3-to-1 rule (three miles a thousand feet, plus ten). It says "Top of descent" and asks the player to ask ATC for descent when no lower level is cleared yet. It flies 280 kt, then 250 kt below 11,500 ft.
5. **Approach** below 5000 ft, within 20 NM, or once cleared for the approach. Each flap setting is flown 10 kt under the next detent's limit, so the next one comes in turn. The gear goes down by 2000 ft or on the glideslope, the landing flaps by 1500 ft with the gear down. The approach is armed once cleared. The landing flaps are flown at the approach speed: 1.3 × VS0 + 5 from the aircraft's design speeds.
6. **Minimums** (the decision height or altitude set, else 200 ft): it says "Autopilot off" and "Your controls" and gives the aircraft back.

```go
e := pilot.New(pilot.Config{CopilotActs: true}, profile.FlapDetents)
// each tick:
out := e.Update(pilot.Input{Now: now, State: state, Air: sample, Plan: plan, ATC: clearance})
for _, a := range out.Actions { /* controls.Set(a.Name, *a.On, state) or SetValue(a.Name, *a.Value, state) */ }
for _, s := range out.Say { /* the pilot flying says it */ }
for _, r := range out.Requests { /* ask the player: r.Say ("Flaps one", "Gear down") */ }
```

A request is done when the aircraft shows it (`Output.Done`). If the player does nothing for `PMTimeout` (8 s) and `CopilotActs` is set, the copilot does it (`Output.TimedOut`, its actions in `Output.Actions`). An action is not sent again within `Resend` (3 s) while it has not shown yet.

Every height and speed is in `Config`, so a per-type profile can tune them. `Config.WithLearned` takes them from the player's recorded flights of the type (`flight.Learn`, #966). With `Config.HandFly` (iteration B) the copilot also flies the take-off and the landing by hand; see below.

## Handovers

The engine takes the controls at the first `Update` where the aircraft is airborne above the engage height: climbing after the take-off, or anywhere in the flight when the copilot is switched on in cruise. It picks the phase from where the flight is: the approach when low and close in, the descent going down, the cruise at the target level, else the climb. `HandBack()` gives the controls to the player mid-flight ("Your controls"; the autopilot is left as it is), and `TakeControl()` has the engine take them again from where the flight is ("I have control").

## Iteration B: hand flying

With `Config.HandFly` the copilot flies the take-off and the landing too, through the flight controls (`systems.Elevator`, `Aileron`, `Rudder`, `Throttle` as `SetValue`). Give it `Input.Runway` (the threshold, its true heading, the elevation and the length of the runway end) and the clearances `Clearance.Takeoff` and `Clearance.Land`. Update it every sim frame while it flies by hand.

- **Take-off.** Once cleared, it says "Takeoff" and sets the thrust (50 % for two seconds, then `TakeoffThrust`, 90 %). The rudder holds the centreline. At VR (the aircraft's, else learned, else V2 − 5) it rotates at 3°/s to the rotation pitch (learned, else 12.5°). In the air it flies the climb pitch (learned, else 15°), adjusted for V2 + 10, wings level on the runway heading, and asks "Positive climb, gear up". At the engage height it engages the autopilot and goes on as in iteration A.
- **Landing.** At minimums, cleared to land, it disconnects the autopilot and the autothrust ("Autopilot off") and flies on by hand:
  - On final it holds the 3° glide path through 50 ft over the threshold by pitch (the sink rate for the ground speed, corrected for the height off the path), the approach speed by throttle, and the centreline by bank. The centreline correction is slow next to the turn, so it does not overshoot.
  - It flares at the learned height (else 30 ft), or 3 s before touching down if that is higher. The flare flies a sink rate that eases with the height (100 ft/min plus 8 a foot), and the thrust goes to idle at 20 ft ("Retard").
  - Below 15 ft it straightens the nose with the rudder.
  - After touchdown it lowers the nose gently, holds the centreline with the rudder, and below 40 kt says "Your controls".
- **Not cleared to land at minimums:** it says "Go around, flaps", sets the thrust to 100 % and hands back.

The loops were tuned on a simple model, so their gains and signs need a live check. The model landing touches down at about −260 ft/min, 740 m past the threshold, on the centreline. The elevator axis sign is assumed: pushed forward is positive (`systems.Elevator`).
