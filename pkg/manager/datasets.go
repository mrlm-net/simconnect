//go:build windows
// +build windows

package manager

import "github.com/mrlm-net/simconnect/pkg/datasets"

func (m *Instance) RegisterDataSet(dataset *datasets.DataSet) error {
	for index, def := range dataset.Definitions {
		err := m.Client().AddToDataDefinition(
			dataset.DefinitionID,
			def.Name,
			def.Unit,
			def.Type,
			def.Epsilon,
			uint32(index),
		)

		if err != nil {
			return err
		}
	}
	return nil
}
