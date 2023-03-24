package main

import (
	"bufio"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

// Executes a command and channel-send its stdout
func cmdStdout(commandline []string) chan string {
	lines := make(chan string)

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
			lines <- scanner.Text()
		}
		close(lines)
	}()

	return lines
}

// Whether one of the filter's regexes is matched on a line
func (f *Filter) match(line string) string {
	for _, regex := range f.compiledRegex {

		if matches := regex.FindStringSubmatch(line); matches != nil {

			match := matches[regex.SubexpIndex(f.patternName)]

			log.Printf("INFO %s.%s: match `%v`\n", f.stream.name, f.name, match)
			return match
		}
	}
	return ""
}

func (f *Filter) execActions(match string) {
	pattern := fmt.Sprintf("<%s>", f.patternName)
	for _, a := range f.Actions {
		go a.exec(match, pattern)
	}
}

func (a *Action) exec(match, pattern string) {
	if a.afterDuration != 0 {
		time.Sleep(a.afterDuration)
	}

	computedCommand := make([]string, 0, len(a.Cmd))
	for _, item := range a.Cmd {
		computedCommand = append(computedCommand, strings.ReplaceAll(item, pattern, match))
	}

	log.Printf("INFO %s.%s.%s: run %s\n", a.filter.stream.name, a.filter.name, a.name, computedCommand)

	cmd := exec.Command(computedCommand[0], computedCommand[1:]...)
	if ret := cmd.Run(); ret != nil {
		log.Printf("ERR  %s.%s.%s: run %s, code %s\n", a.filter.stream.name, a.filter.name, a.name, computedCommand, ret)
	}
}

func (s *Stream) handle() {
	log.Printf("INFO %s: start %s\n", s.name, s.Cmd)

	lines := cmdStdout(s.Cmd)

	for line := range lines {
		for _, filter := range s.Filters {
			if match := filter.match(line); match != "" {
				filter.execActions(match)
			}
		}
	}
}

func main() {
	conf := parseConf("./reaction.yml")

	for _, stream := range conf.Streams {
		go stream.handle()
	}
	// Infinite wait
	<-make(chan bool)
}
