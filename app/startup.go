package app

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	"framagit.org/ppom/reaction/logger"

	"github.com/google/go-jsonnet"
)

func (c *Conf) setup() {
	if c.Concurrency == 0 {
		c.Concurrency = runtime.NumCPU()
	}

	for patternName := range c.Patterns {
		pattern := c.Patterns[patternName]
		pattern.name = patternName
		pattern.nameWithBraces = fmt.Sprintf("<%s>", pattern.name)

		if pattern.Regex == "" {
			logger.Fatalf("Bad configuration: pattern's regex %v is empty!", patternName)
		}

		compiled, err := regexp.Compile(fmt.Sprintf("^%v$", pattern.Regex))
		if err != nil {
			logger.Fatalf("Bad configuration: pattern %v: %v", patternName, err)
		}
		c.Patterns[patternName].Regex = fmt.Sprintf("(?P<%s>%s)", patternName, pattern.Regex)
		for _, ignore := range pattern.Ignore {
			if !compiled.MatchString(ignore) {
				logger.Fatalf("Bad configuration: pattern ignore '%v' doesn't match pattern %v! It should be fixed or removed.", ignore, pattern.nameWithBraces)
			}
		}

		// Compile ignore regexes
		for _, regex := range pattern.IgnoreRegex {
			// Enclose the regex to make sure that it matches the whole detected string
			compiledRegex, err := regexp.Compile("^" + regex + "$")
			if err != nil {
				logger.Fatalf("%vBad configuration: in ignoreregex of pattern %s: %v", logger.FATAL, pattern.name, err)
			}

			pattern.compiledIgnoreRegex = append(pattern.compiledIgnoreRegex, *compiledRegex)
		}
	}

	if len(c.Streams) == 0 {
		logger.Fatalln("Bad configuration: no streams configured!")
	}
	for streamName := range c.Streams {

		stream := c.Streams[streamName]
		stream.name = streamName

		if strings.Contains(stream.name, ".") {
			logger.Fatalf("Bad configuration: character '.' is not allowed in stream names: '%v'", stream.name)
		}

		if len(stream.Filters) == 0 {
			logger.Fatalf("Bad configuration: no filters configured in %v", stream.name)
		}
		for filterName := range stream.Filters {

			filter := stream.Filters[filterName]
			filter.stream = stream
			filter.name = filterName

			if strings.Contains(filter.name, ".") {
				logger.Fatalf("Bad configuration: character '.' is not allowed in filter names: '%v'", filter.name)
			}
			// Parse Duration
			if filter.RetryPeriod == "" {
				if filter.Retry > 1 {
					logger.Fatalf("Bad configuration: retry but no retryperiod in %v.%v", stream.name, filter.name)
				}
			} else {
				retryDuration, err := time.ParseDuration(filter.RetryPeriod)
				if err != nil {
					logger.Fatalf("Bad configuration: Failed to parse retry time in %v.%v: %v", stream.name, filter.name, err)
				}
				filter.retryDuration = retryDuration
			}

			if len(filter.Regex) == 0 {
				logger.Fatalf("Bad configuration: no regexes configured in %v.%v", stream.name, filter.name)
			}
			// Compute Regexes
			// Look for Patterns inside Regexes
			for _, regex := range filter.Regex {
				for _, pattern := range c.Patterns {
					if strings.Contains(regex, pattern.nameWithBraces) {
						if !slices.Contains(filter.pattern, pattern) {
							filter.pattern = append(filter.pattern, pattern)
						}
						regex = strings.Replace(regex, pattern.nameWithBraces, pattern.Regex, 1)
					}
				}
				compiledRegex, err := regexp.Compile(regex)
				if err != nil {
					logger.Fatalf("Bad configuration: regex of filter %s.%s: %v", stream.name, filter.name, err)
				}
				filter.compiledRegex = append(filter.compiledRegex, *compiledRegex)
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
		if err != nil {
			logger.Fatalln("Failed to parse yaml configuration file:", err)
		}
	} else {
		var jsondata string
		jsondata, err = jsonnet.MakeVM().EvaluateFile(filename)
		if err == nil {
			err = json.Unmarshal([]byte(jsondata), &conf)
		}
		if err != nil {
			logger.Fatalln("Failed to parse json configuration file:", err)
		}
	}

	conf.setup()
	return &conf
}
