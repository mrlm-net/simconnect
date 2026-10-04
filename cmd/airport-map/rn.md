# SDK Release Notes<no value>
{{< tagged "internal" >}}
SU6

CHANGE "params.toml"  "link2    = "flighting/msfs-2024-sdk-introduction/"" back to   link2    = "retail/msfs-2024-sdk-introduction/" for Final SU6 release.
UNCOMMENT the removed version changer from the "header.html" file.

<p class="fake-h3">Documentation</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "added" >}}

{{< release-notes-tag "fixed" >}}

{{< release-notes-tag "improved" >}}

<p class="fake-h4">Project Editor</p>

{{< release-notes-tag "added" >}}

{{< release-notes-tag "fixed" >}}

{{< release-notes-tag "improved" >}}

<p class="fake-h4">SDK Tools</p>

{{< release-notes-tag "added" >}}

{{< release-notes-tag "improved" >}}

<p class="fake-h4">Models And Textures</p>

{{< release-notes-tag "added" >}}

{{< release-notes-tag "fixed" >}}

{{< release-notes-tag "improved" >}}

<p class="fake-h4">Sound</p>

{{< release-notes-tag "added" >}}

<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "added" >}}

{{< release-notes-tag "fixed" >}}

{{< release-notes-tag "improved" >}}

<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "added" >}}

{{< release-notes-tag "fixed" >}}

{{< release-notes-tag "improved" >}}

<p class="fake-h4">Samples And Tutorials</p>

{{< release-notes-tag "added" >}}

{{< release-notes-tag "improved" >}}

--------------------template--------------------

<p class="fake-h3">DevMode</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "fixed" >}}

{{< release-notes-tag "added" >}}

{{< release-notes-tag "improved" >}}

<p class="fake-h4">Project Editor</p>

{{< release-notes-tag "fixed" >}}

{{< release-notes-tag "added" >}}

{{< release-notes-tag "improved" >}}

<p class="fake-h4">SimObject Editor</p>

{{< release-notes-tag "fixed" >}}

{{< release-notes-tag "added" >}}

{{< release-notes-tag "improved" >}}

<p class="fake-h4">Scenery Editor</p>

{{< release-notes-tag "fixed" >}}

{{< release-notes-tag "added" >}}

{{< release-notes-tag "improved" >}}

<p class="fake-h4">Material Editor</p>

{{< release-notes-tag "fixed" >}}

{{< release-notes-tag "added" >}}

{{< release-notes-tag "improved" >}}

<p class="fake-h4">VFX Editor</p>

{{< release-notes-tag "fixed" >}}

{{< release-notes-tag "added" >}}

{{< release-notes-tag "improved" >}}

<p class="fake-h4">Biome Editor</p>

{{< release-notes-tag "fixed" >}}

{{< release-notes-tag "added" >}}

{{< release-notes-tag "improved" >}}

<p class="fake-h3">SDK</p>

<p class="fake-h4">Models And Textures</p>

{{< release-notes-tag "added" >}}

{{< release-notes-tag "fixed" >}}

{{< release-notes-tag "improved" >}}

<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "added" >}}

{{< release-notes-tag "fixed" >}}

{{< release-notes-tag "improved" >}}

<p class="fake-h4">Tools</p>

{{< release-notes-tag "fixed" >}}

{{< release-notes-tag "added" >}}

{{< release-notes-tag "improved" >}}

<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "added" >}}

{{< release-notes-tag "fixed" >}}

{{< release-notes-tag "improved" >}}

<p class="fake-h4">Samples</p>

{{< release-notes-tag "added" >}}

{{< release-notes-tag "fixed" >}}

{{< release-notes-tag "improved" >}}

<p class="fake-h3">Documentation</p>

<p class="fake-h4">DevMode</p>

{{< release-notes-tag "added" >}}

{{< release-notes-tag "fixed" >}}

{{< release-notes-tag "improved" >}}

<p class="fake-h4">Models And Textures</p>

{{< release-notes-tag "added" >}}

{{< release-notes-tag "fixed" >}}

{{< release-notes-tag "improved" >}}

<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "added" >}}

{{< release-notes-tag "improved" >}}

{{< release-notes-tag "fixed" >}}

<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "added" >}}

{{< release-notes-tag "fixed" >}}

{{< release-notes-tag "improved" >}}

<p class="fake-h4">Samples And Tutorials</p>

{{< release-notes-tag "added" >}}

{{< release-notes-tag "fixed" >}}

{{< release-notes-tag "improved" >}}

{{< /tagged >}}


## SDK release 1.7.3

{{% tagged "internal" %}}

CL 2371569 - 2557648

{{% /tagged %}}

-----------------------------------------------------------------------------------------------

### DevMode

#### General

{{< release-notes-tag "Added">}}

-   Added new Travel book debug in the Flights section of the Debug menu
-   Added combobox to select a STR in replaced editor from Input tab inSimObject Editor.
-   Added combobox for tag field in Input Profiles Editor
-   Added indicator for input action type override in Input Profiles Editor
-   Added a new asset group Profiling to profile packages
-   Added allowed buttons in Input Device Editor
-   Added max value for WASM memory in SimObject Statistics profiler
-   Added mesh occluder debug in "Debug &gt;Aircraft&gt;Environement occluder".
-   Added a tool to profile airports
-   Added new stats to the SimObject Statistics profiler
-   Added better feedback when building a cfg with parsing errors
-   The SimObject Statistics profiler has a new helper dialog to display LoD curves.

{{< release-notes-tag "Fixed">}}

-   Fixed and improved governor controls in the "Aircraft Debug Engines" window:
    -   Fixed an issue that prevented governor target RPM values from being entered independently for each engine on multi-engine helicopters.
    -   Added validation for the entered target RPM value (limited to the 0-200% range).
    -   Replaced the non-interactive governor status checkbox with a color-coded bullet and a text label distinguishing the "OFF", "INACTIVE", and "ACTIVE" states.
    -   Improved the tooltip displayed when hovering over the "INACTIVE" status.
    -   Made minor layout and punctuation improvements.
-   Fixed "Open local documentation" button in DevMode menu which tried to open old documentation.
-   Fixed checks on VisualEffectLib and VisualEffectLegacyLib output path in PackageBuilder
-   Fixed SPB Compiler not flagging some XML errors in source files (could lead to corrupted SPBs and crashes in the sim)
-   Fixed blurry textures in some cases, like rolling numbers on aircraft instruments.
-   Fixed sounds played at the incorrect position when using different simvar sounds using the same wwise event and different attach nodes.
-   Potential fix for mouse disappearing unexpectedly while using the devmode.
-   Fixed Reset Settings not correctly resetting all devmode settings
-   Fixed empty Device Keys dialog with the Input Profiles Editor
-   Fixed possible crash while building a package with the Aircraft Capture Tool open
-   Developer camera: Fixed an issue with zooming and mouse input.
-   Fixed user light tool no more usable with encrypted packages
-   Fixed leaving Aircraft Capture Tool only restoring maximized window state and not fullscreen state
-   Fixed rendering of "Channel display &gt; clearcoat roughness"

{{< release-notes-tag "Improved">}}

-   Renamed "Flight Plan" section of Debug menu to "Flights"
-   Improved debug vector shape
-   The debug airport is now showing the airport archetype
-   Prevent switching to dev cam while game is in teleport state to avoid camera issues
-   Removed debug option Debug Procedural Generation that does nothing
-   Changed the way we compute mouse coordinate while using Smart Docking System option. This fixes some mouse interaction issues with mouse related debugs.
-   Properly disable developer camera when enabling slew mode to avoid confusion in the menu.
-   Reworked package reorder tool to be more intuitive and easier to use (rework visual feedbacks, allow reorder inside sections, drag and drop).


#### Project Editor

{{< release-notes-tag "Added">}}

-   Added package order hint for Custom travel books

{{< release-notes-tag "Fixed">}}

-   Fixed "Save as" option not working properly.
-   Fixed crash when capturing aircraft thumbnail for aircraft with incorrect static height
-   Fixed incorrect size of thumbnail capture, now capture in 360x240 as required
-   Fixed suggested dimensions for Content Manager Thumbnail (was 412x170, now 360x240)
-   Fixed crash when building outdated packages while closing project


#### Input Profile Editor

{{< release-notes-tag "Added">}}

-   Added Thrustmaster PS5 device type to the Input Profile Editor


#### Scenery Editor

{{< release-notes-tag "Added">}}

-   Added parking light object.
-   Added new option "MastLight" in airport archetype.
-   Added a movement sensitivity threshold to gizmo movements to avoid unwanted microscopic movements while clicking on the gizmo

{{< release-notes-tag "Fixed">}}

-   Fixed "NoDecal" not working for some models.
-   Fixed gizmo not able to change altitude (regression SU6)
-   Fixed projected meshes not rendered sometimes
-   Fixed invalid object type kept selected when switching to regular scenery edition.
-   Fixed drag and drop issues, especially when trying to drag an unselected object
-   Fixed scenery object shift between the editor and the built package
-   Fixed framerate issues in the editor with projected meshes (regression SU6).
-   Fixed collisions not working for instanced objects
-   Fixed "View only current package" not reloading the ground textures.
-   Fixed hiding objects with the eye icon not working for a lot of types
-   Rectangle object: prevent profile creation when heightmap is used and vice-versa.
-   Fixed unwanted rotation upon adding object in SPC edition.
-   Fixed some characters in object names causing BGL compilation errors
-   Fixed light presets rendering issue when the light is rotated.
-   Fixed textured polygons affecting ground materials on a large aera(incorrect mip generation for SurfaceTypeMerge).
-   Fixed not being able to pan on Worldmap while Scenery Editor is opened
-   Fixed single trees using wrong species in some case
-   Fixed viewport rectangle selection taking groups into account

{{< release-notes-tag "Improved">}}

-   Support "no snow " and "no decal" with instancing.
-   Better draw distance for lights attached to instanced meshes, now based on the light intensity.
-   Renamed "Importer from APX" to "Airport XML importer" and fixed crashes.
-   More precise position for rectangles and polygons in BGL (30cm to 1mm).
-   Removed broken option to move selection up/down in the scene tree. Use drag and drop instead
-   Improved vegetation polygon precision.
-   Reduced falloff distance when adding an heightmap to a rectangle.
-   Improved heightmap gird rendering.
-   Better TIN terraforming (tesselate the TIN geometry when needed)
-   Auto expand relevant header in polygon properties
-   The "Show light direction" debug is now shown for each light row vertices.
-   Polygons can now be used for material and vegetation override at the same time
-   It's now possible for a polygon to render material and override vegetation/secondary heightmap at the same time.


#### SimObject Editor

{{< release-notes-tag "Added">}}

-   Added "on and powered" state for circuits, indicating that the circuit has been switched on and receives enough tension to function
-   Added runtime information on lines in the electrical system graph relative to their connection state (switch, breaker, relay).
-   Added condition for hydraulics lines so Valves and Accumulator can't be empty.
-   Added an option in the context menu to give custom name to section or param for a specific package
-   Added hydraulics system minor version 2
-   Added versions and modifier_local_angle_scalar fields in flight_model
-   Added missing InertialSeparatorOnTorque parameter
-   Added new properties to show in runtime graph: average tension, average load, the battery is powering consumers or not, number of powered consumers

{{< release-notes-tag "Fixed">}}

-   Fixed hydraulics valve of type ShutOff required to have a Circuit
-   Fixed possible infinite freeze when opening the navgraph editor
-   Fixed CFG Validation Window allowing modification of parm from VFS files
-   Fixed edit in place incorrectly saving file with empty sections
-   Fixed issue with reload option with a modified asset
-   Fixed impossible to click delete or rename menu option when a livery node is selected
-   Fixed missing prop_betathreshold params
-   Fixed high_n1/n2 maximum set to 100%
-   Fixed possible issue with file not correctly saved for merged sections
-   Fixed impossible to select any container in the User Light Tool
-   Fixed hydraulics line valves and accumulator value making errors in the systems when empty
-   Fixed duplicate property remaining capacity

{{< release-notes-tag "Improved">}}

-   Removed some unused WearAndTearCollission parameters in systems.cfg tab
-   Made some batteries properties read only as they were always overwritten by the system.


#### Visual Effects Editor

{{< release-notes-tag "Fixed">}}

-   Fixed random crash in VFX Templates/Instances debugger
-   Fixed crash upon erasing comment block
-   Fixed double clicking on a node without world position moving developer camera.


#### Biome Editor

{{< release-notes-tag "Fixed">}}

-   Fixed build error when using Rebuild option from File menu.


### SDK

#### Content Configuration

{{< release-notes-tag "Added">}}

-   Electrical System:
    -   Added the possibility to specify a minimum voltage required for a circuit to function called 'MinVoltage'. Default MinVoltage is half of the nominal tension of the circuit.
    -   Added localvars to numerical values possibilities in electrical system cfg parameter definitions (in addition to simvars and globalvars).

-   Engines CFG:
    -   Added an optional `delta_commanded_Ne_control` parameter to the [TURBINEENGINEDATA] section of engines.cfg. It specifies whether the first argument of `delta_commanded_Ne_table` represents the condition lever position or the mixture lever position.
    -   Added an optional `delta_commanded_Ne_table` parameter to the [TURBINEENGINEDATA] section of engines.cfg. This multi-dimensional table is applicable only when either the `corrected_commanded_Ne_table` or `uncorrected_commanded_Ne_table` parameter is used, and defines a correction to corrected or uncorrected commanded Ne, respectively, as a function of condition or mixture lever position and throttle lever position.
    -   Added an optional `uncorrected_commanded_Ne_table` parameter to the [TURBINEENGINEDATA] section of engines.cfg. When specified, this table defines the target uncorrected N1 for turbojet, turbofan, turboprop, and turboshaft engines as a function of throttle lever position. When omitted, the existing behavior remains unchanged to preserve backward compatibility.
    -   Added an optional `corrected_commanded_Ne_table` parameter to the [TURBINEENGINEDATA] section of engines.cfg. This multi-dimensional table allows corrected commanded Ne to be explicitly tuned as a function of pressure altitude, inlet Mach number, and throttle lever position. For turbojet and turbofan engines, it represents the target corrected N2. For turboprop and turboshaft engines, it represents the target corrected N1. When omitted, the existing behavior remains unchanged to preserve backward compatibility.
    -   Added an optional `full_throttle_commanded_n1` parameter to the [TURBINEENGINEDATA] section of engines.cfg (for turbojet, turbofan, turboprop, and turboshaft engines). When specified, this parameter also overrides "high_n1" for consistency and modifies the engine-speed control law for turboprop and turboshaft engines so that it controls actual N1 instead of corrected N1. When omitted, the existing behavior remains unchanged to preserve backward compatibility.

-   Model Behaviours:
    -   Added a way to specify that the `ASOBO_Elevator_Trim_Settings_Config` should use that simvar (opt-in) by setting the parameter `USE_EX2_SIMVAR` to true.

{{< release-notes-tag "Fixed">}}

-   Electrical System:
    -   Fixed batteries charge C rate not being initialized with a default value when parsing a cfg and none is given. Version 2.3 and up required.

-   Engines CFG:
    -   Added validation of the number of arguments, argument ranges, and data values for the following commanded Ne multi-dimensional tables: `corrected_commanded_Ne_table`, `uncorrected_commanded_Ne_table`, `delta_commanded_Ne_table`

-   Cameras CFG:
    -   Fixed an issue causing the [CAMERA_RAY_NODE_COLLISION] nodes to not work properly.

-   Panel CFG:
    -   Fixed default dynamic parameter value for preset and common panel.cfg VPainting definition. When no override has been provided in the livery.cfg the dynamic parameter value will now use the locally defined value in the DynamicParameters section instead of not replacing the parameter. While this lets the developer correctly provide a default VPainting color and allow override in the livery this could change the default color from white to the specified one if a configuration was specified (as it was previously unused)

-   Model Behaviours:
    -   Fixed typos in `ASOBO_Bool_Visibility_Parameters`. When rebuilding a package with the lastest version of the behaviors, update usage as follows:
        -   replace `VISIBILTY_IE_VARIABLE` with `VISIBILITY_IE_VARIABLE`
        -   replace `VISILITY_L_VARIABLE` with `VISIBILITY_L_VARIABLE`

{{< release-notes-tag "Improved">}}

-   Electrical System:
    -   Improved system update for better loops and alternative paths detection, allowing a better load sharing among buses. Version 2.3 and up required.

-   Pneumatic & ECS System:
    -   Changed Outflow Valve behavior from managing cabin pressures to managing cabin alt rates, allowing it to restrict (as much as it can) the cabin climb. Change is Opt-In behind Pneumatics Version 2.

-   Engines CFG:
    -   Refined the descriptions of the recently added `full_throttle_commanded_n1`, `corrected_commanded_ne_table`, and `delta_commanded_ne_table` parameters in the Package Analyzer checks.


#### Programming API's

{{< release-notes-tag "Added">}}

-   SimVars:
    -   Added the `TURB ENG COMMANDED N1 OVERRIDE ACTIVE` SimVar to enable external control of commanded Ne via `TURB ENG COMMANDED N1`.

-   Key Events:
    -   Added key event `GROUND_SERVICE_ATC_MESSAGE` to trigger new Ground Services for user aircraft.

-   SimConnect:
    -   Added frequency name and type to the facility explorer.

-   RPN:
    -   Added ability to scope `O:PATH:TO::VARIABLE` to a specific attachment by alias using the syntax `O:PATH:TO:COMPONENT@alias:VARIABLE`, interior can be targeted using the alias `interior`, exterior using the alias `exterior`

-   JavaScript
    -   Added runway lighting information.
    -   Added a route index that allows looking up routes by name.

{{< release-notes-tag "Fixed">}}

-   Key Events:
    -   Fixed `TOW_PLANE_REQUEST` key event always spawning a winch regardless of the selected launch method
    -   Restored `KEY_PAUSE_TOGGLE` key event
    -   Enabled `KEY_PAUSE_ON` and `KEY_PAUSE_OFF` is every context

-   SimVars:
    -   Fixed uncontrolled RPM acceleration of disengaged turbine engines via `FREEWHEELING UNIT ENGAGED` SimVar. Engine RPM is no longer updated internally while disengaged and can be controlled externally via `GENERAL ENG RPM` SimVar.
    -   Added Simvars `WEAR AND TEAR OVERSTRESS FACTOR` and `WEAR AND TEAR OVERSTRESS DAMAGE RATE`.
    -   Added SimVar `IS ANY OPEN INTERACTIVE POINTS RISKING TO CAUSE CRASH` which returns true if any interactive point would make the aircraft crash if it reached their failure speed (i.e. they have the type Main, Cargo or Emergency, they have a positive failure speed and they are opened at more than 95%)
    -   Added the `FREEWHEELING UNIT ENGAGED` SimVar, which controls whether each helicopter engine transmits torque to the rotor system. See SDK for details.
    -   Added simvar `ELEVATOR TRIM PCT EX2`. This simvar returns the trim level in percent, with the -100%-&gt;0% range being used for the down trim while 0%-&gt;100% is used only for the up trim. This means that setting 0 trim will also result in that simvar returning 0%.

-   WebAssembly:
    -   Fixed loading of WASM systems and improved error handling (was causing infinite loading for some aircraft)
    -   Fixed rare random deadlocks when using the CommBus API or other APIs that depend on it

-   SimConnect:
    -   Fixed `SimConnect_MapInputEventToClientEvent` not working on consoles
    -   Fixed `MapInputEventToClientEvent` registering actions on joystick button 0 when an invalid button index was provided - now triggers a SimConnect exception

-   RPN:
    -   Fixed O/I variables not able to be queried if the target component is defined in the interior or exterior model instance instead of an attachment.

{{< release-notes-tag "Improved">}}

-   SimVars:
    -   Updated `FREEWHEELING UNIT ENGAGED` to use one-based engine indexing and index 0 for all engines.


#### Tools

{{< release-notes-tag "Fixed">}}

-   Fixed a bug that caused the BGL compiler to add holding patterns to multiple airports rather than just the intended one.

-   3DS Max:
    -   Fixed initialize Wiper Mask Tool

-   Blender:
    -   Fixed an issue that caused incorrect vertex colors in an edge case when "Merge Nodes" is enabled.
    -   Fixed apply rotation and scale before merging objects during export (when "Merge Nodes" is enabled). Prevents issues with shaders that use node scale.
    -   Fixed incorrect texture color space when the same texture is used in both sRGB and Non-Color inputs.

{{< release-notes-tag "Added" >}}

-   Added new 3dsMax - Blender Bridge: A tool for transferring data between 3ds Max and Blender.

-   Blender:
    -   Added support export of linked materials.
    -   Added blender scenes in Shared Assets.
    -   Added missing alpha mode to vegetation material.

{{< release-notes-tag "Improved">}}

-   Blender:
    -   GLTF import: Support deprecated 'AsoboMacroLight'. They are automatically imported as Street Lights.
    -   GLTF import: Support deprecated decal materials defined in the 'ASOBO_material_blend_gbuffer' extension.
    -   GLTF import: Disable assignment of a unique ID to each imported object. This prevents issues when importing GLTF files containing nodes with the same ID.
    -   Material input descriptions for the Details Map have been clarified.
    -   Improved scene opening performance for scenes containing old collision primitives.
    -   The "Objects" and "Presets" tabs have been merged into a single "Hierarchy" tab.


### Documentation

{{< release-notes-tag "Added" >}}

- SimVars:
    -   New hydraulic system SimVars have been added to the documentation - `HYDRAULIC COMPONENT LIQUID QUANTITY`, `HYDRAULIC COMPONENT LIQUID QUANTITY RATIO`, `HYDRAULIC COMPONENT PRESSURE`, `HYDRAULIC LINE INPUT SIDE LIQUID QUANTITY`, `HYDRAULIC LINE INPUT SIDE LIQUID QUANTITY RATIO`, `HYDRAULIC LINE INPUT SIDE PRESSURE`, `HYDRAULIC LINE OUTPUT SIDE LIQUID QUANTITY`, `HYDRAULIC LINE OUTPUT SIDE LIQUID QUANTITY RATIO`, `HYDRAULIC LINE OUTPUT SIDE PRESSURE`.
    -   A new helicopter SimVar - `FREEWHEELING UNIT ENGAGED` has been added to the documentation.
    -   New liquid dropping SimVars added to the documentation - `LIQUID DROPPING TANK PCT FULL VOLUME`, `LIQUID DROPPING TANK PCT FULL WEIGHT`.
    -   New SimVar `TURB ENG COMMANDED N1 OVERRIDE ACTIVE` has been added.
    -   New SimVar added: `PNEUMATICS ENGINE BLEED FLOW RATIO`.
    -   Added `ELEVATOR TRIM PCT EX2`.
    -   New SimVar `CONTACT POINT SURFACE TYPE`  added.
    -   Added `IS ANY OPEN INTERACTIVE POINTS RISKING TO CAUSE CRASH`

-   SimConnect:
    -    Add new member - `HAS_LPV200` - to the NavData `API APPROACH` structure.

-   Key Events:
    -   New key events added: `GROUND_SERVICE_ATC_MESSAGE`

-   Blender + 3DS Max:
    -   New cubemap and reflection options for Windshield material added to modelling documentation (Cubemap Reflection Masking and Screen Space Reflection Intensity).
    -   New Bridge tool has been added to the documentation.

-   Blender:
    -   New LOD Viewer tool has been added to the documentation.

-   Note added to animation pages to explain that no constraints are supported natively in the engine.

-   Content Configuration:
    -   Multiple new parameters to provide additional flexibility in defining commanded N - making turbine engine speed laws easier and more precise to tune - have been added to the engines.cfg documentation: `full_throttle_commanded_n1`, `corrected_commanded_Ne_table`, `uncorrected_commanded_Ne_table`, `delta_commanded_Ne_table`, `delta_commanded_Ne_control`
    -   The section on modular simobject merging has beeen updated with missing XML &lt;DynamicParameters&gt; information.
    -   New "Version" number added to Hydraulic system documentation, and hydraulic actuator documentation updated with new "DropPressure" map entry (and improved key descriptions).
    -   Added additional details to the `froude_krylov_scalar` in the flight_model.cfg documentation.
    -   New section added to FLT Information page to explain custom content mission ordering in the UI. Accompanying "difficulty" parameter has been added to the FLT File Properties page as well.
    -   Added three new parameters to the [AUTOPILOT] section of the systems.cfg: `autopilot_version`, `yaw_damper_rudder_gain`, `yaw_damper_max_speed`.
    -   New parameter `APU Pct RPM` added to the FLT files [Systems.N] section.
    -   Added new parameter to the hydraulics system `Actuator.N` hashmap description: `Drop Pressure`.
    -   Hydraulic system version information updated to describe version 2.
    -   New flight_model.cfg parameter added to the `[FLAPS.N]` section: `power_source_name`
    -   New aircraft.cfg parameters added: `yoke_2_anim_x`, `yoke_2_anim_y`, `yoke_2_collision_mesh`, `yoke_2_control_lr`, `yoke_2_control_ud`, `yoke_2_node`, `yoke_release_point_lr`, `yoke_2_release_point_lr`, `yoke_release_point_ud`, `yoke_2_release_point_ud`, `yoke_release_time_lr`, `yoke_2_release_time_lr`, `yoke_release_time_ud`, `yoke_2_release_time_ud`.
