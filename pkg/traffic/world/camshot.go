package world

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mrlm-net/simconnect/pkg/camera"
)

// ShotRequest plays one scene move on one aircraft, to tune it: Back pulls
// the camera away from what it looks at (1 as written, 2 twice as far), Fov
// scales the field of view (1 as written).
type ShotRequest struct {
	ICAO string `json:"icao"`
	Tail string `json:"tail"`
	Move string `json:"move"`
	From string `json:"from"` // in place of Move: a glide From → To (compositions.go)
	To   string `json:"to"`
	// Glide n of the picked ones (from 1; GET /api/camera/glides).
	Glide int     `json:"glide"`
	Side  float64 `json:"side"`
	Sec   float64 `json:"sec"`
	Back  float64 `json:"back"`
	Fov   float64 `json:"fov"`
	// At holds the move still at that point of it (0 its start, 1 its
	// end): a composition to look at.
	At *float64 `json:"at"`
}

// sceneMoveNames are the moves a scene shot (or a ShotRequest) takes.
func sceneMoveNames() []string {
	out := []string{"crane", "runwaySide", "underApproach"}
	for k := range sceneMoves {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// playShot plays one move (mode shot: nothing else picks a shot after it,
// the camera stays where the move ended).
func (m *cameraMan) playShot(req ShotRequest) (string, error) {
	if m.dir == nil {
		return "", errors.New("no camera on this connection")
	}
	m.mu.Lock()
	scene := m.mode == "scene"
	m.mu.Unlock()
	if scene {
		return "", errors.New("a scene is playing")
	}
	it := m.cc.byTail(strings.ToUpper(req.Tail))
	if it == nil {
		return "", fmt.Errorf("no aircraft %q", req.Tail)
	}
	g, err := m.cc.graph(strings.ToUpper(req.ICAO))
	if err != nil {
		return "", err
	}
	if req.Glide > 0 && req.Glide <= len(compositionGlides) {
		req.From, req.To = compositionGlides[req.Glide-1][0], compositionGlides[req.Glide-1][1]
	}
	if req.From != "" && req.Sec == 0 {
		req.Sec = camera.MaxShot.Seconds() // the longest: slow
	}
	shots := castShots(g, map[string]*controlled{"it": it}, []SceneShot{{Move: req.Move, From: req.From, To: req.To, Who: "it", Side: req.Side, Sec: req.Sec}}, "shot")
	if len(shots) == 0 {
		return "", fmt.Errorf("no shot %q on %s (moves: %s)", req.Move, it.Tail, strings.Join(sceneMoveNames(), ", "))
	}
	if err := m.setMode("shot", 0); err != nil {
		return "", err
	}
	shot := shots[0]
	if req.Back > 0 && req.Back != 1 || req.Fov > 0 && req.Fov != 1 {
		shot = tuned{shot, req.Back, req.Fov}
	}
	if req.At != nil {
		at := time.Duration(max(0, min(1, *req.At)) * float64(shot.Length()))
		shot = camera.Hold(shot.Name(), shot.PoseAt(at), 4*time.Second)
	}
	m.mu.Lock()
	m.subject, m.shot = it.Tail, shot.Name()
	m.mu.Unlock()
	m.dir.Play(shot)
	// The shot over, the camera goes back to the simulator (unless another
	// shot came since).
	n := shotPlays.Add(1)
	time.AfterFunc(shot.Length()+time.Second, func() {
		m.mu.Lock()
		still := m.mode == "shot"
		m.mu.Unlock()
		if still && shotPlays.Load() == n {
			m.setMode("off", 0)
		}
	})
	return fmt.Sprintf("%s, %.0f s", shot.Name(), shot.Length().Seconds()), nil
}

// shotPlays counts the shots played, so only the last one gives the camera
// back.
var shotPlays atomic.Int64

// tuned is a shot pulled back (eye farther from the target, on the same
// aircraft) and its field of view scaled.
type tuned struct {
	camera.Shot
	back, fov float64
}

func (s tuned) PoseAt(t time.Duration) camera.Pose {
	p := s.Shot.PoseAt(t)
	if s.back > 0 && p.Eye.Frame == camera.Object && p.Target.Frame == camera.Object && p.Eye.Object == p.Target.Object {
		e, o := p.Eye.Offset, p.Target.Offset
		p.Eye.Offset = camera.Offset{
			Right:   o.Right + (e.Right-o.Right)*s.back,
			Up:      o.Up + (e.Up-o.Up)*s.back,
			Forward: o.Forward + (e.Forward-o.Forward)*s.back,
		}
	}
	if s.fov > 0 {
		f := p.FovDeg
		if f <= 0 {
			f = camera.DefaultFovDeg
		}
		p.FovDeg = f * s.fov
	}
	return p
}
