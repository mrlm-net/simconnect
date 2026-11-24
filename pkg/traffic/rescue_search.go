package traffic

import "github.com/mrlm-net/simconnect/pkg/airport"

// Rescue vehicles' route searches (#1670): the way from the depot to
// standby, in to the aircraft and home are graph searches, at a large
// airport several milliseconds each and a crew's more than a frame. With
// Search set they run off the goroutine that drives the vehicles, on what
// rescueAsk copied, and the first frame after one ends puts what it found
// in place, if the vehicle is still where the search started from.

// rescueFound is a search: what it found to put in place (with the
// vehicle's lock held; nil: nothing), to read once done is closed.
type rescueFound struct {
	done  chan struct{}
	apply func() error
}

// search runs find, which reads only what it was given (no lock taken),
// and puts in place what it returns: at once without Search, else in the
// first frame after it ends (settle). One started over another drops what
// that one finds. r.mu held.
func (r *SimObjectRescue) search(find func() func() error) error {
	if r.Search == nil {
		r.found = nil
		if apply := find(); apply != nil {
			return apply()
		}
		return nil
	}
	f := &rescueFound{done: make(chan struct{})}
	r.found = f
	r.Search(func() {
		defer close(f.done)
		f.apply = find()
	})
	return nil
}

// settle puts in place what the search that has ended found; with one
// still running nothing changes. r.mu held.
func (r *SimObjectRescue) settle() error {
	f := r.found
	if f == nil {
		return nil
	}
	select {
	case <-f.done:
	default:
		return nil
	}
	r.found = nil
	if f.apply == nil || r.done {
		return nil
	}
	return f.apply()
}

// rescueAsk is what a vehicle's searches read: copied with its lock held,
// for a search without it.
type rescueAsk struct {
	r        *SimObjectRescue // to tell it from the others of its convoy
	layout   *airport.Layout
	depot    airport.LatLon
	convoy   *rescueConvoy
	body     *GroundPose
	bodyProf MotionProfile
}

// ask is the copy for a search. r.mu held.
func (r *SimObjectRescue) ask() rescueAsk {
	a := rescueAsk{r: r, layout: r.Layout, depot: r.depot, convoy: r.convoy, bodyProf: r.bodyProf}
	if r.body != nil {
		b := *r.body
		a.body = &b
	}
	return a
}

// samePlace reports whether a vehicle at p is where it was at q.
func samePlace(p, q GroundPose) bool {
	return p.Position == q.Position && p.Heading == q.Heading
}
