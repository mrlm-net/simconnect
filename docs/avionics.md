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

**Checked live in MSFS 2024** on the Fenix A319, reading back `COM STANDBY FREQUENCY:1`, `COM ACTIVE FREQUENCY:1` and `TRANSPONDER CODE:1`:

- **Squawk:** `SetSquawk` took effect (2000 to 4521, and back).
- **Swap:** `SwapCOM(1)` swapped active and standby.
- **Standby:** `SetCOMStandby(1, …)` was ignored at every frequency tried (121.805, 121.800, 119.000). The Fenix runs its radio panels with its own logic, so on such aircraft the event is not obeyed. Read the frequency back to know whether it took.

Stock aircraft have not been checked yet.
