---
title: "Phraseology"
description: "ICAO and FAA radiotelephony phraseology side by side for the gate-to-gate IFR airline flow, with sources, readbacks and differences."
order: 13
section: "traffic"
---

# Phraseology

The radio (see [Radio](traffic-radio.md)) generates what controllers and pilots say, and the text is shown and spoken as it is. This page is the reference that text is checked against: the ICAO wording and the FAA wording for each step of an IFR airline flight, from the clearance at the stand to the stand at the destination, with the readback each one needs and where the two differ.

Rules for this page:

- Every phrase is quoted from a primary document, with its paragraph. ICAO documents print phrases in capitals; they are written here in normal case with the words unchanged. `(...)` is a value to insert, `[...]` is optional, as in the source documents.
- ICAO is Doc 4444 (PANS-ATM). Where Doc 4444 gives no wording or no order, the UK CAP 413 is quoted as the ICAO-based example and marked "CAP 413". CAP 413 is a UK document: it follows ICAO but has UK differences, which are noted.
- FAA is JO 7110.65 (controller) and the AIM (pilot).
- Anything not found in a source that was read is marked **(unverified)**.
- ICAO Doc 9432 (Manual of Radiotelephony) and ICAO Annex 10 Volume II were not available and were not read. Statements that depend on them are marked **(unverified)**.

## Sources

