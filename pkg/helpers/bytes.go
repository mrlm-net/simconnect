//go:build windows
// +build windows

package helpers

import (
	"syscall"
	"unsafe"
)

func StringToBytePtr(name string) (uintptr, error) {
	// Use syscall.BytePtrFromString to convert the string to a null-terminated byte array
	// This is necessary for compatibility with the SimConnect API which expects a pointer to a null-terminated string.
	// Convert name to null-terminated byte array
	nameBytes, err := syscall.BytePtrFromString(name)

	if err != nil {
		return 0, err // Return 0 for uintptr if conversion fails
	}

	return uintptr(unsafe.Pointer(nameBytes)), nil // Convert the byte array to a uintptr pointer
}
