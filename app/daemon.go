package app

import (
	"bufio"
	"syscall"

	// "fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"time"
)

// Executes a command and channel-send its stdout
func cmdStdout(commandline []string) chan *string {
	lines := make(chan *string)

	go func() {
		cmd := exec.Command(commandline[0], commandline[1:]...)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			log.Fatalln("FATAL couldn't open stdout on command:", err)
		}
		if err := cmd.Start(); err != nil {
			log.Fatalln("FATAL couldn't start command:", err)
		}
		defer stdout.Close()

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			lines <- &line
		}
		close(lines)
	}()

	return lines
}

func (p *Pattern) notAnIgnore(match *string) bool {
	for _, ignore := range p.Ignore {
		if ignore == *match {
			return false
		}
	}
	return true
}

// Whether one of the filter's regexes is matched on a line
func (f *Filter) match(line *string) string {
	for _, regex := range f.compiledRegex {

		if matches := regex.FindStringSubmatch(*line); matches != nil {

			match := matches[regex.SubexpIndex(f.pattern.name)]

			if f.pattern.notAnIgnore(&match) {
				log.Printf("INFO  %s.%s: match [%v]\n", f.stream.name, f.name, match)
				return match
			}
		}
	}
	return ""
}

func (f *Filter) execActions(match string, advance time.Duration) {
	for _, a := range f.Actions {
		wgActions.Add(1)
		go a.exec(match, advance)
	}
}

func (a *Action) exec(match string, advance time.Duration) {
	defer wgActions.Done()

	// Wait for either end of sleep time, or actionStore requesting stop
	if a.afterDuration != 0 && a.afterDuration > advance {
		stopAction := actionStore.Register(a, match)
		select {
		case <-time.After(a.afterDuration - advance):
			// Let's not wait for the lock
			go actionStore.Unregister(a, match, stopAction)
		case doExec := <-stopAction:
			// no need to unregister here
			if !doExec {
				return
			}
		}
	}

	computedCommand := make([]string, 0, len(a.Cmd))
	for _, item := range a.Cmd {
		computedCommand = append(computedCommand, strings.ReplaceAll(item, a.filter.pattern.nameWithBraces, match))
	}

	log.Printf("INFO  %s.%s.%s: run %s\n", a.filter.stream.name, a.filter.name, a.name, computedCommand)

	cmd := exec.Command(computedCommand[0], computedCommand[1:]...)

	if ret := cmd.Run(); ret != nil {
		log.Printf("ERROR %s.%s.%s: run %s, code %s\n", a.filter.stream.name, a.filter.name, a.name, computedCommand, ret)
	}
}

func MatchesManager() {
	matches := make(MatchesMap)
	var pf PF
	var pft PFT
	end := false

	for !end {
		select {
		case pf = <-cleanMatchesC:
			delete(matches[pf.f], pf.p)
		case pft, ok := <-startupMatchesC:
			if !ok {
				end = true
			} else {
				_ = matchesManagerHandleMatch(matches, pft)
			}
		}
	}

	for {
		select {
		case pf = <-cleanMatchesC:
			delete(matches[pf.f], pf.p)
		case pft = <-matchesC:

			entry := LogEntry{pft.t, pft.p, pft.f.stream.name, pft.f.name, false}

			entry.Exec = matchesManagerHandleMatch(matches, pft)

			logsC <- entry
		}
	}
}

func matchesManagerHandleMatch(matches MatchesMap, pft PFT) bool {
	var filter *Filter
	var match string
	var now time.Time
	filter = pft.f
	match = pft.p
	now = pft.t

	if filter.Retry > 1 {
		// make sure map exists
		if matches[filter] == nil {
			matches[filter] = make(map[string][]time.Time)
		}
		// clean old matches
		newMatches := make([]time.Time, 0, len(matches[filter][match]))
		for _, old := range matches[filter][match] {
			if old.Add(filter.retryDuration).After(now) {
				newMatches = append(newMatches, old)
			}
		}
		// add new match
		newMatches = append(newMatches, now)
		matches[filter][match] = newMatches
	}

	if filter.Retry <= 1 || len(matches[filter][match]) >= filter.Retry {
		delete(matches[filter], match)
		filter.execActions(match, time.Duration(0))
		return true
	}
	return false
}

func StreamManager(s *Stream, endedSignal chan *Stream) {
	log.Printf("INFO  %s: start %s\n", s.name, s.Cmd)

	lines := cmdStdout(s.Cmd)
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				endedSignal <- s
				return
			}
			for _, filter := range s.Filters {
				if match := filter.match(line); match != "" {
					matchesC <- PFT{match, filter, time.Now()}
				}
			}
		case _, ok := <-stopStreams:
			if !ok {
				return
			}
		}
	}

}

var stopStreams chan bool
var actionStore ActionStore
var wgActions sync.WaitGroup

// MatchesManager → DatabaseManager
var logsC chan LogEntry
// SocketManager → DatabaseManager
var flushesC chan LogEntry

// DatabaseManager → MatchesManager
var startupMatchesC chan PFT
// StreamManager → MatchesManager
var matchesC chan PFT
// StreamManager, DatabaseManager → MatchesManager
var cleanMatchesC chan PF
// MatchesManager → ExecsManager
var execsC chan PA

func Daemon(confFilename string) {
	actionStore.store = make(ActionMap)

	conf := parseConf(confFilename)

	logsC = make(chan LogEntry)
	flushesC = make(chan LogEntry)
	matchesC = make(chan PFT)
	startupMatchesC = make(chan PFT)
	cleanMatchesC = make(chan PF)
	execsC = make(chan PA)

	go DatabaseManager(conf)
	go MatchesManager()

	// Ready to start

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	stopStreams = make(chan bool)

	endSignals := make(chan *Stream)
	nbStreamsInExecution := len(conf.Streams)

	for _, stream := range conf.Streams {
		go StreamManager(stream, endSignals)
	}

	go SocketManager()

	for {
		select {
		case finishedStream := <-endSignals:
			log.Printf("ERROR %s stream finished", finishedStream.name)
			nbStreamsInExecution--
			if nbStreamsInExecution == 0 {
				quit()
			}
		case <-sigs:
			log.Printf("INFO  Received SIGINT/SIGTERM, exiting")
			quit()
		}
	}
}

func quit() {
	log.Println("INFO  Quitting...")
	// stop all streams
	close(stopStreams)
	// stop all actions
	actionStore.Quit()
	// wait for them to complete
	wgActions.Wait()
	// delete pipe
	err := os.Remove(*SocketPath)
	if err != nil {
		log.Println("Failed to remove socket:", err)
	}

	os.Exit(3)
}
