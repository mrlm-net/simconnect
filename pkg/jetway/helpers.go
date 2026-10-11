// Package jetway docks the simulator's jetways to our AI traffic (#572).
//
// MSFS 2024 docks a jetway only to an aircraft in its ATC parking table,
// and our traffic is NonATC; no SimConnect call, event or variable docks
// one to it. What does: an invisible ParkedATC helper aircraft created
// under our parked aircraft, with the door of its type, on the gate. This
// file writes the helper models; the backend that places them is
// helpers' JetwayService.
package jetway

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// HelperTitlePrefix starts every helper's title; traffic never uses these
// (traffic.modelRules "MyCrew *") and lists leave them out (IsHelper).
const HelperTitlePrefix = "MyCrew Jetway Helper "

// HelperTitle is the title of type's helper; codes "" the one without
// airline codes, else the one with them ("ALL").
func HelperTitle(icaoType string, withCodes bool) string {
	t := HelperTitlePrefix + strings.ToUpper(icaoType)
	if withCodes {
		t += " ALL"
	}
	return t
}

// IsHelper reports whether a simulator title is one of the helpers.
func IsHelper(title string) bool { return strings.HasPrefix(title, HelperTitlePrefix) }

// HelperModel is a type to make a helper of: Dir the installed model's
// folder (its flight_model.cfg, engines.cfg, systems.cfg and ai.cfg are
// copied: the door, interactive_point.0, is the one the jetway docks to).
type HelperModel struct {
	Type string
	Dir  string
}

// PackageOptions name the package written.
type PackageOptions struct {
	// Folder is the package's folder name, "mycrew-jetway-helper" if empty.
	Folder string
	// Version is its package_version, "0.1.0" if empty.
	Version string
	// Creator is its manifest creator.
	Creator string
	// Codes are airline codes for each type's second helper ("… ALL"); none:
	// only the helper without codes.
	Codes []string
	// WingSpanFt, when not 0, replaces every helper's wing_span (feet): the
	// sim puts no ParkedATC next to an occupied gate; a small span may let
	// the placeholders take every gate (#572, to be measured).
	WingSpanFt float64
}

// copied are the base model's files a helper takes over.
var copied = []string{"flight_model.cfg", "engines.cfg", "systems.cfg", "ai.cfg"}

// WritePackage writes the helper package into community (the simulator's
// Community folder, or a staging folder): one helper SimObject per model,
// invisible (a model with no LODs), parked at gates only, with the
// manifest and layout the simulator needs. It replaces a package of the
// same folder. The simulator reads it at its next start.
func WritePackage(community string, models []HelperModel, o PackageOptions) (string, error) {
	if o.Folder == "" {
		o.Folder = "mycrew-jetway-helper"
	}
	if o.Version == "" {
		o.Version = "0.1.0"
	}
	root := filepath.Join(community, o.Folder)
	if err := os.RemoveAll(root); err != nil {
		return "", err
	}
	for _, m := range models {
		if err := writeHelper(root, m, o.Codes, o.WingSpanFt); err != nil {
			return "", fmt.Errorf("jetway helper %s: %w", m.Type, err)
		}
	}
	if err := writeLayout(root, o); err != nil {
		return "", err
	}
	return root, nil
}

func writeHelper(root string, m HelperModel, codes []string, spanFt float64) error {
	t := strings.ToUpper(m.Type)
	dir := filepath.Join(root, "SimObjects", "Airplanes", "MyCrew_JetwayHelper_"+t)
	if err := os.MkdirAll(filepath.Join(dir, "model"), 0o755); err != nil {
		return err
	}
	for _, f := range copied {
		if err := copyFile(filepath.Join(m.Dir, f), filepath.Join(dir, f)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	fm := filepath.Join(dir, "flight_model.cfg")
	if _, err := os.Stat(fm); err != nil {
		return fmt.Errorf("no flight_model.cfg in %s", m.Dir)
	}
	if spanFt > 0 {
		if err := setWingSpan(fm, spanFt); err != nil {
			return err
		}
	}
	cfg, err := helperAircraftCfg(filepath.Join(m.Dir, "aircraft.cfg"), t, codes)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "aircraft.cfg"), []byte(cfg), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "model", "model.cfg"), []byte("[models]\nnormal=helper.xml\n"), 0o644); err != nil {
		return err
	}
	xml := "<?xml version=\"1.0\" encoding=\"utf-8\" ?>\n<ModelInfo version=\"1.1\" guid=\"" + guidFor(t) + "\">\n\t<LODS>\n\t</LODS>\n</ModelInfo>\n"
	return os.WriteFile(filepath.Join(dir, "model", "helper.xml"), []byte(xml), 0o644)
}

