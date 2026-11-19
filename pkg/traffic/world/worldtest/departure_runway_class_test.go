package worldtest

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/nav"
)

// TestDepartureRunwayByClassLKTB (#1561): at LKTB the GA stands reach only
// the grass 08/26, the main apron only the paved 09/27. With 27 in use a
// light aircraft from a GA stand takes 26, one from the main apron 27; an
// airliner from the main apron 27, and from a GA stand it is refused with
// the reason, not given a runway it cannot reach or use.
func TestDepartureRunwayByClassLKTB(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the World")
	}
	d := dataDir(t)
	if _, err := os.Stat(filepath.Join(d, "LKTB", "layout.json")); err != nil {
		t.Skip("no LKTB data")
	}
	o := Options{DataDir: d, ICAO: "LKTB", Manual: true, FrameRate: 10}
	if testing.Verbose() {
		o.World.Output = os.Stderr
	}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	h.SetUser(func(u *Object) { u.Lat, u.AltFt, u.OnGround, u.GroundKts = u.Lat+1.5, 35000, false, 450 })
	wx := nav.StaticWeather(270, 8, 9999, 100, 15, 1013)
	h.World.SetWeather(&wx)
	h.Advance(2 * time.Second)
	stand := map[string]int{}
	for i, p := range h.Layout.Parking {
		stand[p.Label()] = i
	}
	main, okMain := stand["3M"]
	ga, okGA := stand["29"]
	if !okMain || !okGA {
		t.Skip("LKTB stands 3M and 29 not in this data")
	}
	for _, c := range []struct {
		tail, model string
		stand       int
		want        string // runway, or the error's words
	}{
		{"ZRA1", "A320", main, "27"},
		{"ZRD1", "DA62", main, "27"},
		{"ZRD2", "DA62", ga, "26"},
		{"ZRA2", "A320", ga, "reaches no runway in use (27)"},
	} {
		_, err := h.World.Do(http.MethodPost, "/api/control", map[string]any{"kind": "departure", "icao": "LKTB", "tail": c.tail, "stand": c.stand, "model": c.model})
		v, _ := h.Aircraft(c.tail)
		switch {
		case len(c.want) > 2:
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("%s %s at %s: %v (runway %q), want refused: %s", c.tail, c.model, h.Layout.Parking[c.stand].Label(), err, v.Runway, c.want)
			}
		case err != nil:
			t.Errorf("%s %s at %s: %v", c.tail, c.model, h.Layout.Parking[c.stand].Label(), err)
		case v.Runway != c.want:
			t.Errorf("%s %s at %s: runway %s, want %s", c.tail, c.model, h.Layout.Parking[c.stand].Label(), v.Runway, c.want)
		}
		if err == nil {
			_, _ = h.World.Do(http.MethodPost, fmt.Sprintf("/api/control/%d/remove", v.ID), nil)
		}
		h.Advance(2 * time.Second)
	}
}
