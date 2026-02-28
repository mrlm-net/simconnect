//go:build windows
// +build windows

package traffic

import "github.com/mrlm-net/simconnect/pkg/datasets"

func init() {
	datasets.Register("traffic/aircraft", "traffic", NewAircraftDataset)
}
