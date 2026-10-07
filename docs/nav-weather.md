---
title: "Weather, Runway in Use & ATIS"
description: "Read the weather with pkg/nav, choose the runway in use from the wind and broadcast an ICAO style ATIS."
order: 2
section: "nav"
---

# Weather, Runway in Use & ATIS

`pkg/nav` holds the navigation side of an airport environment: navigation data, weather and ATIS, flight plans. This page covers weather, the runway in use and the ATIS.

```go
import "github.com/mrlm-net/simconnect/pkg/nav"
```

| Type / function | What it is |
|-----------------|-----------|
| `Weather` | Surface weather: wind, gusts, visibility, ceiling, temperature, dewpoint, QNH, precipitation |
| `WeatherReader` | Reads the ambient weather at the user aircraft from your message loop |
| `StaticWeather` | Weather set by the application (tests, fixed scenarios) |
| `ActiveRunways` | Departure and arrival runway ends for the wind, with an approach hint |
| `RunwaySelector` | Keeps the runway in use until it is out of limits or another has been better for a while |
| `ATIS` | One broadcast: `Text()` with digits, `Spoken()` spelled for a voice |
| `ATISService` | Keeps the current ATIS and advances its letter on significant changes |


## Reading the weather

`WeatherReader` follows the `airport.Loader` pattern: it never reads `client.Stream()` itself. Call `Request` (once) or `Subscribe` (every second, only when changed), and pass every message to `Handle`:

```go
wx := nav.NewWeatherReader(client, 10000, 10001) // data definition ID, request ID
wx.Request()
for msg := range client.Stream() {
    if w, ok := wx.Handle(msg); ok {
        fmt.Printf("wind %03.0f°T %.0f kt, QNH %.0f\n", w.WindDirTrue, w.WindKts, w.QNHhPa)
    }
}
```

It reads these SimVars of the user aircraft (`SIMCONNECT_OBJECT_ID_USER`):

| SimVar | Unit | Weather field |
|--------|------|---------------|
| `AMBIENT WIND DIRECTION` | degrees (true) | `WindDirTrue` |
| `AMBIENT WIND VELOCITY` | knots | `WindKts` |
| `AMBIENT VISIBILITY` | meters | `VisibilityM` |
| `AMBIENT TEMPERATURE` | celsius | `TempC` |
| `SEA LEVEL PRESSURE` | millibars | `QNHhPa` |
| `AMBIENT PRECIP STATE` | mask | `Precip` (`none`, `rain`, `snow`) |
| `AMBIENT IN CLOUD` | bool | `InCloud` |

**Limitation:** SimConnect gives the ambient weather where the user aircraft is, not per airport. That is the airport's weather while the user is on the ground there or close by, which is the case when the airport is the world centre around the user; for other airports it is only an approximation. Gusts, ceiling and dewpoint have no SimVar: the reader leaves `GustKts` and `CeilingFt` at 0 and `DewpointC` NaN, and the ATIS leaves them out. Set them yourself (or build the whole `Weather` with `StaticWeather`) when you have them from elsewhere. `Weather.Variable` marks a variable wind (a METAR's VRB): no runway gets a head- or crosswind from it (`Components` returns 0, 0) and the ATIS says "wind variable". `Stop` ends a subscription, `Last` returns the latest weather read, and `Reset` registers the definition again after a reconnect.

