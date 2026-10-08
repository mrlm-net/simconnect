---
title: "L:vars"
description: "Write local variables (L:vars) on the user aircraft with pkg/lvars, to signal other add-ons or set the ones they document as settable."
order: 15
section: "packages"
---

# L:vars

`pkg/lvars` writes local variables (L:vars) on the user aircraft through SimConnect (Windows only). An application uses it to signal other add-ons (the aircraft's gauges, GSX, FSUIPC, an in-sim package) and to write the variables they document as settable.

An L:var exists once it is written: writing a new name creates it, and every other client reads it. Measured live in MSFS 2024: `L:MYCREW_TEST` written as 42 by one connection was read as 42 by another.

## Writer

```go
w := lvars.NewWriter(client, defBase, 0) // definitions defBase … defBase+63
if err := w.Set("MYCREW_BOARDING", 1); err != nil { // or "L:MYCREW_BOARDING"
    log.Println(err)
}
_ = w.Set(gsx.SetPassengers, 111) // a GSX setting

// on a new connection:
w.Reset(newClient)
```

- `NewWriter(client, defBase, max)`: `client` is anything with `AddToDataDefinition` and `SetDataOnSimObject` (`lvars.Client`: an `engine.Engine` or the manager). Each name gets a data definition of its own, from `defBase` to `defBase+max-1`; `max` 0 means `DefaultMax` (64). Keep that block clear of your other definition IDs.
- `Set(name, value)`: writes `value` (a `float64`, unit `number`) to the user aircraft. The `L:` prefix is optional. The first write of a name defines it on this connection; once `max` names are defined, a new name returns an error.
- `Reset(client)`: forgets the definitions, since a new connection has none; a non-nil `client` replaces the old one. Call it on every reconnect.

A `Writer` is safe for concurrent use.

## Reading

`pkg/lvars` only writes. To read L:vars, list them in an aircraft profile of [pkg/systems](systems.md) (its values land in `State.Values` by name); GSX's own are read by [pkg/gsx](gsx.md).

## See also

- [GSX and L:vars](gsx.md): the GSX variables add-ons may write.
- [Aircraft Systems Profiles](systems.md): per-model L:vars, such as the Fenix's.