| Document | Edition | Where | Parts read |
|---|---|---|---|
| ICAO Doc 4444, PANS-ATM | 16th edition 2016, pages up to Amendment 11 (3/11/22) | [FOCA copy](https://www.bazl.admin.ch/dam/it/sd-web/jMIWMg9YgaoW/icao_doc_4444_airtrafficmanagement.pdf) | 4.5.4, 4.5.7 (clearances, readback), 4.11 (reports, initial call), 6.3.2, 6.5.2 (standard departure/arrival clearances), 7.3, 7.6.3, 7.9.3 (tower), 11.4.2.6.2, chapter 12 (12.2, 12.3, 12.4, 12.7) |
| UK CAA CAP 413, Radiotelephony Manual | Edition 24, effective 1 July 2026 | [caa.co.uk/cap413](https://www.caa.co.uk/cap413) ([PDF](https://www.caa.co.uk/publication/download/18165)) | chapter 2 (numbers, call signs, readback), 3 (general, initial calls), 4 (aerodrome, ATC part), 5 (surveillance), 6 (approach), 11 (flight examples) |
| FAA Order JO 7110.65BB, Air Traffic Control | effective 9 July 2026 | [faa.gov ATC order](https://www.faa.gov/air_traffic/publications/atpubs/atc_html/) | 2-1-17, 2-4, 2-7-2, 3-7, 3-9, 3-10, 4-2, 4-3, 4-5-7, 4-6, 4-7, 4-8, 5-3, 5-6-2, 5-7, 5-9 |
| FAA Aeronautical Information Manual | effective 9 July 2026 | [faa.gov AIM](https://www.faa.gov/air_traffic/publications/atpubs/aim_html/) | 4-2, 4-3-18, 4-3-21, 4-4-3, 4-4-7, 5-2, 5-3-1 |
| FAA Pilot/Controller Glossary | with the above | [P/CG "G"](https://www.faa.gov/air_traffic/publications/atpubs/pcg_html/glossary-g.html) | GO AROUND |
| AIP Czech Republic, LKPR AD 2.18 | current eAIP (read September 2026) | [aim.rlp.cz](https://aim.rlp.cz/eaip/html/eAIP/LK-AD-2.LKPR-en-GB.html) | unit call signs and frequencies at Prague |

## 1. Call signs and numbers

### Aircraft call signs

| | ICAO | FAA |
|---|---|---|
| Airline call sign | Telephony designator + flight number, digits said separately: "Speedbird Two Four Six" for BAW 246 (CAP 413 2.13, Table 4). | Telephony + flight number in group form: "American Fifty-two", "Delta One Hundred", "United One Zero One" (7110.65 2-4-20). Foreign ICAO 3LD: "telephony followed by the flight number in group form, or separate digits may be used if that is the format used by the pilot. Do not abbreviate." (2-4-20) |
| Abbreviation | After satisfactory communication and if no confusion is likely, the ground station may abbreviate; the pilot only after the ground station has. A telephony + flight number call sign ("Midland 640", "Type C") has **no abbreviation** (CAP 413 2.26, Table 10). | Do not abbreviate aircraft with an ICAO 3LD or FAA-authorised call sign (7110.65 2-4-9). |
| Super/heavy | "Super" or "Heavy" after the call sign on the initial call to each unit (Doc 4444 7.3 b, 4.11.3 b; CAP 413 2.30). | "Super"/"heavy" part of the identification in all communications (7110.65 2-4-14); "United Twenty-Five Heavy" (AIM 4-2-4). |
| Callsign position in a reply | Ground to air: callsign, then message. Air to ground: a new request starts with the callsign; a readback or acknowledgement ends with it: "Descend FL80, G-ABCD" (CAP 413 2.49). | Include the aircraft identification in all readbacks (7110.65 2-4-3; AIM 4-4-7). AIM example: "Climbing to Flight Level three three zero, United Twelve". |

The spoken call sign is the telephony designator, never the three-letter code. CSA's telephony is "CSA Lines" (Wikipedia; ICAO Doc 8585 not read, **unverified**), so CSA273 is "CSA Lines two seven three" (ICAO) or "CSA Lines two seventy-three" (FAA group form).

### Unit call signs

| Unit | ICAO (CAP 413 2.19, Table 9) | FAA (7110.65 2-4-19, AIM 4-2-6) |
|---|---|---|
| Clearance delivery | "Delivery" | "Clearance Delivery" ("LaGuardia Clearance Delivery") |
| Ground | "Ground" | "Ground" ("O'Hare Ground") |
| Tower | "Tower" | "Tower" |
| Approach | "Approach"; radar in general "Radar"; "Director", "Departure", "Arrival" where approved | "Approach"; departure function "Departure" ("Boston Departure") |
| Area | "Control" | "Center" |

A real airport's call signs come from its AIP. At Prague (LKPR AD 2.18) they are "Ruzyně Delivery" 120.060, "Ruzyně Ground" 121.910, "Ruzyně Tower" 134.560, "Ruzyně Radar" 119.010 and 118.310, "Praha Radar" 120.530 and 127.580. There is no "Ruzyně Departure".

The station name may be dropped after contact is established if no confusion results (CAP 413 2.23; 7110.65 2-4-9 "Omit the facility identification after communication has been established").

### Numbers

| | ICAO | FAA |
|---|---|---|
| Digits | "Zero, wun, too, tree, fower, fife, six, seven, ait, niner"; "decimal" (CAP 413 Table 3). | TBL 2-4-1 "ICAO Phonetics" gives the same pronunciations (TREE, FOW-ER, FIFE, NIN-ER) for the words Three, Four, Five, Nine (7110.65 2-4-16). In practice the written words are "three", "five", "niner" (AIM examples). |
| Separate digits | Call signs, altimeter settings, flight levels, headings, wind, speeds, frequencies (CAP 413 2.13.1). | Serial numbers, flight levels, headings, codes, wind, speeds, runways, frequencies (7110.65 2-4-17). |
| Thousands/hundreds | Altitude, height, cloud height, visibility, RVR with whole hundreds/thousands: digits of the thousands + "thousand", digits of the hundreds + "hundred": 2500 "two thousand five hundred", 11 000 "one one thousand" (CAP 413 2.13.2, Table 5). UK: QNH 1000 "one thousand", squawk 1000 "one thousand". | Altitudes: "One zero thousand", "One seven thousand niner hundred"; may be restated in group form "Ten thousand" (2-4-17). AIM 4-2-8/4-2-9 the same. |
| Flight level | "Flight level" + digits; UK: whole hundreds "flight level one hundred" (CAP 413 2.13, 3.9.4). | "Flight level" + separate digits: "Flight level one eight zero" (2-4-17). |
| Level with QNH | "(number) feet" (Doc 4444 12.3.1.1 c); UK: "altitude (number) feet" and QNH on the first message (CAP 413 3.9.2). | "(number) thousand": no "feet", no "altitude". |
| Pressure | "QNH (number) [units]" (Doc 4444 12.3.1.8 l); "hectopascals" below 1000 (CAP 413 3.4). | "Altimeter, three zero zero one" (2-4-17). |
| Transition | By state (transition altitude/level published in the AIP). | Flight levels at and above FL 180 (AIM 4-2-9). |
| Heading | "Heading (three digits)"; UK: "degrees" appended when the heading ends in zero (CAP 413 3.3). | "Heading zero three zero", "omitting the word degrees" (2-4-17). |
| Wind | "[Surface] wind (number) degrees (speed) (units)" (Doc 4444 12.3.1.8 a); UK may omit "surface", "degrees", "knots": "wind 270 20" (CAP 413 3.5). | "Wind zero three zero at two five", "Wind two seven zero at one five gusts three five" (2-4-17). |
| Speed | "(number) knots" (Doc 4444 12.4.1.6). | "Two five zero knots"; Mach "Mach point six four" (2-4-17). |
| Frequency | Digits with "decimal" (CAP 413 2.14). UK: all six figures, only the first four if the last two are zero: 118.125 "one one eight decimal one two five", 118.000 "one one eight decimal zero" (CAP 413 2.15, Table 6). The ICAO (Annex 10) rule was not read **(unverified)**. | Digits with "point"; omit digits after the second decimal: 135.275 "One three five point two seven" (2-4-17). AIM 4-2-8 notes ICAO uses "decimal". Ground in the 121 band may be "Contact Ground Point Seven" (2-1-17). |
| Runway | Digits separately (CAP 413 2.13). | "Runway Two Seven Right" (2-4-17). |
| Squawk | Digits separately; UK: "one thousand" for 1000 (CAP 413 Table 5). | "One zero zero zero", "Two one zero zero" (2-4-17). |
| Take-off | "Take-off", hyphenated; used only to clear or cancel a take-off, otherwise "departure" (Doc 4444 7.9.3.3; CAP 413 2.67). | "Takeoff", one word; same restriction (7110.65 4-3-1). |

Taxiway and procedure designators are spoken with the phonetic alphabet (A1 "Alpha One"). CAP 413 speaks SID designators as "Wicken 3 Delta departure" (2.68).

## 2. Clearance delivery

| | ICAO | FAA |
|---|---|---|
| Pilot request | Not worded in Doc 4444. CAP 413 11 shows a request on Ground with stand, ATIS and QNH before start-up. **(unverified for the exact request wording)** | Initial contact: facility, full identification, position on the airport, request (AIM 4-2-3): "Columbia Ground, Cessna Three One Six Zero Foxtrot, south ramp, I-F-R Memphis." |
| Items | Aircraft identification, clearance limit (normally the destination), SID designator, cleared level, SSR code, other instructions such as frequency (Doc 4444 6.3.2.3). Order: identification, limit, route, levels, other (11.4.2.6.2.1). A SID without a cleared level does not authorise climb on the SID profile (6.3.2.3 Note 2). | Order: identification, clearance limit, SID or vectors, route, altitude data, Mach, special information, frequency and beacon code (7110.65 4-2-1). The limit is followed by "airport": "Cleared to (destination) airport" (4-3-2). |
| Phrases | "(aircraft call sign) cleared to" (12.3.2.1 b); "to (location)" / "flight planned route" (12.3.2.2); "(standard departure name and number) departure" / "cleared (designation) departure" (12.3.3.1 d, f); "climb via SID to (level)" (12.3.1.2 z, 6.3.2.4). | "(SID name and number) departure, (transition name) transition" (4-3-2); "then as filed" (4-3-3); "Climb via SID" / "Climb via SID except maintain (altitude)" where the SID has crossing restrictions, otherwise "maintain (altitude)"; "expect (altitude) (time) after departure"; "departure frequency (frequency), squawk (code)" (3-9-3). |
| Example | CAP 413 2.68: "BIGJET 347, cleared to Kennington via A1, Wicken 3 Delta departure, squawk 5501". | 4-3-3: "Cleared to Reynolds Airport; David Two Departure, Kingham Transition; then, as filed. Maintain niner thousand. Expect flight level four one zero, one zero minutes after departure." |
| Readback | Route clearances are always read back (Doc 4444 4.5.7.5.1 a), and levels, SSR codes, runway in use (c). CAP 413 2.68: "Cleared to Kennington via A1, Wicken 3 Delta departure, squawk 5501, BIGJET 347" / controller "BIGJET 347, correct". | AIM 4-4-7: read back altitudes, vectors, runway; "Altitudes contained in charted procedures, such as DPs ... should not be read back unless they are specifically stated". 4-3-2 Note: controllers or pilots may initiate a readback; some company rules require it. |
| Notes | The runway is not in the Doc 4444 item list; it must be read back when given (4.5.7.5.1 c). UK: a route clearance and local departure instructions are not passed in the same transmission (CAP 413 2.66). | "CRAFT" (clearance limit, route, altitude, frequency, transponder) is a pilot memory aid; it is not in 7110.65 or the AIM sections read **(unverified)**. |

## 3. Start-up and pushback

| | ICAO | FAA |
|---|---|---|
| Request | "[aircraft location] request start up" / "... request start up, information (ATIS identification)" (Doc 4444 12.3.4.3 a, b); "[aircraft location] request pushback" (12.3.4.4 a). CAP 413 4.9: "Stourton Ground, BIGJET 347, stand 24 information Bravo, QNH 1022 request start up". | No start-up or pushback phraseology in 7110.65: the ramp is a non-movement area (3-7-2 "Movement of aircraft or vehicles on nonmovement areas is the responsibility of the pilot, the aircraft operator, or the airport management"). Gate hold: "Start engines, advise when ready to taxi" (3-9-2). |
| Approval | "Start up approved" (12.3.4.3 c); "start up at (time)", "expect start up at (time)", "start up at own discretion" (d-f). "Pushback approved" (one word), "pushback at own discretion", "expect (number) minutes delay due (reason)" (12.3.4.4 b-e). CAP 413 4.11: "BIGJET 347, stand 27 pushback/powerback approved". | Ramp control wording is local **(unverified)**. |
| Combined | Doc 4444 has no combined phrase; start-up and pushback are separate items. A combined "start up and pushback approved" is not in Doc 4444 or CAP 413 **(unverified)**. | - |
| Facing / tail | No "facing" phrase in Doc 4444 12.3.4.4 or CAP 413 4.11. Pushback direction wording is local **(unverified)**. | "Tail (direction)" is not in 7110.65 or the AIM **(unverified)**. |
| Readback | Not in the mandatory readback list (Doc 4444 4.5.7.5.1); acknowledge so that it is clearly understood (4.5.7.5.1.1). CAP 413 11 notes: "The word APPROVED is used - not CLEARED". | - |

## 4. Taxi

| | ICAO | FAA |
|---|---|---|
| Request | "[aircraft type] [wake turbulence category if super or heavy] [aircraft location] request taxi [intentions]" (Doc 4444 12.3.4.7 a). | "State your aircraft identification, location, type of operation planned (VFR or IFR), and the point of first intended landing": "Washington ground, Beechcraft One Three One Five Niner at hangar eight, ready to taxi, I-F-R to Chicago." (AIM 4-3-18) |
| Taxi to the runway | "Taxi to holding point [number] [runway (number)] via (specific route to be followed) [time (time)] [hold short of runway (number) (or cross runway (number))]" (12.3.4.7 e). UK drops "to": "G-ABCD, taxi holding point G2 runway 24 via taxiway Charlie" (CAP 413 4.12). | Runway first, then the route: "Runway (number), taxi via (route as necessary) (hold short instructions as necessary)": "Runway Three-Six Left, taxi via Alpha, Charlie, cross Runway One-Zero." (7110.65 3-7-2) Never "cleared" for taxi (3-7-1). |
| Taxi to stand | "Taxi to terminal (or other location) [stand (number)]" (12.3.4.7 j); "your stand (or gate) (designation)" (12.3.4.20 d). CAP 413 4.68: "taxi to Stand 27 via taxiway Alpha". | "Taxi/continue taxiing/proceed via (route)... to (location)" (3-7-2). AIM 4-3-18: "taxi to Page via taxiways echo three, echo one, and echo niner." |
| Hold short | "Hold short of (position)"; "hold (direction) of (position)"; "hold position" (12.3.4.8). A taxi limit beyond a runway needs an explicit clearance to cross or an instruction to hold short (7.6.3.1.1.2). | "Hold short of (runway)", "hold position" (3-7-2, 3-7-4). When the pilot does not read it back: "Read back hold instructions." (3-7-2) |
| Cross a runway | Pilot "request cross runway (number)"; "cross runway (number) [report vacated]"; "expedite crossing runway (number) traffic (aircraft type) (distance) kilometres (or miles) final" (12.3.4.9). CAP 413 4.12: "taxi to the flying club, via A1 cross runway 24 report vacated". | "Cross (runway) at (runway/taxiway), hold short of (runway)": "Cross Runway One-Six Left at Taxiway Bravo, hold short of Runway One-Six Right." One crossing clearance per runway (3-7-2). |
| Runway name when crossing | "(number)": one runway designator. No example in Doc 4444 or CAP 413 names both ends ("12/30"). Which of the two designators to use is not stated in the sources read **(unverified)**. | "Runway Two-Eight Left"; one designator (3-7-2). |
| Readback | Mandatory for taxi, hold short and cross instructions on a runway (Doc 4444 4.5.7.5.1 b). "Hold", "hold position" and "hold short of" are acknowledged with "holding" / "holding short"; "roger" and "wilco" are insufficient (12.3.4.8 note). CAP 413: "Taxi holding point A1 runway 24 via Charlie, G-CD"; "Holding, G-CD"; "Taxi to the flying club, via A1 cross runway 24, Wilco, G-CD". Pilot reports "runway vacated" when the whole aircraft is beyond the runway-holding position (12.3.4.9 e note). | Pilots read back the runway assignment, any clearance to enter a runway, and any hold short or line up and wait (AIM 4-3-18). AIM example readback: "Beechcraft One Three One Five Niner, runway two seven, hold short of runway three three left." (callsign first in this AIM example) |
| Conditional | Order: identification, condition, clearance, reiteration: "SAS 941, behind DC9 on short final, line up behind" (Doc 4444 12.2.7). CAP 413 4.14: "BIGJET 347, behind the landing A320, via Bravo 1 cross runway 26 behind, report vacated". | Conditional instructions depending on an arrival or a departure on the runway are not used: "Do not say, 'Line up and wait behind landing traffic,' or 'Taxi/proceed across Runway Three-Six behind departing/landing Citation.'" (3-7-1) |

## 5. Line up and take-off

| | ICAO | FAA |
|---|---|---|
| Ready | "Report when ready [for departure]", "are you ready [for departure]?", pilot "ready" (Doc 4444 12.3.4.10 b-e). CAP 413 4.20: "G-CD, ready for departure". | "Turbine-powered aircraft may be considered ready for takeoff when they reach the runway unless they advise otherwise." (3-9-10 Note) |
| Line up | "Line up [and wait]"; "line up runway (number)" (runway when confusion is possible in multiple-runway operations); "line up. Be ready for immediate departure" (12.3.4.10 f-h). CAP 413 4.20: "G-CD, runway 28, line-up"; 4.31: "BIGJET 347, line-up and wait Runway 26 - vehicle crossing upwind end of runway". | Runway first: "Runway (number), line up and wait." (3-9-4); intersection: "Runway (number) at (taxiway designator), line up and wait." Traffic information is issued with it. |
| Conditional line up | "(condition) line up (brief reiteration of the condition)"; pilot "(condition) lining up (brief reiteration)" (12.3.4.10 i, j). | Not used (3-7-1, 3-9-4). |
| Take-off clearance | "Runway (number) cleared for take-off [report airborne]" (12.3.4.11 a). The runway designator is included (7.9.3.4). Immediate: CAP 413 4.30 "BIGJET 347, runway 28 cleared for immediate take-off"; from the holding point this means enter and take off without stopping (Doc 4444 7.9.3.5). | "Runway (number), cleared for takeoff." (3-9-10); intersection: "Runway (number) at (taxiway designator) cleared for takeoff." |
| Wind | Doc 4444 chapter 12 has no wind in the take-off clearance and no order. CAP 413 puts it after the clearance: "BIGJET 347, runway 28, cleared for take-off, surface wind calm" (Figure 7); "The surface wind will be passed if there is a significant difference to that already passed" (4.27). The order "wind ..., runway (n), cleared for take-off" is not in either source **(unverified)**. | Civil: surface wind is departure information (3-9-1); the civil take-off phrase has no wind. Military only: "Runway (number), wind (surface wind in direction and velocity). Cleared for takeoff." (3-9-10) |
| Separate transmission | "A take-off clearance shall be issued separately from any other clearance message" (CAP 413 4.29). Doc 4444 lists line-up (12.3.4.10) and take-off (12.3.4.11) as separate phrases. | 3-9-4 and 3-9-10 are separate phrases. |
| Cancel | "Hold position, cancel take-off I say again cancel take-off (reasons)", pilot "holding"; "stop immediately [(repeat call sign) stop immediately]", pilot "stopping"; "take off immediately or vacate runway"; "take off immediately or hold short of runway" (12.3.4.11 c-h). | "Cancel takeoff clearance (reason)." (3-9-11) |
| Readback | Mandatory for entering and taking off from a runway (4.5.7.5.1 b). CAP 413: "Runway 28 line up, G-CD"; "Line up and wait, BIGJET 347"; "Runway 28 cleared for take-off, BIGJET 347" (runway before the clearance). | AIM 4-4-7: initial readback of a departure clearance includes the runway. AIM 5-2-5 pilot LUAW report example: "Cessna 234AR holding in position Runway 24L." |

## 6. After departure

| | ICAO | FAA |
|---|---|---|
| Transfer | "Contact (unit call sign) (frequency) [now]"; "at (or over) (time or place) [or when] [passing/leaving/reaching (level)] contact (unit call sign) (frequency)" (Doc 4444 12.3.1.4 a, b). CAP 413 4.32: "BIGJET 347, contact Westbury Radar 121.750". | "Contact (facility name or location name and terminal function), (frequency)" (7110.65 2-1-17): "Contact Departure." The departure frequency may be omitted if given before or published on the SID. Controllers instruct civil aircraft to contact departure "about 1/2 mile beyond the runway end" (3-9-3). |
| Readback | Frequency changes: CAP 413 2.69 item 12: "Westbury Radar 121.750, BIGJET 347". Doc 4444 4.5.7.5.1 does not list frequencies **(Doc 4444 does not require it; UK does)**. | Acknowledge frequency changes (AIM 4-2-3): "one three four point five, United Two Twenty-Two". |
| Pilot initial call | Station, call sign [super/heavy], "level, including passing and cleared levels if not maintaining the cleared level", assigned speed (Doc 4444 4.11.3, when prescribed by the ATS authority). UK for a SID: "Westbury Departure, BIGJET 347, BIGRO 5D, passing altitude 2300 feet climbing FL80" (CAP 413 6.2). | "(Name) CENTER, (aircraft identification), LEAVING (exact altitude or flight level), CLIMBING TO (altitude or flight level)" (AIM 5-3-1). |
| Identification | "Radar contact [position]"; "identified [position]" (Doc 4444 12.4.1.1 d, e). UK: inside controlled airspace a departing aircraft is not told it is identified (CAP 413 5.6, Table 1); the reply is "BIGJET 347, Westbury Departure, Roger". | "Radar contact (position if required)." (7110.65 5-3-7) |
| Climb | "Climb (or descend)" followed as necessary by "to (level)" (12.3.1.2 a); "maintain" is not used in place of climb or descend (12.3.2.3 Note). UK: "climb FL70" (no "to" with flight levels), "climb to altitude 6000 feet" (CAP 413 3.9). "Climb via SID to (level)" (12.3.1.2 z). | "Climb and maintain (altitude)." (4-5-7) "Climb via SID", "Climb via SID except maintain (altitude)" (4-3-2). |
| Readback | Level instructions are read back (4.5.7.5.1 c): "Climb FL70, G-CD" (CAP 413 3.10). | Read back altitudes (AIM 4-4-7): "Climbing to Flight Level three three zero, United Twelve". |

## 7. Levels, headings and speed in flight

| | ICAO | FAA |
|---|---|---|
| Levels | "Flight level (number)" / "(number) feet" (12.3.1.1). "Stop climb (or descent) at (level)", "expedite climb (or descent)", "when ready climb (or descend) to (level)" (12.3.1.2). A level resumed after a stop for known traffic is plain "climb (or descend) to (level)" (12.3.1.2 a); "clear of traffic [appropriate instructions]" is for passing unknown traffic (12.4.1.8 d). | "Climb/descend and maintain (altitude)", "descend at pilot's discretion, maintain (altitude)", "cross (fix) at (altitude)" (4-5-7). |
| Altimeter | "QNH (number)" with the level: "descend to altitude 2000 feet Borton QNH 1000" (CAP 413 3.9). | "The (facility name) altimeter (setting)" below the lowest usable FL (2-7-2). |
| Heading | "Turn left (or right) heading (three digits) [reason]", "fly heading (three digits)", "continue present heading" (12.4.1.3). Reasons: "due traffic", "for spacing", "for delay", "for downwind (or base, or final)" (12.4.1.5 note). | "Turn left/right heading (degrees)", "fly heading (degrees)", "vector for spacing" (5-6-2). |
| Speed | "Maintain (number) knots [or greater (or or less)] [until (significant point)]"; "increase (or reduce) speed to (number) knots"; "increase (or reduce) speed by (number) knots"; "resume normal speed"; "no [ATC] speed restrictions" (12.4.1.6). CAP 413 6.24: "BIGJET 347, for spacing reduce speed to 210 knots", readback "210 knots, BIGJET 347". | "Increase/reduce speed: to (specified speed in knots) ... or (number of knots) knots": "Reduce speed to two five zero", "Reduce speed twenty knots" (5-7-2). Without "to" the number is a change, not a target. |
| Readback | Heading and speed instructions are read back (4.5.7.5.1 c). | Read back vectors and altitudes in the order given (AIM 4-4-7). |

## 8. Arrival

| | ICAO | FAA |
|---|---|---|
| Initial call to approach | Station, call sign, level (4.11.3). CAP 413 6.8: "Kennington Approach, BIGJET 347, descending FL90 information Charlie". | AIM 4-2-3: "(ATIS) Information Charlie received" may be included; AIM 5-3-1 level format. |
| STAR | "Cleared (designation) arrival" (12.3.3.2 a). Standard arrival clearances contain identification, STAR, runway in use (unless part of the STAR), cleared level (6.5.2.3). A STAR without a cleared level does not authorise descent on the STAR profile (6.5.2.3 Note 2). "Descend via STAR to (level)" (12.3.1.2 ff). | "(STAR name and number) arrival, (transition name) transition" (4-7-1); "Descend via (STAR name and number)", "Descend via (STAR), (runway number)" (4-5-7); "Bayview Three Arrival, Helen Transition, maintain Flight Level Three Three Zero." (4-7-1) |
| Descent | "Descend (to) (level)" (12.3.1.2). CAP 413 6.8: "cleared direct to North Cross descend FL50". | "Descend and maintain (altitude)" (4-5-7). |
| Expected approach | "Expect ILS approach runway 28 QNH 1011" (CAP 413 6.9); "vectoring for (type of approach) approach runway (number)" (Doc 4444 12.4.2.1 a). | Approach clearance or "type approach to be expected" is approach information (4-7-10). "Vector to final approach course" (5-6-2). |
| Approach clearance | "Cleared (type of approach) approach [runway (number)]" (12.3.3.2 f); with surveillance "cleared for (type of approach) approach runway (number)" (12.4.2.2 d). CAP 413 6.28: "BIGJET 347, cleared ILS approach runway 28, QNH 1011". | "Cleared (specific procedure to be flown) approach": "Cleared I-L-S Runway Three-Six Approach." (4-8-1); with vectors: "Four miles from LIMA. Turn right heading three four zero. Maintain two thousand until established on the localizer. Cleared I-L-S runway three six approach." (5-9-4) |
| Vectors off the STAR | A dog-leg or an extended downwind: "fly heading (three digits)" (12.4.1.3 d), "turn left (or right) heading (three digits) [reason]" (e), the reasons "for spacing", "for base" (12.4.1.5 note b, d); back onto the STAR "resume own navigation direct (significant point)" (12.4.1.4 b); onto the final "turn left (or right) heading (three digits) to intercept" with the approach clearance (12.4.2.2 g, d). Headings are magnetic (`HeadingSaid`, airport `MagVar`). | With the approach clearance, as above: "Turn right heading three four zero ... Cleared I-L-S runway three six approach." (5-9-4) |
| Level by a fix | "Cross (significant point) at (or above, or below) (level)" (12.3.2.4 a): a climb over (a descent under) traffic by a fix ahead, the climb going on (#662). | |
| Established | "Report established on localizer (or on [GLS/RNP/MLS] [final] approach [course])" (12.4.2.2 e); "intercept (localizer ...) [runway (number)] [report established]" (m); "report established on glide path" (l). CAP 413 6.27 pilot: "Localiser established runway 28, BIGJET 347". | No "report established" phrase in 5-9 read; the controller confirms established from radar **(the pilot report is not required in the sections read)**. |
| Holding | "Cleared (or proceed) to (fix) [maintain (level)] hold [(direction)] as published expect approach clearance (or further clearance) at (time)" (12.3.3.3 b); "no delay expected", "expected approach time (time)", "delay not determined (reasons)" (12.3.3.4). CAP 413 6.11: "hold at North Cross FL60 expect onward clearance at time 40". | "Cleared to (fix), hold (direction), as published", "expect further clearance (time)" (4-6-1); "Hold (direction) of (fix) on (radial ...)", "(number) minute/mile leg", "left turns" (4-6-4). |
| Sequence | "Number ... follow (aircraft type and position)" (12.3.4.14 b, circuit); CAP 413 6.23: "number 4 in traffic, 18 miles from touchdown". A "lose (n) minutes" phrase was not found in Doc 4444, CAP 413 or the FAA sections read **(unverified)**. | "Number two following a United Seven-Thirty-Seven two mile final" (3-10-6). |
| Readback | Approach clearances, levels, headings, speeds, QNH (4.5.7.5.1; CAP 413 2.69 item 6). CAP 413 readback: "Cleared ILS approach runway 28, QNH 1011, BIGJET 347". | Runway and altitudes (AIM 4-4-7). |

## 9. Landing and after landing

| | ICAO | FAA |
|---|---|---|
| Transfer to tower | "Contact (unit call sign) (frequency)"; UK: "BIGJET 347, contact Kennington Tower 118.5" (CAP 413 6.34); "number 1 contact Tower 118.7" (6.9). | "Contact Tower." (2-1-17); "Over final approach fix. Contact tower one one eight point one." (5-9-4) |
| Initial call to tower | Station, call sign [super/heavy], position (Doc 4444 7.3). CAP 413 Figure 20: "Kennington Tower, BIGJET 347, long final runway 28". | Facility, identification, position (AIM 4-2-3). |
| Landing clearance | "Runway (number) cleared to land" (12.3.4.16 a). | "Runway (number) cleared to land." (3-10-5) Runway changed: "Change to runway (number), runway (number) cleared to land." |
| Wind | No wind and no order in Doc 4444 12.3.4.16. CAP 413 puts it after: "BIGJET 347, runway 28 cleared to land, wind 270 20" (4.51); "runway 28, cleared to land, surface wind 240 10" (Figure 20). The order "wind ..., runway (n), cleared to land" is not in either source **(unverified)**. | Civil: surface wind is landing information (3-10-1); the civil phrase has no wind. Military only: "Runway (number), wind (surface wind direction and velocity), cleared to land." (3-10-5) |
| Continue | "Continue approach [prepare for possible go around]" (12.3.4.15 d). | "Continue" (3-10-1); "Runway (number), continue, traffic holding in position" (3-9-4). |
| Go around | "Go around"; pilot "going around" (12.3.4.18). CAP 413 4.64: "BIGJET 347, go around, I say again, go around, acknowledge" / "Going around, BIGJET 347". | "'Go around' (additional instructions if required)" (P/CG). |
| Vacate | "Vacate runway", "expedite vacating", "take (or turn) first (or second, or convenient) left (or right) and contact ground (frequency)", "when vacated contact ground (frequency)" (12.3.4.7 y, 12.3.4.20). Pilot "runway vacated" (12.3.4.7 z). CAP 413 4.68: "BIGJET 347, vacate left"; "BIGJET 347, when vacated contact Ground 118.350"; Figure 21: "BIGJET 347, vacate convenient right". "Vacate via (taxiway)" appears in CAP 413 only in the military chapter. | "Turn left/right (taxiway/runway)", "If able, turn left/right (taxiway/runway)", "hold short of (runway)" (3-10-9): "Skywest Ten Forty-two, turn right next taxiway, cross runway two one, contact ground point seven." |
| Taxi to stand | "Taxi to terminal [stand (number)]", "your stand (or gate) (designation)" (12.3.4.7 j, 12.3.4.20 d). CAP 413 4.68 pilot on Ground: "Kennington Ground, BIGJET 347, runway vacated" / "BIGJET 347, Kennington Ground, taxi to Stand 27 via taxiway Alpha". | Ground issues the taxi to parking; it does not authorise entering or crossing a runway (AIM 4-3-21). |
| Readback | Landing clearance (4.5.7.5.1 b): "Runway 28 cleared to land, BIGJET 347" (CAP 413). Frequency: "When vacated Ground 118.350, BIGJET 347" (CAP 413 4.68). | Landing clearance with runway (AIM 4-4-7): "November Five Charlie Tango, roger, cleared to land runway nine left." Hold short on exit: read back (3-10-9). |

## 10. Handoffs in general

| | ICAO | FAA |
|---|---|---|
| Phrase | "Contact (unit call sign) (frequency) [now]"; "stand by for (unit call sign) (frequency)"; "monitor (unit call sign) (frequency)"; "remain this frequency" (Doc 4444 12.3.1.4). Identity of the unit and the frequency, in one message; items needing readback go in a separate transmission before the transfer (CAP 413 2.57, 2.58). | "Contact (facility name or location name and terminal function), (frequency)", "[at (time, fix, or altitude)]"; "Monitor Tower"; "change to my frequency (frequency)"; "remain this frequency" (2-1-17). The location name is omitted within the same facility. |
| Readback | UK: frequency changes read back in full: "Wrayton Control 129.125, BIGJET 347" (CAP 413 2.60, 2.69). | Acknowledge: "one three four point five, United Two Twenty-Two" (AIM 4-2-3). |
| First call on the new frequency | Call sign and level only (and assigned speed or heading) (CAP 413 3.26, 3.27; Doc 4444 4.11.3). | "(Name) Center, (identification), level (altitude)" or "leaving (altitude), climbing/descending to (altitude)" (AIM 5-3-1). |

## 11. VFR in the aerodrome traffic circuit

ICAO only so far: the FAA's traffic pattern sections (7110.65 3-10, AIM 4-3) have not been read for this table yet, so the FAA wording is still the ICAO text (`phraseFAA` has no VFR cases).

| | ICAO (Doc 4444 12.3.4.13–12.3.4.17) | In the code |
|---|---|---|
| Entering the circuit, pilot | "[aircraft type] (position) (level) FOR LANDING" (13 a); with ATIS "(aircraft type) (position) (level) INFORMATION (ATIS identification) FOR LANDING" (13 d). | `VFRForLanding`: "Ruzyne Tower, OKABC, Cessna 172, 3 miles south, 1400 feet, information Alpha, for landing" |
| Join | "JOIN [(direction of circuit)] (position in circuit) (runway number) [SURFACE] WIND (direction and speed) (units) [TEMPERATURE ...] QNH (or QFE) (number) [(units)] [TRAFFIC (detail)]" (13 b); with ATIS "JOIN (position in circuit) [RUNWAY (number)] QNH (or QFE) (number) [TRAFFIC (detail)]" (13 e). | `JoinCircuit`: "OKABC, join left downwind runway 24, wind 240 degrees 8 knots, QNH 1013" |
| Straight in | "MAKE STRAIGHT-IN APPROACH, RUNWAY (number) [SURFACE] WIND … QNH … [TRAFFIC (detail)]" (13 c). | `StraightIn` |
| In the circuit, pilot | "(position in circuit, e.g. DOWNWIND/FINAL)" (14 a). | `CircuitReport`: "OKABC, downwind" |
| Sequence | "NUMBER ... FOLLOW (aircraft type and position) [additional instructions if required]" (14 b). | `FollowTraffic`: "OKABC, number 2, follow the Airbus A320 on final" |
| Approach instructions | "MAKE SHORT APPROACH"; "MAKE LONG APPROACH (or EXTEND DOWNWIND)"; "REPORT BASE (or FINAL, or LONG FINAL)"; "CONTINUE APPROACH [PREPARE FOR POSSIBLE GO AROUND]" (15 a–d). "LONG FINAL" is reported when turning final more than 4 NM out, or 8 NM out on a straight-in; "FINAL" is then required at 4 NM (15 note). | `CircuitInstruction` with `InstrShortApproach`, `InstrLongApproach`, `InstrExtendDownwind`, `InstrReportBase`, `InstrReportFinal`, `InstrContinue` |
| Landing | "RUNWAY (number) CLEARED TO LAND" (16 a); special operations: "CLEARED TOUCH AND GO" (16 c), "MAKE FULL STOP" (16 d). | `ClearedToLand`, `ClearedTouchAndGo`, `MakeFullStop` |
| Delaying | "CIRCLE THE AERODROME"; "ORBIT (RIGHT, or LEFT) [FROM PRESENT POSITION]"; "MAKE ANOTHER CIRCUIT" (17 a–c). | `CircuitDelay` with `DelayCircle`, `DelayOrbitRight`, `DelayOrbitLeft`, `DelayAnotherCircuit` |
| Readback | The QNH is read back (4.5.7.5.1); the rest repeats the instruction, as the other clearances in the code do. | `Readback`: "Join left downwind runway 24, QNH 1013, OKABC" |

## Differences at a glance

- Order of runway and taxi route: ICAO "taxi to holding point (x) runway (n) via (route)"; FAA "runway (n), taxi via (route)".
- **Our taxi clearance** (a project choice, asked for in review): "taxi to and hold short of runway (n) [at (entry)] via (route)", read back the same. It names the clearance limit the way the hold short instructions above do, rather than Doc 4444's "taxi to holding point".
- Take-off: ICAO "take-off", FAA "takeoff". Both put the runway before "cleared".
- Wind in a take-off or landing clearance: no position in Doc 4444; after the clearance in CAP 413; not in the FAA civil phrase (military: before "cleared").
- Levels: ICAO "climb (to) flight level (n)" / "(n) feet"; FAA "climb and maintain (altitude)" with no "feet", flight levels from FL180.
- Pressure: ICAO "QNH (hPa)"; FAA "altimeter (inHg)".
- Frequencies: ICAO "decimal", all six digits (UK rule); FAA "point", two digits after the point at most.
- Call signs: ICAO digits separately and "no abbreviation" for airline call signs; FAA group form, no abbreviation.
- Conditional clearances: ICAO allows them with a strict order; FAA forbids conditions on arrivals or departures on the runway.
- Identification: ICAO "identified" or "radar contact"; FAA "radar contact".
- Speed: "reduce speed to (n) knots" is a target in both; FAA "reduce speed (n) knots" without "to" is a reduction by that amount.
