//go:build windows
// +build windows

package addons

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

// The simulator's version and folder with the simulator closed: Steam's
// app manifest (its build id), the Store's package (its version). The SDK
// from its environment variable. Files and the registry only.

// steamAppIDs are the simulators' Steam app ids.
var steamAppIDs = map[string]string{"2024": "2537590", "2020": "1250410"}

// storeFamilies are the simulators' Store package names (before the
// version) and publisher id.
var storeFamilies = map[string]string{"2024": "Microsoft.Limitless", "2020": "Microsoft.FlightSimulator"}

const storePublisher = "8wekyb3d8bbwe"

// sdkVars are the SDK's environment variables, the newest first.
var sdkVars = []struct{ sim, name string }{{"2024", "MSFS2024_SDK"}, {"2020", "MSFS_SDK"}}

// fillVersion sets in.Version and in.Dir where they can be read; "" else.
func fillVersion(in *Install) {
	switch in.Store {
	case "steam":
		if id, ok := steamAppIDs[in.Sim]; ok {
			in.Version, in.Dir = steamApp(steamRoot(), id)
		}
	case "store":
		if fam, ok := storeFamilies[in.Sim]; ok {
			in.Version, in.Dir = storeApp(fam)
		}
	}
}

// SDK is the MSFS SDK named by its environment variable (MSFS2024_SDK,
// else MSFS_SDK for 2020) and its version (the first line of version.txt
// in it); "" when it is not installed or says none.
func SDK() (dir, version string) {
	for _, v := range sdkVars {
		d := strings.TrimSpace(os.Getenv(v.name))
		if d == "" {
			continue
		}
		b, _ := os.ReadFile(filepath.Join(d, "version.txt"))
		first, _, _ := strings.Cut(string(b), "\n") // "1.7.3", then a line more
		return d, strings.TrimSpace(first)
	}
	return "", ""
}

// steamRoot is Steam's folder: HKCU\Software\Valve\Steam SteamPath, else
// the default.
func steamRoot() string {
	if p := regString(hkeyCurrentUser, `Software\Valve\Steam`, "SteamPath"); p != "" {
		return filepath.Clean(p)
	}
	return `C:\Program Files (x86)\Steam`
}

// steamApp is app id's build id and game folder: the library listing it in
// root's libraryfolders.vdf, its steamapps/appmanifest_<id>.acf.
func steamApp(root, appID string) (version, dir string) {
	b, err := os.ReadFile(filepath.Join(root, "steamapps", "libraryfolders.vdf"))
	if err != nil {
		return "", ""
	}
	libs := []string{root}
	if folders, ok := parseVDF(string(b))["libraryfolders"].(map[string]any); ok {
		libs = nil
		for _, f := range folders {
			lib, ok := f.(map[string]any)
			if !ok {
				continue
			}
			path, _ := lib["path"].(string)
			if apps, _ := lib["apps"].(map[string]any); path != "" && apps[appID] != nil {
				libs = append([]string{path}, libs...) // the one listing it first
			} else if path != "" {
				libs = append(libs, path)
			}
		}
	}
	for _, lib := range libs {
		b, err := os.ReadFile(filepath.Join(lib, "steamapps", "appmanifest_"+appID+".acf"))
		if err != nil {
			continue
		}
		st, _ := parseVDF(string(b))["AppState"].(map[string]any)
		build, _ := st["buildid"].(string)
		inst, _ := st["installdir"].(string)
		if inst != "" {
			dir = filepath.Join(lib, "steamapps", "common", inst)
		}
		return build, dir
	}
	return "", ""
}

// parseVDF reads Valve's KeyValues text ("key" "value", "key" { ... }):
// values are strings, sections maps. Malformed input gives what was read.
func parseVDF(s string) map[string]any {
	var toks []string
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '{' || c == '}':
			toks = append(toks, string(c))
		case c == '"':
			var b strings.Builder
			for i++; i < len(s) && s[i] != '"'; i++ {
				if s[i] == '\\' && i+1 < len(s) {
					i++
				}
				b.WriteByte(s[i])
			}
			toks = append(toks, "\x00"+b.String()) // a string, not a brace
		case c == '/' && i+1 < len(s) && s[i+1] == '/':
			for i < len(s) && s[i] != '\n' {
				i++
			}
		}
	}
	var section func(i int) (map[string]any, int)
	section = func(i int) (map[string]any, int) {
		m := map[string]any{}
		for i < len(toks) {
			t := toks[i]
			if t == "}" {
				return m, i + 1
			}
			if t == "{" || i+1 >= len(toks) {
				return m, len(toks)
			}
			key := t[1:]
			if toks[i+1] == "{" {
				m[key], i = section(i + 2)
				continue
			}
			if toks[i+1] == "}" {
				return m, i + 2
			}
			m[key] = toks[i+1][1:]
			i += 2
		}
		return m, i
	}
	m, _ := section(0)
	return m
}

