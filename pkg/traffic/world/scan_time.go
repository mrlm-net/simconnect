package world

import "time"

// scanStale: a scan begun longer ago than this is not the one handed on
// (a director's feed, or none yet): the picture takes it as of now.
const scanStale = 2 * time.Second

// scanTime is the time of the positions of the scan handed on now: when
// it began (the simulator's frame that answered it), not when its last
// entry and its names came in, a frame or more later (a slow frame under
// -race: an overflight's virtual track placed from a pose 200 ms older
// than its time, 50 m behind its aircraft).
func (cc *controlCenter) scanTime() time.Time {
	now := cc.clock.Now()
	ns := cc.core.scanFrom.Load()
	if ns == 0 {
		return now
	}
	at := time.Unix(0, ns)
	if at.After(now) || now.Sub(at) > scanStale {
		return now
	}
	return at
}
