package world

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
	"github.com/mrlm-net/simconnect/pkg/traffic"
)

// BenchmarkAssignStandArrival is an arrival's stand pick at its spawn
// (docs/traffic-smoothness.md), on the connection's goroutine, its
// taxi-ins worked out beforehand (StandAllocator.WarmTaxiIn).
func BenchmarkAssignStandArrival(b *testing.B) {
	raw, err := os.ReadFile("../../airport/testdata/LKPR.json")
	if err != nil {
		b.Fatal(err)
	}
	var r airport.RawAirport
	if err := json.Unmarshal(raw, &r); err != nil {
		b.Fatal(err)
	}
	l, err := airport.BuildLayout(r)
	if err != nil {
		b.Fatal(err)
	}
	g, err := airport.BuildGraph(l)
	if err != nil {
		b.Fatal(err)
	}
	alloc := traffic.NewStandAllocator(nil, g)
	alloc.WarmTaxiIn([]string{"24"}) // as the World does in the background when the airport loads
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		alloc.ReleaseOwner("CSA1")
		req := traffic.StandRequirements{Owner: "CSA1", HalfSpan: 17.9, Runway: "24", OffBlock: time.Now().Add(time.Hour)}
		if _, err := assignStand(alloc, req, standAirline); err != nil {
			b.Fatal(err)
		}
	}
}
