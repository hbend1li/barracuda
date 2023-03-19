package main

import (
	"log"
	"os/exec"
)

type Action struct {
	regex, cmd []string
}

type Stream struct {
	cmd     []string
	actions []Action
}

func streamHandle(stream Stream, execQueue chan string) {
	log.Printf("streamHandle{%v}: start\n", stream.cmd)
	cmd := exec.Command(stream.cmd...)
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
		// try to match and send to execQueue if matching
	}
}

func execQueue() chan string {
	queue := make(chan string)
	go func() {
		for {
			command := <-queue
			return_code := run(command)
			if return_code != 0 {
				log.Printf("Error launching `%v`\n", command)
			}
		}
	}()
	return queue
}

func main() {
	mockstreams := []Stream{Stream{
		[]string{"tail", "-f", "/home/ao/DOWN"},
		[]Action{Action{
			"prout.dev",
			[]string{"echo", "DAMN"},
		}},
	}}
	streams := mockstreams
	log.Println(streams)
	queue := execQueue()
	stop := make(chan bool)
	for _, stream := range streams {
		go streamHandle(stream, queue)
	}
}
