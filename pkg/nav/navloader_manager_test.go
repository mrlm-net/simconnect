//go:build windows
// +build windows

package nav_test

import (
	"github.com/mrlm-net/simconnect/pkg/manager"
	"github.com/mrlm-net/simconnect/pkg/nav"
)

// The manager satisfies the loader, including exception matching (the
// send ID of the last packet). An external test package: manager imports
// traffic, which imports nav.
var (
	_ nav.FacilityClient                                 = manager.Manager(nil)
	_ interface{ GetLastSentPacketID() (uint32, error) } = manager.Manager(nil)
)
