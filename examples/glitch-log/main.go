// Command glitch-log follows the airport map's live traffic log like
// tail -f and prints a line for each suspected glitch: unanswered pilot
// calls, repeated ATC lines, type codes said raw, traffic information in
// the final stage, problems, TCAS, go-arounds, stopped aircraft and wrong
// readbacks. A summary per kind follows every 10 minutes and on exit, so
// nobody has to watch the radio. Usage:
// glitch-log [-from start|end] [-once] [-no-KIND ...] [log file].
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const defaultLog = "airport-map.log" // the map's log, run with its output redirected here

var kinds = []string{"unanswered", "repeated", "typecode", "finalstage", "problems", "tcas", "goaround", "stopped", "readback"}

var (
	from      = flag.String("from", "end", `where to start: "start" or "end" of the log`)
	once      = flag.Bool("once", false, "stop at the end of the log instead of following it")
	answerIn  = flag.Duration("answer", 20*time.Second, "time ATC has to answer a pilot call")
	repeatIn  = flag.Duration("repeat", 60*time.Second, "window for the same ATC text twice")
	correctIn = flag.Duration("correct", 10*time.Second, "time ATC has to correct a wrong readback")
	off       = map[string]*bool{}
)

// line is one parsed log line. Lines without a time or a tail keep only
// the raw text.
type line struct {
	raw  string
	at   time.Duration // since midnight, plus a day per wrap
	tail string
	who  string
	text string
}

// call is a pilot call or a wrong readback waiting for ATC.
type call struct {
	at      time.Duration
	lines   []string
	station string        // Tower, Ground, ... or "" when unknown
	grace   time.Duration // extra time on top of -answer
}

// answer is when the controller of a station answered a call made at
// callAt. Calls queue per station: a call's clock starts when the call
// before it was answered.
type answer struct{ callAt, at time.Duration }

type aircraft struct {
	lastATC  *line
	readBack bool          // the pilot already answered lastATC
	sayAgain time.Duration // last pilot "say again", -1 if none
	calls    []call
	wrong    *call // readback that differs from the ATC line
	said     map[string]time.Duration
	state    string // latest arrival/departure state
	station  string // the station the tail talks to
	quiet    map[string]time.Duration
}

var (
	reLine    = regexp.MustCompile(`^(\d\d):(\d\d):(\d\d)\.(\d{3})  (.*)$`)
	reTail    = regexp.MustCompile(`^([A-Z][A-Z0-9]{1,7}|[A-Z][a-z]+ \d+) +([A-Za-z][^:]{0,40}): (.*)$`)
	reState   = regexp.MustCompile(`→ ([a-z ]+?)(?: of)?  \(`)
	reFreq    = regexp.MustCompile(`\b1[1-3]\d\.\d{1,3}\b`)
	reRunway  = regexp.MustCompile(`(?i)\brunway (\d\d[LRC]?)\b`)
	reStation = regexp.MustCompile(`^(?:(?:Good \w+|Hello), )?(?:[A-Z][a-z]+ )?(?:Tower|Ground|Delivery|Radar|Approach|Director|Center|Centre|Control|Information|Apron)(?:, |$)`)
	// Pilot words that need an answer, and some that never do.
	reExpects     = regexp.MustCompile(`(?i)\brequest\b|ready for departure|holding short|established|vacated|TCAS RA|clear of conflict|going around`)
	reStationWord = regexp.MustCompile(`\b(Tower|Ground|Delivery|Radar|Approach|Director|Center|Centre|Control|Information|Apron)\b`)
	reSlow        = regexp.MustCompile(`(?i)request (clearance|pushback|start up|weather)|holding short of runway \w+ at`)
	reDigits      = regexp.MustCompile(`\d+`)
	reNoReply     = regexp.MustCompile(`(?i)^(wilco|looking out|looking|say again|roger)\b`)
	// Raw ICAO type designators.
	reType1  = regexp.MustCompile(`^[A-Z][0-9][0-9A-Z]{1,2}$`)
	reType2  = regexp.MustCompile(`^(B7[0-9]{2}|A3[0-9]{2}|B3[0-9]M|DH8[A-D]|AT[47][0-9]|CRJ[0-9]|E1[0-9]{2}|C[0-9]{3}|PC12|BE[0-9]{2}|SR2[0-9]|DA[0-9]{2})$`)
	reType3  = regexp.MustCompile(`^[A-Z][A-Z0-9]{2,3}$`) // in a type slot only: BCS3, C25C
	reMaker  = regexp.MustCompile(`^(Airbus|Boeing|Embraer|Cessna|Cirrus|Diamond|Bombardier|Dash|Pilatus|Beechcraft|Piper|ATR|Saab|Fokker|Tecnam|Learjet|Dassault|Gulfstream)$`)
	reModel  = regexp.MustCompile(`^[A-Z]{0,2}\d{2,3}$`) // A320, SR22 after a maker
	reSlot   = regexp.MustCompile(`(?i)(?:follow the|behind the (?:landing|departing)|give way to the) ([^,]+)`)
	reTraffI = regexp.MustCompile(`(?i)\btraffic, .*o'clock`)
	reFinal  = map[string]bool{"landing": true, "rollout": true, "lining up": true, "lined up": true, "departing": true}
	problems = []string{"⚠️", "failed", "cancelled —", "exception", "refused", "not given", "level=ERROR"}
)

