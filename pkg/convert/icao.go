//go:build windows
// +build windows

// https://en.wikipedia.org/wiki/ICAO_airport_code#Prefixes
package convert

// this package will be used for conversions between different unit systems

// IsICAOCode returns true if the code is a valid ICAO airport code prefix (not a full registry check).
func IsICAOCode(code string) bool {
	if len(code) != 4 {
		return false
	}
	// Valid first letters (from Wikipedia ICAO prefix table)
	validFirst := "ABCDEFGHJKLMNOPTUVWYZ"
	if !containsRune(validFirst, rune(code[0])) {
		return false
	}
	// Exclude known pseudo-codes and reserved letters
	if code == "ZZZZ" || code[0] == 'Q' || code[0] == 'X' || code[0] == 'I' || code[0] == 'J' {
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

// icaoPrefixes maps ICAO code prefixes to region/country
var icaoPrefixes = map[string]string{
	// A – Western South Pacific
	"AG": "Solomon Islands",
	"AN": "Nauru",
	"AY": "Papua New Guinea",
	// B – Greenland, Iceland, and Kosovo (European Alternate)
	"BG": "Greenland",
	"BI": "Iceland",
	"BK": "Kosovo",
	// C – Canada
	"C": "Canada",
	// D – Eastern parts of West Africa and Maghreb
	"DA": "Algeria",
	"DB": "Benin",
	"DF": "Burkina Faso",
	"DG": "Ghana",
	"DI": "Côte d'Ivoire",
	"DN": "Nigeria",
	"DR": "Niger",
	"DT": "Tunisia",
	"DX": "Togo",
	// E – Northern Europe
	"EB": "Belgium",
	"ED": "Germany (civil)",
	"EE": "Estonia",
	"EF": "Finland",
	"EG": "United Kingdom (and Crown Dependencies)",
	"EH": "Netherlands",
	"EI": "Ireland",
	"EK": "Denmark and the Faroe Islands",
	"EL": "Luxembourg",
	"EN": "Norway",
	"EP": "Poland",
	"ES": "Sweden",
	"ET": "Germany (military)",
	"EV": "Latvia",
	"EY": "Lithuania",
	// F – Most of Central Africa, Southern Africa, and the Indian Ocean
	"FA": "South Africa",
	"FB": "Botswana",
	"FC": "Republic of the Congo",
	"FD": "Eswatini",
	"FE": "Central African Republic",
	"FG": "Equatorial Guinea",
	"FH": "Saint Helena, Ascension and Tristan da Cunha",
	"FI": "Mauritius",
	"FJ": "British Indian Ocean Territory",
	"FK": "Cameroon",
	"FL": "Zambia",
	"FM": "Comoros, France (Mayotte and Réunion), and Madagascar",
	"FN": "Angola",
	"FO": "Gabon",
	"FP": "São Tomé and Príncipe",
	"FQ": "Mozambique",
	"FS": "Seychelles",
	"FT": "Chad",
	"FV": "Zimbabwe",
	"FW": "Malawi",
	"FX": "Lesotho",
	"FY": "Namibia",
	"FZ": "Democratic Republic of the Congo",
	// G – Western parts of West Africa and Maghreb
	"GA": "Mali",
	"GB": "The Gambia",
	"GC": "Spain (Canary Islands)",
	"GE": "Spain (Ceuta and Melilla)",
	"GF": "Sierra Leone",
	"GG": "Guinea-Bissau",
	"GL": "Liberia",
	"GM": "Morocco",
	"GO": "Senegal",
	"GQ": "Mauritania",
	"GU": "Guinea",
	"GV": "Cape Verde",
	// H – East Africa and Northeast Africa
	"HA": "Ethiopia",
	"HB": "Burundi",
	"HC": "Somalia",
	"HD": "Djibouti",
	"HE": "Egypt",
	"HH": "Eritrea",
	"HJ": "South Sudan",
	"HK": "Kenya",
	"HL": "Libya",
	"HR": "Rwanda",
	"HS": "Sudan",
	"HT": "Tanzania",
	"HU": "Uganda",
	// K – Contiguous United States
	"K": "Contiguous United States",
	// L – Southern Europe, Israel, Palestine and Turkey
	"LA": "Albania",
	"LB": "Bulgaria",
	"LC": "Cyprus",
	"LD": "Croatia",
	"LE": "Spain (mainland section and Balearic Islands)",
	"LF": "France (Metropolitan France, Saint-Pierre and Miquelon)",
	"LG": "Greece",
	"LH": "Hungary",
	"LI": "Italy (and San Marino)",
	"LJ": "Slovenia",
	"LK": "Czech Republic",
	"LL": "Israel",
	"LM": "Malta",
	"LN": "Monaco",
	"LO": "Austria",
	"LP": "Portugal (including the Azores and Madeira)",
	"LQ": "Bosnia and Herzegovina",
	"LR": "Romania",
	"LS": "Switzerland and Liechtenstein",
	"LT": "Turkey",
	"LU": "Moldova",
	"LV": "Palestine/Occupied Palestinian territories",
	"LW": "North Macedonia",
	"LX": "Gibraltar",
	"LY": "Serbia and Montenegro",
	"LZ": "Slovakia",
	// M – Central America, Mexico and northern/western parts of the Caribbean
	"MB": "Turks and Caicos Islands",
	"MD": "Dominican Republic",
	"MG": "Guatemala",
	"MH": "Honduras",
	"MK": "Jamaica",
	"MM": "Mexico",
	"MN": "Nicaragua",
	"MP": "Panama",
	"MR": "Costa Rica",
	"MS": "El Salvador",
	"MT": "Haiti",
	"MU": "Cuba",
	"MW": "Cayman Islands",
	"MY": "Bahamas",
	"MZ": "Belize",
	// N – Most of the South Pacific and New Zealand
	"NC": "Cook Islands",
	"NF": "Fiji, Tonga",
	"NG": "Kiribati (Gilbert Islands), Tuvalu",
	"NI": "Niue",
	"NL": "France (Wallis and Futuna)",
	"NS": "Samoa, United States (American Samoa)",
	"NT": "France (French Polynesia)",
	"NV": "Vanuatu",
	"NW": "France (New Caledonia)",
	"NZ": "New Zealand, parts of Antarctica",
	// O – Gulf States, Iran, Iraq, Pakistan, Jordan, West Bank
	"OA": "Afghanistan",
	"OB": "Bahrain",
	"OE": "Saudi Arabia",
	"OI": "Iran",
	"OJ": "Jordan and the West Bank",
	"OK": "Kuwait",
	"OL": "Lebanon",
	"OM": "United Arab Emirates",
	"OO": "Oman",
	"OP": "Pakistan",
	"OR": "Iraq",
	"OS": "Syria",
	"OT": "Qatar",
	"OY": "Yemen",
	// P – most of the North Pacific, and Kiribati
	"PA": "US (Alaska) (also PF, PO and PP)",
	"PB": "US (Baker Island)",
	"PC": "Kiribati (Canton Airfield, Phoenix Islands)",
	"PF": "US (Alaska) (also PA, PO and PP)",
	"PG": "US (Guam, Northern Mariana Islands)",
	"PH": "US (Hawaii)",
	"PJ": "US (Johnston Atoll)",
	"PK": "Marshall Islands",
	"PL": "Kiribati (Line Islands)",
	"PM": "US (Midway Island)",
	"PO": "US (Alaska) (also PA, PF and PP)",
	"PP": "US (Alaska) (also PA, PF and PO)",
	"PT": "Federated States of Micronesia, Palau",
	"PW": "US (Wake Island)",
	// R – Japan, S. Korea, Philippines
	"RC": "Republic of China (Taiwan)",
	"RJ": "Japan (Mainland)",
	"RK": "South Korea (Republic of Korea)",
	"RO": "Japan (Okinawa)",
	"RP": "Philippines",
	// S – South America
	"SA": "Argentina (including parts of Antarctica) (also SR)",
	"SB": "Brazil (also SD, SI, SJ, SN, SS and SW)",
	"SC": "Chile (including Easter Island and parts of Antarctica) (also SH)",
	"SD": "Brazil (also SB, SI, SJ, SN, SS and SW)",
	"SE": "Ecuador",
	"SF": "Falkland Islands",
	"SG": "Paraguay",
	"SH": "Chile (also SC)",
	"SI": "Brazil (also SB, SD, SJ, SN, SS and SW)",
	"SJ": "Brazil (also SB, SD, SI, SN, SS and SW)",
	"SK": "Colombia",
	"SL": "Bolivia",
	"SM": "Suriname",
	"SN": "Brazil (also SB, SD, SI, SJ, SS and SW)",
	"SO": "France (French Guiana)",
	"SP": "Peru",
	"SS": "Brazil (also SB, SD, SI, SJ, SN and SW)",
	"SU": "Uruguay",
	"SV": "Venezuela",
	"SW": "Brazil (also SB, SD, SI, SJ, SN and SS)",
	"SY": "Guyana",
	// T – Eastern and southern parts of the Caribbean
	"TA": "Antigua and Barbuda",
	"TB": "Barbados",
	"TD": "Dominica",
	"TF": "France (Guadeloupe, Martinique, Saint Barthélemy, Saint Martin)",
	"TG": "Grenada",
	"TI": "US (U.S. Virgin Islands)",
	"TJ": "US (Puerto Rico)",
	"TK": "Saint Kitts and Nevis",
	"TL": "Saint Lucia",
	"TN": "Caribbean Netherlands, Aruba, Curaçao, Sint Maarten",
	"TQ": "Anguilla",
	"TR": "Montserrat",
	"TT": "Trinidad and Tobago",
	"TU": "British Virgin Islands",
	"TV": "Saint Vincent and the Grenadines",
	"TX": "Bermuda",
	// U – Most former Soviet countries
	"U":  "Russia (except as below)",
	"UA": "Kazakhstan",
	"UB": "Azerbaijan",
	"UC": "Kyrgyzstan",
	"UD": "Armenia",
	"UG": "Georgia",
	"UK": "Ukraine",
	"UM": "Belarus and Russia (Kaliningrad Oblast)",
	"UT": "Tajikistan, Turkmenistan",
	"UZ": "Uzbekistan",
	// V – Many South Asian countries, mainland Southeast Asia, Hong Kong and Macau
	"VA": "India (West India)",
	"VC": "Sri Lanka",
	"VD": "Cambodia",
	"VE": "India (East India)",
	"VG": "Bangladesh",
	"VH": "Hong Kong",
	"VI": "India (North India)",
	"VL": "Laos",
	"VM": "Macau",
	"VN": "Nepal",
	"VO": "India (South India)",
	"VQ": "Bhutan",
	"VR": "Maldives",
	"VT": "Thailand",
	"VV": "Vietnam",
	"VY": "Myanmar",
	// W – Most of Maritime Southeast Asia
	"WA": "Indonesia (also WI, WQ and WR)",
	"WB": "Brunei, Malaysia (East Malaysia)",
	"WI": "Indonesia (also WA, WQ and WR)",
	"WM": "Malaysia (Peninsular Malaysia)",
	"WP": "Timor-Leste",
	"WQ": "Indonesia (also WA, WI and WR)",
	"WR": "Indonesia (also WA, WI and WQ)",
	"WS": "Singapore",
	// Y – Australia
	"Y": "Australia (including Norfolk Island, Christmas Island, Cocos (Keeling) Islands and Australian Antarctic Territory)",
	// Z – China, North Korea and Mongolia
	"ZB": "Northern China",
	"ZG": "Southern China",
	"ZH": "Central China",
	"ZJ": "Hainan",
	"ZL": "Northwestern China",
	"ZP": "Yunnan",
	"ZS": "Eastern China",
	"ZU": "Southwestern China",
	"ZW": "Xinjiang",
	"ZY": "Northeast China",
	"ZK": "North Korea",
	"ZM": "Mongolia",
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
