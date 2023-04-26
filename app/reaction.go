package app

import (
	"bufio"
	"encoding/gob"
	"flag"

	// "fmt"
	"log"
	"os"
	"os/exec"
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
			log.Fatal("couldn't open stdout on command:", err)
		}
		if err := cmd.Start(); err != nil {
			log.Fatal("couldn't start command:", err)
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

			log.Printf("INFO %s.%s: match [%v]\n", f.stream.name, f.name, match)
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

func (a *Action) exec(match string, advance time.Duration) {
	defer wgActions.Done()
	if a.afterDuration != 0 && a.afterDuration > advance {
		time.Sleep(a.afterDuration - advance)
	}

	computedCommand := make([]string, 0, len(a.Cmd))
	for _, item := range a.Cmd {
		computedCommand = append(computedCommand, strings.ReplaceAll(item, a.filter.patternWithBraces, match))
	}

	log.Printf("INFO %s.%s.%s: run %s\n", a.filter.stream.name, a.filter.name, a.name, computedCommand)

	cmd := exec.Command(computedCommand[0], computedCommand[1:]...)

	if ret := cmd.Run(); ret != nil {
		log.Printf("ERR  %s.%s.%s: run %s, code %s\n", a.filter.stream.name, a.filter.name, a.name, computedCommand, ret)
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

				f.cleanOldMatches(match)

				f.matches[match] = append(f.matches[match], time.Now())

				if len(f.matches[match]) >= f.Retry {
					entry.exec = true
					delete(f.matches, match)
					f.execActions(match, nil)
				}

				db.Encode(&entry)
			}
		}
	}()

	return lines
}

func multiplex(input chan *string, outputs []chan *string) {
	var wg sync.WaitGroup
	for item := range input {
		for _, output := range outputs {
			wg.Add(1)
			go func(s *string) {
				output <- s
				wg.Done()
			}(item)
		}
	}
	for _, output := range outputs {
		wg.Wait()
		close(output)
	}
}

func (s *Stream) handle(signal chan *Stream) {
	log.Printf("INFO %s: start %s\n", s.name, s.Cmd)

	lines := cmdStdout(s.Cmd)

	var filterInputs = make([]chan *string, 0, len(s.Filters))
	for _, filter := range s.Filters {
		filterInputs = append(filterInputs, filter.handle())
	}

	multiplex(lines, filterInputs)

	signal <- s
}

var wgActions sync.WaitGroup

var db gob.Encoder

func Main() {
	confFilename := flag.String("c", "", "configuration file. see an example at https://framagit.org/ppom/reaction/-/blob/main/reaction.yml")
	flag.Parse()

	if *confFilename == "" {
		flag.PrintDefaults()
		os.Exit(2)
	}

	conf := parseConf(*confFilename)

	db = openDB()

	endSignals := make(chan *Stream)

	for _, stream := range conf.Streams {
		go stream.handle(endSignals)
	}

	for i := 0; i < len(conf.Streams); i++ {
		finishedStream := <-endSignals
		log.Printf("ERR  %s stream finished", finishedStream.name)
	}

	wgActions.Wait()

	os.Exit(3)
}
