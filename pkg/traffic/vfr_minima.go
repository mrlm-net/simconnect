package traffic

import "math"

// VFR weather minima (#1258), from Regulation (EU) No 923/2012 (SERA),
// consolidated text of 26.01.2023:
//
//   - SERA.5001, Table S5-1: flight visibility and distance from cloud by
//     altitude band and class (VFRMinima);
//   - SERA.5005(b): VFR to or from an aerodrome in a control zone needs a
//     ceiling of 1,500 ft and a ground visibility of 5 km, or a special VFR
//     clearance (CheckControlZone);
//   - SERA.5010: special VFR in a control zone (SpecialVFRMinima,
//     CheckControlZone).
//
// SimConnect (2024) gives no cloud base, ceiling or distance from cloud:
// the checks run on visibility and say the cloud part is unknown; they
// never make a ceiling up.

// Classes beside those that matter for separation (airspace.go): A (VFR
// minima for guidance only, Table S5-1 (*2)), B, and F.
const (
	ClassA AirspaceClass = "A"
	ClassB AirspaceClass = "B"
	ClassF AirspaceClass = "F"
)

const (
	// VMCHighVisibilityM: at and above 10,000 ft AMSL, every class.
	VMCHighVisibilityM = 8000
	// VMCVisibilityM: below 10,000 ft AMSL.
	VMCVisibilityM = 5000
	// VMCCloudHorizontalM and VMCCloudVerticalFt: the distance from cloud,
	// in every band but F and G at and below 3,000 ft AMSL / 1,000 ft
	// above terrain.
	VMCCloudHorizontalM = 1500
	VMCCloudVerticalFt  = 1000
	// VMCReducedVisibilityM is the least flight visibility the competent
	// authority may permit for F and G in the low band (Table S5-1 (*3)(a)).
	VMCReducedVisibilityM = 1500

	// The bands: the high band from vmcHighFt AMSL (FL100 where the
	// transition altitude is lower, (*1)); the low band at and below
	// vmcLowAMSLFt AMSL or vmcLowAGLFt above terrain, whichever is higher.
	vmcHighFt    = 10000
	vmcLowAMSLFt = 3000
	vmcLowAGLFt  = 1000

	// ControlZoneVisibilityM and ControlZoneCeilingFt: below either, a VFR
	// flight may not take off or land at an aerodrome in a control zone,
	// or enter its traffic zone or circuit, without a special VFR clearance
	// (SERA.5005(b)).
	ControlZoneVisibilityM = 5000
	ControlZoneCeilingFt   = 1500

	// SpecialVFRVisibilityM (helicopters SpecialVFRHelicopterVisibilityM):
	// the least flight visibility under special VFR (SERA.5010(b)(2)) and
	// the least ground visibility for ATC to clear it (SERA.5010(c)(1)).
	SpecialVFRVisibilityM           = 1500
	SpecialVFRHelicopterVisibilityM = 800
	// SpecialVFRCeilingFt: no special VFR clearance below this ceiling
	// (SERA.5010(c)(2)).
	SpecialVFRCeilingFt = 600
	// SpecialVFRMaxIASKts: special VFR flights fly at this or slower
	// (SERA.5010(b)(3)).
	SpecialVFRMaxIASKts = 140
)

// VMCMinima are the VFR weather minima where a flight is.
type VMCMinima struct {
	Class AirspaceClass `json:"class"`
	// VisibilityM is the least flight visibility.
	VisibilityM float64 `json:"visibilityM"`
	// CloudHorizontalM and CloudVerticalFt are the least distance from
	// cloud; 0 with ClearOfCloud.
	CloudHorizontalM float64 `json:"cloudHorizontalM,omitempty"`
	CloudVerticalFt  float64 `json:"cloudVerticalFt,omitempty"`
	// ClearOfCloud and SurfaceInSight: clear of cloud with the surface in
	// sight instead of a distance (F and G in the low band, special VFR).
	ClearOfCloud   bool `json:"clearOfCloud,omitempty"`
	SurfaceInSight bool `json:"surfaceInSight,omitempty"`
}

// VFRMinimaOption configures VFRMinima.
type VFRMinimaOption func(*vfrMinimaOpts)

type vfrMinimaOpts struct {
	flightLevelFt float64 // NaN: not given
	reducedM      float64
}

// VFRFlightLevel: the transition altitude is below 10,000 ft, so FL100
// bounds the high band (Table S5-1 (*1)); pressureAltFt (altimeter at
// 1013.25 hPa) decides it.
func VFRFlightLevel(pressureAltFt float64) VFRMinimaOption {
	return func(o *vfrMinimaOpts) { o.flightLevelFt = pressureAltFt }
}

// VFRReducedVisibility: the competent authority permits a flight
// visibility down to m for F and G in the low band (Table S5-1 (*3)(a),
// e.g. at 140 kt IAS or less); never below VMCReducedVisibilityM.
func VFRReducedVisibility(m float64) VFRMinimaOption {
	return func(o *vfrMinimaOpts) { o.reducedM = math.Max(m, VMCReducedVisibilityM) }
}

