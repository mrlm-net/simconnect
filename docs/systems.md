---
title: "Aircraft Systems Profiles"
description: "Read the user aircraft's power, radios, engines, lights, doors, transponder, flaps and gear with pkg/systems: standard SimVars, per-model profiles as data, local overrides."
order: 13
section: "packages"
---

# Aircraft Systems Profiles

`pkg/systems` reads the user aircraft's systems through a **profile**. A profile is plain data: which variables give each value. There are three layers:

1. **Default:** the standard SimVars, for every aircraft that follows them.
2. **Shipped per-model profiles:** go on top for aircraft that run their systems on their own variables. The first one is the Fenix A320 family, on L:vars.
3. **Local override files:** the application's own, on top of both. They win per value.

```go
a := systems.Aircraft{Package: pkg.Folder, Title: title, ATCType: atcType} // addons.AircraftPackage gives the folder
p := systems.For(a, localOverrides...)
r := systems.NewReader(client, defID, reqID)
r.Use(p)
r.Request(types.SIMCONNECT_PERIOD_SECOND)
// in the message loop:
if s, ok := r.Handle(msg); ok { /* s.Battery, s.Powered, s.ExtOn, s.Squawk, … */ }
```

After a reconnect, call `Reset`. When another aircraft loads, call `Use` with its profile; the next `Request` registers the new variables.

## Values

| Value | State field | Default (standard SimVar) |
|---|---|---|
| `battery` | Battery | ELECTRICAL MASTER BATTERY |
| `volts` | Volts | ELECTRICAL MAIN BUS VOLTAGE |
| `powered` | Powered | ELECTRICAL MAIN BUS VOLTAGE ≥ 10 V |
| `avionics` | Avionics | AVIONICS MASTER SWITCH |
| `extAvailable`, `extOn` | ExtAvailable, ExtOn | EXTERNAL POWER AVAILABLE:1, EXTERNAL POWER ON:1 |
| `com1`, `com2` | COM1, COM2 (working) | COM STATUS:1/2 = 0 |
| `engines`, `engineRunning1–4`, `starter1–4` | Engines, Running, Starter | NUMBER OF ENGINES, GENERAL ENG COMBUSTION:n, GENERAL ENG STARTER:n |
| `parkingBrake` | ParkingBrake | BRAKE PARKING INDICATOR |
| `lightBeacon`, `lightNav`, `lightStrobe`, `lightLanding`, `lightTaxi` | Beacon, Nav, Strobe, Landing, Taxi | LIGHT BEACON / NAV / STROBE / LANDING / TAXI |
| `door0–3` | Doors | EXIT OPEN:0–3 |
| `xpdrState`, `xpdrCode` | XPDRState, Squawk ("4521") | TRANSPONDER STATE:1, TRANSPONDER CODE:1 (Bco16) |
| `flapsPct`, `gearDown` | FlapsPct, GearDown | FLAPS HANDLE PERCENT, GEAR HANDLE POSITION |
| `weightKg`, `flapsIndex` | (for the speed table) | TOTAL WEIGHT (kilograms), FLAPS HANDLE INDEX |
| `v1Kt`, `vrKt`, `v2Kt`, `daFt`, `mdaFt` | V1Kt, VRKt, V2Kt, DAFt, MDAFt | none: a model whose FMS gives them (its L:vars) |

`State.Values` holds every resolved value by name, including any that a profile adds.

**Take-off speeds.** `State.V1Kt`, `VRKt` and `V2Kt` (knots, 0 unknown) come from the aircraft when its profile reads `v1Kt` and V1 is set (`SpeedsFrom` "fms": the speeds the crew entered). Otherwise they come from the profile's `takeoffSpeeds` table by `flapsIndex` and `weightKg` (`SpeedsFrom` "table"), interpolated between its weights and held at its ends; with neither, none (`SpeedsFrom` ""). `SpeedCheckKt` is the profile's speed check on the take-off roll: 80 by default (Boeing-style procedures), 100 for the A320 family. The tables by type are the dict table `systems.speeds` (`speeds.json`, fed from the MyCrew API's `aircraft-speeds` set; a local override wins): items `{"type": "A319", "aliases": ["A19N", "A318"], "speedCheckKt": 100, "measured": "…", "weightsKg": […], "flaps": {"1": {"v1": […], "vr": […], "v2": […]}, "*": {…}}}`. `SpeedsFor` picks a type by the ATC type, else by a type code in the title ("FenixA319": the A319's); a profile's own `takeoffSpeeds` and `speedCheckKt` win. The "*" row serves a flaps position without its own row; flaps given in a correction replace the rows whole. Shipped today: the A319 (A19N, A318), A320 (A20N) and A321 (A21N), approximate figures (not from an FCOM) for CONF 1+F, 2 and 3. The Fenix profile reads its MCDU speeds from `L:N_MISC_PERF_TO_V1`, `L:N_MISC_PERF_TO_VR` and `L:N_MISC_PERF_TO_V2` (from the FSUIPC7 L:var list, 0 before speeds are entered; entered speeds not yet seen live), and falls back to the table while they read 0.

