//go:build windows

package simconnect

import (
	"syscall"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

func stringToBytePtr(name string) (*byte, error) {
	return syscall.BytePtrFromString(name)
}

func isHRESULTSuccess(hresult uintptr) bool {
	return uint32(hresult) == types.S_OK
}

func isHRESULTFailure(hresult uintptr) bool {
	// HRESULT failure is indicated by the high bit being set (0x80000000)
	return (uint32(hresult) & 0x80000000) != 0
}

func toUnsafePointer[T any](ptr *T) uintptr {
	return uintptr(unsafe.Pointer(ptr))
}
