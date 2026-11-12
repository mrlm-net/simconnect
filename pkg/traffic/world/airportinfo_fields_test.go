package world

import (
	"reflect"
	"testing"
	"time"

	"github.com/mrlm-net/simconnect/pkg/nav"
)

// TestWeatherInfoCoversWeather: the airport info's weather carries every
// field of nav.Weather (#1527: Variable was dropped, a variable wind read
// as one from the north), but its time.
func TestWeatherInfoCoversWeather(t *testing.T) {
	info := reflect.TypeOf(weatherInfo{})
	w := reflect.TypeOf(nav.Weather{})
	for i := 0; i < w.NumField(); i++ {
		f := w.Field(i)
		if !f.IsExported() || f.Name == "Time" {
			continue
		}
		if _, ok := info.FieldByName(f.Name); !ok {
			t.Errorf("weatherInfo has no %s", f.Name)
		}
	}
}

// TestATISStartLetter: the first ATIS is not Alpha everywhere (#1509); the
// same airport and time give the same letter, the next half hour the next.
func TestATISStartLetter(t *testing.T) {
	at := time.Date(2026, 11, 12, 9, 10, 0, 0, time.UTC)
	letters := map[byte]bool{}
	for _, icao := range []string{"LKPR", "LKTB", "EDDM", "EGLL", "LOWW", "KJFK", "LFPG", "EHAM"} {
		l := atisStartLetter(icao, at)
		if l < 'A' || l > 'Z' {
			t.Fatalf("%s: letter %q", icao, l)
		}
		if atisStartLetter(icao, at) != l {
			t.Errorf("%s: not repeatable", icao)
		}
		if n := atisStartLetter(icao, at.Add(30*time.Minute)); n != 'A'+(l-'A'+1)%26 {
			t.Errorf("%s: %c then %c half an hour later", icao, l, n)
		}
		letters[l] = true
	}
	if len(letters) < 4 {
		t.Errorf("8 airports, %d letters", len(letters))
	}
}
