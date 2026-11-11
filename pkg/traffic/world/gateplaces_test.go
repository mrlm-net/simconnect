package world

import (
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/addons"
	"github.com/mrlm-net/simconnect/pkg/calc"
	"github.com/mrlm-net/simconnect/pkg/jetway"
)

// A gate's measure is taken on the helper's own body: the sim parks an
// A332 helper's nose gear 5.4 m into LKPR C24's circle, the A320
// placeholders' 13.7 m; measured by the placeholder, our B738 stood 8.3 m
// behind the A332 helper's door (live 2026-11-11). Kept across starts.
func TestGatePlaceForHelperBody(t *testing.T) {
	b := newJWBed(t)
	gatePlaces.load(filepath.Join(t.TempDir(), "jetway-places.json"))
	t.Cleanup(func() { gatePlaces.load("") })
	a320, a332 := jetway.HelperTitle("A320", false), jetway.HelperTitle("A332", false)+" D-B738"
	setDoors(map[string]addons.ModelDoor{
		a320: {NoseGearFt: 12.0 / 0.3048, HasNoseGear: true},
		a332: {NoseGearFt: 22.0 / 0.3048, HasNoseGear: true},
	}, nil)
	t.Cleanup(func() { setDoors(nil, nil) })
	u := b.stand("C24")
	p := b.g.Layout.Parking[u]
	at := func(inset, nose float64) (float64, float64) { // the reference point for a nose gear inset m in
		return calc.DisplaceByHeading(p.Position.Lat, p.Position.Lon, p.Heading, p.Radius-inset-nose)
	}
	lat, lon := at(13.7, 12)
	b.cc.placeholderParked("LKPR", u, a320, lat, lon, p.Heading)
	if in, _ := b.cc.gatePlaceFor(b.g, u, a332); math.Abs(in-13.7) > 0.1 {
		t.Errorf("no A332 measured: inset %.2f, want the placeholders' 13.7", in)
	}
	lat, lon = at(5.4, 22)
	b.cc.placeholderParked("LKPR", u, a332, lat, lon, p.Heading)
	if in, _ := b.cc.gatePlaceFor(b.g, u, a332); math.Abs(in-5.4) > 0.1 {
		t.Errorf("A332 helper: inset %.2f, want its own 5.4", in)
	}
	if in, _ := b.cc.gatePlaceFor(b.g, u, a320); math.Abs(in-13.7) > 0.1 {
		t.Errorf("A320: inset %.2f, want 13.7", in)
	}
	// A new start: the A332's place from the file (saved in the background).
	file := gatePlaces.file
	for i := 0; i < 100; i++ {
		if _, ok := gatePlaces.get("LKPR", u, a332); ok {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	gatePlaces.load(file)
	c := newControlCenter(nil, b.cc.core)
	if in, _ := c.gatePlaceFor(b.g, u, a332); math.Abs(in-5.4) > 0.1 {
		t.Errorf("after a start: A332 inset %.2f, want 5.4", in)
	}
}
