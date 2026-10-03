---
title: "Installed Add-ons"
description: "Find the simulator's packages folder, list Community and streamed packages, find the package of the loaded aircraft, fingerprint the set and list running processes with pkg/addons."
order: 11
section: "packages"
---

# Installed Add-ons

`pkg/addons` reads what is installed in Microsoft Flight Simulator. It only reads files and the process list; no SimConnect connection is needed. It reports what is there and leaves the meaning to the caller: which package or process stands for ATC, a network client, GSX or a traffic add-on is the application's catalogue, not the library's.

```go
in, err := addons.Find() // MSFS 2024 before 2020
if err != nil {
    return err // addons.ErrNotFound
}
pkgs, err := addons.Scan(in.Packages)
fp := addons.Fingerprint(pkgs) // drop scenery caches when this changes
procs, _ := addons.Processes()  // "Couatl64.exe", "FlightSimulator2024.exe", ...
```

## Where the packages are

`Installs` reads `InstalledPackagesPath` from each `UserCfg.opt` it finds:

| Sim | Store | UserCfg.opt |
|---|---|---|
| 2024 | Steam | `%APPDATA%\Microsoft Flight Simulator 2024\UserCfg.opt` |
| 2024 | Microsoft Store | `%LOCALAPPDATA%\Packages\Microsoft.Limitless_8wekyb3d8bbwe\LocalCache\UserCfg.opt` |
| 2020 | Steam | `%APPDATA%\Microsoft Flight Simulator\UserCfg.opt` |
| 2020 | Microsoft Store | `%LOCALAPPDATA%\Packages\Microsoft.FlightSimulator_8wekyb3d8bbwe\LocalCache\UserCfg.opt` |

Only the 2024 Steam path has been checked on a real install; the others are the usual locations. `ReadPackagesPath` reads one file directly.

## Packages

`Scan` returns one `Package` per folder in `Community`, `Community2024`, `Official2024\<store>`, `Official2020\<store>` and `StreamedPackages`, sorted by source and folder.

- **Community and Official.** Title, creator, `content_type` and `package_version` come from `manifest.json`, kept as written. `content_type` is not reliable: GSX Pro and `navigraph-nav-base` both say `SCENERY`. Linked folders (symlinks, junctions) count; an unreadable manifest leaves only the folder name.
- **Streamed.** These folders have no manifest, only `content\*.fsarchive`. When the name reads as an airport, `Publisher` and `ICAO` are filled (`ParseStreamedName`):

  | Folder | Publisher | ICAO |
  |---|---|---|
  | `fs20-orbx-airport-lkpr-prague` | orbx | LKPR |
  | `fs20-gaya-simulations-airport-loww-vienna` | gaya-simulations | LOWW |
  | `fs24-asobo-airport-c53-lowerloon` | asobo | C53 |
  | `fs20-fps-lkmt-ostrava-airport` | fps | LKMT |

  Names with no airport code (`fs20-microsoft-airport-voloport`, `fs24-asobo-modellib-airport-generic`, travel books) are left without one.

### What a streamed folder means

This was looked at on one MSFS 2024 Steam install, with 1,248 streamed folders:

- **Every folder** holds `content\minimal.fsarchive`, a small stub.
- **788 of them** also hold further `.fsarchive` files, here or in subfolders. That is content the sim downloaded and keeps on disk; `Package.Cached` counts those files.
- **Airports:** 198 folders read as an airport; 23 had cached content, the others only the stub.

Stock and marketplace items appear alike. So a streamed folder shows that **the sim knows the package**, not that it is owned or active. Treat a streamed airport as a hint, not proof that its scenery will load.

## The loaded aircraft's package

`AircraftPackage(pkgs, path)` takes the path the sim reports for the `AircraftLoaded` system state or event. For example, `SimObjects\Airplanes\FNX_32X\presets\fnx\FNX_319_CFM_WF_HD\config\aircraft.CFG` gives `fnx-aircraft-319-321`.

- **Packages with a manifest** are matched on their `layout.json` file list.
- **Streamed packages** match when their content holds the aircraft's folder (`content\SimObjects\Airplanes\Asobo_Baron_G58`).
- **Order:** Community packages are tried first, since they override the others.

## Fingerprint

`Fingerprint` is a SHA-256 over each package's source, folder (lower case) and version, in any order. It changes when a package is added, removed or updated:

- use it as the key of a cache built from scenery, such as airport layouts;
- streamed packages count by name only, because their cached content comes and goes.

## Processes

`Processes` returns the executable names running now, sorted and without duplicates, from a Toolhelp snapshot. Matching them against known add-ons is up to the caller.
