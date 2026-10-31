package world

import (
	"strings"
	"testing"
)

// TestSceneValidate: what a host's scene gets wrong is said before it plays
// (#1324), and every built-in scene passes.
func TestSceneValidate(t *testing.T) {
	cast := []SceneCast{{Role: "dep", Kind: "departure", Tail: "A1"}}
	shot := func(s SceneShot) []SceneBeat { return []SceneBeat{{Shots: []SceneShot{s}}} }
	for _, tc := range []struct {
		name string
		sc   Scene
		want string
	}{
		{"role twice", Scene{Cast: []SceneCast{cast[0], cast[0]}}, "twice"},
		{"no role", Scene{Cast: []SceneCast{{Kind: "departure", Tail: "A"}}}, "without a role"},
		{"kind", Scene{Cast: []SceneCast{{Role: "x", Kind: "vfr", Tail: "A"}}}, "kind"},
		{"no tail", Scene{Cast: []SceneCast{{Role: "x", Kind: "arrival"}}}, "no tail"},
		{"after", Scene{Cast: []SceneCast{{Role: "x", Kind: "arrival", Tail: "A", After: "y:pushback"}}}, "cue"},
		{"listen", Scene{Cast: cast, Listen: "nobody"}, "listen"},
		{"cue role", Scene{Cast: cast, Beats: []SceneBeat{{Cue: "arr:airborne"}}}, "cue"},
		{"cue no event", Scene{Cast: cast, Beats: []SceneBeat{{Cue: "dep"}}}, "cue"},
		{"cue time", Scene{Cast: cast, Beats: []SceneBeat{{Cue: "t:soon"}}}, "seconds"},
		{"local", Scene{Cast: cast, Local: "evening"}, "HH:MM"},
		{"line nobody", Scene{Cast: cast, Lines: []SceneLine{{Text: "x", Frequency: "118.10"}}}, "nobody"},
		{"line no text", Scene{Cast: cast, Lines: []SceneLine{{Who: "dep"}}}, "no text"},
		{"line station", Scene{Cast: cast, Lines: []SceneLine{{Who: "Prague Tower", Text: "x"}}}, "without a frequency"},
		{"line frequency", Scene{Cast: cast, Lines: []SceneLine{{Who: "dep", Text: "x", Frequency: "soon"}}}, "frequency"},
		{"lines overlap", Scene{Cast: cast, Lines: []SceneLine{{AtSec: 1, Who: "dep", Text: "one two three four five six seven eight", Frequency: "118.10"}, {AtSec: 2, Who: "Prague Tower", Text: "x", Frequency: "118.10"}}}, "lasts until"},
		{"shot who", Scene{Cast: cast, Beats: shot(SceneShot{Move: "flyover", Who: "arr"})}, "not in the cast"},
		{"shot move", Scene{Cast: cast, Beats: shot(SceneShot{Move: "barrelRoll", Who: "dep"})}, "no move"},
	} {
		if err := tc.sc.validate(); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want %q", tc.name, err, tc.want)
		}
	}
	good := Scene{Cast: cast, Listen: "132.89", Beats: []SceneBeat{
		{Cue: "start", Shots: []SceneShot{{Move: "crane", Who: "dep"}, {From: "any", Who: "dep"}}},
		{Cue: "t:5", Listen: "dep"},
		{Cue: "dep:height:500"},
	}}
	if err := good.validate(); err != nil {
		t.Error(err)
	}
	all, err := loadScenes()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) == 0 {
		t.Fatal("no built-in scenes")
	}
	for k, sc := range all {
		if err := sc.validate(); err != nil {
			t.Errorf("%s: %v", k, err)
		}
	}
}
