package world

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// defCountClient counts the fields added per definition; it answers nothing.
type defCountClient struct{ fields map[string]int }

func (c *defCountClient) AddToFacilityDefinition(def uint32, field string) error {
	c.fields[field+string(rune('0'+def))]++
	return nil
}

func (c *defCountClient) RequestFacilityDataEX1(uint32, uint32, string, string, byte) error {
	return nil
}

// TestAirwayKeeperDefinesOnce: a second airport's crawl doesn't register the
// definitions again. The simulator refuses a field added twice (live, 34
// exception 20s for each airport after the first).
func TestAirwayKeeperDefinesOnce(t *testing.T) {
	b, err := os.ReadFile("../../airport/testdata/LKPR-procedures.json")
	if err != nil {
		t.Fatal(err)
	}
	var p airport.Procedures
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	job, ok := airwayJobOf(p)
	if !ok {
		t.Fatal("no job")
	}
	job.seeds = job.seeds[:2]
	c := &defCountClient{fields: map[string]int{}}
	k := newAirwayKeeper(c, &recFeed{}, t.TempDir(), 24*time.Hour, 1, 100, t.Logf)
	k.queue = []airwayJob{job, job}
	now := time.Now()
	for i := 0; i < 1000 && (k.cur != nil || len(k.queue) > 0); i++ {
		k.tick(now)
		now = now.Add(time.Minute)
	}
	if k.cur != nil || len(k.queue) > 0 {
		t.Fatal("the crawls never ended")
	}
	for f, n := range c.fields {
		// TYPE is in the waypoint definition twice: of the fix and of its route.
		if n > 2 {
			t.Errorf("field %q added %d times", f, n)
		}
	}
	if len(c.fields) == 0 {
		t.Fatal("no definitions")
	}
}
