# Camera State Example

Monitor and control MSFS camera views programmatically.

## What it demonstrates

- **Camera State Monitoring**: Track current camera view in real-time
- **Camera View Switching**: Programmatically change camera perspectives  
- **State Mapping**: Convert numeric camera states to readable names
- **Periodic Data Requests**: Regular camera state polling
- **View Cycle Control**: Automatic camera view cycling

## How to run

```bash
cd examples/camera-state
go run main.go
```

## Controls

| Key | Action |
|-----|--------|
| `c` | Cycle to next camera view |
| `1-9` | Switch to specific camera view |
| `q` | Quit application |
| Ctrl+C | Emergency shutdown |

## Camera Views

| ID | View Name | Description |
|----|-----------|-------------|
| 2 | Cockpit | Interior pilot view |
| 3 | External/Chase | Follow aircraft externally |
| 4 | Drone | Free-roaming drone camera |
| 5 | Fixed on Plane | Fixed external view |
| 6 | Environment | Environment/scenery focus |
| 7 | Six DoF | 6-degrees-of-freedom view |
| 8 | Gameplay | Gameplay-specific camera |
| 9 | Showcase | Cinematic showcase view |
| 10 | Drone Aircraft | Aircraft-focused drone |

## Key code patterns

```go
// Monitor camera state
client.AddToDataDefinition(CAMERA_STATE_DEFINITION, "CAMERA STATE", "Enum", types.DATATYPE_INT32)
client.RequestDataOnSimObject(CAMERA_STATE_REQUEST, CAMERA_STATE_DEFINITION, types.SIMOBJECT_TYPE_USER, types.PERIOD_SIM_FRAME)

// Switch camera views  
client.MapClientEventToSimEvent(eventID, "CHASE_VIEW_TOGGLE")
client.TransmitClientEvent(eventID, 0)
```

## Requirements

- Running MSFS with any aircraft or scenario
- Camera controls enabled in simulator settings
