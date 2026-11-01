package world

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/traffic"
)

type absorbErrArr struct {
	arrivalCtl
	err error
}

func (f *absorbErrArr) AbsorbDelay(time.Duration) (traffic.Absorption, error) {
	return traffic.Absorption{}, f.err
}

// TestWireErrorKeepsIdentity: an arrival on the final answers
// ErrNotOnProcedure; through the link the sequencer has to recognise it
// (live over a director: "absorbing 2m11s failed: not flying the STAR",
// and closingUp never ran).
func TestWireErrorKeepsIdentity(t *testing.T) {
	dir, act := pipe()
	defer dir.Close()
	srv := newWireServer()
	arr := &absorbErrArr{err: traffic.ErrNotOnProcedure}
	srv.add("arr/1", arr)
	go serve(act, srv)
	a := &remoteArr{c: newWireClient(dir, nil), t: "arr/1"}

	if _, err := a.AbsorbDelay(time.Minute); !errors.Is(err, traffic.ErrNotOnProcedure) {
		t.Errorf("plain: %v is not ErrNotOnProcedure", err)
	}
	arr.err = fmt.Errorf("LNX04: %w", traffic.ErrHolding)
	_, err := a.AbsorbDelay(time.Minute)
	if !errors.Is(err, traffic.ErrHolding) {
		t.Errorf("wrapped: %v is not ErrHolding", err)
	}
	if err == nil || err.Error() != "LNX04: traffic: holding" {
		t.Errorf("text changed: %v", err)
	}
	if errors.Is(err, traffic.ErrNotOnProcedure) {
		t.Errorf("%v is ErrNotOnProcedure too", err)
	}
	arr.err = errors.New("something else")
	if _, err := a.AbsorbDelay(time.Minute); err == nil || err.Error() != "something else" {
		t.Errorf("unknown error: %v", err)
	}
}

// TestWireSentinelsAllFound: each known error survives its text.
func TestWireSentinelsAllFound(t *testing.T) {
	for _, s := range wireSentinels {
		if err := wireError(s.Error()); !errors.Is(err, s) {
			t.Errorf("%q does not come back as itself", s)
		}
	}
}

// TestWireSentinelsComplete: every Err var of the packages the link carries
// errors from is in the list, so a new one cannot be missed.
func TestWireSentinelsComplete(t *testing.T) {
	src, err := os.ReadFile("wire_errors.go")
	if err != nil {
		t.Fatal(err)
	}
	var missing []string
	for pkg, dir := range map[string]string{"traffic": "..", "airport": "../../airport", "nav": "../../nav"} {
		for _, name := range errVars(t, dir) {
			if !strings.Contains(string(src), pkg+"."+name+",") {
				missing = append(missing, pkg+"."+name)
			}
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("not in wireSentinels: %s", strings.Join(missing, " "))
	}
}

// errVars names the package-level Err… vars of the package in dir.
func errVars(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	fset := token.NewFileSet()
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, dir+"/"+n, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok || g.Tok != token.VAR {
				continue
			}
			for _, spec := range g.Specs {
				for _, id := range spec.(*ast.ValueSpec).Names {
					if strings.HasPrefix(id.Name, "Err") {
						names = append(names, id.Name)
					}
				}
			}
		}
	}
	return names
}
