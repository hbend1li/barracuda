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
	Patterns map[string]*Pattern `yaml:"patterns"`
	Streams  map[string]*Stream  `yaml:"streams"`
}

type Pattern struct {
	Regex  string   `yaml:"regex"`
	Ignore []string `yaml:"ignore"`

	name           string `yaml:"-"`
	nameWithBraces string `yaml:"-"`
}

// Stream, Filter & Action structures must never be copied.
// They're always referenced through pointers

type Stream struct {
	name string `yaml:"-"`

	Cmd     []string           `yaml:"cmd"`
	Filters map[string]*Filter `yaml:"filters"`
}

type Filter struct {
	stream *Stream `yaml:"-"`
	name   string  `yaml:"-"`

	Regex         []string        `yaml:"regex"`
	compiledRegex []regexp.Regexp `yaml:"-"`
	pattern       *Pattern        `yaml:"-"`

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

	OnExit bool `yaml:"onexit"`
}

type LogEntry struct {
	T              time.Time
	Pattern        string
	Stream, Filter string
	Exec           bool
}

func (c *Conf) setup() {

	for patternName := range c.Patterns {
		pattern := c.Patterns[patternName]
		pattern.name = patternName
		pattern.nameWithBraces = fmt.Sprintf("<%s>", pattern.name)

		if pattern.Regex == "" {
			log.Fatalf("FATAL Bad configuration: pattern's regex %v is empty!", patternName)
		}

		compiled, err := regexp.Compile(fmt.Sprintf("^%v$", pattern.Regex))
		if err != nil {
			log.Fatalf("FATAL Bad configuration: pattern %v doesn't compile!", patternName)
		}
		c.Patterns[patternName].Regex = fmt.Sprintf("(?P<%s>%s)", patternName, pattern.Regex)
		for _, ignore := range pattern.Ignore {
			if !compiled.MatchString(ignore) {
				log.Fatalf("FATAL Bad configuration: pattern ignore '%v' doesn't match pattern %v! It should be fixed or removed.", ignore, pattern.nameWithBraces)
			}
		}
	}

	if len(c.Streams) == 0 {
		log.Fatalln("FATAL Bad configuration: no streams configured!")
	}
	for streamName := range c.Streams {

		stream := c.Streams[streamName]
		stream.name = streamName

		if len(stream.Filters) == 0 {
			log.Fatalln("FATAL Bad configuration: no filters configured in", stream.name)
		}
		for filterName := range stream.Filters {

			filter := stream.Filters[filterName]
			filter.stream = stream
			filter.name = filterName
			filter.matches = make(map[string][]time.Time)

			// Parse Duration
			if filter.RetryPeriod == "" {
				if filter.Retry > 1 {
					log.Fatalln("FATAL Bad configuration: retry but no retry-duration in", stream.name, ".", filter.name)
				}
			} else {
				retryDuration, err := time.ParseDuration(filter.RetryPeriod)
				if err != nil {
					log.Fatalln("FATAL Bad configuration: Failed to parse retry time in", stream.name, ".", filter.name, ":", err)
				}
				filter.retryDuration = retryDuration
			}

			if len(filter.Regex) == 0 {
				log.Fatalln("FATAL Bad configuration: no regexes configured in", stream.name, ".", filter.name)
			}
			// Compute Regexes
			// Look for Patterns inside Regexes
			for _, regex := range filter.Regex {
				for patternName, pattern := range c.Patterns {
					if strings.Contains(regex, pattern.nameWithBraces) {

						if filter.pattern == nil {
							filter.pattern = pattern
						} else if filter.pattern == pattern {
							// no op
						} else {
							log.Fatalf(
								"Bad configuration: Can't mix different patterns (%s, %s) in same filter (%s.%s)\n",
								filter.pattern.name, patternName, streamName, filterName,
							)
						}

						// FIXME should go in the `if filter.pattern == nil`?
						regex = strings.Replace(regex, pattern.nameWithBraces, pattern.Regex, 1)
					}
				}
				// TODO regexp.Compile and show proper message if it doesn't instead of panicing
				filter.compiledRegex = append(filter.compiledRegex, *regexp.MustCompile(regex))
			}

			if len(filter.Actions) == 0 {
				log.Fatalln("FATAL Bad configuration: no actions configured in", stream.name, ".", filter.name)
			}
			for actionName := range filter.Actions {

				action := filter.Actions[actionName]
				action.filter = filter
				action.name = actionName

				// Parse Duration
				if action.After != "" {
					afterDuration, err := time.ParseDuration(action.After)
					if err != nil {
						log.Fatalln("FATAL Bad configuration: Failed to parse after time in ", stream.name, ".", filter.name, ".", action.name, ":", err)
					}
					action.afterDuration = afterDuration
				} else if action.OnExit {
					log.Fatalln("FATAL Bad configuration: Cannot have `onexit: true` without an `after` directive in", stream.name, ".", filter.name, ".", action.name)
				}
				if filter.longuestActionDuration == nil || filter.longuestActionDuration.Milliseconds() < action.afterDuration.Milliseconds() {
					filter.longuestActionDuration = &action.afterDuration
				}
			}
		}
	}
}

func parseConf(filename string) *Conf {

	data, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalln("FATAL Failed to read configuration file:", err)
	}

	var conf Conf
	err = yaml.Unmarshal(data, &conf)
	if err != nil {
		log.Fatalln("FATAL Failed to parse configuration file:", err)
	}

	conf.setup()
	return &conf
}