**Computed** (`SpeedsFrom` "calc"), when there are no FMS speeds and no table: from the aircraft's own design speeds, read as SimVars (DESIGN SPEED VS0, VS1, MIN ROTATION, DESIGN TAKEOFF SPEED and MAX GROSS WEIGHT; all filled live on the Fenix A319), scaled by √(weight / max weight). VR is the minimum rotation speed, V2 the design take-off speed (at least VR + 4), V1 two knots under VR. Without a rotation speed, VR is 1.1 and V2 1.2 times the stall speed a quarter of the way from VS0 to VS1. An aircraft under 5700 kg maximum weight (`LightAircraftKg`) gets VR only. Source order: fms, table, calc, none.

```json
"speedCheckKt": 100,
"takeoffSpeeds": {
  "weightsKg": [50000, 60000, 70000],
  "flaps": { "1": { "v1": [120, 132, 144], "vr": [121, 133, 145], "v2": [126, 137, 148] } }
}
```

## The profile format

```json
{
  "name": "Fenix A320 family",
  "match": { "packagePrefix": ["fnx-aircraft"], "titleContains": ["FNX"], "atcType": [] },
  "measured": "how and where it was measured",
  "values": {
    "battery":     { "vars": ["L:S_OH_ELEC_BAT1", "L:S_OH_ELEC_BAT2"], "combine": "any", "note": "measured" },
    "volts":       { "vars": ["L:N_ELEC_VOLT_BAT_1", "L:N_ELEC_VOLT_BAT_2"], "combine": "max" },
    "lightStrobe": { "vars": ["L:S_OH_EXT_LT_STROBE"], "trueAt": [2] }
  }
}
```

**Frequencies.** `com1Active`, `com1Standby`, `com2Active` and `com2Standby` are the COM frequencies in MHz (State.COM1Active and the others). The default reads them from `COM ACTIVE/STANDBY FREQUENCY:n`.

**Match.** A profile applies when any one rule matches:
- `packagePrefix`: the start of the aircraft's package folder (`addons.AircraftPackage`);
- `titleContains`: part of the title, any case;
- `atcType`: the ATC TYPE.

**Values.** Each value lists its `vars` in `unit` (default `number`, as L:vars are read), then:
- `combine` joins several vars: `any` is true when any is not 0; `max` and `min` take the largest or smallest; no `combine` takes the first var;
- `trueAt` makes a var true only at those positions, e.g. a three-position switch on only at 2;
- `scale` multiplies the result, e.g. kHz as MHz with 0.001 (applied before `atLeast`);
- `atLeast` makes the result true at that value or more, e.g. volts as powered;
- `note` says whether the value was measured or assumed.

`ReadProfile` reads a profile from JSON and refuses a value with no vars or an unknown `combine`.

**Order and overrides.** `For(aircraft, overrides...)` builds the profile in three steps:
1. starts from `Default()`;
2. puts the matching type base on top, then the first matching model's profile (`Profiles()`; see [Type bases](#type-bases-cabin-signs-and-counted-buttons));
3. puts each matching override on top, in the order given.

`Profiles()` is the shipped set, replaceable at runtime as the `systems.profiles` table of [pkg/dict](dictionaries.md).

An override wins per value: values it doesn't name stay as they were. The same applies to an override without a `match` that has the matched profile's `name`. `Merge(base, over)` is that step on its own.

## The Fenix A320 family

`profiles/fenix-a320.json` matches package folders starting with `fnx-aircraft`. It was measured live in MSFS 2024 at LKPR on the A319 by toggling each switch while tracing both sets of variables. The standard battery, avionics and external power values don't follow the Fenix:
- battery and avionics read on with the aircraft dark;
- the main bus reads 28 V, battery 2's own voltage;
- external power reads "not feeding" with EXT PWR on.

So the profile reads these from the Fenix's L:vars:
- **battery:** BAT1 or BAT2;
- **volts:** the higher of the two battery voltages;
- **powered:** any AC or DC bus powered;
- **avionics:** AC ESS bus powered;
- **external power:** its AVAIL and ON lights.

