//go:build windows
// +build windows

package traffic

import (
	"fmt"
	"hash/fnv"
	"strconv"
	"sync"
)

// Squawks hands out discrete SSR codes for departures. No source for
// per-airport code banks was found, so this is a documented simple scheme:
// codes from a bank (default 4001–4777, octal), the first one tried picked
// from the call sign (the same flight gets the same code), then the next
// free one; codes in use, and the special codes, are never given (0000,
// 1200 and 7000 VFR conspicuity, 2000 no code assigned, 7500 unlawful
// interference, 7600 radio failure, 7700 emergency).
type Squawks struct {
	// First and Last bound the bank, as four-digit octal codes (0: 4001
	// and 4777).
	First, Last string

	mu    sync.Mutex
	inUse map[string]string // code → call sign
}

// SpecialSquawk reports a code that is never assigned as a discrete code.
func SpecialSquawk(code string) bool {
	switch code {
	case "0000", "1200", "2000", "7000", "7500", "7600", "7700":
		return true
	}
	return false
}

// Assign gives cs a discrete code (its own again if it has one), or an
// error when the bank is full.
func (s *Squawks) Assign(cs string) (string, error) {
	first, last, err := s.bank()
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inUse == nil {
		s.inUse = map[string]string{}
	}
	for code, who := range s.inUse {
		if who == cs {
			return code, nil
		}
	}
	n := last - first + 1
	h := fnv.New32a()
	h.Write([]byte(cs))
	start := int(h.Sum32() % uint32(n))
	for i := range n {
		code := fmt.Sprintf("%04o", first+(start+i)%n)
		if SpecialSquawk(code) || s.inUse[code] != "" {
			continue
		}
		s.inUse[code] = cs
		return code, nil
	}
	return "", fmt.Errorf("traffic: no free squawk in %04o–%04o", first, last)
}

// Reserve marks code as in use by cs (a code given elsewhere, or seen on
// another aircraft), so it is not given again.
func (s *Squawks) Reserve(code, cs string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inUse == nil {
		s.inUse = map[string]string{}
	}
	s.inUse[code] = cs
}

// Release frees cs's code.
func (s *Squawks) Release(cs string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for code, who := range s.inUse {
		if who == cs {
			delete(s.inUse, code)
		}
	}
}

// bank is the bank as numbers.
func (s *Squawks) bank() (int, int, error) {
	f, l := s.First, s.Last
	if f == "" {
		f = "4001"
	}
	if l == "" {
		l = "4777"
	}
	first, err1 := strconv.ParseInt(f, 8, 32)
	last, err2 := strconv.ParseInt(l, 8, 32)
	if err1 != nil || err2 != nil || len(f) != 4 || len(l) != 4 || first > last {
		return 0, 0, fmt.Errorf("traffic: squawk bank %q–%q: four octal digits, first not after last", f, l)
	}
	return int(first), int(last), nil
}