var (
	planes   = map[string]*aircraft{}
	counts   = map[string]int{}
	lastAt   time.Duration
	dayShift time.Duration
	probs    *call // tail-less problem lines in a row
	answers  = map[string][]answer{}
)

func main() {
	for _, k := range kinds {
		off[k] = flag.Bool("no-"+k, false, "do not report "+k)
	}
	flag.Parse()
	path := defaultLog
	if flag.NArg() > 0 {
		path = flag.Arg(0)
	}
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()
	if *from == "end" {
		f.Seek(0, io.SeekEnd)
	}

	lines := make(chan string, 256)
	done := make(chan struct{})
	go follow(f, path, lines, done)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	summary := time.NewTicker(10 * time.Minute)
	tick := time.NewTicker(time.Second)
	for {
		select {
		case s := <-lines:
			handle(s)
		case <-tick.C:
			flushProblems()
			// Live: let calls expire on the wall clock when the log is quiet.
			if !*once && lastAt > 0 {
				now := time.Now()
				t := time.Duration(now.Hour())*time.Hour + time.Duration(now.Minute())*time.Minute + time.Duration(now.Second())*time.Second + dayShift
				if t > lastAt && t-lastAt < time.Hour {
					expire(t)
				}
			}
		case <-summary.C:
			printSummary()
		case <-done:
			for len(lines) > 0 {
				handle(<-lines)
			}
			printSummary()
			return
		case <-stop:
			printSummary()
			return
		}
	}
}

// follow reads the log line by line and waits for more at the end, like
// tail -f. A shorter file means the map started a new log: read it again.
func follow(f *os.File, path string, out chan<- string, done chan<- struct{}) {
	r := bufio.NewReader(f)
	var part string
	for {
		s, err := r.ReadString('\n')
		part += s
		if err == nil {
			out <- strings.TrimRight(part, "\r\n")
			part = ""
			continue
		}
		if *once {
			if part != "" {
				out <- strings.TrimRight(part, "\r\n")
			}
			close(done)
			return
		}
		time.Sleep(500 * time.Millisecond)
		pos, _ := f.Seek(0, io.SeekCurrent)
		if st, err := os.Stat(path); err == nil && st.Size() < pos {
			if nf, err := os.Open(path); err == nil {
				f.Close()
				f, part = nf, ""
				r = bufio.NewReader(f)
			}
		}
	}
}

