package addons

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Modular aircraft (MSFS 2024): a SimObjects/Airplanes folder with
// presets/<owner>/<preset>/config, each preset's titles ([FLTSIM.N] of its
// aircraft.cfg) with the door of its exterior attachment (attached_objects.cfg
// attachment_root, attached to the exterior model): that folder's
// config/flight_model.cfg, the type from the [Inherit] base chain of its
// attachment.cfg. Live, the Fenix A320's L1 door is 29.0 ft ahead of its
// datum (Function_Exterior_A320_CFM_SL), the stock A320's 37.65 ft: without
// it the jetway met the helper's door 2.6 m ahead of ours (MyCrew OK-MYV at
// LKPR 23). A preset's liveries spawn as "<preset title> :: <livery name>".

// modularDoors adds the doors of the modular aircraft in the
// SimObjects/Airplanes folders given to out, by title (lower case); a title
// already there stays.
func modularDoors(airplanes []string, out map[string]ModelDoor) {
	for _, root := range airplanes {
		cfgs, _ := filepath.Glob(filepath.Join(root, "*", "presets", "*", "*", "config", "aircraft.cfg"))
		for _, cfg := range cfgs {
			att := exteriorAttachment(filepath.Join(filepath.Dir(cfg), "attached_objects.cfg"))
			if att == "" {
				continue
			}
			d, ok := attachmentDoor(airplanes, att)
			if !ok {
				continue
			}
			d.Folder = filepath.Base(filepath.Dir(filepath.Dir(cfg)))
			for _, t := range fltsimTitles(cfg) {
				if _, have := out[strings.ToLower(t)]; !have {
					out[strings.ToLower(t)] = d
				}
			}
		}
	}
}

// exteriorAttachment is the attachment_root of the [sim_attachment.N] of
// file attached to the exterior model; "" none.
func exteriorAttachment(file string) string {
	f, err := os.Open(file)
	if err != nil {
		return ""
	}
	defer f.Close()
	var root, model string
	found := ""
	done := func() {
		if found == "" && root != "" && strings.EqualFold(model, "exterior") {
			found = root
		}
		root, model = "", ""
	}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") {
			done()
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok || strings.HasPrefix(line, ";") {
			continue
		}
		if i := strings.IndexByte(v, ';'); i >= 0 {
			v = v[:i]
		}
		v = strings.Trim(strings.TrimSpace(v), `"`)
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "attachment_root":
			root = v
		case "attach_to_model":
			model = v
		}
	}
	done()
	return found
}

// attachmentDoor reads the door of the attachment at virtual path p
// ("SimObjects/Airplanes/FNX_32X/attachments/fnx/…", in any of the packages
// of airplanes): the first folder of its [Inherit] chain with a door, the
// first with a type.
func attachmentDoor(airplanes []string, p string) (ModelDoor, bool) {
	var d ModelDoor
	have := false
	typ := ""
	for range 8 {
		dir := resolveVirtual(airplanes, p)
		if dir == "" {
			break
		}
		cfg := filepath.Join(dir, "config")
		if typ == "" {
			typ = strings.Trim(cfgSection(filepath.Join(cfg, "aircraft.cfg"), "GENERAL")["icao_type_designator"], `"`)
		}
		if !have {
			d, have = readModelDoor(cfg, "?")
		}
		if have && typ != "" {
			break
		}
		p = strings.Trim(cfgSection(filepath.Join(dir, "attachment.cfg"), "Inherit")["base"], `"`)
		if p == "" {
			break
		}
	}
	if !have || typ == "" {
		return ModelDoor{}, false
	}
	d.Type = typ
	return d, true
}

// resolveVirtual is virtual path p (from a package's root) in the first
// package of airplanes (their SimObjects/Airplanes folders) that has it;
// "" none.
func resolveVirtual(airplanes []string, p string) string {
	p = filepath.FromSlash(strings.ReplaceAll(p, `\`, "/"))
	for _, root := range airplanes {
		dir := filepath.Join(filepath.Dir(filepath.Dir(root)), p)
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			return dir
		}
	}
	return ""
}
