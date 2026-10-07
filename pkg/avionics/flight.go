//go:build windows
// +build windows

package avionics

import (
	"fmt"
	"unsafe"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// SetFlight sets the user aircraft's call sign for ATC: its airline as
// said ("Czech Air Force", ATC AIRLINE) and flight number ("007", ATC
// FLIGHT NUMBER); "" leaves one as it is. Measured settable in MSFS 2024
// (#680): both read back as set. defBase and defBase+1 are the data
// definitions it uses; with a client that can clear a definition
// (engine.Engine, the manager) they are cleared before each use, so
// SetFlight can be called again and again.
func SetFlight(c Presser, defBase uint32, airline, number string) error {
	if len(airline) > 63 || len(number) > 7 {
		return fmt.Errorf("avionics: airline %q or flight number %q too long (63, 7)", airline, number)
	}
	if airline != "" {
		var a [64]byte
		copy(a[:], airline)
		if err := setString(c, defBase, "ATC AIRLINE", types.SIMCONNECT_DATATYPE_STRING64, a[:]); err != nil {
			return err
		}
	}
	if number != "" {
		var n [8]byte
		copy(n[:], number)
		if err := setString(c, defBase+1, "ATC FLIGHT NUMBER", types.SIMCONNECT_DATATYPE_STRING8, n[:]); err != nil {
			return err
		}
	}
	return nil
}

// clearer is a client that can clear a data definition.
type clearer interface {
	ClearDataDefinition(definitionID uint32) error
}

func setString(c Presser, def uint32, name string, typ types.SIMCONNECT_DATATYPE, b []byte) error {
	// Defined afresh each time: a datum added to a definition that has it
	// already makes it longer than the data set (#22).
	if cl, ok := c.(clearer); ok {
		_ = cl.ClearDataDefinition(def)
	}
	if err := c.AddToDataDefinition(def, name, "", typ, 0, 0); err != nil {
		return fmt.Errorf("avionics: define %s: %w", name, err)
	}
	if err := c.SetDataOnSimObject(def, types.SIMCONNECT_OBJECT_ID_USER, 0, 0, uint32(len(b)), unsafe.Pointer(&b[0])); err != nil {
		return fmt.Errorf("avionics: set %s: %w", name, err)
	}
	return nil
}
