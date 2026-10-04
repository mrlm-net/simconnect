package traffic

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// phasePicture is a picture around LKPR with its layout, and a scan helper:
// one aircraft observed at each position in turn, every 2 s.
func phasePicture(t *testing.T) (*TrafficPicture, *airport.Layout) {
	t.Helper()
	l := lkprGraph(t).Layout
	p := NewTrafficPicture(PictureOptions{Centre: Centre{ICAO: "LKPR"}, RadiusNM: 150,
		Layout: func(icao string) *airport.Layout {
			if icao == "LKPR" {
				return l
			}
			return nil
		}})
	p.SetAirports([]AirportRef{pictureLKPR})
	return p, l
}

func observeTrack(p *TrafficPicture, start time.Time, obs []Observation) TrackedAircraft {
	for i, o := range obs {
		o.ObjectID = 1
		p.Observe(start.Add(time.Duration(i)*2*time.Second), []Observation{o})
	}
	return p.Aircraft()[0]
}

// #622: AI taxiing report 0 kt; the speed comes from their movement, and a
// metre of jitter is not movement.
func TestPicturePhaseDerivedSpeed(t *testing.T) {
	p, l := phasePicture(t)
	now := time.Now()
	// On taxiway (a node off the stands and runways), 15 m every 2 s, reported 0 kt.
	var node airport.LatLon
	for _, tp := range l.TaxiPoints {
		loc, _ := airport.Locate(airport.LocateQuery{Position: tp.Position, OnGround: true}, []*airport.Layout{l})
		then, _ := airport.Locate(airport.LocateQuery{Position: offsetHeading(tp.Position, 90, 15), OnGround: true}, []*airport.Layout{l})
		if loc.Feature == airport.OnTaxiway && then.Feature == airport.OnTaxiway {
			node = tp.Position
			break
		}
	}
	a := observeTrack(p, now, []Observation{
		{Position: node, OnGround: true, Heading: 90},
		{Position: offsetHeading(node, 90, 15), OnGround: true, Heading: 90},
	})
	if !a.SpeedDerived || a.GroundKts < 13 || a.GroundKts > 16 || a.Phase != PhaseTaxiing || a.Where != airport.OnTaxiway || a.Airport != "LKPR" {
		t.Errorf("taxiing at 0 kt reported: %+v", a)
	}
	// A metre of jitter off the stands: holding, not taxiing.
	p, _ = phasePicture(t)
	a = observeTrack(p, now, []Observation{
		{Position: node, OnGround: true, Heading: 90},
		{Position: offsetHeading(node, 0, 1), OnGround: true, Heading: 90},
	})
	if a.Phase != PhaseHolding {
		t.Errorf("a metre of jitter on a taxiway: %s, want holding", a.Phase)
	}
}

// #623: pushback (moving tail first), parked on a stand creeping in, the
// take-off and landing rolls on the runway.
func TestPicturePhaseGround(t *testing.T) {
	_, l := phasePicture(t)
	now := time.Now()
	st := l.ParkingByLabel("C22")[0]
	p, _ := phasePicture(t)
	a := observeTrack(p, now, []Observation{
		{Position: st.Position, OnGround: true, Heading: st.Heading},
		{Position: offsetHeading(st.Position, st.Heading+180, 4), OnGround: true, Heading: st.Heading},
	})
	if a.Phase != PhasePushback {
		t.Errorf("moving tail first off C22: %s, want pushback", a.Phase)
	}
	p, _ = phasePicture(t)
	a = observeTrack(p, now, []Observation{
		{Position: st.Position, OnGround: true, Heading: st.Heading},
		{Position: offsetHeading(st.Position, st.Heading, 2), OnGround: true, Heading: st.Heading},
	})
	if a.Phase != PhaseParked || a.Where != airport.AtParking || a.WhereName != "C22" {
		t.Errorf("creeping 2 m on C22: %s on %s %q, want parked on C22", a.Phase, a.Where, a.WhereName)
	}
	r := l.Runways[0]
	p, _ = phasePicture(t)
	a = observeTrack(p, now, []Observation{
		{Position: r.Center, OnGround: true, GroundKts: 80, Heading: r.Heading},
		{Position: offsetHeading(r.Center, r.Heading, 100), OnGround: true, GroundKts: 100, Heading: r.Heading},
	})
	if a.Phase != PhaseTakeoff {
		t.Errorf("fast along the runway: %s, want takeoff", a.Phase)
	}
	p, _ = phasePicture(t)
	a = observeTrack(p, now, []Observation{
		{Position: offsetHeading(r.Center, r.Heading+180, 300), AGLFt: 30, AltFt: 1250, Heading: r.Heading},
		{Position: r.Center, OnGround: true, GroundKts: 120, Heading: r.Heading},
	})
	if a.Phase != PhaseLanding {
		t.Errorf("rolling just after touching down: %s, want landing", a.Phase)
	}
}