The same `Weather` sets the spacing on final: `traffic.ConditionsFrom(weather, runwayHeadingTrue)` turns it into approach conditions (low visibility, runway surface, headwind), see [Weather on final](traffic-separation.md#weather-on-final).

## Runway in use

```go
lim := nav.RunwayLimits{
    Preferred: []string{"24", "06"}, // LKPR's preferential runways
    // MaxTailwindKts: 5 (default), MaxCrosswindKts: 25 (default), MinLengthM: 0
}
use := nav.ActiveRunways(layout, w, lim)
fmt.Println(use.Departure.Name, use.Arrival.Name, use.Approach)
```

For every runway end of the layout (at least `MinLengthM` long), the headwind and crosswind components come from the end's true heading and the true wind; the limits are checked with the gusts when they are stronger than the mean wind. Then:

1. The first `Preferred` end within the limits wins, even when another end has more headwind. That is how preferential runway systems work: LKPR keeps 24 in calm wind and with up to 5 kt of tailwind.
2. Otherwise the end with the most headwind among those within the limits, ties (within 1 kt) going to the longer runway.
3. When no end is within the limits, the one with the most headwind is taken and `WithinLimits` is false.

`PreferredArrival` gives arrivals their own preference list, for split operations (`RunwayUse.Single()` is then false). `nav.RunwayLimitsFrom(airport.LimitsFor(...))` fills `Preferred` from the airport limits. `Approach` (also `ApproachFor(w)`) is `ApproachILS` when visibility is below 5000 m or the ceiling below 1500 ft, else `ApproachVisual` ("visual/RNAV"); pick the actual procedure from `airport.Procedures`.

### Parallel runways used together

With parallel runways (headings within 15°), `ActiveRunways` also uses the parallels of the chosen end that are in the same direction and within the wind limits. `RunwayUse.Departures` and `Arrivals` list every end in use; `Departure` and `Arrival` are the first of them. `Parallel` says how they work together, and `SpacingM` is the distance between their centre lines.

The mode follows the spacing (`ParallelModeFor`). The figures are from the ICAO draft manual on simultaneous operations on parallel instrument runways (AN-Conf/11-IP/3):

| Spacing | Mode | Use |
|---------|------|-----|
| under 760 m | `ParallelNone` | one runway, for wake turbulence (2.3.3.2) |
| 760 m | `ParallelSegregated` | arrivals on one runway, departures on the other (departures side by side from 760 m, 3.3.2) |
| 915 m | `ParallelDependent` | both runways mixed; approaches dependent, 2 NM diagonally between adjacent finals (2.3.1.1, 2.3.2.2) |
| 1035 m | `ParallelIndependent` | both runways mixed; each final on its own (the Annex 14 distance) |

Runways that cross are never used together. A third parallel joins only if it is far enough from both. `RunwayLimits.Parallel` sets an airport's own mode, never more than the spacing allows; for example `ParallelSegregated` for an airport that keeps one runway for arrivals, or `ParallelNone` for one runway only. `Nearest(layout, ends, p)` is the parallel nearest a point, such as a stand. The ATIS names every runway in use: "runways in use 26L and 26R", or "landing runway 27R, departure runway 27L".

### Keeping the runway in use

An airport does not change runways with every wind shift. `RunwaySelector` keeps the runway in use until one of two things happens:
- the runway in use is out of its tailwind or crosswind limits, gusts included (`MaxTailwindKts`, `MaxCrosswindKts`);
- another runway has been the better choice for `ChangeAfter` (`RunwayChangeAfter`, 10 min).

```go
var sel nav.RunwaySelector // one per airport, kept
use := sel.Choose(time.Now(), layout, weather, limits)
```

A runway is only chosen when it is `RunwayChoiceMarginKts` (2 kt) within its wind limits; one in use is kept up to the limits themselves. Below `RunwayCalmKts` (3 kt) the runway in use stays while it is within its limits. An out-of-limits runway changes at once only when another is within them. `Ready`, when set, picks the moment of a due change (a gap in the traffic), waiting at most `MaxChangeWait` (`RunwayChangeMaxWait`, 15 min); `Pending` reports the change coming, and `Seed` starts a selector from a runway already in use.

While it holds, the headwind and crosswind it reports are those of the runway kept. The airport map uses a selector per airport for traffic and for the Airport panel. Near a tailwind limit in light, variable wind, the runway had flipped between 06 and 24 from one minute to the next.

## ATIS

```go
svc := nav.NewATISService("Ruzyne", layout, lim, 5000, nav.ATISWithMagVar(procs.MagVar))
a, changed := svc.Update(w, time.Now())
if changed {
    fmt.Println(a.Text())
    speak(a.Spoken())
}
```

`Text()` gives the broadcast in ICAO phraseology:

```text
Ruzyne information Alpha, time 1320, runway in use 24, wind 240 degrees 8 knots, visibility 10 kilometers or more, temperature 15, dewpoint 8, QNH 1013, transition level 70, advise on initial contact you have information Alpha.
```

`Spoken()` spells the numbers for a voice library: digits one by one with "niner", whole hundreds and thousands as words and runway suffixes as words:

```text
Ruzyne information Alpha, time one three two zero, runway in use two four, wind two four zero degrees eight knots, visibility one zero kilometers or more, temperature one five, dewpoint eight, QNH one zero one three, transition level seven zero, advise on initial contact you have information Alpha.
```

The broadcast includes, when known: split landing/departure runways, "wind calm" or "wind variable", "expect ILS approach", gusts (10 kt or more above the mean wind), visibility in kilometers from 5 km and in meters below, rain or snow, the ceiling, and a negative temperature as "minus". The wind is reported magnetic when a magnetic variation is given, in the facility data convention of `airport.Procedures.MagVar` (magnetic = true + MagVar; LKPR 356).

**QNH and transition level.** QNH is rounded down to a whole hPa. `TransitionLevel(ta, qnh)` is the lowest flight level in tens at least 1000 ft above the transition altitude at that QNH (27 ft per hPa); at LKPR (TA 5000 ft) that is FL 60 from QNH 1014 up, FL 70 from 1013 down to 977, FL 80 below.

**Letters.** `ATISService` starts at Alpha (`ATISWithLetter` changes it) and wraps from Zulu to Alpha. `Update` issues the next letter when:

- the departure or arrival runway, the approach hint, the QNH or the transition level changes;
- the wind turns 60° or more with 10 kt or more, or the wind or gust speed changes by 10 kt or more;
- visibility crosses 800, 1500, 3000 or 5000 m, or precipitation starts or stops;
- the broadcast is older than `DefaultATISMaxAge` (1 hour; `ATISWithMaxAge`).

The runway in use holds through wind shifts near a limit: the service keeps a `RunwaySelector`, or shares the traffic's with `ATISWithSelector(sel)`, so the ATIS says the runway the traffic uses (#454).

Smaller changes keep the current broadcast, as a real ATIS does between reports. `Current()` returns it; `NewATIS` builds a single broadcast without the service.

## Example

[`examples/atis`](https://github.com/mrlm-net/simconnect/tree/main/examples/atis) connects, reads the weather at the user aircraft, loads the airport layout and prints the ATIS once:

```sh
go run ./examples/atis -icao LKPR -name Ruzyne -prefer 24,06 -ta 5000 -magvar 356
```

```text
Ruzyne information Alpha, time 1646, runway in use 24, wind 120 degrees 6 knots, visibility 10 kilometers or more, temperature 21, QNH 1023, transition level 60, advise on initial contact you have information Alpha.
```

With the wind from 120° at 6 kt the preferred 24 has 3.6 kt of tailwind, within the 5 kt limit, so it stays in use.

## Icing

`IcingConditions(w)` reports weather in which departures need de-icing: at or below `IcingMaxTempC` (+3 °C) with visible moisture — precipitation, visibility below `IcingVisibilityM` (1500 m) or cloud at the aircraft. See [De-icing](traffic-taxi.md#de-icing).
