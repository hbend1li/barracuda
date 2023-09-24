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

func (f *Filter) sendActions(match string, at time.Time) {
	for _, a := range f.Actions {
		actionsC <- PAT{match, a, at.Add(a.afterDuration)}
	}
}

func (a *Action) exec(match string) {
	defer wgActions.Done()

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

func ActionsManager() {
	pendingActionsC := make(chan PAT)
	var (
		pat    PAT
		action *Action
		match  string
		then   time.Time
		now    time.Time
	)
	for {
		select {
		case pat = <-actionsC:
			match = pat.p
			action = pat.a
			then = pat.t
			now = time.Now()
			// check
			if then.Compare(now) <= 0 {
				wgActions.Add(1)
				go action.exec(match)
			} else {
				actionsLock.Lock()
				// make sure map exists
				if actions[action] == nil {
					actions[action] = make(PatternTimes)
				}
				// append() to nil is valid go
				actions[action][match] = append(actions[action][match], then)
				actionsLock.Unlock()
				go func(pat PAT, now time.Time) {
					time.Sleep(pat.t.Sub(now))
					pendingActionsC <- pat
				}(pat, now)
			}
		case pat = <-pendingActionsC:
			match = pat.p
			action = pat.a
			actionsLock.Lock()
			actions[action][match] = actions[action][match][1:]
			actionsLock.Unlock()
			wgActions.Add(1)
			go action.exec(match)
		case _, _ = <-stopActions:
			actionsLock.Lock()
			for action := range actions {
				if action.OnExit {
					for match := range actions[action] {
						wgActions.Add(1)
						go action.exec(match)
					}
				}
			}
			actionsLock.Unlock()
			wgActions.Done()
			return
		}
	}
}

func MatchesManager() {
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
				_ = matchesManagerHandleMatch(pft)
			}
		}
	}

	for {
		select {
		case pf = <-cleanMatchesC:
			matchesLock.Lock()
			delete(matches[pf.f], pf.p)
			matchesLock.Unlock()
		case pft = <-matchesC:

			entry := LogEntry{pft.t, pft.p, pft.f.stream.name, pft.f.name, false}

			matchesLock.Lock()
			entry.Exec = matchesManagerHandleMatch(pft)
			matchesLock.Unlock()

			logsC <- entry
		}
	}
}

func matchesManagerHandleMatch(pft PFT) bool {
	filter, match, then := pft.f, pft.p, pft.t

	if filter.Retry > 1 {
		// make sure map exists
		if matches[filter] == nil {
			matches[filter] = make(PatternTimes)
		}
		// clean old matches
		newMatches := make([]time.Time, 0, len(matches[filter][match]))
		for _, old := range matches[filter][match] {
			if old.Add(filter.retryDuration).After(then) {
				newMatches = append(newMatches, old)
			}
		}
		// add new match
		newMatches = append(newMatches, then)
		matches[filter][match] = newMatches
	}

	if filter.Retry <= 1 || len(matches[filter][match]) >= filter.Retry {
		delete(matches[filter], match)
		filter.sendActions(match, then)
		return true
	}
	return false
}

func StreamManager(s *Stream, endedSignal chan *Stream) {
	defer wgStreams.Done()
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
		case _, _ = <-stopStreams:
			return
		}
	}

}

var actions ActionsMap
var matches MatchesMap
var actionsLock sync.Mutex
var matchesLock sync.Mutex

var stopStreams chan bool
var stopActions chan bool
var wgActions sync.WaitGroup
var wgStreams sync.WaitGroup

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
var actionsC chan PAT

func Daemon(confFilename string) {
	conf := parseConf(confFilename)

	logsC = make(chan LogEntry)
	flushesC = make(chan LogEntry)
	matchesC = make(chan PFT)
	startupMatchesC = make(chan PFT)
	cleanMatchesC = make(chan PF)
	actionsC = make(chan PAT)
	stopActions = make(chan bool)
	stopStreams = make(chan bool)
	actions = make(ActionsMap)
	matches = make(MatchesMap)

	go DatabaseManager(conf)
	go MatchesManager()
	go ActionsManager()

	// Ready to start

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	endSignals := make(chan *Stream)
	nbStreamsInExecution := len(conf.Streams)

	for _, stream := range conf.Streams {
		wgStreams.Add(1)
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
	// send stop to StreamManager·s
	close(stopStreams)
	log.Println("INFO  Waiting for Streams to finish...")
	wgStreams.Wait()
	// ActionsManager calls wgActions.Done() when it has launched all pending actions
	wgActions.Add(1)
	// send stop to ActionsManager
	close(stopActions)
	// stop all actions
	log.Println("INFO  Waiting for Actions to finish...")
	wgActions.Wait()
	// delete pipe
	err := os.Remove(*SocketPath)
	if err != nil {
		log.Println("Failed to remove socket:", err)
	}

	os.Exit(3)
}
