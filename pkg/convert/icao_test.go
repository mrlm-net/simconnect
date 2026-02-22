package convert

import "testing"

func TestIsICAOCode(t *testing.T) {
	tests := []struct {
		name string
		code string
		want bool
	}{
		// valid codes — including previously-broken R and S prefixes
		{name: "KJFK (US)", code: "KJFK", want: true},
		{name: "EGLL (UK)", code: "EGLL", want: true},
		{name: "LKPR (Czech)", code: "LKPR", want: true},
		{name: "YSSY (Australia)", code: "YSSY", want: true},
		{name: "ZBAA (China)", code: "ZBAA", want: true},
		{name: "RJTT (Japan — was broken)", code: "RJTT", want: true},
		{name: "RKSI (Korea — was broken)", code: "RKSI", want: true},
		{name: "RPLL (Philippines — was broken)", code: "RPLL", want: true},
		{name: "SBGR (Brazil — was broken)", code: "SBGR", want: true},
		{name: "SCEL (Chile — was broken)", code: "SCEL", want: true},
		{name: "SKBO (Colombia — was broken)", code: "SKBO", want: true},
		// invalid codes
		{name: "empty string", code: "", want: false},
		{name: "3 chars", code: "KJF", want: false},
		{name: "5 chars", code: "KJFKX", want: false},
		{name: "lowercase", code: "kjfk", want: false},
		{name: "ZZZZ pseudo-code", code: "ZZZZ", want: false},
		{name: "Q prefix (reserved)", code: "QXYZ", want: false},
		{name: "X prefix (reserved)", code: "XABC", want: false},
		{name: "I prefix (unassigned)", code: "IABC", want: false},
		{name: "J prefix (unassigned)", code: "JABC", want: false},
		{name: "numeric", code: "1234", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsICAOCode(tt.code)
			if got != tt.want {
				t.Errorf("IsICAOCode(%q) = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
}

func TestICAORegion(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string
	}{
		{name: "KJFK US", code: "KJFK", want: "Contiguous United States"},
		{name: "EGLL Northern Europe", code: "EGLL", want: "Northern Europe"},
		{name: "YSSY Australia", code: "YSSY", want: "Australia"},
		{name: "LKPR Southern Europe", code: "LKPR", want: "Southern Europe, Israel, Turkey"},
		{name: "ZBAA China", code: "ZBAA", want: "China, North Korea, Mongolia"},
		{name: "CYYZ Canada", code: "CYYZ", want: "Canada"},
		{name: "RJTT Japan", code: "RJTT", want: "Japan, Korea, Philippines"},
		{name: "SBGR South America", code: "SBGR", want: "South America"},
		{name: "empty string", code: "", want: ""},
		{name: "9 prefix unknown", code: "9XXX", want: "Unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ICAORegion(tt.code)
			if got != tt.want {
				t.Errorf("ICAORegion(%q) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

func TestICAOCountry(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string
	}{
		{name: "EGLL via EG prefix", code: "EGLL", want: "United Kingdom (and Crown Dependencies)"},
		{name: "KJFK via K fallback", code: "KJFK", want: "Contiguous United States"},
		{name: "LKPR via LK prefix", code: "LKPR", want: "Czech Republic"},
		{name: "YSSY via Y fallback", code: "YSSY", want: "Australia (including Norfolk Island, Christmas Island, Cocos (Keeling) Islands and Australian Antarctic Territory)"},
		{name: "CYYZ via C fallback", code: "CYYZ", want: "Canada"},
		{name: "empty string", code: "", want: ""},
		{name: "single char", code: "K", want: ""},
		{name: "unknown prefix", code: "99XX", want: "Unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ICAOCountry(tt.code)
			if got != tt.want {
				t.Errorf("ICAOCountry(%q) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}
