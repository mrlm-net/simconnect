package traffic

import "testing"

// What was learnt in one run is handed to the next: learnt struts kept, a
// shorter reach taken, nonsense left out.
func TestStrutsLearntCarriedOver(t *testing.T) {
	a := NewInjector(nil)
	a.struts[850], a.reach[850] = 0.57, 0.45
	l, gen := a.StrutsLearnt()
	if l.Struts[850] != 0.57 || l.Reach[850] != 0.45 {
		t.Fatalf("learnt: %+v", l)
	}
	b := NewInjector(nil)
	b.struts[1246], b.reach[850] = 0.70, 0.40
	l.Struts[1246], l.Struts[999], l.Reach[-1] = 0.80, 9.0, 0.1
	b.SetStrutsLearnt(l)
	if b.struts[850] != 0.57 || b.struts[1246] != 0.70 || b.reach[850] != 0.40 {
		t.Errorf("carried over: struts %v reach %v", b.struts, b.reach)
	}
	if _, ok := b.struts[999]; ok {
		t.Errorf("an extension of 9 ft taken")
	}
	if _, ok := b.reach[-1]; ok {
		t.Errorf("a reach without a model taken")
	}
	if _, g := a.StrutsLearnt(); g != gen {
		t.Errorf("counter moved without learning: %d → %d", gen, g)
	}
}
