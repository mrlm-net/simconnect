---
title: "Dictionaries"
description: "Replace the library's embedded tables at runtime with pkg/dict: airlines, aircraft types, wake, performance, airport names and limits, ATC units, GA types, systems profiles."
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

An item replaces the shipped item of the same id whole: send all its fields. Items with a new id are added after the shipped ones.