`   -   `MinVoltage` added to the electrical system `[Consumer.N]` section.
    -   Information on how "free castering" works and is setup added (see Contact Points: param 7).
    -   Note added to the contact point information to explain how water rudders work with the `point.n` `damping ratio` parameter.
    -   Missing Flight Performance Setup page re-added to the documentation.

-   New Legacy vs Modern Aerodynamics Analysis window added.

-   Mast Light has been added into the scenery editor documentation.

-   ATIS frequency name requirements added to airport object documentation.

-   QuadBillboardType added to FX Output block description, and mentions of QuadOrientationType have been changed to QuadBillboardType throught the FX pages.

-   Additional information added to the "View Only Current Package" option in the edit menu of the Scenery Editor.

-   WASM documentation has been updated to include new "update" callbacks (see the "Update Callbacks" page in the Programming APIs &gt; WebAssembly section).

- The SimObject Statistics profiler:
    -   Added information about the new helper dialog to display LoD curves.
    -   Added new information related to new statistics involved in the profiles.
    -   Added information about package setup for profile assets required for console ingestion on the marketplace.

-   New Project Editor option for asynchronous building has been added to the documentation.

-   New DevMode options added to the documentation: "UTC/Local", "Current SimRate", "Capture devmode inputs", and "Ignore validator errors"

-   Input Profile sample project added to the documentation.


{{< release-notes-tag "Fixed">}}

-   Multiple fixes to formating, broken links, broken images, etc... throughout the documentation (note that the current version of the documentation is still in transition to the new framework and as such is still WIP)

-   Fix for malformed debug info links.

-   Fixed broken link to Ground Vehicle page.

-   Fixes for offline docs:
    -   404 page formatting fixed.
    -   external links fixed so they now correctly open in a browser.

-   Fixed mathjax errors on flight tuning pages.

-   Links have been fixed to split correctly and not overrun pages on lower resolution.

-   Removed all mention of the obsolete "SimAttachment Editor".

-   Fixed erroneous description related to how the SimObject Editor Graph View is opened.

-   Fixed content thumbnail dimensions listed for Project Editor.

-   SimVars:
    -   Fixed error where `LIQUID DROPPING TANK PCT FULL VOLUME` and `LIQUID DROPPING TANK PCT FULL WEIGHT` were flagged as read-only when they are actually settable.
    -   Fixed the units given for `CAMERA GAMEPLAY PITCH YAW`
    -   Fixed incorrect description for "radians" and "grads" in the SimVar Units page.
    -   Minor typo in `HYDRAULIC ACTUATOR ACTIVE` description has been fixed.

-   Content Configuration:
    -   The page on creating aircraft thumbnails has been fixed to reflect the correct setup for non-modular aircraft.
    -   Fixed an error where the parameter `always_execute_model_behavior ` was flagged as obsolete.
    -   Fix for missing additions to APU (Fuel + Pneumatics) docs that were not included in a previous documentation update.
    -   Hydraulics documentation (Pumps and Actuators) has had a few parameter description updates to fix wrong information.
    -   Fixed erroneous information on the model behaviours page related to deprecated XML elements.
    -   Fixed erroneous information for: `min_castering_angle`, `max_castering_angle`.
    -   Fixed Burner System examples.
    -   `ui_fuel_burn_rate` incorrect unit value fixed on the Additional Aircraft Information page.

-   SimConnect:
    -   Fixed `SIMCONNECT_RECV_EXCEPTION` documentation to show correct "unknown" examples.


{{< release-notes-tag "Improved">}}

-   Blender:
    -   Removed obsolete "Textures" option from blender plugin docs.

-   Blender + 3DS Max:
    -   Updated the light documentation to explain the interior/exteriror channel setting.

-   Audio:
    -   Updated some information on the FadeOutTypeand FadeOutTime attributes.
    -   Updates to the sound section, adding in new master mixer AUX buses and fixing outdated install information.

-   Update to the channel display debug page to better explain how it works.

-   Content Configuration:
    -   Update to outflow valve behaviour in the modular pnuematics system (V2+).
    -   Liquid dropping system page information has been updated to flag the "Name" hashmap keys as *not* required.
    -   Removed default value from `LiquidConsumption` description in the hydraulics system `[Actuator.N]` hashmap.
    -   Update to the `ui_fuel_burn_rate` parameter description as well as associated career documentation to better explain how it affects the effective range of the aircraft.
    -   Updated to multiple pages related to APUs (Fuel system, Pneumatics system, etc...) to clarify that the simulation can currently only parse one APU definition.
    -   Updated description for the `OBJ_AIRGEO_WING.N` section of the flight_model.cfg.
    -   Update to the passenger setup documentation to include other flight modes, not just careers.
    -   Updated description for `rotor_friction_torque` in the flight_model.cfg.
    -   Apron element in the scenery XML description updated.

-   All mention of "UVmap2" for Blender has been removed from the documentation, as both Blender and 3DS Max now both use "UV2"references.

-   Minor update to model behaviour debug tool to add in "decimals" option.

-   Biome edtor docs have been reviewed to remove options that no longer exist.

-   SimDimensions debug window removed from the docs.

-   Clarifications added to the page related to the Content Creator Testing Tool.

-   Obsolete "Lod Curve" option removed from the Options menu documentation.

-   The "Creating Or Replacing An Airport" tutorial page has been refreshed to bring it in line with the current DevMode status.

-   Helipad scenery object page updated with additional information on taxiway point setup.

-   Updated the scenery editor "rename" shortcut keys.

-   Updated Material Editor Inspector page to cross-reference texture format information.

-   Update to the Tools > Package Reorder Tool section to reflect minor changes to the tool window and workflow.

-   Rework of the The SimObject Statistics profiler to better clarify the different statistics.

-   The documentation search modal has been cleaned and updated so it looks better, and should take you to the phrase searched on the page, rather than just the nearest header.

-   SimVars:
    -   All electrical system SimVars have been given a complete rewrite to include missing information and fix multiple issues.
    -   `PNEUMATICS PACK FLOW` flagged as writable.
    -   The following SimVars have had their descriptions updated - `TURB ENG CONDITION LEVER POSITION`, `TURB ENG COMMANDED N1`, `TURB ENG THROTTLE COMMANDED N1`.
    -   Removed the "index" from the `TACAN DRIVES NAV1` SimVar description.

-   SimConnect:
    -   Update to the `SimConnect_CameraAcquire` page to mention the `ClientId` parameter.
    -   Update to SimConnect `MapInputEventToClientEvent`/`_EX1` documentation to include additional information for mouse input.


------------------------------------------------------------------------------------------------


### Previous SDK Release Notes

Below you can find a list of the list notes for previous releases of this SDK. Simply click the release notes you want to explore to open them:

{{< expand title="SDK Release 1.7.2" >}}

{{% tagged "internal" %}}

CL 2340019 - 2520915

{{% /tagged %}}

<p class="fake-h3">DevMode</p>

{{< release-notes-tag "Fixed">}}

- Fixed SPB Compiler not flagging some XML errors in source files (could lead to corrupted SPBs and crashes in the sim)


<p class="fake-h4">General</p>

{{< release-notes-tag "Added">}}

- Renamed "Flight Plan" section of Debug menu to "Flights"
- Added new Travel book debug in the Flights section of the Debug menu
- Added a tool to profile airports
- Added new stats to the SimObject Profiler
- Added better feedback when building a cfg with parsing errors


{{< release-notes-tag "Fixed">}}

- Prevent switching to dev cam while game is in teleport state to avoid camera issues
- Changed the way we compute mouse coordinate while using Smart Docking System option. This fixes some mouse interaction issues with mouse related debugs.
- Fixed blurry textures in some cases, like rolling numbers on aircraft instruments.
- Removed debug option Debug Procedural Generation that does nothing
- Fixed Reset Settings not correctly resetting all devmode settings
- Fixed empty Device Keys dialog with the Input Profiles Editor
- Fixed possible crash while building a package with the Aircraft Capture Tool open
- Developer camera: Fixed an issue with zooming and mouse input.
- Fixed user light tool no more usable with encrypted packages
- Fixed leaving Aircraft Capture Tool only restoring maximized window state and not fullscreen state


{{< release-notes-tag "Improved">}}

- Properly disable developer camera when enabling slew mode to avoidconfusion in the menu.
- Reworked package reorder tool to be more intuitive and easier to use (rework visual feedbacks, allow reorder inside sections, drag and drop).


<p class="fake-h4">Project Editor</p>
{{< release-notes-tag "Added">}}

- Added package order hint for Custom travel books


{{< release-notes-tag "Fixed">}}

- Fixed suggested dimensions for Content Manager Thumbnail (was 412x170, now 360x240)
- Fixed crash when building outdated packages while closing project


<p class="fake-h4">Scenery Editor</p>

{{< release-notes-tag "Added">}}

- It's now possible for a polygon to render material and override vegetation/secondary heightmap at the same time.
- Support "no snow " and "no decal" with instancing.
- Fixed "NoDecal" not working for some models.


{{< release-notes-tag "Fixed">}}

- Rectangle object: prevent profile creation when heightmap is used and vice-versa.
- Fixed unwanted rotation upon adding object in SPC edition.
- Fixed some characters in object names causing BGL compilation errors
- Fixed light presets rendering issue when the light is rotated.
- The "Show light direction" debug is now shown for each light row vertices.
- Fixed textured polygons affecting ground materials on a large aera(incorrect mip generation for SurfaceTypeMerge).
- Fixed not being able to pan on Worldmap while Scenery Editor is opened
- Fixed single trees using wrong species in some case
- Fixed viewport rectangle selection taking groups into account


{{< release-notes-tag "Improved">}}

- Renamed "Importer from APX" to "Airport XML importer" and fixed crashes.
- More precise position for rectanglesand polygons in BGL (30cm to 1mm).
- Removed broken option to move selection up/down in the scene tree. Use drag and drop instead
- Improved vegetation polygon precision.
- Reduced falloff distance when adding an heightmap to a rectangle.
- Improved heightmap gird rendering.
- Better TIN terraforming (tesselate the TIN geometry when needed)
- Auto expand relevant header in polygon properties


<p class="fake-h4">SimObject Editor</p>

{{< release-notes-tag "Added">}}

- Added runtime information on lines in the electrical system graph relative to their connection state (switch, breaker, relay).
- Added condition for hydraulics lines so Valves and Accumulator can't beempty.
- Added an option in the context menu to give custom name to section or param for a specific package
- Added hydraulics system minor version 2
- Added versions and modifier_local_angle_scalar fields in flight_model


{{< release-notes-tag "Fixed">}}

- Fixed edit in place incorrectly saving file with empty sections
- Removed some unused WearAndTearCollission parameters in systems.cfg tab
- Added missing InertialSeparatorOnTorque parameter
- Fixed issue with reload option with a modified asset
- Fixed impossible to click delete or rename menu option when a livery node is selected
- Fixed missing prop_betathreshold params
- Fixed high_n1/n2 maximum set to 100%
- Fixed possible issue with file not correctly saved for merged sections
- Fixed impossible to select any container in the User Light Tool
- Fixed hydraulics line valves and accumulator value making errors in the systems when empty


{{< release-notes-tag "Improved">}}

- Made some batteries properties read only as they were alwaysoverwritten by the system.
- Fixed duplicate property remaining capacity
- Added new properties to show in runtime graph: average tension, average load, the battery is powering consumers or not, number of powered consumers

<p class="fake-h4">Visual Effects Editor</p>

{{< release-notes-tag "Fixed">}}

-  Fixed crash upon erasing comment block
- Fixed double clicking on a node without world position moving developer camera.


<p class="fake-h3">SDK</p>

{{< release-notes-tag "Added">}}

- Added frequency name and type to the facility explorer.

{{< release-notes-tag "Fixed">}}

- Fixed a bug that caused the BGL compiler to add holding patterns to multiple airports rather than just the intended one.
- Restored KEY_PAUSE_TOGGLE key event
- Enabled KEY_PAUSE_ON and KEY_PAUSE_OFF is every context


<p class="fake-h4">SimVars</p>

{{< release-notes-tag "Added">}}

- Added simvar ELEVATOR TRIM PCT EX2. This simvar returns the trim level in percent, with the -100%->0% range being used for the down trim while 0%->100% is used only for the up trim. This means that setting 0 trim will also result in that simvar returning 0%.
- Used that simvar in the HUD display for trim, which fixes some errors which caused the HUD to display the trim at 100% before it actually reached the max in cases of asymmetric trim.
- Also added a way to specify that the ASOBO_Elevator_Trim_Settings_Config should use that simvar (opt-in) by setting the parameter USE_EX2_SIMVAR to true.
- Added the FREEWHEELING_UNIT_ENGAGED SimVar, which controls whether each helicopter engine transmits torque to the rotor system. See SDK for details.


<p class="fake-h4">JS API</p>

{{< release-notes-tag "Added">}}

- Added runway lighting information.
- Added a route index that allows looking up routes by name.


<p class="fake-h4">Exporter 3DSMAX</p>

{{< release-notes-tag "Fixed">}}

- Fixed initialize Wiper Mask Tool


<p class="fake-h4">Electricity system</p>

{{< release-notes-tag "Added">}}

- Added localvars to numerical values possibilities in cfg param definitions (in addition to simvars and globalvars).


{{< release-notes-tag "Fixed">}}

- Fixed batteries charge c rate not being initialized with a default value when parsing a cfg and none is given. Version 2.3 and up required.


{{< release-notes-tag "Improved">}}

- Improved system update for better loops and alternative paths detection, allowing a better load sharing among buses. Version 2.3 and up required.


<p class="fake-h4">Pneumatic & ECS System</p>

{{< release-notes-tag "Improved">}}

- Changed Outflow Valve behavior from managing cabin pressures to managing cabin alt rates, allowing it to restrict (as much as it can) the cabin climb. Change is Opt-In behind Pneumatics Version 2.


<p class="fake-h4">NavData</p>

{{< release-notes-tag "Added">}}

- Added threshold crossing height for glideslope.
- Added GBAS/SBAS path points and GLS facilities (but not yet GLS approaches).
- Added boundary designation (short name), and comm frequency sector bearing/altitude/distance restrictions.
- Added LPV200 flag for approach navigation data.


<p class="fake-h3">Documentation</p>

{{< callout context="caution" title="IMPORTANT!" icon="outline/alert-triangle" >}}

This document is still a work-in-progress as we transition to a new publishing framework, and as such may contain broken links, poor formating, and other minor issues. We anticipate having this all resolved for the final Sim Update 6 release. If you require the old documentation it can still be found at the previous URL: [https://docs.flightsimulator.com/msfs2024/html/1_Introduction/Introduction.htm](https://docs.flightsimulator.com/msfs2024/html/1_Introduction/Introduction.htm). We apologise for any inconvenince this may cause.

{{< /callout >}}

{{< release-notes-tag "Added">}}

- New "Version" number added to Hydraulic system documentation, and hydraulic actuator documentation updated with new "DropPressure" map entry (and improved key descriptions).
- Helipad scenery object page updated with additional information on taxiway point setup.
- WASM documentation has been updated to include new "update" callbacks (see the "Update Callbacks" page in the Programming APIs > WebAssembly section).
- Added additional details to the froude_krylov_scalar in the flight_model.cfg documentation.
- New section added to FLT Information page to explain custom content mission ordering in the UI. Accompanying "difficulty" parameter has been added to the FLT File Properties page as well.


{{< release-notes-tag "Fixed">}}

- Hydraulics documentation (Pumps and Actuators) has had a few parameter description updates to fix wrong information.
- SimConnect: Fixed SIMCONNECT_RECV_EXCEPTION documentation to show correct "unknown" examples.


{{< release-notes-tag "Improved">}}

- All mention of "UVmap2" for Blender has been removed from the documentation, as both Blender and 3DS Max now both use "UV2"references.
- Obsolete "Lod Curve" option removed from the Options menudocumentation.
- The "Creating Or Replacing An Airport" tutorial page has been refreshed to bring it in line with the current DevMode status.
- Update to the Tools > Package Reorder Tool section to reflect minor changes to the tool window and workflow.


{{< /expand >}}


{{< expand title="SDK Release 1.6.9" >}}

<p class="fake-h3">Developer mode changes</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "Improved">}}

-   Removed option to select old LOD curve.


<p class="fake-h4">Project Editor</p>

{{< release-notes-tag "Fixed">}}

-   Fixed Model Instance being potentially destroyed upon a Build Package while reloading active LODs (could crash the simulator when building the package of the currently used aircraft).
-   Fixed package validation errors on PERFORMANCE_DATA in flight_performance.cfg.
-   Fixed freeze when building packages. Added a progress bar.
-   Fixed node lookup when merging XML files during Modular SimObject builds (could cause crashes when a RemoveEntry tag targeted an unknown node).

{{< release-notes-tag "Added">}}

-   Added edition of dependencies in the Package Inspector.


<p class="fake-h4">Input Editor</p>

{{< release-notes-tag "Fixed">}}

-   Fixed crash in context filter.


<p class="fake-h4">SimObject Editor</p>

{{< release-notes-tag "Added">}}

-   Added missing hydraulic actuator types: LiquidDroppingDoor and ThrustReverser.


<p class="fake-h4">Scenery Editor</p>

{{< release-notes-tag "Fixed">}}

-   Fixed VectorPlacement no longer being in groups when loaded by the editor.
-   Fixed missing terminal buildings on TIN.
-   Fixed plant mesh not rendered.


<p class="fake-h3">SDK changes</p>

<p class="fake-h4">Models And Textures</p>

{{< release-notes-tag "Fixed">}}

-   Fixed an issue causing Yoke/Collective highlight to not be active in VR on some aircraft.


<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "Added">}}

-   Offline AI Traffic is now enabled as an experimental feature. Traffic add-ons can now be used, though issues may still occur.


<p class="fake-h4">Tools</p>

{{< release-notes-tag "Fixed">}}

-   3DS Max:
    -   Fixed “Animation Groups” menu not appearing when SDK 2020 was installed.
    -   Fixed Multi-Exporter stopping export with an exception if LOD minimum size value had been deleted.
-   Blender:
    -   Fixed glTF light import.
    -   Fixed export of constant animation in NLA Track mode.
-   SimVar Watcher:
    -   Fixed SimVarWatcher debug build in SDK samples.
    -   Fixed SimVarWatcher documentation links.


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "Fixed">}}

-   Fixed race condition when loading key events associated with XML Gauges (could lead to random crashes).
-   Camera API:
    -   Fixed altitude referential using Ellipsoid instead of Geoid and ensured correct SimObject is used for camera rotation.
    -   Fixed SimConnect_SetRelativeCamera6DOF ignoring position parameters and placing the camera at the SimObject origin instead of the eyepoint.
-   WASM:
    -   Fixed fstat for stdin, stdout, and stderr.
    -   Fixed SimConnect_CameraAcquire function through WASM.
    -   Fixed crash on WASM reload caused by incorrect nullptr check.
-   SimVars:
    -   Fixed AIRSPEED INDICATED and AIRSPEED TRUE not being set immediately when loading an FLT using ZVelBodyAxis_IAS.

{{< release-notes-tag "Improved">}}

-   Camera API:
    -   Changed how status update messages are sent when enabling or disabling the add-on camera via the in-game panel.


<p class="fake-h4">Samples</p>

{{< release-notes-tag "Added">}}

-   Added seatbelt setup to the DA62 Blender sample.

{{< release-notes-tag "Fixed">}}

-   DA62 Blender sample:
    -   Fixed landing gear animation issues.
    -   Fixed liveries and registration number issues.


<p class="fake-h3">Documentation</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "Added">}}

-   Added documentation for the new window.
-   Added documentation for the window.

{{< release-notes-tag "Improved">}}

-   The page has been refreshed to reflect tool changes.


<p class="fake-h4">Project Editor</p>

{{< release-notes-tag "Added">}}

-   Added a new section to the **Inspector** documentation.


<p class="fake-h4">SimObject Editor</p>

{{< release-notes-tag "Added">}}

-   Added a new Inputs tab to the documentation.


<p class="fake-h4">Scenery Editor</p>

{{< release-notes-tag "Added">}}

-   Added new transition options to the Airport Object documentation.

{{< release-notes-tag "Improved">}}

-   Updated Airport Object properties documentation to match editor changes.
-   Updated Polygon Objects documentation to clarify how the **Airport Area** option affects TIN buildings.


<p class="fake-h4">SDK Tools</p>

{{< release-notes-tag "Improved">}}

-   Updated the SimObject Statistics page to highlight the most critical problematic values.


<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "Added">}}

-   Added missing keys for some FLT parameter hashmaps.
-   Added additional information related to `Approach.flt` to the General Career Mode Requirements page.
-   Updated FLT Files General Information with additional `Approach.flt` details.
-   Added new **Input** documentation pages:
    -   Aircraft Category Input Profiles
    -   actiondb XML Properties
    -   remapdb XML Properties

{{< release-notes-tag "Fixed">}}

-   Fixed incorrect documentation for the `regionCode` attribute on `<Airport>` elements (this value is a **country code**, not a region).
-   Fixed Cargo Transport weight specifications to match ingestion and validation requirements.

{{< release-notes-tag "Improved">}}

-   Refreshed and approved most pages in the Input Profiles documentation.
-   Renamed the **Cargo Transport** “Super Heavy” category to **Oversized**.


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "Improved">}}

-   SimVars:
    -   Updated Camera Variables documentation with additional details and refreshed screenshots.
    -   Improved radio indexing documentation in Aircraft Radio Navigation Variables.
-   Key Events:
    -   Improved radio indexing documentation in Aircraft Radio Navigation Events.


<p class="fake-h4">SDK Tools</p>

{{< release-notes-tag "Improved">}}

-   Removed all references to the deprecated “Input App”. Users should now use the DevMode
      Input Device Editor and
      Input Profile Editor.

{{< /expand >}}


{{< expand title="SDK Release 1.6.7" >}}

<p class="fake-h3">SDK</p>

<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "Added">}}

WASM MapView API updated with fixes and new functions.
 

<p class="fake-h3">Documentation</p>

<p class="fake-h4">Models and Textures</p>

{{< release-notes-tag "Improved">}}

The page on the LOD Selection System has been updated to show details of the current LOD curve.
 

<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "Added">}}

-   WASM:
    -   MapView API - New functions added: `fsMapViewSetWeatherRadarBankLimitsInRadians`, `fsMapViewSetWeatherRadarPitchLimitsInRadians`, `fsMapViewSetWeatherRadarScanRate`, `fsMapViewSetWeatherRadarStabilization`, `fsMapViewSetWeatherRadarTiltInRadians`.


{{< /expand >}}


{{< expand title="SDK Release 1.6.6" >}}

{{% tagged "internal" %}}

CL2409056-242533

{{% /tagged %}}

<p class="fake-h3">DevMode</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "added" >}}

- Added "Debug biomes" to the Debug &gt; Terrain menu.
- Added "Debug surface type" &gt;to the Debug &gt; Terrain" menu.

{{< release-notes-tag "fixed" >}}

- Fixed AutoSet filter disappearing from the console after Build.

{{< release-notes-tag "improved" >}}

- Merged User light tool and Aircraft debug lights.
- Merged both simobjects stats dialogs.
- Renamed Statistics Profiler to Scenery Statistics.


<p class="fake-h4">Scenery Editor</p>

{{< release-notes-tag "added" >}}

- Added option per runway to remove buildings.

{{< release-notes-tag "fixed" >}}

- Fixed SimPropContainer following the ground during SimPropContainer edition.
- Fixed SimPropContainer sometimes not spawning.
- Fixed incorrect rotation when editing SimPropContainer.
- Fixed "Edit SimPropContainer" option sometimes not using the correct position/rotation.

{{< release-notes-tag "improved" >}}

- Moved all airport navdata inside a subsection.
- Save "View only current package" and "Selection of locked objects" options in user settings.


<p class="fake-h4">SimObject Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed possible issues with edit in place mode modifying incorrect field.
- Fixed edit in place mode + live reload wrongly saving files.

{{< release-notes-tag "improved" >}}

- Merged SimAttachmentEditor in the SimObjectEditor.
- Updated cockpit.cfg params in the editor.


<p class="fake-h3">SDK</p>

<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "added" >}}

- ModelBehaviors: To reflect the change in SImVars and key events, the new "EX1" keys have been added to functions "ASOBO_Hydraulic_Valve_Parameters" and "ASOBO_Pneumatics_Valve_Parameters" and these use the correct simvars to set the target positions of valves.

{{< release-notes-tag "fixed" >}}

- Aircraft systems: Fixed hydraulics system behavior for looping fluid systems.

{{< release-notes-tag "improved" >}}

- Flight model: Blocked update of key motion parameters for user aircraft (position, orientation, translational and angular velocity vectors, translational and  
    angular acceleration vectors, total force and total moment vectors) while the user is in the in-game menu.
    - Additionally, blocked update of translational and angular acceleration vectors, as well as total force and total moment vectors, while in Active Pause mode.
    - This change ensures export of fixed (last valid) motion parameter values via SimVars in situations where motion is not actually being simulated, and also fixes a G-LOC issue during Active Pause.  
- The PERFORMANCE_DATA section of the flight_model.cfg has been moved to the flight_performance.cfg file. It will still be loaded from the flight_model.cfg if it is missing from the flight_performance.cfg to ensure backwards compatibility. 


<p class="fake-h4">Programming  APIs</p>

{{< release-notes-tag "added" >}}

- Key Events:
    - PNEUMATICS_VALVE_SET_EX1 and HYDRAULIC_VALVE_SET_EX1 have been added to replace faulty PNEUMATICS_VALVE_SET and HYDRAULIC_VALVE_SET events.
- SimVars:
    - Added SimVar MOTION_SIMULATION, which indicates whether motion simulation for the user aircraft is active.
- Camera API:
    - Added functions to load some parts of the world.
    - Added the possibility to focus a SimObject and follow its movement based on its object Id.
    - API keeps world around the aircraft loaded while using the camera functions and going far away of the aircraft to avoid it to fall under the ground.

{{< release-notes-tag "fixed" >}}

- SimVars:
    - Fixed setting auto-coordination on/off through SimVar "AUTO COORDINATION"
- WASM:
    - Fixed fseek relative to SEEK_END.
    - Fixed opendir not returning nullptr when files doesn't exists.

{{< release-notes-tag "improved" >}}

- Camera API: Changed the way position and rotation referential are handled.

- SimVars:

    - Added VarSet capabilities for Hydraulic and Pneumatic valves' target position SimVars (HYDRAULIC_VALVE_TARGET_POS and PNEUMATICS_VALVE_TARGET_STATUS), to replace their respective malfunctionning Key Events.


<p class="fake-h4">Samples</p>

{{< release-notes-tag "fixed" >}}

- DA62 sample:
    - Fixed landing gear rig and added tire deformation and UV offset animation in the Blender sample.
    - Fixed open lid in DA62 Blender sample.


<p class="fake-h4">Tools</p>

{{< release-notes-tag "added" >}}

- Blender:
    - New documentation is now included in the add-on folder io_scene_gltf2_msfs_2024\(documentation\)Open_Documentation.html
    - Added button to open documentation in browser.

{{< release-notes-tag "fixed" >}}

- Fixed possible crash when using fspackagetool.exe with a Steam build and modelbehaviors files.
- Blender:
    - Fixed invalid vertex normal on skinned meshes.


<p class="fake-h3">Documentation</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "improved" >}}

- The SDK Overview page has had an image refresh to bring it in line with the current look of the simulation.
- The pages describing the **SimAttachment Editor** have been removed, since all attachments are now edited directly in the SimObject Editor.
- Multiple pages referencing the SimAttachment editor have been updated to reference The SimObject Editor instead.


<p class="fake-h4">Project Editor</p>

{{< release-notes-tag "improved" >}}

- The Validation section of the export documentation has been updated to bring it in line with the current state of the tools.


<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "added" >}}

- The [PERFORMANCE_DATA] section has been documented in the flight_performance.cfg file.
- Multiple new (missing) elements added to the Flight Plan XML (PLN File) Properties page.
- New parameters added:
    - [[FLIGHT_TUNING]](../content-configuration/cfg-files/flight_model.cfg/#FLIGHT_TUNING) - `froude_krylov_scalar`,
- The Modular Hydraulics System Information has had the following additions:
    - New valve type added: **differential**.
    - New actuator types added: **Thrust Reverser**, **Liquid Dropping Door**

{{< release-notes-tag "fixed" >}}

- Removed references to checklists from the Visual Effects XML Properties page.

{{< release-notes-tag "improved" >}}

- Airport XML Properties has been updated to bring it in line with the BGLcomp.xsd specs.
- Runway XML Properties has been updated to bring it in line with the BGLcomp.xsd specs.
- Taxiway XML Properties has been updated to bring it in line with the BGLcomp.xsd specs.
- Scenery XML Properties has been updated to bring it in line with the BGLcomp.xsd specs.
- Flight Plan XML (PLN File) Properties has been updated to give better information and has been reformatted to match other XML spec pages.
- The Sim Attachment Libraries page has had a minor update to reflect changes in package creation.
- flight_model.cfg: The `rotor_brake_scalar` and `rotor_brake_torque` parameters have updated descriptions.


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "added" >}}

- Key Events:
    - Aircraft General Systems Events: `HYDRAULIC_VALVE_SET_EX1`, `PNEUMATICS_VALVE_SET_EX1`,
- SimConnect:
    - Camera API: SimConnect_RequestCameraWorldLocker, SimConnect_DeleteCameraWorldLocker, SimConnect_SubscribeToCameraWorldLockerStatusUpdate, SimConnect_UnsubscribeToCameraWorldLockerStatusUpdate
- WASM:
    - Camera API: fsCameraRequestCameraWorldLocker, fsCameraDeleteCameraWorldLocker, fsCameraSubscribeToCameraWorldLockerStatusUpdate, fsCameraUnsubscribeToCameraWorldLockerStatusUpdate
- SimVars:
    - Helicopter Variables: `ROTOR BRAKE AVAILABLE`.
    - Aircraft System Variables: `HYDRAULIC VALVE POS`, `HYDRAULIC VALVE TARGET POS`.
    - Miscellaneous Variables: `MOTION SIMULATION`.

{{< release-notes-tag "improved" >}}

- Key Events:
    - The following Aircraft General Systems key events have been flagged as legacy: `PNEUMATICS_VALVE_SET`, `HYDRAULIC_VALVE_SET`.

{{< /expand >}}


{{< expand title="SDK Release 1.6.5" >}}

{{< tagged "internal" >}}

CL2390870 - 2408284

{{< /tagged >}}

<p class="fake-h4">General</p>

{{< release-notes-tag "fixed" >}}

- Fixed some debug not correctly reset when closing the devmode.
- Fixed a possible crash when using long names in UserLight Tool.

{{< release-notes-tag "improved" >}}

- Removed "Debug SoundTracks" tool from public builds (showing CRCs in this configuration made the tool useless).


<p class="fake-h4">Project Editor</p>

{{< release-notes-tag "added" >}}

- Added new package validator rules at build and export.

{{< release-notes-tag "improved" >}}

- When created a package with a custom asset group the popup asking for order hint now appears at the end.


<p class="fake-h4">Scenery Editor</p>

{{< release-notes-tag "added" >}}

- Added material name in the decal name.
- Support "region code", 'transition altitude" and "transition level" for airports in the editor.

{{< release-notes-tag "fixed" >}}

- Fixed SimPropContainer scalable in the editor but not ingame.

{{< release-notes-tag "improved" >}}

- Improved rectangle heightmap edition:
    - The heightmap button automatically enable the terraforming.
    - More grid display option.
    - Independent brush strength.
    - Weaker inflate brush.


<p class="fake-h4">SimObject Editor</p>

{{< release-notes-tag "added" >}}

- Added prop_mod_version field.
- Added Avidyne IFD540 and IFD550 included sim attachments for use by aircraft developers.

{{< release-notes-tag "fixed" >}}

- Fixed empty aicraft.cfg file created on save.
- Fixed legacy fields always hidden in search dialog even when use legacy is on.


<p class="fake-h4">SimAttachment Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed loading error.


<p class="fake-h3">SDK</p>

<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "added" >}}

- Added a way to indicate the speed level at which gears take damage when immersed in water.

{{< release-notes-tag "improved" >}}

- Improved Mach number display precision in "Debug Aircraft Tracking" window (from 1 to 3 decimal places).


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "added" >}}

- Added simvar PNEUMATICS_ENGINE_BLEED_AIR_PRESSURE_EX1 that actually gives the bleed air exiting from the engine, whereas PNEUMATICS_ENGINE_BLEED_AIR_PRESSURE gave the pressure of the air inside of the engine.

{{< release-notes-tag "fixed" >}}

- Camera API: Fixed targetted position variable in World referential which were computed wrongly.
- WASM: Fixed approximation error when spawning vfx in world.


<p class="fake-h4">Tools</p>

{{< release-notes-tag "fixed" >}}

- Blender:
    - Fixed da62 sample scene (propeller, LOD issues).
    - Fixed export of skinned meshes, it now works in all cases.
    - Fixed export of skinned propellers.
    - Fixed incorrect positioning of children of a skinned node.
    - Fixed “Reset Origin” not working correctly when the parent node also has the option enabled.
    - Fixed export error when exporting armature in rest pose.
    - Fixed an error when saving
    - Fixed an issue where the clearcoat extension export failed when the detail normal map was not found on disk.
    - Fixed material property"Receive Rain" not saved on clearcoat and windshield.
- BGL Explorer: Fixed incorrect parking information.
- SimConnect: Fixed facilities pavement "enable" member always set to false.

{{< release-notes-tag "added" >}}

- Blender:
    - Added a warning when exporting a skinned object without its armature.
    - Added a warning when “Reset Origin” is enabled on a skinned object.

{{< release-notes-tag "improved" >}}

- Blender:
    - Set the default animation export mode to “ACTIONS” in Blender 4.5. This is now the recommended method for exporting animations, as action slots are simpler to use and export times are shorter since keyframes are not baked by default, unlike the “NLA Tracks” mode.
    - Increased Material UV Tiling maximum value from 10 to 100.
    - Fixed detail normal map on the windshield material.


<p class="fake-h3">Documentation</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "fixed" >}}

- Some minor CSS issues with the light skin have been fixed.
- RMB on the right-hand "mini-ToC" can now be used to copy the section link correctly for sharing.
- Small issue with home page file name has been fixed.


<p class="fake-h4">DevMode</p>

{{< release-notes-tag "added" >}}

- New pages to clarify the creation and use of Sim Attachments and Sim Attachment Libraries have been added.

{{< release-notes-tag "improved" >}}

- The SimAttachment Editor documentation has been updated to reflect the current editor state.
- The Rectangle Objects page has been updated to match the current state of the object properties.


<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "added" >}}

- The [FUEL_SYSTEM] version information has a new version (v8) added.

{{< release-notes-tag "fixed" >}}

- Mistake in the Adding Aircraft VFX code related to FX Graph parameters has been fixed.

{{< release-notes-tag "improved" >}}

- Model behavior templates updated.
- All tables have been tidied for readability, and parameter names should no longer split mid-word, but rather on underscores.


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "added" >}}

- New Input Events: `COLLECTIVE_RELATIVE_AXIS`, `HELICOPTER_THROTTLE_RELATIVE_AXIS`, `MIXTURE_RELATIVE_AXIS`, `PROPELLER_RELATIVE_AXIS`, `THROTTLE_RELATIVE_AXIS`, `SPOILERS_RELATIVE_AXIS`,
- The SimConnect_AddToFacilityDefinition page has anew "schema" image showing the struct hierarchy.
- New SImVar: `PNEUMATICS ENGINE BLEED_AI _PRESSURE EX1`

{{< release-notes-tag "improved" >}}

- The SimConnect_AddToFacilityDefinition page has been tidied and made easier to follow.

{{< /expand >}}



{{< expand title="SDK Release 1.6.4" >}}

{{< tagged "internal" >}}

CL2168484 - 2390870

{{< /tagged >}}

<p class="fake-h3">DevMode</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "added" >}}

- Added support to gamepad in Cotent Creator Dialog.
- Added new DevMode window to display accessibles frequencies (Debug/Aircraft/Radio Frequencies).
- Added 2 new buttons on the Animations tab of the Model Behaviors debugger to filter in/out animation which are missing events and those that are not  
    in the GLTF.
- Added Input Device editor.
- Added Input Profile Editor.
- Added option to export current FLT in Debug / Aircraft / FLT files.
- Added time-step statistics to the "Integration and Rotation Simulation" section of the "Debug Aircraft Weight" window.
- Added consumers list in battery details panel in debug window.
- Added batteries in consumer list of suppliers in debug window.
- Added a "Propeller RPM Limiter Analysis" debug sub-window, accessible via checkbox in the "Debug Aircraft Engines" window. The tool provides a clear and efficient way to tune the new propeller RPM limiter.

{{< release-notes-tag "fixed" >}}

- Fixed Calculator debug dialogs missing.
- Fixed an issue in the "Debug Aircraft Flight Performance" window where pressure altitude was incorrectly interpreted as true altitude during turbine engine performance calculations and export to a file. (Note: This difference between pressure and true altitude increases with altitude when temperature deviates from ISA).
- Fixed fuel tank debug box flipped along the X axis.
- Fixed possible crash when opening the Aircraft Capture Tool.
- Fixed issue with time slider setting incorrect day when local time goes after 23:59.
- Fixed ILS debug color.
- Fixed resize cursor not working.
- Fixed SimObjectSpawner animation list being displayed for SimObject with no animation.

{{< release-notes-tag "improved" >}}

- Improved visualization & colors of all turbulence visualizations (cfd, wake & thermals).
- Improved CFD simulation on building edges
- Removed some irrelevant options from the Devmode Debug menu.
- Removed Camera Editor from public build (obsolete).


<p class="fake-h4">Project Editor</p>

{{< release-notes-tag "added" >}}

- Added templates for SimAttachmentLib asset group to clarify format.

{{< release-notes-tag "fixed" >}}

- Fixed SPB compiler not reporting XML parsing errors when building a project.


<p class="fake-h4">Scenery Editor</p>

{{< release-notes-tag "added" >}}

- Added exclude vegetation and exclude buildings option for helipads.
- Added a warning when ILS frequency not use odd digits in the tenths.
- Added "Ground as reference" option in the scenery editor.
- Added support for instancing of scenery objects that are not snapped to ground.
- Added a warning when a model GUID is not found.

{{< release-notes-tag "fixed" >}}

- Fixed incorrect taxipath edge markings.
- Fixed generated skid marks taking TIN color correction.
- Fixed surface draw order when using aprons with large negative priority.
- Fixed runway tree exclusion when the runway altitude is below 0.
- Fixed airport terraforming generated too late for big airports.
- Fixed rendering issue when using the same instanced and non instanced mesh in a scene.
- Fixed light exclusion polygons not working far away.
- Fixed vegetation density applied by mistake by the editor.
- Fixed gizmo window not applying transformation from input fields.
- Fixed missing center line in taxi parking.
- Fixed grass detail map applied on gravel and sand surfaces for MSFS2020 airports.
- Fixed backward compatibility issue for the airport "apply flatten". MSFS (2020) packages no longer take into runways to compute the terraforming area.
- Fixed apron UV difference between editor and in game. **IMPORTANT! package built with SU5 might have apron UV changes!**
- Fixed 3rd party aerials sometimes not properly displayed on areas that are censored on Bing maps.

{{< release-notes-tag "improved" >}}

- Runway exclusion is now weaker on TIN trees.
- Optimization of airport parking lots parked cars system.
- Hide light type for painted hatched area.
- Removed "Reset points altitude" option from context menu of objects that are always on ground.
- Optimized object window.
- Improved popup to select a model.
- Improved moving / rotating a scenery object with snap to normal enabled.


<p class="fake-h4">SimObject Editor</p>

{{< release-notes-tag "added" >}}

- Added live edition link with jetways for editing interactive points.
- Added min/max constraint in Curve Editor.
- Added new window to debug current container file merge.
- Added fuel system version 7 to enum values.
- Added enum values for ui_typerole field in aircraft.cfg.

{{< release-notes-tag "fixed" >}}

- Fixed default value with required field in material_guid.
- Fixed light live edition after rebuild.
- Fixed an issue causing aircraft with multiple propellers to sometimes have only a single prop rotating after rebuilding the aircraft's package (visual only).
- Fixed some combobox falling back to input field when disabling validation checks.
- Fixed SimObjectEditor live edition and cockpit.cfg.
- Fixed issue with modified state while in live edition mode.
- Fixed parsing of floating point number in pair-type parameters.
- Fixed wrong rotation used in live edition mode for simattachments.
- Fixed sometime check for outside modification triggering incorrectly.
- Fixed system.cfg / areasmintemperature validation condition.
- Fixed sometime check for outside modification triggering incorrectly.
- Fixed navgraph editor automatically opening when switching to navgraph tab.

{{< release-notes-tag "improved" >}}

- Removed input slider in SimObject Editor.
- Updated View menu order.


<p class="fake-h4">Biome Editor</p>

{{< release-notes-tag "added" >}}

- Added reload the biomes on save.

{{< release-notes-tag "fixed" >}}

- Fixed displaying too many surface types.


<p class="fake-h4">VFX Editor</p>

{{< release-notes-tag "improved" >}}

- Improved node list draw time.


<p class="fake-h3">SDK</p>

<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "added" >}}

- Added a step to remove windshield cover in preflights.
- Added new versioning system to propeller system.
- Added missing parameters to define power and thrust threshold behavior on legacy propellers with low beta.
- Added new parameters for surfaces: version and modifier_local_angle_scalar.
- Added ZVelBodyAxis_IAS FLT parameter to specify an airspeed in IAS.
- Added an extra type of cover for windshield + new generic covers to be used for anything that doesn't match the existing cover types. These can all be removed with mission scripts.
- Added support for two types of propeller RPM limiter:  
    - Pitch angle governor–based (for constant-speed propellers only.  
    - Fuel flow governor–based (for turboprop engines only)  
    The limiter is configured via newly added parameters in the [PROPELLER] section of the engine.cfg file.

{{< release-notes-tag "fixed" >}}

- Fixed an issue causing crashes on aircraft with a very number of lines and circuits.
- Fixed an issue that prevented Localization Files from being used in ModelBehaviors within SimAttachments.
- Fixed the SimAttachment heading_indicator_two_knobs heading bug that did not follow the movement of the compass.
- Fixed an issue causing hydraulic rudder actuator to be inoperative if the hydraulic system either does not have an elevator actuator, or that actuator is not operative.
- Fixed wrong force feedback on aircraft that was causing the balloon basket to tilt under extreme windspeeds.
- Fixed an issue causing helicopter hydraulic assistance to be enabled even on aircraft not on the FS2024 Hydraulic system, potentially making them uncontrollable.
- Fixed issue introduced in Sim Update 4 causing visibility behavior code to still be running even when current LoD does not contain the linkednode.
- Fixed an issue on the FSX fuel system causing the fuel level of tanks to be overridden on FLT loads even if that FLT does not provide any fueltank information.
- Fixed Fuel Pump AutoCondition parameter not working when using decimal values.
- Fixed an issue that could cause the flight surfaces to take damage while the aircraft was loading.
- Fixed pneumatics "OpeningNames" initialization to properly take them into account.
- Fixed an issue causing the rotorcraft covers to sometimes appear offset from the rotor model in multiplayer.
- Fixed slow snapping to ground for VectorPlacement.
- Fixed Registration in interior model sometimes not working.
- Fixed model.cfg option withExterior_showInterior causing issue with model loading.
- Fixed Wear & Tear info not fully loaded from cfg file.
- Fixed suppliers having no load when directly connected to a consumer.
- Fixed breakers not having any load when on a line leading to a bus. Minor version 1 or above of the electrical system will have to be set in the settings.cfg definition.

{{< release-notes-tag "improved" >}}

- Wake turbulence intensity finetuning for NPC aircraft.
- When in modern propeller version 2 or later, rotation is ignored as a way to define rotation direction of propelers as it is a deprecated way to define rotation orientations.
- Upgraded Tobii SDK to version 9.0.4.26.
- Improved wake turbulences simulation, including self induced feedback loop on helicopters


<p class="fake-h4">Tools</p>

{{< release-notes-tag "added" >}}

- BglExplorer:
    - Added a tool to search text.
    - Added apron information.
    - Add missing helipad start, approach and VASI lights information.
- Blender:
    - Added Cancel Export button.
    - Added button to show exporter in a floating window.
    - Added a new Export Selected option to the exporter right-click menu.This allows users to export selected items without checking or unchecking list entries.
    - Added "Reset Filters" button in filters panel.
    - Added “Check All” and “Uncheck All” buttons to the list view right-click menu. Available in all list views containing checkable items.
    - Added support for deleting multiple presets using multi-selection.
    - Added logs to inform user when gltf, bin and xml files are set to read-only. If gltf or bin files are set read only, then export is cancelled.
    - Added export time logs.
    - Added new pre-export log messages to warn about empty presets, empty LODs, unset export folders, invalid texture directories etc...
    - Added “Select All” and “Deselect All” options to all exporter list context menus.The menu appears when right-clicking an item.
    - Added New Image Flag Tool: set flags on multiple images at once. Access it from menu "MSFS2024" &gt; "Set Image Flags".
    - New logging system implemented.
    - Added support for Blender 4.5 and the new Action Slots feature.

{{< release-notes-tag "fixed" >}}

- Blender:
    - Fixed invalid tire material type when importing into Blender.
    - Fixed incorrect material export on instanced objects when background export is disabled.
    - Fixed an issue where the XML was missing LOD entries when re-exporting a model with only a subset of its LODs.
    - Fixed lods not in correct order in generated xml when lods have different name prefixes.
    - Fixed incorrect mask preview when a material uses a blend mask.
    - Exporter now correctly handles object and image names with Blender’s numbered suffixes (e.g., “.001”).
    - Resolved a rare issue where export progress would freeze when “Export in Background” was enabled.
    - Fixed export of unwanted objects when working with multiple scenes in a single Blender file.
    - Resolved a rare issue where export progress would freeze when “Export in Background” was enabled.
    - Fixed export of unwanted objects when working with multiple scenes in a single Blender file.
    - Fixed incorrect conversion from light temperature to light color.
    - Fixed Light color not working correctly in Kelvin mode.
    - Fixed missing presets when loading scenes created with older SDK versions.
    - Fixed UI list filters not displaying items that are children of collapsed entries.
    - Fixed glass material not correctly imported.

{{< release-notes-tag "improved" >}}

- Blender:
    - XML generation is now more conservative, aiming to preserve user-defined custom setups whenever possible (e.g., animation declarations, auto-LOD configurations, etc.).
    - Light types are now exposed in the Light Data panel. The light type can be changed on the fly without needing to recreate the light.
    - Drastically reduced export times on big scenes : ( 1min30 to 11 seconds per gltf in a scene with more than 17k objects).
    - Material animation export has been fully reworked and now works reliably in all cases, including animated materials on static objects. In Blender 4.5, material animations can be exported using both “Action Mode” and “NLA Tracks”. In Blender versions earlier than 4.5, material animations can only be exported in “NLA Tracks”. Material animation export is not supported in Blender versions earlier than 3.6.
    - Removed unsupported animation modes from export settings. “NLA Tracks” and “Actions” are now the only supported modes.
    - Refactored collision primitives: they are now curves using a dedicated Geometry Nodes modifier.Collisions can now be selected directly by clicking on their gizmo shapes.
    - Exporter is now compatible with Blender files that contain multiple scenes.
- 3DS Max:
    - Support 3ds max 2025 and 2026.


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "added" >}}

- WASM:
    - Added HEvent in Event API.
    - Added Camera API.
- SimConnect:
    - Added possibility to use CommBus with Simconnect.
    - Added Camera API.
    - Added possibility to add I O and Z vars to Data Definition.
- Key Events:
    - Added COLLECTIVE AXIS RELATIVE, HELICOPTER THROTTLE AXIS RELATIVE, MIXTURE AXIS RELATIVE, PROPELLER AXIS RELATIVE, SPOILERS AXIS RELATIVE, CLOSE_AIRCRAFT_DOORS, CLOSE_AIRCRAFT_DOORS_CRASHING, OPEN_AIRCRAFT_DOORS, SET_AIRCRAFT_DOORS

{{< release-notes-tag "fixed" >}}

- SimConnect:
    - Fixed a SimConnect where the sim stopped sending some messages when a new CSimScheduleGroup was added.
    - Fixed a SimConnect crash that could occur on client destruction if it used NavData API.
- WASM:
    - Fix get_name_of_name_variable causing wasm dirty and returned string getting corrupted.
    - Fixed freeze when set position of VFX just after spawning it using the VFX API.
    - Fixed a race condition when adding textures through the LLVG API which could cause random crashes with WASM modules.
    - Fixed Wasm modules sometimes going dirty after closing a simconnect connection.
    - Fixed an infinite loading that can occurs when wasm modules get simconnect open dispactch with a GetNextDispatch and have a CallDispatch registered.
    - Fixed access to O and I vars on some component of the aircraft.
    - Fixed remove directory in work folder.
    - Fixed consistency between readdir ino and fstatat ino.
    - Fixed access to some AVar stored in buffer (with fsVarsAVarBufferGet).
- SimVars:
    - Fixed simvars used in system definitions cfg not using requested units but default one instead. Version 2.2 and up required.
    - Fixed an issue causing aircraft using BUS LOOKUP INDEX simvar in different XML variable to sometimes cause some of the Simvar read/writes to receive invalid indices.

{{< release-notes-tag "improved" >}}

- SimVars:
    - PROP_THRUST simvar now displays modern thrust value if version is 2 or higher.
- WASM:
    - When Wasm has been waited for too long during a frame, modules are now suspended for the rest of the frame to allow the game to continue to run.


<p class="fake-h4">Samples</p>

{{< release-notes-tag "fixed" >}}

- Fixed missing gear compression on DA62.

{{< release-notes-tag "improved" >}}

- Cleaned Wasm samples to use the new Vars API.


<p class="fake-h3">Documentation</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "added" >}}

- The documentation now has a new "Home" page, specifically for users finding the docs through Google or other serach engines, with the intention of ensuring that the correct documentation is being used (2020, 2024 Flighting, 2024 Retail).

{{< release-notes-tag "fixed" >}}

- Fixed the button that changes between retail/flighting docs to default to the introduction if the page URL cannot be resolved.
- Fixed CSS error with the release notes expanding text.

{{< release-notes-tag "improved" >}}

- Minor reorganisation of the ToC main contents (SDK Tools is now further down the list).
- World Hub documentation removed until the site and documentation can be updated.


<p class="fake-h4">DevMode</p>

{{< release-notes-tag "added" >}}

- New Propeller RPM Limiter Analysis debug window added to the docs.
- New Radio Frequencies debug window added to the docs.
- New RMB options for the following Scenery Objects have been documented:
    - Apron Objects
    - CarParking Objects
    - LightRow Objects
    - PaintedHatchedArea Objects
    - PaintedLine Objects
    - Polygon Objects
    - Rectangle Objects
    - TaxiwayPath Objects
    - TaxiwayPoint Objects
    - VectorPlacement Objects
- New filter options for animations have been added to the Behavior Tabs page.
- New **Export** option in the FLT Files debug window added.
- New Merged Files debug window for the SimObject Editor has been added.
- New Input Device Editor has been added.
- New Input Profile Editor has been added.
- New Force Midday When The Editor Is Open option for the Scenery Editor has been added.
- New **Boost Intensity During Daylight** option added for Airport Objects light presets.
- New **Exclude Vegetations** and **Exclude Buildings** options added to the Helipad Objects properties.

{{< release-notes-tag "fixed" >}}

- The debug POI window has been removed from the documentation as it is no longer available.

{{< release-notes-tag "improved" >}}

- Removed misleading information suggesting you could save FLT files from the simulation UI without using DevMode tools.
- Information on **Model Info** has been added to the following objects:
    - Scenery Objects
    - ProjectedMesh Objects
    - ControlTower Objects
- The Objects section of the View menu page for the Scenery Editor has been updated to include information on *Tags*.


<p class="fake-h4">SDK Tools</p>

{{< release-notes-tag "added" >}}

- New tool for tuning turbine engines has been added to the documentation and SDK: MSFS Turbojet Static Performance Tool.


<p class="fake-h4">Models And Textures</p>

{{< release-notes-tag "added" >}}

- Some extra information has been added to the Propellers, Turbines And Blades related to the pivot position and it's effect on the animation shader.
- Some extra information has been added specifically to help Blender users set up Exterior Lights.
- New section for Collective Animation has been added to the Yoke Animation page (the page has also been renamed)

{{< release-notes-tag "fixed" >}}

- Blender:
    - The Blender Exporter page explains new RMB menu options,
    - New Set Image Flags window added to the documentation.
- Fixed issue where MODEL was used instead of MESH on the Dirt And Grime page.

{{< release-notes-tag "improved" >}}

- The page dedicated to VR Helpers has been updated with additional information to help clarify why these are necessary and how to set them up.
- The FlightSim Materials page has been updated to include Blender information.
- The Exterior Lights, Cockpit And Cabin Lights, and Implementing Lights, pages have all been updated to match the curresnt state of the simulation.
- The Propellers, Turbines And Blades page has been updated with additional information on rotation and turbine setup.


<p class="fake-h4">Sound</p>

{{< release-notes-tag "added" >}}

- A new page has been added to help with Sound Optimisations.


<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "added" >}}

- New pages have been added to explain how to setup your own World Photographer travel book pages:
    - Travelbook Setup (World Photographer Missions)
    - Travelbook Page XML Properties
    - Travelbook Objective XML Properties
    - Travelbook XML Examples
    - Travelbook Lists
- A new page related to Air Traffic has been added (note this information was previously available mixed in with other AI traffic information, and has simply been split into its own page to make the information easier to find).
- A new note has been added for engine propeller configuration: Note On Propeller Pitch Angle
- A new section on Mission Solving Constraints has been added to the General Career Mode Requirements page.
- The flight_model.cfg has the following new parameters:
    - [AIRPLANE_GEOMETRY]: `wing_pos_refchord`
    - [Point.N]: New point list entries (17 and 18) for overspeed damage and catestrophic damage (crashes) when the landing gear is in water.
    - [OBJ_EA1_SURFACE.N]: `version`, `modifier_local_angle_scalar.n`
- The engines.cfg file has the following new parameters:
    - [PROPELLER]: `prop_pitch_control_min_oil_pressure`, `prop_pitch_control_nominal_oil_pressure`, `prop_betathreshold`, `prop_betathresholdpower`, `prop_betathresholdthrust`, `prop_mod_version`
    - [TURBINEENGINEDATA]: `reverser_allowed_in_flight`
- The systems.cfg has the following new parameters:
    - [AUTOPILOT]: `heading_mode_prefer_smallest_angle`
- The flight_performance.cfg has the following new sections and parameters:
    - [PERFORMANCE_DATA]: `TakeOff`, `Climb.N`, `Cruise`, `Descent.N`, `Approach`, `Landing`
- The sim.cfg page has a new section for [DesignSpecs] with the following new parameters: `empty weight`, `empty cg offset`
- The FLT File Properties page has the following new parameters:
    - [Covers]: `windshield`, `generic.N`
    - [SimVars.N]: `ZVelBodyAxis_IAS`
- A new Note On Propeller Rotation Direction has been added.
- New pages have been added related to creating Input Profiles:
    - Input Profiles
    - Device Profiles
    - Transversal Input Profiles
    - Aircraft Category Input Profiles
    - DeviceConfig XML Properties
    - Input Configuration XML Properties

{{< release-notes-tag "fixed" >}}

- Fixed some typos in the descriptions for the `head_hold_pid` and `airspeed_hold_pid` parameters in the systems.cfg.
- Fixed the `[NITROUS SYSTEM.N]` header to remove the erroneous underscore.
- Fixed the Preflight page to include mention of the interaction manager when setting up navigation graphs.

{{< release-notes-tag "improved" >}}

- The Package Tool XML Properties page has been updated with the latest package order hints.
- The `<CompileBehaviors>` , `<Behaviors>`, and `<ModelBehaviors>` elements of the model XML files have been updated with improved version and revision information.
- The following engines.cfg parameters have had their descriptions updated: `variable_inlet`, `supersonic_inlet`, `supersonic_inlet_efficiency_correction_table`, `supersonic_inlet_design_mach`, `high_n1`, `high_n2`, `min_n2_for_fuel_flow`, `prop_mod_aoa_lift_delta_align_beta_deg`, `prop_mod_use_modern`.
- The following cameras.cfg parameters have had their descriptions updated: `InitialZoom`.
- The following flight_model.cfg parameter have had their descriptions updated: `static_cg_height`, `max_water_depth`, `wing_cg_refchord`, `min_flaps_for_spoilerons`.
- The Note On Computing Commanded Ne has had minor updates to improve readability.
- The Modular Hydraulics System Information has been improved with additional links to relevant Key Events and FLT parameters.
- The Preflight page has been updated to include mention of the `<GENERIC_COVER_INDEX>` template element.
- The Program Template XML Properties page has been updated to clarify the `<Do>` loop usage.


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "added" >}}

- The Coherent.js section has stub (WIP) pages added for the following: `SEND_ROUTE_TO_EFB`, `GET_EFB_ROUTE`, `SAVE_EFB_ROUTE`, `SEND_ROUTE_TO_AVIONICS`, `REQUEST_AVIONICS_ROUTE`, `REPLY_TO_AVIONICS_ROUTE_REQUEST`, `FILE_ROUTE_WITH_ATC`, `GET_ATC_ROUTE`, `LAUNCH_PLN_DIALOG`, `FIND_ROUTE`, `GET_MISSION_ROUTE_LOCKS`, `CAN_FILE_WITH_ATC`, `AvionicsRouteRequestResponse`, `EfbRouteFound`, `EfbRouteUpdated`.
- New SimVars added to the documentation:
    - Aircraft Engine Variables - `TURB ENG INLET TEMPERATURE`, `PROP LOCK`, `TURB ENG ANIMATION RPM`.
    - Aircraft System Variables - `KOHLSMAN SETTING HG EX1`, `KOHLSMAN SETTING MB EX1`, `PNEUMATICS ENGINE BLEED AIR PRESSURE`,
    - Aircraft Brake/Landing Gear Variables - `BRAKE LEFT TEMPERATURE`, `BRAKE LEFT TEMPERATURE EFFECT`, `BRAKE LEFT WEIGHT REMAINING`, `BRAKE RIGHT TEMPERATURE`, `BRAKE RIGHT TEMPERATURE EFFECT`, `BRAKE RIGHT WEIGHT REMAINING`
    - Aircraft Misc. Variables - `COVER GENERIC ON`
    - Helicopter Variables - `VORTEX RING STATE PROTECTION ASSISTANCE IMPACT`
    - Miscellaneous Variables - `FAUNA BEHAVIOUR STATE`, `FAUNA FORCE MAGNITUDE`, `FAUNA HEADING ROTATION ANGLE`, `FAUNA TURN RATE`, `GENDER`, `HEIGHT`, `IS CROUCH`, `IS TALKING`
- New Key Events added to the documentation:
    - Doors - `CLOSE_AIRCRAFT_DOORS`, `CLOSE_AIRCRAFT_DOORS_CRASHING`, `OPEN_AIRCRAFT_DOORS`, `SET_AIRCRAFT_DOORS`
    - Aircraft Misc. Events - `COVER_SET`, `TOOLS_QUICK_PREFLIGHT`
- New Camera API has been documented:
    - SimConnect: Camera
    - WASM: Camera API
- Various additions have been made to the Communication (CommBus) API:
    - SimConnect: New Communication API section added.
    - JavaScript: New Communication API section added.
    - WASM: New CommBus information related to SimConnect added - FsCommBusBroadcastFlags.
- The WASM Event API has the following new functions and enums: `fsEventsRegisterHEvent`, `fsEventsHEventCall`, `FsEventsError`

{{< release-notes-tag "fixed" >}}

- Fixed the file paths for the SimConnect INI Definition and SimConnect XML Definition pages.
- Erroneous information in the `Pause_EX1` description has been fixed.
- Minor change to the Ferry Flights and Flightseeing / First Flight documentation to specify that aircraft with floats *and* wheels can perform these career missions.
- Incorrect description for `FieldName` on the SimConnect_AddToFacilityDefinition page has been fixed.
- Incorrect indexing of example code and descriptions on the GDI+ page has been fixed.
- Incorrect information given for the following key events:
    - Aircraft Engine Events: `PROP_LOCK_OFF`, `PROP_LOCK_ON`, `PROP_LOCK_SET`, `PROP_LOCK_TOGGLE`
    - Helicopter Specific Events: `AXIS_ROTOR_BRAKE_SET`, `ROTOR_AXIS_TAIL_ROTOR_SET`

{{< release-notes-tag "improved" >}}

- SimVar documentation improvements:
    - All the SimVar pages have been updated to remove return/set values from the "parameters" column, and this column has now been renamed as "Index", to prevent confusion over what is an input index or component, and what is a return or settable value. In addition, column formatting has been improved for readability.
    - The descriptions for the following SimVars have been updated: `AIRCRAFT WIND X`, `PROP THRUST`, `TCAS INTRUDER DATA`, `PNEUMATICS ENGINE BLEED AIR PRESSURE`,
    - The following SimVars have been moved to the correct section (Aircraft System SimVars) and have had their descriptions updated: `ATTITUDE BARS POSITION`, `ATTITUDE CAGE`, `ATTITUDE INDICATOR BANK DEGREES`, `ATTITUDE INDICATOR PITCH DEGREES`
- The entire section on the SimConnect SDK has been restructured to improve flow and fix issues with documentation breadcrumb links.
- The WebAssembly Core And Helpers page has had some minor updates.


<p class="fake-h4">Samples And Tutorials</p>

{{< release-notes-tag "added" >}}

- A new page has been added related to the CameraAircraft variation of the WASM Modular Aircraft sample.
- A new section detailing the CommBus sample project has been added to the SimConnect samples.

{{< release-notes-tag "improved" >}}

- The name of the aircraft checklist page has been changed to Aircraft Creation Workflow.

{{< /expand >}}



{{< expand title="SDK Release 1.5.7" >}}

<p class="fake-h3">DevMode</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "fixed" >}}

- Fixed crash when disabling/enabling devmode.
- Fixed possible crash in the Aircraft Capture Tool if the selected aircraft is not correctly loaded.


<p class="fake-h4">Scenery Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed grass flying above big slopes.
- Fixed crash in the profile editor when clicking outside the window.
- Fixed fast light distance for VectorPlacement and instanced models.
- Fixed bounding box culling issue with instanced models.
- Fixed vegetation appearing too close to runway.
- Fixed beacon light jitering when moving camera.
- Fixed floating simobjects (e.g. flames on oilrigs).
- Fixed rendering issue on beacon lights.

{{< release-notes-tag "improved" >}}

- Avoid CPU max out at 100% when editing airports.
- Changed "Hide / Skip vegetation details" to "Hide / skip ground details".
- Faster scenery objects snapping to ground.


<p class="fake-h3">SDK</p>

<p class="fake-h4">Models And Textures</p>

{{< release-notes-tag "fixed" >}}

- Fixed helicopter rotor animation rotoscopic effect.


<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "added" >}}

- Added support for cockpit.cfg parameters for the Helicopter Power HUD Gauge.

{{< release-notes-tag "fixed" >}}

- Fixed an issue that caused SimAttachments that were hidden due to their LoD to stop executing their ModelBehaviors even with the always_execute_model_behavior param set to 1.
- Fixed the erroneous formatting of a Fuel System error message that could cause a crash.
- Fixed random crashes in XML Gauges loading.
- Fixed an issue that caused Approach.FLT to be loaded on skip to descent, even on cases where the aircraft should in a final configuration. It should now load final.FLT.
- Fixed a crash in the cabin service navigation graph system.
- Fixed a crash in the navigation graph system.

{{< release-notes-tag "improved" >}}

- Updated system.cfg / electrical so a supplier or consumer can be provided with it's name/id or config.


<p class="fake-h4">Tools</p>

{{< release-notes-tag "fixed" >}}

- Blender:
    - Fixed issue where the ambient occlusion texture's XML was not generated when 'Keep Original' was enabled.
    - Fixed UV1 and UV2 being swapped when exporting with 'Export in Background' disabled in Blender 3.6.
    - Fixed lights kelvin mode.

{{< release-notes-tag "improved" >}}

- SimVar Watcher:
    - Updated SimVars watcher to match the actual data (SimVars) in the Sim.
- Blender:
    - Set Gltf export setting "Disable Viewport for Improved Objects" to false by default , as enabling it can break animation in certain scenarios.


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "added" >}}

- WASM:
    - Added a C++ Wrapper around Vars API in SimUtils Library.
    - Added visual feedback for failed WASM gauges (red/orange background + some relevant information to identify the faulty package/module).
- SimVars:
    - Added a new SimVar `VORTEX RING STATE PROTECTION ASSISTANCE IMPACT` which indicates how much the current collective lever position has been increased by the Vortex Ring State Protection Assistance, compared to the player's input.

{{< release-notes-tag "fixed" >}}

- Key Events:
    - Fixed an issue that caused some lights to not work properly when using _SET events with an index as parameter.
- SimConnect:
    - Fixed buffer overflow crash in the simconnect system and added warning messages.
    - Fixed scheduling crash in the simconnect system and added warning messages.
    - Fixed SimConnect_SubscribeToFacilities(_EX1).
    - Fixed EnumerateSimObjectsAndLiveries interface with Wasm which made the call of this function ineffective.
    - Fixed random crash when Data Definitions are cleared while existing requests reference it.
- WASM:
    - Fixed infinite loading when lauching a flight with a plane containing a VCockpit contains multiple WASM gauges that reference a module that doesn't exist.
    - Fixed failed gauges visual feedback showing for all those of a VCockpit even if only a subset failed.
- SimVars:
    - Fixed parsing of SimVar units when loading an electrical system.


<p class="fake-h4">Samples</p>

{{< release-notes-tag "fixed" >}}

- Fixed typo in wasm samples. Replaced XXX_gauge_kull by XXX_gauge_kill.
- Fixed typo in Living World - Traffic Vehicle sample.

{{< release-notes-tag "improved" >}}

- Removed linker debug info from the Release configuration of the GdiPlusModule project in the WasmAircraft sample.


<p class="fake-h3">Documentation</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "fixed" >}}

- Some pages related to the old method for creating input profiles have been removed as they were still showing up in search results (new documentation will be available in the next update).


<p class="fake-h4">DevMode</p>

{{< release-notes-tag "added" >}}

- One-click placing has been documented for The Navigation Graph Editor.


<p class="fake-h4">SDK Tools</p>

{{< release-notes-tag "fixed" >}}

- Fixed errors in the images for the Using The Localization Manager page.


<p class="fake-h4">Models And Textures</p>

{{< release-notes-tag "added" >}}

- A new page has been added giving guidelines on how to create Passive Aircraft.

<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "fixed" >}}

- The Flightseeing / First Flight page has been fixed to show the correct number of SIT nodes for the navigation graph.

{{< release-notes-tag "added" >}}

- A new section and parameters has been added to the sim.cfg file: `[stability coefficients]`
- A new section has been added to explain .
- The flight model CFG page has the following new parameters:
    - `[AIRPLANE_GEOMETRY]` - `wing_cg_refchord`
    - `[WEIGHT_AND_BALANCE]` - `empty_inertia_tensor`
- The systems.cfg has a new section: [NIGHT_VISION].
- The Physics Objects Information page has the following new section: Note On The Inertia Tensor
- The FLT File Properties now has information on the new `[NIGHT_VISION]` section and parameters.

{{< release-notes-tag "improved" >}}

- Changes have been made to the Commercial Flights And Passengers page to clearly show the difference between small/medium planes and airliners, specifically drawing attention to the fact that small/medium planes cannot be used for commercial flight missions in career mode.
- The `[ELECTRICAL]` system major/minor descriptions have been updated for clarity.


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "fixed" >}}

- The description for `SIM SHOULD SET ON GROUND` has been corrected and updated.

{{< release-notes-tag "added" >}}

- New keys related to have been added: `NIGHT_VISION_DISPLAY_OFF`, `NIGHT_VISION_DISPLAY_ON`, `NIGHT_VISION_DISPLAY_SET`, `NIGHT_VISION_DISPLAY_TOGGLE`, `NIGHT_VISION_INTENSITY_DEC`, `NIGHT_VISION_INTENSITY_INC`, `NIGHT_VISION_INTENSITY_SET`
- New SimVars related to Night Vision have been added: `NIGHT VISION AVAILABLE`, `NIGHT VISION DISPLAYED`, `NIGHT VISION INTENSITY`
- New SimVar for Helicopters added: `VORTEX RING STATE PROTECTION ASSISTANCE IMPACT`

{{< /expand >}}



{{< expand title="SDK Release 1.5.6" >}}

<p class="fake-h3">SDK</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "fixed" >}}

The SDK samples installer now includes all the modelbehavior files. Some of them were missing. 

  

<p class="fake-h4">Tools</p>

{{< release-notes-tag "fixed" >}}

- Blender:
    - Fixed issue where the ambient occlusion texture's XML was not generated when 'Keep Original' was enabled.
    - Fixed UV1 and UV2 being swapped when exporting with 'Export in Background' disabled in Blender 3.6.
    - Fixed lights kelvin mode.


<p class="fake-h3">Documentation</p>

<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "added" >}}

- The flight_model.cfg has the following new parameters:
    - [[HELICOPTER]](../content-configuration/cfg-files/flight_model.cfg/#helicopter) - `vortex_ring_protection_assistance_nominal_planar_airspeed`, `vortex_ring_protection_assistance_engage_planar_airspeed`, `vortex_ring_protection_assistance_vertical_speed_limit`, `vortex_ring_protection_assistance_pid`,
- A new Note On Helicopter Vortex Ring Protection Assistance has been added to the flight model additional information page.

{{< /expand >}}



{{< expand title="SDK Release 1.5.4" >}}

<p class="fake-h3">DevMode</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "fixed" >}}

- Fixed CFD was sometimes not properly affecting surrounding vegetation.
- Fixed world not ready when returning from Aicraft Capture Tool.


<p class="fake-h4">Project Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed Sim Prop Container icon sometimes not rendering properly in package creation wizard.
- Fixed build error when a thumbs.db is in the package folder.

{{< release-notes-tag "added" >}}

- Added INPUT_PROFILE content-type to the project editor which can be use as a filter in the Marketplace.
- Added support for extra files to ModularSimObject asset group.


<p class="fake-h4">SimObject Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed editable_color from livery.cfg not correctly read if not entirely lowercase.
- Fixed fuel system engine index limited to 1 - 4.
- Fixed surface_angle in obj_ea1_surface in flightmodel incorrectly allowing indexed params.
- Fixed several non working conditions.
- Fixed parsing issue with last line of CFG file due to wrongly terminated string.
- Fixed a crash when editing the flight_model.cfg Performance Data if there is no approach rate / IAS.
- Fixed some param with alias not correctly read.
- Fixed Modular Graph layout error on undo.

{{< release-notes-tag "added" >}}

- Added "Reset points altitude" option to right-click context menu of Rectangle and Polygon based objects.
- Added an option to auto resync the aircraft on modifications.

{{< release-notes-tag "improved" >}}

- Reworked polygon properties UX.
- "Add point" action in right-click context now selects the added point for polygon based objects (polygons, aprons, painted lines, etc).


<p class="fake-h4">Tools</p>

{{< release-notes-tag "fixed" >}}

- Fixed a crash when opening the Aircraft Capture Tool.


<p class="fake-h4">Scenery Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed other points moving unexpectedly when moving a polygon point.
- Fix polygon geome override.

{{< release-notes-tag "added" >}}

- Support random meshes in vector placements.
- Added a delete command to remove projected meshes from airports.


<p class="fake-h4">Biome Editor</p>

{{< release-notes-tag "added" >}}

- Added a log window to show Build/Reload error in the editor.
- Added helpers to know if a biome will override an existing one.


<p class="fake-h3">SDK</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "fixed" >}}

- Fixed possible issue with normals/reflections when using incorrect metallic factor in a gltf.

{{< release-notes-tag "improved" >}}

- Relaxed restrictions on VFS overriding (Globally Overriden Base Sim Files): ModelBehaviorDefs need explicit overrides only for some subfolders instead of entire folder.


<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "fixed" >}}

- Fixed a crash in the cabin service system.
- Fixed an issue that caused the HUD to sometimes pick up values from the sim despite being updated each frame in the ModelBehaviors.
- Fixed missing effect of payload station weight changes on aircraft mass properties (weight, CG position, and moments of inertia) when using the "empty_inertia_tensor" parameter.
- Fixed a crash in the aircraft loading when the flight model cfg file was incomplete or contained incorrect data.
- Fixed OBJ_EA1_SURFACE calculation of airfoil surface normal (Z component)
- Fixed non latin character not rendered on company name.
- Fixed initialization of "modifier_position_scalar" & "modifier_surface_relative_position_scalar" in "[OBJ_EA1_\*]" sections of flight_model.cfg when not explicitely specified.
- Fixed brakes Wear and Tear incorrectly ignoring Aircraft Stress assistance option.

{{< release-notes-tag "added" >}}

- Added hud_show_radar_alt to add a small Radar Altitude gauge to the external HUD.
- Added structural_electrical_deice_rate cfg param to control the deice rate.
- Added support for advanced performance planning for turbojet/turbofan aircraft. Note that individual aircraft must be specifically configured to support this feature, and as such this feature will initially not be available for all turbojet/turbofan aircraft.
- Added checks for missing interaction file upon loading navigation graphs (this will help aircraft developers detecting mistakes that may break these interactions).


<p class="fake-h4">Tools</p>

{{< release-notes-tag "fixed" >}}

- Blender:
    - Fixed progress bar and panel tabs affected by undo/redo.
    - Fixed uv reorder breaking when there is uv layer named UV2.
    - Fixed export AO.
    - Fixed texture export errors for textures with special characters in their names.
    - Fixed textures being exported to the root of the GLTF folder even when a custom texture folder was defined.
    - Fixed merge node breaking with object instances.
    - Fixed anisotropic extension name.
    - Fixed export_as_submodel with remove_lod_prefix enabled.
    - Fixed "Merged Node" export option: collisions and lights are now preserved.
    - Fixed hidden collections not being exported.

{{< release-notes-tag "added" >}}

- Blender:
    - Added automatic ‘make relative' on save: external files (textures, links, libraries, etc.) are now saved with relative paths instead of absolute ones, preventing missing file issues when sharing .blend files. This behavior can be disabled in the add-on preferences.
    - Added reparent children to their parent when gltf is exported as submodel.
    - Added source radius and inner angle preview for Advanced and SkyPortal lights.

{{< release-notes-tag "improved" >}}

- Blender:
    - Reworked material emissive scale to improve viewport visualization and make value ranges easier to interpret. See EMISSIVE_PREV.md for details on setting up the viewport for correct preview.
    - Changed default invisible material color to transparent blue.
    - Export paths are now automatically converted to relative when edited by the user or upon scene save.
    - "nla_tracks" export animation mode set by default.
    - Baked material animation frames when export.
- 3DS Max: Removed useless projeted decal lights.


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "fixed" >}}

- Fixed an issue preventing passive aircrafts' lights from being shut down properly using Aircraft Set/Toggle Light events.
- WASM:
    - Fixed WASM gauges not always fully loaded prior to starting the flight (could generate odd aircraft behavior on start and/or restart).
    - Fixed sharing violation when reading a directory where a file is opened for writing.
    - Fixed key event handler not called (could alter interactions in various aircraft or modules).
    - Fixed wasm gauge not loading properly after a restart.
    - Fixed access to fsUtilsGetStrCRC in standalone module.
- SimConnect:
    - Fixed calculation of AI aircraft starting position when spawned EnRoute (fixes aircraft randomly not spawning when using SimConnect_AICreateEnrouteATCAircraft).
    - Fixed SimConnect C\# enum name.
- SimVars: Fixed usage of \`SIMVAR_ANIMATION_DELTA_TIME\` not returning the correct value in material and vfx behavior updates.

{{< release-notes-tag "added" >}}

- Added possibility to target other object in Vars API.


<p class="fake-h4">Samples</p>

{{< release-notes-tag "added" >}}

- Added new Cabri G2 blend scenes.
- Added Blend scene for Bear Sample.


<p class="fake-h3">Documentation</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "fixed" >}}

- Broken footer buttons fixed.


<p class="fake-h4">DevMode</p>

{{< release-notes-tag "improved" >}}

- The Model Thumbnail Baker tool description has been updated to reflect the new ability to use it to preview modellib asset groups.


<p class="fake-h4">SDK Tools</p>

{{< release-notes-tag "added" >}}

- The Decal Per Component Blend Factors (Decal) documentation has been updated with information about the new channel mask options for both Blender and 3DS Max.
- Documentation for the SimObject Spawner has been added.


<p class="fake-h4">Models And Textures</p>

{{< release-notes-tag "added" >}}

- Blender:
    - New Transform Properties added.
    - New section detailing animated materials for blender has been added here: Animated Materials
- 3DS Max: Object Properties documented.

{{< release-notes-tag "improved" >}}

- The Model Exporting page has been updated with information about the need to reset root node transforms on export.
- The Model Exporting page has been updated with information for Blender about the use of negative keyframes in animations.


<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "added" >}}

- The systems.cfg page has been updated with the following new parameters:
    - [DEICE_SYSTEM] - `structural_electrical_deice_rate`
- The cockpit.cfg page has been updated with the following new parameters:
    - [MISC] - `hud_show_radar_alt`, `hud_engine_throttle_var`, `hud_engine_throttle_var_indexed`
- The FLT File Properties page has been updated with the following parameters:
    - [Systems.N] - `DecisionHeight`
- The flight_performance.cfg page has been updated with the following new parameters:
    - [ENGINE_PERFORMANCE] - `fuel_density_table`
    - [CLIMB_PERFORMANCE.N] - `fuel_type_idx`
    - [CRUISE_PERFORMANCE.N] - `fuel_type_idx`
    - [DESCENT_PERFORMANCE.N] - `fuel_type_idx`
    - [AIRCRAFT_LIMITS] - `cruise_max_altitude_table_by_weight_and_ISA_dev`,
- A new page has been added to help with the configuration and use of the `flight_performance.cfg` file: flight_performance.cfg - Setup
- The following new sections have been added to the **engine.cfg - Additional Information** page:
    - Note On The Fuel Flow PID
    - Note On Computing Commanded Ne
- A the following section has been added to the **flight_model.cfg - Additional Information** page: Note On The Stall Protection System
- A new Note On Services has been added related to the pages with information for airliners wishing to participate in commercial flight missions.

{{< release-notes-tag "improved" >}}

- The stall protection parameters added in the [CONTROL_SYSTEM] section of the `flight_model.cfg` have been updated with descriptions.
- The Pre-Mission Checks section of the **General Career information** page has been updated to include information on how the `ui_typerole` parameter affects the starting orientation of the aircraft.
- Some minor updates have been made to the EFB Flight Plan XML (PLN File) Properties page to bring it in line with the current EFB setup.
- The parameter `cruise_Mach` in the [CRUISE_PERFORMANCE.N] section of the `flight_performance.cfg` has been renamed to simply `Mach`.
- The Modular SimObject Merging page has had some minor updates to clarify information.
- The SimVar TURB ENG COMMANDED N1 has had it's information expanded and clarified.
- The following engines.cfg parameters have had their descriptions updated and expanded: `use_commanded_Ne_table`, `JET_density_on_FF_table`, `density_on_torque_table`, `density_on_FF_table`, `mach_0_corrected_commanded_ne_table`, `mach_hi_corrected_commanded_ne_table`, `afterburner_on_thrust_table`

{{< release-notes-tag "fixed" >}}

- Fixed a minor issue with the explanation given in the Note On Propeller Pitch And Throttle.


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "added" >}}

- The following SimVars have been added:
    - Aircraft System Variables - `STALL PROTECTION SYSTEM YOKE SHAKER INTENSITY`
- The following Key Event pages have new keys:
    - Aircraft Autopilot/Flight Assist Events - `AP_ALT_CURRENT_ALT_SET`, `AP_HDG_CURRENT_HDG_SET`
    - Aircraft Engine Events - `AXIS_CONDITION_LEVER_SET`, `PLASMA_OFF`, `PLASMA_ON`, `PLASMA_TOGGLE`, `PLASMA_SET`, `THROTTLE_DETENT_NEXT`, `THROTTLE_DETENT_PREV`, `THROTTLE_INCR_SMALL`, `THROTTLE_RANGE_DECR`, `THROTTLE_RANGE_INCR`
    - Aircraft Misc. Events - `COVER_ON`, `TOGGLE_ALL_AIRCRAFT_DOORS`
    - Aircraft Fuel System Events - `ELECT_FUEL_PUMP_SET`
    - Aircraft General Systems Events - `GRAPPLE_HOOK_OFF`, `GRAPPLE_HOOK_ON`, `GRAPPLE_HOOK_TOGGLE`, `GRAPPLE_HOOK_SET`, `LEAD_POLE_OFF`, `LEAD_POLE_ON`, `LEAD_POLE_TOGGLE`, `LEAD_POLE_SET`
    - Helicopter Specific Events - `ROTOR_BRAKE_LOCK_SET`, `HELICOPTER_FORCE_TRIM_RELEASE_BUTTON_SET`, `QUICK_TRIM`
    - Miscellaneous Events - `ORNI_BOOST_SET`, `ORNI_DIVE_MODE_OFF`, `ORNI_DIVE_MODE_ON`, `ORNI_DIVE_MODE_TOGGLE`, `ORNI_GLIDE_MODE_OFF`, `ORNI_GLIDE_MODE_ON`, `ORNI_GLIDE_MODE_TOGGLE`, `ORNI_WINGS_BRAKE_SET`, `WING_FOLD_OFF`, `WING_FOLD_ON`, `WING_FOLD_SET`
- The page on the Panel XML Properties has been updated to include the `AutoMerge` attribute for the `<PlaneHTMLConfig>` tag.

{{< release-notes-tag "fixed" >}}

- Fixed numerous issues with pages in the Token Variables section (broken/malformed links, pages not included, pages not in ToC).

{{< release-notes-tag "improved" >}}

- The definition for `ui_typerole` has been updated with additional roles.


<p class="fake-h4">Samples And Tutorials</p>

{{< release-notes-tag "added" >}}

- A new section has been added to the aircraft setup tutorial to explain the correct configuration of the simulation UI: Aircraft Classification Details


{{< /expand >}}



{{< expand title="SDK Release 1.5.3" >}}

<p class="fake-h3">DevMode</p>

<p class="fake-h4">Project Editor</p>

{{< release-notes-tag "added" >}}

- Added option to use 360\*240px (MSFS 2024 MyLibrary format) images in the ContentInfo as thumbnails for Community content.


<p class="fake-h4">SimObject Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed unable to remove some optional parameters from the UI.


<p class="fake-h4">Scenery Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed instancing snapped to the ground too slowly.

{{< release-notes-tag "added" >}}

- Added forced altitude for VectorPlacement.


<p class="fake-h3">SDK</p>

<p class="fake-h4">Models And Textures</p>

{{< release-notes-tag "fixed" >}}

- Fixed material emissive not being reset when turning lights off.


<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "fixed" >}}

- Fixed an issue where the legacy hydraulic system did not work, which caused problems such as missing wheel braking on several aircraft.
- Fixed an issue preventing AI Rotorcraft's rotors from properly operating.


<p class="fake-h4">Tools</p>

{{< release-notes-tag "added" >}}

- Blender: Added warning when exporting presets with zero layers selected.

{{< release-notes-tag "improved" >}}

- Blender: Cleaned up shader nodes dedicated to export that remains in the original material.


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "fixed" >}}

- WASM:
    - Fixed call to execute_calculator_code causing ctd has been fixed.  
        - Fixed WASI fd_write function returning an error when unable to write the requested number of bytes.  
        - Fixed WASI fd_pwrite function missing conversion of iovs & nwritten pointers from linear memory to host memory (could lead to crashes).  
        - Fixed fsVarsEnvironmentVarGet in Vars API.


<p class="fake-h3">Documentation</p>

<p class="fake-h4">DevMode</p>

{{< release-notes-tag "added" >}}

- The SimObject editor Nodes window has been added to the docs.
- The SimObject editor Curve Editor window has been added to the docs.
- The SimObject editor menu option Auto Resync On Changes has been documented.
- The new VFX Editor node GetCameraPosition has been added to the documentation.
- The Biomes Editor pages have been updated to reflect additions to the tool:
    - The "Is Default" flag in the inspector.  
        - The Mounted Biomes drop-down list.  
        - The Biome Load Report window.

{{< release-notes-tag "fixed" >}}

- All the "input" documentation has been removed pending a re-write.

{{< release-notes-tag "improved" >}}

- Minor update to The Career Tool page to include information on using the full flow testing profile.
- Minor update to the Content Creator Testing Tool to link to the full flow testing profile information.
- The VectorPlacement Objects page has been updated with information on the **Independent Object** flag.


<p class="fake-h4">Models And Textures</p>

{{< release-notes-tag "added" >}}

- The Propellers, Turbines And Blades has been fixed to show correct information related to setting up turbine fans.
- First pass of the page detailing Edited Liveries has been added.


<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "added" >}}

- The empty_inertia_tensor parameter in the flight_model.cfg has been re-exposed for use with this update.
- The Modular Fuel System documentation has the following updates:
    - The Tank.N hashmap has a new key: `BlocksPressure`.
    - The Pump.N hashmap has a new key: `PctPressurePerPump`.
- Additions to [CONTACT_POINTS]:
    - The parameter `point_order_independent_suspension_solver` has been added to the parameters list.
    - The point.N definition in the contact points section has a new hashmap key documented: `WearAndTearGroup`
- Additions to [AERODYNAMICS]: `presspt_fwd_Spoilers`, `buffetingscalar`,
- New section for the [CONTROL_SYSTEM] added with the following parameters (WIP): `SPS_flight_control_shaker_switch_time`, `SPS_flight_control_shaker_IAS_off`, `SPS_flight_control_shaker_IAS_on`, `SPS_flight_control_shaker_AoA_off`, `SPS_flight_control_shaker_AoA_on`, `SPS_flight_control_shaker_amplitude`, `SPS_flight_control_shaker_frequence`, `SPS_flight_control_limiter_IAS_on`, `SPS_flight_control_limiter_IAS_off`, `SPS_flight_control_limiter_push_rate`, `SPS_flight_control_limiter_aoa`, `SPS_flight_control_limiter_position`
- The engines.cfg - Additional Information page has a new section: Note On Time Constants And Tuning Constants.

{{< release-notes-tag "improved" >}}

- The flight_model.cfg page has updated descriptions for the following parameters: `modifier[n]`, `modifier_angle_scalar[n]`, `modifier_position_scalar[n]`, `modifier_surface_relative_position_scalar[n]`,
- The [ELECTRICAL] system version number information has been updated to include version 2.1 of the system.

{{< release-notes-tag "fixed" >}}

- A broken link on the Visual Effects Templates page has been fixed.
- Minor issue with the explanation given for `ASOBO_Get_Enum_Parameters_Helper` fixed on the Interaction Configurations page.


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "added" >}}

- Missing environment functions missing from the Vars API have been added.

{{< release-notes-tag "fixed" >}}

- Fixed small typo with `parameterString` on the Creating WASM Systems page.
- Broken images on the Creating A WASM Project page have been fixed.

{{< release-notes-tag "improved" >}}

- The following SimVar descriptions have been updated: `AILERON LEFT DEFLECTION`, `AILERON LEFT DEFLECTION`, `AILERON POSITION`, `RUDDER DEFLECTION`, `RUDDER POSITION`, `ELEVATOR DEFLECTION`, `ELEVATOR POSITION`


<p class="fake-h4">Samples And Tutorials</p>

{{< release-notes-tag "improved" >}}

- The Basic Aerodynamics page has been updated with information on the CFD and modern flight model, as well as updated images.
- The Geometry section related to tuning the flight model has been updated and split into additional pages to help cover the MSFS2024 physics objects with additional geometry features.
- The Wheels And Contact Points page has been updated with additional information related to the modern ground contact model and other changes specific to MSFS2024.
- The Weight And Balance page has been updated with improved information specific to MSFS2024.

{{< /expand >}}



{{< expand title="SDK Release 1.5.2" >}}

<p class="fake-h3">DevMode</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "fixed" >}}

- Fixing some inconsistencies and unit issues in the Pitch debug for aircraft.
- Fixed CFD debug not properly affecting surrounding vegetation.
- Fixed CFD not compatible with inverted flight.
- Fixed a crash in the CFD simulation.

{{< release-notes-tag "added" >}}

- Added an error message if a glTF skeleton references nodes with duplicated ASOBO_unique_id as this might break some animations. Only the message was added, the behavior is not impacted.
- Added Community2024 folder, used to allow sharing of Community packages with Microsoft Flight Simulator 2020, without making this iteration load 2024 packages.

{{< release-notes-tag "improved" >}}

- Improved debug of aerodynamic forces applied onto surfaces to make very small forces more visible.
- Improved aircraft weight debug page to display weight stations.
- Updated the "Debug Aircraft Weight" tool to support the optional full inertia tensor and to display the status of associated rotational dynamics fix.
- Renamed incorrectly named root parameter display in pitch flight model debug window.


<p class="fake-h4">Project Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed error message not displayed when building gltf.
- Fixed not reusing the original flt file when reloading the aircraft after a build.
- Fixed discovery, landing and tutorial mission templates.
- Fixed modifications to package definition not triggering a regeneration of manifest.json.

{{< release-notes-tag "added" >}}

- Added bushtrip mission template.
- Added copy position button to airport creation wizard.

{{< release-notes-tag "improved" >}}

- Removed useless generate icao button from airport creation wizard.


<p class="fake-h4">SimObject Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed parsing issue with last line of CFG file due to wrongly terminated string.
- Fixed a crash when editing the flight_model.cfg Performance Data if there is no approach rate / IAS.
- Fixed the attachment root field now to correctly list the local attachments.
- Fixed some parameters with alias not being correctly read.
- Fixed possible duplication of node in the modular graph.

{{< release-notes-tag "added" >}}

- Added button to open the flight performance debug to the corresponding tab in the SimObject Editor.
- Added a curve editor for lists and (legacy) 2d array parameters.

{{< release-notes-tag "improved" >}}

- Hidden parameters now shown in ctrl+f results.


<p class="fake-h4">Scenery Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed potential crash when loading a polygon based element (polygons, vector placements, aprons, etc) with no points.
- Fixed building exclusion from airport taxiways.
- Fixed airport light objects randomly not showing or showing when there is no runway.
- Fixed vertex color gradient for projected meshes.
- Fixed issue with mipstreaming on projected meshes.
- Fixed other points moving unexpectedly when moving a polygon point.
- Fixed beacon lights wrong position, incorrect intensity, and not being visible in the editor.
- Fixed invalid scenery files automatically converted to SPC.
- Fixed invalid terraforming profile issues.
- Fixed missing "Optimizable" flag for trees in SimpropContainer.
- Fixed crash when using one-click placing with no object selected.
- Fixed "remove duplicate objects" option.
- Fixed "Create Legacy Hierarchy" not opening the confirm popup anymore.
- Fixed snap to normal taking into account trees.
- Fixed sub edition point selection after splitting an edge, new point is now selected.
- Fixed projected mesh child object appearing after a delete and undo when it should stay hidden.
- Fixed independent projected meshes disappearing when updating nearby airport.

{{< release-notes-tag "added" >}}

- Added a delete command to remove projected meshes from airports.
- Added tooltips for "Optimizable" and "Only in edition" flag.
- Added texture UV offset for polygons.
- Added fallback name for Visual Effects with no tag using file name.
- Added conversion option in aprons context menu to convert to polygons (UV are shifted until we implement corresponding options in Polygon object).
- Added option to convert Projected meshes and vector placements to independent (and vice-versa) in their respective context menu (right-click).

{{< release-notes-tag "improved" >}}

- Reworked polygon properties UX.
- "Add point" action in right-click context now selects the added point for polygon based objects (polygons, aprons, painted lines, etc).
- Added "Reset points altitude" option to right-click context menu of Rectangle and Polygon based objects.
- Increased draw distance of beacon lights.
- Reduce the minimal distance allowed between profile points on the terraforming profile editor.
- Made the object list alphabetically ordered without case-sensitivity.


<p class="fake-h4">Material Editor</p>

{{< release-notes-tag "added" >}}

- Added the possibility to filter per material GUID.


<p class="fake-h4">VFX Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed crash when duplicating a BezierCurve node (and other edits being impossible) after editing a point's value from the Current point position field under the Current Point Settings header.


<p class="fake-h4">Biome Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed possible crash when reloading the biomes.
- Fixed issue with drag and drop in lists.

{{< release-notes-tag "added" >}}

- Added biome loading error messages in the console.
- Added helpers to know if a biome will override an existing one.

{{< release-notes-tag "improved" >}}

- Improved Editor default values and error messages to prevent building wrong biome xml files.
- Improved field names.


<p class="fake-h3">SDK</p>

<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "fixed" >}}

- Fixed panel.cfg registration font and style different than 2020.
- Fixed `IVar` registration in compiled behaviors.
- Fixed some emissive flashing issues when disabling an interaction highlight.
- Fixed FLT_SIM category inside livery.cfg to still be considered instead of only the (correct) FLT_SIM.0.
- Fixed crash in navigation graph system.
- Fixed partially uneffective incidence parameter for new htail objects.
- Fixed partially missing usage of sweep parameter in new airplane htail object surfaces.
- Fixed Vpainting in panel.cfg using the font color transparent.
- Fixed a crash on some the electrical system var get when the passed index was greater than int max.
- Fixed livery.cfg panel dynamic parameters not overriding default values defined in common/preset panel.cfg.

{{< release-notes-tag "added" >}}

- Added support for new parameter `external_camera_maximum_pitch` to the camera.cfg to tune the 3rd person camera pitch limits.
- Added `fuel_flow_scalar_idle` to the engine.cfg to more easily control the effect of the throttle on the fuel flow.
- Added range limitations for two CFD variables in the cfg that could have caused crashes
    - CFD_VoxelNbVoxels now limited to 8 - 40.
    - CFD_VoxelSizeScale now limited to 0.1 to 10.
- Re-added Added support for a full inertia tensor used in aircraft rotation modeling via the new `empty_inertia_tensor` parameter in the  
    [WEIGHT_AND_BALANCE] section of flight_model.cfg file. When this parameter is specified, it also enables the following fixes:
    - Fixed incorrect effect of payload stations on MOI changes. Note: if you previously compensated for this by increasing the empty aircraft MOIs, re-adjustment may now be necessary.
    - Fixed incorrect MOI recalculation during in-simulation weight changes.
    - Fixed several issues in rotational dynamics modeling: the laws of conservation and evolution of angular momentum and rotational kinetic energy are now satisfied with high accuracy.
    - Increased limits for angular velocity (to 10 revs/sec) and angular acceleration (to 100 revs/sec^2). These limits only safeguard against crashes from incorrect flight model settings and are not expected to trigger in normal gameplay.


<p class="fake-h4">Tools</p>

{{< release-notes-tag "fixed" >}}

- InputApp: Fixed some unusable actions being listed in control settings.
- ModellibThumbnailBaker: Added an option to bake all thumbnails of a package.
- Blender:
    - Fixed "export_as_submodel" with "remove_lod_prefix" enabled.
    - Fixed not exporting "Flip back face normal" when "double sided" is not enabled.
    - Fixed preset settings affected to presets when the settings of the group has changed.
    - Fixed light temperature not working.
    - Fixed reparented objects that lost their parent during join.

{{< release-notes-tag "added" >}}

- Blender:
    - Added reset nodes and reset roots origin in parameters.
    - Added Blend scene for Bear Sample.
    - Added light temperature preview in UI.
- BglExplorer: Display runway material GUID added.
- VFX Projector: Added gltf renormalization in VFS Projector (Gltf and bin are now transformed in gltf projector to be usable).

{{< release-notes-tag "improved" >}}

- Blender:
    - Updated DA62 blend scenes.
    - "nla_tracks" export animation mode set by default.
    - Material animation frames are baked when exported.
    - Set default detail OMR color to white.
    - Instantiated collections can now be exported.
- 3DS Max:
    - Removed "Flux" intensity for lights.
    - Moved "Flare Only" to Flare category.


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "fixed" >}}

- JS: Fixed coherent call "SET_MAP_PARAMS" to allow LatLongAlt and LatLong argument.
- WASM:
    - Fixed visual studio detection of C++ version in Wasm projects.
    - Fixed access to fsUtilsGetStrCRC in standalone module.
- Flow API: Fixed FsFlowUnregisterAll function that didn't handle callbacks being registered if it was called in the same frame as a fsFlowRegister.
- Systems API: Fixed reload wasm systems on build package.
- SimVars
    - Fixed an issue that caused the non-indexed version of the PITOT HEAT SWITCH simvar to sometimes return true even though all pitot heat switches are set to off.
    - Fixed usage of ANIMATION DELTA TIME not returning the correct value in material and vfx behavior updates.
    - Fixed PNEUMATICS PACK TEMPERATURE simvar always returning 0.

{{< release-notes-tag "added" >}}

- WASM: Added better error handling when WASM module fails to load which will avoid random crashes.
- Vars API:
    - Added Support for `BVar`.
    - Added possibility to target objects other than the user object.
    - Added in callback to inform removing of I, O, and Z Var, as well reseting of `LVar`.
    - Added support for struct variables and strings.
    - Added fsVarsGetLVarName in Vars API to replace get_name_of_named_variable.
- SimVars:
    - Added a new simvar PYLON HEIGHT
    - Added a new simvar PYLON COLLISION POSITION
- SimConnect: Added possibility to use name index for data definition.

{{< release-notes-tag "improved" >}}

- Renamed function in vars API to use A, E, L, I, O, Z.


<p class="fake-h4">Samples</p>

{{< release-notes-tag "fixed" >}}

- Fixed sound in Wasm Sound Sample - wwise events were missing in sound.xml.

{{< release-notes-tag "added" >}}

- Added Blend scene to jetway sample.


<p class="fake-h3">Documentation</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "fixed" >}}

- The Vars API has been correctly linked in the Table Of Contents.


<p class="fake-h4">DevMode</p>

{{< release-notes-tag "added" >}}

- A page describing the operation of the Virtual File System has been added.
- The The Aircraft Capture Tool has a new section outlining how to setup the tool to correctly create EFB silhouette images.
- The SimObject Editor and Behaviors Debug sections have been updated with links to the following new systems debug pages:
    - ECS / Pneumatic System Debug
    - Electrical System Debug
    - Fuel System Debug
    - Hydraulic System Debug
- The new FLT Files debug window has been added into the documentation.
- The Channel Display page now has information on the **Clearcoat** display option.

{{< release-notes-tag "fixed" >}}

- The section describing the **Ambient Occlusion** view of the Channel Display options has had the image fixed to show the correct colours.
- The section describing the **Velocity** view of the Channel Display options has been restored.

{{< release-notes-tag "improved" >}}

- The Model Thumbnail Baker section has been updated to reflect the current state of the tool.


<p class="fake-h4">Models And Textures</p>

{{< release-notes-tag "added" >}}

- The The 3DS Max Plugin page has a new section detailing the RMB menu options.
- A section explaining the KTX2 and KTX2P format has been added: KTX2 And KTX2P Files
- New page explaining the Simplygon Utilities has been added.

{{< release-notes-tag "improved" >}}

- The page on Thumbnails has been updated with info about the EFB shape image.
- The entire section related to The 3DS Max Plugin has been updated and improved.
- Minor updates have been made to The Blender Plugins page to clarify install information.
- The page on Model Exporting has been updated to cover both exporters (3DS Max and Blender) and is now focused on modular aircraft.


<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "added" >}}

- The pages related to hydraulics have been updated to include:
    - Custom Actuators on the Hydraulics System Setup Information.
    - new actuator hashmap key (`AssistancePct`) added to definition.
    - new `WearAndTearCollision` key added to line, actuator, accumulator, and valve definitions.
- The `<BehaviourState>` element has a new attribute documented: `UseMassInstancing`.
- The model behaviors pages have a new section (cross-referenced where appropriate) for Behavior Version Improvements, outlining the differences between the version 1 and version 2 of the behavior elements.
- The new `external_camera_maximum_pitch` parameter has been added into the cameras.cfg page.
- The new `fuel_flow_scalar_idle` parameter has been added into the engines.cfg page.

{{< release-notes-tag "improved" >}}

- The Instrument Attachments page has been updated with more attachments.
- All the various Key Events pages have been updated with expanded information and Key Names.
- The FX_WaterDrop_LowSpeed template information has been updated.
- The `SIMCONNECT_SIMOBJECT_TYPE` page has been updated with the correct enum values.
- The LOC Files (Localization) page has been updated with correct information about which languages are supported.
- The SimObject Model XML and CFG Setup page has been updated to be in line with the modular aircraft approach.
- A minor clarification has been made to the Aerial Advertising page in the section on how power is calculated.
- The [SimVarForSpawningInTheAir] section of the FLT File has been flagged as obsolete.

{{< release-notes-tag "fixed" >}}

- Multiple links on the CFG Parameter Index which were malformed have been fixed.


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "added" >}}

- The following SimVars have been added to the docs:
    - Aircraft Radio Navigation Variables: `ADF DISTANCE`, `ATC ARRIVAL AIRPORT HAS ATIS`, `ATC ARRIVAL AIRPORT IS TOWERED`, `ATC CLEARED OVERFLIGHT CTR`, `ATC DEPARTURE AIRPORT IS TOWERED`, `ATC DESIGNATED GATE ARRIVAL`, `ATC DESIGNATED GATE DEPARTURE`, `ATC DESIGNATED RUNWAY LANDING`, `ATC DESIGNATED RUNWAY TAKEOFF`, `ATC FIRST VFR ARRIVAL PATTERN WAYPOINT NAME`, `ATC FP ARRIVAL TRAFFIC PATTERN IS LEFTHANDED`, `ATC FP DEPARTURE TRAFFIC PATTERN IS LEFTHANDED`, `ATC FUTURE AGENT DISTANCE`, `ATC FUTURE AGENT FREQUENCY`, `ATC FUTURE AGENT NAME`, `ATC FUTURE AGENT TYPE`, `ATC MISSION FP IFR`, `ATC NAME`, `ATC NEXT WAYPOINT NAME`, `ATC OVERFLYING AIRPORT UNANNOUNCED`
    - Balloon / Airship Variables: `BALLOON AUTO FILL ACTIVE`, `BURNER FUEL FLOW RATE`
    - Aircraft Flight Model Variables: `INTERACTIVE POINT BANK EX1`, `INTERACTIVE POINT GOAL EX1`, `INTERACTIVE POINT HEADING EX1`, `INTERACTIVE POINT JETWAY LEFT BEND EX1`, `INTERACTIVE POINT JETWAY LEFT DEPLOYMENT EX1`, `INTERACTIVE POINT JETWAY RIGHT BEND EX1`, `INTERACTIVE POINT JETWAY RIGHT DEPLOYMENT EX1`, `INTERACTIVE POINT JETWAY TOP HORIZONTAL EX1`, `INTERACTIVE POINT JETWAY TOP VERTICAL EX1`, `INTERACTIVE POINT OPEN EX1`, `INTERACTIVE POINT PITCH EX1`, `INTERACTIVE POINT POSX EX1`, `INTERACTIVE POINT POSY EX1`, `INTERACTIVE POINT POSZ EX1`, `INTERACTIVE POINT TYPE EX1`
    - Miscellaneous Variables: `PYLON HEIGHT`, `PYLON COLLISION POSITION`
- The following new Key event IDs have been added to the docs:
    - Aircraft Autopilot/Flight Assist Events: `AP_RPM_SLOT_INDEX_SET`, `AP_SPEED_SLOT_INDEX_SET`, `AP_VS_SLOT_INDEX_SET`
    - Miscellaneous Events: `AXIS_SENSOR_TOGGLE`, `QUICK_TRIM`, `DEMO_RECORD_MESSAGE`, `DEBUG_NUMPAD_0 - 9`, `DEBUG_DOWN`, `DEBUG_ENTER`, `DEBUG_LALT`, `DEBUG_LCTRL`, `DEBUG_LEFT`, `DEBUG_LSHIFT`, `DEBUG_MENU`, `DEBUG_PAUSE`, `DEBUG_RALT`, `DEBUG_RCTRL`, `DEBUG_RIGHT`, `DEBUG_RSHIFT`, `DEBUG_TAB`, `DEBUG_UP`, `AXIS_PC_FPV_ROTATION_X`, `AXIS_PC_FPV_ROTATION_Y`, `AXIS_PC_MOVE_X`, `AXIS_PC_MOVE_Z`, `PC_CROUCH_TOGGLE`, `PC_FPV_LOOK_DOWN`, `PC_FPV_LOOK_DOWN_LEFT`, `PC_FPV_LOOK_DOWN_RIGHT`, `PC_FPV_LOOK_LEFT`, `PC_FPV_LOOK_RIGHT`, `PC_FPV_LOOK_UP`, `PC_FPV_LOOK_UP_LEFT`, `PC_FPV_LOOK_UP_RIGHT`, `PC_MOVE_BACKWARD`, `PC_MOVE_BACKWARD_LEFT`, `PC_MOVE_BACKWARD_RIGHT`, `PC_MOVE_FORWARD`, `PC_MOVE_BACKWARD_LEFT`, `PC_MOVE_FORWARD_RIGHT`, `PC_MOVE_FORWARD_LEFT`, `PC_MOVE_LEFT`, `PC_MOVE_RIGHT`, `PC_RUN_SET`, `3RD_PARTY_WINDOW_OPEN_PRIMARY`, `3RD_PARTY_WINDOW_OPEN_SECONDARY`, `3RD_PARTY_WINDOW_MOVE_DOWN`, `3RD_PARTY_WINDOW_MOVE_UP`, `3RD_PARTY_WINDOW_VALIDATE`, `MENU_RENO_KICK_PLAYER`, `MENU_SR_EFB_TOGGLE`
    - Aircraft Flight Control Events: `AXIS_SENSOR_ELEVATOR_SET`, `AXIS_SENSOR_AILERONS_SET`, `AXIS_SENSOR_RUDDER_SET`
    - Aircraft General Systems Events: `FIREFIGHTING_SCOOP_DOORS`, `HYDRAULIC_ACTUATOR_ACTIVE_OFF`, `HYDRAULIC_ACTUATOR_ACTIVE_ON`, `HYDRAULIC_ACTUATOR_ACTIVE_SET`, `HYDRAULIC_ACTUATOR_ACTIVE_TOGGLE`, `PARKING_BRAKES_OFF`, `PARKING_BRAKES_ON`
    - Aircraft Misc. Events: `INTERACTION_UNLOCK`
    - Helicopter Specific Events: `HELICOPTER_FORCE_TRIM_RELEASE_BUTTON_SET`

{{< release-notes-tag "fixed" >}}

- The Slings and Hoists key events are no longer flagged as legacy.
- The `ROTOR_BRAKE_SET` event has been removed as it does not exist in the simulation code (use `AXIS_ROTOR_BRAKE_SET` instead).
- The `VERTICAL_SPEED_SET` event has been removed as it does not exist in the simulation code (use `AXIS_VERTICAL_SPEED_SET` instead).
- Some of the Miscellaneous Variables did not specify that they used the *camera* position for their data acquisition, this has been fixed.

{{< release-notes-tag "improved" >}}

- The SimVar `HYDRAULIC ACTUATOR ACTIVE` now mentions **custom** actuator types.
- Building An Aircraft Package has been updated to reflect the current methods used in the simulation (ie: modular aircraft).

{{< /expand >}}



{{< expand title="SDK Release 1.4.5" >}}

<p class="fake-h3">SDK changes</p>

<p class="fake-h4">Tools</p>

{{< release-notes-tag "fixed" >}}

- Blender: Fixed "export_position"


<p class="fake-h4">Samples</p>

{{< release-notes-tag "fixed" >}}

- Changed extension of bear sample's scene from .max2022 to .max

{{< /expand >}}



{{< expand title="SDK Release 1.4.4" >}}

<p class="fake-h3">Developer mode changes</p>

<p class="fake-h4">Project Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed thumbnail selection in Package Inspector. 


<p class="fake-h3">SDK changes</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "improved" >}}

- The LOD curve (max vertex count vs vertical screen size) has been improved by splitting it into two parts: linear from 0% to 100%, parabolic above 100%. This means more vertices than before for sizes up to 100% and the same amount for above 100%. The aim here is to help developers create less detailed LODs by letting them use more vertices than before. This new LOD curve is activated as the default one, and a debug option is available from the DevMode Debug menu to switch to the old curve values.


<p class="fake-h4">SimObject Editor</p>

{{< release-notes-tag "added" >}}

- Added support for the new [AIRCRAFT_LIMITS] section in the "flight_performance.cfg" file. The section contains a duplicate of the "cruise_max_altitude_table_by_weight_and_ISA_dev" table currently.
- Added new "max_cruise_alt" parameter in the [REFERENCE SPEEDS] section of the "flight_model.cfg" file.

<p class="fake-h4"> Tools</p>

{{< release-notes-tag "added" >}}

- Blender: Added technical guidelines about UV2 and Vertex Color attributes to markdown docs.
- Blender: Added markdown doc for export settings.

{{< release-notes-tag "improved" >}}

- Blender: Markdown docs have updated presets screenshots.
- Blender: Updated export settings tooltips.


<p class="fake-h4">Programming</p>

{{< release-notes-tag "fixed" >}}

- Fixed wrongly filtering custom F:KeyEvent upon executing RPN (was preventing events creating through SimConnect from being triggered)

{{< release-notes-tag "added" >}}

- Added EX1 variants of all INTERACTIVE POINTS SimVars, which will only be processed if the index or name parameter is valid.


<p class="fake-h3">Documentation</p>

<p class="fake-h4">DevMode</p>

{{< release-notes-tag "added" >}}

- A new page has been added to explain The Modular SimObject Graph View.
- The SimProp Container and Scenery Editor documentation has been updated with the following new pages:
    - ProceduralInstance Objects
    - Light Objects
- The The Project Inspector page has been updated with information on the **Globally Overriden Base Sim Files** tab.
- The Ruler Tool has been given a dedicated page that explains the new **polyline** functionality as well as how to use it.
- The Behavior Viewer page has been completed.

{{< release-notes-tag "fixed" >}}

- The SimProp Container object page has been updated with correct information about the currently available objects for use inside them.

{{< release-notes-tag "improved" >}}

- The objects that can be added into a SimProp Container have been more clearly identified on their respective pages, and also now include SimProp Container specific information (Scenery Objects, SimObj Objects, Tree Objects).


<p class="fake-h4">SDK Tools</p>

{{< release-notes-tag "added" >}}

- A new page has been added outlining the VS Code Extension that is now available with the SDK.
- A section has been added for the SDK LodProcessingPresets.


<p class="fake-h4">Models And Textures</p>

{{< release-notes-tag "added" >}}

- Dedicated page for the Wiper Mask Generator Tool added to cover both 3Ds Max and Blender implementations.
- The page on the Blender Plugin Properties has new sections related to UV setup and Vertex Colour setup.

{{< release-notes-tag "improved" >}}

- The page related to the The Blender Exporter has been updated to reflect the current state of the plugin.
- The page related to the Blender Plugin Properties has been updated to reflect the current state of the plugin.
- The Using Simplygon To Generate LODs page now references the use of LodProcessingPresets.


<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "added" >}}

- A new project schema has been added to better illustrate the Modular SimObject Project Structure.
- A new page with Fuel System Examples has been added to help with the modular fuel system setup.
- The [REFERENCE SPEEDS] section of the flight_model.cfg has the following new parameter: `max_cruise_alt`
- The Modular Hydraulics System Information has the following new parameters:
    - Pump.N - `LiquidCapacity`
    - Actuator.N - `LiquidCapacity`, `LiquidConsumption`
- The `<ServiceState>` tag of the Navigation Services XML Properties has a new attribute documented: `EFBDefaultStateBoarding`
- The Navigation Services XML Properties page has the following new content:
    - The `<ServiceState>` tag has the following new attributes documented: `EFBEnabled`, `EFBDefaultStateBoarding`, `EFBPostBoarding`
    - The `<MoveToGraphNode>` tag has a new attribute documented: `NodeTagsToExclude`
    - The `<Sequence>` tag has a new attribute documented: `RecomputeInteractiveObjectsLinksAction`
    - The `<Sequence>` tag has the following new sub-elements: `<SetHighQualityForRTCRenderFlag />`, `<RequireLOD0 />`, `<RegisterForInteractionVolume>`
    - The `<DetachFromAircraft>` tag has a new attribute documented: `StayInCabinService`
- The `<Service>` tag in the Services XML Properties has a new attribute documented: `AlternativeBoarding`
- The [CabinService.N] section of the FLT file has the following new parameter documented: `CharacterQuality`
- The [CabinServiceObject.N] section of the FLT file has the following new parameters documented: `ReduceQuantityOnSharedSeatWithCopilot`, `CountByMass`
- The FLT file page has been updated with the new section for the [Water Ballast System.N].
- The [MassSection.N] of the navigation_graph.cfg has the following new parameter: `fillPriority`
- The Package Tool XML Properties reference has been updated with the `<GloballyOverridenBaseSimFiles>` and `<GloballyOverridenBaseSimFile>` tags.
- The Animation XML Properties page has a new tag for MouseRects: `<PrioritizeVCockpits>`
- A page dedicated to the Back On Track system has been added.
- New tags for tailwheel detection in wear and tear have been listed in the Wear And Tear XML Tags section.
- The General Attachments page has been updated with setup information for multiple included SimAttachments.

{{< release-notes-tag "improved" >}}

- The page on Instrument Attachments has been updated with setup information for more displays.
- The Skydiving page has been updated to better explain that the entry/exit of the aircraft can be on the left *or* right.
- Additional information added to the General Career Mode Requirements FLT information related to communication frequencies in FLT files.
- The Navigation Services XML Properties page has received multiple minor updates to improve the clarity of information for many of the tag attributes.
- The Interaction XML Properties page has received multiple minor updates to improve the clarity of information for many of the tag attributes.
- The SimMission Navigation Services page has received multiple minor updates to improve the clarity of information for many of the tag attributes.
- The [Node.N] section of the navigation_graph.cfg has had some updates made to the listed parameters to clarify information.
- The `withExterior_showInterior_hideFirstLod` parameter in the model.cfg file has had it's description improved to match changes in the code.
- The Package Tool XML Properties has been updated with a full list of package order hints.
- The page on Skydiving career setup has additional information about the `climb.flt` file.

{{< release-notes-tag "fixed" >}}

- Out of date information about folders/files used in Modular SimObject Project Structure has been fixed.
- Minor fix to the [Avionics.N] section of the FLT files to correct an issue with the `TransponderState` enum missing "Test".
- The [CabinServiceObject.N] section of the FLT file has had the following parameters removed as no longer used: `Weight`, `PropSetContainerGUID`, `PropSetFile`
- The page on the Ferry Flights career setup has been fixed to include missing information about the `ApronWithoutCovers.flt`
- The page on Search And Rescue missions now correctly lists the interactive object to spawn for hoists to work correctly.


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "added" >}}

- The Reverse Polish Notation page has been updated with the following new `F:` functions: `InputAction`, `MapRange`, `Lerp`, `Clamp`, `Ratio`, `AnimPos`, `EmissivePos`, `VisibilityPos`

{{< /expand >}}



{{< expand title="SDK Release 1.4.3" >}}

<p class="fake-h3">Developer mode changes</p>

<p class="fake-h4">Scenery Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixes made to avoid freezes due to airport rendering.


<p class="fake-h4">Career Tool</p>

{{< release-notes-tag "improved" >}}

- Updated missions available through the Career tool.


<p class="fake-h3">SDK changes</p>

<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "added" >}}

- Added ForceFlare parameter to LightAttributes section of FX files.


<p class="fake-h4">WebAssembly</p>

{{< release-notes-tag "fixed" >}}

- Fixed Wasm module with WasmSystems not reloading/recompiling when restarting.


<p class="fake-h4">Samples</p>

{{< release-notes-tag "improved" >}}

- Updated DA62 sample.
- Updated Cabri G2 sample.


<p class="fake-h3">Documentation</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "fixed" >}}

- All documentation relating to a new flight model parameter (the `empty_inertia_tensor`) has been **removed** as this feature will not be available until the next update (SU4).


<p class="fake-h4">DevMode</p>

{{< release-notes-tag "added" >}}

- The Debug documentation has had the following entries added:
    - Web View Profiler (WIP - Stub only)
    - Debug Flight Plan (WIP - Stub only)
- Pages have been added to describe The Biomes Editor and The Biomes Inspector.
- A page has been added explaining Creating A Biome.

{{< release-notes-tag "improved" >}}

- The section covering the Asphalt Dirt airport archetype override has been updated with additional material information.


<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "added" >}}

- The sim.cfg file docs have a new section: `[Tags]`,
- New pages added to give additional information related to the included SimAttachments (note that these pages are largely placeholders and will be updated over time):
    - General Attachments (WIP)
    - Instrument Attachments (WIP)

{{< release-notes-tag "improved" >}}

- The Texture XML Properties have been updated with an improved list of MTL slots available.
- The section on Cell Types has been updated with new and improved information relating to the pre-defined battery cell parameters of the electrical system.
- The [EXITS] section of the aircraft.cfg and the [INTERACTIVE POINTS] section of the flight_model.cfg have been updated to include information on CFG versioning effects.
- The career documentation for Search And Rescue helicopter missions has been updated with changes to the navigation graph setup.

{{< release-notes-tag "fixed" >}}

- The Ferry Flights page has been changed to remove the `ApronWithoutCovers.flt` requirement.


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "added" >}}

- The new WebAssembly Charts API has been added to the SDK.

{{< release-notes-tag "fixed" >}}

- SIMCONNECT_SIMOBJECT_TYPE now correctly lists all available SimObject types.


<p class="fake-h4">Samples</p>

{{< release-notes-tag "added" >}}

- The new ModelBehavior samples folder has been added to the samples documentation.

{{< release-notes-tag "improved" >}}

- The AirportVehiclesSample page has been given a refresh.
- The Jetway page has been given a refresh.
- The WindsockSample page has been given a refresh.
- The EFB Template Sample page has been given a refresh.
- The SimpleFX page has been given a refresh.
- The TrafficVehiclesSample page has been given a refresh.
- The StandaloneModule page has been given a refresh.
- The WASMAircraft page has been given a refresh, as have **all** the associated sub-pages for the different aircraft presets.

{{< /expand >}}



{{< expand title="SDK Release 1.4.2" >}}

<p class="fake-h3">Developer mode changes</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "improved" >}}

- Updated the "Debug Aircraft Weight" tool to perform the same validation checks on MOI as during aircraft mass properties loading.
- A few GUI improvements to the "Debug Aircraft Weight" tool to make it clearer:
    - corrected grammar and improved wording.  
        added missed units, or grouped them instead to avoid cluttering the window.  
        reordered a few lines and unified indents.  
        reordered "Rs" to align with MOI order.  
        fixed missing "Current Total Improved Weight" display for non-airplane aircraft types.  
        added a clue to clarify triangle inequality violations for principal MOIs.


<p class="fake-h4">Scenery Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed crash when undoing a "replace with SPC".


<p class="fake-h4">Biome Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed creation of imposter material.


<p class="fake-h4">VFX Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed crash when using VFX spawner after cloning a VFX.

{{< release-notes-tag "improved" >}}

- Improved cloning process.


<p class="fake-h3">SDK changes</p>

<p class="fake-h4">Tools</p>

{{< release-notes-tag "fixed" >}}

- Blender: Fixed enable new layer children and update parent layers.
- Blender: Fixed check nested collections and collection list.
- 3DS Max: Fixed AO texture issue in multi-exporter (Babylon).


<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "fixed" >}}

- Fixed an issue that prevented gravity based fuel transfers in some cases.
- Fixed `withExterior_showInterior_hideFirstLod` hiding the entire interior of modular aircrafts if interior model was an empty model. Instead this option will hide the first LOD of all interior models of the aircraft hierarchy, except for these empty models.
- Fixed damage resistance of helicopters tails and skids.


<p class="fake-h4">WebAssembly</p>

{{< release-notes-tag "fixed" >}}

- Fixed deferred events not properly copying their arguments in a multithreaded context (fixes execute_calculator_code not correctly triggering multiple events specified through one single string).


<p class="fake-h4">Samples</p>

{{< release-notes-tag "added" >}}

- Re-added blender scene to the DA62 sample.
- Added 3DS max scene for bears sample.
- Added Blender scene for vehiculs sample.


<p class="fake-h3">Documentation</p>

<p class="fake-h4">DevMode</p>

{{< release-notes-tag "added" >}}

- The Debug documentation has had the following entries added:
    - SimProp Containers
    - Debug Scenery Packages
    - Sound Limiter
    - {{< release-notes-tag "improved" >}}

- The page for Polygon Objects now correctly states the upper vertex limit for these objects.
- The Statistics Profiler documentation has been refreshed to reflect changes in the tool.
- The SimObject Containers documentation has been refreshed to reflect changes in the tool.


<p class="fake-h4">Models And Textures</p>

{{< release-notes-tag "improved" >}}

- The section on Wipers for aircraft windshields has been improved with clearer information regarding setup and animating the wiper material.


<p class="fake-h4">Sound</p>

{{< release-notes-tag "added" >}}

- New section related to animations in SimAttachments has been added to the Sounds And Modular SimObjects page.
- New `<Sound>` and `<WwiseRTPC>` attribute in the Sound XML Properties has been documented - `SimAttachmentAlias`.
- New `<WwiseRTPC>` attribute in the Sound XML Properties has been documented - `AutoSmoothOnAI`.


<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "fixed" >}}

- The pages on Implementing Lights and Light FX Properties have been revised to undo changes made in the previous flighting release as the way the simulation generates the flare post-processing effect has been changed.

{{< release-notes-tag "added" >}}

- The flight_model.cfg has been updated with the following new parameters:
    - [WEIGHT_AND_BALANCE] - `empty_inertia_tensor`
    - [FUEL_SYSTEM] - `Version` (parameter has a new version number added)
- The FLT File Properties has been updated with the following new parameters:
    - [Engine Parameters.N.i] - `RadiatorTemperature_DegR`, `CHT_DegR`,
- The Sound XML Properties has been updated with the new `<SharedPackage />` element.
- New model behavior pages have been added:
    - Interaction Configurations
    - Interaction Procedures

{{< release-notes-tag "improved" >}}

- The Preflight page has been updated with information about using the `Asobo_Preflight` shared package.
- The entire [PNEUMATIC_SYSTEM_EX1.N] section of the FLT files page has been updated to reflect the most recent changes to the pneumatics system.
- Docs updated to reflect the fact that the [HYDRAULICS_SYSTEM_EX1] and [PNEUMATIC_SYSTEM_EX1] are no longer WIP and version 1 is now the current "stable" version. Future updates to these systems will increment the version number.
- The Physics Objects Information has been improved with detailed information on the use and effects of the `empty_inertia_tensor` parameter.
- The Model Behaviors Component Overview has been updated with more information on parameter functions.


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "added" >}}

- Aircraft Radio Navigation Variables - `ATC NEXT WAYPOINT ALTITUDE`, `ATC WAYPOINTS HIGHEST ALTITUDE`


<p class="fake-h4">Samples</p>

{{< release-notes-tag "improved" >}}

- The AirportServices sample project has been give a minor update to improve the information given.
- The LivingWorld page has been given a refresh.
- The SimpleAerial page has been given a refresh.
- The SimpleAirport page has been given a refresh.
- The SimpleBiomes page has been given a refresh.
- The SimpleProjectedMesh page has been given a refresh.
- The SimpleScenery page has been given a refresh.
- The SimpleWasmAirport page has been given a refresh.
- The BearsSampleProject page has been given a refresh.

{{< /expand >}}



{{< expand title="SDK Release 1.4.1" >}}

<p class="fake-h3">Developer mode changes</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "fixed" >}}

- Fixed appearing tools not taking focus when docked to current node and a subwindow is docked on another node.
- Fixed toolbar min size too small + made it resizable in width.
- Fixed parking occupation debug showing nothing.
- Fixed SimObjects debug View button doing nothing.
- Fixed incorrect package name displayed by debug when a new package is mounted.
- Fixed change in texture.cfg not taken into account for BGL.
- Fixed debug "mesh collision" not working corectly when render scale or DLSS is enabled.
- Fixed User lights tools display warning about "emissive mesh need for material" for fake lights.

{{< release-notes-tag "added" >}}

- Added a "Close project" button to package order hint popup.
- Added a new debug in the SimObjects Debug to show the loaded sound packages.
- Added a new SimObject Spawner tool to spawn any SimObject.
- Added a checkbox "Include prop(s) force" to the "Actual State point" In the "Debug Aircraft Wind Tunnel" window. This can help in estimating the actual glide ratio on idle, etc...

{{< release-notes-tag "improved" >}}

- Reduce framerate impact when enabling "Debug model lods".
- Improved Parking Occupation debug window - It is now possible to spawn a passive aircraft at a chosen parking spot and request its ground services.


<p class="fake-h4">Project Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed crash when building scenery with polygon that has duplicated points.
- Fixed simobject path value $simobject$ not correctly replaced.

{{< release-notes-tag "added" >}}

- Added validation of the inertia tensor defined by the "empty_weight_pitch_MOI", "empty_weight_yaw_MOI", "empty_weight_roll_MOI", and "empty_weight_coupled_MOI" parameters in the [WEIGHT_AND_BALANCE] section of the flight_model.cfg file, with error messages printed to the console during loading.


<p class="fake-h4">Scenery Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed biome picker not showing deprecated biomes (but not selectable in the dropdown list).
- Fixed crash when detecting invalid token in scenery XMLs.
- Fixed duplicated model instance for independent projected meshes.
- Fixed live asset reloading of projected meshes.
- Fixed approach light still displayed though it was disabled.

{{< release-notes-tag "improved" >}}

- Allow VectorPlacement objects without airport.
- Don't hide the main scene when editing SimPropContainers (and the "View only current package" option is disabled).


<p class="fake-h4">SimObject Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed parse state of N-dimension arrays.
- Fixed live edition not correctly reloading flt files.
- Fixed livery.cfg incorrectly loaded.
- Fixed crash on save.
- Fixed edition of simobject paths.
- Fixed issues with modular graph edition.
- Fixed modified status after undo or redo when renaming nodes.
- Fixed unselecting the animation when spawning or rebuilding the package in the Animation Editor.
- Fixed confusing and possibly erroneous aircraft geometry parameters

{{< release-notes-tag "added" >}}

- Added editor for label files.
- Added missing field from livery cfg.
- Added debug info visualization when spawning or selecting a container that is not the user in the live edition toolbar.


<p class="fake-h4">Navigation Graph Editor</p>

{{< release-notes-tag "added" >}}

- Added one click placing in Navigation Graph Editor for Aircraft to facilitate edition.


<p class="fake-h4">Biome Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed crash when baking impostors.
- Fixed tree baked with the wrong bitmap mips.

{{< release-notes-tag "added" >}}

- Added a new editor for Biomes.

{{< release-notes-tag "improved" >}}

- Allow streaming of package containing biome xml.
- Read biome xml from all packages.


<p class="fake-h4">Model Thumbnail Baker</p>

{{< release-notes-tag "added" >}}

- Added the option to load a ModelLib package into the tool and see its thumbnails.
- Added option to choose the package to bake.
- Parallelize ModelLib thumbnail bake.


<p class="fake-h3">SDK changes</p>

<p class="fake-h4">Tools</p>

{{< release-notes-tag "fixed" >}}

- Blender: Day/Night Cycle not supported in advanced lights.
- Blender: Fixed emissive material Animation.
- Blender: Fixed material animation on different material slot of same object.
- Blender: Fixed export alpha cutoff only when blend mode is mask.
- Blender: Fixed Primitive Collisions visible in Isolate mode.
- 3DS Max: Fixed export animation options.
- 3DS Max: Fixed minor issue with text overflow in wiper tool.
- 3DS Max: Fixed scene hierarchy refresh + mesh inspector statistics display
- BGL Compiler: Fixed corrupted BGL when it contains only models with invalid bounding box.

{{< release-notes-tag "added" >}}

- 3DS Max: Added "ground" tag for collision. Sim objects will use them as ground elevation.
- 3DS Max: Added two new inputs for emissive (dayMultiplier and nightMultiplier).
- 3DS Max: Added new button "Remove all" to delet all animation groups from view.
- Blender: Added "ground" tag for collision. Sim objects will use them as ground elevation.
- Blender: Added texture preview in material panel.
- Blender: Added a new button to assign base texture set in material panel (albedo, comp, normal, emissive).
- Blender: Added emissiveDayMultiplier and emissiveNightMultiplier.
- Blender: Added support for multi selection in export hierarchy.
- Blender: Export Properties can now be set on multiple items.
- Flightsim Materials: Added auto-adjusting brightness settings for emissive materials.
- BGL Explorer: Added parking and helipad info.

{{< release-notes-tag "improved" >}}

- Blender: MSFS2024 lights preview have been reworked and are now closer to in-game render.
- Blender: Reworked UI Hierarchy. Hierarchy now uses blender list preview template, enabling new options for filtering list , objects selection etc...
- FlightSim Materials: Decals are now supported on SSS materials.


<p class="fake-h4">SimConnect</p>

{{< release-notes-tag "fixed" >}}

- Fixed the size and alignment issue that you could encounter if you use STRINGV type variable in SimConnect if a localized string was returned.
- Fixed SimConnect_EnumerateInputEventParams enumerating the wrong count for some code parameters.
- Fixed issues with input event parameters popping in rpn and when reading the bytes sent to Simconnect_SetInputEvent.

{{< release-notes-tag "added" >}}

- Added RequestAllFacilities function which allows the user to request all airport, vor, ndb or waypoint across the world.


<p class="fake-h4">WebAssembly</p>

{{< release-notes-tag "fixed" >}}

- Fixed WASM memory leak in Event API.

{{< release-notes-tag "improved" >}}

- Changed the Wasm (re)compilation detection to use hash of Wasm file instead of date + size (to avoid multiple recompilation when changing another file in the panel folder for instance)


<p class="fake-h4">SimVars</p>

{{< release-notes-tag "added" >}}

- Added SIMVAR_ATC_NEXT_WAYPOINT_ALTITUDE which indicates the altitude of the next waypoint of the flightpath.
- Added SIMVAR_ATC_WAYPOINTS_HIGHEST_ALTITUDE which indicates the highest altitude of all the waypoints of the flightpath.


<p class="fake-h4">Samples</p>

{{< release-notes-tag "fixed" >}}

- Fixed Simple Biome sample.

{{< release-notes-tag "improved" >}}

- Updated Bear Sample.


<p class="fake-h3">Documentation</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "added" >}}

- New button added to switch between the current SimUpdate documentation, and the Flighting documentation.


<p class="fake-h4">DevMode</p>

{{< release-notes-tag "added" >}}

- New debug window for the Flow and Communication APIs has been added to the documentation: Global Platform Dispatcher
- The Project Editor page has been updated with information on the Order Hint Selection window.

{{< release-notes-tag "improved" >}}

- The page for The Material Inspector has been updated to reflect minor changes with the editor window.


<p class="fake-h4">Models And Textures</p>

{{< release-notes-tag "added" >}}

- New page outlining how Cockpit And Cabin Lights should be setup.

{{< release-notes-tag "improved" >}}

- The page related to **emissive elements** has been merged into the new page on cockpit and cabin light


<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "fixed" >}}

- The Scenery Editor Object XML Properties has had the polygon object BiomeName parameter fixed to reflect the biomes currently available in the simulation.
- The Light FX Properties has been updated to flag the [PARTICLE.0] section as obsolete.

{{< release-notes-tag "added" >}}

- The [Engine Parameters.N.i] section of the FLT files has a new parameter: `OilTemperatire_DegR`,

- The navigation_graph.cfg has the following new parameters:
    - [Node.N] - isEnter,

- The Navigation Services XML Properties has been updated with the following new elements: `<RegisterForInteractionVolume>`, `<UnRegisterForInteractionVolume>`, `<NodeStep>`,

- The sim.cfg file has the following new parameters:
    - [BoardingRamp] - `top_position_when_straight_YZ`,

- The cockpit.cfg file has been updated with the following new sections: `[HELICOPTER_ROTOR_RPM]`, `[BALL_INDICATOR.N]`

- The engines.cfg file has been updated with the following new parameters:
    - [TURBOPROP_ENGINE] - `InertialSeparatorOnTorque`

- The page on Implementing Lights has been updated to include information on light flare effects.

{{< release-notes-tag "improved" >}}

- The `DeleteOnNodeNotFound` attribute of the Sequence tag has been renamed to `HideOnNodeNotFound`.
- The following ai.cfg parameters are considered obsolete and have been moved to the appropriate page: `brakeDifferentialPID`, `throttleDifferentialPID`,
- The navigation_graph.cfg documentation has a added a new possible input value to the `interactiveVolumeConstraint` parameter: `RegisterOnly`.
- The Navigation Services XML Properties page has been updated with a additional attributes for:
    - &lt;ServiceState&gt; - `RecomputeInteractiveObjectsLinks`,
    - &lt;MoveToGraphNode&gt; - `MaxSpeed`, `MaxSpeedAlways`, `HideOnNodeNotFound`, `CheckNodeReserved`
    - &lt;TeleportToGraphNode&gt; - `TeleportWhenNodeAvailable`, `CheckNodeReserved`
    - &lt;Sequence&gt; - `RequireLOD0`, `SetHighQualityForRTCRenderFlag`
- The Aircraft Loads page has been updated to complete the testing and debugging section.


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "added" >}}

- New sections for the Flow API (WASM) and Flow API (JavaScript) have been added.
- The following new SimVars have been added to the documentation:
    - Services Variables - `BOARDINGRAMP STRAIGHT POSITION Y`, `BOARDINGRAMP STRAIGHT POSITION Z`


<p class="fake-h4">Samples And Tutorials</p>

{{< release-notes-tag "improved" >}}

- The SimpleNavData sample page has been updated with screenshots appropriate to Microsoft Flight Simulator 2024.

{{< /expand >}}



{{< expand title="SDK Release 1.3.4" >}}

<p class="fake-h3">Developer mode changes</p>

<p class="fake-h4">SimObject Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed livery.cfg and attachment.cfg not loading.


<p class="fake-h3">SDK changes</p>

<p class="fake-h4">Tools</p>

{{< release-notes-tag "fixed" >}}

- Blender - Progress now shows 100% at the end of export process.
- Blender - Fixed typos and duplicated shader nodes.

{{< release-notes-tag "improved" >}}

- Blender - Reduced stuttering during export.

{{< release-notes-tag "added" >}}

- Added Visual Studio Code extension for model behaviors to the SDK installer.


<p class="fake-h3">Documentation</p>

<p class="fake-h4">DevMode</p>

{{< release-notes-tag "added" >}}

- The Career Compatibility page has been added to the documentation.

{{< release-notes-tag "improved" >}}

- The SimObject Editor pages have been refreshed to match the current state of the simulation.
- The page on Custom or Erroneous Parameters has been improved to suit the updates to the SimObject Editor.


<p class="fake-h4">Models And Textures</p>

{{< release-notes-tag "added" >}}

- New page dedicated to the MSFS 2024 glTF Format has been added.
- New page on LOD Technical Information has been added with general LOD creation guidelines.
- New page with a LOD Example added.
- New section on Animation Guidelines has been added to the modeling documentation.

{{< release-notes-tag "fixed" >}}

- The Wiper Mask Generator docs have been fixed to reflect the current state of the tool.

{{< release-notes-tag "improved" >}}

- The Decal materials page now correctly specifies over which other material types they will be rendered.
- The The 3DS Max Plugin page has been updated with additional render option information.
- The Modeling Technical Information page has been updated with additional general modeling guidelines.
- The Texturing Techincal Information page has been updated with additional general texturing guidelines.


<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "added" >}}

- The page on General Career Mode Requirements has new sections added for **Takeoff Weight** and **Maximum Altitude** constraints.

{{< release-notes-tag "improved" >}}

- The cockpit_type parameter has been given an updated and improved description.
- The Input Application page has been updated to match the current iteration of the application, and all documentation related to input profiles has been removed pending an update to correct errors and improve the information.


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "added" >}}

- New SimConnect sample - SpawnSimObjectWithLivery - has been documented.

{{< /expand >}}



{{< expand title="SDK Release 1.3.3" >}}

<p class="fake-h3">Developer mode changes</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "fixed" >}}

- Fixed a crash that was happening when selecting an air traffic aircraft in the Aircraft Selector tool.
- Fixed a random crash when switching to VR with DevMode & Smart Docking System activated.


<p class="fake-h4">Scenery Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed terraforming discontinuity at the end of runways.
- Fixed custom runway start not being used in freeflight.
- Fixed incorrect runway used for spawn when the assistance option 'ATC enforce flight plan' is disabled and another runway was selected as departure.
- Fixed discontinuity when a terraforming polygon is split.
- Fixed wrong texture directories being cached when generating scenery index.
- Fixed rectangles changing length when edited.


<p class="fake-h4">Project Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed incorrect priority for "INVALID" package order hint.

{{< release-notes-tag "improved" >}}

- New automatic order hint detection and reworked UX of order selection.


<p class="fake-h4">SimObject Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed GLTF node detection in SimObject editor career compatibility tab.


<p class="fake-h3">SDK change</p>

<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "fixed" >}}

- Fixed usage of EFB tag in panel.cfg when it was not a sim-attachment.

{{< release-notes-tag "added" >}}

- Implemented a similar logic to the yoke's for the helicopter collective in VR, requiring new parameters in the aircraft.cfg and flight_model.cfg.


<p class="fake-h4">WebAssembly</p>

{{< release-notes-tag "fixed" >}}

- Fixed crash when clsing the game during a flight with an aircraft that do calls to the CommBus API in the kill callback.


<p class="fake-h4">JavaScript</p>

{{< release-notes-tag "added" >}}

- Added access to legacy fs2020 UI framework , to let you access it you can await window.legacyInit(); before loading your js.


<p class="fake-h4">Tools</p>

{{< release-notes-tag "fixed" >}}

- Blender / Max - Wiper Tool: Fixed the incorrect wiperMesh generation when they're multilples meshes
- Blender / Max - Wiper Tool: Fixed render settings output in wiper mask generator (always output an RGBA)
- Blender: Fixed flags in texture xml generation

{{< release-notes-tag "improved" >}}

- Blender / Max - Wiper Tool: Removed AnimationOut : We generated the out animation by inversing the AnimationIn. The outputed babylon animation must be only the In Animation.


<p class="fake-h3">Documentation</p>

<p class="fake-h4">Developer Mode</p>

{{< release-notes-tag "added" >}}

- The Model Thumbnail Baker in the **Tools** menu has been documented.
- The The Video Capture Tool in the **Tools** menu has been documented.
- New WASM Floating Point Exceptions added to the **Options** menu documentation.

{{< release-notes-tag "improved" >}}

- The Tools menu page has been updated to reflect the current state of the DevMode menu.
- The The Aircraft Capture Tool page has been improved with information on creating thumbnails and using the new thumbnail preset file included with the SDK.


<p class="fake-h4">Models And Textures</p>

{{< release-notes-tag "added" >}}

- The FlightSim Materials documentation has been updated with new parameters for the ClearCoat material.
- Information on modelling and exporting Seats And Seatbelts has been added.
- Information related to cubemap setup has been added to the Translucent Elements docs.
- Information about the setup for airframe Dirt And Grime has been added.

{{< release-notes-tag "fixed" >}}

- Incorrect information related to the Glass Material setup has been removed from the Translucent Elements docs.
- The Windshield And Windows documentation has been updated to show correct rain setup information.

{{< release-notes-tag "improved" >}}

- Information related to setting up Ice and Frost on Windshield And Windows has been updated.
- Information related to setting up Frost And Ice on the airframe has been updated.


<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "fixed" >}}

- The aircraft.cfg section on loading tips/images has been moved to CFG Files Obsolete Parameters as it is not used in MSFS 2024.
- The `always_execute_associate_js` and `always_execute_model_behavior` from the attached_objects.cfg have been moved into the CFG Files Obsolete Parameters page as they are no longer used in MSFS 2024.
- The General Template XML Properties has fixed the `UpdateFrequencyPreset` text to show it as a component *attribute* rather than a *sub-element*.
- The Airport XML Properties now correctly lists the traffic overrides for airport archetypes.

{{< release-notes-tag "added" >}}

- The flight_model.cfg has the following new parameters:
    -   In the `[AIRPLANE_GEOMETRY]` section - `fly_by_wire_load_factor_normalize_bank`, `control_aileron_failed_hydraulic_weight`, `control_elevator_failed_hydraulic_weight`, `control_rudder_failed_hydraulic_weight`
    -   In the `[HELICOPTER]` section - `collective_move_rate_limit_vr`
- The aircraft.cfg has the following new parameters:
    -   In the `[FLTSIM.N]` section - `ui_electrical_engine_consumption`, `ui_engine_available_electrical_capacity`,
    -   In the `[VR]` section - `collective_anim`, `collective_node`, `collective_collision_mesh`
- The livery.cfg has the following new parameters:
    -   in the `[FLTSIM.N]` section - `icao_airline`, `atc_parking_types`, `atc_parking_codes`, `atc_id`
- A new section detailing **contact point definition order** has been added to the flight_model.cfg - Additional Information page.
- New career mission documentation added:
    -   General Career Mode Requirements
    -   Career Activities Additional Information
    -   Aircraft Loads
- The following parameters have been added to the FLT File Properties:
    -   `[Controls.N]` - `YokeLock`, `RudderLock`
- The &lt;RunwayTransitionLegs&gt; element has been updated with new attributes: `clearanceAltitudeJet`, `clearanceAltitudeProp`.
- A new page specifically to help with the setup of Aircraft Seats And Seatbelts has been added.
- New `<TrueRandom>` element added to the Livery XML Properties page.
- A new page dedicated to Sounds And Modular SimObjects has been added.

{{< release-notes-tag "improved" >}}

- The page on SimPropContainer XML Properties has been updated with missing information.
- The FLT Files General Information page has been updated with additional information on the various FLT files that can be used.
- In the `[TURBINEENGINEDATA]` section of the engines.cfg file, the description for `rated_N2_rpm` has been updated.
- The Modular Hydraulics System Information for the Pump.N section has been updated to include APU Driven pumps.
- The Spawn Pilot (Transversal) and Commercial Flights And Passengers has been improved with relation to seat and seatbelt interactions.
- All the Career Activities pages have been improved with more precise information and changes to the FLT setup.
- The EFB Flight Plan XML (PLN File) Properties page has been improved to correct some minor issues.


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "added" >}}

- SimVars: New variables added to the documentation.
    -   Miscellaneous Variables - `VR GROUP INTERACTION ON`

    -   Aircraft Engine Variables - `THROTTLE INPUT BLOCKED BY LOWER BOUNDS`, `THROTTLE INPUT BLOCKED BY UPPER BOUNDS`

    -   Aircraft Radio Navigation Variables - `ATC ASSIGNED ALTITUDE`
- RPN: New environment variable `IS CAMERA LOCKED` added.


<p class="fake-h4">Samples</p>

{{< release-notes-tag "added" >}}

- The LFPG Airport sample has been fully documented.
- The SimplygonSimpleScene sample has been fully documented.
- The DA62 Mod sample has been fully documented.

{{< release-notes-tag "improved" >}}

- Minor updates and corrections have been made to the main Samples, Tutorials and Primers page.
- Minor updates to the PackageInstaller page have been made.

{{< /expand >}}



{{< expand title="SDK Release 1.3.1" >}}

<p class="fake-h3">Developer mode changes</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "fixed" >}}

- Fixed stuck in slew mode when teleporting avatar from devmode.
- Fixed keyboard input latency when low framerate and pressing key for long time using the DevMode Camera.
- Fixed altitude isoline debug window.

{{< release-notes-tag "added" >}}

- Added debug for airport starts (available in Debug &gt; Airports &gt; Draw Debug).
- Added SimPropContainer debug in the debug menu.
- Added visualization for wake turbulence CFD in CFD Debug window.

{{< release-notes-tag "improved" >}}

- Rename "Scenery package order (WIP)" to "Package reorder tool".
- Sorted editors by alphabetical order in main menu bar.
- Better warning message when loading object from BGL failed.


<p class="fake-h4">Career Tool</p>

{{< release-notes-tag "fixed" >}}

- Fixed mission which couldn't be started using the Career Tool.
- Fixed DevMode display which could disappear (even if it was enabled) after a mission using the Career Tool or by moving to some menus.

{{< release-notes-tag "improved" >}}

- Career tool has clearer UI, and mission parameters are properly taken into account.
- Updated missions offered in Career Tool.
- Career Tool now displays mission titles.


<p class="fake-h4">Project Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed SimPropContainer wizard.
- Fixed crash when building a package that does not include release notes for its current version.
- Fixed possible crash when removing a package containing the current aircraft while in the aircraft configurator.
- Fixed incorrect value for month in revision notes.
- Fixed crash when cleaning package that contain long paths.
- Fixed crash in the airport creation wizard.

{{< release-notes-tag "added" >}}

- Added Animal & Boat templates.
- Added Ground Vehicle template.
- Added empty scene in the wizard.


<p class="fake-h4">SimObject Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed a crash when changing the sound file in sound tab.
- Fixed unchecked deletion of folders.
- Fixed loading indexed section without index.
- Fixed possible crash in modular hydraulic system section.
- Fixed possible crash with incorrect data and float3 parameter.
- Fixed some parameters incorrectly shown as unknown parameter error.
- Fixed having no aircraft after a build package when the fltsim title changes.
- Fixed using undo with edit in place mode.
- Fixed NavGraph Data Migration dialog size.
- Fixed editing modular graph not correctly reloading attached_object cfg panel.
- Fixed Aircraft Template having errors in SimObject Editor.
- Fixed using gizmo not correctly updating the modified state.
- Fixed parsing legacy hashmap parameter.
- Fixed resync done before save when showing the validation error dialog.

{{< release-notes-tag "added" >}}

- Added new debug window to debug parachute aerodynamics.
- Added new Live Edition mode to allows instant reload of cfgs.
- Added a gizmo for boarding ramp extent.
- Added missing LocalVars section.
- Added Node Lister window to list the nodes found in GLTF files in the current asset.
- Added gizmo for attached_objects.cfg / attach_offset.
- Added a popup to ask if user wants to save his scenery when opening another asset.
- Added a popup to ask if user wants to save his scenery when clicking on "Go back main scene" in SimPropContainer edition instead of always saving.

{{< release-notes-tag "improved" >}}

- Improved node list dialog to also get animation names and current container hierarchy.
- Improved debug of engines for aircraft: turbines, propellers, piston, jet...
- Now correctly list navigation graph from modular hierarchy.
- Improved UI by adding live edition tip and link to the online documentation.
- Improved list of parameters with multiple missing entries.
- In Live Edition mode, the attachements transform will be updated using attach_offset / pbh and scale.
- Improved attached_object.cfg asset file selection.


<p class="fake-h4">Material Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed the "remove before flight" material that was very dark in the shade, due to AO being black instead of white.

{{< release-notes-tag "added" >}}

- Added "Flip backface" option for VFX and dynamic materials.

{{< release-notes-tag "improved" >}}

- Now only hide runtime material if a package and a lib are selected.


<p class="fake-h4">Scenery Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed decal objects applied on aircrafts.
- Fixed rectangle objects being picked up in priority when they shouldn't be.
- Fixed exclude generated buildings from taxiways.
- Fixed osm road draw ignoring exclusion polygon.
- Fixed objects drag and drop warning tooltip.
- Fixed airport lights not supporting multiples meshes or root orientation.
- Fixed strobe pole rotation.
- Fixed build error when a scenery object has both "NoSnow" and "SnapToBuilding" flags.
- Fixed backward compatibility with dirt on taxiway parkings.
- Fixed resume edition for polygons and polylines (undo/redo and direction).
- Fixed multi edit not working properly for some object types.
- Fixed an issue in SimPropContainer edition, where the first object placed wasn't movable.
- Fixed group move/rotate/rescale.
- Fixed an issue where undo/redo wouldn't affect some properties.
- Fixed a render issue that could occur when scaling down a polygon, or removing points from it.
- Fixed undo/redo issues with groups in SimPropContainer edition.
- Fixed Apply Flatten option.
- Fixed bushes, scrubs and grass not spawning when using a vegetation polygon.
- Fixed biome override not working correctly on bushes and scrubs.
- Fixed ground merging on aerial with texture synthesis for polygons and projected meshes.
- Fixed add to selection with ctrl+click inside viewport which wouldn't select objects occluded by an already selected object.
- Fixed remove from selection with alt+click inside viewport which wouldn't unselect objects occluded by an object that isn't selected.
- Fixed dirt size not correct for parking ramps.
- Fixed buildings occasionally spawing too close to a runway.
- Fixed gltf lights not rendered in airport lights or VectorPlacement.
- Fixed most detected buildings spawn on parking spots.

{{< release-notes-tag "added" >}}

- Added a pickup priority system by type to improve pickup order when several objects are overlapping at mouse click position.
- Added support for detail maps for projected meshes
- Added debug showing scenery object packages.
- Added light edition in SimPropContainer (street lights and advanced lights).
- Added "force elevation" option for terraformers.
- Added option to disable dirt on taxi parkings.
- Added option to select all similar scenery objects.
- Added option to use mass instancing for library objects.

{{< release-notes-tag "improved" >}}

- Made "Referencing unknown SimPropContainer" a warning instead of an error.
- Refresh simobject list when a package is mounted or built.
- Removed polygon option "Exclude power lines".
- Exclude trees from parking spots and taxi paths.
- Removed SimPropSet object during SPC edition.
- Support collision for instanced scenery objects.
- Use parking radius instead of parking type for dirt on the ground.
- Enhanced rectangle selection when polygon was selected, to allow user to select other objects if no polygon points was selected.


<p class="fake-h4">VFX Editor</p>

{{< release-notes-tag "improved" >}}

- Now allows you to select links and comment nodes when selecting multiples item in the node graph.


<p class="fake-h4">Model Behavior Debug</p>

{{< release-notes-tag "fixed" >}}

- Fixed some wording and reduce verbosity wherever possible.
- Fixed copy to clipboard duplicate prompts.

{{< release-notes-tag "added" >}}

- Added copy to clipboard prompt wherever it was missing to be on par with instance debug.
- Added an option to toggle the inspector side bar visibility (hidden by default).

{{< release-notes-tag "improved" >}}

- Tooltips tab now replaces dynamic parameters with their known value.
- Colorize most numeric/flags/string fields.
- Wrap code text.
- Hide some fields when they are not relevant.
- Allow debugging of one preset at a time.
- Persist some additional settings (Interaction, Option and General &gt; View &gt; Inspector side bar menus).


<p class="fake-h4">Statistic Profile Window</p>

{{< release-notes-tag "fixed" >}}

- Fixed Grand Total count in Scenery Statistics when sorted by packages.

{{< release-notes-tag "improved" >}}

- Use GPU size for bitmap.
- More detailed information for airport face count.


<p class="fake-h4">Systems Debug (Electrical)</p>

{{< release-notes-tag "fixed" >}}

- Fixed crash when opening list of powered consumers in a supplier details panel, circuits and relays are now displayed in separated lists.

{{< release-notes-tag "added" >}}

- Added new columns (Tension, Power, number of suppliers) to relays quick access information.
- Added circuit type filter in circuits quick access debug window.
    -   Average load, average tension and power for suppliers.
    -   Tension, Power and number of suppliers for the circuits.
    -   Power for the buses.
- Added informations in the connectibles details (right part of main panel), lists of suppliers powering a consumer or a list of consumers powered by a supplier are now available via a new menu.

{{< release-notes-tag "improved" >}}

- The debug window can now be opened without needing the behaviors debug window to be opened as well.
- Allow item selection across all columns in quick access lists.
- Made columns in lines and breakers debug window tabs hideable.
- Improved electrical system debug window with better button displays in the quick access panel.
- Quick access window upgrades:
    -   Added a 'Power setting' column in the circuits section.
    -   Moved explicative text in a button in the menu bar.
    -   Improved columns readability by greying out some values when they are at 0.
    -   Replaced huge values in diodes voltages tooltip in the lines tab with "INF".


<p class="fake-h4">Aircraft Capture Tool</p>

{{< release-notes-tag "fixed" >}}

- Fixed reset UTC DateTime when closing the AircraftCaptureTool.


<p class="fake-h4">Navigation Graph Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed saving of names in Navigation Graph Editor.

{{< release-notes-tag "improved" >}}

- Changed Navigation Graph layout file extension from .xml to .edition for consistency across editors.


<p class="fake-h3">SDK change</p>

<p class="fake-h4">WASM API's:</p>

{{< release-notes-tag "fixed" >}}

- Fixed crash when loading aircraft with Wasm.
- Fixed infinite loading with Aircraft containing WASM when loading creates a low memory.
- Fixed crashed occuring when ending flight with plane using Wasm and SimConnect.
- Fixed potential crash when deleting a vfx with the WASM VFX API.
- Fixed delay on wasm callback.
- Fixed crash happening when cancelling loading of flight with a plane containing multiple WASM module.
- Fixed a bug where Wasm Vars and Events APIs where not usable in standalone or airport modules.

{{< release-notes-tag "added" >}}

- Added Write function in Wasm Fs IO API.


<p class="fake-h4">SimVars</p>

{{< release-notes-tag "fixed" >}}

- Fixed VarSet of the simvars RECIP CARBURETOR TEMPERATURE and RECIP MIXTURE RATIO.

{{< release-notes-tag "added" >}}

- Added simvar AIRSPEED INDICATED THEORETICAL that indicates the airspeed ignoring any failure, cover, or icing on the pitot.
- Added ROTOR RADIUS SimVar which returns the radius in feet of rotors (parameter needs to be either 1 or 2).
- Added a new simvar VR GROUP INTERACTION ON which is true when in VR and the VR group interaction is on
- Added FUEL TOTAL QUANTITY EX1 and FUEL TOTAL QUANTITY WEIGHT EX1 simvars to fix inconsistancies in default simvars. These will always provide include the unusable fuel regardless of which fuel system is used.
- Added simvar AIRCRAFT OBJECT CLASS for retrieving the object class of a simobject.
- Added simvars ENG TILT PITCH PERCENT EX1 and ENG TILT YAW PERCENT EX1 which change the angles of the engines using a linear range from the minimum to the maximum angle.

{{< release-notes-tag "improved" >}}

- Modified THROTTLE INPUT BLOCKED BY (LOWER / UPPER) BOUNDS' to be indexable. Previous uses (without index) keep the same behavior: get - returns true if any index is true, set - all indexes to the specified value.


<p class="fake-h4">RPN</p>

{{< release-notes-tag "fixed" >}}

- Fixed signed integer overflow in rpn integer parsing when converting from double.
- Fixed integers division in RPN expressions

{{< release-notes-tag "added" >}}

- Added new environment var (P:IS CAMERA LOCKED, Bool) to know if the current camera is locked.


<p class="fake-h4">SimConnect</p>

{{< release-notes-tag "fixed" >}}

- Fixed Aircraft spawning on the same parking slot when spawned with Simconnect.


<p class="fake-h4">systems.cfg</p>

{{< release-notes-tag "fixed" >}}

- [Electrical] - Fixed external power not outputing any tension when of AC type.
- [Electrical] - Fixed load sharing issues with batteries.

{{< release-notes-tag "added" >}}</p>

- [Electrical] -Added new parameter for AC suppliers: TensionDropRPM. Giving this parameter will change the way AC suppliers produce tension, where above the given value VRMS will be used as outputed tension, and under the given value the outputed tension will be linearly decreased from VRMS to 0. Also, if this parameter is given, the system will ignore others based on frequency, such as: Frequency, Phase, ReferenceFrequency, ReferenceRPM, NumberOfPoles.


<p class="fake-h4">flight_model.cfg</p>

{{< release-notes-tag "fixed" >}}

- Fixed several aerodynamics backwards compatibility issues with drag.
- Fixed crash happening when obj_ea1_fuselage element_number members product is less than 2.

{{< release-notes-tag "added" >}}

- Added surface_cx and element_weight parameters to tweak the parachute aerodynamics.
- Added [FUEL_SYSTEM] version 6.
- Added a new parameter tailwheel_algo_detection which can be used to let the sim use the new tailwheel detection algorithm.
- Added new parameter max_water_depth to [CONTACT_POINTS] section to allow to simulate deeper water.
- Added into the [AIRPLANE_GEOMETRY] section - control_aileron_failed_hydraulic_weight, control_elevator_failed_hydraulic_weight, control_rudder_failed_hydraulic_weight. Used to make Control surfaces heavier when facing hydraulic failure.


<p class="fake-h4">cameras.cfg</p>

{{< release-notes-tag "fixed" >}}

- Fixed [CAMERADEFINITION] VarToggle_EX1 visibility not reset when changing camera.


<p class="fake-h4">aircraft.cfg</p>

{{< release-notes-tag "added" >}}

- A validation process has been developed to give more control to the aircraft developer over the career compatibility. A new field has been added in the [FLTSIM] section "targeted_specializations" (supported by the SimObjectEditor) to list the expected specialization with which the given aircraft should be compatible. During the ingestion process, the field will be compare to the result of the career compatibility process to confirm its result. THIS FIELD MUST BE FILLED TO BE COMPATIBLE WITH CAREERS.


<p class="fake-h4">Model Behaviors</p>

{{< release-notes-tag "fixed" >}}

- Fixed frequency presets which were no longer functioning in MSFS2024
- Fixed frequency presets parsing from crashing the build package process


<p class="fake-h4">FLT Files</p>

{{< release-notes-tag "fixed" >}}

- Fixed an issue causing LocalVars_EX1 FLT parameters not to get saved properly.


<p class="fake-h4">Audio XML</p>

{{< release-notes-tag "added" >}}

- Added support for an optional "AbsoluteValue" attribute on WwiseRTPC tags in sound.xml/soundai.xml files, it expects a boolean value and tells whether to use the absolute value of the variable.


<p class="fake-h4">Samples</p>

{{< release-notes-tag "added" >}}

- Added a sample to show how to use new function : SimConnect_EnumerateSimObjectsAndLiveries.
- Added Blend sample for windsock.
- Added a new sample that shows how to mod an existing aircraft.

{{< release-notes-tag "improved" >}}

- Updated Cabri G2 SDK sample:
    -   Fixed Livery_Static_01 LOD issue
    -   Fixed emissive wrongly appearing in cockpit
- Updated DA62 SDK sample:
    -   Added sunvisors in cockpit.
    -   Added new screws on glareshield and ceiling.
    -   Fixed seatbelts positions.
    -   Added loads in trunks.
    -   Fixed art windows.
    -   Fixed Fueling nozzle and animation not appearing.
    -   Added LODs for smoother switch.
    -   Fixed color interior for passenger and scientific variation.
    -   Fixed interior details map (seat, ceiling, alcantara, leather).
    -   Added a detail map of leather hole.
    -   Added subdivision for LOD00 of all interiors (scientific and passenger).
    -   Fixed bug UV in interior (seatbelt, seat).
    -   Cleaned Livery_Static_01
    -   Cleaned .max and LODs for SDK
    -   Added new blend scenes for DA62 sample.
- Updated PackageInstaller sample to use latest WiX Toolset version.


<p class="fake-h4">Tools</p>

{{< release-notes-tag "fixed" >}}

- BglExplorer
    -   Fixed missing SimObjects and Runways info.
    -   Fixed GUID that can't be copied.
    -   Fixed display of projected mesh info.



- Blender
    -   Fixed alpha mode not exposed in Tree material.
    -   Update normal link when normal detail texture is set.
    -   Fixed set detail windshield normal scale.
    -   Fixed default values for windshield.
    -   Fixed export "asobo_material_rain_options" extension.
    -   Fixed UV offset orientation following in-game orientation.
    -   Fixed gltf lights import (hierarchy, transforms, light parameters etc).
    -   Fixed invalid emissive scale and emissive color values on gltf import.
    -   Fixed emissive texture preview.
    -   Fixed emissive scale not properly set on material creation.
    -   Fixed import type error when importing a gltf containing a Parallax Window material.
    -   "Add collision" operator now works in edit mesh mode (In edit mode, operator adds a collision fitting selection bounding box.
    -   In object mode, operator adds a collision fitting entire object).
    -   Fixed Collision primitives display : collisions could disappear or flicker when unselected.
    -   Fixed export lights.
    -   Fixed export material animations
    -   Fixed export objects.
    -   Fixed set Blend Mask Texture



- 3DS Max
    -   Fix check if texture name contains whitespaces for texturelib xml generation.
    -   Properly handle invalid simplygon license in 3dsmax blobmesh tool.
    -   Fixed for bad meshes in blob mesh tool : tool was crashing when trying to optimize meshes with dead elements.
    -   Fixed export options save bug : export options could not be re-opened until multi-exporter is closed and re-opened + dirty flag was not going away when saving options.



- Input App
    -   Fixed InputApp crash when device not properly recognized.
    -   Fixed device axis feedback point not moving on curve control per input action settings



- Coherent
    -   Fixed the frame distribution system and views refresh rate to match KH when &lt;= 60fps. When higher than 60FPS, we schedule things with the proper refresh rate (as much as we can, obviously).
    -   Fixed support for AnimationFrameDefer=0.0 (unused, but we may want this for the UI)
    -   Fixed SetAnimationFrameTimeFactor to same reference time as the view creation + adjust settings to match KH behaviour.

{{< release-notes-tag "added" >}}

- Blender
    -   Added color conversion in fast lights and advanced lights from kernel temperature to color.
    -   Added support for alpha Mask Mode and alpha cutoff in viewport shader.
    -   Added flare/illumination property to street lights and advanced lights.
    -   Added "Base normal affect coat" parameter in clearcoat material.
    -   Added "decal channels" to decal materials.
    -   Added "Settings presets" to export settings
    -   Added "import material" and additionnal texture dirs" options for gltf importer ("Import material" unticked will import objects without material. "Additionnal textures dirs" enable user to provide a list of external texture directories. Meant to be used when textures paths can't be resolved automatically by gltf importer).
    -   Added automatic resolve of texture paths on gltf Import.
    -   Added enable/disable export meshs.
    -   Added Tire material.
    -   Added "export as submodel" option



- 3DS Max
    -   Added clipping plane and geometry options in blob mesh tool for 3dsmax.
    -   Added an option to streetlights in 3dsMax to have the lens-flare effect as well.



- Coherent
    -   Implemented KH's frame distribition system, behind the flag `UseFixedFrameDistribution`. This always assumes a framerate of 60fps for the distribution, turning the system into a frame based frequency instead of a real time frequency. This is what was done in KH except that we are computing this per view instead of for all the views.

{{< release-notes-tag "improved" >}}

- Blender
    -   Export is done in a separate background process. It allows user to continue working while exporting. Can be disabled using "Export In Background" checkbox in export settings.
    -   A progress bar with process infos is displayed during background export.
    -   Primitives collisions can now be imported.
    -   Adjusted Import Panel UI for more readibility.
    -   Collisions are now highlighted when selected.
    -   Update default emissive scale.
    -   Clarify use of material_type update function.
    -   Adjustments of standard shader : Rename uv2 input and Unlink AO when there are no base color texture of detail color texture set.



- 3DS Max
    -   Sync parameter names for parallax material with Blender.
    -   3dsmax viewport material emissive adjusted so blender and 3dsmax should get more consistent previews.
    -   3dsmax blob mesh generation tool now support more material types.
    -   3dsmax material type drop down reordered.



- Coherent
    -   Made the frame distribution system easily configurable at runtime by exposing it as settings and tweakables.
    -   Allowed to use the game deltatime instead the wall clock (enabled by default).


<p class="fake-h3">Documentation</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "fixed" >}}

- The apron control setup pages for Passenger Transport and Catering are now correctly referenced in the ToC.
- The Airport Archetype Overrides page is now correctly linked in the main ToC.
- Broken ToC links to Content Creator Testing Tool and Balloon / Airship Specific Events have been fixed.
- Broken mini-ToC links due to malformed CSS on several pages have been fixed.


<p class="fake-h4">Developer Mode</p>

{{< release-notes-tag "added" >}}

- New sections added to the page about Airport Debugging: Parking Occupationand Apron Control.
- The Aircraft Debugging tools page has a new section for the Sim Wind Tunnel debug window.
- The The Material Inspector page has been updated with new features for controlling ground material heightmaps and transparency.


{{< release-notes-tag "improved" >}}

- The obsolete Sim Polar ClCd window documentation has been removed to reflect changes in the aircraft debugging tools.
- The description for the Interior Mask channel display option has been updated.
- The description for the Polygon Object **Priority** option has been updated for clarity.
- Warning added to ExclusionRectangle Objects to emphasise navigation issues with the "DeleteAirport" option.
- The obsolete option "Exclude Power Lines" has been removed from the page on Polygon Objects (this is now achieved using the ExclusionRectangle Objects).
- The ProjectedMesh Objects documentation has been updated with an improved list of possible textures that can be used in the projected mesh material.
- The Live Edition section of the SimObject Editor menus documentation has been updated to include more precise information on what parameters can be edited live.


<p class="fake-h4">SDK Tools</p>

{{< release-notes-tag "fixed" >}}

- The Package Tool XML Properties page has been fixed to correctly show the `<PackageOrderHint>` element.


<p class="fake-h4">Models And Textures</p>

{{< release-notes-tag "improved" >}}

- The 3DS Max Plugin has been updated to correctly reference the supported versions.


{{< release-notes-tag "fixed" >}}

- The material documentation for the Render On ClearCoat parameter has been updated to clarify differences between 3DS Max and Blender.
- The material documentation for the Parallax Parameters parameter has been updated to reflect naming changes bringing the 3DS Max version in line with the Blender version.


<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "added" >}}

- The engines.cfg has the following new parameters: `prop_rotation_threshold`, `prop_acceleration_threshold`
- The flight_model.cfg has the following new parameters:
    -   In the `[AERODYNAMICS]` section - `fuselage_vertical_cx`, `fuselage_longitudinal_cx`, `fuselage_max_rigidity`,
    -   In the `[OBJ_EA1_FUSELAGE.N]` section - `position`, `size`, `group`, `element_number`, `dim_scale_top`, `dim_scale_lat`, `dim_scale_bottom`, `dim_offset_middle`, `surface_cx`, `surface_cx_tangent`, `surface_cx_normal`, `surface_cx_efficiency`, `surface_cx_nscaler`, `surface_cx_npower`
    -   In the `[OBJ_EA1_SURFACE.N]` section - `position`, `size`, `group`, `element_number`, `surface_cx`, `surface_cx_tangent`, `surface_cx_normal`, `surface_cx_efficiency`, `surface_cx_nscaler`, `surface_cx_npower`, `modifier`, `modifier_angle_scalar`, `modifier_position_scalar`
    -   In the `[OBJ_EA1_ANCHORROPE.N]` section - `position`, `size`
    -   In the `[OBJ_EA1_YAWSTRING.N]` section - `position`, `position`, `element_number`, `element_weight`, `element_spacing`, `element_width`, `surface_relative_position.N`, `surface_angle.N`
    -   In the `[OBJ_AIRGEO_FUSELAGE.N]` section - `position`, `size`, `group`
    -   In the `[OBJ_AIRGEO_VTAIL.N]` section - `position`, `area`, `span`, `sweep`, `dihedral`, `incidence`, `group`
    -   In the `[OBJ_AIRGEO_HTAIL.N]` section - `position`, `area`, `span`, `sweep`, `incidence`, `group`
    -   In the `[OBJ_AIRGEO_WING.N]` section - `position`, `area`, `span`, `sweep`, `incidence`, `dihedral`, `twist`, `stallalpha`, `stallalpha_ff`, `group`
    -   In the `[CONTACT_POINTS]` section - `tailwheel_algo_detection`, `max_water_depth`
- A new information page for the flight model - related to additional details specific to the MSFS 2024 physics objects - has been added: flight_model.cfg - Physics Objects Information
- The Modular Pneumatics System Information has been updated to add the `Volume` key to most component hashmaps.
- The Panel XML Properties page has been updated with new element tags related to the EFB and flight performance data.
- The gameplay.cfg has had new tailwheel elements added to the `inhibitElement` parameter.
- The navigation_graph.cfg has a new section for defining airliner decks: `[Decks.N]`
- New page on how to setup Aircraft Loads has been added.
- The loads.lbl file has been added to the documentation.
- The following model behaviour documentation has been updated:
    -   `FX_WaterDrop_LowSpeed` - A new graph parameter has been added to control water direction.
    -   The `SonicBoom` section has been updated to use the new MSFS 2024 version of the effect.


{{< release-notes-tag "improved" >}}

- The `<Start />` element section of the Airport XML Properties reference page has been updated.
- The Notes On Tailwheels section has been updated to reflect changes with the way tailwheel calculations are performed.
- The pages on Modular Pneumatics System Information and Pneumatic System Examples have been updated to fix incorrect information about some hashmap keys and the examples have been made more generic.
- The flight_model.cfg section `[OBJ_EA1_PITOTFLAG.N]` section has had the following parameter descriptions updated: `surface_relative_position.N`, `surface_angle.N`
- Various sections of the navigation_graph.cfg have been updated with parameters related to the new `[Decks.N]` section.
- The section on Testing on the Commercial Flights And Passengers page has been updated with more precise information for testing passenger spawning.
- The page on Modular SimObject Project Structure has been updated to include the `loads` folder.
- The page on SimPropContainer XML Properties has been updated with missing information.
- Multiple CFG file pages have been updated to show exactly what can be edited "live" in the simulation using Live Edition in the SimObject editor.


{{< release-notes-tag "fixed" >}}

- The panel.cfg parameter `backlight` has been updated to reflect more accurately the way the parameter works.
- The engines.cfg parameter `engine_type` now correctly lists all available engine types (it was missing *electric* engines).
- The Version parameter for the modern fuel system has been fixed to show correct version information.
- The aircraft.cfg section on loading tips/images has been moved to CFG Files Obsolete Parameters as it is not used in MSFS 2024.


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "added" >}}

- New SimVars added to the documentation:
    - Aircraft Control Variables: `ELEVATOR TRIM LIMIT RATIO`
    - Aircraft Brake/Landing Gear Variables: `CONTACT POINT STEER ANGLE`, `CONTACT POINT STEER ANGLE PCT`
    - Aircraft Fuel Variables: `FUELSYSTEM TANK USABLE CAPACITY`
    - Aircraft Engine Variables: `ENG TILT PITCH PERCENT EX1`, `ENG TILT YAW PERCENT EX1`
    - Aircraft Fuel Variables: `FUEL TOTAL QUANTITY EX1`, `FUEL TOTAL QUANTITY WEIGHT EX1`
    - Aircraft System Variables: `PNEUMATICS COMPONENT TEMPERATURE`, `PNEUMATICS COMPONENT NUMBER OF MOLES`, `PNEUMATICS COMPONENT PRESSURE`, `PNEUMATICS COMPONENT VOLUME`
    - Aircraft Misc. Variables: `LIVERY NAME`, `LIVERY FOLDER`
    - Helicopter Variables: `ROTOR RADIUS`


{{< release-notes-tag "improved" >}}

- Further improvements to the Reverse Polish Notation documentation to better explain the `flr` and `int` operators.
- The Creating A WASM Project page has had the screenshots updated to match the current state of the WASM SDK Visual Studio templates.
- The Runway `surface` type enum has been updated on the SimConnect_AddToFacilityDefinition page.


{{< release-notes-tag "fixed" >}} 

- Wrong link on the `SimConnect_SubscribeToSystemEvent` page has been fixed.
- The VOR `type` enum has been fixed to show the correct values on the SimConnect_AddToFacilityDefinition page.
- The syntax for `fsVarsNamedVarGet` and `fsVarsNamedVarSet` has been fixed to show the correct naming.

{{< /expand >}}



{{< expand title="SDK Release 1.2.4" >}}

<p class="fake-h3">SDK Changes</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "fixed" >}}

- Fixed a bug that the first call to CommandHandler is ignored (trigger_key_event(_EX1) for instance)


<p class="fake-h4">Tools</p>

{{< release-notes-tag "fixed" >}}

- Blender: Fixed "clearcoatNormalTiling" extension


<p class="fake-h3">Documentation</p>

<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "added" >}}

- A new page about development guidelines to set up aircrafts for career mode has been added to the documentation : General Career Mode Requirements


<p class="fake-h4">DevMode</p>

{{< release-notes-tag "added" >}}

- A new section about setting up Marketplace assets has been added to the package export documentation: Marketplace Requirements


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "fixed" >}}

- The page on RPN has been updated to correct capitalisation issues with some keywords.


{{< /expand >}}



{{< expand title="SDK Release 1.2.3" >}}

<p class="fake-h3">Developer Mode Changes</p>

<p class="fake-h4">Scenery Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed "Start" element of type "RUNWAY" unsupported in airport scenery files.
- Fixed incorrect terraforming falloff in FS2020 packages and in the editor.
- Fixed object not displayed when an airport light uses either the FakeBingTerrain material code or the geodecal material code.

{{< release-notes-tag "improved" >}}

- "Start" element of type "RUNWAY" now supported for backward compatibility and also converted to actual runway starts upon loading in the Scenery Editor.


<p class="fake-h3">SDK Changes</p>

<p class="fake-h4">Tools</p>

{{< release-notes-tag "fixed" >}}

- Blender: Fix set default values for existing materials in scene and fix materials properties
- Blender: Fix windshield properties declaration
- Blender: Fix max and default values for material ghost
- Blender: Fixed decal extension export
- Blender: Fix "Inverse roughness" parameter on clearcoat material type
- 3DS Max: Fix max and default values for material ghost
- 3DS Max: Fix remove IBL Menu when SDK 2020 is installed
- fspackagetool: Fix issues when using the Steam version of the sim

{{< release-notes-tag "improved" >}}

- Blender: Update SSS color (4.2 LTS)
- Blender: Set "Export Light" to True by default
- Blender: Remove "Wear overlay scale" property from sail, tree and vegetation material types

{{< release-notes-tag "added" >}}

- Blender: Added "unique_id" to neutral_bone added by Khronos exporter


<p class="fake-h3">Documentation</p>

<p class="fake-h4">DevMode</p>

{{< release-notes-tag "added" >}}

- A new section about setting up Marketplace assets has been added to the package export documentation: Marketplace Requirements


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "fixed" >}}

- The page on RPN has been updated to correct capitalisation issues with some keywords.

{{< /expand >}}



{{< expand title="SDK Release 1.2.2" >}}

<p class="fake-h3">Developer Mode Changes</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "fixed" >}}

- Fixed VFS Projector randomly hanging when opening files.

  

<p class="fake-h4">Project Editor</p>

{{< release-notes-tag "fixed" >}}

- Fix misleading error 'Cannot open layout.json' at the beginning of package build inside project editor.
- Fixed Export Items window always exporting all packages from the project instead of only the selected ones.
- Fixed incorrect value for month in revision notes

  

<p class="fake-h4">Scenery Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed build error when a scenery object has both "NoSnow" and "SnapToBuilding" flags.
- Fixed backward compatibility with dirt on taxiway parkings.
- Fixed incorrect position of building exclusion polygon around taxiway parkings.
- Fixed taxiway parkings building exclusion not updating during edition in Scenery editor.
- Fixed multi edit not working properly for some properties (ex: parking headings).
- Fixed missing shader when an airport light use the material code FakeBingTerrain or geodecal.
- Fixed taxiway signs too bright. Expose the parameter in airport archetype.

{{< release-notes-tag "improved" >}}

- Removed runway landing tire gum for 2020 packages.

{{< release-notes-tag "added" >}}

- Added option in the exclusion rectangles to remove single trees.

  

<p class="fake-h4">SimObject Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed parsing legacy hashmap parameter.


<p class="fake-h3">SDK Changes</p>

<p class="fake-h4">Aircraft</p>

{{< release-notes-tag "fixed" >}}

- Fixed an issue causing some aircrafts to start with 0 fuel.
- Fixed helicopter CFD / rotor wash aerodynamics issues on edges of platforms.

  

<p class="fake-h4">Samples</p>

{{< release-notes-tag "fixed" >}}

- Fixed memory corruption in the Network Aircraft preset of the Wasm Aircraft that could lead to the module crashing.

  

<p class="fake-h4">SimConnect</p>

{{< release-notes-tag "added" >}}

- Added new specific object IDs to request data for Aircraft, Avatar or Current user object: SIMCONNECT_OBJECT_ID_USER_AIRCRAFT, SIMCONNECT_OBJECT_ID_USER_AVATAR, SIMCONNECT_OBJECT_ID_USER_CURRENT.

  

<p class="fake-h4">SimVars</p>

{{< release-notes-tag "fixed" >}}

- Fixed PRESSURIZATION_CABIN_ALTITUDE_GOAL simvar which incorrectly indicated the current altitude.
- Fixed SimVars ZULU DAY OF WEEK, ZULU DAY OF MONTH, ZULU MONTH OF YEAR, ZULU DAY OF YEAR, ZULU YEAR.

  

<p class="fake-h4">WASM API</p>

{{< release-notes-tag "fixed" >}}

- Fixed Wasm System runing in main menu after a flight
- Fixed potential crashes (buffer overflow) when using the Network API to send a PUT HTTP request that returns a body
- Fixed VFX API spawning effects on Avatar when in walkaround mode
- Fixed wasm module not always reloaded on restart
- Fixed not all gauge reload sometimes on restart
- Fixed crash in wasm allocator causing random dirty on allocation
- Fixed a crash in wasm when requesting ICAO in some context
- Fixed Wasm module dirty status cleaning not cleaning when restart a Wasm that crash during kill callback
- Fixed read from work folder when using Fs IO API
- Fixed the WASI fd_readdir function - it now lists (in a non-recursive way) files that match the provided VFS path and belong to the same package as the WASM module
- Fixed random locks when using the Network extension

{{< release-notes-tag "added" >}}

- Added Write function in Wasm Fs IO API

  

<p class="fake-h4">JS API</p>

{{< release-notes-tag "fixed" >}}

- B:Events never received when called only from JS

  

<p class="fake-h4">Tools</p>

{{< release-notes-tag "fixed" >}}

- Fixed Material UV animation

    Blender: Fix alpha mode for Blender 4.2LTS

    Blender: Fix "Reload LODS" for grouped by collection

    Blender: Fix light intensity and light collection when opening a scene

    Blender: Fix lights conversion

    Blender: Fix glass width conversion

    Blender: Fix Blender plugin version

    Blender: Fix Rain options

    Blender: Fix windshield extension

    Blender: Fix set freeze factor to default when we re-open a saved blend scene

    Blender: Fix maximum values for clear coat normal tiling and clear coat roughness


improvedu&gt;

- Blender: Update plugin for 4.2 LTS
- Blender: Remove wear overlay UV scale from propeller material

  


<p class="fake-h3">Documentation</p>

<p class="fake-h4">Developer Mode</p>

{{< release-notes-tag "added" >}}

- New **Force Elevation** option documented for Polygon Objects and Rectangle Objects.
- New **Draw Dirt** option added to the TaxiwayParking Object docs.
- New **Instancing** option added to the Scenery Objects docs.
- New Taxiway Sign Airport Archetype Override added to the docs.

{{< release-notes-tag "improved" >}}

- The Package Order Hint section of the Project Editor inspector page has been updated.
- The Runway Object material documentation has been updated to reflect changes in the application of custom materials.


<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "added" >}}

- New section for airships added to the `flight_model.cfg` documentation: [AIRSHIP_SYSTEM]
- New section added to the FLT documentation: [Airship_System.N]
- New pages have been added to the model behaviour documentation:
    - Adding Sound To Interactions
    - Advanced Interactions
    - Adding Aircraft VFX
    - VFX Common XML Properties
    - Visual Effects Templates
    - VFX Template Examples
- The cockpit.cfg has the following new parameters: `hud_show_scoop_indicator`, `hud_show_spray_indicator`,

{{< release-notes-tag "improved" >}}

- The [AUTOPILOT] section of the `systems.cfg` file has been cleaned of obsolete parameters that are no longer used in the sim. All other parameters in that section have been updated with improved descriptions and default values.
- The Career documentation for Agricultural Aviation has been updated with improved information on the VFX setup.


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "added" >}}

- New key events added to documentation:
    - Aircraft Electrical Events: `LIGHT_AMBIENT_COLOR_SET`,
    - Balloon / Airship Specific Events: `AIRSHIP_VALVE_1_CLOSE`, `AIRSHIP_VALVE_1_OPEN`, `AIRSHIP_VALVE_1_SET`, `AIRSHIP_VALVE_1_TOGGLE`, `AIRSHIP_VALVE_2_CLOSE`, `AIRSHIP_VALVE_2_OPEN`, `AIRSHIP_VALVE_2_SET`, `AIRSHIP_VALVE_2_TOGGLE`, `AIRSHIP_VALVE_3_CLOSE`, `AIRSHIP_VALVE_3_OPEN`, `AIRSHIP_VALVE_3_SET`, `AIRSHIP_VALVE_3_TOGGLE`, `AIRSHIP_VALVE_4_CLOSE`, `AIRSHIP_VALVE_4_OPEN`, `AIRSHIP_VALVE_4_SET`, `AIRSHIP_VALVE_4_TOGGLE`, `AIRSHIP_VALVE_CLOSE`, `AIRSHIP_VALVE_OPEN`, `AIRSHIP_VALVE_SET`, `AIRSHIP_VALVE_TOGGLE`,
- New SimVars added to the documentation:
    - Aircraft System Variables: `LIGHT AMBIENT COLOR START RGBA`, `LIGHT AMBIENT COLOR START R`, `LIGHT AMBIENT COLOR START G`, `LIGHT AMBIENT COLOR START B`, `LIGHT AMBIENT COLOR START A`, `LIGHT AMBIENT COLOR END RGBA`, `LIGHT AMBIENT COLOR END R`, `LIGHT AMBIENT COLOR END G`, `LIGHT AMBIENT COLOR END B`, `LIGHT AMBIENT COLOR END A`,
    - Balloon / Airship Variables: `BALLOON FILL AMOUNT`, `AIRSHIP COMPARTMENT GAS TYPE`, `AIRSHIP COMPARTMENT OVERPRESSURE`, `AIRSHIP COMPARTMENT PRESSURE`, `AIRSHIP COMPARTMENT TEMPERATURE`, `AIRSHIP COMPARTMENT VOLUME`, `AIRSHIP COMPARTMENT WEIGHT`, `AIRSHIP DAMPER STATE`, `AIRSHIP FAN POWER PCT`, `AIRSHIP VALVE STATE`, `GROUND ATTACHMENT`, `MAST TRUCK DEPLOYMENT`, `MAST TRUCK EXTENSION`,
    - Aircraft Electrics Variables: `ELECTRICAL RELAY LINE OPEN`, `ELECTRICAL RELAY POWERED`, `ELECTRICAL RELAY BREAKER PULLED`, `ELECTRICAL RELAY CONNECTION ON`, `ELECTRICAL RELAY AMPS`, `ELECTRICAL RELAY VOLTAGE`,
    - Aircraft Engine Variables: `ENG TILT PITCH`, `ENG TILT PITCH PERCENT`, `ENG TILT YAW`, `ENG TILT YAW PERCENT`,

{{< release-notes-tag "fixed" >}}

- Multiple SimVars that were previously labelled as "legacy" have now been correctly labelled as "obsolete" for MSFS 2024.
- The text for the example shown on the page about `fsVarsGetAircraftVarId` has been fixed to show the correct return values.

{{< /expand >}}



{{< expand title="SDK Release 1.2.0" >}}

<p class="fake-h3">Developer Mode Changes</p>

<p class="fake-h4">Tools</p>

{{< release-notes-tag "fixed" >}}

- Fixed VFS Projector randomly hanging when opening files


<p class="fake-h4">Project Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed incorrect value for month in revision notes

<p class="fake-h4"> SimObject Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed parsing legacy hashmap parameter


<p class="fake-h3">SDK Changes</p>

<p class="fake-h4">Tools</p>

{{< release-notes-tag "improved" >}}

- Blender: Update plugin for 4.2 LTS


{{< release-notes-tag "fixed" >}}

- Blender: Fix "Reload LODS" for grouped by collection
- Blender: Fix light intensity and light collection when opening a scene


<p class="fake-h4">SimVars</p>

- Fixed SimVars ZULU DAY OF WEEK, ZULU DAY OF MONTH, ZULU MONTH OF YEAR, ZULU DAY OF YEAR, ZULU YEAR

<p class="fake-h4"> WASM API</p>

{{< release-notes-tag "fixed" >}}

- Fixed Wasm module dirty status cleaning not cleaning when restart a Wasm that crash during kill callback
- Fixed read from work folder when using Fs IO API
- Fixed the WASI fd_readdir function - it now lists (in a non-recursive way) files that match the provided VFS path and belong to the same package as the WASM module
- Fixed random locks when using the Network extension
- Fixed crash in wasm allocator causing random dirty on allocation
- Fixed a crash in wasm when requesting ICAO in some context
- Fixed Wasm module dirty status cleaning not cleaning when restart a Wasm that crash during kill callback

{{< release-notes-tag "added" >}}

- Added Write function in Wasm Fs IO API


<p class="fake-h4">JS API</p>

{{< release-notes-tag "fixed" >}}

- B:Events never received when called only from JS


<p class="fake-h3">Documentation</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "improved" >}}

- The Content Creator Testing Tool docs have been updated with information on the WASM options for XBox.
- The main Developer Mode page has been updated with information on the **Quick Menu** available using {{< input "Ctrl" />}} + {{< input "Space" />}}.


<p class="fake-h4">Developer Mode</p>

{{< release-notes-tag "added" >}}

- New **Draw Dirt** option added to the TaxiwayParking Object docs.
- New **Force Elevation** option documented for Polygon Objects and Rectangle Objects.
- New **Exclude Secondary Heightmaps** option for Polygon Objects documented.
- New **Rocks** options for Polygon Objects documented.
- New **Draw Dirt** option added to the TaxiwayParking Object docs.
- New **Instancing** option added to the Scenery Objects docs.
- New Tree Object documented in as part of the Scenery Editor.
- The scenery editor Airport Objects page has been updated with new information about airport archetypes.
- New RMB option to **Select Similar Objects** added to the documentation.

{{< release-notes-tag "improved" >}}

- The main Scenery Editor pages have all had a refresh to reflect changes to the editor.
- The scenery editor Airport Objects page has been updated with information on the WASM property.
- The WASM Debug has been improved with updated information and images.

{{< release-notes-tag "fixed" >}}

- Small typo fixed on The Virtual File System page.
- The texture resolution notification on various scenery object pages has been fixed to show the correct value (4mm/pixel).


<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "added" >}}

- New parameter added to the flight_model.cfg `[AERODYNAMICS]` section: `normalizationmethod`
- New sections and parameters added in the flight_model.cfg: [OBJ_EA1_FUSELAGE.N]
- New information section added to the flight_model.cfg - Additional Information file to help understand the new object surface physics.
- New parameter added to the systems.cfg `supplier.N` hashmap: `TensionDropRPM`
- New parameter added to the attached_objects.cfg `[SIM_ATTACHMENT.N]` section: `attachment_guid`

{{< release-notes-tag "improved" >}}

- The Airport XML Properties page has updated information on the `<WasmModule>` element.
- The Airport Archetype Overrides page has been refreshed with updated information and improved screenshots.


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "added" >}}

- New WebAssembly section for Creating WASM Airports.
- New Planned Route API documentation.

{{< release-notes-tag "improved" >}}

- The page "Platform Toolset" in the WASM section has been removed as it only contained duplicate information found elsewhere.
- The Electronic Flight Bag API docs have been updates with a "Quick start guide" and a section on the available environment variables.
- The `CAMERA STATE` SimVar docs have been updated to reflect changes.

{{< release-notes-tag "fixed" >}}

- Added missing parameter to the SimConnect_FlightSave


<p class="fake-h4">Samples And Tutorials</p>

{{< release-notes-tag "added" >}}

- New SimpleWasmAirport page added.
- Multiple stub pages added in preparation for future updates.

{{< release-notes-tag "improved" >}}

- The "Scenery Model Samples" page has been removed, since it is irrelevant given the changes to the simulation and the fact that the samples it references are no longer a part of the SDK.
- The Creating WASM Systems page has had a minor update to clarify certain points and add in a bit more information for context.

{{< /expand >}}



{{< expand title="SDK Release 1.1.2" >}}

<p class="fake-h3">SDK Changes</p>

<p class="fake-h4">Tools</p>

{{< release-notes-tag "added" >}}

- Blender: Export as submodel

{{< /expand >}}



{{< expand title="SDK Release 1.1.1" >}}

<p class="fake-h3">Developer Mode Changes</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "fixed" >}}

- Fixed "Exit devmode" option always greyed out.

- Fixed and improved scenery package order.

- Fixed layout issues in editor and tool windows when using a 4k monitor

- Fixed a crash that could occur when closing devmode windows from Windows (e.g. with alt+F4)

- Fixed texture paths returned in Package Builder messages (was returning a hash instead of the real path)

{{< release-notes-tag "added" >}}

- VFSProjector now grants access to encrypted files that are not protected.


<p class="fake-h4">Scenery Editor</p>
Note that if you already opened and saved your scenery in MSFS 2024, the conversion won't be done again and you might need to update it by yourself. If no other changes were made to that scenery, you can also re-import the package to get this done.

{{< release-notes-tag "fixed" >}}

- Fixed material polygon not rendered.
- Fixed jetway link sometimes relinking to incorrect altitude when moving the jetway.
- Fixed light too bright on light rows without preset.
- Fixed secondary heightmap not flattened when vegetation exclusion is disabled on a runway.
- Fixed crash when generating sim object list.
- Fixed crash when moving an invalid world script.
- Fixed projected mesh flagged as marking text not drawn after other markings.
- Fixed scenery editor scaling to look better an higher resolutions.
- Fixed a conversion issue on carparking heading with packages imported from FS2020.
- Prevent material discontinuity on aprons.

{{< release-notes-tag "improved" >}}

- Improved scenery list generation time by creating a scenery name list with the PackageCompressor.

- Clarified how polygon\\'s vegetation density slider works, with an info message when graphics settings aren't high enough to see all trees.

- Selected all packages by default in the Objects panel.

- Moved the "Replace with..." buttons of scenery/sim objects/simprop containers inside a single button.

- Removed taxiway dirt from legacy airports archetype. There is still unwanted dirt near parkings in legacy packages.


<p class="fake-h4">SimObject Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed possible crash when using Save and Apply near an airport.
- Fixed stringlist parsing error due to unwanted space between values.
- Fixed hashmap parsing adding unwanted unknown param.
- Fixed modified state in camera cfg tab.
- Fixed excluded param incorrectly parsed during the merge.
- Fixed typo in career compatibility tab.

{{< release-notes-tag "added" >}}

- Add live edition option for contact points position.


<p class="fake-h3">SDK Changes</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "fixed" >}}

- Fix limit on maximum number of terrain instances. Separate terrain from EFB/Minimap to let developers use the 9 available maps.
- Fixed XML gauge conditional text color not working for some expressions.
- Fixed scenery VFX objects not spawning their effect.
- Fixed WASM gauge loaded even if the aircraft referencing it is in a different package.
- Fixed windsock ModelBehavior template to keep wind direction no matter what the object rotation is.


<p class="fake-h4">Audio</p>

{{< release-notes-tag "added" >}}

- Added support for an optional "AbsoluteValue" attribute on WwiseRTPC tags in sound.xml/soundai.xml files, it expects a boolean value and tells whether to use the absolute value of the variable


<p class="fake-h4">Samples</p>

{{< release-notes-tag "added" >}}

- Added a new sample that shows how to mod an existing aircraft.
- Added a sample to show how to use new function : `SimConnect_EnumerateSimObjectsAndLiveries`.
- Added new blend scenes for DA62 sample.


<p class="fake-h4">SimConnect</p>

{{< release-notes-tag "fixed" >}}

- Fixed Aircraft spawning on the same parking slot when spawned with SimConnect.

{{< release-notes-tag "added" >}}

- Added extended functions to SimConnect to specify livery when spawning a SimObject.
- Implemented `SimConnect_EnumerateSimObjectsAndLiveries` function that returns a list a spawnable simbobjects and their liveries.

{{< release-notes-tag "improved" >}}

- Removed DoneIO popup.  
    Brought `SIMCONNECT_SIMOBJECT_TYPE` back to its previous order and added new values at the end.


<p class="fake-h4">SimVars</p>

{{< release-notes-tag "added" >}}

- Added "LIVERY FOLDER" and "LIVERY NAME" SimVars for modular aircraft liveries.

{{< release-notes-tag "fixed" >}}

- Fixed ZULU time


<p class="fake-h4">WASM API</p>

{{< release-notes-tag "fixed" >}}

- Fixed a bug where WASM Vars and Events APIs where not usable in standalone or airport modules

{{< release-notes-tag "added" >}}

- Added WASM build in Release With Debug Info detection in the WASM Debug Window
- Added access to environment var in vars extension


<p class="fake-h4">EFB API</p>

{{< release-notes-tag "improved" >}}

- Updated efb_api to the latest version.


<p class="fake-h4">Tools</p>

{{< release-notes-tag "fixed" >}}

- Blender: Fix export lights
- Blender: Fix export material animations
- Blender: Fixed export objects
- Blender: Fixed set Blend Mask Texture

{{< release-notes-tag "added" >}}

- Blender: Added Tire material
- Blender: Add enable/disable export meshs

updated

- Blender: Adjustments of standard shader : Rename uv2 input and Unlink AO when there are no base color texture of detail color texture set
- Blender: Clarify use of material_type update function
- Blender: Update default emissive scale


<p class="fake-h3">Documentation</p>

<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "added" >}}

- New attribute for the `<WwiseRtpc>` element of the sound XML has been added: `AbsoluteValue`


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "added" >}}

- New WebAssembly section dedicated to the IO API.
- New WebAssembly section for MSFS core: Core And Helpers
- New SimConnect functions documented:
    - SimConnect_AICreateEnrouteATCAircraft_EX1
    - SimConnect_AICreateNonATCAircraft_EX1
    - SimConnect_AICreateParkedATCAircraft_EX1
    - SimConnect_AICreateSimulatedObject_EX1
    - SimConnect_EnumerateSimObjectsAndLiveries

{{< /expand >}}



{{< expand title="SDK Release 1.0.1" >}}

<p class="fake-h3">SDK Changes</p>

<p class="fake-h4">Plugins</p>

{{< release-notes-tag "fixed" >}}

- Blender: Fixed export objects
- Blender: Fixed set Blend Mask Texture

{{< release-notes-tag "added" >}}

- Blender: Added Tire material


<p class="fake-h4">Samples</p>

{{< release-notes-tag "added" >}}

- New sample to show how to mod an existing modular aircraft.


<p class="fake-h3">Documentation</p>

<p class="fake-h4">Developer Mode</p>

{{< release-notes-tag "improved" >}}

- Debug Aircraft Flight Performance window page has been updated.


<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "added" >}}

- New page flight_performance.cfg - Additional Information added.
- New [Deck.N] section added to the navigation_graph.cfg.

{{< release-notes-tag "improved" >}}

- Finalised the documentation for the flight_performance.cfg file.



<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "added" >}}

- The SimVar GPS GSI NEEDLE has been added.
- New page on EFB Notifications has been added.
- New section in the Panel XML docs to explain the &lt;EFB&gt; element.

{{< release-notes-tag "improved" >}}

- The entire TCAS section of the SimVars docs has been updated.
- All SimVar docs pages have been updated with more obvious parameter descriptions and requirements.
- The main Simulation Variables page has improved description of RPN and SimVars with two parameters.

{{< /expand >}}



{{< expand title="SDK Release 1.0.0 (MSFS Initial Public Release)" >}}

<p class="fake-h3">Developer Mode Changes</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "fixed" >}}

- Fixed Aircraft Selector/Configurator Variation/Livery handling for MSFS 2020 packages
- Adaptive detail LOD factor does not affect User aircraft anymore

{{< release-notes-tag "added" >}}

- New Content Creator Testing Tool has been added to permit registered partners to test streamed packages.


<p class="fake-h4">Scenery Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed projected mesh size not taken into account for mip streaming causing blurry projected meshes.


<p class="fake-h4">SimObject Editor</p>

{{< release-notes-tag "improved" >}}

- Changed the way tailwheel is computed in the Career Compatibility Tab

{{< release-notes-tag "added" >}}

- target_specializations as a constraint which is shown in the output of the Career Compatibility Tab


<p class="fake-h3">SDK Changes</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "fixed" >}}

- Fixed XML gauges not working after first flight.
- Fixed bitmaps not working anymore in XML gauges

{{< release-notes-tag "improved" >}}

- Hide avatar in aircraft if it's not edited for SR pilot system
- The Wear&Tear system is now deactivated on 2020 aircraft


<p class="fake-h4">SimVars</p>

{{< release-notes-tag "fixed" >}}

- Fixed "SIMULATION TIME" environment variable returning inconsistent values when switching from/to avatar mode


<p class="fake-h4">Tools</p>

{{< release-notes-tag "fixed" >}}

- 3dsmax: Fixed flags in texture tool
- Blender: Fixed reload lods for collections

{{< release-notes-tag "improved" >}}

- Blender: Added flags edition to texture
- Blender: Update texturelib generation


<p class="fake-h3">DOCUMENTATION</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "added" >}}

- Dark/Light mode has been added to the documentation to improve accessibility.
- New page added to give a reference for the The Colour Picker.

{{< release-notes-tag "improved" >}}

- The structure of the SDK documentation has been improved to enable finding the information you require faster and easier, including improved search, an improved index, and related topics links on all pages.
- SDK Tools have been moved into their own unique section in the Table Of Contents and all relevant information consolidated under that section.
- Aircraft samples have been converted into the following two modular aircraft:
    -   SimpleAircraft
    -   WASMAircraft


<p class="fake-h4">SDK Tools</p>

{{< release-notes-tag "added" >}}

- Documentation has been added to describe the new Input Application for creating custom input profiles for devices and aircraft. This includes the following pages:
    - Global Input Profile Package Setup  
        Aircraft Input Profile Package Setup  
        Creating An Input Profile  
        DeviceConfig XML Properties  
        Input Configuration XML Properties

- A page has been added for The Mission Tool. This tool is a temporary fix to allow developers to test certain aspects of mission creation and will be removed in future versions of the Microsoft Flight Simulator 2024 DevMode.


<p class="fake-h4">Developer Mode - General</p>

{{< release-notes-tag "added" >}}

- A new option for Lighting Display debugging has been documented.
- The DevMode Toolbar has been documented.
- The Behaviors Debug window has been documented

{{< release-notes-tag "improved" >}}

- The "Debug PBR" menu page has been renamed as Channel Display and has had some additional updates to cover new options.
- Some menus can now be "detached" so you can easily access common options. See here for more information: Detachable Menus
- The "Debug PBR" menu page has been renamed as Channel Display and has had some additional updates to cover new options.
- The **Aircraft Editor** section has been removed and replaced by the more general SimObject Editor.
- The section on using the Screenshot tool has been updated.
- The Developer Camera Settings section has been updated with new options.


<p class="fake-h4">The Project Editor</p>

{{< release-notes-tag "added" >}}

- The new Package States have been documented.
- New Package Order Hint section has been added to The Project Inspector page.

{{< release-notes-tag "improved" >}}

- The Asset Types page has been tidied and now shows information for all relevant asset types available, including the new Airport asset group type.
- All references to the setup for the Marketplace have been removed from the documentation as this is now all handled on the Marketplace itself.


<p class="fake-h4">The Scenery Editor</p>

{{< release-notes-tag "added" >}}

- Airport Objects - Override Archetypes options have been added.
- PaintedLine Objects - New option for outlines has been added.
- Decal Objects - New decal object for the Scenery Editor has been documented.
- TaxiwayServiceStand Objects - New taxiway service stand object has been documented.
- TextMarking Objects - New text object for the scenery editor has been documented.
- VDGS Objects - New Visual Docking Guidance System (VDGS) object for the scenery editor has been documented.
- SimPropContainer Object - New scenery object type has been added. See also: Modeling SimPropContainers
- SimPropContainer XML Properties and SimPropContainer Object Examples have been added to describe the XML used by the new SimPropContainer object.
- New "Apron Services" option has been documented for Airport Objects, TaxiwayParking Objects and Helipad Objects.
- New Navigation Graph options have been documented for The View Menu.
- New render options have been added to The Rendering Menu page.

{{< release-notes-tag "improved" >}}

- Helipad Objects - Multiple new options for helipads have been documented.
- TaxiwayParking Objects - Multiple new options for taxiway parking objects have been documented.
- Information has been added to the Creating A SimObject and Creating A Scenery Object pages.
- Information has been added to the Scenery XML And Model CFG page.


<p class="fake-h4">The Material Editor</p>

{{< release-notes-tag "added" >}}

- New page on The Bitmap Manager has been added.

{{< release-notes-tag "improved" >}}

- The Material Inspector page has been updated with new "extra settings" section.


<p class="fake-h4">The SimObject Editor (Previously the Aircraft Editor)</p>

{{< release-notes-tag "added" >}}

- The SimObject Editor main page has been added withinformation about the new editor
- The Attached Objects tab has been documented.
- The Attachment tab has been documented.
- The Navigation Graph tab has been documented.
- The Livery tab has been documented
- The Effects tab has been documented
- The Texture tab has been documented
- The Model LOD tab has been documented.
- The Animations tab has been documented.
- The Sounds tab has been documented.
- A dedicated page listing all SimObject Editor parameters have been added: The SimObject Editor Parameter Index
- The new debug window - Debug Dirt And Scratches - has been added to the documentation.
- The new debug window - Covers And Chocks - has been added to the documentation.
- The new debug window - Debug Aircraft Attachments - has been added to the documentation.

{{< release-notes-tag "improved" >}}

- The Aircraft Data debug window docs have been updated to reflect additions to the window.


<p class="fake-h4">The Navigation Graph Editor</p>

{{< release-notes-tag "added" >}}

- New page describing The Navigation Graph Editor has been added. Tis editor is for desiging **navigation graphs** which are used to help objects navigate around both the world and aircraft, as well as many other things.
- New page describing Using The Navigation Graph Editor has been added.


<p class="fake-h4">The SimAttachment Editor</p>

{{< release-notes-tag "added" >}}

- The SimAttachment Editor has been added to the documentation. This editor is similar to the SimObject Editor, but designed for editing stand-along attachments that can then be added to your aircraft and scenery.


<p class="fake-h4">Models And Textures</p>

{{< release-notes-tag "added" >}}

- The new Texture Tool has been documented.
- New pages have been added for the Texture XML Properties and texture.json files.
- A page has been added explaining how to use the Simplygon SDK to simplify the creation of scenery object LODs: Using Simplygon To Generate LODs
- A page has been added to explain the updated LOD Selection System for all 3D models.
- The Modeling Aircraft documentation has had the following pages added:
    - Propellers, Turbines And Blades  
        Clearcoat  
        Dirt And Grime  
        Registration Numbers  
        Covers, Chocks, And Pins  
        Doors, Hatches, And Canopies  
        Fuel Truck And GPU Connections

{{< release-notes-tag "improved" >}}

- FlightSim Materials (all material documentation has been updated to the new MSFS 2024 standards)
- The glTF Schemas page has been updated with information on the new schemas available.
- The Multi-Exporter documentation has received a major update.
- The Modeling Aircraft documentation has had the following pages updated:
    - File Setup  
        Airframe Textures And Materials  
        Windshield And Windows  
        Exterior Lights  
        Wheels  
        Airframe Details  
        Ice  
        Airframe Collision Meshes  
        Ambient Occlusion  
        Parallax Windows  
        Aircraft LODs


<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "added" >}}

- New section outlining details of the updated MSFS Avionics Framework for MSFS 2024 has been added.
- CFG Files General Information page has been added, and now contains updated information about the different **data types**, including the new table format that permits *n*D tables.
- New Simple SimObject types have been documented:
    - Procedural Characters  
        Simple Objects
- New pages have been added related to the setup for Modular Aircraft:
    - Modular Aircraft Project Structure  
        Modular Aircraft Merging  
        Modular Aircraft XML Properties  
        Modular Aircraft CFG Files  
        Sim Attachments  
        Included Attachments
- New pages have been added related to the configuration of models within the simulation:
    - Livery XML Properties
- New pages have been added to describe the setup of light effects:
    - Implementing Lights  
        Light FX Properties  
        effects.cfg
- A new section has been added for Navigation Services And Interactions:
    - Services XML Properties  
        Navigation Services XML Properties  
        SimMission Navigation Services  
        Interaction XML Properties  
        Interaction Presets
- A new section has been added for Apron Services:
    - Apron Control XML Properties  
        Apron Services XML Properties  
        Apron Control Mission XML Properties  
        Built-In Apron Services  
        Global Apron Services
- A new Careers section has been added outlining the setup for custom aircraft to be used in the various career missions. This section covers the following:
    - Preflight  
        Spawn Pilot (Transversal)  
        Aerial Advertising  
        Aerial Construction  
        Aerial Firefighting  
        Agricultural Aviation  
        Cargo Transport  
        Charter Service  
        Commercial Flights And Passengers  
        Ferry Flights  
        Flightseeing / First Flight  
        Medevac  
        Scientific Research  
        Search And Rescue  
        Skydiving  
        Using Included SimAttachments
- The page for the `<SimMission.Calculator>` has new items listed specifically for use with Navigation Services, Apron Services, and Interactions:
    - &lt;CabinServiceBehaviourParameter&gt;  
        &lt;CabinServiceStateParameter&gt;  
        &lt;CabinServiceGraphParameter&gt;  
        &lt;ParentCabinServiceBehaviourParameter&gt;  
        &lt;ParentCabinServiceStateParameter&gt;  
        &lt;ApronContextParameter&gt;  
        &lt;ApronControlMissionContext&gt;  
        &lt;InteractivePointParameter&gt;  
        &lt;InteractivePointUser&gt;  
        &lt;InteractivePoint&gt;  
        &lt;InteractivePointType&gt;  
        &lt;InteractivePointSearchPreferences&gt;  
        &lt;ParkingParameter&gt;  
        &lt;ParkingName&gt;  
        &lt;ParkingNumber&gt;  
        &lt;ParkingSuffix&gt;  
        &lt;StandType&gt;  
        &lt;StandRole&gt;  
        &lt;HelipadParameter&gt;  
        &lt;HelipadProperty&gt;  
        &lt;DesiredValue&gt;  
        &lt;Units&gt;
- FLT file documentation has been updated with a new page for FLT Additional Information and the following new sections:
    - `[HYDRAULICS_SYSTEM_EX1.N][PNEUMATIC_SYSTEM_EX1.N][Liquid Dropping System.N][Gauges.N][Navigation_Graph.N][WEAR_AND_TEAR_SYSTEM.N][CabinService.N][CabinServiceObject.N][Dirt.N]`
- The FLT file documentation also has the following new parameters:
    - `[Systems.N]` - `GovernorTarget[Covers]` - `chock`, `engine`, `pitot`, `static_port`, `rotor`, `landing_gear[Sim.N]`: `Livery`,
- New section on Changes Between MSFS 2020 And MSFS 2024
- The Checklist XML Properties page has the following new elements: `<Link>`,
- New page that includes a detailed overview of the new features of the MSFS 2024 audio system has been added: Audio Migration And Updating
- The page related to the Localization (LOC Files) has a new section outlining Supported Languages.
- New CFG file for dealing with **navigation graphs** has been documented: `navigation_graph.cfg`.
- New section in the gameplay.cfg has been documented: `[WEAR_AND_TEAR_SYSTEM]`
- The cockpit.cfg documentation has the following new section and parameters:
    - Various new parameters have been added to customise the external HUD: `[MISC]`.  
        New section and parameters related to general cockpit information: `[General]`.
- The systems.cfg file documentation has received the following additions:
    - `[HYDRAULICS_SYSTEM_EX1]`, Hydraulics System Setup Information, Hydraulic System Examples.  
        `[PNEUMATIC_SYSTEM_EX1]`, Pneumatic System Setup Information, Pneumatic System Examples.  
        `[WATER BALLAST SYSTEM][Liquid Dropping System][ELECTRICAL]`, Electric System Setup Information  
        `[BRAKES]`: toe_brakes_pressure_increment, toe_brakes_pressure_release_delay, toe_brakes_pressure_decrease_tc. rto_min_speed_for_trigger,  
        `[AUTOPILOT]`: auto_throttle_derivative_boundary, auto_throttle_derivative_control, auto_throttle_integrator_boundary, auto_throttle_integrator_control, auto_throttle_proportional_control,  
        `[LocalVars_EX1]`: new section for `L:1` type vars added.
- The ai.cfg has the following new section: `[AI_INPUT]`
- The aircraft.cfg has the following new pages, sections and parameters:
    - The `[GENERAL]` section has the following new parameters: object_class,  
        The `[SERVICES]` section has a new parameter: `WINCH`.  
        The `[PILOT]` section has new parameters (and all previous parameters are now considered legacy): cabin_service, generated_copilot, copilot_behavior. hide_avatar,  
        The `[FLTSIM.N]` section has the following new parameters: ui_powerplant_specifics, ui_instrumentation, ui_additional_information, operating_status, targeted_specializations, military, premium,  
        A new page has been added to include additional information for the `aircraft.cfg` and `sim.cfg` files: Aircraft.cfg / Sim.cfg Additional Information
- The flight_model.cfg has the following new pages, sections and parameters:
    - The `[AERODYNAMICS]` section has the following new parameters:
        - `stallalphastallalpha_ff`,
    - The `[WEIGHT_AND_BALANCE]` section has the following new parameters:
        -   max_takeoff_weight, max_landing_weight,
    - The `[AIRPLANE_GEOMETRY]` section has the following new parameters:
        -   cockpit_width, cockpit_height, control_aileron_forcebased, control_aileron_maxforce_student, control_aileron_minforce_student, control_aileron_maxforce_pilot, control_aileron_minforce_pilot, control_aileron_maxforce_testpilot, control_aileron_minforce_testpilot, control_aileron_still_force_at_max, control_aileron_still_force_to_move, control_aileron_dynpres_ratio_force_at_max, control_aileron_dynpres_ratio_force_to_move, control_aileron_neutral_return_force_scalar, control_elevator_forcebased, control_elevator_maxforce_student, control_elevator_minforce_student, control_elevator_maxforce_pilot, control_elevator_minforce_pilot, control_elevator_maxforce_testpilot, control_elevator_maxforce_testpilot, control_elevator_still_force_at_max, control_elevator_still_force_to_move, control_elevator_dynpres_ratio_force_at_max, control_elevator_dynpres_ratio_force_to_move, control_elevator_neutral_return_force_scalar, control_rudder_forcebased, control_rudder_maxforce_student, control_rudder_minforce_student, control_rudder_maxforce_pilot, control_rudder_minforce_pilot, control_rudder_maxforce_testpilot, control_rudder_minforce_testpilot, control_rudder_still_force_at_max, control_rudder_still_force_to_move, control_rudder_dynpres_ratio_force_at_max, control_rudder_dynpres_ratio_force_to_move, control_rudder_neutral_return_force_scalar,
    - The `[HELICOPTER]` section has the following new parameters:
        -   cyclic_move_rate_limit, rudder_pedals_move_rate_limit, rotor_node.n,
    - The `[MAINROTOR]` and `[SECONDARYROTOR]` sections have the following new parameters:
        -   BrakeCircuit,
    - The `[FLIGHT_TUNING]` section has the following new parameters:
        -   ground_new_contact_model_gear_flex, ground_new_contact_model_gear_flex_damping, ground_new_contact_model_rolling_stickyness, ground_new_contact_model_up_to_speed_lateral, ground_new_contact_model_up_to_speed_lateral_steering, ground_new_contact_model_up_to_speed_longitudinal, enable_high_accuracy_integration,
    - The `[WEIGHT_AND_BALANCE]` section has had the following new parameters added:
        -   max_zero_fuel_weight,
    - The `[FUEL_SYSTEM]` section has been updated and expanded.
    - A new section for `[COLLISION_DAMAGE]` has been added as part of the damage/wear and Tear system. Included in this addition, new parameters have been added to the `[FLAPS.N]` section as well:
        -   FlapSurface_Left, FlapSurface_Right, FlapCable_Left, FlapCable_Right.
    - A new section and parameters have been added for hot air balloons: `[BALLOON]`.
    - The `point.N` parameter in the `[CONTACT_POINTS]` section has received a new table entry to control how the gear handle will interact with the landing gear when this is extensible.
    - A new section (with parameters) has been added that permits you to disable the standard geometric representation of certain elements of the flight model. See the `[DESIGN_ACTIVATION]` section for more details. To accompany this, there are also the following new sections (with parameters) that deal with custom geometries and other physical items in the simulation:
        - [OBJ_EA1_BALLOON.N]  
            [OBJ_EA1_PITOTFLAG.N]  
            [OBJ_EA1_YAWSTRING.N]  
            [OBJ_EA1_ROPECRATE.N]  
            [OBJ_EA1_BANNER.N]  
            [OBJ_EA1_SIMPLEGEAR.N]  
            [OBJ_EA1_SURFACE.N]
- The engines.cfg has the following new pages, sections and parameters:
    - New page containing additional information to help setup the engine CFG file has been added here: engines.cfg - Additional Information
    - The `[GENERALENGINEDATA]` section has the following new parameters: smoke_protection,
    - The `[PROPELLER]` section has the following new parameters:
        -   number_of_propellers, prop_node.n, governor_prop_pitch_rate, feathering_prop_pitch_rate, beta_range_prop_pitch_rate, beta_forced_prop_pitch_rate, min_flight_beta_throttle_pos, rotation,
    - The `[TURBINEENGINEDATA]` section has the following new parameters:
        -   turbine_node.n, turbine_blades, N1_100pc_rpm, bleed_air_high_gain, bleed_air_med_gain, bleed_air_lo_gain, bleed_air_hi_bkpt, bleed_air_lo_bkpt,
- The following new CFG files have been documented:
    - attached_objects.cfg  
        attachment.cfg  
        effects.cfg  
        flight_performance.cfg  
        livery.cfg  
        model.cfg  
        texture.cfg


{{< release-notes-tag "improved" >}}

- The SimObj documentation has been split into two sections given the changes to aircraft SimObj. There is now a section for Simple SimObjects and another for Modular Aircraft SimObjects.
- The mission documentation has been updated with a new section detailing the mission XML elements for navigation services: SimMission Navigation Services
- The following updates and additions have also been made to various Model Behaviour XML files:
    - The `<Parameters>` element has a new available value for the `Process` attribute: **"Macro"**.  
        The `<MouseFlags>` element has two new flags available: `Enter`, `Exit`.  
        The `<CompileBehaviors>` element has been documented.  
        Changes to the `<Binding>` element have been documented.  
        Changes to the `<EmissiveFactor>` element have been documented.  
        New attributes for the `<Condition>` element have been documented.  
        Input event name restrictions have been documented (alpha-numeric with the "-"and "_" symbols *only* are permitted).
- The entire section dedicated to creating Checklists has been updated.
- The following scenery XML pages have been updated:
    - General Environment XML Properties
        - `<Vertex />` has been updated.
        - `<Ndb>` / `<Vor>` / `<Dme />` have all been added.
    - Airport XML Properties
        - `<Airport>` / `<Apron>` have been updated.
        - `<AirportArchetype>` /`<ParamOverride />` / `<ApronControl>` have all been added.
    - Taxiway XML Properties
        - `<TaxiwayParking />` has been updated.
        - `<TaxiwayServiceStand />` has been added.
    - Scenery Editor Object XML Properties
        - `<Decal />` has been added.
- The information relating to setting up aircraft audio has been updated.
- There has been a general update to all the pages related to Model Behaviors to bring them in line with the MSFS 2024 standards. It is worth noting that the following pages are either new or have received a complete rewrite:
    - Important Templates  
        Using Model Behaviors  
        Model Behaviors Parameters  
        Model Behaviors Inputs  
        Creating Cockpit Interactions (previously called "Creating Interactions With Input Events")  
        Creating Interaction Tooltips
- The following parameters across various CFG files have been changed and no longer require a list, and instead take a hash-map: `lightdef.N`, `point.N`, `interactive_point.N`,


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "added" >}}

- SimVar Documentation has been updated with multiple new sections:
    - Pneumatics  
        Hydraulics  
        Water Ballast  
        Liquid Dropping System  
        Aircraft Wear And Tear States
- New SimVars added in the following sections:
    - Aircraft System Variables
        -   `LIQUID DROPPING DOOR FLOW VOLUME`, `LIQUID DROPPING SCOOP FLOW VOLUME`, `LIQUID DROPPING TANK CAPACITY VOLUME`, `LIQUID DROPPING TANK CURRENT VOLUME`, `LIQUID DROPPING TANK TOTAL CAPACITY VOLUME`, `LIQUID DROPPING TANK TOTAL CURRENT VOLUME`, `LIQUID DROPPING TOTAL DROPPED FLOW VOLUME`, `LIQUID DROPPING TOTAL SCOOPED FLOW VOLUME`.

    - Aircraft Engine Variables
        -   `RECIP ENG BRAKE POWER PCT`,

    - Aircraft Electrics Variables:
        -   `CIRCUIT CABIN SIGNAL GO`, `CIRCUIT CABIN SIGNAL STANDBY`, `CIRCUIT CABIN SIGNAL STOP`, `ELECTRICAL CIRCUIT EXISTS`, `ELECTRICAL GENERATOR ACTIVE`, `ELECTRICAL GENERATOR AMPS`, `ELECTRICAL GENERATOR VOLTAGE`, `ELECTRICAL GENERATOR SWITCH`, `ELECTRICAL CIRCUIT AMPS`, `ELECTRICAL CIRCUIT VOLTAGE`, `ELECTRICAL EXTERNAL POWER AMPS`, `ELECTRICAL EXTERNAL POWER VOLTAGE`,

    - Aircraft Control Variables:
        -   `ELEVATOR TRIM MIN`, `ELEVATOR TRIM MAX`, `ELEVATOR TRIM PCT EX1`, `RUDDER TRIM MIN`, `RUDDER TRIM MAX`, `RUDDER TRIM PCT EX1`, `AILERON TRIM MIN`, `AILERON TRIM MAX`, `AILERON TRIM PCT EX1`, `FLAPS CURRENT SPEED LIMITATION`,

    - Aircraft Flight Model Variables:
        -   `REFERENCE SPEED MAX IAS`, `REFERENCE SPEED MAX IAS GEAR DOWN`, `MAX LANDING WEIGHT`, `MAX TAKEOFF WEIGHT`,

    - Helicopter Variables:
        -   `HOVER INDUCED VELOCITY`, `ROTOR GOV TARGET PCT`, `ROTOR GOV ENGINE TRIM`,

    - Aircraft Misc. Variables:
        -   `BALLOON GAS DENSITY`, `BALLOON GAS TEMPERATURE`, `BALLOON VENT OPEN VALUE`, `BURNER HEATING POWER`, `BURNER PILOT LIGHT ON`, `BURNER VALVE OPEN VALUE`,

    - Miscellaneous Variables:

        -   `AMBIENT IN SMOKE`, `ENV CLOUD DENSITY`, `ENV SMOKE DENSITY`, `SEA LEVEL AMBIENT TEMPERATURE`, `HIDE AVATAR IN AIRCRAFT`,
- New section explaining Convenience SimVars.
- New section for the Electronic Flight Bag API has been added.
- New page added to the Key Event ID documentation:
    -   Aircraft General Systems Events
        -   New key events for liquid dropping system added:

            `LIQUID_DROPPING_SYSTEM_DOOR_CLOSELIQUID_DROPPING_SYSTEM_DOOR_COMMAND_GROUP_CLOSELIQUID_DROPPING_SYSTEM_DOOR_COMMAND_GROUP_OPENLIQUID_DROPPING_SYSTEM_DOOR_COMMAND_GROUP_SETLIQUID_DROPPING_SYSTEM_DOOR_COMMAND_GROUP_TOGGLELIQUID_DROPPING_SYSTEM_DOOR_OPENLIQUID_DROPPING_SYSTEM_DOOR_SETLIQUID_DROPPING_SYSTEM_DOOR_TOGGLELIQUID_DROPPING_SYSTEM_SCOOP_CLOSELIQUID_DROPPING_SYSTEM_SCOOP_OPENLIQUID_DROPPING_SYSTEM_SCOOP_SETLIQUID_DROPPINGSYSTEM_SCOOP_TOGGLE`

        -   New key events for the pnuematics system added:

            `PNEUMATICS_AREA_TEMPERATURE_DECPNEUMATICS_AREA_TEMPERATURE_INCPNEUMATICS_AREA_TEMPERATURE_SETPNEUMATICS_FAN_SETPNEUMATICS_JUNCTION_LINE_OPENING_STATUS_SETPNEUMATICS_PACK_FLOW_AUTO_OFFPNEUMATICS_PACK_FLOW_AUTO_ONPNEUMATICS_PACK_FLOW_AUTO_SETPNEUMATICS_PACK_FLOW_MODE_HIGHPNEUMATICS_PACK_FLOW_MODE_LOWPNEUMATICS_PACK_FLOW_MODE_NORMPNEUMATICS_PACK_FLOW_MODE_SETPNEUMATICS_PACK_OFFPNEUMATICS_PACK_ONPNEUMATICS_PACK_SETPNEUMATICS_PACK_TOGGLEPNEUMATICS_PACKS_FLOW_DECPNEUMATICS_PACKS_FLOW_INCPNEUMATICS_PACKS_FLOW_SETPNEUMATICS_TARGET_CABIN_ALTITUDE_DECPNEUMATICS_TARGET_CABIN_ALTITUDE_INCPNEUMATICS_TARGET_CABIN_ALTITUDE_SETPNEUMATICS_VALVE_CLOSEPNEUMATICS_VALVE_MODE_AUTOPNEUMATICS_VALVE_MODE_CLOSEDPNEUMATICS_VALVE_MODE_OPENPNEUMATICS_VALVE_MODE_SETPNEUMATICS_VALVE_OPENPNEUMATICS_VALVE_SETPNEUMATICS_VALVE_TOGGLE`

        -   New key events for the hydraulics system added:

            `HYDRAULIC_SWITCH_TOGGLEHYDRAULIC_VALVE_CLOSEHYDRAULIC_VALVE_OPENHYDRAULIC_VALVE_SETHYDRAULIC_VALVE_TOGGLE`
- New page added to the Key Event ID documentation:
    -   Balloon Specific Events
        -   New key events for balloon burner systems have been added:

            `AXIS_BURNER_PITCH_SETAXIS_BURNER_ROLL_SETBURNER_PITCH_DECBURNER_PITCH_INCBURNER_ROLL_DECBURNER_ROLL_INCBURNER_VALVE_CLOSEBURNER_VALVE_OPENBURNER_VALVE_SETBURNER_VAL`

        -   New key events for balloon envelopes have been added:

            `BALLOON_VENT_CLOSEBALLOON_VENT_OPENBALLOON_VENT_SETBALLOON_VENT_TOGGLE`

{{< release-notes-tag "improved" >}}

- SimVar Documentation has been updated with information explaining that many of the aircraft system SimVars can now take a *name* (a string) instead of the usual index value when addressing specific components. Each of the sections where where this is applicable explain the setup at the top.
- Reverse Polish Notation documentation updated to include the following:

    

    - New **events** added to the Mouse Variables: `Enter`, `Exit`,  
        New vector operators: `xyz`, `pbh`, `agl`, `&`,  
        New string operators: `slen`,  
        New `L:1` variable type added.

{{< /expand >}}



{{< expand title="SDK Release 0.10.1" >}}

<p class="fake-h3">Developer Mode Changes</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "fixed" >}}

- Fixed Aircraft Selector/Configurator Variation/Livery handling for MSFS 2020 packages
- Adaptive detail LOD factor does not affect User aircraft anymore

{{< release-notes-tag "added" >}}

- New Content Creator Testing Tool has been added to permit registered partners to test streamed packages.


<p class="fake-h4">Scenery Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed projected mesh size not taken into account for mip streaming causing blurry projected meshes.


<p class="fake-h4">SimObject Editor</p>

{{< release-notes-tag "improved" >}}

- Changed the way tailwheel is computed in the Career Compatibility Tab

{{< release-notes-tag "added" >}}

- target_specializations as a constraint which is shown in the output of the Career Compatibility Tab


<p class="fake-h3">SDK Changes</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "fixed" >}}

- Fixed XML gauges not working after first flight.
- Fixed bitmaps not working anymore in XML gauges

{{< release-notes-tag "improved" >}}

- Hide avatar in aircraft if it's not edited for SR pilot system
- The Wear&Tear system is now deactivated on 2020 aircraft


<p class="fake-h4">SimVars</p>

{{< release-notes-tag "fixed" >}}

- Fixed "SIMULATION TIME" environment variable returning inconsistent values when switching from/to avatar mode


<p class="fake-h4">Tools</p>

{{< release-notes-tag "fixed" >}}

- 3dsmax: Fixed flags in texture tool
- Blender: Fixed reload lods for collections

{{< release-notes-tag "improved" >}}

- Blender: Added flags edition to texture
- Blender: Update texturelib generation


<p class="fake-h3">Documentation</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "improved" >}}

- All references to textures being compiled to DDS have been changed to KTX2 to reflect changes in the render engine between MSFS 2020 and MSFS 2024.


<p class="fake-h4">Developer Mode</p>

{{< release-notes-tag "added" >}}

- The Content Creator Testing Tool has been documented. 

{{< release-notes-tag "improved" >}}

- The ProjectedMesh Objects page has updated information related to the **Surface Type** option.
- The SimObject Debug Menu docs have been improved with updated information on the **Weight** and **Tracking** debug windows.

{{< release-notes-tag "fixed" >}}

- Page describing experimental features within the simulation has been removed as no longer relevant.


<p class="fake-h4">Model And Textures</p>

{{< release-notes-tag "improved" >}}

- The page on Liveries has been completed.
- The page on Static Liveries has been completed.
- The page on Dynamic Liveries has been completed.


<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "added" >}}

- New page describing the EFB flight plan file format: EFB Flight Plan XML (PLN File) Properties
- New helicopter cyclic control variables added to the flight_model.cfg file: cyclic_roll_control_scalar_negative, cyclic_pitch_control_scalar_negative.
- New page describing how to Use Included SimAttachments has been added to the career documentation

{{< release-notes-tag "improved" >}}

- The page covering Liveries setup has received a polish and an update.
- The interactive points section of the FLT File Properties page has been flagged as "coming soon", since it's currently not available in Microsoft Flight Simulator 2024.
- The light documentation in the `systems.cfg` file has been updated to remove obsolete enum values and correctly label "interior only" lights.
- Note related to setting up tailwheels has been added to the flight_model.cfg info page: Notes On Tailwheels
- The following career pages have been updated with mentions of the available included SimAttachments they can use: Aerial Advertising, Aerial Firefighting, Medevac, Scientific Research


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "added" >}}

- New environment variables documented: `IS IN RTC`, `IS AVATAR`, `IS AIRCRAFT`, `EFB_BRIGHTNESS`, `SWITCH USER ALWAYS ALLOWED`

- New helicopter autopilot parameters added into the systems.cfg file:

    pitch_attitude_hold_i  
    pitch_attitude_hold_p  
    pitch_attitude_hold_d  
    pitch_attitude_hold_vel  
    bank_attitude_hold_i  
    bank_attitude_hold_p  
    bank_attitude_hold_d  
    bank_attitude_hold_vel  
    heading_hold_rate  
    heading_hold_rate_max  
    heading_hold_bank_ang_max  
    heading_hold_i  
    heading_hold_p  
    heading_hold_d  
    altitude_hold_rate  
    altitude_hold_max  
    altitude_hold_pitch_ang_max  
    vs_hold_pitch_i  
    vs_hold_pitch_p  
    vs_hold_pitch_d  
    pa_collective_offset  
    vs_hold_collective_i  
    vs_hold_collective_p  
    vs_hold_collective_d  
    attitude_hold_off_speed  
    attitude_hold_on_speed  
    pedal_assist_off_speed  
    pedal_assist_on_speed  
    pedal_assist_heading_max_angle  
    pedal_assist_heading_angle_scalar  
    speed_hold_pitch_i  
    speed_hold_pitch_p  
    speed_hold_pitch_d  
    speed_hold_pitch_ang_max  
    speed_hold_bank_i  
    speed_hold_bank_p  
    speed_hold_bank_d  
    speed_hold_bank_ang_max  
    speed_hold_collective_i  
    speed_hold_collective_p  
    speed_hold_collective_d  
    auto_hover_vertical_i  
    auto_hover_vertical_imax  
    auto_hover_vertical_p  
    auto_hover_vertical_pmax


<p class="fake-h3">Developer Mode Changes</p>

<p class="fake-h4">Scenery Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed a crash when editing several decals in SPC
- Fixed scenery objects (decals/scenery/simobjects/etc) multi edit not working properly
- Fixed no reload of scenery objects when editing several at once.


<p class="fake-h4">SimObject Editor</p>

{{< release-notes-tag "fixed" >}}

- Fixed some bugs about Career Compatibility tab

- Fixed parameters reported as missing, even when manually added to cfg

- Fixed resync done before save when showing the validation error dialog

- Tag count in navigation graph were wrongly computed if the node was in a subgraph referenced by name

{{< release-notes-tag "improved" >}}

- Removed unwanted required condition in flight model
- Some constraints are now longer whitespace dependent


<p class="fake-h3">SDK Changes</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "fixed" >}}

- Fixed not working text and .png images for XML Gauges

<p class="fake-h4"> SimConnect</p>

{{< release-notes-tag "fixed" >}}

- Fixed a crash while requesting unknown navigation data


<p class="fake-h4">Tools</p>

{{< release-notes-tag "fixed" >}}

- 3dsmax: Write textures by copying them and fix gltf after exporting them
- Blender:
    - Fix write textures  
    - Fix export Detail map  
    - Fix reload LODs removing the old group lods when needed  
    - Fix "images is None"  
    - Fix force set detail normal scale  
    - Fix Unlink/link blendmask texture
- Simplygon bugfixes and presets adjustments.


<p class="fake-h3">Documentation</p>

<p class="fake-h4">Developer Mode</p>

{{< release-notes-tag "added" >}}

- New page to describe The Career Tool, which is designed to help developers test their aircraft in the various activity scenarios.


<p class="fake-h4">Model And Textures</p>

{{< release-notes-tag "improved" >}}

- Minor update to the section on Sky Portals to fix an error in one of the images.
- The 3DS Max plugin supported versions have been updated to mention that currently 2025 is not supported.
- New section on aircraft liveries has been added: Modeling Aircraft Liveries, Static Liveries, Dynamic Liveries
- New page added to help explain how to generate aircraft Thumbnails correctly.


<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "added" >}}

- New pages related to livery setup and creation (WIP):
    - Liveries
    - Livery XML Properties
    - Livery XML Examples
    - Dynamic Library (LBL) Files
        - liveries.lbl  
        - material.lbl  
        - palette.lbl  
        - text.lbl  
        - texture.lbl
- New Wear And Tear parameters have been added to the [COLLISION_DAMAGE] section: `FlapsLeft`, `FlapsLeftCable`, `FlapsRight`, `FlapsRightCable`, `EngineOilTank.X`.
- New [WASM_SYSTEM.N] section added to the systems.cfg with the following parameters: `ModulePath`, `SystemName`, `ParameterString`.


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "added" >}}

- The Helicopter Variables page has been updated with the following new SimVars: `NUMBER_OF_MAIN_ROTORS`,
- The WASM API pages have been updated with the following:
    -   Creating A WASM Project
    -   Creating WASM Systems
- New JS Listener page added: JS_LISTENER_PLANNEDROUTE
- New JS Events added: AvionicsRouteRequested, AvionicsRouteSync
- New Coherent calls added: GET_EFB_ROUTE, REPLY_TO_AVIONICS_ROUTE_REQUEST
- New WebAssumbly API documented: Planned Route API

{{< release-notes-tag "improved" >}}

- The GPS Variables pages have been flagged as **Legacy** and information added to the main index page about what to do in MSFS2024.

{{< /expand >}}


{{< expand title="SDK Release 0.9.3" >}}

<p class="fake-h3">Documentation</p>

<p class="fake-h4">Developer Mode</p>

{{< release-notes-tag "added" >}}

- New page documenting the Report An Issue feature has been added.

{{< release-notes-tag "improved" >}}

- The SDK Contents page has been updated to reflect changes to the SDK install (Shared Assets, ModelBehaviourDefs, etc...)
- The page documenting the **Virtual File System** has been updated, and a new section describing the VFS Projector has been added.
- The page documenting The Aircraft Capture Tool has been updated to reflect changes in the tool and workflow.


<p class="fake-h4">SDK Tools</p>

{{< release-notes-tag "fixed" >}}

- Updated SDK Tools page to accurately reflect the current tools available

{{< release-notes-tag "added" >}}

- New page added explaining how to update to the MSFS 2024 version of the 3DS Max plugin: Updating The 3DS Max Plugin And Projects

{{< release-notes-tag "improved" >}}

- The The 3DS Max Plugin documentation has been updated to bring it in line with MSFS 2024 standards (ongoing WIP).


<p class="fake-h4">Model And Textures</p>

{{< release-notes-tag "improved" >}}

- The Propellers, Turbines And Blades art documentation has been updated with additional information related to blade pitch and feathering.


<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "added" >}}

- New FLT section documented [Burner_System.N].
- New page listing obsolete parameters added: CFG Files Obsolete Parameters
- The electrical system has a new Circuit Type: `DEICE_SYSTEM`
- The de-icing system parameter `structural_deice_type` has been updated to include the new **electrical** setting.
- The panel.cfg lists new `[EFB]` section and parameters.
- The flight_model.cfg has the following new parameters:
    -   In [CONTACT_POINTS]: `water_longitudinal_friction_scalar`, `water_lateral_friction_scalar`, `water_steering_friction_scalar `
    -   In [COLLISION_DAMAGE]: `Engine.N`

{{< release-notes-tag "improved" >}}

- Legacy and obsolete parameters have been revised and more accurately labelled in the CFG Parameter Index and other files.
- The Modular SimObject Project Structure page has been improved with additional information related to thumbnails.


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "added" >}}

- The WebAssembly documentation has been updated with information on the new Vars API and Event API, which replace the Gauge API. The Gauge API is now considered a *legacy* API which should no longer be used.

- The SimVar page for Aircraft Electrics Variables has been updated to show obsolete SimVars, and two new SimVars: `ELECTRICAL BUS AMPS`, `ELECTRICAL BUS VOLTAGE`.

- The Skydiving Key Event documentation has been updated with the following additions:
    - SKYDIVE_DOORLIGHTS_DEC  
    - SKYDIVE_DOORLIGHTS_GETREADY  
    - SKYDIVE_DOORLIGHTS_INC  
    - SKYDIVE_DOORLIGHTS_JUMP  
    - SKYDIVE_DOORLIGHTS_OFF

- The Liquid Dropping System Key Event documentation has been updated with the following additions:
    - SPRAY_ON  
    - SPRAY_TOGGLE  
    - SPRAY_SET

- The General Engine Key Event docs have the following new events documented:
    - AXIS_THRUST_VECTOR_HORIZONTAL_SET  
    - AXIS_THRUST_VECTOR_VERTICAL_SET  
    - THRUST_VECTOR_HORIZONTAL_DECREASE  
    - THRUST_VECTOR_HORIZONTAL_INCREASE  
    - THRUST_VECTOR_VERTICAL_DECREASE  
    - THRUST_VECTOR_VERTICAL_INCREASE


{{< /expand >}}



{{< expand title="SDK Release 0.9.0 (MSFS2024 Initial Alpha Release)" >}}

<p class="fake-h3">DOCUMENTATION</p>

<p class="fake-h4">General</p>

{{< release-notes-tag "added" >}}

- Dark/Light mode has been added to the documentation to improve accessibility.
- New page added to give a reference for the The Colour Picker.

{{< release-notes-tag "improved" >}}

- The structure of the SDK documentation has been improved to enable finding the information you require faster and easier, including improved search, an improved index, and related topics links on all pages.
- SDK Tools have been moved into their own unique section in the Table Of Contents and all relevant information consolidated under that section.
- Aircraft samples have been converted into the following two modular aircraft:
    -   SimpleAircraft
    -   WASMAircraft


<p class="fake-h4">SDK Tools</p>

{{< release-notes-tag "added" >}}

- Documentation has been added to describe the new Input Application for creating custom input profiles for devices and aircraft. This includes the following pages:
    - Global Input Profile Package Setup  
    - Aircraft Input Profile Package Setup  
    - Creating An Input Profile  
    - DeviceConfig XML Properties  
    - Input Configuration XML Properties

- A page has been added for The Mission Tool. This tool is a temporary fix to allow developers to test certain aspects of mission creation and will be removed in future versions of the Microsoft Flight Simulator 2024 DevMode.


<p class="fake-h4">Developer Mode - General</p>

{{< release-notes-tag "added" >}}

- A new option for Lighting Display debugging has been documented.
- The DevMode Toolbar has been documented.
- The Behaviors Debug window has been documented

{{< release-notes-tag "improved" >}}

- The "Debug PBR" menu page has been renamed as Channel Display and has had some additional updates to cover new options.
- Some menus can now be "detached" so you can easily access common options. See here for more information: Detachable Menus
- The "Debug PBR" menu page has been renamed as Channel Display and has had some additional updates to cover new options.
- The **Aircraft Editor** section has been removed and replaced by the more general SimObject Editor.
- The section on using the Screenshot tool has been updated.
- The Developer Camera Settings section has been updated with new options.


<p class="fake-h4">The Project Editor</p>

{{< release-notes-tag "added" >}}

- The new Package States have been documented.
- New Package Order Hint section has been added to The Project Inspector page.

{{< release-notes-tag "improved" >}}

- The Asset Types page has been tidied and now shows information for all relevant asset types available, including the new Airport asset group type.
- All references to the setup for the Marketplace have been removed from the documentation as this is now all handled on the Marketplace itself.


<p class="fake-h4">The Scenery Editor</p>

{{< release-notes-tag "added" >}}

- Airport Objects - Override Archetypes options have been added.
- PaintedLine Objects - New option for outlines has been added.
- Decal Objects - New decal object for the Scenery Editor has been documented.
- TaxiwayServiceStand Objects - New taxiway service stand object has been documented.
- TextMarking Objects - New text object for the scenery editor has been documented.
- VDGS Objects - New Visual Docking Guidance System (VDGS) object for the scenery editor has been documented.
- SimPropContainer Object - New scenery object type has been added. See also: Modeling SimPropContainers
- SimPropContainer XML Properties and SimPropContainer Object Examples have been added to describe the XML used by the new SimPropContainer object.
- New "Apron Services" option has been documented for Airport Objects, TaxiwayParking Objects and Helipad Objects.
- New Navigation Graph options have been documented for The View Menu.
- New render options have been added to The Rendering Menu page.

{{< release-notes-tag "improved" >}}

- Helipad Objects - Multiple new options for helipads have been documented.
- TaxiwayParking Objects - Multiple new options for taxiway parking objects have been documented.
- Information has been added to the Creating A SimObject and Creating A Scenery Object pages.
- Information has been added to the Scenery XML And Model CFG page.


<p class="fake-h4">The Material Editor</p>

{{< release-notes-tag "added" >}}

- New page on The Bitmap Manager has been added.

{{< release-notes-tag "improved" >}}

- The Material Inspector page has been updated with new "extra settings" section.


<p class="fake-h4">The SimObject Editor (Previously the Aircraft Editor)</p>

{{< release-notes-tag "added" >}}

- The SimObject Editor main page has been added withinformation about the new editor
- The Attached Objects tab has been documented.
- The Attachment tab has been documented.
- The Navigation Graph tab has been documented.
- The Livery tab has been documented
- The Effects tab has been documented
- The Texture tab has been documented
- The Model LOD tab has been documented.
- The Animations tab has been documented.
- The Sounds tab has been documented.
- A dedicated page listing all SimObject Editor parameters have been added: The SimObject Editor Parameter Index
- The new debug window - Debug Dirt And Scratches - has been added to the documentation.
- The new debug window - Covers And Chocks - has been added to the documentation.
- The new debug window - Debug Aircraft Attachments - has been added to the documentation.

{{< release-notes-tag "improved" >}}

- The Aircraft Data debug window docs have been updated to reflect additions to the window.


<p class="fake-h4">The Navigation Graph Editor</p>

{{< release-notes-tag "added" >}}

- New page describing The Navigation Graph Editor has been added. Tis editor is for desiging **navigation graphs** which are used to help objects navigate around both the world and aircraft, as well as many other things.
- New page describing Using The Navigation Graph Editor has been added.


<p class="fake-h4">The SimAttachment Editor</p>

{{< release-notes-tag "added" >}}

- The SimAttachment Editor has been added to the documentation. This editor is similar to the SimObject Editor, but designed for editing stand-along attachments that can then be added to your aircraft and scenery.


<p class="fake-h4">Models And Textures</p>

{{< release-notes-tag "added" >}}

- The new Texture Tool has been documented.
- New pages have been added for the Texture XML Properties and texture.json files.
- A page has been added explaining how to use the Simplygon SDK to simplify the creation of scenery object LODs: Using Simplygon To Generate LODs
- A page has been added to explain the updated LOD Selection System for all 3D models.
- The Modeling Aircraft documentation has had the following pages added:
    - Propellers, Turbines And Blades  
    - Clearcoat  
    - Dirt And Grime  
    - Registration Numbers  
    - Covers, Chocks, And Pins  
    - Doors, Hatches, And Canopies  
    - Fuel Truck And GPU Connections

{{< release-notes-tag "improved" >}}

- FlightSim Materials (all material documentation has been updated to the new MSFS 2024 standards)
- The glTF Schemas page has been updated with information on the new schemas available.
- The Multi-Exporter documentation has received a major update.
- The Modeling Aircraft documentation has had the following pages updated:
    - File Setup  
    - Airframe Textures And Materials  
    - Windshield And Windows  
    - Exterior Lights  
    - Wheels  
    - Airframe Details  
    - Ice  
    - Airframe Collision Meshes  
    - Ambient Occlusion  
    - Parallax Windows  
    - Aircraft LODs


<p class="fake-h4">Content Configuration</p>

{{< release-notes-tag "added" >}}

- New section outlining details of the updated MSFS Avionics Framework for MSFS 2024 has been added.
- CFG Files General Information page has been added, and now contains updated information about the different **data types**, including the new table format that permits *n*D tables.
- New Simple SimObject types have been documented:
    - Procedural Characters  
    - Simple Objects
- New pages have been added related to the setup for Modular Aircraft:
    - Modular Aircraft Project Structure  
    - Modular Aircraft Merging  
    - Modular Aircraft XML Properties  
    - Modular Aircraft CFG Files  
    - Sim Attachments  
    - Included Attachments
- New pages have been added related to the configuration of models within the simulation:
    - Livery XML Properties
- New pages have been added to describe the setup of light effects:
    - Implementing Lights  
    - Light FX Properties  
    - effects.cfg
- A new section has been added for Navigation Services And Interactions:
    - Services XML Properties  
    - Navigation Services XML Properties  
    - SimMission Navigation Services  
    - Interaction XML Properties  
    - Interaction Presets
- A new section has been added for Apron Services:
    - Apron Control XML Properties  
    - Apron Services XML Properties  
    - Apron Control Mission XML Properties  
    - Built-In Apron Services  
    - Global Apron Services
- A new Careers section has been added outlining the setup for custom aircraft to be used in the various career missions. This section covers the following:
    - Preflight  
    - Spawn Pilot (Transversal)  
    - Aerial Advertising  
    - Aerial Construction  
    - Aerial Firefighting  
    - Agricultural Aviation  
    - Cargo Transport  
    - Charter Service  
    - Commercial Flights And Passengers  
    - Ferry Flights  
    - Flightseeing / First Flight  
    - Medevac  
    - Scientific Research  
    - Search And Rescue  
    - Skydiving  
    - Using Included SimAttachments
- The page for the `<SimMission.Calculator>` has new items listed specifically for use with Navigation Services, Apron Services, and Interactions:
    - &lt;CabinServiceBehaviourParameter&gt;  
    - &lt;CabinServiceStateParameter&gt;  
    - &lt;CabinServiceGraphParameter&gt;  
    - &lt;ParentCabinServiceBehaviourParameter&gt;  
    - &lt;ParentCabinServiceStateParameter&gt;  
    - &lt;ApronContextParameter&gt;  
    - &lt;ApronControlMissionContext&gt;  
    - &lt;InteractivePointParameter&gt;  
    - &lt;InteractivePointUser&gt;  
    - &lt;InteractivePoint&gt;  
    - &lt;InteractivePointType&gt;  
    - &lt;InteractivePointSearchPreferences&gt;  
    - &lt;ParkingParameter&gt;  
    - &lt;ParkingName&gt;  
    - &lt;ParkingNumber&gt;  
    - &lt;ParkingSuffix&gt;  
    - &lt;StandType&gt;  
    - &lt;StandRole&gt;  
    - &lt;HelipadParameter&gt;  
    - &lt;HelipadProperty&gt;  
    - &lt;DesiredValue&gt;  
    - &lt;Units&gt;
- FLT file documentation has been updated with a new page for FLT Additional Information and the following new sections:
    - `[HYDRAULICS_SYSTEM_EX1.N][PNEUMATIC_SYSTEM_EX1.N][Liquid Dropping System.N][Gauges.N][Navigation_Graph.N][WEAR_AND_TEAR_SYSTEM.N][CabinService.N][CabinServiceObject.N][Dirt.N]`
- The FLT file documentation also has the following new parameters:
    - `[Systems.N]` - `GovernorTarget[Covers]` - `chock`, `engine`, `pitot`, `static_port`, `rotor`, `landing_gear[Sim.N]`: `Livery`,
- New section on Changes Between MSFS 2020 And MSFS 2024
- The Checklist XML Properties page has the following new elements: `<Link>`,
- New page that includes a detailed overview of the new features of the MSFS 2024 audio system has been added: Audio Migration And Updating
- The page related to the Localization (LOC Files) has a new section outlining Supported Languages.
- New CFG file for dealing with **navigation graphs** has been documented: `navigation_graph.cfg`.
- New section in the gameplay.cfg has been documented: `[WEAR_AND_TEAR_SYSTEM]`
- The cockpit.cfg documentation has the following new section and parameters:
    - Various new parameters have been added to customise the external HUD: [MISC].  
        New section and parameters related to general cockpit information: `[General]`.
- The systems.cfg file documentation has received the following additions:
    - `[HYDRAULICS_SYSTEM_EX1]`, Hydraulics System Setup Information, Hydraulic System Examples.  
    - `[PNEUMATIC_SYSTEM_EX1]`, Pneumatic System Setup Information, Pneumatic System Examples.  
    - `[WATER BALLAST SYSTEM][Liquid Dropping System][ELECTRICAL]`, Electric System Setup Information  
    - `[BRAKES]`: toe_brakes_pressure_increment, toe_brakes_pressure_release_delay, toe_brakes_pressure_decrease_tc. rto_min_speed_for_trigger,  
    - `[AUTOPILOT]`: auto_throttle_derivative_boundary, auto_throttle_derivative_control, auto_throttle_integrator_boundary, auto_throttle_integrator_control, auto_throttle_proportional_control,  
    - `[LocalVars_EX1]`: new section for `L:1` type vars added.
- The ai.cfg has the following new section: `[AI_INPUT]`
- The aircraft.cfg has the following new pages, sections and parameters:
    - The `[GENERAL]` section has the following new parameters: object_class,  
    -   The `[SERVICES]` section has a new parameter: `WINCH`.  
    -   The `[PILOT]` section has new parameters (and all previous parameters are now considered legacy): cabin_service, generated_copilot, copilot_behavior. hide_avatar,  
    -   The `[FLTSIM.N]` section has the following new parameters: ui_powerplant_specifics, ui_instrumentation, ui_additional_information, operating_status, targeted_specializations, military, premium,  
    -   A new page has been added to include additional information for the `aircraft.cfg` and `sim.cfg` files: Aircraft.cfg / Sim.cfg Additional Information
- The flight_model.cfg has the following new pages, sections and parameters:
    - The `[AERODYNAMICS]` section has the following new parameters:
        - `stallalphastallalpha_ff`,
    - The `[WEIGHT_AND_BALANCE]` section has the following new parameters:
        -   max_takeoff_weight, max_landing_weight,
    - The `[AIRPLANE_GEOMETRY]` section has the following new parameters:
        -   cockpit_width, cockpit_height, control_aileron_forcebased, control_aileron_maxforce_student, control_aileron_minforce_student, control_aileron_maxforce_pilot, control_aileron_minforce_pilot, control_aileron_maxforce_testpilot, control_aileron_minforce_testpilot, control_aileron_still_force_at_max, control_aileron_still_force_to_move, control_aileron_dynpres_ratio_force_at_max, control_aileron_dynpres_ratio_force_to_move, control_aileron_neutral_return_force_scalar, control_elevator_forcebased, control_elevator_maxforce_student, control_elevator_minforce_student, control_elevator_maxforce_pilot, control_elevator_minforce_pilot, control_elevator_maxforce_testpilot, control_elevator_maxforce_testpilot, control_elevator_still_force_at_max, control_elevator_still_force_to_move, control_elevator_dynpres_ratio_force_at_max, control_elevator_dynpres_ratio_force_to_move, control_elevator_neutral_return_force_scalar, control_rudder_forcebased, control_rudder_maxforce_student, control_rudder_minforce_student, control_rudder_maxforce_pilot, control_rudder_minforce_pilot, control_rudder_maxforce_testpilot, control_rudder_minforce_testpilot, control_rudder_still_force_at_max, control_rudder_still_force_to_move, control_rudder_dynpres_ratio_force_at_max, control_rudder_dynpres_ratio_force_to_move, control_rudder_neutral_return_force_scalar,
    - The `[HELICOPTER]` section has the following new parameters:
        -   cyclic_move_rate_limit, rudder_pedals_move_rate_limit, rotor_node.n,
    - The `[MAINROTOR]` and `[SECONDARYROTOR]` sections have the following new parameters:
        -   BrakeCircuit,
    - The `[FLIGHT_TUNING]` section has the following new parameters:
        -   ground_new_contact_model_gear_flex, ground_new_contact_model_gear_flex_damping, ground_new_contact_model_rolling_stickyness, ground_new_contact_model_up_to_speed_lateral, ground_new_contact_model_up_to_speed_lateral_steering, ground_new_contact_model_up_to_speed_longitudinal, enable_high_accuracy_integration,
    - The `[WEIGHT_AND_BALANCE]` section has had the following new parameters added:
        -   max_zero_fuel_weight,
    - The `[FUEL_SYSTEM]` section has been updated and expanded.
    - A new section for `[COLLISION_DAMAGE]` has been added as part of the damage/wear and Tear system. Included in this addition, new parameters have been added to the `[FLAPS.N]` section as well:
        -   FlapSurface_Left, FlapSurface_Right, FlapCable_Left, FlapCable_Right.
    - A new section and parameters have been added for hot air balloons: `[BALLOON]`.
    - The `point.N` parameter in the `[CONTACT_POINTS]` section has received a new table entry to control how the gear handle will interact with the landing gear when this is extensible.
    - A new section (with parameters) has been added that permits you to disable the standard geometric representation of certain elements of the flight model. See the `[DESIGN_ACTIVATION]` section for more details. To accompany this, there are also the following new sections (with parameters) that deal with custom geometries and other physical items in the simulation:
        - [OBJ_EA1_BALLOON.N]  
        - [OBJ_EA1_PITOTFLAG.N]  
        - [OBJ_EA1_YAWSTRING.N]  
        - [OBJ_EA1_ROPECRATE.N]  
        - [OBJ_EA1_BANNER.N]  
        - [OBJ_EA1_SIMPLEGEAR.N]  
        - [OBJ_EA1_SURFACE.N]
- The engines.cfg has the following new pages, sections and parameters:
    - New page containing additional information to help setup the engine CFG file has been added here: engines.cfg - Additional Information
    - The `[GENERALENGINEDATA]` section has the following new parameters: smoke_protection,
    - The `[PROPELLER]` section has the following new parameters:
        -   number_of_propellers, prop_node.n, governor_prop_pitch_rate, feathering_prop_pitch_rate, beta_range_prop_pitch_rate, beta_forced_prop_pitch_rate, min_flight_beta_throttle_pos, rotation,
    - The `[TURBINEENGINEDATA]` section has the following new parameters:
        -   turbine_node.n, turbine_blades, N1_100pc_rpm, bleed_air_high_gain, bleed_air_med_gain, bleed_air_lo_gain, bleed_air_hi_bkpt, bleed_air_lo_bkpt,
- The following new CFG files have been documented:
    - attached_objects.cfg  
    - attachment.cfg  
    - effects.cfg  
    - flight_performance.cfg  
    - livery.cfg  
    - model.cfg  
    - texture.cfg

{{< release-notes-tag "improved" >}}

- The SimObj documentation has been split into two sections given the changes to aircraft SimObj. There is now a section for Simple SimObjects and another for Modular Aircraft SimObjects.
- The mission documentation has been updated with a new section detailing the mission XML elements for navigation services: SimMission Navigation Services
- The following updates and additions have also been made to various Model Behaviour XML files:
    - The `<Parameters>` element has a new available value for the `Process` attribute: **"Macro"**.  
    - The `<MouseFlags>` element has two new flags available: `Enter`, `Exit`.  
    - The `<CompileBehaviors>` element has been documented.  
    - Changes to the `<Binding>` element have been documented.  
    - Changes to the `<EmissiveFactor>` element have been documented.  
    - New attributes for the `<Condition>` element have been documented.  
    - Input event name restrictions have been documented (alpha-numeric with the "-"and "_" symbols *only* are permitted).
- The entire section dedicated to creating Checklists has been updated.
- The following scenery XML pages have been updated:
    - General Environment XML Properties
        - `<Vertex />` has been updated.
        - `<Ndb>` / `<Vor>` / `<Dme />` have all been added.
    - Airport XML Properties
        - `<Airport>` / `<Apron>` have been updated.
        - `<AirportArchetype>` /`<ParamOverride />` / `<ApronControl>` have all been added.
    - Taxiway XML Properties
        - `<TaxiwayParking />` has been updated.
        - `<TaxiwayServiceStand />` has been added.
    - Scenery Editor Object XML Properties
        - `<Decal />` has been added.
- The information relating to setting up aircraft audio has been updated.
- There has been a general update to all the pages related to Model Behaviors to bring them in line with the MSFS 2024 standards. It is worth noting that the following pages are either new or have received a complete rewrite:
    - Important Templates  
    - Using Model Behaviors  
    - Model Behaviors Parameters  
    - Model Behaviors Inputs  
    - Creating Cockpit Interactions (previously called "Creating Interactions With Input Events")  
    - Creating Interaction Tooltips
- The following parameters across various CFG files have been changed and no longer require a list, and instead take a hash-map: `lightdef.N`, `point.N`, `interactive_point.N`,


<p class="fake-h4">Programming APIs</p>

{{< release-notes-tag "added" >}}

- SimVar Documentation has been updated with multiple new sections:
    - Pneumatics  
    - Hydraulics  
    - Water Ballast  
    - Liquid Dropping System  
    - Aircraft Wear And Tear States
- New SimVars added in the following sections:
    - Aircraft System Variables
        -   `LIQUID DROPPING DOOR FLOW VOLUME`, `LIQUID DROPPING SCOOP FLOW VOLUME`, `LIQUID DROPPING TANK CAPACITY VOLUME`, `LIQUID DROPPING TANK CURRENT VOLUME`, `LIQUID DROPPING TANK TOTAL CAPACITY VOLUME`, `LIQUID DROPPING TANK TOTAL CURRENT VOLUME`, `LIQUID DROPPING TOTAL DROPPED FLOW VOLUME`, `LIQUID DROPPING TOTAL SCOOPED FLOW VOLUME`.
    - Aircraft Engine Variables
        -   `RECIP ENG BRAKE POWER PCT`,
    - Aircraft Electrics Variables:
        -   `CIRCUIT CABIN SIGNAL GO`, `CIRCUIT CABIN SIGNAL STANDBY`, `CIRCUIT CABIN SIGNAL STOP`, `ELECTRICAL CIRCUIT EXISTS`, `ELECTRICAL GENERATOR ACTIVE`, `ELECTRICAL GENERATOR AMPS`, `ELECTRICAL GENERATOR VOLTAGE`, `ELECTRICAL GENERATOR SWITCH`, `ELECTRICAL CIRCUIT AMPS`, `ELECTRICAL CIRCUIT VOLTAGE`, `ELECTRICAL EXTERNAL POWER AMPS`, `ELECTRICAL EXTERNAL POWER VOLTAGE`,
    - Aircraft Control Variables:
        -   `ELEVATOR TRIM MIN`, `ELEVATOR TRIM MAX`, `ELEVATOR TRIM PCT EX1`, `RUDDER TRIM MIN`, `RUDDER TRIM MAX`, `RUDDER TRIM PCT EX1`, `AILERON TRIM MIN`, `AILERON TRIM MAX`, `AILERON TRIM PCT EX1`, `FLAPS CURRENT SPEED LIMITATION`,
    - Aircraft Flight Model Variables:
        -   `REFERENCE SPEED MAX IAS`, `REFERENCE SPEED MAX IAS GEAR DOWN`, `MAX LANDING WEIGHT`, `MAX TAKEOFF WEIGHT`,
    - Helicopter Variables:
        -   `HOVER INDUCED VELOCITY`, `ROTOR GOV TARGET PCT`, `ROTOR GOV ENGINE TRIM`,
    - Aircraft Misc. Variables:
        -   `BALLOON GAS DENSITY`, `BALLOON GAS TEMPERATURE`, `BALLOON VENT OPEN VALUE`, `BURNER HEATING POWER`, `BURNER PILOT LIGHT ON`, `BURNER VALVE OPEN VALUE`,

    - Miscellaneous Variables:
        -   `AMBIENT IN SMOKE`, `ENV CLOUD DENSITY`, `ENV SMOKE DENSITY`, `SEA LEVEL AMBIENT TEMPERATURE`, `HIDE AVATAR IN AIRCRAFT`,
- New section explaining Convenience SimVars.
- New section for the Electronic Flight Bag API has been added.
- New page added to the Key Event ID documentation:
    -   Aircraft General Systems Events
        -   New key events for liquid dropping system added: `LIQUID_DROPPING_SYSTEM_DOOR_CLOSELIQUID_DROPPING_SYSTEM_DOOR_COMMAND_GROUP_CLOSELIQUID_DROPPING_SYSTEM_DOOR_COMMAND_GROUP_OPENLIQUID_DROPPING_SYSTEM_DOOR_COMMAND_GROUP_SETLIQUID_DROPPING_SYSTEM_DOOR_COMMAND_GROUP_TOGGLELIQUID_DROPPING_SYSTEM_DOOR_OPENLIQUID_DROPPING_SYSTEM_DOOR_SETLIQUID_DROPPING_SYSTEM_DOOR_TOGGLELIQUID_DROPPING_SYSTEM_SCOOP_CLOSELIQUID_DROPPING_SYSTEM_SCOOP_OPENLIQUID_DROPPING_SYSTEM_SCOOP_SETLIQUID_DROPPINGSYSTEM_SCOOP_TOGGLE`

        -   New key events for the pnuematics system added: `PNEUMATICS_AREA_TEMPERATURE_DECPNEUMATICS_AREA_TEMPERATURE_INCPNEUMATICS_AREA_TEMPERATURE_SETPNEUMATICS_FAN_SETPNEUMATICS_JUNCTION_LINE_OPENING_STATUS_SETPNEUMATICS_PACK_FLOW_AUTO_OFFPNEUMATICS_PACK_FLOW_AUTO_ONPNEUMATICS_PACK_FLOW_AUTO_SETPNEUMATICS_PACK_FLOW_MODE_HIGHPNEUMATICS_PACK_FLOW_MODE_LOWPNEUMATICS_PACK_FLOW_MODE_NORMPNEUMATICS_PACK_FLOW_MODE_SETPNEUMATICS_PACK_OFFPNEUMATICS_PACK_ONPNEUMATICS_PACK_SETPNEUMATICS_PACK_TOGGLEPNEUMATICS_PACKS_FLOW_DECPNEUMATICS_PACKS_FLOW_INCPNEUMATICS_PACKS_FLOW_SETPNEUMATICS_TARGET_CABIN_ALTITUDE_DECPNEUMATICS_TARGET_CABIN_ALTITUDE_INCPNEUMATICS_TARGET_CABIN_ALTITUDE_SETPNEUMATICS_VALVE_CLOSEPNEUMATICS_VALVE_MODE_AUTOPNEUMATICS_VALVE_MODE_CLOSEDPNEUMATICS_VALVE_MODE_OPENPNEUMATICS_VALVE_MODE_SETPNEUMATICS_VALVE_OPENPNEUMATICS_VALVE_SETPNEUMATICS_VALVE_TOGGLE`

        -   New key events for the hydraulics system added: `HYDRAULIC_SWITCH_TOGGLEHYDRAULIC_VALVE_CLOSEHYDRAULIC_VALVE_OPENHYDRAULIC_VALVE_SETHYDRAULIC_VALVE_TOGGLE`
- New page added to the Key Event ID documentation:
    -   Balloon Specific Events
        -   New key events for balloon burner systems have been added: `AXIS_BURNER_PITCH_SETAXIS_BURNER_ROLL_SETBURNER_PITCH_DECBURNER_PITCH_INCBURNER_ROLL_DECBURNER_ROLL_INCBURNER_VALVE_CLOSEBURNER_VALVE_OPENBURNER_VALVE_SETBURNER_VAL`

        -   New key events for balloon envelopes have been added: `BALLOON_VENT_CLOSEBALLOON_VENT_OPENBALLOON_VENT_SETBALLOON_VENT_TOGGLE`

{{< release-notes-tag "improved" >}}

- SimVar Documentation has been updated with information explaining that many of the aircraft system SimVars can now take a *name* (a string) instead of the usual index value when addressing specific components. Each of the sections where where this is applicable explain the setup at the top.
- Reverse Polish Notation documentation updated to include the following:
    - New **events** added to the Mouse Variables: `Enter`, `Exit`,  
    - New vector operators: `xyz`, `pbh`, `agl`, `&`,  
    - New string operators: `slen`,  
    - New `L:1` variable type added.

{{< /expand >}}