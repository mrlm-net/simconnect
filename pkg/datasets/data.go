//go:build windows
// +build windows

package datasets

import "github.com/mrlm-net/simconnect/pkg/types"

type DataDefinition struct {
	Name    string
	Unit    string // Should be type types.SIMCONNECT_UNITS later on
	Type    types.SIMCONNECT_DATATYPE
	Epsilon float32
}
