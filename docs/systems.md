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

`State.Values` holds every resolved value by name, including any that a profile adds.

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
- `atLeast` makes the result true at that value or more, e.g. volts as powered;
- `note` says whether the value was measured or assumed.

`ReadProfile` reads a profile from JSON and refuses a value with no vars or an unknown `combine`.

**Order and overrides.** `For(aircraft, overrides...)` builds the profile in three steps:
1. starts from `Default()`;
2. puts the first matching shipped profile (`Profiles()`) on top;
3. puts each matching override on top, in the order given.

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
