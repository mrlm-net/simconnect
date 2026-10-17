package world

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

// A schedule setting taken is kept and set again through the same handler
// on the next connection; one refused is not kept.
func TestScheduleKeptForTheNextConnection(t *testing.T) {
	st := &state{}
	mux := http.NewServeMux()
	registerSchedule(mux, st)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/schedule", bytes.NewReader([]byte(`{"enabled":true}`))))
	if rec.Code != http.StatusServiceUnavailable || len(st.scheduleSet) != 0 {
		t.Fatalf("not connected: %d, kept %d, want 503 and none kept", rec.Code, len(st.scheduleSet))
	}
	st.scheduleSet = [][]byte{[]byte(`{"enabled":true}`)}
	var got []string
	st.scheduleReplay = func(body []byte) int { got = append(got, string(body)); return http.StatusNoContent }
	st.replaySchedule()
	if len(got) != 1 || got[0] != `{"enabled":true}` {
		t.Errorf("set again: %q, want the setting kept", got)
	}
}
