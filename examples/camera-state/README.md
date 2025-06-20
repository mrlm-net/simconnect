# Camera State Example

An example demonstrating how to monitor and control camera state in the flight simulator using SimConnect.

## Features

- Monitor current camera state/view via simulation variable
- Display real-time camera state changes  
- Cycle through different camera views programmatically
- Support for all major camera types (cockpit, external, drone, etc.)

## Usage

```bash
go run main.go
```

The program will:
1. Connect to SimConnect and monitor camera state
2. Display current camera view in real-time
3. Allow cycling through camera views (implementation dependent)
4. Show camera state changes as they occur

## What it demonstrates

- Monitoring camera-related simulation variables (`CAMERA STATE`)
- Real-time tracking of view changes
- Handling different camera modes and states
- Continuous data monitoring with appropriate update frequencies

## Camera States

The example recognizes these camera states:
- **Cockpit** (2) - Internal cockpit view
- **External/Chase** (3) - External chase camera
- **Drone** (4) - Free-roaming drone camera
- **Fixed on Plane** (5) - Fixed external view
- **Environment** (6) - Environment/scenery camera
- **Six DoF** (7) - Six degrees of freedom camera
- **Gameplay** (8) - Gameplay-specific camera
- **Showcase** (9) - Showcase/cinematic camera
- **Drone Aircraft** (10) - Aircraft-focused drone camera

## Key Concepts

- **Camera Variables**: Accessing camera-related simulation variables
- **State Monitoring**: Continuous monitoring of camera state changes
- **View Management**: Understanding different camera modes available in the simulator
