//go:build windows

package checklist

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"

	"github.com/mrlm-net/simconnect/pkg/systems"
)

// Importing an aircraft's own MSFS checklist (the readable community ones,
// FlyByWire's: SimObjects/<aircraft>/Checklist/*.xml): each Page becomes a
// List, each Checkpoint an Item with its subject and expectation, due at
// its Step (mapped to our stages and phases, the step's own id beside
// them). A checkpoint's test becomes a Check where it is one simple test
// of a SimVar pkg/systems names generically; the rest is crew-confirmed.
// The checkpoints' copilot actions (raw key events) are not imported:
// imported items are verify only.

// msfsSteps maps an MSFS checklist step to our stages and pilot phases.
var msfsSteps = map[string][]string{
	"PREFLIGHT_GATE":     {"before-start"},
	"PREFLIGHT_PUSHBACK": {"after-start"},
	"PREFLIGHT_TAXI_OUT": {"before-takeoff"},
	"FLIGHT_RUNWAY":      {"before-takeoff"},
	"FLIGHT_TAKEOFF":     {"takeoff", "climb"},
	"FLIGHT_CLIMB":       {"climb"},
	"FLIGHT_CRUISE":      {"cruise"},
	"FLIGHT_DESCENT":     {"descent"},
	"LANDING_APPROACH":   {"approach"},
	"LANDING_TOUCHDOWN":  {"landing"},
	"LANDING_GROUNDROLL": {"after-landing"},
	"LANDING_TAXI_IN":    {"after-landing"},
	"LANDING_GATE":       {"parking"},
}

// msfsVars maps a SimVar (without its index) to the pkg/systems value a
// check names.
var msfsVars = map[string]string{
	"GEAR HANDLE POSITION":         systems.GearDown,
	"FLAPS HANDLE INDEX":           systems.FlapsIndex,
	"LIGHT BEACON":                 systems.LightBeacon,
	"LIGHT STROBE":                 systems.LightStrobe,
	"LIGHT LANDING":                systems.LightLanding,
	"LIGHT NAV":                    systems.LightNav,
	"LIGHT TAXI":                   systems.LightTaxi,
	"BRAKE PARKING POSITION":       systems.ParkingBrake,
	"BRAKE PARKING INDICATOR":      systems.ParkingBrake,
	"SPOILERS ARMED":               systems.SpoilersArmed,
	"CABIN SEATBELTS ALERT SWITCH": systems.Seatbelts,
	"AUTOPILOT MASTER":             systems.APMaster,
	"TRANSPONDER STATE":            systems.XPDRState,
}

type msfsDoc struct {
	Steps       []msfsStep    `xml:"Checklist.Checklist>Step"`
	Checkpoints []msfsCheckpt `xml:"Checklist.CheckpointLibrary>Checkpoint"`
}

type msfsStep struct {
	ID    string     `xml:"ChecklistStepId,attr"`
	Pages []msfsPage `xml:"Page"`
}

type msfsPage struct {
	Subject string      `xml:"SubjectTT,attr"`
	Entries []msfsEntry `xml:",any"`
}

// msfsEntry is a Checkpoint (by reference, maybe with its own words) or a
// Block of them.
type msfsEntry struct {
	XMLName  xml.Name
	Ref      string      `xml:"ReferenceId,attr"`
	Subject  string      `xml:"SubjectTT,attr"`
	Desc     *msfsDesc   `xml:"CheckpointDesc"`
	Children []msfsEntry `xml:"Checkpoint"`
}

type msfsDesc struct {
	Subject     string `xml:"SubjectTT,attr"`
	Expectation string `xml:"ExpectationTT,attr"`
}

type msfsCheckpt struct {
	ID    string     `xml:"Id,attr"`
	Desc  msfsDesc   `xml:"CheckpointDesc"`
	Tests []msfsTest `xml:"Test"`
	Seq   []msfsTest `xml:"Sequence>Test"`
}

type msfsTest struct {
	Op  *msfsOp  `xml:"TestValue>Operator"`
	Val *msfsVal `xml:"TestValue>Val"`
}

type msfsOp struct {
	Type string    `xml:"OpType,attr"`
	Vals []msfsVal `xml:"Val"`
}

type msfsVal struct {
	SimVar string `xml:"SimVarName,attr"`
	Value  string `xml:"Value,attr"`
}

