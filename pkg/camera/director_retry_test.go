package camera

import (
	"errors"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/types"
)

type lockAPI struct {
	fakeAPI
	locks int
	fail  bool
}

func (l *lockAPI) RequestCameraWorldLocker(types.SIMCONNECT_DATA_XYZ, types.SIMCONNECT_POSITION_REFERENTIAL, uint32) error {
	l.locks++
	if l.fail {
		return errors.New("refused")
	}
	return nil
}
func (l *lockAPI) DeleteCameraWorldLocker() error { return nil }

// The camera not given is asked for again after AcquireRetry; one that is
// someone else's is not (#26).
func TestDirectorAcquireRetry(t *testing.T) {
	f := &fakeAPI{}
	d := NewDirector(f, "test")
	d.Play(Hold("a", Pose{Eye: On(1, 10, 0, 0)}, 2*time.Second))
	now := time.Now()
	d.Tick(now)
	d.Handle(status(types.SIMCONNECT_CAMERA_NOT_ACQUIRED))
	d.Tick(now.Add(time.Second))
	if f.acquired != 1 {
		t.Fatalf("asked again too soon: %d", f.acquired)
	}
	d.Tick(now.Add(AcquireRetry))
	if f.acquired != 2 {
		t.Fatalf("not asked again: %d", f.acquired)
	}
	d.Handle(status(types.SIMCONNECT_CAMERA_ACQUIRED_BY_OTHER))
	d.Tick(now.Add(3 * AcquireRetry))
	if f.acquired != 2 {
		t.Errorf("asked while someone else's: %d", f.acquired)
	}
	// A new connection: asked again at once.
	g := &fakeAPI{}
	d.Reset(g)
	d.Tick(now.Add(3 * AcquireRetry))
	if g.acquired != 1 || f.acquired != 2 {
		t.Errorf("after Reset asked %d (old %d)", g.acquired, f.acquired)
	}
}

// A world lock that failed is tried again after AcquireRetry (#26).
func TestDirectorLockRetry(t *testing.T) {
	l := &lockAPI{fail: true}
	d := NewDirector(l, "test")
	d.LockWorld(At(50, 14, 400))
	d.Play(Hold("a", Pose{Eye: On(1, 10, 0, 0)}, 20*time.Second))
	now := time.Now()
	d.Tick(now)
	d.Handle(status(types.SIMCONNECT_CAMERA_ACQUIRED))
	if err := d.Tick(now); err == nil || l.locks != 1 {
		t.Fatalf("lock: %v %d", err, l.locks)
	}
	d.Tick(now.Add(time.Second))
	if l.locks != 1 {
		t.Fatalf("tried again too soon: %d", l.locks)
	}
	l.fail = false
	if err := d.Tick(now.Add(AcquireRetry)); err != nil || l.locks != 2 {
		t.Fatalf("retry: %v %d", err, l.locks)
	}
	d.Tick(now.Add(2 * AcquireRetry))
	if l.locks != 2 {
		t.Errorf("locked again: %d", l.locks)
	}
}

// Keys are sorted by At (E11).
func TestPathSortsKeys(t *testing.T) {
	p := Path("p", 4*time.Second, Linear,
		Key{1, Pose{Eye: On(1, 10, 10, 0)}}, Key{0, Pose{Eye: On(1, 0, 0, 0)}}, Key{0.5, Pose{Eye: On(1, 10, 0, 0)}})
	if e := p.PoseAt(0).Eye.Offset; e.Right != 0 || e.Up != 0 {
		t.Errorf("start %+v", e)
	}
	if e := p.PoseAt(4 * time.Second).Eye.Offset; e.Up != 10 {
		t.Errorf("end %+v", e)
	}
}
