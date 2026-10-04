package world

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// trafficLog records everything traffic control does — spawns, clearances,
// state and light changes, holds, errors — to the console, a log file and
// a buffer the map shows (GET /api/control/log).
type trafficLog struct {
	mu    sync.Mutex
	file  *os.File
	lines []string
}

var tlog = &trafficLog{}

// openTrafficLog starts the log file in dir.
func openTrafficLog(dir string) {
	name := fmt.Sprintf("%s/traffic-%s.log", dir, time.Now().Format("20060102-150405"))
	f, err := os.Create(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ traffic log: %v\n", err)
		return
	}
	tlog.mu.Lock()
	tlog.file = f
	tlog.mu.Unlock()
	fmt.Printf("📝 Traffic log: %s\n", name)
}

func (l *trafficLog) printf(format string, a ...any) {
	line := time.Now().Format("15:04:05.000") + "  " + fmt.Sprintf(format, a...)
	fmt.Println(line)
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		fmt.Fprintln(l.file, line)
	}
	l.lines = append(l.lines, line)
	if len(l.lines) > 500 {
		l.lines = l.lines[len(l.lines)-500:]
	}
}

// recent returns the last n lines, newest last.
func (l *trafficLog) recent(n int) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if n > len(l.lines) {
		n = len(l.lines)
	}
	return append([]string(nil), l.lines[len(l.lines)-n:]...)
}
