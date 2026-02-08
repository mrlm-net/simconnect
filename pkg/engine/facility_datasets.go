//go:build windows
// +build windows

package engine

import "github.com/mrlm-net/simconnect/pkg/datasets"

func (e *Engine) RegisterFacilityDataset(definitionID uint32, dataset *datasets.FacilityDataSet) error {
	for _, def := range dataset.Definitions {
		err := e.AddToFacilityDefinition(definitionID, string(def))
		if err != nil {
			return err
		}
	}
	return nil
}