// #623: the vertical phase follows the altitude trend, not one scan: a
// blip does not turn a cruise into a climb; an approach lasts while it
// levels off low.
func TestPicturePhaseAir(t *testing.T) {
	p, _ := phasePicture(t)
	now := time.Now()
	far := offsetHeading(pictureLKPR.Position, 0, 80*1852)
	var obs []Observation
	for i := range 16 {
		alt := 35000.0
		if i == 10 {
			alt = 35080 // a blip
		}
		obs = append(obs, Observation{Position: far, AltFt: alt, AGLFt: alt - 1000, VSFpm: 2400 * float64(i%2)})
	}
	if a := observeTrack(p, now, obs); a.Phase != PhaseEnroute {
		t.Errorf("level with a blip: %s, want enroute", a.Phase)
	}
	p, _ = phasePicture(t)
	obs = nil
	for i := range 16 {
		alt := 20000 + 33*float64(i*2) // 1000 fpm
		obs = append(obs, Observation{Position: far, AltFt: alt, AGLFt: alt - 1000, VSFpm: 1000})
	}
	if a := observeTrack(p, now, obs); a.Phase != PhaseClimbing {
		t.Errorf("climbing far from airports: %s, want climbing", a.Phase)
	}
	// Descending to 2500 ft on final, then level: still the approach.
	p, _ = phasePicture(t)
	final := offsetHeading(pictureLKPR.Position, 64, 6*1852)
	obs = nil
	for i := range 16 {
		alt := 3700 - 25*float64(i*2) // 750 fpm down
		obs = append(obs, Observation{Position: final, AltFt: alt, AGLFt: alt - 1200, VSFpm: -750})
	}
	for range 10 {
		obs = append(obs, Observation{Position: final, AltFt: 2900, AGLFt: 1700})
	}
	if a := observeTrack(p, now, obs); a.Phase != PhaseApproach || a.Airport != "LKPR" {
		t.Errorf("levelled off low on final: %s at %q, want approach at LKPR", a.Phase, a.Airport)
	}
}

// An arriving aircraft belongs to the airport it heads for, not the nearest
// one within the terminal distance (BAW1989 at 8,000 ft read LKKQ).
func TestPictureFlightAirport(t *testing.T) {
	now := time.Now()
	east := offsetHeading(pictureLKPR.Position, 90, 20*1852)                      // 20 NM east of LKPR
	side := AirportRef{ICAO: "LKSD", Position: offsetHeading(east, 0, 5*1852)}    // 5 NM north of it
	front := AirportRef{ICAO: "LKFR", Position: offsetHeading(east, 270, 8*1852)} // 8 NM ahead, no layout
	descend := func(o Observation) []Observation {
		var obs []Observation
		for i := range 16 {
			alt := 9000 - 50*float64(i*2) // 1500 fpm down
			o.AltFt, o.AGLFt, o.VSFpm = alt, alt-1000, -1500
			obs = append(obs, o)
		}
		return obs
	}
	for _, c := range []struct {
		name     string
		airports []AirportRef
		obs      Observation
		want     string
	}{
		{"nearest is off to the side", []AirportRef{pictureLKPR, side}, Observation{Position: east, Heading: 270}, "LKPR"},
		{"a nearer one ahead has no layout", []AirportRef{pictureLKPR, side, front}, Observation{Position: east, Heading: 270}, "LKPR"},
		{"its destination", []AirportRef{pictureLKPR, side}, Observation{Position: east, Heading: 270, To: "LKSD"}, "LKSD"},
		{"destination out of range", []AirportRef{pictureLKPR, side, pictureLOWW}, Observation{Position: east, Heading: 270, To: "LOWW"}, "LKPR"},
		{"nothing ahead", []AirportRef{pictureLKPR, side}, Observation{Position: east, Heading: 90}, "LKSD"},
	} {
		p, _ := phasePicture(t)
		p.SetAirports(c.airports)
		a := observeTrack(p, now, descend(c.obs))
		if a.Phase != PhaseArriving || a.Airport != c.want {
			t.Errorf("%s: %s at %q, want arriving at %s", c.name, a.Phase, a.Airport, c.want)
		}
	}
	// Climbing out: the airport behind it.
	p, _ := phasePicture(t)
	p.SetAirports([]AirportRef{pictureLKPR, side})
	var obs []Observation
	for i := range 16 {
		alt := 3000 + 50*float64(i*2)
		obs = append(obs, Observation{Position: east, Heading: 90, AltFt: alt, AGLFt: alt - 1000, VSFpm: 1500})
	}
	if a := observeTrack(p, now, obs); a.Phase != PhaseDeparting || a.Airport != "LKPR" {
		t.Errorf("climbing out east: %s at %q, want departing LKPR", a.Phase, a.Airport)
	}
}

