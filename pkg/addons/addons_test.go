//go:build windows
// +build windows

package addons

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// packagesTree builds a small packages folder shaped like MSFS 2024's.
func packagesTree(t *testing.T) string {
	root := t.TempDir()
	write(t, filepath.Join(root, "Community", "fsdreamteam-gsx-pro", "manifest.json"),
		`{"content_type":"SCENERY","title":"GSX Pro","creator":"Fsdreamteam","package_version":"4.0.23"}`)
	write(t, filepath.Join(root, "Community", "fnx-aircraft-319-321", "manifest.json"),
		`{"content_type":"AIRCRAFT","title":"Fenix A319","creator":"Fenix","package_version":"2.1.0"}`)
	write(t, filepath.Join(root, "Community", "fnx-aircraft-319-321", "layout.json"),
		`{"content":[{"path":"SimObjects/Airplanes/FNX_32X/presets/fnx/FNX_319_CFM_WF_HD/config/aircraft.cfg","size":1,"date":1}]}`)
	write(t, filepath.Join(root, "Community", "broken", "manifest.json"), `{not json`)
	write(t, filepath.Join(root, "StreamedPackages", "fs20-orbx-airport-lkpr-prague", "content", "minimal.fsarchive"), "x")
	write(t, filepath.Join(root, "StreamedPackages", "fs20-aerosoft-airport-lktb-brno", "content", "minimal.fsarchive"), "x")
	write(t, filepath.Join(root, "StreamedPackages", "fs20-aerosoft-airport-lktb-brno", "content", "SimObjects", "njuvoctp.fsarchive"), "x")
	write(t, filepath.Join(root, "StreamedPackages", "fs20-asobo-aircraft-baron-g58", "content", "SimObjects", "Airplanes", "Asobo_Baron_G58", "soundAI", "x.PCK"), "x")
	return root
}

func TestReadPackagesPath(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "UserCfg.opt")
	write(t, cfg, "Version 1\r\n\tInstalledPackagesPath \"C:\\Sim\\Packages\"\r\nOther 2\r\n")
	got, err := ReadPackagesPath(cfg)
	if err != nil || got != `C:\Sim\Packages` {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestScan(t *testing.T) {
	pkgs, err := Scan(packagesTree(t))
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]Package{}
	for _, p := range pkgs {
		by[p.Folder] = p
	}
	if len(pkgs) != 6 {
		t.Fatalf("%d packages, want 6: %+v", len(pkgs), pkgs)
	}
	if g := by["fsdreamteam-gsx-pro"]; g.Source != Community || g.Title != "GSX Pro" || g.ContentType != "SCENERY" || g.Version != "4.0.23" || g.Creator != "Fsdreamteam" {
		t.Errorf("gsx: %+v", g)
	}
	if b := by["broken"]; b.Title != "" || b.Source != Community {
		t.Errorf("broken manifest: %+v", b)
	}
	if p := by["fs20-orbx-airport-lkpr-prague"]; p.Source != Streamed || p.Publisher != "orbx" || p.ICAO != "LKPR" || p.Cached != 0 {
		t.Errorf("lkpr: %+v", p)
	}
	if p := by["fs20-aerosoft-airport-lktb-brno"]; p.ICAO != "LKTB" || p.Cached != 1 {
		t.Errorf("lktb: %+v", p)
	}
	if _, err := Scan(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("missing packages path: want an error")
	}
}

func TestParseStreamedName(t *testing.T) {
	for _, c := range []struct{ name, pub, icao string }{
		{"fs20-orbx-airport-lkpr-prague", "orbx", "LKPR"},
		{"fs20-aerosoft-airport-lktb-brno", "aerosoft", "LKTB"},
		{"fs20-gaya-simulations-airport-loww-vienna", "gaya-simulations", "LOWW"},
		{"fs24-asobo-airport-c53-lowerloon", "asobo", "C53"},
		{"fs24-asobo-airport-keb-nanwalek", "asobo", "KEB"},
		{"fs20-fps-lkmt-ostrava-airport", "fps", "LKMT"},
		{"fs20-microsoft-airport-voloport", "", ""},
		{"fs20-microsoft-modellib-airport-glider", "", ""},
		{"fs20-asobo-aircraft-baron-g58", "", ""},
		{"fs20-aerosoft-paderborn", "", ""},
	} {
		pub, icao, ok := ParseStreamedName(c.name)
		if pub != c.pub || icao != c.icao || ok != (c.icao != "") {
			t.Errorf("%s: %q %q %v, want %q %q", c.name, pub, icao, ok, c.pub, c.icao)
		}
	}
}