func parse(s string) line {
	l := line{raw: strings.TrimPrefix(s, "\ufeff"), at: -1}
	m := reLine.FindStringSubmatch(l.raw)
	if m == nil {
		return l
	}
	// Atoi, not Sscan: Sscan reads "09" as a bad octal number.
	h, _ := strconv.Atoi(m[1])
	mi, _ := strconv.Atoi(m[2])
	se, _ := strconv.Atoi(m[3])
	ms, _ := strconv.Atoi(m[4])
	l.at = time.Duration(h)*time.Hour + time.Duration(mi)*time.Minute + time.Duration(se)*time.Second + time.Duration(ms)*time.Millisecond + dayShift
	if l.at < lastAt-12*time.Hour { // past midnight
		dayShift += 24 * time.Hour
		l.at += 24 * time.Hour
	}
	if t := reTail.FindStringSubmatch(m[5]); t != nil {
		l.tail, l.who, l.text = t[1], t[2], t[3]
	}
	return l
}

func plane(tail string) *aircraft {
	a := planes[tail]
	if a == nil {
		a = &aircraft{sayAgain: -1, said: map[string]time.Duration{}, quiet: map[string]time.Duration{}}
		planes[tail] = a
	}
	return a
}

func handle(s string) {
	l := parse(s)
	if l.at >= 0 {
		expire(l.at)
		lastAt = l.at
	}
	problem(l)
	if l.tail == "" {
		return
	}
	a := plane(l.tail)
	low := strings.ToLower(l.text)
	switch {
	case l.who == "ATC":
		atc(a, l, low)
	case l.who == "pilot":
		pilot(a, l, low)
	case l.who == "TCAS RA" || l.who == "TCAS TA":
		// An RA repeats while it lasts: one report per tail and 30 s.
		if a.once(l.who, l.at, 30*time.Second) {
			report(l.at, "tcas", l.tail, l.who, l.raw)
		}
	case strings.HasPrefix(l.who, "tower ") && strings.HasPrefix(l.text, "waits"):
		// Tuned: the tower holds a ready aircraft silently while the
		// runway is busy ("waits — CSA761 on the runway"), after telling
		// it the order. Deliberate, so the call is not unanswered.
		a.calls = nil
	case l.who == "arrival" || l.who == "departure":
		if m := reState.FindStringSubmatch(l.text); m != nil {
			a.state = m[1]
			if m[1] == "complete" || m[1] == "cancelled" || m[1] == "failed" || m[1] == "parked" {
				a.calls = nil // gone or done: nobody answers any more
			}
		}
		if strings.HasPrefix(l.text, "stopped 20s") {
			report(l.at, "stopped", l.tail, l.text, l.raw)
		}
	}
	if strings.Contains(low, "go around") || strings.Contains(low, "going around") {
		// The crew line, the pilot call and ATC all say it: one per minute.
		if a.once("goaround", l.at, time.Minute) {
			report(l.at, "goaround", l.tail, l.who, l.raw)
		}
	}
	if l.who == "schedule" && strings.HasPrefix(l.text, "removed") {
		delete(planes, l.tail)
	}
}

// problem reports problem lines. One SimConnect exception comes as three
// lines (slog, the ⚠️ line, the timed one), so tail-less problems in a row
// make one report.
func problem(l line) {
	what := ""
	for _, p := range problems {
		if strings.Contains(l.raw, p) {
			what = p
			break
		}
	}
	if what != "" && l.tail == "" {
		if probs == nil {
			at := l.at
			if at < 0 {
				at = lastAt
			}
			probs = &call{at: at, lines: []string{what}}
		}
		probs.lines = append(probs.lines, l.raw)
		return
	}
	flushProblems()
	// The same problem of one aircraft every few seconds ("level refused"
	// 35 times in 3 minutes) is one report per 5 minutes.
	if what != "" && plane(l.tail).once("problem "+reDigits.ReplaceAllString(l.text, "N"), l.at, 5*time.Minute) {
		report(l.at, "problems", l.tail, what, l.raw)
	}
}

func flushProblems() {
	if probs != nil {
		report(probs.at, "problems", "", probs.lines[0], probs.lines[1:]...)
		probs = nil
	}
}

// once is true when kind was not reported for this aircraft within d.
func (a *aircraft) once(kind string, at, d time.Duration) bool {
	if t, ok := a.quiet[kind]; ok && at-t < d {
		return false
	}
	a.quiet[kind] = at
	return true
}

