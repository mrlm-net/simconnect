# SimConnect Benchmark Example

## Overview

Stress-tests the SimConnect SDK by exercising the full Manager stack under load: data subscriptions at sim-frame rate, facility queries, state change monitoring, and concurrent channel consumption. Collects CPU/memory metrics using three complementary approaches.

## Profiling Approaches

| Approach | Flag / Env | What It Measures |
|----------|------------|------------------|
| GC trace | `GODEBUG=gctrace=1` | GC frequency, heap size, pause times |
| pprof | `-pprof` | CPU flame graphs, heap profiles, goroutine dumps |
| Runtime stats | Built-in | Heap allocation, GC cycles, goroutine count, message throughput |

## Running

### Basic (runtime stats only)

```bash
cd examples/simconnect-benchmark
go run . -duration 60s
```

### With GC trace

```bash
GODEBUG=gctrace=1 go run . -duration 60s
```

### With pprof (CPU + heap profiling)

```bash
go run . -duration 120s -pprof
```

Then in another terminal:

```bash
# CPU profile (30 seconds)
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# Heap profile
go tool pprof http://localhost:6060/debug/pprof/heap

# Goroutine dump
go tool pprof http://localhost:6060/debug/pprof/goroutine
```

### All three combined

```bash
GODEBUG=gctrace=1 go run . -duration 120s -pprof -interval 10s
```

## Saving Results

Save benchmark output for tracking:

```bash
go run . -duration 60s 2>&1 | tee results/$(date +%Y%m%d-%H%M%S).txt
```

## CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-duration` | `60s` | Benchmark duration |
| `-pprof` | `false` | Enable pprof HTTP server on `:6060` |
| `-interval` | `5s` | Stats reporting interval |
| `-buffer` | `512` | Engine message buffer size |

## Prerequisites

- Windows OS with MSFS 2020/2024 running
- SimConnect SDK installed (or use auto-detection)
- For best results, load a flight with AI traffic enabled

## Output Format

Periodic stats lines are machine-parseable:

```
[BENCH] t=15s msgs=1234 state=56 subs=789 fac=12 heap=4.2MB sys=8.1MB objs=15234 gc=5 pause=1.2ms goroutines=12
```

Final summary provides aggregate metrics for comparison across runs.

## What It Tests

The benchmark exercises all major SDK components simultaneously:

1. **High-frequency data subscriptions**: Camera and aircraft telemetry at `SIMCONNECT_PERIOD_SIM_FRAME` (every frame)
2. **Traffic queries**: All aircraft within 50km radius
3. **Facility queries**: Airport list requests
4. **State monitoring**: Connection state and simulator state changes
5. **Channel subscriptions**: Three concurrent channel consumers processing messages
6. **Manager lifecycle**: Auto-reconnect, heartbeat, graceful shutdown

This generates realistic load that stresses:
- DLL syscall overhead
- Message buffering and dispatch
- Tiered buffer pooling
- Subscription delivery
- State change notifications
- Memory allocation patterns
- Goroutine coordination

## Interpreting Results

### Runtime Stats (stderr output)

- **msgs/s**: Message throughput — higher is better, typical range 100-1000/s depending on sim activity
- **heap**: Current heap allocation — stable or slowly growing is good, rapid growth indicates leaks
- **sys**: Heap system memory — OS memory reserved for heap
- **gc**: Number of GC cycles — fewer is better, indicates lower allocation pressure
- **pause**: Total GC pause time — lower is better, high values cause stutter
- **goroutines**: Number of goroutines — should be stable after startup

### GC Trace (stdout with GODEBUG=gctrace=1)

Look for:
- GC frequency (time between cycles) — longer is better
- Heap size before/after GC — delta indicates allocation rate
- Pause times — <1ms is excellent, >10ms is problematic

### pprof Profiles

CPU profile:
- Identify hot paths (heavy functions)
- Look for unexpected allocations in hot paths
- Check syscall overhead vs. Go code

Heap profile:
- Find allocation hotspots
- Identify potential memory leaks
- Verify buffer pooling effectiveness

Goroutine dump:
- Check for goroutine leaks
- Verify subscription cleanup
- Identify blocked goroutines

## Troubleshooting

**No messages received**: Ensure MSFS is running and a flight is loaded. Try increasing `-duration` to allow connection time.

**Low throughput**: Check sim settings — high frame rates generate more data. Enable AI traffic for more activity.

**High GC pressure**: Reduce `-buffer` size or increase `-interval` to reduce logging overhead.

**pprof connection refused**: Verify firewall allows localhost:6060 or check for port conflicts.
