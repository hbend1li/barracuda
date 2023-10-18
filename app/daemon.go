package app

import (
	"bufio"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"framagit.org/ppom/reaction/logger"
)

// Executes a command and channel-send its stdout
func cmdStdout(commandline []string) chan *string {
	lines := make(chan *string)

	go func() {
		cmd := exec.Command(commandline[0], commandline[1:]...)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			logger.Fatalln("couldn't open stdout on command:", err)
		}
		if err := cmd.Start(); err != nil {
			logger.Fatalln("couldn't start command:", err)
		}
		defer stdout.Close()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			lines <- &line
			logger.Println(logger.DEBUG, "stdout:", line)
		}
		close(lines)
	}()

	return lines
}

func runCommands(commands [][]string, moment string) {
	for _, command := range commands {
		cmd := exec.Command(command[0], command[1:]...)
		if err := cmd.Start(); err != nil {
			logger.Printf(logger.ERROR, "couldn't execute %v command: %v", moment, err)
		}
	}
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
				logger.Printf(logger.INFO, "%s.%s: match [%v]\n", f.stream.name, f.name, match)
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
	wgActions.Add(1)
	go func() {
		defer wgActions.Done()

		computedCommand := make([]string, 0, len(a.Cmd))
		for _, item := range a.Cmd {
			computedCommand = append(computedCommand, strings.ReplaceAll(item, a.filter.pattern.nameWithBraces, match))
		}

		logger.Printf(logger.INFO, "%s.%s.%s: run %s\n", a.filter.stream.name, a.filter.name, a.name, computedCommand)

		cmd := exec.Command(computedCommand[0], computedCommand[1:]...)

		if ret := cmd.Run(); ret != nil {
			logger.Printf(logger.ERROR, "%s.%s.%s: run %s, code %s\n", a.filter.stream.name, a.filter.name, a.name, computedCommand, ret)
		}
	}()
}

func ActionsManager() {
	pendingActionsC := make(chan PAT)
	for {
		select {
		case pat := <-actionsC:
			pa := PA{pat.p, pat.a}
			pattern, action, then := pat.p, pat.a, pat.t
			now := time.Now()
			// check if must be executed now
			if then.Compare(now) <= 0 {
				action.exec(pattern)
			} else {
				actionsLock.Lock()
				if actions[pa] == nil {
					actions[pa] = make(map[time.Time]struct{})
				}
				actions[PA{pattern, action}][then] = struct{}{}
				actionsLock.Unlock()
				go func(insidePat PAT, insideNow time.Time) {
					time.Sleep(insidePat.t.Sub(insideNow))
					pendingActionsC <- insidePat
				}(pat, now)
			}
		case pat := <-pendingActionsC:
			pa := PA{pat.p, pat.a}
			pattern, action, then := pat.p, pat.a, pat.t
			actionsLock.Lock()
			if actions[pa] != nil {
				if _, ok := actions[pa][then]; ok {
					delete(actions[pa], then)
					action.exec(pattern)
				}
			}
			actionsLock.Unlock()
			action.exec(pattern)
		case fo := <-flushToActionsC:
			ret := make(ActionsMap)
			actionsLock.Lock()
			for pa := range actions {
				if pa.p == fo.p {
					for range actions[pa] {
						pa.a.exec(pa.p)
					}
					ret[pa] = actions[pa]
					delete(actions, pa)
				}
			}
			actionsLock.Unlock()
			fo.ret <- ret
		case _, _ = <-stopActions:
			actionsLock.Lock()
			for pat := range actions {
				if pat.a.OnExit {
					pat.a.exec(pat.p)
				}
			}
			actionsLock.Unlock()
			wgActions.Done()
			return
		}
	}
}

func MatchesManager() {
	var fo FlushMatchOrder
	var pft PFT
	end := false

	for !end {
		select {
		case fo = <-flushToMatchesC:
			matchesManagerHandleFlush(fo)
		case fo, ok := <-startupMatchesC:
			if !ok {
				end = true
			} else {
				_ = matchesManagerHandleMatch(fo)
			}
		}
	}

	for {
		select {
		case fo = <-flushToMatchesC:
			matchesManagerHandleFlush(fo)
		case pft = <-matchesC:

			entry := LogEntry{pft.t, pft.p, pft.f.stream.name, pft.f.name, false}

			entry.Exec = matchesManagerHandleMatch(pft)

			logsC <- entry
		}
	}
}

