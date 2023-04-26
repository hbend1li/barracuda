package app

import (
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Conf struct {
	Patterns map[string]string  `yaml:"patterns"`
	Streams  map[string]*Stream `yaml:"streams"`
}

type Stream struct {
	name string `yaml:"-"`

	Cmd     []string           `yaml:"cmd"`
	Filters map[string]*Filter `yaml:"filters"`
}

type Filter struct {
	stream *Stream `yaml:"-"`
	name   string  `yaml:"-"`

	Regex             []string        `yaml:"regex"`
	compiledRegex     []regexp.Regexp `yaml:"-"`
	patternName       string          `yaml:"-"`
	patternWithBraces string          `yaml:"-"`

	Retry         int           `yaml:"retry"`
	RetryPeriod   string        `yaml:"retry-period"`
	retryDuration time.Duration `yaml:"-"`

	Actions                map[string]*Action `yaml:"actions"`
	longuestActionDuration *time.Duration

	matches map[string][]time.Time `yaml:"-"`
}

type Action struct {
	filter *Filter `yaml:"-"`
	name   string  `yaml:"-"`

	Cmd []string `yaml:"cmd"`

	After         string        `yaml:"after"`
	afterDuration time.Duration `yaml:"-"`
}

type LogEntry struct {
	t              time.Time
	pattern        string
	stream, filter string
	exec           bool
}

func (c *Conf) setup() {
	for patternName, pattern := range c.Patterns {
		c.Patterns[patternName] = fmt.Sprintf("(?P<%s>%s)", patternName, pattern)
	}
	if len(c.Streams) == 0 {
		log.Fatalln("Bad configuration: no streams configured!")
	}
	for streamName := range c.Streams {

		stream := c.Streams[streamName]
		stream.name = streamName

		if len(stream.Filters) == 0 {
			log.Fatalln("Bad configuration: no filters configured in '%s'!", stream.name)
		}
		for filterName := range stream.Filters {

			filter := stream.Filters[filterName]
			filter.stream = stream
			filter.name = filterName
			filter.matches = make(map[string][]time.Time)

			// Parse Duration
			retryDuration, err := time.ParseDuration(filter.RetryPeriod)
			if err != nil {
				log.Fatalln("Failed to parse time in configuration file:", err)
			}
			filter.retryDuration = retryDuration

			if len(filter.Regex) == 0 {
				log.Fatalln("Bad configuration: no regexes configured in '%s.%s'!", stream.name, filter.name)
			}
			// Compute Regexes
			// Look for Patterns inside Regexes
			for _, regex := range filter.Regex {
				for patternName, pattern := range c.Patterns {
					if strings.Contains(regex, patternName) {

						switch filter.patternName {
						case "":
							filter.patternName = patternName
							filter.patternWithBraces = fmt.Sprintf("<%s>", patternName)
						case patternName:
							// no op
						default:
							log.Fatalf(
								"ERROR Can't mix different patterns (%s, %s) in same filter (%s.%s)\n",
								filter.patternName, patternName, streamName, filterName,
							)
						}

						regex = strings.Replace(regex, fmt.Sprintf("<%s>", patternName), pattern, 1)
					}
				}
				filter.compiledRegex = append(filter.compiledRegex, *regexp.MustCompile(regex))
			}

			if len(filter.Actions) == 0 {
				log.Fatalln("Bad configuration: no actions configured in '%s.%s'!", stream.name, filter.name)
			}
			for actionName := range filter.Actions {

				action := filter.Actions[actionName]
				action.filter = filter
				action.name = actionName

				// Parse Duration
				if action.After != "" {
					afterDuration, err := time.ParseDuration(action.After)
					if err != nil {
						log.Fatalln("Failed to parse time in configuration file:", err)
					}
					action.afterDuration = afterDuration
				}
			}
		}
	}
}

var DBname = "./reaction.db"
var DBnewName = "./reaction.new.db"

func (c *Conf) updateFromDB() {
	file, err := os.Open(DBname)
	if err != nil {
		if err == os.ErrNotExist {
			log.Printf("WARN: No DB found at %s\n", DBname)
			return
		}
		log.Fatalln("Failed to open DB:", err)
	}
	dec := gob.NewDecoder(&file)

	newfile, err := os.Create(DBnewName)
	if err != nil {
		log.Fatalln("Failed to open DB:", err)
	}
	enc := gob.NewEncoder(&newfile)

	// This extra code is made to warn only one time for each non-existant filter
	type SF struct{ s, f string }
	discardedEntries := make(map[SF]bool)
	malformedEntries := 0
	defer func() {
		for sf, t := range discardedEntries {
			if t {
				log.Printf("WARN: info discarded from the DB: stream/filter not found: %s.%s\n", sf.s, sf.f)
			}
		}
		if malformedEntries > 0 {
			log.Printf("WARN: %v malformed entry discarded from the DB\n", malformedEntries)
		}
	}()

	encodeOrFatal := func(entry LogEntry) {
		err = enc.Encode(entry)
		if err != nil {
			log.Fatalln("ERRO: couldn't write to new DB:", err)
		}
	}

	now := time.Now()
	for {
		var entry LogEntry
		var filter *Filter

		// decode entry
		err = dec.Decode(&entry)
		if err != nil {
			if err == io.EOF {
				return
			}
			malformedEntries++
			continue
		}

		// retrieve related filter
		if s := c.Streams[entry.stream]; stream != nil {
			filter = stream.Filters[entry.filter]
		}
		if filter == nil {
			discardedEntries[SF{entry.stream, entry.filter}] = true
			continue
		}

		// store matches
		if !entry.exec && entry.t+filter.retryDuration > now {
			filter.matches[entry.pattern] = append(f.matches[entry.pattern], entry.t)

			encodeOrFatal(entry)
		}

		// replay executions
		if entry.exec && entry.t+filter.longuestActionDuration > now {
			delete(filter.matches, match)
			filter.execActions(match, now-entry.t)

			encodeOrFatal(entry)
		}
	}

	err = os.Rename(DBnewName, DBname)
	if err != nil {
		log.Fatalln("ERRO: Failed to replace old DB with new one:", err)
	}
}

func openDB() {
	f, err := os.OpenFile(DBname, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		log.Fatalln("Failed to open DB:", err)
	}
	return gob.NewEncoder(&f)
}

func parseConf(filename string) *Conf {

	data, err := os.ReadFile(filename)

	if err != nil {
		log.Fatalln("Failed to read configuration file:", err)
	}

	var conf Conf
	err = yaml.Unmarshal(data, &conf)
	if err != nil {
		log.Fatalln("Failed to parse configuration file:", err)
	}

	conf.setup()

	return &conf
}
