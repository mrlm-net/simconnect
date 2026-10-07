package dict

import (
	"strings"
	"testing"
)

type testItem struct {
	ID    string            `json:"id"`
	Name  string            `json:"name"`
	Speed float64           `json:"speed"`
	Units map[string]string `json:"units,omitempty"`
}

var shippedUnits = map[string]string{"121.9": "Ground"}

func testTable(t *testing.T) func() []testItem {
	t.Helper()
	var now []testItem
	Register(Keyed("test.items", "id", "", "", func() []testItem {
		return []testItem{{ID: "A", Name: "Alpha", Speed: 250, Units: shippedUnits}, {ID: "B", Name: "Bravo", Speed: 300}}
	}, func(i testItem) string { return strings.ToUpper(i.ID) }, func(items []testItem) { now = items }).ForSet("test-set", nil))
	t.Cleanup(func() {
		mu.Lock()
		delete(tables, "test.items")
		mu.Unlock()
	})
	return func() []testItem { return now }
}

// An item given for a shipped id replaces only the values it gives: local
// wins per value (#27); the shipped item is not written to.
func TestUsePerValue(t *testing.T) {
	now := testTable(t)
	if err := Use("test.items", []byte(`[{"id":"a","speed":180,"units":{"118.1":"Tower"}},{"id":"C","name":"Charlie"}]`)); err != nil {
		t.Fatal(err)
	}
	items := now()
	if len(items) != 3 || items[0].Name != "Alpha" || items[0].Speed != 180 || items[1].Name != "Bravo" || items[2].Name != "Charlie" {
		t.Fatalf("items %+v", items)
	}
	if items[0].Units["121.9"] != "Ground" || items[0].Units["118.1"] != "Tower" || len(shippedUnits) != 1 {
		t.Errorf("units %v, shipped %v", items[0].Units, shippedUnits)
	}
	if err := Use("test.items", []byte(`[{"name":"no id"}]`)); err == nil {
		t.Error("an item without an id accepted")
	}
}

// The API's set: a null payload leaves the shipped item, an item without a
// key is an error, never zero-value items (#28).
func TestUseSetNullPayload(t *testing.T) {
	now := testTable(t)
	if _, err := UseSet("test-set", []byte(`{"items":[{"key":"A","payload":null},{"key":"B","payload":{"speed":320}}]}`)); err != nil {
		t.Fatal(err)
	}
	items := now()
	if len(items) != 2 || items[0].Speed != 250 || items[1].Speed != 320 || items[1].Name != "Bravo" {
		t.Errorf("items %+v", items)
	}
	if _, err := UseSet("test-set", []byte(`{"items":[{"key":"","payload":{"speed":1}}]}`)); err == nil {
		t.Error("an API item without a key accepted")
	}
}