// ImportMSFS reads an MSFS checklist (checklist) with its checkpoint
// libraries into a set named name: the libraries define the checkpoints
// the checklist refers to (a reference no library defines keeps the
// words the checklist gives it, if any).
func ImportMSFS(name string, checklist io.Reader, libraries ...io.Reader) (Set, error) {
	defs := map[string]msfsCheckpt{}
	for _, r := range libraries {
		var doc msfsDoc
		if err := decodeMSFS(r, &doc); err != nil {
			return Set{}, err
		}
		for _, c := range doc.Checkpoints {
			defs[c.ID] = c
		}
	}
	var doc msfsDoc
	if err := decodeMSFS(checklist, &doc); err != nil {
		return Set{}, err
	}
	set := Set{Name: name, Note: "imported from the aircraft's MSFS checklist; verify only"}
	seen := map[string]int{}
	for _, st := range doc.Steps {
		stages := append([]string{strings.ToLower(st.ID)}, msfsSteps[st.ID]...)
		for _, pg := range st.Pages {
			title := spoken(pg.Subject)
			l := List{Name: slug(title), Title: title, Stages: stages}
			if n := seen[l.Name]; n > 0 {
				l.Name = fmt.Sprintf("%s-%d", l.Name, n+1)
			}
			seen[slug(title)]++
			l.Items = importEntries(pg.Entries, defs)
			if len(l.Items) > 0 {
				set.Lists = append(set.Lists, l)
			}
		}
	}
	return set, set.Validate()
}

func importEntries(es []msfsEntry, defs map[string]msfsCheckpt) []Item {
	var out []Item
	for _, e := range es {
		switch e.XMLName.Local {
		case "Block":
			out = append(out, importEntries(e.Children, defs)...)
		case "Checkpoint":
			def, ok := defs[e.Ref]
			desc := def.Desc
			if e.Desc != nil {
				desc = *e.Desc
			}
			if !ok && e.Desc == nil {
				continue // nothing known of it
			}
			it := Item{Challenge: spoken(desc.Subject), Response: spoken(desc.Expectation), Note: e.Ref}
			if it.Challenge == "" {
				continue
			}
			if len(def.Tests) == 1 && len(def.Seq) == 0 {
				it.Check = msfsCheck(def.Tests[0])
			}
			out = append(out, it)
		}
	}
	return out
}

// msfsCheck is a test as a Check: a SimVar on (Val), off (NOT), or equal
// to a value (EQUAL); nil when it is anything else or its SimVar has no
// generic name.
func msfsCheck(t msfsTest) *Check {
	generic := func(v msfsVal) string {
		name, _, _ := strings.Cut(strings.ToUpper(strings.TrimSpace(v.SimVar)), ":")
		return msfsVars[name]
	}
	on, off := true, false
	switch {
	case t.Val != nil && t.Op == nil:
		if g := generic(*t.Val); g != "" {
			return &Check{Value: g, Is: &on}
		}
	case t.Op != nil && strings.EqualFold(t.Op.Type, "NOT") && len(t.Op.Vals) == 1:
		if g := generic(t.Op.Vals[0]); g != "" {
			return &Check{Value: g, Is: &off}
		}
	case t.Op != nil && strings.EqualFold(t.Op.Type, "EQUAL") && len(t.Op.Vals) == 2:
		g := generic(t.Op.Vals[0])
		v, err := strconv.ParseFloat(t.Op.Vals[1].Value, 64)
		if g != "" && err == nil {
			return &Check{Value: g, Min: &v, Max: &v}
		}
	}
	return nil
}

// spoken is a checklist text as said: "TT:" dropped, a localisation key
// ("GAME.CHECKLIST_BATTERY_SWITCHES") as its words, upper case.
func spoken(s string) string {
	s = strings.TrimSpace(strings.TrimPrefix(s, "TT:"))
	if rest, ok := strings.CutPrefix(s, "GAME."); ok {
		rest = strings.TrimPrefix(rest, "CHECKLIST_")
		s = strings.ReplaceAll(rest, "_", " ")
	}
	return strings.ToUpper(s)
}

// slug is a title as a list name: "COCKPIT PREPARATION" → "cockpit-preparation".
func slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

// decodeMSFS decodes an MSFS XML document, Windows-1252 or UTF-8.
func decodeMSFS(r io.Reader, v any) error {
	d := xml.NewDecoder(r)
	d.CharsetReader = func(charset string, in io.Reader) (io.Reader, error) {
		switch strings.ToLower(charset) {
		case "windows-1252", "cp1252", "iso-8859-1", "latin1":
			raw, err := io.ReadAll(in)
			if err != nil {
				return nil, err
			}
			return bytes.NewReader(fromCP1252(raw)), nil
		}
		return nil, fmt.Errorf("checklist: charset %s", charset)
	}
	d.Strict = false
	if err := d.Decode(v); err != nil {
		return fmt.Errorf("checklist: reading MSFS XML: %w", err)
	}
	return nil
}

// cp1252 are Windows-1252's characters 0x80…0x9F (0 undefined).
var cp1252 = [32]rune{0x20AC, 0, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021, 0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0, 0x017D, 0,
	0, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014, 0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0, 0x017E, 0x0178}

func fromCP1252(b []byte) []byte {
	var out bytes.Buffer
	for _, c := range b {
		switch {
		case c < 0x80:
			out.WriteByte(c)
		case c < 0xA0 && cp1252[c-0x80] != 0:
			out.WriteRune(cp1252[c-0x80])
		default:
			out.WriteRune(rune(c))
		}
	}
	return out.Bytes()
}
