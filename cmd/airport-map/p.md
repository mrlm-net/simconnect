# Miscellaneous Variables<no value>
The tables below indicate the requirements for the [Simulation Variables](../simulation-variables/) that can be used to get and set the different miscellaneous properties. For information on the units listed for each variable, please see here: [Simulation Variable Units](../simulation-variable-units/)

You can find a complete index of all available SimVars here: [SimVar Index](../simulation-variables/#simvar-index)


### General Misc.

{{< table-wrapper "colclass-2=font-size:0.75rem" "200px" "100px" "" "200px" "100px" >}}

| Simulation Variable | Index | Description | Units | Settable |
|---|---|---|---|:---:|
| {{< anchor id="ANIMATION-DELTA-TIME" />}}`ANIMATION DELTA TIME` | N/A | Difference of time between the current frame and the last frame where this SimObject has been animated | Seconds |  {{< check/bad >}}  |
| {{< anchor id="ARTIFICIAL-GROUND-ELEVATION" />}}`ARTIFICIAL GROUND ELEVATION` | N/A | In case scenery is not loaded for AI planes, this variable can be used to set a default surface elevation. | Feet |  {{< check/bad >}}  |
| {{< anchor id="CATEGORY" />}}`CATEGORY` | N/A | Returns the SimObject category as defined in either the [aircraft.cfg](../../../content-configuration/cfg-files/aircraft.cfg/) or the [sim.cfg](../../../content-configuration/cfg-files/sim.cfg/) files.  | String (aircraft.cfg):<span></span>{{< params/smallfont-list >}}"Airplane"<span></span>"Airship"<span></span>"Helicopter"<span></span>"HotAirBalloon"{{< /params/smallfont-list >}}<span></span>String (sim.cfg only):<span></span>{{< params/smallfont-list >}}"AircraftPilot"<span></span>"Animal"<span></span>"Boat"<span></span>"CharacterSim"<span></span>"Container"<span></span>"ControlTower"<span></span>"FlyingAnimal"<span></span>"GroundVehicle"<span></span>"GroundVehiclePilot"<span></span>"Human"<span></span>"Liquid"<span></span>"Parachute"<span></span>"ProceduralCharacter"<span></span>"PylonSim"<span></span>"SimpleObject"<span></span>"SimpleObjectSim"<span></span>"StaticObject"<span></span>"Train"<span></span>"VDGS"<span></span>"Viewer"<span></span>"VR"<span></span>"Winch"{{< /params/smallfont-list>}} |  {{< check/bad >}}  |
| {{< anchor id="CONTROLLABLE" />}}<span class="obsolete">`CONTROLLABLE`</span> | N/A | ***Not used by the simulation.*** | Bool |  {{< check/bad >}}  |
| {{< anchor id="CRASH-FLAG" />}}<span class="obsolete">`CRASH FLAG`</span> | N/A | Flag value that indicates the cause of a crash. | Enum:<span></span>{{< params/smallfont-list >}}0:[index] None<span></span>2:[index] Mountain<span></span>4:[index] General<span></span>6:[index] Building<span></span>8:[index] Splash<span></span>10:[index] Gear up<span></span>12:[index] Overstress<span></span>14:[index] Building<span></span>16:[index] Aircraft<span></span>18:[index] Fuel Truck{{< /params/smallfont-list >}} |  {{< check/bad >}}  |
| {{< anchor id="CRASH-SEQUENCE" />}}<span class="obsolete">`CRASH SEQUENCE`</span> | N/A | The state of the crash event sequence. | Enum:<span></span>{{< params/smallfont-list >}}0:[index] off<span></span>1:[index] complete<span></span>3:[index] reset<span></span>4:[index] pause<span></span>11:[index] start{{< /params/smallfont-list >}} |  {{< check/bad >}}  |
| {{< anchor id="GROUND-ALTITUDE" />}}`GROUND ALTITUDE` | N/A | Altitude of surface.<span></span>**NOTE**: This SimVar will always be calculated from the user aircraft position, regardless of where the camera is, or what aircraft is selected. | Meters |  {{< check/bad >}}  |
| {{< anchor id="HAND-ANIM-STATE" />}}`HAND ANIM STATE` | N/A | What frame of the hand is currently used. | Enum:<span></span>{{< params/smallfont-list >}}0 = Hover<span></span>1 = Index push/Point<span></span>2 = Pinch large<span></span>3 = Pinch medium<span></span>4 = Pinch small<span></span>5 = Pinch lateral<span></span>6 = Press all/Fist<span></span>7 = Toggle lever up/down<span></span>8 = Thumb push<span></span>9 = Holde yoke thin<span></span>10 = Hold throttle<span></span>11 = Hold yokes<span></span>12 = Push/pull lever{{< /params/smallfont-list >}} |  {{< check/good >}}  |
| {{< anchor id="HIDE-AVATAR-IN-AIRCRAFT" />}}`HIDE AVATAR IN AIRCRAFT` | N/A | Return the status of the [`hide_avatar`](../../../content-configuration/cfg-files/aircraft.cfg/#hide_avatar) variable from the [aircraft.cfg](../../../content-configuration/cfg-files/aircraft.cfg/) file. | Boolean |  {{< check/bad >}}  |
| {{< anchor id="IDLE-ANIMATION-ID" />}}<span class="obsolete">`IDLE ANIMATION ID`</span> | N/A | The ID of the idle animation for the sim object.<span></span>***Not used by the simulation.*** | String |  {{< check/bad >}}  |
| {{< anchor id="MAGVAR" />}}`MAGVAR` | N/A | Magnetic variation.<span></span>**NOTE**: This SimVar will always be calculated from the user aircraft position, regardless of where the camera is, or what aircraft is selected. | Degrees |  {{< check/bad >}}  |
| {{< anchor id="MISSION-SCORE" />}}`MISSION SCORE` | N/A |  | Number |  {{< check/good >}}  |
| {{< anchor id="MOTION-SIMULATION" />}}`MOTION SIMULATION` | N/A | This will be `TRUE` when the motion simulation for the user aircraft is *active* and `FALSE` when it is *inactive*. The motion simulation for the user aircraft is **paused** when:<span></span>The user is in the menu screens<span></span>During the {{< glossterm >}}rtc{{< /glossterm >}} phase when starting a flight *in the air* (when on the ground the motion simulation is not active).<span></span>The aircraft is in free flight and the flight is paused or the aircraft is on Active Pause. | Bool |  {{< check/bad >}}  |
| {{< anchor id="PARACHUTE-OPEN" />}}`PARACHUTE OPEN` | N/A | This will be `TRUE` if the parachute has opened and `FALSE` otherwise. Currently this is only applied to the Parachute SimObject used by [Winches](../../../content-configuration/modular-simobjects/simobjects/ground-vehicles/winch-and-parachute-definitions/). | Bool |  {{< check/good >}}  |
| {{< anchor id="REALISM" />}}`REALISM` | N/A | General realism percent. | Number |  {{< check/good >}}  |
| {{< anchor id="REALISM-CRASH-DETECTION" />}}`REALISM CRASH DETECTION` | N/A | `TRUE` indicates crash detection is turned on. | Bool |  {{< check/bad >}}  |
| {{< anchor id="REALISM-CRASH-WITH-OTHERS" />}}`REALISM CRASH WITH OTHERS` | N/A | `TRUE` indicates crashing with other aircraft is possible. | Bool |  {{< check/bad >}}  |
| {{< anchor id="SIM-DISABLED" />}}`SIM DISABLED` | N/A | Is sim disabled. | Bool |  {{< check/good >}}  |
| {{< anchor id="SIM-ON-GROUND" />}}`SIM ON GROUND` | N/A | On ground flag.<span></span>**NOTE**: This is available in multiplayer to all **far** aircraft. See here for more information: [Note On SimVars In Multiplayer](../simulation-variables/#note-on-simvars-in-multiplayer). | Bool |  {{< check/bad >}}  |
| {{< anchor id="SIM-SHOULD-SET-ON-GROUND" />}}`SIM SHOULD SET ON GROUND` | N/A | This is a "read only" flag that is only ever used internally and has no use to third parties. If you need to know if the user aircraft is on the ground then use `IS ON GROUND`, and if you need to set the aircraft on the ground then use [`SIMCONNECT_DATA_INITPOSITION`](../../simconnect/api-reference/structures-and-enumerations/simconnect_data_initposition/). | Bool |  {{< check/bad >}}  |
| {{< anchor id="TRACK-IR-ENABLE" />}}`TRACK IR ENABLE` | N/A | Returns `TRUE` if [Track IR](https://www.trackir.fr/en/) is enabled or not. | Bool |  {{< check/good >}}  |
| {{< anchor id="TOTAL-WORLD-VELOCITY" />}}`TOTAL WORLD VELOCITY` | N/A | Speed relative to the earths center. | Feet per second |  {{< check/bad >}}  |
| {{< anchor id="USER-INPUT-ENABLED" />}}`USER INPUT ENABLED` | N/A | Is input allowed from the user. | Bool |  {{< check/good >}}  |
| {{< anchor id="VISUAL-MODEL-RADIUS" />}}`VISUAL MODEL RADIUS` | N/A | Model radius. | Meters |  {{< check/bad >}}  |
| {{< anchor id="VELOCITY-WORLD-X" />}}`VELOCITY WORLD X` | N/A | Speed relative to earth, in East/West direction.<span></span>**NOTE**: This is available in multiplayer to all **far** aircraft. See here for more information: [Note On SimVars In Multiplayer](../simulation-variables/#note-on-simvars-in-multiplayer). | Feet per second |  {{< check/good >}}  |
| {{< anchor id="VELOCITY-WORLD-Y" />}}`VELOCITY WORLD Y` | N/A | Speed relative to earth, in vertical direction.<span></span>**NOTE**: This is available in multiplayer to all **far** aircraft. See here for more information: [Note On SimVars In Multiplayer](../simulation-variables/#note-on-simvars-in-multiplayer). | Feet per second |  {{< check/good >}}  |
| {{< anchor id="VELOCITY-WORLD-Z" />}}`VELOCITY WORLD Z` | N/A | Speed relative to earth, in North/South direction.<span></span>**NOTE**: This is available in multiplayer to all **far** aircraft. See here for more information: [Note On SimVars In Multiplayer](../simulation-variables/#note-on-simvars-in-multiplayer). | Feet per second |  {{< check/good >}}  |
| {{< anchor id="VR-GROUP-INTERACTION-ON" />}}`VR GROUP INTERACTION ON` | N/A | This will return 1 (`TRUE`) if VR is on *and* group interaction is on. Otherwise it will return 0 (`FALSE`). | Bool |  {{< check/bad >}}  |

{{< /table-wrapper >}}


### Race Pylons

{{< table-wrapper "colclass-2=font-size:0.75rem" "200px" "100px" "" "150px" "100px" >}}

| Simulation Variable   | Index | Description   | Units  |Settable     |
|---|---|---|---|:---:|
| {{< anchor id="PYLON-HEIGHT" />}}`PYLON HEIGHT` | N/A | The height of the pylon.  | Meters | {{< check/bad >}} |
| {{< anchor id="PYLON-COLLISION-POSITION" />}}`PYLON COLLISION POSITION` | N/A | Distance to the collision object relative to the pylon. | Meters | {{< check/bad >}} |

{{< /table-wrapper >}}


### Fauna

{{< table-wrapper "colclass-2=font-size:0.75rem" "200px" "100px" "" "150px" "100px" >}}

| Simulation Variable     | Index | Description | Units   |Settable     |
|---|---|---|---|:---:|
| {{< anchor id="FAUNA-BEHAVIOUR-STATE" />}}`FAUNA BEHAVIOUR STATE` | N/A | Name of the Fauna Behaviour State associated with an animation transition (only for animals)    | String  | {{< check/bad >}} |
| {{< anchor id="FAUNA-FORCE-MAGNITUDE" />}}`FAUNA FORCE MAGNITUDE` | N/A | Force applied to animal while moving on a 0.0 - 1.0 scale (only for living world animals) | Number  | {{< check/bad >}} |
| {{< anchor id="FAUNA-HEADING-ROTATION-ANGLE" />}}`FAUNA HEADING ROTATION ANGLE` | N/A | Angle between where the animal is facing and where it intends to face (only for living world animals) | Degrees | {{< check/bad >}} |
| {{< anchor id="FAUNA-TURN-RATE" />}}`FAUNA TURN RATE` | N/A | Direction and rate of turning for animals when moving (only for living world animals)     | Number  | {{< check/bad >}} |

{{< /table-wrapper >}}


### Humans

{{< table-wrapper "colclass-2=font-size:0.75rem" "200px" "100px" "" "150px" "100px" >}}

| Simulation Variable | Index | Description | Units | Settable |
|---|---|---|---|:---:|
| {{< anchor id="GENDER" />}}`GENDER` | N/A | The gender of the corresponding SimObject (mainly used for human SimObjects). | Number:<span></span>{{< params/smallfont-list >}}0 = Male<span></span>1 = Female{{< /params/smallfont-list >}} |  {{< check/good >}}  |
| {{< anchor id="HEIGHT" />}}`HEIGHT` | N/A | The height of the corresponding SimObject (mainly used for human SimObjects). | Number |  {{< check/good >}}  |
| {{< anchor id="IS-CROUCH" />}}`IS CROUCH` | N/A | This will return 1 (`TRUE`) if the object is in the crouched state, or 0 (`FALSE`) otherwise (only used for human SimObjects, specifically *only* the user avatar). | Bool |  {{< check/good >}}  |
| {{< anchor id="IS-TALKING" />}}`IS TALKING` | N/A | This will return 1 (`TRUE`) if the object is in the crouched state, or 0 (`FALSE`) otherwise (only used for human SimObjects, specifically *only* the user avatar). | Bool |  {{< check/good >}}  |

{{< /table-wrapper >}}


### Ambient / Weather

{{< table-wrapper "colclass-2=font-size:0.75rem" "200px" "100px" "" "150px" "100px" >}}

| Simulation Variable | Index | Description | Units | Settable |
|---|---|---|---|:---:|
| {{< anchor id="AMBIENT-DENSITY" />}}`AMBIENT DENSITY` | N/A | Ambient density.<span></span>**NOTE**: This SimVar will always be calculated from the user aircraft position, regardless of where the camera is, or what aircraft is selected. | Slugs per cubic feet |  {{< check/bad >}}  |
| {{< anchor id="AMBIENT-IN-CLOUD" />}}`AMBIENT IN CLOUD` | N/A | `TRUE` if the aircraft is in a cloud. | Bool |  {{< check/bad >}}  |
| {{< anchor id="AMBIENT-IN-SMOKE" />}}`AMBIENT IN SMOKE` | N/A | `TRUE` if the camera position is flying through smoke. Note that if this is `TRUE`, then the aircraft will be taking increased [Wear and Tear](../../../content-configuration/cfg-files/flight-model/additional-flight-model-information/#wear-and-tear) damage.<span></span>**NOTE**: This SimVar will always be calculated from the camera position, regardless of where the user aircraft is, or what aircraft is selected. | Bool |  {{< check/bad >}}  |
| {{< anchor id="AMBIENT-PRECIP-RATE" />}}`AMBIENT PRECIP RATE` | N/A | The current precipitation rate.<span></span>**NOTE**: This SimVar will always be calculated from the user aircraft position, regardless of where the camera is, or what aircraft is selected. | millimeters of water |  {{< check/bad >}}  |
| {{< anchor id="AMBIENT-PRECIP-STATE" />}}`AMBIENT PRECIP STATE` | N/A | The current state of precipitation.<span></span>**NOTE**: This SimVar will always be calculated from the user aircraft position, regardless of where the camera is, or what aircraft is selected. | Mask:<span></span>{{< params/smallfont-list >}}2 = None<span></span>4 = Rain<span></span>8 = Snow{{< /params/smallfont-list >}} |  {{< check/bad >}}  |
| {{< anchor id="AMBIENT-PRESSURE" />}}`AMBIENT PRESSURE` | N/A | Ambient pressure.<span></span>**NOTE**: This SimVar will always be calculated from the user aircraft position, regardless of where the camera is, or what aircraft is selected. | Inches of mercury, {{< glossterm >}}inhg{{< /glossterm >}} |  {{< check/bad >}}  |
| {{< anchor id="AMBIENT-TEMPERATURE" />}}`AMBIENT TEMPERATURE` | N/A | Ambient temperature.<span></span>**NOTE**: This SimVar will always be calculated from the user aircraft position, regardless of where the camera is, or what aircraft is selected. | Celsius |  {{< check/bad >}}  |
| {{< anchor id="AMBIENT-VISIBILITY" />}}`AMBIENT VISIBILITY` | N/A | Ambient visibility (only measures ambient particle visibility - related to ambient density). | Meters |  {{< check/bad >}}  |
| {{< anchor id="AMBIENT-WIND-DIRECTION" />}}`AMBIENT WIND DIRECTION` | N/A | Wind direction, relative to true north.<span></span>**NOTE**: This SimVar will always be calculated from the user aircraft position, regardless of where the camera is, or what aircraft is selected. | Degrees |  {{< check/bad >}}  |
| {{< anchor id="AMBIENT-WIND-VELOCITY" />}}`AMBIENT WIND VELOCITY` | N/A | Wind velocity.<span></span>**NOTE**: This SimVar will always be calculated from the user aircraft position, regardless of where the camera is, or what aircraft is selected. | {{< glossterm >}}knot{{< /glossterm >}}s |  {{< check/bad >}}  |
| {{< anchor id="AMBIENT-WIND-X" />}}`AMBIENT WIND X` | N/A | Wind component in East/West direction.<span></span>**NOTE**: This SimVar will always be calculated from the user aircraft position, regardless of where the camera is, or what aircraft is selected. | Meters per second |  {{< check/bad >}}  |
| {{< anchor id="AMBIENT-WIND-Y" />}}`AMBIENT WIND Y` | N/A | Wind component in vertical direction.<span></span>**NOTE**: This SimVar will always be calculated from the user aircraft position, regardless of where the camera is, or what aircraft is selected. | Meters per second |  {{< check/bad >}}  |
| {{< anchor id="AMBIENT-WIND-Z" />}}`AMBIENT WIND Z` | N/A | Wind component in North/South direction.<span></span>**NOTE**: This SimVar will always be calculated from the user aircraft position, regardless of where the camera is, or what aircraft is selected. | Meters per second |  {{< check/bad >}}  |
| {{< anchor id="BAROMETER-PRESSURE" />}}`BAROMETER PRESSURE` | N/A | Barometric pressure.<span></span>**NOTE**: This SimVar will always be calculated from the user aircraft position, regardless of where the camera is, or what aircraft is selected. | Millibars |  {{< check/bad >}}  |
| {{< anchor id="DENSITY-ALTITUDE" />}}`DENSITY ALTITUDE` | N/A | The density altitude is the altitude relative to standard atmospheric conditions at which the air density would be equal to the indicated air density at the place of observation. The calculation is as follows:<span></span>`density_altitude = pressure_altitude + 118.8 * (outside_air_temp - ISA_temp)` | {{< glossterm >}}ft{{< /glossterm >}} |  {{< check/bad >}}  |
| {{< anchor id="ENV-CLOUD-DENSITY" />}}`ENV CLOUD DENSITY` | N/A | This SimVar returns the current density of the clouds at the position of the user aircraft. Density is expressed as a value between 0 and 1, where 0 is no clouds, and 1 is very dense cloud.<span></span>**NOTE**: This SimVar will always be calculated from the user aircraft position, regardless of where the camera is, or what aircraft is selected. | {{< glossterm >}}percent_over_100{{< /glossterm >}} |  {{< check/bad >}}  |
| {{< anchor id="ENV-SMOKE-DENSITY" />}}`ENV SMOKE DENSITY` | N/A | This SimVar returns the current density of any smoke at the position of the camera. Density is expressed as a value between 0 and 1, where 0 is no smoke, and 1 is very dense smoke. Note that if this value is greater than 0 then the user aircraft will be taking [Wear and Tear](../../../content-configuration/cfg-files/flight-model/additional-flight-model-information/#wear-and-tear) damage, and the higher the value the more damage will be taken.<span></span>**NOTE**: This SimVar will always be calculated from the camera position, regardless of where the user aircraft is, or what aircraft is selected. | {{< glossterm >}}percent_over_100{{< /glossterm >}} |  {{< check/bad >}}  |
| {{< anchor id="SEA-LEVEL-AMBIENT-TEMPERATURE" />}}`SEA LEVEL AMBIENT TEMPERATURE` | N/A | The ambient temperature of the air at sea level.<span></span>**NOTE**: This SimVar will always be calculated from the user aircraft position, regardless of where the camera is, or what aircraft is selected. | Celsius |  {{< check/bad >}}  |
| {{< anchor id="SEA-LEVEL-PRESSURE" />}}`SEA LEVEL PRESSURE` | N/A | Barometric pressure at sea level.<span></span>**NOTE**: This SimVar will always be calculated from the user aircraft position, regardless of where the camera is, or what aircraft is selected. | Millibars |  {{< check/bad >}}  |
| {{< anchor id="STRUCT-AMBIENT-WIND" />}}`STRUCT AMBIENT WIND` | N/A | X (latitude), Y (vertical) and Z (longitude) components of the wind.<span></span>**NOTE**: This SimVar will always be calculated from the user aircraft position, regardless of where the camera is, or what aircraft is selected. | Feet per second |  {{< check/bad >}}  |

{{< /table-wrapper >}}


### AI<span class="obsolete"></span>

The following SimVars are still supported by Microsoft Flight Simulator 2024, but are considered legacy and will not be getting any further support or updates. As such, we don't recommend their use in new aircraft.

{{< table-wrapper "colclass-2=font-size:0.75rem" "200px" "100px" "" "150px" "100px" >}}

| Simulation Variable | Index | Description | Units | Settable |
|---|---|---|---|:---:|
| {{< anchor id="AI-DESIRED-SPEED" />}}<span class="obsolete">`AI DESIRED SPEED`</span> | N/A | The desired speed of the AI object. | {{< glossterm >}}knot{{< /glossterm >}}s |  {{< check/good >}}  |
| {{< anchor id="AI-WAYPOINT-LIST" />}}<span class="obsolete">`AI WAYPOINT LIST`</span> | N/A | A list of waypoints that an AI controlled object should follow. | `SIMCONNECT_DATA_WAYPOINT` |  {{< check/good >}}  |
| {{< anchor id="AI-CURRENT-WAYPOINT" />}}<span class="obsolete">`AI CURRENT WAYPOINT`</span> | N/A | The current waypoint in the list. | Number |  {{< check/good >}}  |
| {{< anchor id="AI-DESIRED-HEADING" />}}<span class="obsolete">`AI DESIRED HEADING`</span> | N/A | The desired heading of the AI object. | Degrees |  {{< check/good >}}  |
| {{< anchor id="AI-GROUNDTURNTIME" />}}<span class="obsolete">`AI GROUNDTURNTIME`</span> | N/A | The time required by the AI aircraft to make a 90º turn. | Seconds |  {{< check/good >}}  |
| {{< anchor id="AI-GROUNDCRUISESPEED" />}}<span class="obsolete">`AI GROUNDCRUISESPEED`</span> | N/A | The ground cruising speed for the AI aircraft. | {{< glossterm >}}knot{{< /glossterm >}}s |  {{< check/good >}}  |
| {{< anchor id="AI-GROUNDTURNSPEED" />}}<span class="obsolete">`AI GROUNDTURNSPEED`</span> | N/A | The ground turning speed for the AI aircraft. | {{< glossterm >}}knot{{< /glossterm >}}s |  {{< check/good >}}  |
| {{< anchor id="AI-TRAFFIC-ISIFR" />}}<span class="obsolete">`AI TRAFFIC ISIFR`</span> | N/A | This can be used to request whether the AI aircraft is IFR or VFR. Note that if an aircraft does not have a flight plan, the value returned will be 0 (or `FALSE`). | Bool |  {{< check/bad >}}  |
| {{< anchor id="AI-TRAFFIC-STATE" />}}<span class="obsolete">`AI TRAFFIC STATE`</span> | N/A | This will return a string describing an AI aircraft state. If the aircraft is under ATC control the string will be one of the following:<span></span>"init"<span></span>"sleep"<span></span>"flt plan"<span></span>"startup"<span></span>"preflight support"<span></span>"clearance"<span></span>"push back 1"<span></span>"push back 2"<span></span>"pre taxi out"<span></span>"taxi out"<span></span>"takeoff 1"<span></span>"takeoff 2"<span></span>"T&G depart"<span></span>"enroute"<span></span>"pattern"<span></span>"landing"<span></span>"rollout"<span></span>"go around"<span></span>"taxi in"<span></span>"shutdown"<span></span>"postflight support"<span></span>If the aircraft is not under ATC control, the string will be one of these:<span></span>"Sleep"<span></span>"Waypoint"<span></span>"Takeoff"<span></span>"Landing"<span></span>"Taxi"<span></span>Note that if an aircraft does not have a flight plan, the value returned will be an empty string "". | String |  {{< check/bad >}}  |
| {{< anchor id="AI-TRAFFIC-CURRENT-AIRPORT" />}}<span class="obsolete">`AI TRAFFIC CURRENT AIRPORT`</span> | N/A | The {{< glossterm >}}icao_code{{< /glossterm >}} of the current airport. If an aircraft does not have a flight plan, the value returned will be an empty string "". | String |  {{< check/bad >}}  |
| {{< anchor id="AI-TRAFFIC-ASSIGNED-RUNWAY" />}}<span class="obsolete">`AI TRAFFIC ASSIGNED RUNWAY`</span> | N/A | The assigned runway name (for example: "32R"). If an aircraft does not have a flight plan, the value returned will be an empty string "". | String |  {{< check/bad >}}  |
| {{< anchor id="AI-TRAFFIC-ASSIGNED-PARKING" />}}<span class="obsolete">`AI TRAFFIC ASSIGNED PARKING`</span> | N/A | The assigned parking name. The string will take the form:<span></span>`Name + Number, Type ( radius )`<span></span>For example:<span></span>"Ramp 1, RAMP sml (10m)"<span></span>"Gate G 4, RAMP lrg (18m)"<span></span>If an aircraft does not have a flight plan, the value returned will be an empty string "". | String |  {{< check/bad >}}  |
| {{< anchor id="AI-TRAFFIC-FROMAIRPORT" />}}<span class="obsolete">`AI TRAFFIC FROMAIRPORT`</span> | N/A | The {{< glossterm >}}icao_code{{< /glossterm >}} of the departure airport in the current schedule.<span></span>This variable is only valid for aircraft generated by the traffic database that have schedules. If an aircraft does not have a schedule, the value returned will an empty string"". | String |  {{< check/bad >}}  |
| {{< anchor id="AI-TRAFFIC-TOAIRPORT" />}}<span class="obsolete">`AI TRAFFIC TOAIRPORT`</span> | N/A | The {{< glossterm >}}icao_code{{< /glossterm >}} of the destination airport in the current schedule.<span></span>This variable is only valid for aircraft generated by the traffic database that have schedules. If an aircraft does not have a schedule, the value returned will an empty string"". | String |  {{< check/bad >}}  |
| {{< anchor id="AI-TRAFFIC-ETD" />}}<span class="obsolete">`AI TRAFFIC ETD`</span> | N/A | The estimated time of departure for the current schedule entry, given as the number of seconds difference from the current simulation time. This can be negative if ETD is earlier than the current simulation time<span></span>This variable is only valid for aircraft generated by the traffic database that have schedules. If an aircraft does not have a schedule, the value returned will be 0. | Seconds |  {{< check/bad >}}  |
| {{< anchor id="AI-TRAFFIC-ETA" />}}<span class="obsolete">`AI TRAFFIC ETA`</span> | N/A | Estimated time of arrival for the current schedule entry, given as the number of seconds difference from the current simulated time. This can be negative if ETA is earlier than the current simulated time.<span></span>This variable is only valid for aircraft generated by the traffic database that have schedules. If an aircraft does not have a schedule, the value returned will be 0. | Seconds |  {{< check/bad >}}  |

{{< /table-wrapper >}}


### Structures

{{< table-wrapper "colclass-2=font-size:0.75rem" "200px" "100px" "" "150px" "100px" >}}

| Simulation Variable | Index | Description | Units | Settable |
|---|---|---|---|:---:|
| {{< anchor id="STRUCT-DAMAGEVISIBLE" />}}<span class="obsolete">`STRUCT DAMAGEVISIBLE`</span> | N/A | ***No longer used by the simulation.*** | - |  {{< check/bad >}}  |
| {{< anchor id="STRUC-AIRSPEED-HOLD-PID-CONSTS" />}}<span class="obsolete">`STRUC AIRSPEED HOLD PID CONSTS`</span> | N/A | Returns the various airspeed {{< glossterm >}}pid{{< /glossterm >}} constants. This is generally only used for AI controlled aircraft and boats, although it may be useful when working with RTCs and the user aircraft.<span></span>***Not used in the simulation.*** | [`PID_STRUCT`](../simulation-variable-units/#PID_STRUCT) |  {{< check/bad >}}  |
| {{< anchor id="STRUC-HEADING-HOLD-PID-CONSTS" />}}<span class="obsolete">`STRUC HEADING HOLD PID CONSTS`</span> | N/A | Returns the various airspeed {{< glossterm >}}pid{{< /glossterm >}} constants. This is generally only used for AI controlled aircraft and boats, although it may be useful when working with RTCs and the user aircraft.<span></span>***Not used in the simulation.*** | [`PID_STRUCT`](../simulation-variable-units/#PID_STRUCT) |  {{< check/bad >}}  |
| {{< anchor id="STRUCT-BODY-ROTATION-ACCELERATION" />}}<span class="obsolete">`STRUCT BODY ROTATION ACCELERATION`</span> | N/A | The body rotation acceleration.<span></span>***Not used in the simulation.*** | [Struct `XYZ`](../simulation-variable-units/#XYZ)<span></span>radians per second |  {{< check/bad >}}  |
| {{< anchor id="STRUCT-PBH32" />}}<span class="obsolete">`STRUCT PBH32`</span> | N/A | Returns a pitch, bank and heading value (for what will depend on the SimVar being used).<span></span>***No longer used by the simulation.*** | [Struct `PBH`](../simulation-variable-units/#PBH)<span></span>degrees |  {{< check/bad >}}  |
| {{< anchor id="STRUCT-REALISM-VARS" />}}<span class="obsolete">`STRUCT REALISM VARS`</span> | N/A | ***No longer used by the simulation.*** | - |  {{< check/bad >}}  |
| {{< anchor id="STRUCT-SURFACE-RELATIVE-VELOCITY" />}}`STRUCT SURFACE RELATIVE VELOCITY` | N/A | The relative surface velocity. | [Struct `XYZ`](../simulation-variable-units/#XYZ) structure, feet per second |  {{< check/bad >}}  |
| {{< anchor id="STRUCT-WORLDVELOCITY" />}}`STRUCT WORLDVELOCITY` | N/A | The world velocity. | [Struct `XYZ`](../simulation-variable-units/#XYZ) structure, feet per second |  {{< check/bad >}}  |
| {{< anchor id="STRUCT-WORLD-ACCELERATION" />}}`STRUCT WORLD ACCELERATION` | N/A | The world acceleration for each axis. Individual world acceleration values are in the Aircraft Position and Speed section. | [Struct `XYZ`](../simulation-variable-units/#XYZ) structure, feet per second squared |  {{< check/bad >}}  |
| {{< anchor id="STRUCT-WORLD-ROTATION-VELOCITY" />}}`STRUCT WORLD ROTATION VELOCITY` | N/A | The world rotation velocity. | [Struct `XYZ`](../simulation-variable-units/#XYZ) structure, radians per second |  {{< check/bad >}}  |

{{< /table-wrapper >}}


### CFS Variables

{{< table-wrapper "colclass-2=font-size:0.75rem" "200px" "100px" "" "150px" "100px" >}}

| Simulation Variable| Index | Description  | Units  |   Settable   |
|---|---|---|---|:---:|
| {{< anchor id="BOMB-AMMO" />}}<span class="obsolete">`BOMB AMMO`</span> | N/A | The amount of bomb ammunition available.   | Number | {{< check/good >}} |
| {{< anchor id="CANNON-AMMO" />}}<span class="obsolete">`CANNON AMMO`</span> | N/A | The amount of cannon ammunition available. | Number | {{< check/good >}} |
| {{< anchor id="GUN-AMMO" />}}<span class="obsolete">`GUN AMMO`</span> | N/A | The amount of gun ammunition available.    | Number | {{< check/good >}} |
| {{< anchor id="ROCKET-AMMO" />}}<span class="obsolete">`ROCKET AMMO`</span> | N/A | The amount of rocket ammunition available. | Number | {{< check/good >}} |

{{< /table-wrapper >}}