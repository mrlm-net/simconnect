//go:build windows
// +build windows

package helpers

import "github.com/mrlm-net/simconnect/pkg/types"

func IsHRESULTSuccess(hresult uintptr) bool {
	return uint32(hresult) == types.S_OK
}
func IsHRESULTFailure(hresult uintptr) bool {
	return uint32(hresult) != types.S_OK
}
