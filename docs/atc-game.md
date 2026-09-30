---
title: "ATC Game"
description: "Work ground and tower at an MSFS airport with the airport map: arrivals on STARs, departures on SIDs, every clearance yours, scored."
order: 6
section: "traffic"
---

# ATC Game

The [airport map](airport-layout.md#seeing-it-on-a-map) doubles as a small ATC game built only on the SDK (v0.10): you work ground and tower at one airport while traffic comes and goes.

```bash
cd examples/airport-map && go run .
# open http://127.0.0.1:8080/?icao=LKPR, pick the runway in use under Route,
# then ATC game → ▶ Start game
```

## How it plays

- A new flight appears every few minutes (the interval ±30 %), alternating departures and arrivals, at most 8 at a time.
- **Departures** appear on a free stand that fits them (airline from the call sign) with a SID for the runway, and wait for your **Pushback**.
- **Arrivals** appear at the entry of a STAR for the runway; MSFS AI flies the STAR and the approach transition, the injected approach takes over on final. After landing they wait clear of the runway for your **Taxi**.
- Every aircraft holds at each clearance: pushback, taxi (or a progressive taxi by clicking route points), crossings, line up, take-off. When something is in the way: **Hold position**, **Go around**, **Abort take-off** ([ATC Commands](traffic-commands.md)).
- Aircraft keep their own distance on the ground (they queue and give way at crossings), but they do not think for you on the runway.

## Score

| | Points |
|---|---|
| Departure airborne, arrival on its stand | +10 |
| A flight waiting for your clearance over a minute | −1 every 30 s |
| Wingtip separation lost on the ground (safe zones overlap, one of them moving) | −50 |
| Two aircraft on one runway (lining up, lined up or rolling, landing or rolling out) | −100 |

The same conflict counts again only after a minute. The panel shows the score, flights handled and the next flight; the events log what happened, and the traffic log file keeps every clearance.

## API

`GET /api/game` returns the state (`on`, `icao`, `runway`, `intervalSec`, `score`, `handled`, `spawned`, `started`, `nextInSec`, and the last 40 `events`); `POST /api/game` with `{"on": true, "icao": "LKPR", "runway": "24", "intervalSec": 180}` starts it, `{"on": false}` ends it. An empty `runway` means the runway in use.

The game spawns its own flights and gives you every clearance. For traffic that runs by itself from a timetable, see the [Traffic Manager](traffic-manager.md).