// VFRMinima are the VMC minima of Table S5-1 (SERA.5001) in class at
// altFt AMSL, aglFt above terrain (NaN: not known, the low band by AMSL
// alone):
//
//   - at and above 10,000 ft AMSL: 8 km, 1,500 m horizontally and 1,000 ft
//     vertically from cloud, every class;
//   - below: 5 km and the same distance from cloud;
//   - F and G at and below 3,000 ft AMSL or 1,000 ft above terrain,
//     whichever is higher: 5 km, clear of cloud with the surface in sight.
//
// A class not known ("") takes the B–E minima.
func VFRMinima(class AirspaceClass, altFt, aglFt float64, opts ...VFRMinimaOption) VMCMinima {
	o := vfrMinimaOpts{flightLevelFt: math.NaN()}
	for _, f := range opts {
		f(&o)
	}
	m := VMCMinima{Class: class, VisibilityM: VMCVisibilityM, CloudHorizontalM: VMCCloudHorizontalM, CloudVerticalFt: VMCCloudVerticalFt}
	high := altFt >= vmcHighFt
	if !math.IsNaN(o.flightLevelFt) {
		high = o.flightLevelFt >= vmcHighFt
	}
	switch {
	case high:
		m.VisibilityM = VMCHighVisibilityM
	case (class == ClassF || class == ClassG) && (altFt <= vmcLowAMSLFt || aglFt <= vmcLowAGLFt):
		m.CloudHorizontalM, m.CloudVerticalFt = 0, 0
		m.ClearOfCloud, m.SurfaceInSight = true, true
		if o.reducedM > 0 {
			m.VisibilityM = o.reducedM
		}
	}
	return m
}

// SpecialVFRMinima are a special VFR flight's in a control zone
// (SERA.5010(b)): clear of cloud with the surface in sight, a flight
// visibility of 1,500 m (helicopters 800 m), at SpecialVFRMaxIASKts or
// slower, by day unless the authority permits otherwise.
func SpecialVFRMinima(helicopter bool) VMCMinima {
	m := VMCMinima{VisibilityM: SpecialVFRVisibilityM, ClearOfCloud: true, SurfaceInSight: true}
	if helicopter {
		m.VisibilityM = SpecialVFRHelicopterVisibilityM
	}
	return m
}

// VMCCheck is a VFR flight's visibility against its minima.
type VMCCheck struct {
	Minima      VMCMinima `json:"minima"`
	VisibilityM float64   `json:"visibilityM"`
	// VisibilityKnown: a visibility was given (above 0); VisibilityOK: it
	// meets the minimum (false when not known).
	VisibilityKnown bool `json:"visibilityKnown"`
	VisibilityOK    bool `json:"visibilityOK"`
	// CloudUnknown: the distance from cloud (or clear of cloud and the
	// surface in sight) is not checked: SimConnect gives no cloud. Always
	// true; neither met nor failed.
	CloudUnknown bool `json:"cloudUnknown"`
}

// Check is visibilityM (flight visibility; 0 or less: not known) against
// m.
func (m VMCMinima) Check(visibilityM float64) VMCCheck {
	known := visibilityM > 0
	return VMCCheck{Minima: m, VisibilityM: visibilityM, VisibilityKnown: known,
		VisibilityOK: known && visibilityM >= m.VisibilityM, CloudUnknown: true}
}

// ControlZoneCheck is the weather at an aerodrome in a control zone for
// VFR flights taking off, landing, or entering its traffic zone or
// circuit.
type ControlZoneCheck struct {
	GroundVisibilityM float64 `json:"groundVisibilityM"`
	CeilingFt         float64 `json:"ceilingFt,omitempty"`
	// VFR: ordinary VFR is allowed, as far as known (SERA.5005(b)).
	VFR bool `json:"vfr"`
	// SpecialVFR: below the VFR minima, but ATC may clear special VFR
	// (SERA.5010(c)); false when VFR is.
	SpecialVFR bool `json:"specialVFR"`
	// VisibilityUnknown: no ground visibility given; nothing is decided by
	// it.
	VisibilityUnknown bool `json:"visibilityUnknown,omitempty"`
	// CeilingUnknown: no ceiling given (SimConnect 2024 has none): VFR and
	// SpecialVFR follow the visibility alone, and the ceiling may still
	// rule them out.
	CeilingUnknown bool `json:"ceilingUnknown,omitempty"`
}

// CheckControlZone checks a control zone aerodrome's reported ground
// visibility and ceiling (0: not known) for VFR (SERA.5005(b): 5 km and
// 1,500 ft) and, below that, special VFR (SERA.5010(c): 1,500 m, 800 m
// for helicopters, and 600 ft). Nothing not known is made up: an unknown
// value does not rule anything out, and says so.
func CheckControlZone(groundVisibilityM, ceilingFt float64, helicopter bool) ControlZoneCheck {
	c := ControlZoneCheck{GroundVisibilityM: groundVisibilityM, CeilingFt: ceilingFt,
		VisibilityUnknown: groundVisibilityM <= 0, CeilingUnknown: ceilingFt <= 0}
	atLeast := func(v, min float64) bool { return v <= 0 || v >= min }
	svfrVis := float64(SpecialVFRVisibilityM)
	if helicopter {
		svfrVis = SpecialVFRHelicopterVisibilityM
	}
	c.VFR = atLeast(groundVisibilityM, ControlZoneVisibilityM) && atLeast(ceilingFt, ControlZoneCeilingFt)
	c.SpecialVFR = !c.VFR && atLeast(groundVisibilityM, svfrVis) && atLeast(ceilingFt, SpecialVFRCeilingFt)
	return c
}
