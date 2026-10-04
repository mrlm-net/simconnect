package traffic

import (
	"math"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// SunElevation is the sun's elevation above the horizon at p at t, in
// degrees: the usual approximation from the fractional year (the sun's
// declination and the equation of time as short Fourier series), good to
// a fraction of a degree, plenty to tell day from night.
func SunElevation(p airport.LatLon, t time.Time) float64 {
	t = t.UTC()
	hour := float64(t.Hour()) + float64(t.Minute())/60 + float64(t.Second())/3600
	g := 2 * math.Pi / 365 * (float64(t.YearDay()-1) + (hour-12)/24)
	eqtime := 229.18 * (0.000075 + 0.001868*math.Cos(g) - 0.032077*math.Sin(g) - 0.014615*math.Cos(2*g) - 0.040849*math.Sin(2*g))
	decl := 0.006918 - 0.399912*math.Cos(g) + 0.070257*math.Sin(g) - 0.006758*math.Cos(2*g) +
		0.000907*math.Sin(2*g) - 0.002697*math.Cos(3*g) + 0.00148*math.Sin(3*g)
	solarMin := hour*60 + eqtime + 4*p.Lon // true solar time, minutes
	ha := (solarMin/4 - 180) * math.Pi / 180
	lat := p.Lat * math.Pi / 180
	cosZ := math.Sin(lat)*math.Sin(decl) + math.Cos(lat)*math.Cos(decl)*math.Cos(ha)
	return 90 - math.Acos(math.Max(-1, math.Min(1, cosZ)))*180/math.Pi
}

// DaylightSunDeg: the sun at least this high (civil twilight, 6° below the
// horizon) counts as day for VFR flights (Daylight).
const DaylightSunDeg = -6.0

// Daylight reports whether it is day at p at t (DaylightSunDeg).
func Daylight(p airport.LatLon, t time.Time) bool {
	return SunElevation(p, t) >= DaylightSunDeg
}
