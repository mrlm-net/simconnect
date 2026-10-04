package airport

import (
	"encoding/json"
	"sync"
	"testing"
)

func TestCacheGraphBuiltOnce(t *testing.T) {
	c := NewCache()
	l := loadLKPR(t)
	c.Put(l)
	var wg sync.WaitGroup
	graphs := make([]*Graph, 32)
	for i := range graphs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			g, err := c.Graph("lkpr")
			if err != nil {
				t.Error(err)
			}
			graphs[i] = g
		}()
	}
	wg.Wait()
	for _, g := range graphs[1:] {
		if g != graphs[0] {
			t.Fatal("concurrent callers got different graphs")
		}
	}
	if got, ok := c.Layout("LKPR"); !ok || got != l {
		t.Error("Layout(LKPR) not cached")
	}
	if got := c.ICAOs(); len(got) != 1 || got[0] != "LKPR" {
		t.Errorf("ICAOs = %v", got)
	}
}

func TestCachePutReplacesAndInvalidate(t *testing.T) {
	c := NewCache()
	c.Put(loadLKPR(t))
	g1, _ := c.Graph("LKPR")
	c.Put(loadLKPR(t))
	g2, _ := c.Graph("LKPR")
	if g1 == g2 {
		t.Error("Put did not replace the graph")
	}
	c.Invalidate("lkpr")
	if _, ok := c.Layout("LKPR"); ok {
		t.Error("Invalidate kept the layout")
	}
	if _, err := c.Graph("LKPR"); err == nil {
		t.Error("Graph of an unloaded airport succeeded")
	}
}

func TestLoaderWithCache(t *testing.T) {
	c := NewCache()
	l := NewLoader(&fakeClient{}, LoaderWithCache(c))
	if err := l.Request("LKPR"); err != nil {
		t.Fatal(err)
	}
	res := feed(t, l, simulate(lkprRaw(t), DefaultLoaderRequestBase, false))
	if len(res) != 1 || res[0].Err != nil {
		t.Fatalf("results %+v", res)
	}
	if got, ok := c.Layout("LKPR"); !ok || got != res[0].Layout {
		t.Error("Loader did not store the layout in the cache")
	}
	if len(res[0].Raw.TaxiPaths) != 2350 {
		t.Error("Result.Raw not set")
	}
}

func TestGeoJSON(t *testing.T) {
	l := loadLKPR(t)
	b, err := l.GeoJSON()
	if err != nil {
		t.Fatal(err)
	}
	var fc struct {
		Type     string `json:"type"`
		Features []struct {
			Geometry struct {
				Type        string          `json:"type"`
				Coordinates json.RawMessage `json:"coordinates"`
			} `json:"geometry"`
			Properties map[string]any `json:"properties"`
		} `json:"features"`
	}
	if err := json.Unmarshal(b, &fc); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, f := range fc.Features {
		counts[f.Properties["kind"].(string)+"/"+f.Geometry.Type]++
	}
	want := map[string]int{"runway/Polygon": 2, "taxiPath/LineString": 2350, "parking/Point": 89, "taxiPoint/Point": 1967}
	if fc.Type != "FeatureCollection" || len(counts) != len(want) {
		t.Fatalf("type %q, counts %v", fc.Type, counts)
	}
	for k, n := range want {
		if counts[k] != n {
			t.Errorf("%s = %d, want %d", k, counts[k], n)
		}
	}
	// Coordinates are [lon, lat]: LKPR is at 14.26°E, 50.10°N.
	var pt [2]float64
	for _, f := range fc.Features {
		if f.Properties["kind"] == "parking" {
			if err := json.Unmarshal(f.Geometry.Coordinates, &pt); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if pt[0] < 14 || pt[0] > 15 || pt[1] < 50 || pt[1] > 51 {
		t.Errorf("parking coordinates %v are not [lon, lat] near LKPR", pt)
	}
}

func TestRouteFeature(t *testing.T) {
	g := lkprGraph(t)
	r, err := g.RouteToRunway(18, "24", RouteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	f := r.Feature()
	if f.Geometry.Type != "LineString" || len(f.Geometry.Coordinates.([][2]float64)) != len(r.Points) || f.Properties["runwayEnd"] != "24" {
		t.Errorf("route feature %+v", f.Properties)
	}
}