// helperAircraftCfg is the base model's aircraft.cfg without its [FLTSIM.N]
// sections, then the helper's: without codes, and with codes when given.
// atc_parking_types GATE is required (without it the sim chose ramps).
func helperAircraftCfg(base, icaoType string, codes []string) (string, error) {
	f, err := os.Open(base)
	if err != nil {
		return "", err
	}
	defer f.Close()
	var b strings.Builder
	skip := false
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if s := strings.TrimSpace(line); strings.HasPrefix(s, "[") {
			skip = strings.HasPrefix(strings.ToUpper(s), "[FLTSIM.")
		}
		if !skip {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	fltsim := func(n int, title, codes string) {
		fmt.Fprintf(&b, "\n[FLTSIM.%d]\ntitle = %q\nmodel = \"\"\ntexture = \"\"\natc_airline = \"\"\nui_manufacturer = \"MyCrew\"\nui_type = \"Jetway Helper %s\"\nui_variation = \"Invisible\"\nui_typerole = \"Commercial Airliner\"\natc_parking_codes = %q\natc_parking_types = \"GATE\"\natc_heavy = 0\nisairtraffic = 0\nisuserselectable = 0\natc_id_enable = 0\nicao_airline = \"\"\npanel = \"\"\nsoundai = \"\"\n", n, title, icaoType, codes)
	}
	fltsim(0, HelperTitle(icaoType, false), "")
	if len(codes) > 0 {
		fltsim(1, HelperTitle(icaoType, true), strings.Join(codes, ","))
	}
	return b.String(), nil
}

// setWingSpan sets the wing_span line of a flight_model.cfg to ft.
func setWingSpan(path string, ft float64) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(b), "\n")
	for i, l := range lines {
		if k, _, ok := strings.Cut(l, "="); ok && strings.EqualFold(strings.TrimSpace(k), "wing_span") {
			lines[i] = fmt.Sprintf("wing_span = %g ; MyCrew jetway helper (#572)", ft)
			if strings.HasSuffix(l, "\r") {
				lines[i] += "\r"
			}
		}
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

// guidFor is a stable GUID for a type's helper model.
func guidFor(t string) string {
	var h [16]byte
	for i, c := range []byte("mycrew-jetway-" + t) {
		h[i%16] = h[i%16]*31 + c
	}
	return fmt.Sprintf("{%08x-%04x-%04x-%04x-%012x}", uint32(h[0])<<24|uint32(h[1])<<16|uint32(h[2])<<8|uint32(h[3]),
		uint16(h[4])<<8|uint16(h[5]), uint16(h[6])<<8|uint16(h[7]), uint16(h[8])<<8|uint16(h[9]),
		uint64(h[10])<<40|uint64(h[11])<<32|uint64(h[12])<<24|uint64(h[13])<<16|uint64(h[14])<<8|uint64(h[15]))
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(to)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// writeLayout writes layout.json (every file with its size and time) and
// manifest.json.
func writeLayout(root string, o PackageOptions) error {
	type entry struct {
		Path string `json:"path"`
		Size int64  `json:"size"`
		Date int64  `json:"date"`
	}
	var content []entry
	var total int64
	err := filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if rel == "layout.json" || rel == "manifest.json" {
			return nil
		}
		content = append(content, entry{Path: rel, Size: fi.Size(), Date: fileTime(fi.ModTime())})
		total += fi.Size()
		return nil
	})
	if err != nil {
		return err
	}
	sort.Slice(content, func(i, j int) bool { return content[i].Path < content[j].Path })
	lb, err := json.MarshalIndent(map[string]any{"content": content}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "layout.json"), lb, 0o644); err != nil {
		return err
	}
	manifest := map[string]any{
		"dependencies": []any{}, "content_type": "AIRCRAFT", "title": "MyCrew jetway helpers",
		"manufacturer": "MyCrew", "creator": o.Creator, "package_version": o.Version,
		"minimum_game_version": "1.8.14", "export_type": "Community", "builder": "MyCrew",
		"package_order_hint": "AIRCRAFT", "release_notes": map[string]any{"neutral": map[string]string{"LastUpdate": "", "OlderHistory": ""}},
		"total_package_size": fmt.Sprintf("%d", total),
	}
	mb, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "manifest.json"), mb, 0o644)
}

// fileTime is t as a Windows FILETIME (100 ns since 1601), as layout.json
// has it.
func fileTime(t time.Time) int64 {
	return t.UnixNano()/100 + 116444736000000000
}
