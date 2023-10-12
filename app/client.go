package app

import (
	"bufio"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"regexp"

	"framagit.org/ppom/reaction/logger"
	"sigs.k8s.io/yaml"
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
}

func SendAndRetrieve(data Request) Response {
	conn, err := net.Dial("unix", *SocketPath)
	if err != nil {
		logger.Fatalln("Error opening connection top daemon:", err)
	}
	defer conn.Close()

	err = gob.NewEncoder(conn).Encode(data)
	if err != nil {
		logger.Fatalln("Can't send message:", err)
	}

	var response Response
	err = gob.NewDecoder(conn).Decode(&response)
	if err != nil {
		logger.Fatalln("Invalid answer from daemon:", err)
	}
	return response
}

type PatternStatus struct {
	Matches int                 `json:"matches,omitempty"`
	Actions map[string][]string `json:"actions,omitempty"`
}
type MapPatternStatus map[string]*PatternStatus
type MapPatternStatusFlush MapPatternStatus

type ClientStatus map[string]map[string]MapPatternStatus
type ClientStatusFlush ClientStatus

func (mps MapPatternStatusFlush) MarshalJSON() ([]byte, error) {
	for _, v := range mps {
		return json.Marshal(v)
	}
	return []byte(""), nil
}

func (csf ClientStatusFlush) MarshalJSON() ([]byte, error) {
	ret := make(map[string]map[string]MapPatternStatusFlush)
	for k, v := range csf {
		ret[k] = make(map[string]MapPatternStatusFlush)
		for kk, vv := range v {
			ret[k][kk] = MapPatternStatusFlush(vv)
		}
	}
	return json.Marshal(ret)
}

// end block

func usage(err string) {
	fmt.Println("Usage: reactionc")
	fmt.Println("Usage: reactionc flush <PATTERN>")
	logger.Fatalln(err)
}

func ClientShow(streamfilter, format string) {
	response := SendAndRetrieve(Request{Show, streamfilter})
	if response.Err != nil {
		logger.Fatalln("Received error from daemon:", response.Err)
		os.Exit(1)
	}
	var text []byte
	var err error
	if format == "json" {
		text, err = json.MarshalIndent(response.ClientStatus, "", "  ")
	} else {
		text, err = yaml.Marshal(response.ClientStatus)
	}
	if err != nil {
		logger.Fatalln("Failed to convert daemon binary response to text format:", err)
	}
	fmt.Println(string(text))
	os.Exit(0)
}

func ClientFlush(pattern, streamfilter, format string) {
	response := SendAndRetrieve(Request{Flush, pattern})
	if response.Err != nil {
		logger.Fatalln("Received error from daemon:", response.Err)
		os.Exit(1)
	}
	var text []byte
	var err error
	if format == "json" {
		text, err = json.MarshalIndent(ClientStatusFlush(response.ClientStatus), "", "  ")
	} else {
		text, err = yaml.Marshal(ClientStatusFlush(response.ClientStatus))
	}
	if err != nil {
		logger.Fatalln("Failed to convert daemon binary response to text format:", err)
	}
	fmt.Println(string(text))
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
