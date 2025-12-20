//go:build windows
// +build windows

package manager

import "github.com/mrlm-net/simconnect/pkg/datasets"

func (m *Instance) RegisterDataset(definitionID uint32, dataset *datasets.DataSet) error {
	return m.Client().RegisterDataset(definitionID, dataset)
}
