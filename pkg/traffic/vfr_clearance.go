package traffic

import (
	"fmt"
	"strings"
)

// VFRDeparture is a VFR departure's instructions from a controlled
// aerodrome, as CAP 413 (Edition 24, Figure 24, "VFR – Departure
// Instructions and Take-off Clearance") gives them: "G-CD, after
// departure, left turn approved, climb not above altitude 2500 feet until
// reaching the zone boundary" — DEPARTURE, not TAKE-OFF; APPROVED, not
// CLEARED; read back in full. The route via a point is said as in CAP 413
// 6.7 ("route via Whiskey"); the squawk follows when one is given.
type VFRDeparture struct {
	Turn   string  // "left" or "right" turn approved after departure; "" none said
	Via    string  // the visual reporting point or route out, e.g. "Sierra"; "" none
	MaxFt  float64 // not above this altitude (feet) until the zone boundary; 0 none
	Squawk string  // e.g. "7000" or a discrete code; "" none
}

// VFRDepartureInstructions is the controller's VFR departure instructions
// to cs.
func VFRDepartureInstructions(pos Position, cs string, d VFRDeparture) Transmission {
	parts := []string{cs, "after departure"}
	parts = append(parts, d.said(false)...)
	return Transmission{Position: pos, Callsign: cs, Intent: IntentVFRDeparture, Text: strings.Join(parts, ", ")}
}

// VFRDepartureReadback is the crew's full readback of d (CAP 413 Figure 24:
// "Left turn approved. Not above altitude 2500 feet until zone boundary,
// G-CD").
func VFRDepartureReadback(pos Position, cs string, d VFRDeparture) Transmission {
	parts := d.said(true)
	if len(parts) > 0 {
		parts[0] = strings.ToUpper(parts[0][:1]) + parts[0][1:]
	}
	return Transmission{Position: pos, Callsign: cs, Intent: IntentReadback, Text: strings.Join(append(parts, cs), ", ")}
}

// said are d's instructions in order, as the controller says them or (rb)
// as the crew reads them back.
func (d VFRDeparture) said(rb bool) []string {
	var out []string
	if d.Turn != "" {
		out = append(out, d.Turn+" turn approved")
	}
	if d.Via != "" {
		out = append(out, "route via "+d.Via)
	}
	if d.MaxFt > 0 {
		if rb {
			out = append(out, fmt.Sprintf("not above altitude %.0f feet until zone boundary", d.MaxFt))
		} else {
			out = append(out, fmt.Sprintf("climb not above altitude %.0f feet until reaching the zone boundary", d.MaxFt))
		}
	}
	if d.Squawk != "" {
		out = append(out, "squawk "+d.Squawk)
	}
	return out
}
