package app

import (
	"encoding/gob"
	"errors"
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
	T              time.Time
	Pattern        string
	Stream, Filter string
	Exec           bool
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
				if filter.longuestActionDuration == nil || filter.longuestActionDuration.Milliseconds() < action.afterDuration.Milliseconds() {
					filter.longuestActionDuration = &action.afterDuration
				}
			}
		}
	}
}

var DBname = "./reaction.db"
var DBnewName = "./reaction.new.db"

func (c *Conf) updateFromDB() *gob.Encoder {
	file, err := os.Open(DBname)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Printf("WARN: No DB found at %s\n", DBname)

			file, err := os.Create(DBname)
			if err != nil {
				log.Fatalln("Failed to create DB:", err)
			}
			return gob.NewEncoder(file)
		}
		log.Fatalln("Failed to open DB:", err)
	}
	dec := gob.NewDecoder(file)

	newfile, err := os.Create(DBnewName)
	if err != nil {
		log.Fatalln("Failed to create new DB:", err)
	}
	enc := gob.NewEncoder(newfile)

	defer func() {
		err := file.Close()
		if err != nil {
			log.Fatalln("ERRO: Failed to close old DB:", err)
		}

		// It should be ok to rename an open file
		err = os.Rename(DBnewName, DBname)
		if err != nil {
			log.Fatalln("ERRO: Failed to replace old DB with new one:", err)
		}
	}()

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
			log.Printf("WARN: %v malformed entries discarded from the DB\n", malformedEntries)
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
				return enc
			}
			malformedEntries++
			continue
		}

		// retrieve related filter
		if stream := c.Streams[entry.Stream]; stream != nil {
			filter = stream.Filters[entry.Filter]
		}
		if filter == nil {
			discardedEntries[SF{entry.Stream, entry.Filter}] = true
			continue
		}

		// store matches
		if !entry.Exec && entry.T.Add(filter.retryDuration).Unix() > now.Unix() {
			filter.matches[entry.Pattern] = append(filter.matches[entry.Pattern], entry.T)

			encodeOrFatal(entry)
		}

		// replay executions
		if entry.Exec && entry.T.Add(*filter.longuestActionDuration).Unix() > now.Unix() {
			delete(filter.matches, entry.Pattern)
			filter.execActions(entry.Pattern, now.Sub(entry.T))

			encodeOrFatal(entry)
		}
	}
}

func parseConf(filename string) (*Conf, *gob.Encoder) {

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
	enc := conf.updateFromDB()

	return &conf, enc
}
