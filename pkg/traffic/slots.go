package traffic

import (
	"sort"
	"time"
)

// SlotCapacity is what an airport's runway takes in a slot bin, as slot
// coordination declares it: the timetable is planned within it, so its
// flights can keep their times and a delay is news. Zero fields take the
// defaults.
type SlotCapacity struct {
	Bin        time.Duration // default 10 min
	Arrivals   int           // arrivals in a bin; default 3 (one in 3 min, DefaultLandingGap, with room for departures)
	Departures int           // departures in a bin; default 3
	Movements  int           // both in a bin; default 5
	// MaxShift is how far a flight is moved to a free slot (default 1 h);
	// one with no slot within it is left out.
	MaxShift time.Duration
}

func (c *SlotCapacity) defaults() {
	if c.Bin <= 0 {
		c.Bin = 10 * time.Minute
	}
	if c.Arrivals <= 0 {
		c.Arrivals = 3
	}
	if c.Departures <= 0 {
		c.Departures = 3
	}
	if c.Movements <= 0 {
		c.Movements = 5
	}
	if c.MaxShift <= 0 {
		c.MaxShift = time.Hour
	}
}

// CoordinateSlots plans flights within the runway capacity of airports, as
// slot coordination does: in order of their time there, each takes the
// first bin with room at or after its own, its STD and STA moved on
// together; one with no room within MaxShift is left out. VFR flights (the
// circuit), real ones (Observed) and replayed paths keep their times
// (live, LKPR: 8 arrivals due in 15 minutes, the last landing 20 minutes
// after its STA).
func CoordinateSlots(flights []Flight, airports []string, c SlotCapacity) []Flight {
	c.defaults()
	focus := map[string]bool{}
	for _, a := range airports {
		focus[a] = true
	}
	type count struct{ arr, dep int }
	used := map[string]count{} // by "ICAO bin"
	key := func(icao string, t time.Time) string { return icao + " " + t.Truncate(c.Bin).Format(time.RFC3339) }
	at := func(f Flight) time.Time {
		if focus[f.Origin] {
			return f.STD
		}
		return f.STA
	}
	out := append([]Flight(nil), flights...)
	sort.SliceStable(out, func(i, j int) bool { return at(out[i]).Before(at(out[j])) })
	kept := out[:0]
	for _, f := range out {
		dep, arr := focus[f.Origin], focus[f.Destination]
		if !dep && !arr || f.Rules == "VFR" || f.Observed != nil || len(f.Path) > 0 {
			kept = append(kept, f)
			continue
		}
		fits := func(shift time.Duration) bool {
			if dep {
				u := used[key(f.Origin, f.STD.Add(shift))]
				if u.dep >= c.Departures || u.arr+u.dep >= c.Movements {
					return false
				}
			}
			if arr {
				u := used[key(f.Destination, f.STA.Add(shift))]
				if u.arr >= c.Arrivals || u.arr+u.dep >= c.Movements {
					return false
				}
			}
			return true
		}
		shift, ok := time.Duration(0), false
		for ; shift <= c.MaxShift; shift += c.Bin {
			if ok = fits(shift); ok {
				break
			}
		}
		if !ok {
			continue
		}
		f.STD, f.STA = f.STD.Add(shift), f.STA.Add(shift)
		if dep {
			k := key(f.Origin, f.STD)
			u := used[k]
			u.dep++
			used[k] = u
		}
		if arr {
			k := key(f.Destination, f.STA)
			u := used[k]
			u.arr++
			used[k] = u
		}
		kept = append(kept, f)
	}
	return kept
}
