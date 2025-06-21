# Event System

Advanced SimConnect event handling, mapping, and triggering patterns.

## Event Mapping

### Basic Event Mapping

```go
// Map client event to SimConnect event
client.MapClientEventToSimEvent(
    EVENT_TOGGLE_EXTERNAL_POWER, // Client-side event ID
    "TOGGLE_EXTERNAL_POWER",     // SimConnect event name
)
```

### Event Groups

Group related events for organized management:

```go
const (
    GROUP_ENGINES = 1
    GROUP_LIGHTS  = 2
    GROUP_FLIGHT_CONTROLS = 3
)

// Add events to groups
client.AddClientEventToNotificationGroup(GROUP_ENGINES, EVENT_ENGINE_START, false)
client.AddClientEventToNotificationGroup(GROUP_ENGINES, EVENT_ENGINE_STOP, false)
```

## Event Types

### Key Events

Most common event type - aircraft controls and systems:

```go
// Engine controls
client.MapClientEventToSimEvent(EVENT_ENGINE_START, "ENGINE_AUTO_START")
client.MapClientEventToSimEvent(EVENT_ENGINE_STOP, "ENGINE_AUTO_SHUTDOWN")

// Flight controls  
client.MapClientEventToSimEvent(EVENT_GEAR_TOGGLE, "GEAR_TOGGLE")
client.MapClientEventToSimEvent(EVENT_FLAPS_UP, "FLAPS_UP")
```

### System Events

Monitor simulator state changes:

```go
// System state monitoring
client.SubscribeToSystemEvent(EVENT_PAUSED, "Paused")
client.SubscribeToSystemEvent(EVENT_UNPAUSED, "Unpaused")
client.SubscribeToSystemEvent(EVENT_CRASHED, "Crashed")
```

## Event Parameters

### Simple Events

Events without parameters:

```go
client.TransmitClientEvent(EVENT_GEAR_TOGGLE, 0)
client.TransmitClientEvent(EVENT_AUTOPILOT_TOGGLE, 0)
```

### Parametric Events

Events with specific values:

```go
// Set specific COM frequency (parameter in Hz * 1000000)
client.TransmitClientEvent(EVENT_COM_RADIO_SET, 125750000) // 125.75 MHz

// Set autopilot altitude (parameter in feet)
client.TransmitClientEvent(EVENT_AP_ALT_SET, 10000)

// Aircraft doors (door number 1-4)
client.TransmitClientEvent(EVENT_TOGGLE_AIRCRAFT_EXIT, 1) // Door 1
```

### Advanced Parameters

Some events accept multiple parameter formats:

```go
// Heading bug set (0-359 degrees)
client.TransmitClientEvent(EVENT_HEADING_BUG_SET, 270) // Set to 270°

// Throttle set (0-16384 range, where 16384 = 100%)
client.TransmitClientEvent(EVENT_THROTTLE_SET, 8192) // 50% throttle
```

## Event Handling Patterns

### Event Confirmation

Check for event acknowledgment:

```go
case msg.IsEvent():
    if event, ok := msg.GetEvent(); ok {
        switch event.EventID {
        case EVENT_ENGINE_START:
            fmt.Println("Engine start event confirmed")
        case EVENT_GEAR_TOGGLE:
            fmt.Println("Gear toggle event confirmed")
        }
    }
```

### Error Handling

Handle event exceptions:

```go
case msg.IsException():
    if exception, ok := msg.GetException(); ok {
        switch exception.ExceptionCode {
        case types.EXCEPTION_UNRECOGNIZED_ID:
            fmt.Printf("Invalid event ID: %d\n", exception.SendID)
        case types.EXCEPTION_EVENT_ID_DUPLICATE:
            fmt.Printf("Duplicate event mapping: %d\n", exception.SendID)
        }
    }
```

## Common Event Categories

### Aircraft Systems

```go
// Electrical
"TOGGLE_EXTERNAL_POWER"
"TOGGLE_MASTER_BATTERY"
"TOGGLE_ALTERNATOR1"

// Engines  
"ENGINE_AUTO_START"
"ENGINE_AUTO_SHUTDOWN"
"TOGGLE_STARTER1"

// Flight Controls
"GEAR_TOGGLE"
"FLAPS_UP" / "FLAPS_DOWN"
"SPOILERS_TOGGLE"
```

### Autopilot

```go
// Master controls
"AP_MASTER"
"AP_PANEL_HEADING_HOLD"
"AP_PANEL_ALTITUDE_HOLD"

// Navigation
"AP_NAV1_HOLD"
"AP_APPROACH_HOLD"
"AP_BACKCOURSE_HOLD"
```

### Communication/Navigation

```go
// Radios
"COM_RADIO_SET_HZ"
"NAV1_RADIO_SET_HZ"
"XPNDR_SET"

// Navigation aids
"VOR1_OBI_INC"
"ADF_CARD_INC"
```

## Best Practices

### Event Timing

```go
// ❌ Rapid-fire events can overwhelm SimConnect
for i := 0; i < 10; i++ {
    client.TransmitClientEvent(EVENT_FLAPS_UP, 0)
}

// ✅ Add delays between events
client.TransmitClientEvent(EVENT_FLAPS_UP, 0)
time.Sleep(100 * time.Millisecond)
client.TransmitClientEvent(EVENT_FLAPS_UP, 0)
```

### Event Validation

```go
// Validate parameters before sending
func setAutopilotAltitude(client *client.Engine, altitude int) {
    if altitude < 0 || altitude > 50000 {
        fmt.Printf("Invalid altitude: %d\n", altitude)
        return
    }
    client.TransmitClientEvent(EVENT_AP_ALT_SET, uint32(altitude))
}
```
