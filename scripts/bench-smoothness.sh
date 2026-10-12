#!/usr/bin/env sh
# Traffic smoothness benchmarks (docs/traffic-smoothness.md): runs them,
# keeps the output, and draws the charts from it and the recorded history.
#   scripts/bench-smoothness.sh            # run, then draw
#   BENCHTIME=5x scripts/bench-smoothness.sh
set -e
cd "$(dirname "$0")/.."
out=docs/images/smoothness
mkdir -p "$out"
go test -run '^$' -bench 'GroundChecksFrame|Spawn|PushPlan|DepartureTaxiFrame|AssignStandArrival' \
  -benchmem -benchtime "${BENCHTIME:-10x}" ./pkg/traffic/ ./pkg/traffic/world/ | tee "$out/bench.txt"
go run ./examples/smoothness-report -bench "$out/bench.txt" -history "$out/history.csv" -out "$out"
