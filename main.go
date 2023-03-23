package main

import (
	"bufio"
	"log"
	"os/exec"
	"regexp"
)

type Action struct {
	regex, cmd []string
}

type compiledAction struct {
	regex []regexp.Regexp
	cmd   []string
}

type Stream struct {
	cmd     []string
	actions []Action
}

func compileAction(action Action) compiledAction {
	var ca compiledAction
	ca.cmd = action.cmd
	for _, regex := range action.regex {
		ca.regex = append(ca.regex, *regexp.MustCompile(regex))
	}
	return ca
}

// Handle a log command
// Must be started in a goroutine
func streamHandle(stream Stream, execQueue chan []string) {
	log.Printf("streamHandle{%v}: start\n", stream.cmd)
	cmd := exec.Command(stream.cmd[0], stream.cmd[1:]...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal("couldn't open stdout on command:", err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatal("couldn't start command:", err)
	}
	defer stdout.Close()

	compiledActions := make([]compiledAction, 0, len(stream.actions))
	for _, action := range stream.actions {
		compiledActions = append(compiledActions, compileAction(action))
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		for _, action := range compiledActions {
			for _, regex := range action.regex {
				if match := regex.FindString(line); match != "" {
					log.Printf("match `%v` in line: `%v`\n", regex.String(), line)
					execQueue <- action.cmd
				}
			}
		}
	}
}

func execQueue() chan []string {
	queue := make(chan []string)
	go func() {
		for {
			command := <-queue
			cmd := exec.Command(command[0], command[1:]...)
			if ret := cmd.Run(); ret != nil {
				log.Printf("Error launching `%v`: code %v\n", cmd, ret)
			}
		}
	}()
	return queue
}

func main() {
	conf := parseConf("./reaction.yml")
	conf = conf
	// mockstreams := []Stream{Stream{
	// 	[]string{"tail", "-f", "/home/ao/DOWN"},
	// 	[]Action{Action{
	// 		[]string{"prout.dev"},
	// 		[]string{"touch", "/home/ao/DAMN"},
	// 	}},
	// }}
	// streams := mockstreams
	// log.Println(streams)
	// queue := execQueue()
	// for _, stream := range streams {
	// 	go streamHandle(stream, queue)
	// }
	// // Infinite wait
	// <-make(chan bool)
}
