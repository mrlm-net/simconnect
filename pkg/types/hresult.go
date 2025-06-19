//go:build windows
// +build windows

package types

// TODO HRESULT constants
const (
	S_OK         = uint32(0x00000000)
	E_FAIL       = uint32(0x80004005)
	E_INVALIDARG = uint32(0x80070057)
)
