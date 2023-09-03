package app

import (
	"bufio"
	"encoding/gob"
	"fmt"
	"log"
	"net"
	"os"
	"regexp"
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
	Number  int
}

func SendAndRetrieve(data Request) Response {
	conn, err := net.Dial("unix", *SocketPath)
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

func ClientQuery(streamfilter string) {
	response := SendAndRetrieve(Request{Query, streamfilter})
	if response.Err != nil {
		log.Fatalln("Received error from daemon:", response.Err)
		os.Exit(1)
	}
	fmt.Println(response.Actions.ToString())
	os.Exit(0)
}

func ClientFlush(pattern, streamfilter string) {
	response := SendAndRetrieve(Request{Flush, pattern})
	if response.Err != nil {
		log.Fatalln("Received error from daemon:", response.Err)
		os.Exit(1)
	}
	fmt.Printf("flushed pattern %v times\n", response.Number)
	os.Exit(0)
}

func Match(reg *regexp.Regexp, line string) {
	if reg.MatchString(line) {
		fmt.Printf("\033[32mmatching\033[0m: %v\n", line)
	} else {
		fmt.Printf("\033[31mno match\033[0m: %v\n", line)
	}
}

func MatchStdin(reg *regexp.Regexp) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		Match(reg, scanner.Text())
	}
}
