package app

import (
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path"
	"time"
)

const (
	Query = 0
	Flush = 1
)

type Request struct {
	Request int
	Id      int
	Pattern string
}

type Response struct {
	Err     error
	Actions ReadableMap
}

// Runtime files:
// /run/user/<uid>/reaction/reaction.pipe
// /run/user/<uid>/reaction/id.response

func RuntimeDirectory() string {
	return fmt.Sprintf("/run/user/%v/reaction/", os.Getuid())
}

func PipePath() string {
	return path.Join(RuntimeDirectory(), "reaction.pipe")
}

func (r Request) ResponsePath() string {
	return path.Join(RuntimeDirectory(), string(r.Id))
}

func Send(data Request) {
	pipePath := PipePath()
	pipe, err := os.OpenFile(pipePath, os.O_RDWR, os.ModeNamedPipe)
	if err != nil {
		log.Println("Failed to open", pipePath, ":", err)
		log.Fatalln("Is the reaction daemon running? Does the CLI run as the same user?")
	}
	log.Println("DEBUG opening ok, encoding...")
	enc := gob.NewEncoder(pipe)
	err = enc.Encode(data)
	if err != nil {
		log.Fatalf("Failed to write to %s: %s", pipePath, err)
	}
}

func SendAndRetrieve(data Request) Response {
	if data.Id == 0 {
		data.Id = rand.Int()
	}
	log.Println("DEBUG sending:", data)
	Send(data)
	responsePath := data.ResponsePath()
	d, _ := time.ParseDuration("100ms")
	for tries := 20; tries > 0; tries-- {
		log.Println("DEBUG waiting for answer...")
		file, err := os.Open(responsePath)
		if errors.Is(err, os.ErrNotExist) {
			time.Sleep(d)
			continue
		}
		defer os.Remove(responsePath)
		if err != nil {
			log.Fatalf("Error opening daemon answer: %s", err)
		}
		var response Response
		err = gob.NewDecoder(file).Decode(&response)
		if err != nil {
			log.Fatalf("Error parsing daemon answer: %s", err)
		}
		return response
	}
	log.Fatalln("Timeout while waiting answer from the daemon")
	return Response{errors.New("unreachable code"), nil}
}

func usage(err string) {
	fmt.Println("Usage: reactionc")
	fmt.Println("Usage: reactionc flush <PATTERN>")
	log.Fatalln(err)
}

func CLI() {
	if len(os.Args) <= 1 {
		response := SendAndRetrieve(Request{Query, 0, ""})
		if response.Err != nil {
			log.Fatalln("Received error from daemon:", response.Err)
		}
		fmt.Println(response.Actions.ToString())
		os.Exit(0)
	}
	switch os.Args[1] {
	case "flush":
		if len(os.Args) != 3 {
			usage("flush takes one <PATTERN> argument")
		}
		response := SendAndRetrieve(Request{Flush, 0, os.Args[2]})
		if response.Err != nil {
			log.Fatalln("Received error from daemon:", response.Err)
		}
		os.Exit(0)
	default:
		usage("first argument must be `flush`")
	}
}