Lights, parking brake, EXIT OPEN:0 and the transponder follow the standard variables and stay as they are. Live, the two profiles disagreed only where the Fenix differs: external power on, and the volts.

**Radios.** These come from the RMPs, measured powered and dark:
- COM working: `L:B_PED_RMP1_POWER` and `RMP2_POWER`.
- The frequencies: `L:N_PED_RMP1_ACTIVE` and `_STDBY`, in kHz scaled to MHz (`scale`: 0.001). `COM STANDBY FREQUENCY:1` does not follow the RMP.
- RMP 2 is assumed to work as RMP 1, and marked so.

The profile's **actions** give the COM swap as the RMP transfer key, `L:S_PED_RMP1_XFER` (see [Radios and Transponder](avionics.md)).

## Actions

`actions` names how a control is operated on a model where the standard key events do not do it: `{"com1Swap": {"press": "L:S_PED_RMP1_XFER"}}` presses that variable (1, then 0). `pkg/avionics` takes them with `Radios.Use(profile.Actions)`. They merge like values: an override wins per action.

An action is one of: `press` (a button variable clicked), `set` (a variable set to the state wanted, 1 or 0), `event` (a key event; with `toggle` sent only when the state differs, with `data` for its parameter), `efb` (a boolean data ref written through the aircraft's tablet API, `Profile.EFB`; `Controls.SetEFBHost` for an app on another machine), or `counter` (a counted push button, see below). `note` documents it.

## Ground controls

`Controls` operates the user aircraft's ground controls by name, the same way for every aircraft (#667): `Door(n)`, `Chocks`, `GPU`, `ParkingBrake`. The profile says how:

- **Default:** the exits by `TOGGLE_AIRCRAFT_EXIT` with their index from 1, toggled only when not as wanted; the parking brake by `PARKING_BRAKES`, likewise. No chocks or GPU.
- **Fenix A320 family** (measured live on the Fenix A319, 2026-10-04): the main door and the parking brake by the default key events; chocks and its GPU through its EFB API (`efb` actions: `fenix.efb.chocks`, `groundservice.groundpower`, GraphQL `writeBool` on port 8083, as its EFB does; a write to `L:B_CONFIG_CHOCKS` or `L:B_CONFIG_GPU` does not stick); read back from those L:vars. With the chocks off the Fenix reads its parking brake released.

```go
ctl := systems.NewControls(client, 0)
ctl.Use(profile) // systems.For(the aircraft)
if ctl.Can(systems.Chocks) {
	ctl.Set(systems.Chocks, false, reader.State()) // remove them
}
ctl.Set(systems.Door(0), true, reader.State()) // open the main door
```

A profile's `doors` names its exits. As names (`["Door 1", "Door 2"]`) they go in the order of EXIT OPEN and TOGGLE_AIRCRAFT_EXIT (`Door(0)` is exit 1): "Door 1"…"Door 4" by default. As objects they name each door's exit (`Profile.Exits`): the Fenix A319's passenger doors `[{"name":"L1","exit":1},{"name":"L2","exit":4},{"name":"R1","exit":5},{"name":"R2","exit":8}]` (measured 2026-10-05; exits 2, 3, 6, 7, 12 and 13 move but are no passenger door, not named yet). TOGGLE_AIRCRAFT_EXIT k toggles EXIT OPEN:k-1, so each door reads and toggles its exit. Their number is how many it has. `State.DoorsOpen` and `DoorNames` are all of them (`Doors` keeps the first four).

`State` reads `Chocks` and `GPU`, with `HasChocks` and `HasGPU` when the model has them. `Can` tells the app which buttons to show; a control the profile does not give returns `ErrNoControl`.

`NewControls(client, 0)` takes its client event and data definition IDs from `DefaultControlBase` (0x7B00), a block of `ControlIDs` (64); past that, `ErrNoIDs`. After a reconnect call `Reset(client)` (nil keeps the client); `Reader.Reset(client)` works the same way.

## Ground services

The sim's own ground services for the user aircraft are requested by name with `Controls.Request` (#666): `Jetway` (`TOGGLE_JETWAY`, at a parking spot; asked again, sent away), `Stairs` (`TOGGLE_RAMPTRUCK`), `Baggage` (`REQUEST_LUGGAGE`), `Catering` (`REQUEST_CATERING`), `PowerSupply` (`REQUEST_POWER_SUPPLY`), `FuelTruck` (`REQUEST_FUEL_KEY`, at a parking spot) and `Pushback` (`TOGGLE_PUSHBACK`): the standard key events (MSFS 2024 SDK Key Events) by default, a model's own way where its profile gives one. `State` reads the pushback: `PushbackAttached`, `PushbackAvailable`, `PushbackWait` (Services Variables). An app that drives GSX uses it instead where GSX runs.

## Copilot callouts

`State` carries what a copilot calls on the take-off roll and the landing roll:

- `SpoilersArmed` and `SpoilersPct`, the higher of the left and right ground spoilers deployed (`spoilersArmed`, `spoilersPct`; `SPOILERS ARMED`, `SPOILERS LEFT/RIGHT POSITION`): "spoilers" on touchdown.
- Per engine 1–4: `Reverser[i]` engaged and `ReverserPct[i]` nozzle deployed (`Reverser(n)`, `ReverserPct(n)`; `GENERAL ENG REVERSE THRUST ENGAGED:n`, `TURB ENG REVERSE NOZZLE PERCENT:n`): "reverse green".
- Per engine 1–4: `N1[i]` (`N1(n)`; `TURB ENG N1:n`, percent): "thrust set" once stable.
- `FlapsSaid`: the flap lever's position as said, from the profile's `flapDetents` by `FLAPS HANDLE INDEX`. The A320 family has "zero", "one", "two", "three", "full", and the 737 base profile "up", "1", "2", "5", "10", "15", "25", "30", "40". It is "" when the profile gives none.
- `DAFt` (`DECISION ALTITUDE MSL`) and `DHFt` (`DECISION HEIGHT`, radio minimums): "one hundred above" and "minimums". A model with its own (an MCDU's) overrides them.
- The landing lights are a control: `Controls.Set(LightLanding, on, state)` sends `LANDING_LIGHTS_ON` or `LANDING_LIGHTS_OFF`. The Fenix sets its switch `L:S_OH_EXT_LT_LANDING_BOTH` to 2 (on) or 1 (off), not yet measured. An action may give its own `on` and `off` values (a switch of more positions) and an `offEvent`.

These are the default SimVars and have not been measured live yet. A model that drives its own (the Fenix may) overrides them in its profile like any other value.

## Autopilot and flight controls

`State.AP` is the autopilot, and `Controls` operates it, by generic names. The default profile uses the standard SimVars and key events (SDK: Autopilot/Assistant Variables; Autopilot/Flight Assist, Flight Control, Engine and Landing Gear events). An add-on with its own FCU (the Fenix) overrides the same names in its profile JSON.

| Name | Read (`State.AP`) | Operate |
|---|---|---|
| `apMaster`, `fd`, `athr` | `AUTOPILOT MASTER`, `…FLIGHT DIRECTOR ACTIVE`, `…THROTTLE ARM` | `Set`: `AUTOPILOT_ON`/`OFF`, `TOGGLE_FLIGHT_DIRECTOR`, `AUTO_THROTTLE_ARM` (toggled when it differs) |
| `athrActive` | `AUTOPILOT MANAGED THROTTLE ACTIVE` | — |
| `apHeadingSel`, `apAltitudeSel`, `apVSSel`, `apSpeedSel`, `apMachSel` | `…HEADING LOCK DIR`, `…ALTITUDE LOCK VAR`, `…VERTICAL HOLD VAR`, `…AIRSPEED HOLD VAR`, `…MACH HOLD VAR` | `SetValue`: `HEADING_BUG_SET`, `AP_ALT_VAR_SET_ENGLISH`, `AP_VS_VAR_SET_ENGLISH`, `AP_SPD_VAR_SET`, `AP_MACH_VAR_SET` (Mach × 100) |
| `apHeadingManaged`, `apSpeedManaged`, `apAltitudeManaged`, `apVSManaged` | the slot index is 2 | `Set`: on pushes (slot 2), off pulls (slot 1): `HEADING_SLOT_INDEX_SET` … |
| `apHeadingHold`, `apAltitudeHold`, `apVSHold`, `apFLC`, `apSpeedHold`, `apMachHold`, `apNav`, `apApproach`, `apGlideslope` | the holds engaged | `Set`: `AP_HDG_HOLD_ON`/`OFF`, `AP_ALT_HOLD_…`, `AP_VS_…`, `FLIGHT_LEVEL_CHANGE_…`, `AP_AIRSPEED_…`, `AP_MACH_…`, `AP_NAV1_HOLD_…`, `AP_APR_HOLD_…` |
| `apLoc` | none by default | `Set`: `AP_LOC_HOLD_ON`/`OFF` |
| `apApproachArmed`, `apGSArmed`, `apAltitudeArmed` | `…APPROACH ARM`, `…GLIDESLOPE ARM`, `…ALTITUDE ARM` | — |

The flight controls are actions for a pilot flying by hand: `SetValue(Elevator, …)`, `Aileron` and `Rudder` take −100…100 (nose up, right roll, right yaw positive) as `AXIS_ELEVATOR_SET`, `AXIS_AILERONS_SET` and `AXIS_RUDDER_SET`. `Throttle` (all engines) or `ThrottleN(n)` take 0…100 as `AXIS_THROTTLE_SET` or `AXIS_THROTTLEn_SET`. `FlapsLever` takes 0…100 of its travel (`FLAPS_SET`, the nearest detent). `FlapsUp` and `FlapsDown` move one detent. `Set(GearDown, …)` sends `GEAR_DOWN` or `GEAR_UP`, and `Set(SpoilersArmed, …)` sends `SPOILERS_ARM_ON` or `SPOILERS_ARM_OFF`.

An action whose event carries a value has `"value": true`, with `"scale"` multiplying it. A negative value is sent as its two's complement. Not measured live yet: which slot means managed (2 as the stock Airbus uses it is assumed), negative vertical speeds, and the elevator axis's sign.

## EFB

A profile's `efb` is where the aircraft serves its tablet over HTTP: the Fenix's EFB on port 8083 (`{"port": 8083, "path": "/"}`, plain HTTP, all interfaces); none in the default.

## Type bases, cabin signs and counted buttons

Profiles come in layers (#759): the default for any aircraft, then a base for the aircraft's type, then the model's profile where it differs. A base has `"base": true` and matches by type (`a320-family.json`: ATC TYPE A318 to A321, A20N, A21N, or the title). It is used alone when no model's profile matches. A model's profile names its base with `"extends"`: the Fenix extends "A320 family".

| Name | Base (A320 family) | Fenix A319 (measured 2026-10-05) |
|---|---|---|
| `seatbelts` | CABIN SEATBELTS ALERT SWITCH; CABIN_SEATBELTS_ALERT_SWITCH_TOGGLE | read the standard variable (it follows); set L:S_OH_SIGNS 0/1 |
| `noSmoking` | CABIN NO SMOKING ALERT SWITCH; CABIN_NO_SMOKING_ALERT_SWITCH_TOGGLE | L:S_OH_SIGNS_SMOKING 0 off, 1 auto, 2 on (the standard stays 0) |
| `extPower` | EXTERNAL POWER ON:1; TOGGLE_EXTERNAL_POWER 1 | the ON light L:I_OH_ELEC_EXT_PWR_L; pressed on the counted L:S_OH_ELEC_EXT_PWR |
| `cabinCall` | none | pressed on the counted L:S_OH_CALLS_ALL |
| cargo doors | none | "FWD cargo" exit 9 (EXIT OPEN:8), "AFT cargo" exit 10 (EXIT OPEN:9), moved through the EFB: `doors.cargo.forward`, `doors.cargo.aft` |

A door object may carry `"efb"`: that door moves through the tablet (`Profile.DoorEFB`), not TOGGLE_AIRCRAFT_EXIT. An action's `"counter"` is a push button counted up, as FSUIPC's presets press it: from an even count to +1 (press), then +2 (release). Its count is read as the value `<action>Counter`. `Controls.Press(name, now)` presses a control once whatever its state (`cabinCall`). `Controls.SetValue(name, v, now)` sets a many-way switch (`noSmoking` 2).

### Knobs: push and pull, relative encoders

An FCU knob pushed and pulled on one variable (the Fenix's `S_FCU_SPEED`: +1 push, −1 pull, 0 released) is a press with values: `{"press": "L:S_FCU_SPEED", "on": 1, "off": -1}`. `Set(name, true)` presses it with `on`, `Set(name, false)` with `off`, and each is released to 0. Without `on`/`off`, a press writes 1.

A relative encoder (the Fenix's `E_FCU_SPEED`, a counter turned a click at a time) is set with `SetValue`. Its action names the counter, the value showing the setting and its step: `{"encoder": "L:E_FCU_SPEED", "display": "fcuSpeed", "step": 1}`. The profile reads both as values: the display (`"fcuSpeed": {"vars": ["L:N_FCU_SPEED"]}`) and the counter as the action's name with `Encoder` (`"apSpeedSelEncoder": {"vars": ["L:E_FCU_SPEED"]}`). `SetValue` moves the counter by the clicks from the value shown to the one wanted. With `"wake": true`, a display that reads 0 is taken as dashed (the Fenix's managed heading shows nothing until turned). One click wakes it, and `SetValue` returns `ErrEncoderWoken`: call it again with a state read after it.