func TestFingerprint(t *testing.T) {
	a := []Package{{Source: Community, Folder: "a", Version: "1"}, {Source: Streamed, Folder: "b"}}
	b := []Package{{Source: Streamed, Folder: "b", Cached: 3}, {Source: Community, Folder: "A", Version: "1"}}
	if Fingerprint(a) != Fingerprint(b) {
		t.Error("order, case or cached content changed the fingerprint")
	}
	a[0].Version = "2"
	if Fingerprint(a) == Fingerprint(b) {
		t.Error("a version change kept the fingerprint")
	}
}

func TestAircraftPackage(t *testing.T) {
	pkgs, _ := Scan(packagesTree(t))
	for _, c := range []struct{ path, want string }{
		{`SimObjects\Airplanes\FNX_32X\presets\fnx\FNX_319_CFM_WF_HD\config\aircraft.CFG`, "fnx-aircraft-319-321"},
		{`SimObjects\Airplanes\Asobo_Baron_G58\aircraft.CFG`, "fs20-asobo-aircraft-baron-g58"},
		{`SimObjects\Airplanes\Nothing\aircraft.CFG`, ""},
		{"", ""},
	} {
		p, ok := AircraftPackage(pkgs, c.path)
		if p.Folder != c.want || ok != (c.want != "") {
			t.Errorf("%s: %q %v, want %q", c.path, p.Folder, ok, c.want)
		}
	}
}

func TestProcesses(t *testing.T) {
	names, err := Processes()
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Base(os.Args[0])
	for _, n := range names {
		if n == exe {
			return
		}
	}
	t.Errorf("%s not among %d processes", exe, len(names))
}

// TestInstalled scans this machine's simulator when there is one.
func TestInstalled(t *testing.T) {
	in, err := Find()
	if err != nil {
		t.Skip("no simulator installed")
	}
	pkgs, err := Scan(in.Packages)
	if err != nil {
		t.Fatal(err)
	}
	airports := 0
	for _, p := range pkgs {
		if p.ICAO != "" {
			airports++
		}
	}
	t.Logf("MSFS %s %s: %d packages, %d streamed airports, fingerprint %s", in.Sim, in.Store, len(pkgs), airports, Fingerprint(pkgs)[:12])
}

// TestLayoutCache: a layout.json is read once while unchanged, and again
// when it changes (E13).
func TestLayoutCache(t *testing.T) {
	file := filepath.Join(t.TempDir(), "layout.json")
	write(t, file, `{"content":[{"path":"SimObjects/Airplanes/A/aircraft.cfg"},{"path":"scenery/x.bgl"}]}`)
	if p := layoutSimObjects(file); !p["simobjects/airplanes/a/aircraft.cfg"] || len(p) != 1 {
		t.Fatalf("paths %v", p)
	}
	layoutCache.Lock()
	e := layoutCache.m[file]
	e.paths = map[string]bool{"cached": true}
	layoutCache.m[file] = e
	layoutCache.Unlock()
	if !layoutSimObjects(file)["cached"] {
		t.Error("read again while unchanged")
	}
	write(t, file, `{"content":[{"path":"SimObjects/Airplanes/B/aircraft.cfg"},{"path":"SimObjects/Airplanes/B/model.cfg"}]}`)
	if p := layoutSimObjects(file); !p["simobjects/airplanes/b/aircraft.cfg"] {
		t.Errorf("changed file not read again: %v", p)
	}
}
