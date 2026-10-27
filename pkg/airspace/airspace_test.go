package airspace

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

// store opens the offline airspaces (a simconnect-data checkout):
// $SIMCONNECT_DATA, else beside this repository's checkout. The test skips
// without it.
func store(t *testing.T, opts ...StoreOption) *Store {
	t.Helper()
	d := os.Getenv("SIMCONNECT_DATA")
	if d == "" {
		d = filepath.Join("..", "..", "..", "simconnect-data")
	}
	dir := filepath.Join(d, "airspaces")
	if _, err := os.Stat(filepath.Join(dir, "index.json")); err != nil {
		t.Skipf("no offline airspaces at %s (SIMCONNECT_DATA)", dir)
	}
	s, err := Open(os.DirFS(dir), opts...)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

var lkpr = Point{Lat: 50.1008, Lon: 14.26}

// TestRuzyneCTR: LKPR is in Ruzyne CTR, class D, GND–3,500 ft MSL; once
// per data set, once with StorePreferNewerSet (the newer).
func TestRuzyneCTR(t *testing.T) {
	for _, c := range []struct {
		opts []StoreOption
		want int
	}{{nil, 2}, {[]StoreOption{StorePreferNewerSet()}, 1}} {
		s := store(t, c.opts...)
		at, err := s.At(lkpr.Lat, lkpr.Lon)
		if err != nil {
			t.Fatal(err)
		}
		var ctr []*Airspace
		for _, a := range at {
			if a.Name == "RUZYNE CTR" {
				ctr = append(ctr, a)
			}
		}
		if len(ctr) != c.want {
			t.Fatalf("RUZYNE CTR %d times at LKPR, want %d", len(ctr), c.want)
		}
		for _, a := range ctr {
			if a.Type != ClassD || a.MinAlt != 0 || math.Abs(a.MaxAlt-1066.8) > 0.01 || a.MinAltType != AltMSL || a.MaxAltType != AltMSL {
				t.Errorf("RUZYNE CTR (set %d): %v %.1f–%.1f m, types %d/%d", a.DataSet(), a.Type, a.MinAlt, a.MaxAlt, a.MinAltType, a.MaxAltType)
			}
			sh, err := a.Shape()
			if err != nil {
				t.Fatal(err)
			}
			if !sh.Contains(lkpr.Lat, lkpr.Lon, 400, 380) || sh.Contains(lkpr.Lat, lkpr.Lon, 1100, 380) {
				t.Errorf("RUZYNE CTR (set %d): 400 m in, 1,100 m out", a.DataSet())
			}
			// Brno, Kladno-north and Melnik are outside.
			for _, p := range []Point{{49.151, 16.694}, {50.30, 14.10}, {50.35, 14.47}} {
				if sh.Inside(p.Lat, p.Lon) {
					t.Errorf("RUZYNE CTR (set %d) holds %v", a.DataSet(), p)
				}
			}
		}
		if c.want == 1 && ctr[0].DataSet() < 9000 {
			t.Errorf("kept the older set %d", ctr[0].DataSet())
		}
	}
}

// TestCircle: the USTI NAD LABEM restricted area is a 926 m circle: its
// centre in, 1,000 m off out.
func TestCircle(t *testing.T) {
	s := store(t)
	list, err := s.Area(BoxAround(50.6564, 14.0202, 3000))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range list {
		if a.Name != "USTI NAD LABEM" || len(a.Vectors) != 2 || a.Vectors[1].Type != VectorCircle {
			continue
		}
		found = true
		sh, err := a.Shape()
		if err != nil {
			t.Fatal(err)
		}
		c := Point{a.Vectors[0].Lat, a.Vectors[0].Lon}
		if !sh.Inside(c.Lat, c.Lon) || !sh.Inside(c.Lat+800/metersPerDeg, c.Lon) || sh.Inside(c.Lat+1000/metersPerDeg, c.Lon) {
			t.Errorf("circle: centre/800 m in, 1,000 m out failed")
		}
	}
	if !found {
		t.Fatal("no USTI NAD LABEM circle")
	}
}

// TestWideBoxes: an area at the antimeridian finds the airspaces whose
// box spans -180 to 180 (Magadan CTA), and an area in the Atlantic does
// not pick up a box that only touches 180.
func TestWideBoxes(t *testing.T) {
	s := store(t)
	var magadan, wide int
	q := BoxAround(65, -175, 50000)
	for _, e := range s.Index() {
		b := e.Box()
		if b.wide() {
			wide++
		}
		if e.Name == "MAGADAN CTA" && b.Overlaps(q) {
			magadan++
		}
	}
	if wide == 0 || magadan == 0 {
		t.Errorf("%d wide boxes, Magadan CTA %d times near 65N 175W", wide, magadan)
	}
	if (Box{North: 70, South: 60, West: 170, East: 180}).Overlaps(BoxAround(65, -30, 50000)) {
		t.Error("a box at 170..180 E overlaps the Atlantic")
	}
	if !(Box{North: 10, South: 0, West: 170, East: -170}).Overlaps(Box{North: 5, South: 4, West: -175, East: -174}) {
		t.Error("a box across 180 misses a point at 175 W")
	}
}

// TestTileShapes: every unbroken record of the Czech tile builds an
// outline that holds its first point's neighbourhood somewhere and lies
// in its box.
func TestTileShapes(t *testing.T) {
	s := store(t)
	box := Box{North: 51, South: 48.5, West: 12, East: 19}
	list, err := s.Area(box)
	if err != nil {
		t.Fatal(err)
	}
	want := 0
	for _, e := range s.Index() {
		if e.Box().Overlaps(box) {
			want++
		}
	}
	if len(list) != want || want < 500 {
		t.Fatalf("%d airspaces over Czechia, the index has %d", len(list), want)
	}
	out := 0
	for _, a := range list {
		if a.Broken() {
			continue
		}
		sh, err := a.Shape()
		if err != nil {
			t.Errorf("%s: %v", a.Name, err)
			continue
		}
		b := a.Box()
		if b.wide() {
			continue // the outline runs on past 180
		}
		// The simulator's box does not always hold an arc's full sweep
		// (POLICKA ATZ: a 345° clockwise arc, the box stops at its end
		// point's longitude): a few may stick out, not many.
		const tol = 0.02
	rings:
		for _, r := range sh.Rings {
			for _, p := range r {
				if p.Lat > b.North+tol || p.Lat < b.South-tol || p.Lon < b.West-tol || p.Lon > b.East+tol {
					out++
					t.Logf("%s (%d): point %v outside its box %+v", a.Name, a.ID, p, b)
					break rings
				}
			}
		}
	}
	if out > len(list)/100 {
		t.Errorf("%d of %d outlines outside their box", out, len(list))
	}
}

// TestArc: a clockwise quarter round a centre from north to east passes
// north-east at the radius; counter-clockwise it goes the long way round.
func TestArc(t *testing.T) {
	c := Point{50, 14}
	kx := math.Cos(50 * math.Pi / 180)
	r := 0.1
	n, e := Point{50 + r, 14}, Point{50, 14 + r/kx}
	cw := arc(c, n, e, true)
	ccw := arc(c, n, e, false)
	if len(ccw) < 2*len(cw) {
		t.Errorf("CCW %d points, CW %d: not the long way", len(ccw), len(cw))
	}
	mid := cw[len(cw)/2]
	if mid.Lat < 50 || mid.Lon < 14 || math.Abs(math.Hypot(mid.Lat-50, (mid.Lon-14)*kx)-r) > 1e-9 {
		t.Errorf("CW mid point %v: not north-east at the radius", mid)
	}
	if cw[len(cw)-1] != e {
		t.Error("does not end on its point")
	}
}

// TestAntimeridianRing: a square across 180 holds points either side.
func TestAntimeridianRing(t *testing.T) {
	a := &Airspace{MinAltType: AltMSL, MaxAltType: AltMSL, MaxAlt: 1000, Vectors: []Vector{
		{Type: VectorStart, Lat: 10, Lon: 175}, {Type: VectorLine, Lat: 10, Lon: -175},
		{Type: VectorLine, Lat: 0, Lon: -175}, {Type: VectorLine, Lat: 0, Lon: 175},
	}}
	sh, err := a.Shape()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		lon float64
		in  bool
	}{{179, true}, {-179, true}, {170, false}, {-170, false}, {0, false}} {
		if got := sh.Inside(5, c.lon); got != c.in {
			t.Errorf("5N %v: inside %v, want %v", c.lon, got, c.in)
		}
	}
	if (&Airspace{MinAltType: AltMSL, MaxAltType: AltMSL, Vectors: []Vector{{Type: VectorLine, Lat: 1, Lon: 1}}}).Broken() != true {
		t.Error("no start vector: not broken")
	}
	if a.Broken() {
		t.Error("square: broken")
	}
}

// TestAGLFloor: an AGL floor is over the ground given.
func TestAGLFloor(t *testing.T) {
	a := &Airspace{MinAlt: 300, MinAltType: AltAGL, MaxAlt: 3000, MaxAltType: AltMSL}
	if a.Floor(400) != 700 || a.Ceiling(400) != 3000 {
		t.Errorf("floor %v ceiling %v over 400 m ground", a.Floor(400), a.Ceiling(400))
	}
}
