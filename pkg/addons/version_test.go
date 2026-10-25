//go:build windows
// +build windows

package addons

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSteamAppVersion(t *testing.T) {
	root, lib := t.TempDir(), t.TempDir()
	write := func(p, s string) {
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// The game on a second library, as libraryfolders.vdf lists it.
	write(filepath.Join(root, "steamapps", "libraryfolders.vdf"), `"libraryfolders"
{
	"0"
	{
		"path"		"`+filepath.ToSlash(root)+`"
		"apps"
		{
			"228980"		"54886688"
		}
	}
	"1"
	{
		"path"		"`+filepath.ToSlash(lib)+`"
		"apps"
		{
			"2537590"		"10221835494"
		}
	}
}`)
	write(filepath.Join(lib, "steamapps", "appmanifest_2537590.acf"), `"AppState"
{
	"appid"		"2537590"
	"name"		"Microsoft Flight Simulator 2024"
	"buildid"		"26058316"
	"installdir"		"MSFS2024"
}`)
	v, dir := steamApp(root, "2537590")
	if v != "26058316" || filepath.Clean(dir) != filepath.Join(lib, "steamapps", "common", "MSFS2024") {
		t.Errorf("steamApp: %q %q", v, dir)
	}
	if v, _ := steamApp(root, "1250410"); v != "" {
		t.Errorf("2020 not installed: %q", v)
	}
}

func TestStorePackageVersion(t *testing.T) {
	for name, want := range map[string]string{
		"Microsoft.Limitless_1.2.13.0_x64__8wekyb3d8bbwe":                    "1.2.13.0",
		"Microsoft.Limitless_1.2.13.0_neutral_split.scale-100_8wekyb3d8bbwe": "",
		"Microsoft.LimitlessBeta_1.0.0.0_x64__8wekyb3d8bbwe":                 "",
		"Microsoft.FlightSimulator_1.37.19.0_x64__8wekyb3d8bbwe":             "",
	} {
		if got, _ := storePackageVersion(name, "Microsoft.Limitless"); got != want {
			t.Errorf("%s: %q, want %q", name, got, want)
		}
	}
}

func TestSDK(t *testing.T) {
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, "version.txt"), []byte("1.7.3\r\n0\r\n"), 0o644)
	t.Setenv("MSFS2024_SDK", d)
	if dir, v := SDK(); dir != d || v != "1.7.3" {
		t.Errorf("SDK: %q %q", dir, v)
	}
	t.Setenv("MSFS2024_SDK", "")
	t.Setenv("MSFS_SDK", "")
	if dir, v := SDK(); dir != "" || v != "" {
		t.Errorf("no SDK: %q %q", dir, v)
	}
}

// On this machine: what Installs and SDK read (a log, not a check).
func TestInstallsHere(t *testing.T) {
	for _, in := range Installs() {
		t.Logf("%s %s: version %q dir %q", in.Sim, in.Store, in.Version, in.Dir)
	}
	dir, v := SDK()
	t.Logf("SDK %q %q; Store lookup (Calculator): %q", dir, v, func() string { v, _ := storeApp("Microsoft.WindowsCalculator"); return v }())
}
