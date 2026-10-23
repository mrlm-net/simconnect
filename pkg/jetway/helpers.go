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

// HelperVariantTitle is the title of type's helper with its door moved k
// steps (PackageOptions.SideStepM) to the right, left when negative: at a
// gate where the simulator parks off the parking line, the variant whose
// door makes up for it lets our aircraft stand on the line with the
// jetway at its door. k 0 is the helper itself.
func HelperVariantTitle(icaoType string, k int) string {
	if k == 0 {
		return HelperTitle(icaoType, false)
	}
	return fmt.Sprintf("%s R%+d", HelperTitle(icaoType, false), k)
}

// IsHelperVariant reports whether title is a helper variant (not a type's
// own helper).
func IsHelperVariant(title string) bool {
	if !IsHelper(title) {
		return false
	}
	if strings.Contains(title, " D-") {
		return true // HelperDoorTitle
	}
	for _, s := range []string{" R", " A"} {
		i := strings.LastIndex(title, s)
		if i > 0 && len(title) > i+2 && (title[i+2] == '+' || title[i+2] == '-') {
			return true
		}
	}
	return false
}

// HelperAheadTitle is the title of type's helper with its door aheadM
// ahead (back when negative): at a gate where our aircraft stands off the
// simulator's place along the line (a stand rule, LKPR C22 9 m back), the
// variant parked at the simulator's place has its door where ours is, so
// the jetway meets our aircraft. aheadM 0 is the helper itself.
func HelperAheadTitle(icaoType string, aheadM float64) string {
	if aheadM == 0 {
		return HelperTitle(icaoType, false)
	}
	return fmt.Sprintf("%s A%+.1f", HelperTitle(icaoType, false), aheadM)
}

// HelperDoorTitle is the title of type's helper with doorType's door: as
// wide as type, so the simulator parks it only at gates that big, with the
// door of a smaller aircraft standing there, whose jetway meets that
// door, not type's higher one further back (live, LKPR B14: a B748 helper
// for a B738, the jetway docked above and beside its door).
func HelperDoorTitle(icaoType, doorType string) string {
	return HelperTitle(icaoType, false) + " D-" + strings.ToUpper(doorType)
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
	// SideSteps, with SideStepM, adds each type's variants (HelperVariantTitle)
	// with the door SideStepM × k to the right, k from -SideSteps to
	// SideSteps: none when 0.
	SideSteps int
	SideStepM float64
	// AheadM adds each type's variants (HelperAheadTitle) with the door
	// moved each of these meters ahead (back when negative): the offsets
	// of the stand rules along the line (traffic.StandRules ForwardM).
	AheadM []float64
	// DoorOf adds, for each type a key, its variants with the door of each
	// type listed (HelperDoorTitle), taken from that type's model (among
	// the models written).
	DoorOf map[string][]string
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
		t := strings.ToUpper(m.Type)
		if err := writeHelper(root, m, o.Codes, o.WingSpanFt, "", HelperTitle(t, false), 0, 0); err != nil {
			return "", fmt.Errorf("jetway helper %s: %w", m.Type, err)
		}
		for k := -o.SideSteps; k <= o.SideSteps && o.SideStepM > 0; k++ {
			if k == 0 {
				continue
			}
			if err := writeHelper(root, m, nil, o.WingSpanFt, fmt.Sprintf("R%+d", k), HelperVariantTitle(t, k), float64(k)*o.SideStepM, 0); err != nil {
				return "", fmt.Errorf("jetway helper %s %+d: %w", m.Type, k, err)
			}
		}
		for _, d := range o.DoorOf[t] {
			var from *HelperModel
			for i := range models {
				if strings.EqualFold(models[i].Type, d) {
					from = &models[i]
				}
			}
			if from == nil {
				continue // that type's model not among them
			}
			v := "D-" + strings.ToUpper(d)
			if err := writeHelper(root, m, nil, o.WingSpanFt, v, HelperDoorTitle(t, d), 0, 0); err != nil {
				return "", fmt.Errorf("jetway helper %s door of %s: %w", m.Type, d, err)
			}
			fm := filepath.Join(root, "SimObjects", "Airplanes", "MyCrew_JetwayHelper_"+t+"_"+v, "flight_model.cfg")
			if err := copyDoor(fm, filepath.Join(from.Dir, "flight_model.cfg")); err != nil {
				return "", fmt.Errorf("jetway helper %s door of %s: %w", m.Type, d, err)
			}
		}
		for _, a := range o.AheadM {
			if a == 0 {
				continue
			}
			if err := writeHelper(root, m, nil, o.WingSpanFt, fmt.Sprintf("A%+.1f", a), HelperAheadTitle(t, a), 0, a); err != nil {
				return "", fmt.Errorf("jetway helper %s ahead %+.1f: %w", m.Type, a, err)
			}
		}
	}
	if err := writeLayout(root, o); err != nil {
		return "", err
	}
	return root, nil
}

