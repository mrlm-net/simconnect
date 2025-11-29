//go:build windows
// +build windows

package types

// HRESULT constants commonly used in SimConnect operations
// Standard Windows HRESULT values for error handling
// https://docs.microsoft.com/en-us/windows/win32/com/error-handling-in-com
const (
	// Success codes
	S_OK    = uint32(0x00000000) // Operation succeeded
	S_FALSE = uint32(0x00000001) // Operation succeeded but returned false

	// Generic error codes
	E_FAIL         = uint32(0x80004005) // Unspecified error
	E_INVALIDARG   = uint32(0x80070057) // One or more arguments are invalid
	E_OUTOFMEMORY  = uint32(0x8007000E) // Out of memory
	E_NOTIMPL      = uint32(0x80004001) // Not implemented
	E_NOINTERFACE  = uint32(0x80004002) // No such interface supported
	E_POINTER      = uint32(0x80004003) // Invalid pointer
	E_HANDLE       = uint32(0x80070006) // Invalid handle
	E_ABORT        = uint32(0x80004004) // Operation aborted
	E_ACCESSDENIED = uint32(0x80070005) // Access denied
)
