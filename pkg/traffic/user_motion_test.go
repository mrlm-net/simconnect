package traffic

import (
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/airport"
)

// TestReportUserMotion: the user aircraft taxiing has its way ahead in the
// ground picture (ours and vehicles give way to it), pushing its corridor
// behind (a push, given way to), stopped nothing.
func TestReportUserMotion(t *testing.T) {
	g := NewGroundPicture()
	pos := airport.LatLon{Lat: 50.1, Lon: 14.26}
	g.Report(9, pos, 90, DefaultMotionProfile(), time.Now())
	g.ReportUserMotion(9, pos, 90, 15, 18, 16, false)
	e := g.aircraft[9]
	if len(e.ahead) < 2 || e.pushing || alongHeading(pos, 90, e.ahead[len(e.ahead)-1]) < UserLookMeters {
		t.Fatalf("taxiing: %d points ahead, pushing %v", len(e.ahead), e.pushing)
	}
	g.ReportUserMotion(9, pos, 90, 2, 18, 16, true)
	e = g.aircraft[9]
	if !e.pushing || len(e.ahead) < 2 || alongHeading(pos, 90, e.ahead[len(e.ahead)-1]) > -UserPushMeters {
		t.Fatalf("pushing: %d points, pushing %v", len(e.ahead), e.pushing)
	}
	g.ReportUserMotion(9, pos, 90, 0, 18, 16, false)
	if e = g.aircraft[9]; len(e.ahead) != 0 || e.pushing {
		t.Fatalf("stopped: %d points, pushing %v", len(e.ahead), e.pushing)
	}
}
