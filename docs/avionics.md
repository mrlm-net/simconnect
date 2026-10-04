---
title: "Radios and Transponder"
description: "Set the user aircraft's COM frequencies, swap them and set the squawk with pkg/avionics."
order: 12
section: "packages"
---

# Radios and Transponder

`pkg/avionics` sets the user aircraft's radios through the simulator's key events. Each event is mapped on first use; call `Reset` after a reconnect.

```go
r := avionics.New(client, 0)  // client: the engine or a manager instance
r.SetCOMStandby(1, 134.560)   // COM_STBY_RADIO_SET_HZ
r.SwapCOM(1)                  // COM1_RADIO_SWAP
r.SetCOMActive(2, 118.105)    // COM2_RADIO_SET_HZ (8.33 kHz channels too)
r.SetSquawk("4521")           // XPNDR_SET, BCD16 (SquawkBCD)
```

**Events** (MSFS 2024 "Aircraft Radio Navigation Events"):

| Action | COM1 | COM2 | COM3 |
|---|---|---|---|
| Standby, in Hz | `COM_STBY_RADIO_SET_HZ` | `COM2_STBY_RADIO_SET_HZ` | `COM3_STBY_RADIO_SET_HZ` |
| Active, in Hz | `COM_RADIO_SET_HZ` | `COM2_RADIO_SET_HZ` | `COM3_RADIO_SET_HZ` |
| Swap | `COM1_RADIO_SWAP` | `COM2_RADIO_SWAP` | `COM3_RADIO_SWAP` |

`XPNDR_SET` sets the squawk; MSFS supports one transponder.

**Errors:** `ErrBadRadio` (COM 1–3 only), `ErrBadFrequency` (118.000–136.990 MHz) and `ErrBadSquawk` (four digits 0–7) are returned before anything is sent.

**Checked live in MSFS 2024** on the Fenix A319, with the aircraft powered:

- **Squawk:** `SetSquawk` takes effect.
- **Standby:** `SetCOMStandby(1, …)` sets the RMP 1 standby. Read it back from `L:N_PED_RMP1_STDBY` (kHz, the systems profile's `com1Standby`), not from `COM STANDBY FREQUENCY:1`, which does not follow the RMP.
- **Swap:** the stock `COM1_RADIO_SWAP` swaps the simulator's own pair, which the RMP's standby never reached. On the Fenix the swap must be its transfer key, `L:S_PED_RMP1_XFER`. `COM ACTIVE FREQUENCY:1` then follows.
- **Dark aircraft:** with the aircraft dark the RMP is unpowered and nothing changes. The first test was dark and wrongly read as "the Fenix ignores the event".

**Per-model actions.** `Use(profile.Actions)` takes a model's actions from its systems profile (`pkg/systems`). On the Fenix the profile gives `"com1Swap": {"press": "L:S_PED_RMP1_XFER"}`, so `SwapCOM` presses that key (1, then 0 after 300 ms) instead of sending the event. Pressing needs a client that can set variables (`Presser`: the engine and the manager both can). Aircraft without such an action keep the key events.

```go
p := systems.For(aircraft)
r := avionics.New(client, 0)
r.Use(p.Actions)
r.SetCOMStandby(1, 121.805)
r.SwapCOM(1) // the Fenix: its RMP transfer key
```
Stock aircraft have not been checked yet.

## The call sign for ATC

`SetFlight(client, defBase, airline, number)` sets the user aircraft's ATC AIRLINE ("Czech Air Force", the call sign as said) and ATC FLIGHT NUMBER ("007"); "" leaves one as it is (#680). Measured in MSFS 2024: both are settable and read back as set (ATC ID, the registration, is separate). An app can offer to use the flight plan's call sign in the sim.
