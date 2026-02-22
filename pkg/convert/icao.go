// https://en.wikipedia.org/wiki/ICAO_airport_code#Prefixes
package convert

// IsICAOCode returns true if the code is a valid ICAO airport code prefix (not a full registry check).
func IsICAOCode(code string) bool {
	if len(code) != 4 {
		return false
	}
	// Valid first letters (from Wikipedia ICAO prefix table).
	// Excluded: I, J (unassigned), Q, X (reserved), and pseudo-code ZZZZ.
	validFirst := "ABCDEFGHKLMNOPRSTUVWYZ"
	if !containsRune(validFirst, rune(code[0])) {
		return false
	}
	if code == "ZZZZ" {
		return false
	}
	return true
}

// containsRune returns true if r is in s
func containsRune(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}

// ICAORegion returns the region for a given ICAO code (first letter)
func ICAORegion(code string) string {
	if len(code) < 1 {
		return ""
	}
	switch code[0] {
	case 'A':
		return "Western South Pacific"
	case 'B':
		return "Greenland, Iceland, Kosovo"
	case 'C':
		return "Canada"
	case 'D':
		return "West Africa, Maghreb"
	case 'E':
		return "Northern Europe"
	case 'F':
		return "Central/Southern Africa, Indian Ocean"
	case 'G':
		return "West Africa, Maghreb"
	case 'H':
		return "East/Northeast Africa"
	case 'K':
		return "Contiguous United States"
	case 'L':
		return "Southern Europe, Israel, Turkey"
	case 'M':
		return "Central America, Mexico, Caribbean"
	case 'N':
		return "South Pacific, New Zealand"
	case 'O':
		return "Gulf States, Iran, Iraq, Pakistan, Jordan, West Bank"
	case 'P':
		return "North Pacific, Kiribati"
	case 'R':
		return "Japan, Korea, Philippines"
	case 'S':
		return "South America"
	case 'T':
		return "Caribbean"
	case 'U':
		return "Former Soviet countries"
	case 'V':
		return "South Asia, SE Asia, Hong Kong, Macau"
	case 'W':
		return "Maritime Southeast Asia"
	case 'Y':
		return "Australia"
	case 'Z':
		return "China, North Korea, Mongolia"
	default:
		return "Unknown"
	}
}

// ICAOCountry returns the country or territory for a given ICAO code (first two letters)
func ICAOCountry(code string) string {
	if len(code) < 2 {
		return ""
	}
	prefix := code[:2]
	if country, ok := icaoPrefixes[prefix]; ok {
		return country
	}
	// fallback: try single-letter prefix
	prefix = code[:1]
	if country, ok := icaoPrefixes[prefix]; ok {
		return country
	}
	return "Unknown"
}
