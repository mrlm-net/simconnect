package addons

import (
	"bufio"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Model doors (#572, #1026): where an installed aircraft model's main door
// is, read from its own files: flight_model.cfg [INTERACTIVE POINTS] (the
// frontmost main door on the left, where stairs go; interactive_point.0,
// the one jetways dock to, is the second door on widebodies) and
// [AIRPLANE_GEOMETRY] wing_span, with the type from aircraft.cfg [GENERAL]
// icao_type_designator. A livery folder without a flight_model.cfg has no
// door of its own (its base model has).

// InteractivePoint is one interactive_point line of a flight_model.cfg:
// Raw the values as written (a generated model copies it whole), Long, Lat
// and Vert the second to fourth values (feet from the model's reference
// datum: along, across (left negative), up), Kind the fifth (0 a main door,
// 1 cargo).
type InteractivePoint struct {
	Raw             string
	Long, Lat, Vert float64
	Kind            int
}

// ModelDoor is a model's main door: its folder, ICAO type, Door the
// frontmost main door on the left (stairs), First interactive_point.0 (the
// jetway's door), and the wing span (feet, 0 not given).
type ModelDoor struct {
	Folder     string
	Type       string
	Door       InteractivePoint
	First      InteractivePoint
	WingSpanFt float64
}

// ParseInteractivePoint reads the values of an interactive_point line
// ("0.4, 37.654, -6.137, 0.203, 0, …", a trailing ; comment left out);
// ok is false with fewer than four numbers.
func ParseInteractivePoint(v string) (InteractivePoint, bool) {
	if i := strings.IndexByte(v, ';'); i >= 0 {
		v = v[:i]
	}
	v = strings.TrimSpace(v)
	f := strings.Split(v, ",")
	if len(f) < 4 {
		return InteractivePoint{}, false
	}
	var n [4]float64
	for i := range n {
		x, err := strconv.ParseFloat(strings.TrimSpace(f[i]), 64)
		if err != nil {
			return InteractivePoint{}, false
		}
		n[i] = x
	}
	p := InteractivePoint{Raw: v, Long: n[1], Lat: n[2], Vert: n[3]}
	if len(f) > 4 {
		if k, err := strconv.ParseFloat(strings.TrimSpace(f[4]), 64); err == nil {
			p.Kind = int(k)
		}
	}
	return p, true
}

// ReadModelDoor reads the door of the model in folder dir (aircraft.cfg and
// flight_model.cfg there); ok is false without a door or a type.
func ReadModelDoor(dir string) (ModelDoor, bool) {
	d := ModelDoor{Folder: filepath.Base(dir)}
	gen := cfgSection(filepath.Join(dir, "aircraft.cfg"), "GENERAL")
	d.Type = strings.Trim(gen["icao_type_designator"], `"`)
	fm := filepath.Join(dir, "flight_model.cfg")
	points := cfgSection(fm, "INTERACTIVE POINTS")
	first, ok := ParseInteractivePoint(points["interactive_point.0"])
	if !ok || d.Type == "" {
		return ModelDoor{}, false
	}
	d.First, d.Door = first, first
	best := math.Inf(-1)
	for k, v := range points {
		if !strings.HasPrefix(k, "interactive_point.") {
			continue
		}
		if p, ok := ParseInteractivePoint(v); ok && p.Kind == 0 && p.Lat < 0 && p.Long > best {
			d.Door, best = p, p.Long
		}
	}
	if s, err := strconv.ParseFloat(firstField(cfgSection(fm, "AIRPLANE_GEOMETRY")["wing_span"]), 64); err == nil {
		d.WingSpanFt = s
	}
	return d, true
}

// DoorsByType reads the doors of the models in the SimObjects/Airplanes
// folders given, one per ICAO type: the first folder in name order with a
// door (a base model, "FSLTL_A320", before its liveries).
func DoorsByType(airplanes ...string) map[string]ModelDoor {
	out := map[string]ModelDoor{}
	for _, root := range airplanes {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			if e.IsDir() {
				names = append(names, e.Name())
			}
		}
		sort.Strings(names)
		for _, n := range names {
			d, ok := ReadModelDoor(filepath.Join(root, n))
			if !ok {
				continue
			}
			if _, have := out[d.Type]; !have {
				out[d.Type] = d
			}
		}
	}
	return out
}

// DoorsByTitle reads the doors of every model in the SimObjects/Airplanes
// folders given, by its titles (lower case; [FLTSIM.N] title in its
// aircraft.cfg): a livery folder has the door of its base model
// ([VARIATION] base_container). Our traffic spawns titles, so each one it
// can spawn from these folders has its own model's door, whatever its
// flight's type (a B38M flown as an installed B738).
func DoorsByTitle(airplanes ...string) map[string]ModelDoor {
	out := map[string]ModelDoor{}
	base := map[string]*ModelDoor{} // folder path → its door (nil: none)
	doorOf := func(dir string) *ModelDoor {
		dir = filepath.Clean(dir)
		if d, ok := base[dir]; ok {
			return d
		}
		var p *ModelDoor
		if d, ok := ReadModelDoor(dir); ok {
			p = &d
		}
		base[dir] = p
		return p
	}
	for _, root := range airplanes {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			dir := filepath.Join(root, e.Name())
			cfg := filepath.Join(dir, "aircraft.cfg")
			d := doorOf(dir)
			if d == nil {
				if b := strings.Trim(cfgSection(cfg, "VARIATION")["base_container"], `"`); b != "" {
					d = doorOf(filepath.Join(dir, filepath.FromSlash(strings.ReplaceAll(b, `\`, "/"))))
				}
			}
			if d == nil {
				continue
			}
			for _, t := range fltsimTitles(cfg) {
				out[strings.ToLower(t)] = *d
			}
		}
	}
	return out
}

type fltsim struct {
	title      string
	selectable bool
}

// fltsimTitles are the titles of an aircraft.cfg's [FLTSIM.N] sections.
func fltsimTitles(file string) []string {
	var out []string
	for _, f := range fltsims(file) {
		out = append(out, f.title)
	}
	return out
}

// fltsims are the [FLTSIM.N] sections of a .cfg: title and
// isUserSelectable.
func fltsims(file string) []fltsim {
	f, err := os.Open(file)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []fltsim
	in := false
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") {
			in = strings.HasPrefix(strings.ToUpper(line), "[FLTSIM.")
			if in {
				out = append(out, fltsim{})
			}
			continue
		}
		if !in {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if i := strings.IndexByte(v, ';'); i >= 0 {
			v = v[:i]
		}
		v = strings.Trim(strings.TrimSpace(v), `"`)
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "title":
			out[len(out)-1].title = v
		case "isuserselectable":
			out[len(out)-1].selectable = v == "1" || strings.EqualFold(v, "true")
		}
	}
	if sc.Err() != nil {
		return nil
	}
	kept := out[:0]
	for _, f := range out {
		if f.title != "" {
			kept = append(kept, f)
		}
	}
	return kept
}

// cfgSection is the keys of [name] in a .cfg file (lower case keys, values
// trimmed of a trailing ; comment); nil when the file cannot be read.
func cfgSection(file, name string) map[string]string {
	f, err := os.Open(file)
	if err != nil {
		return nil
	}
	defer f.Close()
	out := map[string]string{}
	in := false
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") {
			in = strings.EqualFold(strings.Trim(line, "[] \t"), name)
			continue
		}
		if !in || line == "" || line[0] == ';' {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if i := strings.IndexByte(v, ';'); i >= 0 && !strings.HasPrefix(strings.ToLower(strings.TrimSpace(k)), "interactive_point") {
			v = v[:i]
		}
		out[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
	}
	if sc.Err() != nil {
		return nil
	}
	return out
}

func firstField(v string) string {
	return strings.TrimSpace(strings.Split(v, ",")[0])
}