// MSFS reports FSLTL AI on short final climbing (live, BAW1989 at LKPR:
// +500…+940 fpm while descending about 1,100 fpm); the phase and the
// vertical speed come from the altitude.
func TestPictureVerticalSpeedFromAltitude(t *testing.T) {
	p, _ := phasePicture(t)
	final := offsetHeading(pictureLKPR.Position, 64, 3*1852)
	var obs []Observation
	for i := range 10 {
		alt := 2200 - 37*float64(i) // 37 ft a 2 s scan: about 1,110 fpm down
		obs = append(obs, Observation{Position: final, Heading: 244, AltFt: alt, AGLFt: alt - 1200, VSFpm: 600 + 40*float64(i)})
	}
	a := observeTrack(p, time.Now(), obs)
	if a.Phase != PhaseApproach || a.Airport != "LKPR" {
		t.Errorf("short final reported climbing: %s at %q, want approach at LKPR", a.Phase, a.Airport)
	}
	if !a.VSDerived || a.VSFpm > -1000 || a.VSFpm < -1200 {
		t.Errorf("vertical speed %.0f fpm (derived %v), want about -1110", a.VSFpm, a.VSDerived)
	}
}

// Lined up with a runway: that airport, though a nearer one with a layout
// is ahead too (live: OKLTU on LKPR 06's centreline read LKHY, the app
// having layouts for 40 airports around).
func TestPictureFlightAirportAligned(t *testing.T) {
	p, l := phasePicture(t)
	var end airport.RunwayEnd
	for _, rw := range l.Runways {
		for _, e := range []airport.RunwayEnd{rw.Primary, rw.Secondary} {
			if e.Name == "06" {
				end = e
			}
		}
	}
	out := offsetHeading(end.Threshold, end.Heading+180, 14*1852) // 14 NM final
	other := AirportRef{ICAO: "LKHY", Position: offsetHeading(out, end.Heading+40, 6*1852)}
	small := &airport.Layout{ICAO: "LKHY", Runways: []airport.Runway{{Length: 2500,
		Primary: airport.RunwayEnd{Name: "15", Heading: 150, Threshold: other.Position}, Secondary: airport.RunwayEnd{Name: "33", Heading: 330, Threshold: other.Position}}}}
	p.opts.Layout = func(icao string) *airport.Layout {
		return map[string]*airport.Layout{"LKPR": l, "LKHY": small}[icao]
	}
	p.SetAirports([]AirportRef{pictureLKPR, other})
	var obs []Observation
	for i := range 16 {
		alt := 5800 - 16*float64(i) // about 500 fpm down
		obs = append(obs, Observation{Position: out, Heading: end.Heading, AltFt: alt, AGLFt: alt - 1200, VSFpm: -970})
	}
	if a := observeTrack(p, time.Now(), obs); a.Phase != PhaseArriving || a.Airport != "LKPR" {
		t.Errorf("on LKPR 06's centreline: %s at %q, want arriving at LKPR", a.Phase, a.Airport)
	}
}
