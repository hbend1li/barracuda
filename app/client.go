package app

import (
	"bufio"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"

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

func ClientShow(format, stream, filter string, regex *regexp.Regexp) {
	response := SendAndRetrieve(Request{Show, ""})
	if response.Err != nil {
		logger.Fatalln("Received error from daemon:", response.Err)
	}

	// Remove empty structs
	for streamName := range response.ClientStatus {
		for filterName := range response.ClientStatus[streamName] {
			for patternName, patternMap := range response.ClientStatus[streamName][filterName] {
				if len(patternMap.Actions) == 0 && patternMap.Matches == 0 {
					delete(response.ClientStatus[streamName][filterName], patternName)
				}
			}
			if len(response.ClientStatus[streamName][filterName]) == 0 {
				delete(response.ClientStatus[streamName], filterName)
			}
		}
		if len(response.ClientStatus[streamName]) == 0 {
			delete(response.ClientStatus, streamName)
		}
	}

	// Limit to stream, filter if exists
	if stream != "" {
		exists := false
		for streamName := range response.ClientStatus {
			if stream == streamName {
				if filter != "" {
					for filterName := range response.ClientStatus[streamName] {
						if filter == filterName {
							exists = true
						} else {
							delete(response.ClientStatus[streamName], filterName)
						}
					}
				} else {
					exists = true
				}
			} else {
				delete(response.ClientStatus, streamName)
			}
		}
		if !exists {
			logger.Println(logger.WARN, "No matching stream.filter items found. This does not mean it doesn't exist, maybe it just didn't receive any match.")
			os.Exit(1)
		}
	}

	// Limit to pattern
	if regex != nil {
		for streamName := range response.ClientStatus {
			for filterName := range response.ClientStatus[streamName] {
				for patternName := range response.ClientStatus[streamName][filterName] {
					if !regex.MatchString(patternName) {
						delete(response.ClientStatus[streamName][filterName], patternName)
					}
				}
				if len(response.ClientStatus[streamName][filterName]) == 0 {
					delete(response.ClientStatus[streamName], filterName)
				}
			}
			if len(response.ClientStatus[streamName]) == 0 {
				delete(response.ClientStatus, streamName)
			}
		}
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

func Match(confFilename, regex, line string) {
	conf := parseConf(confFilename)

	// Code close to app/startup.go
	var usedPattern *Pattern
	for patternName, pattern := range conf.Patterns {
		if strings.Contains(regex, pattern.nameWithBraces) {
			if usedPattern != nil {
				logger.Fatalf("Bad regex: Can't mix different patterns (%s, %s) in same line", usedPattern.name, patternName)
			}
			usedPattern = pattern
			regex = strings.Replace(regex, pattern.nameWithBraces, pattern.Regex, 1)
		}
	}
	reg, err := regexp.Compile(regex)
	if err != nil {
		logger.Fatalln("ERROR the specified regex is invalid: %v", err)
		os.Exit(1)
	}

	match := func(line string) {
		if matches := reg.FindStringSubmatch(line); matches != nil {
			if usedPattern != nil {
				match := matches[reg.SubexpIndex(usedPattern.name)]

				if usedPattern.notAnIgnore(&match) {
					fmt.Printf("\033[32mmatching\033[0m [%v]: %v\n", match, line)
				} else {
					fmt.Printf("\033[33mignore matching\033[0m [%v]: %v\n", match, line)
				}
			} else {
				fmt.Printf("\033[32mmatching\033[0m [%v]:\n", line)
			}
		} else {
			fmt.Printf("\033[31mno match\033[0m: %v\n", line)
		}
	}

	if line != "" {
		match(line)
	} else {
		logger.Println(logger.INFO, "no second argument: reading from stdin")
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			match(scanner.Text())
		}
	}
}
