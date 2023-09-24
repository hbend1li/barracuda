package app

import (
	"bufio"
	"encoding/gob"
	"fmt"
	"log"
	"net"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

const (
	Show  = 0
	Flush = 1
)

type Request struct {
	Request int
	Pattern string
}

type Response struct {
	Err          error
	ClientStatus ClientStatus
	Number       int
}

func SendAndRetrieve(data Request) Response {
	conn, err := net.Dial("unix", *SocketPath)
	if err != nil {
		log.Fatalln("Error opening connection top daemon:", err)
	}
	defer conn.Close()

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

type PatternStatus struct {
	Matches int                 `yaml:"matches_since_last_trigger"`
	Actions map[string][]string `yaml:"pending_actions"`
}
type MapPatternStatus map[string]*PatternStatus
type ClientStatus map[string]map[string]MapPatternStatus

// This block is made to hide pending_actions when empty
// and matches_since_last_trigger when zero
type FullPatternStatus PatternStatus
type MatchesStatus struct {
	Matches int `yaml:"matches_since_last_trigger"`
}
type ActionsStatus struct {
	Actions map[string][]string `yaml:"pending_actions"`
}

func (mps MapPatternStatus) MarshalYAML() (interface{}, error) {
	ret := make(map[string]interface{})
	for k, v := range mps {
		if v.Matches == 0 {
			if len(v.Actions) != 0 {
				ret[k] = ActionsStatus{v.Actions}
			}
		} else {
			if len(v.Actions) != 0 {
				ret[k] = v
			} else {
				ret[k] = MatchesStatus{v.Matches}
			}
		}
	}
	return ret, nil
}

// end block

func usage(err string) {
	fmt.Println("Usage: reactionc")
	fmt.Println("Usage: reactionc flush <PATTERN>")
	log.Fatalln(err)
}

func ClientShow(streamfilter string) {
	response := SendAndRetrieve(Request{Show, streamfilter})
	if response.Err != nil {
		log.Fatalln("Received error from daemon:", response.Err)
		os.Exit(1)
	}
	text, err := yaml.Marshal(response.ClientStatus)
	if err != nil {
		log.Fatalln("Failed to convert daemon binary response to text format:", err)
	}
	fmt.Println(string(text))
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
