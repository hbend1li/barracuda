package main

import (
	"bufio"
	"log"
	"os/exec"
)

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

func (f *Filter) match(line string) bool {
	log.Printf("trying to match line {%s}...\n", line)
	for _, regex := range f.compiledRegex {
		log.Printf("...on %s\n", regex.String())
		if match := regex.FindString(line); match != "" {
			log.Printf("match `%v` in line: `%v`\n", regex.String(), line)
			return true
		}
	}
	return false
}

func (f *Filter) launch(line *string) {
	for _, a := range f.Actions {
		go a.launch(line)
	}
}

func (a *Action) launch(line *string) {
	log.Printf("INFO %s.%s.%s: line {%s} → run {%s}\n", a.streamName, a.filterName, a.name, *line, a.Cmd)

	cmd := exec.Command(a.Cmd[0], a.Cmd[1:]...)
	if ret := cmd.Run(); ret != nil {
		log.Printf("ERR  %s.%s.%s: line {%s} → run %s, code {%s}\n", a.streamName, a.filterName, a.name, *line, a.Cmd, ret)
	}
}

func (s *Stream) handle() {
	log.Printf("streamHandle{%v}: start\n", s.Cmd)

	lines := cmdStdout(s.Cmd)

	for line := range lines {
		for _, filter := range s.Filters {
			if filter.match(line) {
				filter.launch(&line)
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
