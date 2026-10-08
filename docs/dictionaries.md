---
title: "Dictionaries"
description: "Replace the library's embedded tables at runtime with pkg/dict: airlines, aircraft types, wake, performance, airport names and limits, ATC units, GA types, systems profiles."
order: 17
section: "packages"
---

# Dictionaries

The library's embedded tables can be replaced at runtime (#768). The shipped copy stays the default; a host's data (the MyCrew app, fed from its API and a local cache) is merged over it by id. Replacing is safe while other goroutines look things up.

```go
names := dict.Names()                    // the tables
b, _ := dict.Export("traffic.telephony") // the shipped copy as JSON, to seed a store
err := dict.Use("traffic.telephony", b)  // the host's data, merged by id
dict.Reset("traffic.telephony")          // back to the shipped copy
```

A table's JSON is an envelope; `Use` also takes the bare items array:

```json
{"name": "traffic.telephony", "id": "icao", "source": "Wikipedia, \"List of airline codes\" …", "licence": "CC BY-SA 4.0 …",
 "items": [{"icao": "CSA", "telephony": "CSA LINES", "name": "Czech Airlines"}]}
```

| Table | Id | Items |
|---|---|---|
| `traffic.telephony` | `icao` | airline radio call signs (`TelephonyItem`); source and licence CC BY-SA 4.0 |
| `traffic.initialisms` | `word` | words said letter by letter ("CSA") |
| `traffic.aircraftTypes` | `type` | known types: title matches, airframe, approach and ground figures, take-off profile, flaps (`AircraftTypeItem`) |
| `traffic.wake` | `type` | ICAO and RECAT-EU wake categories (`WakeItem`) |
| `nav.performance` | `type` | cruise, climb, descent, fuel (`nav.Performance`) |
| `traffic.airportNames` | `icao` | airports as ATC names them in a clearance |
| `airport.limits` | `ICAO` | transition altitude, climb hand-over, preferential runways, tower, de-icing (`airport.Limits`, Go field names) |
| `traffic.aipUnits` | `icao` | ATC unit call signs by frequency (`AIPUnitItem`) |
| `traffic.airlines` | `icao` | the schedule's airlines with fleets and bases (`traffic.Airline`) |
| `traffic.gaTypes` | `kind` | the types each kind of GA operator flies, by weight |
| `systems.profiles` | `name` | aircraft systems profiles (`systems.Profile`; Windows builds) |

An item of a shipped id is read onto a copy of the shipped item, so it replaces only the fields it gives; the others keep their shipped value. Items with a new id are added after the shipped ones. An item without an id is refused. A name not in `Names()` gives `dict.ErrNoTable`.

## MyCrew API sets

`UseSet(set, data)` feeds a set of the MyCrew API (`GET /v1/aviation/{set}`) to every table fed from it and returns the tables fed; `Sets()` lists the sets in use. The set's items are `{"key": "A319", "closed": false, "deprecated": false, "payload": {…}}`: closed and deprecated items are left out, `key` is the item's id, and each payload field replaces the shipped item's field (null or empty strings keep the shipped value). `Use` takes this form too. Three tables are fed from sets: `systems.speeds` from `aircraft-speeds` (take-off speeds by type, see systems.md), `traffic.wake` from `aircraft-types` (its `wtc` and `recatEU` fields), and `traffic.telephony` from `airlines` (its `callsign` as the spoken call sign, and `name`), over the shipped list.
