//go:build windows
// +build windows

package main

import (
	"flag"
	"log"
	"net/http"
	_ "net/http/pprof" // on http.DefaultServeMux, served only with -pprof
	"runtime"
	"time"
)

// pprofAddr serves Go's profiler (/debug/pprof/) for finding where the map
// spends its time, e.g. -pprof 127.0.0.1:6060; off by default.
var pprofAddr = flag.String("pprof", "", "serve the Go profiler on this address (e.g. 127.0.0.1:6060)")

// pprofLocks also records who waits on what (/debug/pprof/mutex, /block).
// Off unless asked: with both on, the Go runtime (1.27.1, Windows) froze
// the whole map twice within hours, the profiler included — a GC worker
// recording a lock event while holding a runtime lock, every goroutine
// then waiting on the profile's lock (dump read with Delve, 2026-10-16).
var pprofLocks = flag.Bool("pprof-locks", false, "with -pprof, also profile locks and blocking (has frozen the Go runtime; short runs only)")

func startPprof() {
	if *pprofAddr == "" {
		return
	}
	if *pprofLocks {
		runtime.SetMutexProfileFraction(1)
		runtime.SetBlockProfileRate(int(time.Millisecond))
	}
	go func() { log.Println(http.ListenAndServe(*pprofAddr, nil)) }()
}
