# ai-taxi

Spawns an AI aircraft at a parking stand and taxis it to a runway using [`pkg/airport`](../../docs/airport-layout.md) for routing and the `pkg/traffic` departure taxi controller:

1. **Spawn** at the stand, facing the stand's heading.
2. **Pushback** straight back along the stand axis to the taxiway.
3. **Taxi** along the computed route (e.g. `H1 → H → A`), slowing for turns and near the end.
4. **Hold short** of the runway and wait for clearance.
5. **Line up** and take off once cleared (Enter, or `-takeoff-after`).

```bash
go run ./examples/ai-taxi                                   # LKPR C22 → RWY 24
go run ./examples/ai-taxi -stand B2 -runway 30 -takeoff-after 10s
```

| Flag | Default | Description |
|------|---------|-------------|
| `-icao` | `LKPR` | Airport |
| `-stand` | `C22` | Parking stand label (`airport.Layout.ParkingIndex`) |
| `-runway` | `24` | Runway end to depart from |
| `-model` | `FSLTL A320 Air France SL` | Aircraft container title; must be installed |
| `-livery` | | Livery folder name (default livery if empty) |
| `-tail` | `CSA123` | Tail number / call sign |
| `-takeoff-after` | `0` | Clear for take-off automatically after holding short this long; `0` waits for Enter |

Ctrl+C removes the aircraft and exits. After take-off the controller stops tracking the aircraft, which keeps flying its climb waypoints until you exit.

## How it fits together

The example owns the only message loop. Every message goes to `airport.Loader.Handle` (facility data) and `traffic.TaxiController.Handle` (object creation and position updates); neither reads the engine stream itself. Progress arrives on `TaxiController.Events()`.

## Known limitations

- **Pushback is straight.** MSFS AI cannot steer while reversing: a bent reverse leg makes the aircraft spin or turn round and drive forward. The aircraft is pushed straight back to the taxiway and turns onto it going forward.
- **Taxi speed.** 15 kt is requested on straights, but MSFS AI taxis at about 6–9 kt.
- **Intersection departure.** The aircraft lines up abeam its hold-short point rather than back-tracking to the threshold.
- **No traffic separation.** Other aircraft on the route are ignored (planned for v0.8, #269).
