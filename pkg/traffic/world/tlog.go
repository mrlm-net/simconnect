package world

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// trafficLog records everything traffic control does — spawns, clearances,
// state and light changes, holds, errors — to the console, a log file and
// a buffer the map shows (GET /api/control/log).
type trafficLog struct {
	mu    sync.Mutex
	file  *os.File
	dir   string
	size  int64
	lines []string
}

var tlog = &trafficLog{}

// trafficLogMaxBytes: a log file this large is closed and a new one begun
// (#67: one file grew for as long as the World ran).
const trafficLogMaxBytes = 50 << 20

// openTrafficLog starts the log file in dir.
func openTrafficLog(dir string) {
	tlog.mu.Lock()
	defer tlog.mu.Unlock()
	tlog.dir = dir
	tlog.openLocked()
}

// openLocked starts a new log file, closing the one before (#67: left
// open). l.mu held.
func (l *trafficLog) openLocked() {
	name := fmt.Sprintf("%s/traffic-%s.log", l.dir, time.Now().Format("20060102-150405"))
	f, err := os.Create(name)
	if err != nil {
		fmt.Fprintf(stdout, "❌ traffic log: %v\n", err)
		return
	}
	if l.file != nil {
		l.file.Close()
	}
	l.file, l.size = f, 0
	fmt.Fprintf(stdout, "📝 Traffic log: %s\n", name)
}

// logLine keeps a line one line: what it quotes (call signs, names, a
// host's text) cannot start a forged line of its own (#67).
var logLine = strings.NewReplacer("\r\n", "⏎", "\n", "⏎", "\r", "⏎")

func (l *trafficLog) printf(format string, a ...any) {
	line := time.Now().Format("15:04:05.000") + "  " + logLine.Replace(fmt.Sprintf(format, a...))
	fmt.Fprintln(stdout, line)
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		if l.size > trafficLogMaxBytes && l.dir != "" {
			l.openLocked()
		}
		n, _ := fmt.Fprintln(l.file, line)
		l.size += int64(n)
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

// stdout is where the World's console lines go (Options.Output).
var stdout io.Writer = os.Stdout
