package camera

import (
	"errors"
	"sync"
	"time"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/types"
)

// API is what the Director needs of a SimConnect client (*engine.Engine).
type API interface {
	CameraAcquire(clientID string) error
	CameraRelease(cameraDef string) error
	CameraSet(data types.SIMCONNECT_DATA_CAMERA, mask types.SIMCONNECT_CAMERA_DATA_MASK) error
}

// Locker is a client that can keep the world loaded around a point while
// the camera is away from the user aircraft (*engine.Engine).
type Locker interface {
	RequestCameraWorldLocker(position types.SIMCONNECT_DATA_XYZ, referential types.SIMCONNECT_POSITION_REFERENTIAL, objectID uint32) error
	DeleteCameraWorldLocker() error
}

// Director plays shots on the add-on camera: it acquires the camera, sets
// the current shot's pose at every Tick and moves on to the next shot when
// one ends. With nothing to play it holds the last pose. Safe for
// concurrent use; Tick and Handle belong to the connection's goroutine.
type Director struct {
	api      API
	clientID string

	mu       sync.Mutex
	queue    []Shot
	cur      Shot
	started  time.Time
	acquired bool
	asked    bool
	last     Pose
	havePose bool
	// OnShot hears each shot as it starts.
	onShot func(Shot)
	// lock: the point the world stays loaded around while the camera is
	// ours (LockWorld); locked once set.
	lock   *Point
	locked bool
	// settle: a cut was made; settleUntil the new shot holds its first
	// pose until then.
	settle      bool
	settleUntil time.Time
}

// LockWorld keeps the terrain, scenery and objects around p loaded while
// the camera is ours, so cuts across the airport do not show it loading
// (when the client is a Locker). Released with the camera.
func (d *Director) LockWorld(p Point) {
	d.mu.Lock()
	d.lock, d.locked = &p, false
	d.mu.Unlock()
}

// NewDirector drives the camera through api for clientID.
func NewDirector(api API, clientID string) *Director {
	return &Director{api: api, clientID: clientID}
}

// OnShot is called with each shot as it starts (outside the lock).
func (d *Director) OnShot(f func(Shot)) {
	d.mu.Lock()
	d.onShot = f
	d.mu.Unlock()
}

// Play replaces what is queued with shots, starting at the next Tick: a cut.
func (d *Director) Play(shots ...Shot) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.queue, d.cur = append([]Shot(nil), shots...), nil
	d.settle = true
}

// CutSettle: after a cut the new shot holds its first pose this long before
// it moves. The simulator's picture takes about a second to settle on a new
// view (exposure, temporal smoothing, models loading); the move starts once
// it has, not through it.
const CutSettle = 600 * time.Millisecond

// Then queues shots after those queued.
func (d *Director) Then(shots ...Shot) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.queue = append(d.queue, shots...)
}

// Current is the shot playing and how far into it, or nil.
func (d *Director) Current(now time.Time) (Shot, time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.cur == nil {
		return nil, 0
	}
	return d.cur, now.Sub(d.started)
}

// Remaining is how many shots wait after the current one.
func (d *Director) Remaining() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.queue)
}

// ErrNotAcquired: the camera is not ours (yet), or the user disabled it.
var ErrNotAcquired = errors.New("camera: not acquired")

// Tick sets the camera for now: acquiring it first (once there is a shot
// to play), then the pose of the shot playing. Call it from the connection's goroutine at the frame rate
// wanted (30–60 Hz).
func (d *Director) Tick(now time.Time) error {
	d.mu.Lock()
	if !d.acquired && d.cur == nil && len(d.queue) == 0 {
		d.mu.Unlock()
		return nil // nothing to show: the camera stays the user's
	}
	if !d.acquired {
		ask := !d.asked
		d.asked = true
		d.mu.Unlock()
		if ask {
			return d.api.CameraAcquire(d.clientID)
		}
		return ErrNotAcquired
	}
	if l, ok := d.api.(Locker); ok && d.lock != nil && !d.locked {
		d.locked = true
		r, id, xyz := d.lock.referential()
		if err := l.RequestCameraWorldLocker(xyz, r, uint32(id)); err != nil {
			d.mu.Unlock()
			return err
		}
	}
	var started Shot
	for d.cur == nil || now.Sub(d.started) >= d.cur.Length() {
		if len(d.queue) == 0 {
			if d.cur != nil {
				d.last, d.havePose = d.cur.PoseAt(d.cur.Length()), true
				d.cur = nil
			}
			break
		}
		d.cur, d.queue, d.started = d.queue[0], d.queue[1:], now
		d.settle, d.settleUntil = true, time.Time{} // every new shot is a cut
		started = d.cur
	}
	if d.cur != nil && d.settle {
		if d.settleUntil.IsZero() {
			d.settleUntil = now.Add(CutSettle)
		}
		if now.Before(d.settleUntil) {
			d.started = now // time stands still at the shot's start
		} else {
			d.settle, d.settleUntil = false, time.Time{}
		}
	}
	pose, ok := d.last, d.havePose
	if d.cur != nil {
		pose, ok = d.cur.PoseAt(now.Sub(d.started)), true
		d.last, d.havePose = pose, true
	}
	cb := d.onShot
	d.mu.Unlock()
	if started != nil && cb != nil {
		cb(started)
	}
	if !ok {
		return nil
	}
	data, mask := pose.Data()
	return d.api.CameraSet(data, mask)
}

// Handle takes the camera status messages: acquired, or lost (another
// add-on, the user).
func (d *Director) Handle(msg engine.Message) bool {
	if msg.SIMCONNECT_RECV == nil || types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_CAMERA_STATUS {
		return false
	}
	s := msg.AsCameraStatus()
	d.mu.Lock()
	d.acquired = s.AcquiredState == types.SIMCONNECT_CAMERA_ACQUIRED
	if !d.acquired && s.AcquiredState != types.SIMCONNECT_CAMERA_NOT_ACQUIRED {
		d.asked = true // someone else's, or disabled: do not ask again
	}
	d.mu.Unlock()
	return true
}

// Acquired reports whether the camera is ours.
func (d *Director) Acquired() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.acquired
}

// Release gives the camera back to the simulator and forgets the shots.
func (d *Director) Release() error {
	d.mu.Lock()
	was, locked := d.acquired || d.asked, d.locked
	d.acquired, d.asked, d.queue, d.cur, d.havePose, d.locked = false, false, nil, nil, false, false
	d.mu.Unlock()
	if l, ok := d.api.(Locker); ok && locked {
		l.DeleteCameraWorldLocker()
	}
	if !was {
		return nil
	}
	return d.api.CameraRelease("")
}
