# simconnect-events

Example demonstrating the new system event hookups in the manager package.

This example shows two ways to consume events:

1. Callback-style handlers via `OnFlightLoaded`, `OnAircraftLoaded`, `OnFlightPlanActivated`, `OnObjectAdded`, `OnObjectRemoved`.
2. Channel-style subscriptions via `SubscribeOnFlightLoaded`, `SubscribeOnAircraftLoaded`, `SubscribeOnFlightPlanActivated`, `SubscribeOnObjectAdded`, `SubscribeOnObjectRemoved`.

Run:

```bash
go run ./examples/simconnect-events
```

The program will print both handler-invoked messages and subscription messages when events occur in the simulator.
