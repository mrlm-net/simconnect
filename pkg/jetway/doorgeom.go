package jetway

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// geometry is what of a flight_model.cfg puts a door on the ground: the
// reference datum (reference_datum_position, feet: along, across, up from
// the simulator's position), the nose gear (the foremost wheel of
// [CONTACT_POINTS], feet ahead of the datum), and how the aircraft rests
// (static_cg_height, feet, and static_pitch, degrees, + up).
type geometry struct {
	datum           [3]float64
	nose            float64
	hasNose         bool
	cgHeight, pitch float64
	hasCG, hasPitch bool
}

// readGeometry reads the geometry of a flight_model.cfg's lines.
func readGeometry(lines []string) geometry {
	var g geometry
	section := ""
	nose := math.Inf(-1)
	for _, l := range lines {
		s := strings.TrimSpace(l)
		if strings.HasPrefix(s, "[") {
			section = strings.ToUpper(strings.Trim(s, "[] \t"))
			continue
		}
		k, v, ok := strings.Cut(s, "=")
		if !ok {
			continue
		}
		k = strings.ToLower(strings.TrimSpace(k))
		v, _, _ = strings.Cut(v, ";")
		f := strings.Split(v, ",")
		num := func(i int) (float64, bool) {
			if i >= len(f) {
				return 0, false
			}
			x, err := strconv.ParseFloat(strings.TrimSpace(f[i]), 64)
			return x, err == nil
		}
		switch {
		case section == "WEIGHT_AND_BALANCE" && k == "reference_datum_position":
			for i := range g.datum {
				g.datum[i], _ = num(i)
			}
		case section == "CONTACT_POINTS" && k == "static_cg_height":
			g.cgHeight, g.hasCG = num(0)
		case section == "CONTACT_POINTS" && k == "static_pitch":
			g.pitch, g.hasPitch = num(0)
		case section == "CONTACT_POINTS" && strings.HasPrefix(k, "point."):
			if c, ok := num(0); !ok || c != 1 {
				continue // not a wheel
			}
			if x, ok := num(1); ok && x > nose {
				nose = x
			}
		}
	}
	if !math.IsInf(nose, -1) {
		g.nose, g.hasNose = nose, true
	}
	return g
}

// doorFrom is the interactive_point.0 line door (of the aircraft of
// geometry from) put on the aircraft of geometry to where it is on from:
// as far behind the nose gear (both park their nose gear on the same
// spot) and as high above the ground, at rest. Its side stays (both
// stand on the same line). ok is false when the nose gears are not both
// known (the line then is copied as written); without both resting
// heights and pitches only its place along the fuselage moves.
func doorFrom(from, to geometry, door string) (string, bool) {
	if !from.hasNose || !to.hasNose {
		return "", false
	}
	k, v, ok := strings.Cut(door, "=")
	if !ok {
		return "", false
	}
	v, comment, hasComment := strings.Cut(v, ";")
	f := strings.Split(v, ",")
	if len(f) < 4 {
		return "", false
	}
	long, err1 := strconv.ParseFloat(strings.TrimSpace(f[1]), 64)
	vert, err2 := strconv.ParseFloat(strings.TrimSpace(f[3]), 64)
	if err1 != nil || err2 != nil {
		return "", false
	}
	nl := long - from.nose + to.nose
	f[1] = fmt.Sprintf(" %.3f", nl)
	if from.hasCG && from.hasPitch && to.hasCG && to.hasPitch {
		// Its height above the ground on from: the simulator's position
		// static_cg_height up, the datum and the door from there, tilted
		// by the pitch; the same height on to.
		up := from.cgHeight + from.datum[2] + vert + (from.datum[0]+long)*math.Sin(from.pitch*math.Pi/180)
		nv := up - to.cgHeight - to.datum[2] - (to.datum[0]+nl)*math.Sin(to.pitch*math.Pi/180)
		f[3] = fmt.Sprintf(" %.3f", nv)
	}
	l := strings.TrimRight(k, " ") + " = " + strings.TrimSpace(strings.Join(f, ","))
	if c := strings.TrimSpace(comment); hasComment && c != "" {
		l += " ; " + c
	}
	return l, true
}
