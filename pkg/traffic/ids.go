package traffic

import (
	"errors"
	"sync"
)

// IDBlocks hands out blocks of SimConnect data definition and request IDs
// for controllers (TaxiWithIDs, ArrivalWithIDs) and takes them back when
// the aircraft is gone, so a long session reuses a fixed range instead of
// counting up without end (#370). A block reused is registered again by
// its next controller: controllers clear their definitions before adding
// to them (Fleet remembers which it defined).
type IDBlocks struct {
	defBase, reqBase, size uint32

	mu   sync.Mutex
	free []uint32 // block indexes, the lowest reused first
	used map[uint32]bool
	n    uint32
}

// ErrNoIDs is returned when every block is in use.
var ErrNoIDs = errors.New("traffic: no free ID block")

// NewIDBlocks creates count blocks of size IDs: definitions from defBase,
// requests from reqBase.
func NewIDBlocks(defBase, reqBase, size, count uint32) *IDBlocks {
	b := &IDBlocks{defBase: defBase, reqBase: reqBase, size: size, used: map[uint32]bool{}, n: count}
	for i := uint32(0); i < count; i++ {
		b.free = append(b.free, i)
	}
	return b
}

// Acquire takes a free block: its first definition and request IDs.
func (b *IDBlocks) Acquire() (defBase, reqBase uint32, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.free) == 0 {
		return 0, 0, ErrNoIDs
	}
	i := b.free[0]
	b.free = b.free[1:]
	b.used[i] = true
	return b.defBase + i*b.size, b.reqBase + i*b.size, nil
}

// Release returns the block starting at defBase; unknown blocks are
// ignored.
func (b *IDBlocks) Release(defBase uint32) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if defBase < b.defBase || (defBase-b.defBase)%b.size != 0 {
		return
	}
	i := (defBase - b.defBase) / b.size
	if !b.used[i] {
		return
	}
	delete(b.used, i)
	// Keep the free list sorted: the lowest block is reused first.
	at := len(b.free)
	for k, f := range b.free {
		if f > i {
			at = k
			break
		}
	}
	b.free = append(b.free[:at], append([]uint32{i}, b.free[at:]...)...)
}

// InUse counts the blocks handed out.
func (b *IDBlocks) InUse() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.used)
}
