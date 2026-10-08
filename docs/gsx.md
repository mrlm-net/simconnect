---
title: "GSX and L:vars"
order: 14
section: "packages"
description: Read GSX Pro's state and write L:vars of your own for other add-ons.
---

# GSX and L:vars

## Reading GSX (`pkg/gsx`)

GSX Pro publishes its state in L:vars on the user aircraft, documented in its manual (`GSX_manual_MSFS.pdf`, "DEVELOPERS - Interfacing with GSX", pp. 106–109). `gsx.Reader` reads them all, on top of a `systems.Reader`:

```go
r := gsx.NewReader(client, defID, reqID)
_ = r.Request(types.SIMCONNECT_PERIOD_SECOND)
// for each message:
if s, ok := r.Handle(msg); ok {
	fmt.Println(s.Boarding, s.PassengersBoarding, s.WaitingFor)
}
```

`gsx.State` has:

- `Running`: GSX is running, so its services have a state.
- Each service's state (`Boarding`, `Deboarding`, `Catering`, `Refueling`, `Departure`, `Deice`), as GSX numbers them:

  | Value | State |
  |---|---|
  | 1 | callable |
  | 2 | not available |
  | 3 | bypassed |
  | 4 | requested |
  | 5 | performing |
  | 6 | completed |

  Before GSX runs the state is 0, `Unknown`.
- Passengers: the number to board, boarded or deboarded on this bus and in all, and SimBrief's maximum.
- Cargo: loading or unloading, and the progress in % (the average of the loaders).
- `WaitingFor`: the doors GSX waits for you to open or close ("exit 1", "service 2", "cargo 1", "main cargo").
- Refuelling: the hydrant hose connected, the fuel counter and its maximum.
- `Frozen`: the pushback has frozen the aircraft (`L:FSDT_VAR_Frozen`). `BypassPin`: the bypass pin is in.
- `PilotsOnBoard` and `CrewOnBoard`, as GSX considers them.
- `Gate`: the parking selected in GSX, named like the map's stands ("C19"). Its name follows the SDK's parking-name enum.
- `DeiceFluid`: the de-icing fluid type asked for, 1 to 4.

`gsx.Read(values)` builds the same state from the variables' values by name.

Read live, the Fenix A319 at LKPR before any service: running, every service callable, 150 passengers maximum.

## Writing GSX settings

L:vars are written with `pkg/lvars` ([L:vars](lvars.md)): `w.Set(gsx.SetPassengers, 111)`.

The GSX variables add-ons may write are constants in `pkg/gsx`:

- `SetPassengers`, `SetPilots`, `SetCrew`: set before boarding; 0 lets GSX decide.
- `PilotsNotBoarding`, `CrewNotBoarding`, `PilotsNotDeboarding`, `CrewNotDeboarding`: 1 keeps them on board.
- `DisableDoorsMessage`: hides the "waiting for your action" messages.
- `RemoteControl`: puts the GSX toolbar menu under an application's control.

GSX's other variables are read-only; its manual asks add-ons not to write them.
