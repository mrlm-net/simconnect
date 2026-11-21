package airport

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/mrlm-net/simconnect/pkg/types"
)

// glsProcedures is EDDF 25L as an AIP transcribes it: the ILS with its
// LEDKI transition, a GLS Z with the same transition and missed approach
// whose final is TF to the FAF and TF to the threshold at TCH 50 ft
// (threshold 110.6 m + 15.24 m), and an RNAV. No localizer frequency
// anywhere: a procedures file has none.
const glsProcedures = `{"icao":"EDDF","magVar":-3,"departures":[],"arrivals":[],"approaches":[
 {"type":"ILS","name":"ILS 25L","runway":"25L",
  "transitions":[{"name":"LEDKI","legs":[
   {"type":"IF","fix":"LEDKI","region":"ED","fixKind":"W","position":{"lat":50.16,"lon":9.08},"iaf":true,"altDesc":"AT_OR_ABOVE","alt1":1524},
   {"type":"TF","fix":"DF680","region":"ED","fixKind":"W","position":{"lat":50.12323882430792,"lon":8.937936276197433},"altDesc":"AT_OR_ABOVE","alt1":1219.2}]}],
  "final":[
   {"type":"IF","fix":"DF680","region":"ED","fixKind":"W","position":{"lat":50.12323882430792,"lon":8.937936276197433},"altDesc":"AT_OR_ABOVE","alt1":1219.2},
   {"type":"CF","fix":"DF620","region":"ED","fixKind":"W","position":{"lat":50.104936100542545,"lon":8.860122263431549},"course":246,"altDesc":"AT_OR_ABOVE","alt1":1219.2,"faf":true},
   {"type":"CF","fix":"RW25L","region":"ED","fixKind":"R","position":{"lat":50.03379486501217,"lon":8.560321480035782},"course":246,"flyOver":true,"altDesc":"AT","alt1":125.84,"map":true}],
  "missed":[{"type":"CA","course":246,"altDesc":"AT_OR_ABOVE","alt1":1524}]},
 {"type":"GLS","name":"GLS 25L Z","runway":"25L","suffix":"Z",
  "transitions":[{"name":"LEDKI","legs":[
   {"type":"IF","fix":"LEDKI","region":"ED","fixKind":"W","position":{"lat":50.16,"lon":9.08},"iaf":true,"altDesc":"AT_OR_ABOVE","alt1":1524},
   {"type":"TF","fix":"DF680","region":"ED","fixKind":"W","position":{"lat":50.12323882430792,"lon":8.937936276197433},"altDesc":"AT_OR_ABOVE","alt1":1219.2}]}],
  "final":[
   {"type":"TF","fix":"DF620","region":"ED","fixKind":"W","position":{"lat":50.104936100542545,"lon":8.860122263431549},"altDesc":"AT_OR_ABOVE","alt1":1219.2,"faf":true},
   {"type":"TF","fix":"RW25L","region":"ED","fixKind":"R","position":{"lat":50.03379486501217,"lon":8.560321480035782},"flyOver":true,"altDesc":"AT","alt1":125.84,"map":true}],
  "missed":[{"type":"CA","course":246,"altDesc":"AT_OR_ABOVE","alt1":1524}]},
 {"type":"RNAV","name":"RNAV 25L Z","runway":"25L","suffix":"Z","transitions":[],
  "final":[
   {"type":"IF","fix":"DF620","region":"ED","fixKind":"W","position":{"lat":50.104936100542545,"lon":8.860122263431549},"altDesc":"AT_OR_ABOVE","alt1":1219.2,"faf":true},
   {"type":"TF","fix":"RW25L","region":"ED","fixKind":"R","position":{"lat":50.03379486501217,"lon":8.560321480035782},"altDesc":"AT","alt1":125.84,"map":true}],
  "missed":[]}]}`

