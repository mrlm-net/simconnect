---
title: "ATC Game & Staffed Stations"
description: "Let a human controller work the World's traffic: staffed stations hand chosen positions to the player while the AI runs the rest, the per-aircraft manual toggle, and the airport map's scored ground-and-tower game."
order: 11
section: "traffic"
---

# ATC Game & Staffed Stations

A human can work the World's traffic as its controller. The usual way is to **staff stations**: the player takes chosen ATC positions of an airport and the AI keeps working the rest. A single aircraft can also be taken by hand, and the airport map has a small scored game of its own.

## Staffed stations

`World.SetStaffed([]world.StaffedStation{...})` (`PUT /api/stations/staffed`, a JSON list replacing the one in use) sets the stations players work; an empty list makes everything automatic again. `World.Staffed()` (`GET /api/stations/staffed`) lists them.

```go
w.SetStaffed([]world.StaffedStation{
    {ICAO: "LKPR", Position: "tower"},                 // the whole position
    {ICAO: "LKPR", Station: "Ruzyne Ground North"},    // one station of a position, by name
})
```

A `StaffedStation` is a whole position (`Position`: "delivery", "ground", "tower", "approach", …) or one station of it by its radio name (`Station`), at an airport (`ICAO`). Several entries make a band-box: one player working several stations. The stations of an airport and their sectors are its ATC stations (`GET /api/stations`, `PUT /api/stations`, a local `DataDir/stations.json` over the scenery's frequencies): a position may be split into sectors, such as Ground North and South by taxiway or an apron by area.

**What it does.** Once a second, every aircraft of ours is checked against the staffed stations by the position and station working it now:

- **On a staffed station** it is the player's: the AI gives it nothing (no clearances, speeds or vectors), as the per-aircraft manual toggle does. Its crew still calls on its own and flies its last clearance, and it keeps its place in the landing sequence.
- **Delivery staffed:** a departure calls for its en-route clearance, and the AI does not answer. The view offers the action `clearance`, which gives it (`World.Phrase(id, "clearance")` is its wording); delivery then hands it to ground. The view names delivery as its position until the clearance is given, the position allowed to clear it in network play.
  - **Amended:** `World.GiveClearance(id, ClearanceAmend{Runway, SID, LevelFt})` (`POST /api/control/{id}/clearance?runway=06&sid=…&level=4000`) gives it off another runway, with the SID named or the one off that runway from the same fix, and with a lower initial climb, which tower then keeps. It is said as applied: "DOBE 7D departure, runway 06, climb via SID to 4000 feet".
- **Leaving the staffed stations** (handed to a position the AI works) it goes back to the AI from its current state, and a request still waiting is answered, a departure clearance too.
- **Its own toggle wins:** an aircraft set manual or automatic by hand (below) is left as set.

The player gives clearances through the same calls as any host: `World.Clear(id, action)` (`POST /api/control/{id}/{action}`), `World.Approach(icao, callsign, action)` for the arrivals' sequence, `World.ApproachDescend(icao, callsign, ft)` for a lower level on the STAR (said against the transition altitude, never a climb), and `World.Phrase(id, action)` for what a clearance would say. Each is said on the radio with its readback, as the automatic controllers' are.

**Not yet.** `StaffedStation.Mode` (what the AI does on a staffed station: assist, silent, safety net) and `AutoHandoff` are accepted and kept but not acted on yet: today a staffed station behaves as "the AI gives nothing there".

## One aircraft by hand

`POST /api/control/{id}/manual?on=1` puts a single aircraft under the user's control; `?on=0` gives it back to the automatic controllers, and a request it is waiting on is answered. On the airport map this is the aircraft card's manual switch. In network play only the position working the aircraft may change it.

## The airport map's game

The [airport map](airport-layout.md#seeing-it-on-a-map) has a small scored game of its own: you work ground and tower at one airport while the game spawns the traffic.

```bash
go run -C cmd/airport-map .
# open http://127.0.0.1:8080/?icao=LKPR, then Schedule → ATC game → Start game
```

- A new flight appears every few minutes (the interval ±30 %), alternating departures and arrivals, at most 8 at a time.
- **Departures** appear on a free stand that fits them (airline from the call sign) with a SID for the runway, and wait for your **Pushback**.
- **Arrivals** appear at the entry of a STAR for the runway; MSFS AI flies the STAR and the approach transition, the injected approach takes over on final. After landing they wait clear of the runway for your **Taxi**.
- Every aircraft holds at each clearance: pushback, taxi (or a progressive taxi by clicking route points), crossings, line up, take-off. When something is in the way: **Hold position**, **Go around**, **Abort take-off** ([ATC Commands](traffic-commands.md)).
- Aircraft keep their own distance on the ground (they queue and give way at crossings), but they do not think for you on the runway.

| Score | Points |
|---|---|
| Departure airborne, arrival on its stand | +10 |
| A flight waiting for your clearance over a minute | −1 every 30 s |
| Wingtip separation lost on the ground (safe zones overlap, one of them moving) | −50 |
| Two aircraft on one runway (lining up, lined up or rolling, landing or rolling out) | −100 |
| An arrival within 10 NM of the threshold closer to the one ahead than its spacing on final | −25 |

The same conflict counts again only after a minute. The panel shows the score, flights handled and the next flight; the events log what happened, and the traffic log file keeps every clearance.

`GET /api/game` returns the state (`on`, `icao`, `runway`, `intervalSec`, `score`, `handled`, `spawned`, `started`, `nextInSec`, and the last 40 `events`); `POST /api/game` with `{"on": true, "icao": "LKPR", "runway": "24", "intervalSec": 180}` starts it, `{"on": false}` ends it. An empty `runway` means the runway in use. Without `intervalSec` a flight comes about every 3 minutes; `"intervalSec": 0` spawns none and scores the traffic the host makes, from the schedule for example.

The game spawns its own flights and gives you every clearance. For traffic that runs by itself from a timetable, with you on some stations only, use staffed stations with the [Traffic Manager](traffic-manager.md)'s schedule.