const packagesKey = `Software\Classes\Local Settings\Software\Microsoft\Windows\CurrentVersion\AppModel\Repository\Packages`

// storeApp is the newest installed version of a Store package family and
// its folder: the per-user package repository's keys, named
// Name_Version_Arch__Publisher, with PackageRootFolder.
func storeApp(family string) (version, dir string) {
	var best []int
	for _, k := range regSubkeys(hkeyCurrentUser, packagesKey) {
		v, ok := storePackageVersion(k, family)
		if !ok {
			continue
		}
		if n := versionParts(v); best == nil || slices.Compare(n, best) > 0 {
			best, version = n, v
			dir = regString(hkeyCurrentUser, packagesKey+`\`+k, "PackageRootFolder")
		}
	}
	return version, dir
}

// storePackageVersion is the version in a package full name of family,
// its main package (not a resource or framework split): ok false else.
func storePackageVersion(fullName, family string) (string, bool) {
	rest, ok := strings.CutPrefix(fullName, family+"_")
	if !ok || !strings.HasSuffix(fullName, "__"+storePublisher) {
		return "", false
	}
	p := strings.SplitN(rest, "_", 3)
	if len(p) < 3 || p[1] == "neutral" || strings.Contains(p[2], "split") {
		return "", false
	}
	return p[0], true
}

func versionParts(v string) []int {
	var n []int
	for _, s := range strings.Split(v, ".") {
		x, _ := strconv.Atoi(s)
		n = append(n, x)
	}
	return n
}

// The registry through advapi32 (the standard library has no registry
// package; golang.org/x/sys is a dependency).
const (
	hkeyCurrentUser = uintptr(0x80000001)
	keyRead         = 0x20019
	rrfRTRegSZ      = 0x00000002
)

var (
	advapi32        = syscall.NewLazyDLL("advapi32.dll")
	procRegGetValue = advapi32.NewProc("RegGetValueW")
	procRegOpenKey  = advapi32.NewProc("RegOpenKeyExW")
	procRegEnumKey  = advapi32.NewProc("RegEnumKeyExW")
	procRegCloseKey = advapi32.NewProc("RegCloseKey")
)

// regString reads a REG_SZ value; "" when it is missing.
func regString(root uintptr, path, name string) string {
	p, _ := syscall.UTF16PtrFromString(path)
	n, _ := syscall.UTF16PtrFromString(name)
	var size uint32
	if r, _, _ := procRegGetValue.Call(root, uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(n)), rrfRTRegSZ, 0, 0, uintptr(unsafe.Pointer(&size))); r != 0 || size == 0 {
		return ""
	}
	buf := make([]uint16, size/2+1)
	if r, _, _ := procRegGetValue.Call(root, uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(n)), rrfRTRegSZ, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size))); r != 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}

// regSubkeys lists the names of a key's subkeys; none when it is missing.
func regSubkeys(root uintptr, path string) []string {
	p, _ := syscall.UTF16PtrFromString(path)
	var h syscall.Handle
	if r, _, _ := procRegOpenKey.Call(root, uintptr(unsafe.Pointer(p)), 0, keyRead, uintptr(unsafe.Pointer(&h))); r != 0 {
		return nil
	}
	defer procRegCloseKey.Call(uintptr(h))
	var out []string
	buf := make([]uint16, 512)
	for i := uint32(0); ; i++ {
		n := uint32(len(buf))
		if r, _, _ := procRegEnumKey.Call(uintptr(h), uintptr(i), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)), 0, 0, 0, 0); r != 0 {
			return out // ERROR_NO_MORE_ITEMS, or an error: what was read
		}
		out = append(out, syscall.UTF16ToString(buf[:n]))
	}
}
