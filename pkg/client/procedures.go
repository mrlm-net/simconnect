//go:build windows
// +build windows

package client

import "syscall"

var (
	// SimConnect connection procedures
	SimConnect_Open  *syscall.LazyProc
	SimConnect_Close *syscall.LazyProc
	// SimConnect message handling procedures
	SimConnect_CallDispatch    *syscall.LazyProc
	SimConnect_GetNextDispatch *syscall.LazyProc
	// SimConnect System state procedure
	SimConnect_RequestSystemState *syscall.LazyProc
	// SimConnect procedures for setting up and getting values of simvars
	SimConnect_AddToDataDefinition        *syscall.LazyProc
	SimConnect_RequestDataOnSimObject     *syscall.LazyProc
	SimConnect_RequestDataOnSimObjectType *syscall.LazyProc
	SimConnect_SetDataOnSimObject         *syscall.LazyProc
)

func (e *Engine) bootstrapProcedures() {
	SimConnect_Open = e.dll.NewProc("SimConnect_Open")
	SimConnect_Close = e.dll.NewProc("SimConnect_Close")
	SimConnect_CallDispatch = e.dll.NewProc("SimConnect_CallDispatch")
	SimConnect_GetNextDispatch = e.dll.NewProc("SimConnect_GetNextDispatch")
	SimConnect_RequestSystemState = e.dll.NewProc("SimConnect_RequestSystemState")
	SimConnect_AddToDataDefinition = e.dll.NewProc("SimConnect_AddToDataDefinition")
	SimConnect_RequestDataOnSimObject = e.dll.NewProc("SimConnect_RequestDataOnSimObject")
	SimConnect_RequestDataOnSimObjectType = e.dll.NewProc("SimConnect_RequestDataOnSimObjectType")
	SimConnect_SetDataOnSimObject = e.dll.NewProc("SimConnect_SetDataOnSimObject")
}
