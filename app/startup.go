package app

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"framagit.org/ppom/reaction/logger"

	"github.com/google/go-jsonnet"
)

func (c *Conf) setup() {

	for patternName := range c.Patterns {
		pattern := c.Patterns[patternName]
		pattern.name = patternName
		pattern.nameWithBraces = fmt.Sprintf("<%s>", pattern.name)

		if pattern.Regex == "" {
			logger.Fatalf("Bad configuration: pattern's regex %v is empty!", patternName)
		}

		compiled, err := regexp.Compile(fmt.Sprintf("^%v$", pattern.Regex))
		if err != nil {
			logger.Fatalf("Bad configuration: pattern %v doesn't compile!", patternName)
		}
		c.Patterns[patternName].Regex = fmt.Sprintf("(?P<%s>%s)", patternName, pattern.Regex)
		for _, ignore := range pattern.Ignore {
			if !compiled.MatchString(ignore) {
				logger.Fatalf("Bad configuration: pattern ignore '%v' doesn't match pattern %v! It should be fixed or removed.", ignore, pattern.nameWithBraces)
			}
		}
	}

	if len(c.Streams) == 0 {
		logger.Fatalln("Bad configuration: no streams configured!")
	}
	for streamName := range c.Streams {

		stream := c.Streams[streamName]
		stream.name = streamName

		if strings.Contains(stream.name, ".") {
			logger.Fatalln("Bad configuration: character '.' is not allowed in stream names", stream.name)
		}

		if len(stream.Filters) == 0 {
			logger.Fatalln("Bad configuration: no filters configured in", stream.name)
		}
		for filterName := range stream.Filters {

			filter := stream.Filters[filterName]
			filter.stream = stream
			filter.name = filterName

			if strings.Contains(filter.name, ".") {
				logger.Fatalln("Bad configuration: character '.' is not allowed in filter names", filter.name)
			}
			// Parse Duration
			if filter.RetryPeriod == "" {
				if filter.Retry > 1 {
					logger.Fatalln("Bad configuration: retry but no retry-duration in", stream.name, ".", filter.name)
				}
			} else {
				retryDuration, err := time.ParseDuration(filter.RetryPeriod)
				if err != nil {
					logger.Fatalln("Bad configuration: Failed to parse retry time in", stream.name, ".", filter.name, ":", err)
				}
				filter.retryDuration = retryDuration
			}

			if len(filter.Regex) == 0 {
				logger.Fatalln("Bad configuration: no regexes configured in", stream.name, ".", filter.name)
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
							logger.Fatalf(
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
				logger.Fatalln("Bad configuration: no actions configured in", stream.name, ".", filter.name)
			}
			for actionName := range filter.Actions {

				action := filter.Actions[actionName]
				action.filter = filter
				action.name = actionName

				if strings.Contains(action.name, ".") {
					logger.Fatalln("Bad configuration: character '.' is not allowed in action names", action.name)
				}
				// Parse Duration
				if action.After != "" {
					afterDuration, err := time.ParseDuration(action.After)
					if err != nil {
						logger.Fatalln("Bad configuration: Failed to parse after time in ", stream.name, ".", filter.name, ".", action.name, ":", err)
					}
					action.afterDuration = afterDuration
				} else if action.OnExit {
					logger.Fatalln("Bad configuration: Cannot have `onexit: true` without an `after` directive in", stream.name, ".", filter.name, ".", action.name)
				}
				if filter.longuestActionDuration == nil || filter.longuestActionDuration.Milliseconds() < action.afterDuration.Milliseconds() {
					filter.longuestActionDuration = &action.afterDuration
				}
			}
		}
	}
}

func parseConf(filename string) *Conf {

	data, err := os.Open(filename)
	if err != nil {
		logger.Fatalln("Failed to read configuration file:", err)
	}

	var conf Conf
	if filename[len(filename)-4:] == ".yml" || filename[len(filename)-5:] == ".yaml" {
		err = jsonnet.NewYAMLToJSONDecoder(data).Decode(&conf)
	} else {
		var jsondata string
		jsondata, err = jsonnet.MakeVM().EvaluateFile(filename)
		if err == nil {
			err = json.Unmarshal([]byte(jsondata), &conf)
		}
	}
	if err != nil {
		logger.Fatalln("Failed to parse configuration file:", err)
	}

	conf.setup()
	return &conf
}
