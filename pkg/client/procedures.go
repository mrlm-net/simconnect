//go:build windows
// +build windows

package client

import "syscall"

var (
	SimConnect_Open            *syscall.LazyProc
	SimConnect_Close           *syscall.LazyProc
	SimConnect_CallDispatch    *syscall.LazyProc
	SimConnect_GetNextDispatch *syscall.LazyProc
)

func (e *Engine) lazyloadProcedures() {

	SimConnect_Open = e.dll.NewProc("SimConnect_Open")
	SimConnect_Close = e.dll.NewProc("SimConnect_Close")
	SimConnect_CallDispatch = e.dll.NewProc("SimConnect_CallDispatch")
	SimConnect_GetNextDispatch = e.dll.NewProc("SimConnect_GetNextDispatch")
}
