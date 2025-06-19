package types

type SIMCONNECT_PERIOD uint32

// SIMCONNECT_PERIOD defines the frequency at which is used for SimConnect_RequestDataOnSimObject
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_PERIOD.htm
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_RequestDataOnSimObject.htm
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_AddToDataDefinition.htm
const (
	SIMCONNECT_PERIOD_NEVER        SIMCONNECT_PERIOD = iota // Never send data
	SIMCONNECT_PERIOD_ONCE                                  // Send data once only
	SIMCONNECT_PERIOD_VISUAL_FRAME                          // Send data every visual frame
	SIMCONNECT_PERIOD_SIM_FRAME                             // Send data every simulation frame
	SIMCONNECT_PERIOD_SECOND                                // Send data once per second
)

type SIMCONNECT_CLIENT_DATA_PERIOD uint32

// SIMCONNECT_CLIENT_DATA_PERIOD defines the frequency at which is used for SimConnect_RequestClientData
// https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Structures_And_Enumerations/SIMCONNECT_CLIENT_DATA_PERIOD.htm
const (
	SIMCONNECT_CLIENT_DATA_PERIOD_NEVER        SIMCONNECT_CLIENT_DATA_PERIOD = iota // Never send (SIMCONNECT_PERIOD_NEVER)
	SIMCONNECT_CLIENT_DATA_PERIOD_ONCE                                              // Send data once only (SIMCONNECT_PERIOD_ONCE)
	SIMCONNECT_CLIENT_DATA_PERIOD_VISUAL_FRAME                                      // Send data every visual frame (SIMCONNECT_PERIOD_VISUAL_FRAME)
	SIMCONNECT_CLIENT_DATA_PERIOD_ON_SET                                            // Send data when sim variables are changed (SIMCONNECT_PERIOD_ON_SET)
	SIMCONNECT_CLIENT_DATA_PERIOD_SECOND                                            // Send data once per second (SIMCONNECT_PERIOD_SECOND)
)
