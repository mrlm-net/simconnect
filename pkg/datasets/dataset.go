//go:build windows
// +build windows

package datasets

type DataSet struct {
	Name         string
	DefinitionID uint32
	Definitions  []DataDefinition
}
