//go:build windows

package simconnect

import (
	"syscall"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

func stringToBytePtr(name string) (uintptr, error) {
	// Use syscall.BytePtrFromString to convert the string to a null-terminated byte array
	// This is necessary for compatibility with the SimConnect API which expects a pointer to a null-terminated string.
	// Convert name to null-terminated byte array
	nameBytes, err := syscall.BytePtrFromString(name)

	if err != nil {
		return 0, err // Return 0 for uintptr if conversion fails
	}

	return uintptr(unsafe.Pointer(nameBytes)), nil // Convert the byte array to a uintptr pointer
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