func atc(a *aircraft, l line, low string) {
	// Any ATC transmission answers the open calls.
	for _, c := range a.calls {
		answers[c.station] = append(answers[c.station], answer{c.at, l.at})
	}
	a.calls = nil
	if strings.Contains(low, "contact") {
		if s := reStationWord.FindString(l.text); s != "" {
			a.station = s
		}
	}
	if a.wrong != nil {
		// Corrected: deliberate crew variety. Anything else from ATC
		// means no correction is coming.
		if !strings.Contains(low, "negative") {
			report(a.wrong.at, "readback", l.tail, a.wrong.lines[0], a.wrong.lines[1:]...)
		}
		a.wrong = nil
	}
	// Repeats: "negative" corrections and answers to "say again" are meant.
	if t, ok := a.said[l.text]; ok && l.at-t <= *repeatIn && !strings.Contains(low, "negative") &&
		!(a.sayAgain >= 0 && l.at-a.sayAgain <= *answerIn) &&
		a.once("repeated "+l.text, l.at, *repeatIn) { // a burst (one every 2 s) is one report
		report(l.at, "repeated", l.tail, fmt.Sprintf("again after %s", fmtDur(l.at-t)), l.raw)
	}
	a.said[l.text] = l.at
	for k, t := range a.said {
		if l.at-t > *repeatIn {
			delete(a.said, k)
		}
	}
	if raw := typeCodes(l.text); raw != "" {
		report(l.at, "typecode", l.tail, raw, l.raw)
	}
	if reTraffI.MatchString(l.text) && reFinal[a.state] {
		report(l.at, "finalstage", l.tail, "traffic info while "+a.state, l.raw)
	}
	a.lastATC, a.readBack = &l, false
}

func pilot(a *aircraft, l line, low string) {
	if strings.HasPrefix(low, "say again") {
		a.sayAgain = l.at
	}
	// The first pilot line within 2 s of ATC to the same tail is its
	// readback. Tuned: after "contact Tower" the map logs the readback and
	// the initial call to Tower in the same millisecond, so only the first
	// line counts as the readback.
	if a.lastATC != nil && !a.readBack && l.at-a.lastATC.at <= 2*time.Second {
		a.readBack = true
		if d := differs(a.lastATC.text, l.text); d != "" {
			a.wrong = &call{at: l.at, lines: []string{d, a.lastATC.raw, l.raw}}
		}
		return
	}
	if reNoReply.MatchString(l.text) {
		return
	}
	initial := reStation.MatchString(l.text)
	if s := reStationWord.FindString(l.text); initial && s != "" {
		a.station = s
	}
	if strings.HasPrefix(l.tail, "Tug ") || strings.HasPrefix(l.tail, "Fuel ") {
		a.station = "Ground"
	}
	if initial || reExpects.MatchString(l.text) {
		c := call{at: l.at, lines: []string{l.raw}, station: a.station}
		// Tuned: Delivery and Ground take 20-27 s for the first clearance
		// or pushback of a slot (14:30:00, 16:40:00 in the logs) before any
		// queue builds; weather requests and runway crossings ("holding
		// short of runway 12 at F") take 20-29 s too. These get twice the time.
		if reSlow.MatchString(l.text) {
			c.grace = *answerIn
		}
		// Two calls in one breath ("Ruzyne Radar, ... passing 3000 feet"
		// then "request direct DONAD") wait as one.
		if n := len(a.calls); n > 0 && l.at-a.calls[n-1].at <= 2*time.Second {
			a.calls[n-1].lines = append(a.calls[n-1].lines, l.raw)
			return
		}
		a.calls = append(a.calls, c)
	}
}

// differs names a frequency or runway in the readback that is not the one
// ATC said. Only values present on both sides count.
func differs(atc, rb string) string {
	for _, re := range []*regexp.Regexp{reFreq, reRunway} {
		a, b := re.FindAllString(atc, -1), re.FindAllString(rb, -1)
		if len(a) == 0 || len(b) == 0 {
			continue
		}
		for _, x := range b {
			if !contains(a, x) {
				return fmt.Sprintf("read back %q, ATC said %q", x, strings.Join(a, ", "))
			}
		}
	}
	return ""
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if strings.EqualFold(y, x) {
			return true
		}
	}
	return false
}

