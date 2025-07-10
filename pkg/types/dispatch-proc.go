package types

// DispatchProc is a callback function for SimConnect_CallDispatch.
// pData: pointer to a SIMCONNECT_RECV structure (base for all SimConnect messages)
// cbData: size of the data buffer in bytes
// pContext: user-defined context pointer
// https://docs.flightsimulator.com/msfs2024/html/6_Programming_APIs/SimConnect/API_Reference/General/DispatchProc.htm
type DispatchProc func(pData *SIMCONNECT_RECV, cbData uint32, pContext uintptr)
