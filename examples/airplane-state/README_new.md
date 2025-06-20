# Aircraft State Monitor with Web GUI

## Overview

This demo application provides a comprehensive real-time aircraft state monitoring system for Microsoft Flight Simulator using SimConnect. It displays essential flight data through a modern web interface with live updates at 20 FPS.

## Features

### Real-time Data Display
- **Flight Data**: Altitude, airspeed (IAS/TAS), ground speed, vertical speed, Mach number
- **Position & Attitude**: Latitude/longitude coordinates, pitch/bank angles, heading (magnetic/true)
- **Surface Information**: Runway status, parking state, surface type/condition
- **Environmental**: Total air temperature, standard atmosphere temperature, user aircraft status

### Web Interface
- **Modern UI**: Dark theme with color-coded sections using Tailwind CSS
- **Real-time Updates**: 50ms refresh rate for smooth data visualization
- **Connection Status**: Live indicator showing SimConnect connection health
- **Responsive Design**: Works on desktop and mobile devices

## Technical Implementation

### SimConnect Data Architecture

The application follows a carefully designed pattern for SimConnect data handling:

1. **Data Structure Alignment**: The `AircraftData` struct must exactly match the order and count of SimVars added to the data definition
2. **Pointer Casting**: Uses `unsafe.Pointer` casting to map SimConnect data directly to the struct (following working examples)
3. **Unit Conversions**: Properly converts radians to degrees for attitude/position data, Rankine to Fahrenheit for temperature
4. **Thread Safety**: Uses mutex protection for concurrent access to aircraft state data

### Working SimVars (19 total)

All SimVars have been verified against official MSFS documentation and tested with real aircraft:

#### Basic Flight Data (0-11)
- `PLANE ALTITUDE` (feet)
- `GROUND VELOCITY` (knots)
- `PLANE LATITUDE` (radians → degrees)
- `PLANE LONGITUDE` (radians → degrees)
- `VERTICAL SPEED` (feet per minute)
- `PLANE PITCH DEGREES` (radians → degrees)
- `PLANE BANK DEGREES` (radians → degrees)
- `PLANE HEADING DEGREES TRUE` (radians → degrees)
- `PLANE HEADING DEGREES MAGNETIC` (radians → degrees)
- `AIRSPEED INDICATED` (knots)
- `AIRSPEED TRUE` (knots)
- `AIRSPEED MACH` (mach)

#### Surface and Aircraft State (12-18)
- `ON ANY RUNWAY` (boolean)
- `PLANE IN PARKING STATE` (boolean)
- `SURFACE TYPE` (enum)
- `SURFACE CONDITION` (enum)
- `TOTAL AIR TEMPERATURE` (celsius)
- `STANDARD ATM TEMPERATURE` (rankine → fahrenheit)
- `IS USER SIM` (boolean)

### Known Issues and Limitations

#### Removed SimVars
The following SimVars were removed during testing due to compatibility issues:
- **Control Surface Data**: `YOKE X/Y POSITION`, `ELEVATOR DEFLECTION PCT`, `AILERON AVERAGE DEFLECTION`, `RUDDER DEFLECTION PCT`
- **Warning Systems**: `WARNING FUEL`, `WARNING OIL PRESSURE`, `WARNING VOLTAGE`
- **Ice Detection**: `STRUCTURAL ICE PCT`

These SimVars either caused SimConnect exceptions or returned invalid data with certain aircraft models.

### Resolution Process

The development followed a systematic debugging approach:

1. **Start Simple**: Begin with one known-good SimVar (`PLANE ALTITUDE`)
2. **Incremental Addition**: Add SimVars one by one to identify problematic ones
3. **Struct Alignment**: Ensure the data structure exactly matches the definition order
4. **Official Documentation**: Use MSFS documentation for correct SimVar names and units
5. **Real Testing**: Test with actual aircraft in MSFS to verify data accuracy

### Validated Test Results

Testing with aircraft at Prague (LKPR):
```
Position: 50.106289°N, 14.262050°E, 1180.2 ft
Speed: IAS=0.0 TAS=0.0 GS=0.0 VS=0.0 M=0.000
Attitude: P=0.00° B=0.00° H(T)=0.0° H(M)=0.0°
Surface: Type=4 Cond=0 Runway=false Parking=true
Environment: TAT=15.0°C SAT=59.0°F UserSim=true
```

## Usage

### Prerequisites
- Windows OS (SimConnect requirement)
- Microsoft Flight Simulator running
- Go 1.19+ installed

### Running the Application

1. **Start MSFS**: Launch Microsoft Flight Simulator and load an aircraft
2. **Run Monitor**: Execute the Go application
   ```bash
   cd examples/airplane-state
   go run main.go
   ```
3. **Open Web Interface**: Navigate to `http://localhost:8080`
4. **Monitor Data**: Real-time aircraft data will display automatically

### Configuration

- **Web Port**: Change `WEB_PORT` constant to use different port
- **Update Rate**: Modify `setInterval(updateData, 50)` in HTML for different refresh rates
- **Data Request**: Uses `SIMCONNECT_PERIOD_SIM_FRAME` for maximum update frequency

## Architecture Details

### Data Flow
1. **SimConnect Connection**: Establishes connection to MSFS
2. **Data Definition**: Registers all SimVars with specific types and units
3. **Data Request**: Requests continuous updates on sim frame basis
4. **Message Processing**: Handles incoming SimConnect messages
5. **Data Parsing**: Casts raw data to structured format
6. **Web API**: Serves data via JSON endpoint with mutex protection
7. **Frontend Update**: JavaScript polls API and updates DOM elements

### Error Handling
- **Connection Monitoring**: Displays connection status in web interface
- **Data Validation**: Validates altitude ranges and other critical values
- **Exception Handling**: Logs SimConnect exceptions without crashing
- **Graceful Shutdown**: Proper cleanup on SIGTERM/SIGINT

### Performance Considerations
- **Efficient Updates**: Only updates changed data using `SIMCONNECT_DATA_REQUEST_FLAG_CHANGED`
- **Minimal Overhead**: Direct struct casting avoids unnecessary data copying
- **Thread Safety**: Mutex protection prevents race conditions
- **Optimized Frontend**: 50ms update interval balances responsiveness with performance

## Development Notes

### Best Practices Learned
1. **Always test incrementally** when adding new SimVars
2. **Use official documentation** for SimVar names and units
3. **Match struct order exactly** to data definition order
4. **Implement proper error handling** for SimConnect exceptions
5. **Test with multiple aircraft** to ensure compatibility
6. **Use thread-safe patterns** for concurrent data access

### Common Pitfalls Avoided
- Mismatched struct field order causing data corruption
- Missing unit conversions for angle and temperature data
- Race conditions in multi-threaded access to shared state
- Using invalid SimVar names that cause SimConnect exceptions
- Blocking UI operations with synchronous SimConnect calls

## Future Enhancements

Potential improvements for advanced implementations:
- Engine parameters (RPM, fuel flow, temperatures)
- Navigation data (GPS coordinates, waypoints)
- Communication frequencies and transponder settings
- Autopilot status and settings
- Weather and atmospheric conditions
- Multiple aircraft support for multiplayer scenarios

This implementation serves as a solid foundation for more complex flight simulator integrations while maintaining reliability and performance.
