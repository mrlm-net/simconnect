# Camera Controller Demo

This demo provides comprehensive camera monitoring and control for Microsoft Flight Simulator through a modern web interface.

## Features

### Real-time Camera Monitoring
- **Camera State**: Monitor current camera mode (Cockpit, External, Drone, etc.)
- **Camera Substate**: Track camera behavior (Locked, Unlocked, Quickview, Smart, Instrument)
- **View Type & Index**: See current view configuration within each camera mode
- **Gameplay Camera**: Monitor pitch and yaw angles in real-time

### Interactive Camera Control
- **Quick Camera Switching**: One-click buttons to switch between major camera modes
- **Cockpit Camera Controls**: Adjust height, zoom, speed, and momentum with live sliders
- **Drone Camera Controls**: Control FOV, focus, rotation speed, and travel speed
- **Smart Camera Integration**: Monitor smart camera state and target information

### Advanced Camera Parameters
- **Cockpit Camera**:
  - Height adjustment (0-100%)
  - Zoom/FOV control (0-100%)
  - Movement speed (0-100%)
  - Momentum settings (0-100%)
  - Zoom speed control (0-100%)
  - Upper position toggle
  - Instrument autoselect monitoring

- **External/Chase Camera**:
  - Headlook mode monitoring
  - Momentum and speed controls
  - Zoom and zoom speed adjustment

- **Drone Camera**:
  - Field of View control (0-100%)
  - Focus adjustment (0-100%)
  - Rotation speed (0-100%)
  - Travel speed (0-100%)
  - Follow mode monitoring
  - Lock state monitoring
  - Focus mode tracking (Auto/Manual/Deactivated)

## Usage

1. **Start Microsoft Flight Simulator** and load into a flight
2. **Run the demo**:
   ```bash
   go run main.go
   ```
   Or for verbose output:
   ```bash
   go run main.go -v
   ```
3. **Open your web browser** and navigate to `http://localhost:8080`
4. **Control cameras** using the web interface:
   - Click camera state buttons to switch views
   - Use sliders to adjust camera parameters in real-time
   - Monitor all camera states and settings

## Camera States Supported

Based on Microsoft Flight Simulator's camera system:

- **Cockpit (2)**: Interior aircraft view
- **External/Chase (3)**: External following camera
- **Drone (4)**: Free-flying drone camera
- **Fixed on Plane (5)**: Fixed external view
- **Environment (6)**: Environment-based camera
- **Six DoF (7)**: Six degrees of freedom camera
- **Gameplay (8)**: Standard gameplay camera
- **Showcase (9)**: Showcase/cinematic cameras
- **Drone Aircraft (10)**: Aircraft-focused drone view

## Technical Implementation

### SimVars Monitored
- `CAMERA STATE` - Current camera mode
- `CAMERA SUBSTATE` - Camera behavior state
- `CAMERA VIEW TYPE AND INDEX` - View configuration
- `CAMERA GAMEPLAY PITCH YAW` - Gameplay camera orientation
- `COCKPIT CAMERA *` - All cockpit camera parameters
- `CHASE CAMERA *` - All external camera parameters  
- `DRONE CAMERA *` - All drone camera parameters
- `SMART CAMERA *` - Smart camera system state

### Real-time Updates
- Updates camera data every frame for smooth monitoring
- Web interface refreshes at 10 FPS for responsive control
- Bidirectional communication (monitor changes from sim or web interface)

### Modern Web Interface
- **Tailwind CSS** for modern, responsive design
- **Glass morphism** effects and smooth animations
- **Real-time sliders** with immediate feedback
- **Color-coded sections** for different camera types
- **Status indicators** for connection and camera states

## Architecture

The demo follows the established SimConnect pattern:
1. **Data Definition**: Sets up SimConnect data structure for camera variables
2. **Real-time Monitoring**: Continuously requests camera data from the simulator
3. **Web Server**: Serves the control interface and API endpoints
4. **Bidirectional Control**: Allows both monitoring simulator changes and sending control commands

## Browser Compatibility

The web interface works with all modern browsers:
- Chrome/Chromium
- Firefox
- Safari
- Edge

## Notes

- Camera parameter changes are applied immediately
- Some camera settings may only take effect in specific camera modes
- The interface shows current values even when parameters are changed from within the simulator
- Smart camera features require compatible aircraft and scenarios

This demo showcases the full capability of SimConnect's camera system, providing both comprehensive monitoring and precise control through an intuitive web interface.
