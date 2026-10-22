package world

import "testing"

func TestParseComposition(t *testing.T) {
	c, ok := parseComposition("revealRise_dist1.6_t50")
	if !ok || c.move != "revealRise" || c.back != 1.6 || c.at != 0.5 {
		t.Fatalf("got %+v %v", c, ok)
	}
	if _, ok := parseComposition("revealRise"); ok {
		t.Fatal("a bare move parsed")
	}
}

// TestCompositionGlides: every glide joins two picked frames of one move,
// and every picked frame is a known move.
func TestCompositionGlides(t *testing.T) {
	picked := map[string]bool{}
	for _, s := range sceneCompositions {
		c, ok := parseComposition(s)
		if !ok {
			t.Fatalf("%s does not parse", s)
		}
		if _, known := sceneMoves[c.move]; !known && c.move != "crane" {
			t.Errorf("%s: no move %q", s, c.move)
		}
		picked[s] = true
	}
	if len(compositionGlides) < 50 {
		t.Errorf("%d glides", len(compositionGlides))
	}
	dollies := 0
	for _, g := range compositionGlides {
		a, _ := parseComposition(g[0])
		if a.move == "chaseRise" || a.move == "crane" {
			t.Errorf("glide %v from behind", g)
		}
		if g[1] == dollyTo {
			dollies++
			if a.move == "topDown" {
				t.Errorf("dolly %v not from the side", g)
			}
			continue
		}
		b, _ := parseComposition(g[1])
		if !picked[g[0]] || !picked[g[1]] || a.move != b.move || g[0] == g[1] {
			t.Errorf("glide %v", g)
		}
	}
	if dollies < len(compositionGlides)/3 {
		t.Errorf("%d dollies of %d glides", dollies, len(compositionGlides))
	}
}