// writeHelper writes type m's helper titled title; a variant (its folder
// suffix, "" none) with its door rightM to the right and aheadM ahead,
// without codes.
func writeHelper(root string, m HelperModel, codes []string, spanFt float64, variant, title string, rightM, aheadM float64) error {
	t := strings.ToUpper(m.Type)
	folder := "MyCrew_JetwayHelper_" + t
	guidKey := t // the helper's own GUID as before; a variant's its folder's
	if variant != "" {
		folder += "_" + variant
		guidKey = folder
	}
	dir := filepath.Join(root, "SimObjects", "Airplanes", folder)
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
	if variant != "" {
		if err := shiftDoor(fm, rightM/0.3048, aheadM/0.3048); err != nil {
			return err
		}
	}
	cfg, err := helperAircraftCfg(filepath.Join(m.Dir, "aircraft.cfg"), t, codes, title)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "aircraft.cfg"), []byte(cfg), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "model", "model.cfg"), []byte("[models]\nnormal=helper.xml\n"), 0o644); err != nil {
		return err
	}
	xml := "<?xml version=\"1.0\" encoding=\"utf-8\" ?>\n<ModelInfo version=\"1.1\" guid=\"" + guidFor(guidKey) + "\">\n\t<LODS>\n\t</LODS>\n</ModelInfo>\n"
	return os.WriteFile(filepath.Join(dir, "model", "helper.xml"), []byte(xml), 0o644)
}

// helperAircraftCfg is the base model's aircraft.cfg without its [FLTSIM.N]
// sections, then the helper's: without codes, and with codes when given.
// atc_parking_types GATE is required (without it the sim chose ramps).
func helperAircraftCfg(base, icaoType string, codes []string, title string) (string, error) {
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
	fltsim(0, title, "")
	if len(codes) > 0 {
		fltsim(1, HelperTitle(icaoType, true), strings.Join(codes, ","))
	}
	return b.String(), nil
}

// shiftDoor moves interactive_point.0 (the door the jetway docks to) of a
// flight_model.cfg rightFt to the right (its third value, lateral, feet)
// and aheadFt ahead (its second, longitudinal).
func shiftDoor(path string, rightFt, aheadFt float64) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(b), "\n")
	done := false
	for i, l := range lines {
		k, v, ok := strings.Cut(l, "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(k), "interactive_point.0") {
			continue
		}
		cr := strings.HasSuffix(v, "\r")
		v, comment, _ := strings.Cut(strings.TrimSuffix(v, "\r"), ";")
		f := strings.Split(v, ",")
		if len(f) < 3 {
			return fmt.Errorf("interactive_point.0: %q", l)
		}
		var lon, lat float64
		if _, err := fmt.Sscan(strings.TrimSpace(f[1]), &lon); err != nil {
			return fmt.Errorf("interactive_point.0 longitudinal: %w", err)
		}
		if _, err := fmt.Sscan(strings.TrimSpace(f[2]), &lat); err != nil {
			return fmt.Errorf("interactive_point.0 lateral: %w", err)
		}
		if aheadFt != 0 {
			f[1] = fmt.Sprintf(" %.3f", lon+aheadFt)
		}
		if rightFt != 0 {
			f[2] = fmt.Sprintf(" %.3f", lat+rightFt)
		}
		lines[i] = strings.TrimRight(k, " ") + " = " + strings.TrimSpace(strings.Join(f, ","))
		if c := strings.TrimSpace(comment); c != "" {
			lines[i] += " ; " + c
		}
		lines[i] += fmt.Sprintf(" ; MyCrew: door %+.2f ft right, %+.2f ft ahead", rightFt, aheadFt)
		if cr {
			lines[i] += "\r"
		}
		done = true
	}
	if !done {
		return fmt.Errorf("no interactive_point.0 in %s", path)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

// copyDoor sets interactive_point.0 (the door the jetway docks to) of the
// flight_model.cfg at path to the one of the flight_model.cfg at from.
func copyDoor(path, from string) error {
	door := func(p string) (int, []string, error) {
		b, err := os.ReadFile(p)
		if err != nil {
			return 0, nil, err
		}
		lines := strings.Split(string(b), "\n")
		for i, l := range lines {
			if k, _, ok := strings.Cut(l, "="); ok && strings.EqualFold(strings.TrimSpace(k), "interactive_point.0") {
				return i, lines, nil
			}
		}
		return 0, nil, fmt.Errorf("no interactive_point.0 in %s", p)
	}
	j, src, err := door(from)
	if err != nil {
		return err
	}
	i, lines, err := door(path)
	if err != nil {
		return err
	}
	l := strings.TrimSuffix(src[j], "\r") + " ; MyCrew: the door of " + filepath.Base(filepath.Dir(from))
	if strings.HasSuffix(lines[i], "\r") {
		l += "\r"
	}
	lines[i] = l
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
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
