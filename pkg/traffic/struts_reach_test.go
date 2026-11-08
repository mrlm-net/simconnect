package traffic

import (
	"math"
	"testing"
)

// A model raised ahead of rotation and shown back at its static height
// (live OKBZI CJ3: 5.08 ↔ 4.84 ft) learns how far its struts reach, and the
// next raise stays under it; the same sample on a landing rollout (not
// raised ahead of rotation) learns nothing.
func TestStrutsReachLearnt(t *testing.T) {
	const cg, static = 9.93, 0.5
	inj := NewInjector(&eventClient{})
	o := &injected{cgFt: cg, sentGround: 1, cgShown: cg + 0.30}
	g := &injectGround{CGFt: cg, StaticPitch: static, PlanePitch: static, OnGround: 1, GS: 130}
	inj.noteStruts(o, g, cg+0.01)
	if _, ok := inj.reachOf(o); ok {
		t.Fatal("reach learnt off a take-off raise")
	}
	o.strutsUp = true
	inj.noteStruts(o, g, cg+0.01)
	r, ok := inj.reachOf(o)
	if !ok || math.Abs(r-(0.30-strutsReachBackFt)) > 1e-9 {
		t.Fatalf("reach %.3f (%v), want %.3f", r, ok, 0.30-strutsReachBackFt)
	}
	inj.struts[strutsKey(cg)] = 0.45
	if h := inj.strutsHeld(o); h > r+1e-9 {
		t.Errorf("held %.2f above its reach %.2f", h, r)
	}
}
