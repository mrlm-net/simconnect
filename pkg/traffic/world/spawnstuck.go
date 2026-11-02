package world

import (
	"time"

	"github.com/mrlm-net/simconnect/pkg/engine"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

const (
	// spawnAnswerLimit is how long an aircraft waits for the simulator to
	// create it: slow enough for a cold model, a lost creation never comes
	// (#1396).
	spawnAnswerLimit = 3 * time.Minute
	// lateAnswerHold keeps the IDs of an aircraft removed before the
	// answer: a late answer must not reach whoever gets them next.
	lateAnswerHold = 10 * time.Minute
)

// heldIDs is an ID block waiting out a late answer.
type heldIDs struct {
	defBase uint32
	since   time.Time
}

// spawnWatch gives up on aircraft the simulator has not created within the
// limit, which frees their stands, and frees the IDs that waited long
// enough; in the connection goroutine.
func (cc *controlCenter) spawnWatch() {
	cc.mu.Lock()
	items := make([]*controlled, 0, len(cc.items))
	for _, it := range cc.items {
		items = append(items, it)
	}
	keep := cc.heldIDs[:0]
	for _, h := range cc.heldIDs {
		if traffic.Since(h.since) >= lateAnswerHold {
			cc.ids.Release(h.defBase)
		} else {
			keep = append(keep, h)
		}
	}
	cc.heldIDs = keep
	cc.mu.Unlock()
	for _, it := range items {
		it.mu.Lock()
		stuck := it.view.State == traffic.TaxiSpawning.String() && traffic.Since(it.askedAt) >= spawnAnswerLimit
		it.mu.Unlock()
		if !stuck {
			continue
		}
		cc.log.printf("%-6s the simulator did not create it in %s: given up, stand released", it.Tail, spawnAnswerLimit)
		if err := cc.remove(it); err != nil {
			cc.log.printf("%-6s removing it: %v", it.Tail, err)
		}
	}
}

// lateCreation removes an aircraft created after it was given up or removed.
func (cc *controlCenter) lateCreation(msg engine.Message) bool {
	p, ok, err := cc.fleet.HandleAbandoned(msg)
	if !ok {
		return false
	}
	cc.log.printf("%-6s created after it was given up: removed (%v)", p.Tail, err)
	return true
}
