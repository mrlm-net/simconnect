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
- **Approach staffed:** an arrival's first call goes unanswered, and its STAR clearance waits for the action `clearance`. The crew flies its flight plan meanwhile. Without an approach clearance it asks for one about 3 minutes before the end of its STAR, its clearance limit, and holds there. The action `approach` clears it onto the approach.
- **Departure staffed:** handed over after take-off, a departure climbs to the level of its clearance and levels there. Its crew asks for its cruising level. The action `climb` (`?level=` feet) clears it on.
- **Landing:** on a staffed station, the crew goes around without the player's `land`, as it does without the AI tower's.
- **Handing over:** leaving a staffed station, the crew stays on the player's frequency until the action `handoff` ("contact Ruzyne Tower 134.56"). A station with `AutoHandoff` hands off by itself, as the AI does.
- **Leaving the staffed stations** (handed to a position the AI works) it goes back to the AI from its current state, and a request still waiting is answered, a departure clearance too.
- **Its own toggle wins:** an aircraft set manual or automatic by hand (below) is left as set.

`ControlView.Actions` follows the crew: on the stand, nothing until it has its IFR clearance and has called, then what it called for. `ControlView.Request` says what it called for. A clearance ahead of the crew is still taken, as a controller can expedite; `ControlView.ClearedEarly` lists those, so a host can warn or score them. An aircraft cleared early gets no "contact ground" after its late IFR clearance (#1308).

`ControlView.ActionParams` names the offered actions that need a query parameter: `upto` and `vacate` need `node`, `climb` needs `level`. `upto` without `node` answers 400 (#1308).

A clearance is taken at once and said when the frequency clears. Until then `ControlView.SayingUntil` holds when what is booked for that aircraft has been said, so a host can show it as queued; `GET /api/radio` lists a queued transmission with its `at` in the future (#1312).

An arrival's `Actions` follow its phase the same way: `land` and `goaround` while it is in the air, `vacate` with them until the wheels are down, nothing on the landing roll — the aeroplane is stopping and the next word is its own — and the ground clearances from the moment it is off the runway. It becomes ground's at `ArrivalVacating`, not at the full stop, so a staffed tower hands it on as it leaves the runway rather than after it has parked on a taxiway waiting.

`vacate` names the exit, as `taxi` and `upto` name a point: `POST /api/control/{id}/vacate?node=N`, where N is any node on the exit — the turn-off, the taxiway behind it, or the runway centreline where it begins. It is the tower's word before the landing; after touchdown it is refused, because the rollout's braking is already planned to the exit it was given.


The player gives clearances through the same calls as any host: `World.Clear(id, action)` (`POST /api/control/{id}/{action}`), `World.Approach(icao, callsign, action)` for the arrivals' sequence, `World.ApproachDescend(icao, callsign, ft)` for a lower level on the STAR (said against the transition altitude, never a climb), and `World.Phrase(id, action)` for what a clearance would say. Each is said on the radio with its readback, as the automatic controllers' are.

**Modes.** `StaffedStation.Mode` is what the AI does on the station when an aircraft on it is predicted to lose separation. In every mode the routine clearances, speeds, levels and vectors stay the player's, and the AI gives no traffic information there either.

- `assist` (also `""`, the default): an alert. `World.Alerts()` (`alerts` in `GET /api/separation`) lists each conflict of the player's aircraft with its pair, the time to the loss of separation, the staffed callsigns and their modes. `Suggested` is what the AI would say first, worded as on the radio: to an en-route aircraft or a departure its resolution (a level, a heading, a crossing restriction); to the arrival landing later of two on their STARs, a stop of its descent above the other well out, else a speed for spacing ("number 2, for spacing reduce speed to 180 knots"). The AI does nothing about it.
- `silent`: no alert, nothing from the AI.
- `safetynet` (`safety-net` too): an alert with a `Deadline`, at least 30 s after the alert, 1 minute after the player's last instruction to either aircraft, and 2 minutes before the earliest loss predicted. Is the conflict still predicted then, the AI resolves it as for its own aircraft: a level, a crossing restriction, a speed, a vector. The alert shows `SafetyNet`, the log says so, the resolution in `GET /api/separation` is marked `safetyNet`, and the safety net's calls carry `Params["by"] = "safetynet"` so a game can show who said them.

An aircraft taken over by hand (its manual toggle, or a clearance from the user off the staffed stations) has no station mode: silent.

**Network play.** Each player staffs their own stations. `PUT /api/stations/staffed?as=tower` (or the `X-ATC-Position` header), or `World.SetStaffedBy("tower", list)` in Go, sets the stations of the player playing as `tower` and leaves the other players' as they are; an empty list gives that player's stations back to the AI. Each entry is kept with `By`, the player. Without `as` (the host), the list replaces everything, as before.

- A player clears the aircraft on every station they staff, whatever position they play as: a tower player band-boxing ground gives the ground clearances too.
- On a station a player staffs, only that player clears. The position's own client is refused there as well.
- A single station staffed by one player (Ground North) wins over its whole position staffed by another (ground).

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
