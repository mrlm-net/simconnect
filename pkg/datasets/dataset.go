//go:build windows
// +build windows

package datasets

type DataSet struct {
	DefinitionID uint32
	Definitions  []DataDefinition
}
