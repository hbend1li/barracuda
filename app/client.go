package app

import (
	"encoding/gob"
	"fmt"
	"log"
	"net"
	"os"
)

const (
	Query = 0
	Flush = 1
)

type Request struct {
	Request int
	Pattern string
}

type Response struct {
	Err     error
	Actions ReadableMap
}

const SocketPath = "/run/reaction/reaction.sock"

func SendAndRetrieve(data Request) Response {
	conn, err := net.Dial("unix", SocketPath)
	if err != nil {
		log.Fatalln("Error opening connection top daemon:", err)
	}

	err = gob.NewEncoder(conn).Encode(data)
	if err != nil {
		log.Fatalln("Can't send message:", err)
	}

	var response Response
	err = gob.NewDecoder(conn).Decode(&response)
	if err != nil {
		log.Fatalln("Invalid answer from daemon:", err)
	}
	return response
}

func usage(err string) {
	fmt.Println("Usage: reactionc")
	fmt.Println("Usage: reactionc flush <PATTERN>")
	log.Fatalln(err)
}

func CLI() {
	if len(os.Args) <= 1 {
		response := SendAndRetrieve(Request{Query, ""})
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
		response := SendAndRetrieve(Request{Flush, os.Args[2]})
		if response.Err != nil {
			log.Fatalln("Received error from daemon:", response.Err)
		}
		os.Exit(0)
	default:
		usage("first argument must be `flush`")
	}
}
