//go:build windows
// +build windows

package engine

import "github.com/mrlm-net/simconnect/pkg/datasets"

func (e *Engine) RegisterDataset(definitionID uint32, dataset *datasets.DataSet) error {
	for index, def := range dataset.Definitions {
		err := e.AddToDataDefinition(
			definitionID,
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
