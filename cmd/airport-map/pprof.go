//go:build windows
// +build windows

package main

import (
	"flag"
	"log"
	"net/http"
	_ "net/http/pprof" // on http.DefaultServeMux, served only with -pprof
)

// pprofAddr serves Go's profiler (/debug/pprof/) for finding where the map
// spends its time, e.g. -pprof 127.0.0.1:6060; off by default.
var pprofAddr = flag.String("pprof", "", "serve the Go profiler on this address (e.g. 127.0.0.1:6060)")

func startPprof() {
	if *pprofAddr == "" {
		return
	}
	go func() { log.Println(http.ListenAndServe(*pprofAddr, nil)) }()
}
