package app

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Conf struct {
	Patterns map[string]string
	Streams  map[string]*Stream
}

type Stream struct {
	name string

	Cmd     []string
	Filters map[string]*Filter
}

type Filter struct {
	stream *Stream
	name   string

	Regex                          []string
	compiledRegex                  []regexp.Regexp
	patternName, patternWithBraces string

	Retry         uint
	RetryPeriod   string `yaml:"retry-period"`
	retryDuration time.Duration

	Actions map[string]*Action

	matches map[string][]time.Time
}

type Action struct {
	filter *Filter
	name   string

	Cmd []string

	After         string `yaml:",omitempty"`
	afterDuration time.Duration
}

func (c *Conf) setup() {
	for patternName, pattern := range c.Patterns {
		c.Patterns[patternName] = fmt.Sprintf("(?P<%s>%s)", patternName, pattern)
	}
	for streamName := range c.Streams {

		stream := c.Streams[streamName]
		stream.name = streamName

		for filterName := range stream.Filters {

			filter := stream.Filters[filterName]
			filter.stream = stream
			filter.name = filterName

			// Parse Duration
			retryDuration, err := time.ParseDuration(filter.RetryPeriod)
			if err != nil {
				log.Fatalln("Failed to parse time in configuration file:", err)
			}
			filter.retryDuration = retryDuration

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
