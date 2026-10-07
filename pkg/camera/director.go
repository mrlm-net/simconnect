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
	// asked: acquiring was asked at askedAt (asked again after
	// AcquireRetry while not acquired); refused: the camera is someone
	// else's or disabled by the user, not asked again until Release.
	asked    bool
	askedAt  time.Time
	refused  bool
	last     Pose
	havePose bool
	// OnShot hears each shot as it starts.
	onShot func(Shot)
	// lock: the point the world stays loaded around while the camera is
	// ours (LockWorld); locked once set, tried again from lockRetry when
	// it failed.
	lock      *Point
	locked    bool
	lockRetry time.Time
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
	d.lock, d.locked, d.lockRetry = &p, false, time.Time{}
	d.mu.Unlock()
}

// AcquireRetry is how long the Director waits before asking for the
// camera again when it was not given (NOT_ACQUIRED, or the request
// failed), and before trying the world lock again when it failed.
const AcquireRetry = 5 * time.Second

// Reset forgets the camera's state (a new connection): it is acquired,
// and the world locked, again on the next Tick with something to play, on
// api (nil: the same). The shots queued and the last pose stay.
func (d *Director) Reset(api API) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if api != nil {
		d.api = api
	}
	d.acquired, d.asked, d.askedAt, d.refused = false, false, time.Time{}, false
	d.locked, d.lockRetry = false, time.Time{}
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
	api := d.api
	if !d.acquired {
		// Asked once, and again after AcquireRetry while the camera is
		// not given (#26); never while someone else has it.
		ask := !d.refused && (!d.asked || now.Sub(d.askedAt) >= AcquireRetry)
		if ask {
			d.asked, d.askedAt = true, now
		}
		d.mu.Unlock()
		if ask {
			return api.CameraAcquire(d.clientID)
		}
		return ErrNotAcquired
	}
	if l, ok := api.(Locker); ok && d.lock != nil && !d.locked && !now.Before(d.lockRetry) {
		r, id, xyz := d.lock.referential()
		if err := l.RequestCameraWorldLocker(xyz, r, uint32(id)); err != nil {
			d.lockRetry = now.Add(AcquireRetry) // tried again then (#26)
			d.mu.Unlock()
			return err
		}
		d.locked = true
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
	return api.CameraSet(data, mask)
}

// Handle takes the camera status messages: acquired, or lost (another
// add-on, the user).
func (d *Director) Handle(msg engine.Message) bool {
	if msg.SIMCONNECT_RECV == nil || types.SIMCONNECT_RECV_ID(msg.DwID) != types.SIMCONNECT_RECV_ID_CAMERA_STATUS {
		return false
	}
	s := msg.AsCameraStatus()
	d.mu.Lock()
	was := d.acquired
	d.acquired = s.AcquiredState == types.SIMCONNECT_CAMERA_ACQUIRED
	switch s.AcquiredState {
	case types.SIMCONNECT_CAMERA_ACQUIRED:
		d.refused = false
	case types.SIMCONNECT_CAMERA_NOT_ACQUIRED:
		// Asked again AcquireRetry after the ask, or after losing it.
		if was {
			d.asked, d.askedAt = true, time.Now()
		}
	default:
		d.asked, d.refused = true, true // someone else's, or disabled: do not ask again
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
	was, locked, api := d.acquired || d.asked, d.locked, d.api
	d.acquired, d.asked, d.queue, d.cur, d.havePose, d.locked = false, false, nil, nil, false, false
	d.askedAt, d.refused, d.lockRetry = time.Time{}, false, time.Time{}
	d.mu.Unlock()
	if l, ok := api.(Locker); ok && locked {
		l.DeleteCameraWorldLocker()
	}
	if !was {
		return nil
	}
	return api.CameraRelease("")
}