func loadGLSProcedures(t *testing.T) Procedures {
	t.Helper()
	var p Procedures
	if err := json.Unmarshal([]byte(glsProcedures), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestGLSApproach: a GLS approach is read by name and written back as
// "GLS", named "GLS 25L Z", and resolves like the ILS: transition, FAF,
// threshold at its TCH height as the missed approach point, without a
// localizer frequency.
func TestGLSApproach(t *testing.T) {
	p := loadGLSProcedures(t)
	a, ok := p.findApproach("gls 25l z")
	if !ok || a.Type != types.SIMCONNECT_FACILITY_APPROACH_TYPE_GLS {
		t.Fatalf("GLS 25L Z: %+v %v", a.Type, ok)
	}
	if got := approachName(types.SIMCONNECT_FACILITY_APPROACH_TYPE_GLS, "25L", "Z"); got != "GLS 25L Z" {
		t.Errorf("name %q", got)
	}
	b, err := json.Marshal(a)
	if err != nil || !strings.Contains(string(b), `"type":"GLS"`) {
		t.Errorf("written as %s %v", b, err)
	}

	pts, err := p.ResolveApproach("GLS 25L Z", "LEDKI")
	if err != nil {
		t.Fatal(err)
	}
	if got := idents(pts); got != "LEDKI DF680 DF620 RW25L" {
		t.Fatalf("GLS 25L Z via LEDKI: %s", got)
	}
	thr := pts[len(pts)-1]
	if !pts[0].IAF || !pts[2].FAF || !thr.MAP || math.Abs(thr.AltMin-125.84) > 0.01 || thr.AltMax != thr.AltMin {
		t.Errorf("flags or threshold %+v", pts)
	}
	if math.Abs(thr.Course-pts[2].Course) > 1 || thr.Course < 247 || thr.Course > 252 {
		t.Errorf("final course %.1f (FAF leg %.1f), want 246 magnetic, 249 true", thr.Course, pts[2].Course)
	}
	// The final alone (vectors to final): FAF, then the threshold.
	if pts, err := p.ResolveApproach("GLS 25L Z", ""); err != nil || idents(pts) != "DF620 RW25L" || !pts[1].MAP {
		t.Errorf("final: %s %v", idents(pts), err)
	}
	// Its missed approach is the ILS one, from the threshold.
	m, err := p.MissedApproach("GLS 25L Z")
	ilsM, _ := p.MissedApproach("ILS 25L")
	if err != nil || len(m) != 1 || len(ilsM) != 1 || m[0].Position != ilsM[0].Position {
		t.Errorf("missed %+v vs ILS %+v, %v", m, ilsM, err)
	}
}

// TestGLSBestApproach: GLS ranks with the ILS, ahead of RNAV: the ILS
// first where both are published, the GLS ahead of an RNAV listed
// before it, and Arrival then flies the GLS to the threshold.
func TestGLSBestApproach(t *testing.T) {
	p := loadGLSProcedures(t)
	if a, ok := p.BestApproach("25L"); !ok || a.Name != "ILS 25L" {
		t.Errorf("with the ILS: %s", a.Name)
	}
	gls, rnav := p.Approaches[1], p.Approaches[2]
	for _, list := range [][]Approach{{gls, rnav}, {rnav, gls}} {
		q := Procedures{ICAO: p.ICAO, MagVar: p.MagVar, Approaches: list}
		if a, ok := q.BestApproach("25L"); !ok || a.Name != "GLS 25L Z" {
			t.Errorf("GLS and RNAV: %s", a.Name)
		}
	}
	q := Procedures{ICAO: p.ICAO, MagVar: p.MagVar, Approaches: []Approach{rnav, gls}}
	pts, err := q.Arrival("25L", "LEDKI")
	if err != nil || idents(pts) != "LEDKI DF680 DF620 RW25L" || !pts[len(pts)-1].MAP {
		t.Errorf("arrival: %s %v", idents(pts), err)
	}
}