// typeCodes finds ICAO type designators said raw in the places ATC names
// an aircraft type: "follow the", "behind the landing", "give way to the"
// and the fields of traffic information. Runways, taxiways, stands,
// levels and frequencies sit elsewhere, so they never get here.
func typeCodes(text string) string {
	var slots []string
	for _, m := range reSlot.FindAllStringSubmatch(text, -1) {
		slots = append(slots, m[1])
	}
	if reTraffI.MatchString(text) {
		slots = append(slots, strings.Split(text, ", ")...)
	}
	var found []string
	for _, s := range slots {
		words := strings.Fields(s)
		for i, w := range words {
			w = strings.Trim(w, ".,;")
			if !(reType1.MatchString(w) || reType2.MatchString(w) || (reType3.MatchString(w) && strings.ContainsAny(w, "0123456789"))) {
				continue
			}
			// "Airbus A320" and "Cirrus SR22" are how it should sound;
			// "Airbus A20N" is still raw.
			if i > 0 && reMaker.MatchString(words[i-1]) && reModel.MatchString(w) {
				continue
			}
			if !contains(found, w) {
				found = append(found, w)
			}
		}
	}
	return strings.Join(found, ", ")
}

// expire reports calls nobody answered and readbacks nobody corrected.
func expire(now time.Duration) {
	for s, as := range answers {
		for len(as) > 0 && now-as[0].at > 10*time.Minute {
			as = as[1:]
		}
		answers[s] = as
	}
	for tail, a := range planes {
		for len(a.calls) > 0 {
			c := a.calls[0]
			start, ahead := queued(c, tail)
			if ahead || now-start <= *answerIn+c.grace {
				break
			}
			a.calls = a.calls[1:]
			detail := fmt.Sprintf("no ATC within %s", fmtDur(*answerIn+c.grace))
			if c.station != "" {
				detail = c.station + ": " + detail
			}
			report(c.at, "unanswered", tail, detail, c.lines...)
		}
		if a.wrong != nil && now-a.wrong.at > *correctIn {
			w := a.wrong
			a.wrong = nil
			report(w.at, "readback", tail, w.lines[0], w.lines[1:]...)
		}
	}
}

// queued gives when the clock of call c starts: when it was made, or later
// when the controller answered an earlier call on the same station (one
// transmission at a time: four pushback calls at 16:40:00 got answers at
// 27, 39, 53 and 67 s). ahead is true while an earlier call on the station
// still waits: it is reported first.
func queued(c call, tail string) (start time.Duration, ahead bool) {
	start = c.at
	if c.station == "" {
		return start, false
	}
	for _, an := range answers[c.station] {
		if an.callAt <= c.at && an.at > start {
			start = an.at
		}
	}
	for t, a := range planes {
		for _, o := range a.calls {
			if t != tail && o.station == c.station && o.at < c.at {
				return start, true
			}
		}
	}
	return start, false
}

func report(at time.Duration, kind, tail, detail string, lines ...string) {
	if *off[kind] {
		return
	}
	counts[kind]++
	if tail == "" {
		tail = "-"
	}
	fmt.Printf("%s  %-10s  %-7s  %s\n", clock(at), kind, tail, detail)
	for _, l := range lines {
		fmt.Printf("    > %s\n", l)
	}
}

func printSummary() {
	flushProblems()
	var parts []string
	for _, k := range kinds {
		if !*off[k] {
			parts = append(parts, fmt.Sprintf("%s %d", k, counts[k]))
		}
	}
	fmt.Printf("%s  summary     %s\n", clock(lastAt), strings.Join(parts, ", "))
}

func clock(d time.Duration) string {
	if d < 0 {
		return "--:--:--"
	}
	d %= 24 * time.Hour
	return fmt.Sprintf("%02d:%02d:%02d", int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60)
}

func fmtDur(d time.Duration) string { return d.Round(time.Second).String() }
