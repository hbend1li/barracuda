package app

import (
	"bufio"
	"encoding/gob"
	"flag"
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

// Whether one of the filter's regexes is matched on a line
func (f *Filter) match(line *string) string {
	for _, regex := range f.compiledRegex {

		if matches := regex.FindStringSubmatch(*line); matches != nil {

			match := matches[regex.SubexpIndex(f.patternName)]

			log.Printf("INFO  %s.%s: match [%v]\n", f.stream.name, f.name, match)
			return match
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

func sleep(d time.Duration) chan bool {
	c := make(chan bool)
	go func() {
		time.Sleep(d)
		c <- true
		close(c)
	}()
	return c
}

func (a *Action) exec(match string, advance time.Duration) {
	defer wgActions.Done()
	doExec := true

	// Wait for either end of sleep time, or actionStore requesting stop
	if a.afterDuration != 0 && a.afterDuration > advance {
		stopAction := actionStore.Register(a, match)
		select {
		case <-sleep(a.afterDuration - advance):
			// no-op
		case doExec = <-stopAction:
			// no-op
		}
		// Let's not wait for the lock
		go actionStore.Unregister(a, match, stopAction)
	}

	if doExec {
		computedCommand := make([]string, 0, len(a.Cmd))
		for _, item := range a.Cmd {
			computedCommand = append(computedCommand, strings.ReplaceAll(item, a.filter.patternWithBraces, match))
		}

		log.Printf("INFO  %s.%s.%s: run %s\n", a.filter.stream.name, a.filter.name, a.name, computedCommand)

		cmd := exec.Command(computedCommand[0], computedCommand[1:]...)

		if ret := cmd.Run(); ret != nil {
			log.Printf("ERROR %s.%s.%s: run %s, code %s\n", a.filter.stream.name, a.filter.name, a.name, computedCommand, ret)
		}
	}
}

func (f *Filter) cleanOldMatches(match string) {
	now := time.Now()
	newMatches := make([]time.Time, 0, len(f.matches[match]))
	for _, old := range f.matches[match] {
		if old.Add(f.retryDuration).After(now) {
			newMatches = append(newMatches, old)
		}
	}
	f.matches[match] = newMatches
}

func (f *Filter) handle() chan *string {
	lines := make(chan *string)

	go func() {
		for line := range lines {
			if match := f.match(line); match != "" {

				entry := LogEntry{time.Now(), match, f.stream.name, f.name, false}

				if f.Retry > 1 {
					f.cleanOldMatches(match)

					f.matches[match] = append(f.matches[match], time.Now())
				}

				if f.Retry <= 1 || len(f.matches[match]) >= f.Retry {
					entry.Exec = true
					delete(f.matches, match)
					f.execActions(match, time.Duration(0))
				}

				db.Encode(&entry)
			}
		}
	}()

	return lines
}

func (s *Stream) handle(endedSignal chan *Stream) {
	log.Printf("INFO  %s: start %s\n", s.name, s.Cmd)

	lines := cmdStdout(s.Cmd)

	var filterInputs = make([]chan *string, 0, len(s.Filters))
	for _, filter := range s.Filters {
		filterInputs = append(filterInputs, filter.handle())
	}
	defer func() {
		for _, filterInput := range filterInputs {
			close(filterInput)
		}
	}()

	for {
		select {
		case line, ok := <-lines:
			if !ok {
				endedSignal <- s
				return
			}
			for _, filterInput := range filterInputs {
				filterInput <- line
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

var db *gob.Encoder

func Main() {
	confFilename := flag.String("c", "", "configuration file. see an example at https://framagit.org/ppom/reaction/-/blob/main/reaction.yml")
	flag.Parse()

	if *confFilename == "" {
		flag.PrintDefaults()
		os.Exit(2)
	}

	actionStore.store = make(ActionMap)

	conf := parseConf(*confFilename)
	db = conf.updateFromDB()

	// Ready to start

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	stopStreams = make(chan bool)

	endSignals := make(chan *Stream)
	noStreamsInExecution := len(conf.Streams)

	for _, stream := range conf.Streams {
		go stream.handle(endSignals)
	}

	go Serve()

	for {
		select {
		case finishedStream := <-endSignals:
			log.Printf("ERROR %s stream finished", finishedStream.name)
			noStreamsInExecution--
			if noStreamsInExecution == 0 {
				quit()
			}
		case <-sigs:
			log.Printf("INFO  Received SIGINT/SIGTERM, exiting")
			quit()
		}
	}
}

func quit() {
	// stop all streams
	close(stopStreams)
	// stop all actions
	actionStore.Quit()
	// wait for them to complete
	wgActions.Wait()
	// delete pipe
	err := os.Remove(SocketPath)
	if err != nil {
		log.Println("Failed to remove socket:", err)
	}

	os.Exit(3)
}