func matchesManagerHandleFlush(fo FlushMatchOrder) {
	ret := make(MatchesMap)
	matchesLock.Lock()
	for pf := range matches {
		if fo.p == pf.p {
			if fo.ret != nil {
				ret[pf] = matches[pf]
			}
			delete(matches, pf)
		}
	}
	matchesLock.Unlock()
	if fo.ret != nil {
		fo.ret <- ret
	}
}

func matchesManagerHandleMatch(pft PFT) bool {
	matchesLock.Lock()
	defer matchesLock.Unlock()

	filter, pattern, then := pft.f, pft.p, pft.t
	pf := PF{pft.p, pft.f}

	if filter.Retry > 1 {
		// make sure map exists
		if matches[pf] == nil {
			matches[pf] = make(map[time.Time]struct{})
		}
		// add new match
		matches[pf][then] = struct{}{}
		// remove match when expired
		go func(pf PF, then time.Time) {
			time.Sleep(then.Sub(time.Now()) + filter.retryDuration)
			matchesLock.Lock()
			if matches[pf] != nil {
				// FIXME replace this and all similar occurences
				// by clear() when switching to go 1.21
				delete(matches[pf], then)
			}
			matchesLock.Unlock()
		}(pf, then)
	}

	if filter.Retry <= 1 || len(matches[pf]) >= filter.Retry {
		delete(matches, pf)
		filter.sendActions(pattern, then)
		return true
	}
	return false
}

func StreamManager(s *Stream, endedSignal chan *Stream) {
	defer wgStreams.Done()
	logger.Printf(logger.INFO, "%s: start %s\n", s.name, s.Cmd)

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

/*
<StreamCmds>
 ↓
StreamManager          onstartup:matches
 ↓                     ↓               ↑
 matches→ MatchesManager →logs→ DatabaseManager ←·
                 ↑     ↓                         ↑
                 ↑     actions→ ActionsManager   ↑
                 ↑              ↑                ↑
SocketManager →flushes→→→→→→→→→→·→→→→→→→→→→→→→→→→·
 ↑
<Clients>
*/

// DatabaseManager → MatchesManager
var startupMatchesC chan PFT

// StreamManager → MatchesManager
var matchesC chan PFT

// MatchesManager → DatabaseManager
var logsC chan LogEntry

// MatchesManager → ActionsManager
var actionsC chan PAT

// SocketManager, DatabaseManager → MatchesManager
var flushToMatchesC chan FlushMatchOrder

// SocketManager → ActionsManager
var flushToActionsC chan FlushActionOrder

// SocketManager → DatabaseManager
var flushToDatabaseC chan LogEntry

func Daemon(confFilename string) {
	conf := parseConf(confFilename)

	startupMatchesC = make(chan PFT)
	matchesC = make(chan PFT)
	logsC = make(chan LogEntry)
	actionsC = make(chan PAT)
	flushToMatchesC = make(chan FlushMatchOrder)
	flushToActionsC = make(chan FlushActionOrder)
	flushToDatabaseC = make(chan LogEntry)
	stopActions = make(chan bool)
	stopStreams = make(chan bool)
	actions = make(ActionsMap)
	matches = make(MatchesMap)

	runCommands(conf.Start, "start")

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

	go SocketManager(conf.Streams)

	for {
		select {
		case finishedStream := <-endSignals:
			logger.Printf(logger.ERROR, "%s stream finished", finishedStream.name)
			nbStreamsInExecution--
			if nbStreamsInExecution == 0 {
				quit(conf)
			}
		case <-sigs:
			logger.Printf(logger.INFO, "Received SIGINT/SIGTERM, exiting")
			quit(conf)
		}
	}
}

func quit(conf *Conf) {
	// send stop to StreamManager·s
	close(stopStreams)
	logger.Println(logger.INFO, "Waiting for Streams to finish...")
	wgStreams.Wait()
	// ActionsManager calls wgActions.Done() when it has launched all pending actions
	wgActions.Add(1)
	// send stop to ActionsManager
	close(stopActions)
	// stop all actions
	logger.Println(logger.INFO, "Waiting for Actions to finish...")
	wgActions.Wait()
	// run stop commands
	runCommands(conf.Stop, "stop")
	// delete pipe
	err := os.Remove(*SocketPath)
	if err != nil {
		logger.Println(logger.ERROR, "Failed to remove socket:", err)
	}

	os.Exit(3)
}
